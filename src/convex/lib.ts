// INFINEON backend shared domain library.
//
// Money: integer kobo (1 NGN = 100 kobo) — exact arithmetic, never floats.
// Dates: "YYYY-MM-DD" strings in property-local time (Africa/Lagos).

import { v } from "convex/values";
import { ConvexError } from "convex/values";
import { internalQuery, internalMutation, type MutationCtx, type QueryCtx } from "./_generated/server";
import { internal } from "./_generated/api";
import type { Doc, Id } from "./_generated/dataModel";

export const NGN = "NGN";
export const VAT_RATE = 0.075;
export const SERVICE_CHARGE_RATE = 0.1;
export const DEFAULT_DEPOSIT_PCT = 0.2;

// ── Money helpers (integer kobo) ─────────────────────────────────────────
export const toKobo = (naira: number): number => Math.round(naira * 100);
export const mulKobo = (qty: number, unitKobo: number): number => Math.round(qty * unitKobo);
export const pctKobo = (amountKobo: number, rate: number): number => Math.round(amountKobo * rate);

// ── Date helpers ("YYYY-MM-DD" property-local) ───────────────────────────
export function parseISODate(iso: string): number {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) throw new ConvexError(`Invalid date "${iso}" — expected YYYY-MM-DD`);
  const t = Date.parse(`${iso}T00:00:00Z`);
  if (Number.isNaN(t)) throw new ConvexError(`Invalid calendar date "${iso}"`);
  return t;
}

export function diffNights(checkIn: string, checkOut: string): number {
  return Math.round((parseISODate(checkOut) - parseISODate(checkIn)) / 86_400_000);
}

export function addDaysISO(iso: string, days: number): string {
  return new Date(parseISODate(iso) + days * 86_400_000).toISOString().slice(0, 10);
}

export function todayPropertyLocal(timezone = "Africa/Lagos"): string {
  return new Date().toLocaleDateString("en-CA", { timeZone: timezone }); // YYYY-MM-DD
}

/** Half-open date intervals: [aIn, aOut) ∩ [bIn, bOut) ≠ ∅ */
export function overlaps(aIn: string, aOut: string, bIn: string, bOut: string): boolean {
  return aIn < bOut && bIn < aOut;
}

// ── Deterministic counters (OCC makes single-doc read/write atomic) ──────
export const counterArgs = v.object({ propertyId: v.id("properties"), key: v.string() });

export const nextSeq = internalQuery({
  args: counterArgs,
  handler: async (ctx, { propertyId, key }): Promise<number> => {
    const row = await ctx.db
      .query("counters")
      .withIndex("by_property_key", (q) => q.eq("propertyId", propertyId).eq("key", key))
      .unique();
    return (row?.value ?? 0) + 1;
  },
});

export const bumpSeq = internalMutation({
  args: counterArgs,
  handler: async (ctx, { propertyId, key }): Promise<number> => {
    const row = await ctx.db
      .query("counters")
      .withIndex("by_property_key", (q) => q.eq("propertyId", propertyId).eq("key", key))
      .unique();
    const value = (row?.value ?? 0) + 1;
    if (row) await ctx.db.patch(row._id, { value });
    else await ctx.db.insert("counters", { propertyId, key, value });
    return value;
  },
});

// ── Availability core ────────────────────────────────────────────────────
// Bookable unit = sellable rooms of a type minus overlapping bookings minus
// date-bounded room blocks, for every night of the requested stay.

export interface AvailabilityRow {
  roomTypeId: Id<"roomTypes">;
  code: string;
  name: string;
  description: string;
  maxOccupancy: number;
  bedType: string;
  amenities: string[];
  rateKobo: number;
  ratePlanId: Id<"ratePlans"> | null;
  totalRooms: number;
  booked: number;
  blocked: number;
  available: number;
}

export function computeAvailability(
  roomTypes: Doc<"roomTypes">[],
  rooms: Doc<"rooms">[],
  reservations: Pick<Doc<"reservations">, "roomTypeId" | "checkInDate" | "checkOutDate" | "status">[],
  blocks: Doc<"roomBlocks">[],
  ratePlans: Doc<"ratePlans">[],
  checkIn: string,
  checkOut: string,
): AvailabilityRow[] {
  const active = reservations.filter((r) => r.status === "pending" || r.status === "confirmed" || r.status === "checked_in");
  return roomTypes
    .slice()
    .sort((a, b) => a.basePriceKobo - b.basePriceKobo)
    .map((rt) => {
      const totalRooms = rooms.filter((r) => r.roomTypeId === rt._id && r.status !== "out_of_order").length;
      const booked = active.filter((r) => r.roomTypeId === rt._id && overlaps(checkIn, checkOut, r.checkInDate, r.checkOutDate)).length;
      const blocked = blocks
        .filter((b) => b.roomTypeId === rt._id && overlaps(checkIn, checkOut, b.startDate, b.endDate))
        .reduce((acc, b) => acc + b.quantity, 0);
      const available = Math.max(0, totalRooms - booked - blocked);
      const plans = ratePlans.filter((p) => p.roomTypeId === rt._id && p.active).sort((a, b) => a.priceKobo - b.priceKobo);
      const plan = plans[0] ?? null;
      return {
        roomTypeId: rt._id,
        code: rt.code,
        name: rt.name,
        description: rt.description,
        maxOccupancy: rt.maxOccupancy,
        bedType: rt.bedType,
        amenities: rt.amenities,
        rateKobo: plan ? plan.priceKobo : rt.basePriceKobo,
        ratePlanId: plan ? plan._id : null,
        totalRooms,
        booked,
        blocked,
        available,
      };
    });
}

// ── Seed: Grand Palm Hotel Lagos (mirrors src/data/demo.ts) ──────────────

const SEED_PK = "grand-palm-lagos";

const seedRoomTypes = [
  { code: "STD", name: "Standard Room", description: "Garden-view room with king bed, work desk and rain shower.", basePriceKobo: 2_500_000, maxOccupancy: 2, bedType: "King", sizeSqm: 28, amenities: ["WiFi", "A/C", "Smart TV", "Rain shower", "Work desk"] },
  { code: "DLX", name: "Deluxe Room", description: "City-view room with lounge corner and premium amenities.", basePriceKobo: 4_000_000, maxOccupancy: 2, bedType: "King", sizeSqm: 36, amenities: ["WiFi", "A/C", "Smart TV", "Lounge corner", "Mini bar", "Bathtub"] },
  { code: "EXE", name: "Executive Suite", description: "Separate living room, dining nook and executive lounge access.", basePriceKobo: 6_500_000, maxOccupancy: 2, bedType: "King", sizeSqm: 55, amenities: ["WiFi", "A/C", "Living room", "Executive lounge", "Mini bar", "Nespresso"] },
  { code: "PRM", name: "Presidential Suite", description: "Two-bedroom suite with private lounge, butler service and skyline terrace.", basePriceKobo: 15_000_000, maxOccupancy: 4, bedType: "King + Twin", sizeSqm: 120, amenities: ["WiFi", "A/C", "Private lounge", "Butler service", "Terrace", "Dining nook"] },
];

const seedRatePlans = [
  { rt: "STD", code: "STD-BB", name: "Standard · Bed & Breakfast", priceKobo: 2_500_000, includesBreakfast: true, refundable: true, minNights: 1 },
  { rt: "STD", code: "STD-RO", name: "Standard · Room Only", priceKobo: 2_250_000, includesBreakfast: false, refundable: true, minNights: 1 },
  { rt: "DLX", code: "DLX-BB", name: "Deluxe · Bed & Breakfast", priceKobo: 4_000_000, includesBreakfast: true, refundable: true, minNights: 1 },
  { rt: "DLX", code: "DLX-NR", name: "Deluxe · Non-Refundable Saver", priceKobo: 3_600_000, includesBreakfast: true, refundable: false, minNights: 2 },
  { rt: "EXE", code: "EXE-BB", name: "Executive Suite · B&B", priceKobo: 6_500_000, includesBreakfast: true, refundable: true, minNights: 1 },
  { rt: "PRM", code: "PRM-FB", name: "Presidential · Full Board", priceKobo: 15_000_000, includesBreakfast: true, refundable: true, minNights: 1 },
];

// status per room number 1..50, mirroring the demo dataset snapshot
const roomStatuses: Record<number, { status: string; hk: string }> = Object.fromEntries(
  Array.from({ length: 50 }, (_, i) => {
    const n = i + 1;
    const s = ([1, 2, 5, 10, 15, 20, 41, 42].includes(n) && { status: "occupied_clean", hk: "clean" }) ||
      ([3, 6, 7, 8, 30, 31, 32, 33, 34].includes(n) && { status: "vacant_clean", hk: "clean" }) ||
      ([35, 36, 37, 38, 39, 43].includes(n) && { status: "reserved", hk: "clean" }) ||
      ([45, 46].includes(n) && { status: "out_of_order", hk: "dirty" }) ||
      ([48, 49, 50].includes(n) && { status: "occupied_dirty", hk: "dirty" }) ||
      ({ status: "vacant_dirty", hk: "dirty" });
    return [n, s];
  }),
);

const typeOf = (n: number) => (n <= 40 ? "STD" : n <= 47 ? "DLX" : n <= 49 ? "EXE" : "PRM");
const levelOf = (n: number) => (n <= 10 ? 1 : n <= 20 ? 2 : n <= 30 ? 3 : n <= 40 ? 4 : 5);

export const seedSpec = {
  property: {
    pk: SEED_PK,
    name: "Grand Palm Hotel Lagos",
    code: "GPHL",
    address: "14 Palm Springs Avenue, Victoria Island",
    city: "Lagos",
    country: "Nigeria",
    phone: "+234-1-700-INFINEON",
    email: "reservations@grandpalmlagos.com",
    timezone: "Africa/Lagos",
    currency: NGN,
    checkInTime: "14:00",
    checkOutTime: "12:00",
    taxRateBps: 750,
    serviceChargeBps: 1000,
    active: true,
    createdAt: Date.now(),
  },
  roomTypes: seedRoomTypes,
  ratePlans: seedRatePlans,
  roomCount: 50,
  roomStatuses,
  typeOf,
  levelOf,
  users: [
    { email: "admin@infineon.com", name: "Chidinma Ike" },
    { email: "gm@infineon.com", name: "Sade Ogunlesi" },
    { email: "frontdesk@infineon.com", name: "Blessing Effiong" },
    { email: "housekeeping@infineon.com", name: "Ngozi Okafor" },
    { email: "finance@infineon.com", name: "Halima Bello" },
  ],
} as const;

/** Idempotent seed — safe to call from any module or the dashboard. */
export const seed = internalMutation({
  args: {},
  handler: async (ctx): Promise<{ propertyId: Id<"properties">; created: boolean }> => {
    const existing = await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique();
    if (existing) return { propertyId: existing._id, created: false };

    const now = Date.now();
    const propertyId = await ctx.db.insert("properties", { ...seedSpec.property, createdAt: now });

    for (const u of seedSpec.users) {
      const dupe = await ctx.db.query("users").withIndex("by_email", (q) => q.eq("email", u.email)).unique();
      if (!dupe) await ctx.db.insert("users", u);
    }

    const rtIds = new Map<string, Id<"roomTypes">>();
    for (const rt of seedSpec.roomTypes) {
      rtIds.set(rt.code, await ctx.db.insert("roomTypes", { propertyId, ...rt }));
    }
    for (const rp of seedSpec.ratePlans) {
      await ctx.db.insert("ratePlans", {
        propertyId,
        roomTypeId: rtIds.get(rp.rt)!,
        code: rp.code,
        name: rp.name,
        priceKobo: rp.priceKobo,
        includesBreakfast: rp.includesBreakfast,
        refundable: rp.refundable,
        minNights: rp.minNights,
        active: true,
      });
    }

    const floorIds = new Map<number, Id<"floors">>();
    for (const level of [1, 2, 3, 4, 5]) {
      floorIds.set(level, await ctx.db.insert("floors", { propertyId, name: `Floor ${level}`, level }));
    }

    for (let n = 1; n <= seedSpec.roomCount; n++) {
      const s = seedSpec.roomStatuses[n];
      await ctx.db.insert("rooms", {
        propertyId,
        number: String(n).padStart(3, "0"),
        roomTypeId: rtIds.get(seedSpec.typeOf(n))!,
        floorId: floorIds.get(seedSpec.levelOf(n))!,
        status: s.status,
        hkStatus: s.hk,
        lastCleanedAt: undefined,
      });
    }

    await ctx.db.insert("auditLogs", {
      propertyId,
      actorName: "system",
      action: "platform.seeded",
      entity: "property",
      entityId: String(propertyId),
      severity: "info",
      details: { rooms: seedSpec.roomCount, roomTypes: seedSpec.roomTypes.length },
      at: now,
    });

    return { propertyId, created: true };
  },
});

/** Call before any public flow touches data. Cheap no-op once seeded. */
export const ensureSeed = internalMutation({
  args: {},
  handler: async (ctx): Promise<Id<"properties">> => {
    const existing = await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique();
    if (existing) return existing._id;
    const { propertyId } = await ctx.runMutation(internal.lib.seed, {});
    return propertyId;
  },
});

export const getPrimaryProperty = internalQuery({
  args: {},
  handler: async (ctx): Promise<Doc<"properties">> => {
    const p = await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique();
    if (!p) throw new ConvexError("Property not initialised — run seed first");
    return p;
  },
});

/** Read-only variant for queries (they cannot run mutations to seed). */
export const primaryPropertyOrNull = internalQuery({
  args: {},
  handler: async (ctx): Promise<Doc<"properties"> | null> => {
    return ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique() ?? null;
  },
});

/** Plain helper usable from queries, mutations and actions alike. */
export async function primaryProperty(ctx: QueryCtx | MutationCtx): Promise<Doc<"properties"> | null> {
  return (await ctx.db.query("properties").withIndex("by_pk", (q) => q.eq("pk", SEED_PK)).unique()) ?? null;
}

export async function requireProperty(ctx: QueryCtx | MutationCtx): Promise<Doc<"properties">> {
  const property = await primaryProperty(ctx);
  if (!property) throw new ConvexError("Property not initialised — run seed first");
  return property;
}

// ── Timezone-aware local-day range helpers (revenue/stat windows) ────────
function tzOffsetMs(utcMs: number, timezone: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    hour12: false,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).formatToParts(new Date(utcMs));
  const get = (t: string) => Number(parts.find((p) => p.type === t)?.value ?? "0");
  const asUtc = Date.UTC(get("year"), get("month") - 1, get("day"), get("hour") % 24, get("minute"), get("second"));
  return asUtc - utcMs;
}

/** [start, end) UTC timestamps covering the local calendar day `iso`. */
export function localDayRange(iso: string, timezone: string): [number, number] {
  const guess = Date.parse(`${iso}T00:00:00Z`);
  let start = guess - tzOffsetMs(guess, timezone);
  start = guess - tzOffsetMs(start, timezone); // second pass for DST edges
  return [start, start + 86_400_000];
}
