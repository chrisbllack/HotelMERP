// Cross-module reporting — the console's read side.
//
// Every figure is derived live from the operational tables (reservations,
// folioLines, rooms, posOrders, glEntries, hkTasks …), so the dashboard,
// front desk and yield views can never drift from what the engines wrote.

import { v } from "convex/values";
import { query, type QueryCtx } from "./_generated/server";
import type { Doc } from "./_generated/dataModel";
import { addDaysISO, localDayRange, todayPropertyLocal, requireProperty } from "./lib";
import { requireAuth } from "./authz";

const ACTIVE_STAY = new Set(["pending", "confirmed", "checked_in"]);

interface DayKpi {
  date: string;
  roomsSold: number;
  roomsAvailable: number;
  occupancyPct: number;
  roomRevenueKobo: number;
  adrKobo: number;
  revparKobo: number;
}

async function dayKpi(ctx: QueryCtx, property: Doc<"properties">, date: string, roomsTotal: number, ooo: number): Promise<DayKpi> {
  const reservations = await ctx.db.query("reservations").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect();
  const sold = reservations.filter((r) => ACTIVE_STAY.has(r.status) && r.checkInDate <= date && date < r.checkOutDate).length;
  const [start, end] = localDayRange(date, property.timezone);
  const lines = (await ctx.db.query("folioLines").withIndex("by_property", (q) => q.eq("propertyId", property._id)).collect()).filter(
    (l) => l.category === "room" && l.amountKobo > 0 && l.postedAt >= start && l.postedAt < end,
  );
  const roomRevenueKobo = lines.reduce((s, l) => s + l.amountKobo, 0);
  const roomsAvailable = Math.max(0, roomsTotal - ooo);
  return {
    date,
    roomsSold: sold,
    roomsAvailable,
    occupancyPct: roomsAvailable > 0 ? Math.round((sold / roomsAvailable) * 10_000) / 100 : 0,
    roomRevenueKobo,
    adrKobo: sold > 0 ? Math.round(roomRevenueKobo / sold) : 0,
    revparKobo: roomsAvailable > 0 ? Math.round(roomRevenueKobo / roomsAvailable) : 0,
  };
}

// ── Dashboard ────────────────────────────────────────────────────────────
export const dashboard = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "dashboard.view");
    const property = await requireProperty(ctx);
    const today = todayPropertyLocal(property.timezone);

    const [rooms, reservations, hkTasks, tickets, items, posOrders, folios, payments, audits] = await Promise.all([
      ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("reservations").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("hkTasks").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("maintenanceTickets").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("inventoryItems").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("posOrders").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("folios").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("payments").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("auditLogs").withIndex("by_property_at", (q) => q.eq("propertyId", auth.propertyId)).order("desc").take(15),
    ]);

    const ooo = rooms.filter((r) => r.status === "out_of_order").length;
    const kpi = await dayKpi(ctx, property, today, rooms.length, ooo);

    const arrivals = reservations.filter((r) => r.checkInDate === today && (r.status === "confirmed" || r.status === "pending"));
    const departures = reservations.filter((r) => r.checkOutDate === today && r.status === "checked_in");
    const inHouse = reservations.filter((r) => r.status === "checked_in");

    const openFolios = folios.filter((f) => f.status === "open");
    const outstandingKobo = openFolios.reduce((s, f) => s + Math.max(0, f.totalKobo - f.paidKobo), 0);
    const [monthStart] = localDayRange(today.slice(0, 8) + "01", property.timezone);
    const mtdPayments = payments.filter((p) => p.status === "success" && (p.paidAt ?? p.createdAt) >= monthStart);

    const pendingHk = hkTasks.filter((t) => t.status === "pending" || t.status === "in_progress");
    const openTickets = tickets.filter((t) => t.status === "open" || t.status === "in_progress");
    const lowStock = items.filter((i) => i.qtyOnHand <= i.reorderLevel);

    const dayStart = localDayRange(today, property.timezone)[0];
    const todayPos = posOrders.filter((o) => o.status === "paid" && (o.closedAt ?? 0) >= dayStart);

    const lastClosedRow = (
      await ctx.db
        .query("settings")
        .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", "nightaudit.last_closed"))
        .collect()
    )[0];
    const lastAudit = (
      await ctx.db
        .query("nightAudits")
        .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
        .order("desc")
        .take(1)
    )[0];

    return {
      property,
      date: today,
      rooms: {
        total: rooms.length,
        outOfOrder: ooo,
        sellable: rooms.length - ooo,
        occupied: rooms.filter((r) => r.status.startsWith("occupied")).length,
        dirty: rooms.filter((r) => r.hkStatus === "dirty").length,
        outOfOrderRooms: rooms.filter((r) => r.status === "out_of_order"),
      },
      kpi,
      arrivals: { count: arrivals.length, rows: arrivals.slice(0, 20) },
      departures: { count: departures.length, rows: departures.slice(0, 20) },
      inHouse: { count: inHouse.length },
      housekeeping: { pending: pendingHk.length, dirtyRooms: hkTasks.filter((t) => t.status === "pending").length },
      maintenance: { open: openTickets.length },
      inventory: { low: lowStock.length, lowRows: lowStock.slice(0, 10) },
      finance: {
        outstandingKobo,
        openFolios: openFolios.length,
        collectionsTodayKobo: mtdPayments.filter((p) => (p.paidAt ?? 0) >= dayStart).reduce((s, p) => s + p.amountKobo, 0),
        collectionsMtdKobo: mtdPayments.reduce((s, p) => s + p.amountKobo, 0),
      },
      fnb: {
        ordersToday: todayPos.length,
        revenueTodayKobo: todayPos.reduce((s, o) => s + o.subtotalKobo, 0),
      },
      nightAudit: { lastClosedDate: lastClosedRow?.value ?? null, lastRun: lastAudit ?? null, open: (lastClosedRow?.value ?? "0000-00-00") < today },
      recentAudit: audits,
    };
  },
});

// ── Front desk workspace ─────────────────────────────────────────────────
export const frontDesk = query({
  args: { token: v.string(), date: v.optional(v.string()) },
  handler: async (ctx, { token, date }) => {
    const auth = await requireAuth(ctx, token, "lodging.reservations.view");
    const property = await requireProperty(ctx);
    const day = date ?? todayPropertyLocal(property.timezone);

    const reservations = await ctx.db.query("reservations").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const guests = await ctx.db.query("guests").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const roomTypes = await ctx.db.query("roomTypes").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const rooms = await ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const assignments = await ctx.db.query("roomAssignments").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const folios = await ctx.db.query("folios").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();

    const guestById = new Map(guests.map((g) => [g._id, g]));
    const typeById = new Map(roomTypes.map((t) => [t._id, t]));
    const roomById = new Map(rooms.map((r) => [r._id, r]));
    const folioByRes = new Map(folios.map((f) => [f.reservationId, f]));
    const roomForRes = new Map(assignments.map((a) => [a.reservationId, roomById.get(a.roomId) ?? null]));

    const decorate = (r: Doc<"reservations">) => {
      const g = guestById.get(r.guestId);
      const folio = folioByRes.get(r._id);
      return {
        ...r,
        guestName: g ? `${g.firstName} ${g.lastName}` : "?",
        guestVip: g?.vip ?? false,
        guestBlacklisted: g?.blacklisted ?? false,
        roomTypeName: typeById.get(r.roomTypeId)?.name ?? "?",
        room: roomForRes.get(r._id) ?? null,
        folio: folio ? { ...folio, balanceKobo: folio.totalKobo - folio.paidKobo } : null,
      };
    };

    return {
      date: day,
      arrivals: reservations.filter((r) => r.checkInDate === day && (r.status === "confirmed" || r.status === "pending")).map(decorate),
      inHouse: reservations.filter((r) => r.status === "checked_in").map(decorate),
      departures: reservations.filter((r) => r.checkOutDate === day && r.status === "checked_in").map(decorate),
      dueOut: reservations.filter((r) => r.checkOutDate < day && r.status === "checked_in").map(decorate),
      waitlist: [] as unknown[],
      roomTypes,
      rooms,
    };
  },
});

// ── Occupancy calendar (yield & distribution views) ──────────────────────
export const occupancyCalendar = query({
  args: { token: v.string(), from: v.string(), to: v.string() },
  handler: async (ctx, { token, from, to }) => {
    const auth = await requireAuth(ctx, token, "lodging.reservations.view");
    const property = await requireProperty(ctx);
    const rooms = await ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const ooo = rooms.filter((r) => r.status === "out_of_order").length;
    const out: DayKpi[] = [];
    let cursor = from;
    let guard = 0;
    while (cursor < to && guard < 370) {
      out.push(await dayKpi(ctx, property, cursor, rooms.length, ooo));
      cursor = addDaysISO(cursor, 1);
      guard++;
    }
    return out;
  },
});

// ── Revenue by department (from the ledger — always reconciled) ──────────
export const revenueByDepartment = query({
  args: { token: v.string(), from: v.string(), to: v.string() },
  handler: async (ctx, { token, from, to }) => {
    const auth = await requireAuth(ctx, token, "fin.reports.view");
    const accounts = await ctx.db.query("glAccounts").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const revenueCodes = new Set(accounts.filter((a) => a.type === "revenue").map((a) => a.code));
    const entries = await ctx.db
      .query("glEntries")
      .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).gte("date", from).lte("date", to))
      .collect();
    const byCode = new Map<string, number>();
    for (const e of entries) {
      if (!revenueCodes.has(e.accountCode)) continue;
      byCode.set(e.accountCode, (byCode.get(e.accountCode) ?? 0) + (e.creditKobo - e.debitKobo));
    }
    return accounts
      .filter((a) => revenueCodes.has(a.code))
      .map((a) => ({ accountCode: a.code, name: a.name, amountKobo: byCode.get(a.code) ?? 0 }))
      .sort((a, b) => b.amountKobo - a.amountKobo);
  },
});
