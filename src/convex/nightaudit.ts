// M-BUS — night audit / business-day close.
//
// The nightly close is the heartbeat of the property:
//   1. post every consumed room night to the guest folio (revenue + GL, dated
//      on the night itself — idempotent, so a re-run never double-bills),
//   2. sweep arrivals that never showed (no-show billing),
//   3. compute the day's statistics (occupancy, ADR, RevPAR, revenue mix),
//   4. seal the business date so it can never be re-closed.
// Dates must close sequentially — no gaps, no rewrites.

import { v, ConvexError } from "convex/values";
import { mutation, query, type MutationCtx, type QueryCtx } from "./_generated/server";
import type { Doc } from "./_generated/dataModel";
import { addDaysISO, localDayRange, todayPropertyLocal, requireProperty } from "./lib";
import { ensureForReservation, postRoomNight } from "./folio";
import { applyNoShow } from "./stays";
import { emit, EVENTS } from "./events";
import { requireAuth, type AuthContext } from "./authz";

const LAST_CLOSED_KEY = "nightaudit.last_closed";

export interface NightStats {
  businessDate: string;
  occupancyPct: number;
  adrKobo: number;
  revparKobo: number;
  roomRevenueKobo: number;
  fnbRevenueKobo: number;
  otherRevenueKobo: number;
  roomsSold: number;
  roomsAvailable: number;
  inHouse: number;
  arrivals: number;
  departures: number;
  noShows: number;
  transactionsCount: number;
}

const FNB_LIKE_CATEGORIES = new Set(["minibar", "laundry", "spa", "parking", "other"]);

/** Compute the close statistics for a business date (read-only). */
export async function computeNightStats(
  ctx: QueryCtx | MutationCtx,
  property: Doc<"properties">,
  businessDate: string,
): Promise<NightStats> {
  const [start, end] = localDayRange(businessDate, property.timezone);
  const rooms = await ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect();
  const reservations = await ctx.db
    .query("reservations")
    .withIndex("by_property", (q) => q.eq("propertyId", property._id))
    .collect();

  const roomsAvailable = rooms.filter((r) => r.status !== "out_of_order").length;
  const inHouseRows = reservations.filter((r) => r.status === "checked_in" && r.checkInDate <= businessDate && businessDate < r.checkOutDate);
  const arrivals = reservations.filter((r) => r.checkInDate === businessDate && (r.status === "confirmed" || r.status === "checked_in")).length;
  const departures = reservations.filter((r) => r.checkOutDate === businessDate && r.status === "checked_out").length;
  const noShows = reservations.filter((r) => r.status === "no_show").length;

  const lines = (await ctx.db.query("folioLines").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect()).filter(
    (l) => l.postedAt >= start && l.postedAt < end,
  );
  const roomRevenueKobo = lines.filter((l) => l.category === "room" && l.amountKobo > 0).reduce((s, l) => s + l.amountKobo, 0);
  const otherRevenueKobo = lines.filter((l) => FNB_LIKE_CATEGORIES.has(l.category) && l.amountKobo > 0).reduce((s, l) => s + l.amountKobo, 0);

  const orders = (await ctx.db.query("posOrders").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect()).filter(
    (o) => o.status === "paid" && o.closedAt !== undefined && o.closedAt >= start && o.closedAt < end,
  );
  const fnbRevenueKobo = orders.reduce((s, o) => s + o.subtotalKobo, 0);

  const payments = (await ctx.db.query("payments").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect()).filter(
    (p) => p.createdAt >= start && p.createdAt < end && p.status === "success",
  );

  const roomsSold = inHouseRows.length;
  return {
    businessDate,
    occupancyPct: roomsAvailable > 0 ? Math.round((roomsSold / roomsAvailable) * 10_000) / 100 : 0,
    adrKobo: roomsSold > 0 ? Math.round(roomRevenueKobo / roomsSold) : 0,
    revparKobo: roomsAvailable > 0 ? Math.round(roomRevenueKobo / roomsAvailable) : 0,
    roomRevenueKobo,
    fnbRevenueKobo,
    otherRevenueKobo,
    roomsSold,
    roomsAvailable,
    inHouse: inHouseRows.length,
    arrivals,
    departures,
    noShows,
    transactionsCount: lines.length + payments.length,
  };
}

// ── Run the close ────────────────────────────────────────────────────────
export const runNight = mutation({
  args: { token: v.string(), businessDate: v.optional(v.string()), notes: v.optional(v.string()) },
  handler: async (ctx, { token, businessDate, notes }) => {
    const auth: AuthContext = await requireAuth(ctx, token, "lodging.night_audit.run");
    const property = await requireProperty(ctx);
    const today = todayPropertyLocal(property.timezone);
    const date = businessDate ?? today;
    if (date > today) throw new ConvexError(`Cannot close a future business date (${date})`);

    // Sequential close: yesterday-or-later only, never twice.
    const lastClosedRow = await ctx.db
      .query("settings")
      .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", LAST_CLOSED_KEY))
      .unique();
    if (lastClosedRow) {
      if (date <= lastClosedRow.value) {
        throw new ConvexError(`Business date ${date} is already closed (closed through ${lastClosedRow.value})`);
      }
      const next = addDaysISO(lastClosedRow.value, 1);
      if (date !== next) {
        throw new ConvexError(`Business dates close sequentially — the next open date is ${next}`);
      }
    }
    const already = await ctx.db
      .query("nightAudits")
      .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).eq("businessDate", date))
      .unique();
    if (already) throw new ConvexError(`Night audit for ${date} already exists`);

    // 1. Post room nights for every in-house stay (idempotent per night).
    const reservations = await ctx.db
      .query("reservations")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect();
    const inHouse = reservations.filter((r) => r.status === "checked_in" && r.checkInDate <= date && date < r.checkOutDate);
    let nightsPosted = 0;
    for (const res of inHouse) {
      const folio = await ensureForReservation(ctx, res);
      if (await postRoomNight(ctx, res, folio, date, auth.name)) nightsPosted++;
    }

    // 2. No-show sweep: arrivals scheduled for this date that never checked in.
    //    Only meaningful once the arrival day is effectively over — a same-day
    //    close run before late evening must not sweep guests still expected.
    const currentHour = Number(
      new Intl.DateTimeFormat("en-US", { timeZone: property.timezone, hour: "2-digit", hour12: false }).format(new Date()),
    );
    const dayOver = date < today || currentHour >= 23;
    const arrivals = dayOver
      ? reservations.filter((r) => r.checkInDate === date && (r.status === "pending" || r.status === "confirmed"))
      : [];
    let noShowsMarked = 0;
    for (const res of arrivals) {
      if (await applyNoShow(ctx, auth, res, date)) noShowsMarked++;
    }

    // 3. Statistics over the sealed day.
    const stats = await computeNightStats(ctx, property, date);

    // 4. Seal.
    const now = Date.now();
    await ctx.db.insert("nightAudits", {
      propertyId: auth.propertyId,
      businessDate: date,
      runAt: now,
      runBy: auth.name,
      status: "completed",
      occupancyPct: stats.occupancyPct,
      adrKobo: stats.adrKobo,
      revparKobo: stats.revparKobo,
      roomRevenueKobo: stats.roomRevenueKobo,
      fnbRevenueKobo: stats.fnbRevenueKobo,
      otherRevenueKobo: stats.otherRevenueKobo,
      transactionsCount: stats.transactionsCount,
      notes: notes ?? (nightsPosted > 0 || noShowsMarked > 0 ? `${nightsPosted} room night(s) posted, ${noShowsMarked} no-show(s) swept` : undefined),
    });
    if (lastClosedRow) await ctx.db.patch(lastClosedRow._id, { value: date, updatedBy: auth.name, updatedAt: now });
    else
      await ctx.db.insert("settings", {
        propertyId: auth.propertyId,
        key: LAST_CLOSED_KEY,
        value: date,
        updatedBy: auth.name,
        updatedAt: now,
      });

    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.nightAuditClosed,
      source: "nightaudit",
      entityId: date,
      payload: { ...stats, nightsPosted, noShowsMarked },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "night_audit.closed",
      entity: "businessDate",
      entityId: date,
      severity: "info",
      details: { nightsPosted, noShowsMarked, ...stats },
      at: now,
    });

    return { ok: true as const, stats, nightsPosted, noShowsMarked };
  },
});

// ── Reads ────────────────────────────────────────────────────────────────
export const status = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "dashboard.view");
    const property = await requireProperty(ctx);
    const lastClosedRow = await ctx.db
      .query("settings")
      .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", LAST_CLOSED_KEY))
      .unique();
    const lastRun = (
      await ctx.db
        .query("nightAudits")
        .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
        .order("desc")
        .take(1)
    )[0];
    const today = todayPropertyLocal(property.timezone);
    return {
      lastClosedDate: lastClosedRow?.value ?? null,
      nextOpenDate: lastClosedRow ? addDaysISO(lastClosedRow.value, 1) : null,
      isTodayOpen: (lastClosedRow?.value ?? "0000-00-00") < today,
      today,
      lastRun,
    };
  },
});

export const preview = query({
  args: { token: v.string(), businessDate: v.optional(v.string()) },
  handler: async (ctx, { token, businessDate }) => {
    const auth = await requireAuth(ctx, token, "lodging.night_audit.run");
    const property = await requireProperty(ctx);
    const date = businessDate ?? todayPropertyLocal(property.timezone);
    const reservations = await ctx.db
      .query("reservations")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect();
    const pendingNights = reservations.filter(
      (r) => r.status === "checked_in" && r.checkInDate <= date && date < r.checkOutDate,
    ).length;
    const pendingNoShows = reservations.filter((r) => r.checkInDate === date && (r.status === "pending" || r.status === "confirmed")).length;
    const stats = await computeNightStats(ctx, property, date);
    return { date, pendingNights, pendingNoShows, stats };
  },
});

export const history = query({
  args: { token: v.string(), limit: v.optional(v.number()) },
  handler: async (ctx, { token, limit }) => {
    const auth = await requireAuth(ctx, token, "fin.reports.view");
    return ctx.db
      .query("nightAudits")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .order("desc")
      .take(Math.min(limit ?? 30, 120));
  },
});
