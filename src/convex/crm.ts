// M-CRM — loyalty & guest value.
//
// Consumers subscribe through the domain event bus: a checkout accrues points
// and lifetime value without stays.ts ever importing this module. The
// loyaltyLedger table keeps every balance change auditable.

import { v } from "convex/values";
import { internalMutation, type MutationCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { requireProperty, todayPropertyLocal } from "./lib";
import { query } from "./_generated/server";
import { requireAuth } from "./authz";

// Tier thresholds on lifetime value (kobo): ₦200k / ₦500k / ₦1m.
const TIER_THRESHOLDS: Array<{ tier: string; minLifetimeKobo: number }> = [
  { tier: "bronze", minLifetimeKobo: 0 },
  { tier: "silver", minLifetimeKobo: 20_000_000 },
  { tier: "gold", minLifetimeKobo: 50_000_000 },
  { tier: "platinum", minLifetimeKobo: 100_000_000 },
];

export function tierFor(lifetimeValueKobo: number): string {
  let tier = "bronze";
  for (const t of TIER_THRESHOLDS) if (lifetimeValueKobo >= t.minLifetimeKobo) tier = t.tier;
  return tier;
}

/** 1 point per ₦100 spent + 25 bonus points per night. */
export function stayPoints(totalKobo: number, nights: number): number {
  return Math.round(totalKobo / 10_000) + nights * 25;
}

async function recordMovement(
  ctx: MutationCtx,
  input: {
    propertyId: Id<"properties">;
    guestId: Id<"guests">;
    reservationId?: Id<"reservations">;
    delta: number;
    reason: string;
    balanceAfter: number;
  },
): Promise<void> {
  await ctx.db.insert("loyaltyLedger", { ...input, at: Date.now() });
}

async function loadOrCreateMember(
  ctx: MutationCtx,
  propertyId: Id<"properties">,
  guestId: Id<"guests">,
): Promise<Doc<"loyaltyMembers">> {
  const rows = await ctx.db.query("loyaltyMembers").withIndex("by_guest", (q) => q.eq("guestId", guestId)).collect();
  const existing = rows.find((m) => m.propertyId === propertyId);
  if (existing) return existing;
  const id = await ctx.db.insert("loyaltyMembers", {
    propertyId,
    guestId,
    tier: "bronze",
    points: 0,
    lifetimeValueKobo: 0,
    joinedAt: Date.now(),
  });
  return (await ctx.db.get(id)) as Doc<"loyaltyMembers">;
}

interface StayCompletedPayload {
  propertyId: Id<"properties">;
  guestId: Id<"guests">;
  reservationId: Id<"reservations">;
  nights: number;
  totalKobo: number;
  folioCode?: string;
}

/** reservation.checked_out → accrue points, LTV and tier. Idempotent per reservation via ledger reason. */
export const onStayCompleted = internalMutation({
  args: { payload: v.any() },
  handler: async (ctx, { payload }) => {
    const p = payload as StayCompletedPayload;
    if (!p?.propertyId || !p.guestId || !p.reservationId) return;

    // Idempotency guard: one "stay" movement per reservation.
    const movements = await ctx.db
      .query("loyaltyLedger")
      .withIndex("by_guest", (q) => q.eq("guestId", p.guestId))
      .collect();
    if (movements.some((m) => m.reservationId === p.reservationId && m.reason === "stay")) return;

    const member = await loadOrCreateMember(ctx, p.propertyId, p.guestId);
    const points = stayPoints(p.totalKobo, Math.max(0, p.nights));
    const lifetimeValueKobo = member.lifetimeValueKobo + p.totalKobo;
    const balanceAfter = member.points + points;
    await ctx.db.patch(member._id, {
      points: balanceAfter,
      lifetimeValueKobo,
      tier: tierFor(lifetimeValueKobo),
    });
    await recordMovement(ctx, {
      propertyId: p.propertyId,
      guestId: p.guestId,
      reservationId: p.reservationId,
      delta: points,
      reason: "stay",
      balanceAfter,
    });
  },
});

interface CheckedInPayload {
  propertyId: Id<"properties">;
  guestId: Id<"guests">;
  reservationId: Id<"reservations">;
}

/** reservation.checked_in → one-time welcome bonus for first-ever stay. */
export const onGuestCheckedIn = internalMutation({
  args: { payload: v.any() },
  handler: async (ctx, { payload }) => {
    const p = payload as CheckedInPayload;
    if (!p?.propertyId || !p.guestId || !p.reservationId) return;
    const movements = await ctx.db
      .query("loyaltyLedger")
      .withIndex("by_guest", (q) => q.eq("guestId", p.guestId))
      .collect();
    if (movements.length > 0) return; // member already has history — no welcome bonus

    const member = await loadOrCreateMember(ctx, p.propertyId, p.guestId);
    const balanceAfter = member.points + 100;
    await ctx.db.patch(member._id, { points: balanceAfter });
    await recordMovement(ctx, {
      propertyId: p.propertyId,
      guestId: p.guestId,
      reservationId: p.reservationId,
      delta: 100,
      reason: "welcome",
      balanceAfter,
    });
  },
});

// ── Reads ────────────────────────────────────────────────────────────────
export const guest360 = query({
  args: { token: v.string(), guestId: v.id("guests") },
  handler: async (ctx, { token, guestId }) => {
    const auth = await requireAuth(ctx, token, "crm.view");
    const property = await requireProperty(ctx);
    const guest = (await ctx.db.get(guestId)) as Doc<"guests"> | null;
    if (!guest || guest.propertyId !== auth.propertyId) return null;

    const reservations = (await ctx.db
      .query("reservations")
      .withIndex("by_guest", (q) => q.eq("guestId", guestId))
      .collect()).filter((r) => r.propertyId === auth.propertyId);
    const memberRows = await ctx.db.query("loyaltyMembers").withIndex("by_guest", (q) => q.eq("guestId", guestId)).collect();
    const member = memberRows.find((m) => m.propertyId === auth.propertyId) ?? null;
    const movements = member
      ? (await ctx.db.query("loyaltyLedger").withIndex("by_guest", (q) => q.eq("guestId", guestId)).collect()).filter((m) => m.propertyId === auth.propertyId)
      : [];

    const completed = reservations.filter((r) => r.status === "checked_out");
    const stays = completed.length;
    const nights = completed.reduce((s, r) => s + r.nights, 0);
    const spendKobo = member?.lifetimeValueKobo ?? completed.reduce((s, r) => s + r.totalKobo, 0);
    const today = todayPropertyLocal(property.timezone);
    const upcoming = reservations.filter((r) => (r.status === "confirmed" || r.status === "pending") && r.checkOutDate > today);

    return {
      guest,
      member,
      stays,
      nights,
      spendKobo,
      reservations,
      upcoming,
      movements: movements.sort((a, b) => b.at - a.at).slice(0, 50),
    };
  },
});

export const directory = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "crm.view");
    const guests = (await ctx.db.query("guests").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect()).sort(
      (a, b) => b.createdAt - a.createdAt,
    );
    const members = await ctx.db.query("loyaltyMembers").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const memberByGuest = new Map(members.map((m) => [m.guestId, m]));
    return guests.map((g) => ({ guest: g, member: memberByGuest.get(g._id) ?? null }));
  },
});
