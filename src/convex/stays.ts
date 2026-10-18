// M-LOD — reservation lifecycle state machine.
//
// Check-in and check-out are the system's highest-fanout transactions:
//   check-in  → room status + assignment + folio opening (deposits carried)
//               + domain events (CRM welcome points).
//   check-out → backfill any missed room nights, enforce settlement, close
//               folio, flip room to dirty, create housekeeping task, and
//               publish reservation.checked_out (CRM loyalty subscriber).
// All of it commits atomically — a failed step leaves no partial stay.

import { v, ConvexError } from "convex/values";
import { mutation, type MutationCtx, type QueryCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { addDaysISO, parseISODate, todayPropertyLocal, requireProperty } from "./lib";
import { emit, EVENTS } from "./events";
import { ensureForReservation, postChargeInternal, postRoomNight } from "./folio";
import { requireAuth, type AuthContext } from "./authz";

async function loadReservation(
  ctx: QueryCtx | MutationCtx,
  auth: AuthContext,
  reservationId: Id<"reservations">,
): Promise<Doc<"reservations">> {
  const res = (await ctx.db.get(reservationId)) as Doc<"reservations"> | null;
  if (!res || res.propertyId !== auth.propertyId) throw new ConvexError("Reservation not found");
  return res;
}

async function setRoomStatus(
  ctx: MutationCtx,
  auth: AuthContext,
  room: Doc<"rooms">,
  status: string,
  hkStatus?: string,
): Promise<void> {
  await ctx.db.patch(room._id, hkStatus ? { status, hkStatus } : { status });
  await emit(ctx, {
    propertyId: auth.propertyId,
    type: EVENTS.roomStatusChanged,
    source: "stays",
    entityId: room.number,
    payload: { roomId: room._id, number: room.number, status, hkStatus: hkStatus ?? room.hkStatus, previous: room.status },
    by: auth.name,
  });
}

async function activeAssignment(
  ctx: QueryCtx | MutationCtx,
  reservationId: Id<"reservations">,
): Promise<Doc<"roomAssignments"> | null> {
  const rows = await ctx.db.query("roomAssignments").withIndex("by_reservation", (q) => q.eq("reservationId", reservationId)).collect();
  return rows[0] ?? null;
}

// ── Check-in ─────────────────────────────────────────────────────────────
export const checkIn = mutation({
  args: { token: v.string(), reservationId: v.id("reservations"), roomId: v.optional(v.id("rooms")) },
  handler: async (ctx, { token, reservationId, roomId }) => {
    const auth = await requireAuth(ctx, token, "lodging.frontdesk.check_in");
    const property = await requireProperty(ctx);
    const res = await loadReservation(ctx, auth, reservationId);
    if (res.status === "checked_in") throw new ConvexError("Reservation is already checked in");
    if (res.status !== "pending" && res.status !== "confirmed") {
      throw new ConvexError(`Cannot check in from status "${res.status}"`);
    }

    const guest = (await ctx.db.get(res.guestId)) as Doc<"guests"> | null;
    if (guest?.blacklisted) throw new ConvexError("Guest is flagged — resolution required before check-in");

    // Resolve the room: explicit pick > existing assignment > auto-assign.
    let room: Doc<"rooms"> | null = null;
    if (roomId) {
      room = (await ctx.db.get(roomId)) as Doc<"rooms"> | null;
      if (!room || room.propertyId !== auth.propertyId) throw new ConvexError("Room not found");
      if (room.roomTypeId !== res.roomTypeId) throw new ConvexError("Room type does not match the reservation");
      if (room.status === "out_of_order") throw new ConvexError(`Room ${room.number} is out of order`);
      if (room.status.startsWith("occupied")) throw new ConvexError(`Room ${room.number} is occupied`);
      if (room.status === "reserved") {
        const holders = await ctx.db
          .query("roomAssignments")
          .withIndex("by_room", (q) => q.eq("roomId", room!._id))
          .collect();
        for (const h of holders) {
          if (h.reservationId === reservationId) continue;
          const held = (await ctx.db.get(h.reservationId)) as Doc<"reservations"> | null;
          if (held && (held.status === "pending" || held.status === "confirmed" || held.status === "checked_in")) {
            throw new ConvexError(`Room ${room.number} is reserved for arrival ${held.code}`);
          }
        }
      }
    } else {
      const existingAssignment = await activeAssignment(ctx, reservationId);
      if (existingAssignment) {
        room = (await ctx.db.get(existingAssignment.roomId)) as Doc<"rooms"> | null;
      }
      if (!room) {
        const candidates = await ctx.db
          .query("rooms")
          .withIndex("by_room_type", (q) => q.eq("roomTypeId", res.roomTypeId))
          .collect();
        const free = candidates
          .filter((r) => r.propertyId === auth.propertyId && (r.status === "vacant_clean" || r.status === "vacant_dirty") && r.hkStatus !== "dirty")
          .sort((a, b) => a.number.localeCompare(b.number));
        room = free[0] ?? null;
        if (!room) {
          const dirtyCount = candidates.filter(
            (r) => r.propertyId === auth.propertyId && (r.status === "vacant_dirty" || r.hkStatus === "dirty"),
          ).length;
          throw new ConvexError(
            dirtyCount > 0
              ? `No inspected room of type ${res.roomTypeId} available — ${dirtyCount} room(s) awaiting housekeeping`
              : "No available room of the booked type",
          );
        }
      }
    }

    const now = Date.now();
    const today = todayPropertyLocal(property.timezone);

    // Room assignment bookkeeping.
    const assignment = await activeAssignment(ctx, reservationId);
    if (assignment && assignment.roomId !== room._id) {
      await ctx.db.delete(assignment._id);
      await ctx.db.insert("roomAssignments", {
        propertyId: auth.propertyId,
        reservationId,
        roomId: room._id,
        assignedAt: now,
        assignedBy: auth.name,
      });
    } else if (!assignment) {
      await ctx.db.insert("roomAssignments", {
        propertyId: auth.propertyId,
        reservationId,
        roomId: room._id,
        assignedAt: now,
        assignedBy: auth.name,
      });
    }

    await setRoomStatus(ctx, auth, room, room.hkStatus === "dirty" ? "occupied_dirty" : "occupied_clean");

    // Reservation → checked_in; folio opens carrying prepayments.
    await ctx.db.patch(reservationId, { status: "checked_in", checkedInAt: now });
    const freshRes = (await ctx.db.get(reservationId)) as Doc<"reservations">;
    const folio = await ensureForReservation(ctx, freshRes);

    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.reservationCheckedIn,
      source: "stays",
      entityId: res.code,
      payload: {
        propertyId: auth.propertyId,
        guestId: res.guestId,
        reservationId,
        roomId: room._id,
        roomNumber: room.number,
        folioId: folio._id,
        folioCode: folio.code,
        checkInDate: res.checkInDate,
      },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "reservation.checked_in",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { room: room.number, folio: folio.code, checkedInOn: today },
      at: now,
    });

    return { ok: true as const, roomNumber: room.number, folioCode: folio.code };
  },
});

// ── Check-out ────────────────────────────────────────────────────────────
export const checkOut = mutation({
  args: { token: v.string(), reservationId: v.id("reservations") },
  handler: async (ctx, { token, reservationId }) => {
    const auth = await requireAuth(ctx, token, "lodging.frontdesk.check_out");
    const property = await requireProperty(ctx);
    const res = await loadReservation(ctx, auth, reservationId);
    if (res.status !== "checked_in") throw new ConvexError(`Cannot check out from status "${res.status}"`);

    const folio = await ensureForReservation(ctx, res);
    const today = todayPropertyLocal(property.timezone);

    // Backfill room nights missed by an interrupted night-audit run.
    const lastClosedSetting = await ctx.db
      .query("settings")
      .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", "nightaudit.last_closed"))
      .unique();
    let night = lastClosedSetting ? addDaysISO(lastClosedSetting.value, 1) : res.checkInDate;
    if (parseISODate(night) < parseISODate(res.checkInDate)) night = res.checkInDate;
    let nightsPosted = 0;
    while (night < res.checkOutDate && night < today) {
      if (await postRoomNight(ctx, res, folio, night, auth.name)) nightsPosted++;
      night = addDaysISO(night, 1);
    }

    // Settlement is mandatory — money never walks out the door.
    const freshFolio = (await ctx.db.get(folio._id)) as Doc<"folios">;
    const balanceKobo = freshFolio.totalKobo - freshFolio.paidKobo;
    if (balanceKobo > 0) {
      throw new ConvexError(
        `Folio ${freshFolio.code} is not settled — outstanding ${balanceKobo} kobo. Post settlement before check-out.`,
      );
    }

    const now = Date.now();
    await ctx.db.patch(folio._id, { status: "closed", closedAt: now });
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.folioClosed,
      source: "stays",
      entityId: freshFolio.code,
      payload: { folioId: folio._id, reservationId, totalKobo: freshFolio.totalKobo, paidKobo: freshFolio.paidKobo },
      by: auth.name,
    });

    await ctx.db.patch(reservationId, { status: "checked_out", checkedOutAt: now });

    // Room turns over: dirty + cleaning task for housekeeping.
    const assignment = await activeAssignment(ctx, reservationId);
    if (assignment) {
      const room = (await ctx.db.get(assignment.roomId)) as Doc<"rooms"> | null;
      if (room && !room.status.startsWith("out_of_order")) {
        await setRoomStatus(ctx, auth, room, "vacant_dirty", "dirty");
      }
      if (room) {
        await ctx.db.insert("hkTasks", {
          propertyId: auth.propertyId,
          roomId: room._id,
          type: "cleaning",
          priority: "high",
          status: "pending",
          scheduledDate: today,
          notes: `Checkout after stay ${res.code}`,
        });
      }
    }

    // Fan-out to CRM (loyalty accrual) through the bus — idempotent per stay.
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.reservationCheckedOut,
      source: "stays",
      entityId: res.code,
      payload: {
        propertyId: auth.propertyId,
        guestId: res.guestId,
        reservationId,
        nights: res.nights,
        totalKobo: freshFolio.totalKobo,
        folioCode: freshFolio.code,
      },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "reservation.checked_out",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { folio: freshFolio.code, totalKobo: freshFolio.totalKobo, nightsPosted },
      at: now,
    });

    return { ok: true as const, folioCode: freshFolio.code, nightsPosted, totalKobo: freshFolio.totalKobo };
  },
});

// ── Cancellation (policy-enforced) ───────────────────────────────────────
export const cancel = mutation({
  args: { token: v.string(), reservationId: v.id("reservations"), reason: v.string() },
  handler: async (ctx, { token, reservationId, reason }) => {
    const auth = await requireAuth(ctx, token, "lodging.reservations.cancel");
    const property = await requireProperty(ctx);
    const res = await loadReservation(ctx, auth, reservationId);
    if (res.status !== "pending" && res.status !== "confirmed") {
      throw new ConvexError(`Cannot cancel from status "${res.status}"`);
    }

    const ratePlan = res.ratePlanId ? ((await ctx.db.get(res.ratePlanId)) as Doc<"ratePlans"> | null) : null;
    const refundable = ratePlan ? ratePlan.refundable : true;
    const today = todayPropertyLocal(property.timezone);
    const daysUntilArrival = Math.round((parseISODate(res.checkInDate) - parseISODate(today)) / 86_400_000);

    // Policy: non-refundable rates forfeit the full booking; flexible rates
    // are free until the day before arrival, then forfeit one night.
    let feeKobo = 0;
    if (!refundable) feeKobo = res.totalKobo;
    else if (daysUntilArrival <= 0) feeKobo = res.nightlyRateKobo;
    feeKobo = Math.min(feeKobo, res.totalKobo);
    const refundDueKobo = Math.max(0, res.paidKobo - feeKobo);

    const now = Date.now();
    await ctx.db.patch(reservationId, {
      status: "cancelled",
      cancelledAt: now,
      cancellationReason: `${reason} · fee ${feeKobo} kobo · refund due ${refundDueKobo} kobo`,
    });

    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.reservationCancelled,
      source: "stays",
      entityId: res.code,
      payload: { reservationId, guestId: res.guestId, reason, feeKobo, refundDueKobo, paidKobo: res.paidKobo },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "reservation.cancelled",
      entity: "reservation",
      entityId: res.code,
      severity: feeKobo > 0 ? "warning" : "info",
      details: { reason, feeKobo, refundDueKobo, refundable, daysUntilArrival },
      at: now,
    });

    return { ok: true as const, feeKobo, refundDueKobo };
  },
});

// ── No-show ──────────────────────────────────────────────────────────────
/** Apply no-show status + one-night billing. Shared with the night-audit sweep. */
export async function applyNoShow(
  ctx: MutationCtx,
  auth: Pick<AuthContext, "propertyId" | "name">,
  res: Doc<"reservations">,
  businessDate: string,
): Promise<boolean> {
  if (res.status !== "pending" && res.status !== "confirmed") return false;
  const folio = await ensureForReservation(ctx, res);
  await postChargeInternal(ctx, {
    propertyId: res.propertyId,
    folioId: folio._id,
    folioCode: folio.code,
    category: "room",
    description: `No-show charge · ${businessDate}`,
    qty: 1,
    unitKobo: res.nightlyRateKobo,
    department: "rooms",
    date: businessDate,
    by: auth.name,
  });
  await ctx.db.patch(res._id, { status: "no_show" });
  await emit(ctx, {
    propertyId: res.propertyId,
    type: EVENTS.reservationNoShow,
    source: "stays",
    entityId: res.code,
    payload: { reservationId: res._id, guestId: res.guestId, businessDate, chargedKobo: res.nightlyRateKobo },
    by: auth.name,
  });
  await ctx.db.insert("auditLogs", {
    propertyId: res.propertyId,
    actorName: auth.name,
    action: "reservation.no_show",
    entity: "reservation",
    entityId: res.code,
    severity: "warning",
    details: { businessDate, chargedKobo: res.nightlyRateKobo },
    at: Date.now(),
  });
  return true;
}

export const markNoShow = mutation({
  args: { token: v.string(), reservationId: v.id("reservations") },
  handler: async (ctx, { token, reservationId }) => {
    const auth = await requireAuth(ctx, token, "lodging.frontdesk.no_show");
    const property = await requireProperty(ctx);
    const res = await loadReservation(ctx, auth, reservationId);
    if (res.status !== "pending" && res.status !== "confirmed") {
      throw new ConvexError(`Cannot mark no-show from status "${res.status}"`);
    }
    const today = todayPropertyLocal(property.timezone);
    if (res.checkInDate > today) throw new ConvexError("Arrival date has not been reached");
    const applied = await applyNoShow(ctx, auth, res, today);
    return { ok: true as const, charged: applied };
  },
});

// ── Modification ─────────────────────────────────────────────────────────
export const updateDates = mutation({
  args: { token: v.string(), reservationId: v.id("reservations"), checkInDate: v.string(), checkOutDate: v.string() },
  handler: async (ctx, { token, reservationId, checkInDate, checkOutDate }) => {
    const auth = await requireAuth(ctx, token, "lodging.reservations.modify");
    const res = await loadReservation(ctx, auth, reservationId);
    if (res.status !== "pending" && res.status !== "confirmed") {
      throw new ConvexError(`Cannot modify dates from status "${res.status}"`);
    }
    const nights = Math.round((parseISODate(checkOutDate) - parseISODate(checkInDate)) / 86_400_000);
    if (nights < 1) throw new ConvexError("Check-out must be after check-in");
    const totalKobo = nights * res.nightlyRateKobo;
    await ctx.db.patch(reservationId, { checkInDate, checkOutDate, nights, totalKobo });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "reservation.modified",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { from: { checkInDate: res.checkInDate, checkOutDate: res.checkOutDate }, to: { checkInDate, checkOutDate }, totalKobo },
      at: Date.now(),
    });
    return { ok: true as const, nights, totalKobo };
  },
});
