// M-LOD console operations over live website bookings.
// These functions operate on real bookings created through the booking site
// (source = "website") — the console becomes the operational surface for the
// same data the website writes.

import { v } from "convex/values";
import { ConvexError } from "convex/values";
import { internalQuery, mutation, query } from "./_generated/server";
import type { Id } from "./_generated/dataModel";
import { computeAvailability, todayPropertyLocal } from "./lib";
import type { AvailabilityRow } from "./lib";
import { emit, EVENTS } from "./events";

const SEED_PK = "grand-palm-lagos";

// ── Console availability search ──────────────────────────────────────────
export const availability = query({
  args: { checkIn: v.string(), checkOut: v.string() },
  handler: async (ctx, { checkIn, checkOut }): Promise<AvailabilityRow[]> => {
    const property = await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique();
    if (!property) return [];
    const [roomTypes, rooms, reservations, blocks, ratePlans] = await Promise.all([
      ctx.db.query("roomTypes").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
      ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
      ctx.db.query("reservations").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
      ctx.db.query("roomBlocks").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
      ctx.db.query("ratePlans").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
    ]);
    return computeAvailability(roomTypes, rooms, reservations, blocks, ratePlans, checkIn, checkOut);
  },
});

// ── Confirm a website booking (M-LOD front desk action) ──────────────────
export const confirmWebsiteBooking = mutation({
  args: { reservationId: v.id("reservations") },
  handler: async (ctx, { reservationId }): Promise<{ ok: boolean }> => {
    const res = await ctx.db.get(reservationId);
    if (!res) throw new ConvexError("Reservation not found");
    if (res.source !== "website") throw new ConvexError("Not a website booking");
    if (res.status !== "pending") throw new ConvexError(`Cannot confirm from status "${res.status}"`);
    await ctx.db.patch(reservationId, { status: "confirmed" });
    await ctx.db.insert("auditLogs", {
      propertyId: res.propertyId,
      actorName: "front desk",
      action: "reservation.confirmed",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { source: "website" },
      at: Date.now(),
    });
    return { ok: true as const };
  },
});

// ── Assign a room to a website booking ───────────────────────────────────
export const assignRoom = mutation({
  args: { reservationId: v.id("reservations"), roomId: v.id("rooms") },
  handler: async (ctx, { reservationId, roomId }): Promise<{ ok: boolean }> => {
    const res = await ctx.db.get(reservationId);
    if (!res) throw new ConvexError("Reservation not found");
    const room = await ctx.db.get(roomId);
    if (!room || room.propertyId !== res.propertyId) throw new ConvexError("Room not in this property");
    if (room.status !== "vacant_clean" && room.status !== "vacant_dirty") {
      throw new ConvexError(`Room ${room.number} is not vacant`);
    }
    if (room.roomTypeId !== res.roomTypeId) throw new ConvexError("Room type does not match the booked category");

    const existing = await ctx.db
      .query("roomAssignments")
      .withIndex("by_reservation", (q) => q.eq("reservationId", reservationId))
      .unique();
    if (existing) await ctx.db.delete(existing._id);

    await ctx.db.insert("roomAssignments", {
      propertyId: res.propertyId,
      reservationId,
      roomId,
      assignedAt: Date.now(),
      assignedBy: "front desk",
    });
    await ctx.db.patch(roomId, { status: "reserved" });
    await ctx.db.insert("auditLogs", {
      propertyId: res.propertyId,
      actorName: "front desk",
      action: "room.assigned",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { room: room.number },
      at: Date.now(),
    });
    return { ok: true as const };
  },
});

// ── Record a manual (front-desk) payment for a website booking ───────────
export const recordManualPayment = mutation({
  args: { reservationId: v.id("reservations"), amountKobo: v.number(), method: v.string() },
  handler: async (ctx, { reservationId, amountKobo, method }): Promise<{ ok: boolean; paidKobo: number }> => {
    if (amountKobo <= 0) throw new ConvexError("Amount must be positive");
    const res = await ctx.db.get(reservationId);
    if (!res) throw new ConvexError("Reservation not found");
    const now = Date.now();
    const reference = `MAN-${res.code}-${now}`;
    const paidKobo = Math.min(res.totalKobo, res.paidKobo + amountKobo);
    await ctx.db.patch(reservationId, {
      paidKobo,
      paymentStatus: paidKobo >= res.totalKobo ? "paid" : "partial",
      status: res.status === "pending" ? "confirmed" : res.status,
    });
    await ctx.db.insert("payments", {
      propertyId: res.propertyId,
      reservationId,
      method,
      amountKobo,
      currency: "NGN",
      reference,
      gateway: "manual",
      status: "success",
      channelResponse: "Recorded at front desk",
      createdAt: now,
      paidAt: now,
    });
    // GL posting flows through the payment.captured contract (single rule).
    const property = await ctx.db.get(res.propertyId);
    await emit(ctx, {
      propertyId: res.propertyId,
      type: EVENTS.paymentCaptured,
      source: "frontdesk",
      entityId: res.code,
      payload: {
        propertyId: res.propertyId,
        date: todayPropertyLocal((property as { timezone: string } | null)?.timezone ?? "Africa/Lagos"),
        method,
        amountKobo,
        reference,
        description: `Front-desk payment ${res.code}`,
      },
      by: "front desk",
    });
    await ctx.db.insert("auditLogs", {
      propertyId: res.propertyId,
      actorName: "front desk",
      action: "folio.payment_posted",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { amountKobo, method },
      at: now,
    });
    return { ok: true as const, paidKobo };
  },
});

// ── Read helpers for the console panel ───────────────────────────────────
export const roomsOfType = internalQuery({
  args: { roomTypeId: v.id("roomTypes") },
  handler: async (ctx, { roomTypeId }) => {
    return ctx.db.query("rooms").withIndex("by_room_type", (q) => q.eq("roomTypeId", roomTypeId)).collect();
  },
});

export const countByType = query({
  args: {},
  handler: async (ctx): Promise<Array<{ roomTypeId: Id<"roomTypes">; code: string; name: string; rooms: number }>> => {
    const property = await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique();
    if (!property) return [] as Array<{ roomTypeId: Id<"roomTypes">; code: string; name: string; rooms: number }>;
    const [types, rooms] = await Promise.all([
      ctx.db.query("roomTypes").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
      ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect(),
    ]);
    return types.map((t) => ({
      roomTypeId: t._id,
      code: t.code,
      name: t.name,
      rooms: rooms.filter((r) => r.roomTypeId === t._id && r.status !== "out_of_order").length,
    }));
  },
});
