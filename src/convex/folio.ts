// M-FIN — folio (guest account) posting engine.
//
// Single funnel for guest-facing money: room nights, POS room-charges,
// minibar, spa, taxes, service charge, discounts and settlements. Each charge
// writes the folio line AND the matching double-entry GL pair through the
// ledger module — folio balance and general ledger can never diverge.

import { v, ConvexError } from "convex/values";
import { mutation, query, type MutationCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { pctKobo, todayPropertyLocal } from "./lib";
import { emit, EVENTS } from "./events";
import { postJournal, ACCOUNTS } from "./ledger";
import { requireAuth } from "./authz";

// Revenue/liability account credited per folio line category.
const CATEGORY_ACCOUNT: Record<string, string> = {
  room: ACCOUNTS.roomRevenue,
  fnb: ACCOUNTS.fbRevenue,
  events: ACCOUNTS.eventsRevenue,
  minibar: ACCOUNTS.otherRevenue,
  laundry: ACCOUNTS.otherRevenue,
  spa: ACCOUNTS.otherRevenue,
  parking: ACCOUNTS.otherRevenue,
  other: ACCOUNTS.otherRevenue,
  tax: ACCOUNTS.vatPayable,
  service: ACCOUNTS.serviceCharge,
};

export interface ChargeInput {
  propertyId: Id<"properties">;
  folioId: Id<"folios">;
  folioCode: string;
  category: string;
  description: string;
  qty?: number;
  unitKobo: number;
  department?: string;
  by?: string;
  /** Business date for the GL entry (defaults to property-local today). */
  date?: string;
  /** Add VAT + service charge as separate folio lines (POS-style pricing). */
  addTaxAndService?: boolean;
}

export interface ChargeResult {
  chargeKobo: number;
  taxKobo: number;
  serviceKobo: number;
  totalAddedKobo: number;
}

/**
 * Post one charge (and its GL pair) to a folio. Negative amounts are
 * discounts: Dr Discounts & Allowances, Cr Guest Ledger.
 */
export async function postChargeInternal(ctx: MutationCtx, input: ChargeInput): Promise<ChargeResult> {
  const property = (await ctx.db.get(input.propertyId)) as Doc<"properties"> | null;
  if (!property) throw new ConvexError("Property not found");

  const qty = input.qty ?? 1;
  const chargeKobo = Math.round(qty * input.unitKobo);
  const date = input.date ?? todayPropertyLocal(property.timezone);
  const now = Date.now();

  await ctx.db.insert("folioLines", {
    propertyId: input.propertyId,
    folioId: input.folioId,
    postedAt: now,
    category: input.category,
    description: input.description,
    qty,
    unitKobo: input.unitKobo,
    amountKobo: chargeKobo,
    department: input.department,
    postedBy: input.by,
  });

  const folio = (await ctx.db.get(input.folioId)) as Doc<"folios"> | null;
  if (!folio) throw new ConvexError("Folio not found");
  await ctx.db.patch(input.folioId, { totalKobo: folio.totalKobo + chargeKobo });

  if (chargeKobo !== 0) {
    const description = `${input.description} [${input.folioCode}]`;
    if (chargeKobo > 0) {
      const revenue = CATEGORY_ACCOUNT[input.category] ?? ACCOUNTS.otherRevenue;
      await postJournal(ctx, {
        propertyId: input.propertyId,
        date,
        source: "folio",
        ref: input.folioCode,
        by: input.by,
        lines: [
          { accountCode: ACCOUNTS.guestLedger, debitKobo: chargeKobo, creditKobo: 0, description },
          { accountCode: revenue, debitKobo: 0, creditKobo: chargeKobo, description },
        ],
      });
    } else {
      await postJournal(ctx, {
        propertyId: input.propertyId,
        date,
        source: "folio",
        ref: input.folioCode,
        by: input.by,
        lines: [
          { accountCode: ACCOUNTS.discounts, debitKobo: -chargeKobo, creditKobo: 0, description: `Discount: ${description}` },
          { accountCode: ACCOUNTS.guestLedger, debitKobo: 0, creditKobo: -chargeKobo, description: `Discount: ${description}` },
        ],
      });
    }
  }

  let taxKobo = 0;
  let serviceKobo = 0;
  if (input.addTaxAndService && chargeKobo > 0) {
    taxKobo = pctKobo(chargeKobo, property.taxRateBps / 10_000);
    serviceKobo = pctKobo(chargeKobo, property.serviceChargeBps / 10_000);
    if (taxKobo > 0) {
      await postChargeInternal(ctx, {
        ...input,
        addTaxAndService: false,
        qty: 1,
        unitKobo: taxKobo,
        category: "tax",
        description: `VAT ${(property.taxRateBps / 100).toFixed(2)}% · ${input.description}`,
        date,
      });
    }
    if (serviceKobo > 0) {
      await postChargeInternal(ctx, {
        ...input,
        addTaxAndService: false,
        qty: 1,
        unitKobo: serviceKobo,
        category: "service",
        description: `Service ${(property.serviceChargeBps / 100).toFixed(2)}% · ${input.description}`,
        date,
      });
    }
  }

  await emit(ctx, {
    propertyId: input.propertyId,
    type: EVENTS.folioChargePosted,
    source: "folio",
    entityId: input.folioCode,
    payload: { folioId: input.folioId, category: input.category, chargeKobo, taxKobo, serviceKobo },
    by: input.by,
  });

  return { chargeKobo, taxKobo, serviceKobo, totalAddedKobo: chargeKobo + taxKobo + serviceKobo };
}

/** Open (or fetch) the folio for a reservation, carrying prepaid amounts. */
export async function ensureForReservation(ctx: MutationCtx, res: Doc<"reservations">): Promise<Doc<"folios">> {
  const existing =
    (await ctx.db.query("folios").withIndex("by_reservation", (q) => q.eq("reservationId", res._id)).collect())[0] ?? null;
  if (existing) return existing;

  const counter = await ctx.db
    .query("counters")
    .withIndex("by_property_key", (q) => q.eq("propertyId", res.propertyId).eq("key", "folio"))
    .unique();
  const value = (counter?.value ?? 0) + 1;
  if (counter) await ctx.db.patch(counter._id, { value });
  else await ctx.db.insert("counters", { propertyId: res.propertyId, key: "folio", value });

  const code = `FOL-${String(value).padStart(6, "0")}`;
  const id = await ctx.db.insert("folios", {
    propertyId: res.propertyId,
    reservationId: res._id,
    code,
    status: "open",
    totalKobo: 0,
    paidKobo: res.paidKobo, // deposits/prepayments carried onto the guest account
    openedAt: Date.now(),
  });
  await emit(ctx, {
    propertyId: res.propertyId,
    type: EVENTS.folioOpened,
    source: "stays",
    entityId: code,
    payload: { folioId: id, reservationId: res._id, carriedPaidKobo: res.paidKobo },
  });
  return (await ctx.db.get(id)) as Doc<"folios">;
}

const roomNightDescription = (night: string) => `Room night · ${night}`;

/**
 * Post one consumed room night for a stay — idempotent per (folio, night),
 * GL-dated on the night itself so revenue lands on the correct business date.
 * Returns true when a new night was posted.
 */
export async function postRoomNight(
  ctx: MutationCtx,
  res: Doc<"reservations">,
  folio: Doc<"folios">,
  night: string,
  by?: string,
): Promise<boolean> {
  const lines = await ctx.db.query("folioLines").withIndex("by_folio", (q) => q.eq("folioId", folio._id)).collect();
  const description = roomNightDescription(night);
  if (lines.some((l) => l.category === "room" && l.description === description)) return false;
  await postChargeInternal(ctx, {
    propertyId: res.propertyId,
    folioId: folio._id,
    folioCode: folio.code,
    category: "room",
    description,
    qty: 1,
    unitKobo: res.nightlyRateKobo,
    department: "rooms",
    date: night,
    by,
  });
  return true;
}

// ── Settlement ───────────────────────────────────────────────────────────
export const settle = mutation({
  args: {
    token: v.string(),
    folioId: v.id("folios"),
    amountKobo: v.number(),
    method: v.string(), // cash, card, transfer, pos_terminal, online
    reference: v.optional(v.string()),
  },
  handler: async (ctx, { token, folioId, amountKobo, method, reference }) => {
    const auth = await requireAuth(ctx, token, "fin.folio.settle");
    if (!(amountKobo > 0)) throw new ConvexError("Amount must be positive");
    const folio = (await ctx.db.get(folioId)) as Doc<"folios"> | null;
    if (!folio || folio.propertyId !== auth.propertyId) throw new ConvexError("Folio not found");
    if (folio.status !== "open") throw new ConvexError("Folio is closed");

    const property = (await ctx.db.get(auth.propertyId)) as Doc<"properties">;
    const now = Date.now();
    const ref = reference ?? `SET-${folio.code}-${now}`;
    await ctx.db.insert("payments", {
      propertyId: auth.propertyId,
      folioId,
      method,
      amountKobo,
      currency: property.currency,
      reference: ref,
      gateway: "manual",
      status: "success",
      channelResponse: "Settled in console",
      createdAt: now,
      paidAt: now,
      receivedBy: auth.name,
    });
    const paidKobo = folio.paidKobo + amountKobo;
    await ctx.db.patch(folioId, { paidKobo });

    // GL: Dr cash/bank, Cr guest ledger — delivered via the payment.captured
    // outbox so website captures and console settlements share one posting rule.
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.paymentCaptured,
      source: "folio",
      entityId: folio.code,
      payload: {
        propertyId: auth.propertyId,
        date: todayPropertyLocal(property.timezone),
        method,
        amountKobo,
        reference: ref,
        description: `Folio settlement ${folio.code}`,
      },
      by: auth.name,
    });
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.folioSettled,
      source: "folio",
      entityId: folio.code,
      payload: { folioId, amountKobo, method, paidKobo, balanceKobo: folio.totalKobo - paidKobo },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "folio.payment_posted",
      entity: "folio",
      entityId: folio.code,
      severity: "info",
      details: { amountKobo, method, paidKobo },
      at: now,
    });
    return { ok: true as const, paidKobo, balanceKobo: folio.totalKobo - paidKobo };
  },
});

export const charge = mutation({
  args: {
    token: v.string(),
    folioId: v.id("folios"),
    category: v.string(),
    description: v.string(),
    qty: v.optional(v.number()),
    unitKobo: v.number(),
    department: v.optional(v.string()),
    addTaxAndService: v.optional(v.boolean()),
  },
  handler: async (ctx, args) => {
    const auth = await requireAuth(ctx, args.token, "fin.folio.charge");
    const folio = (await ctx.db.get(args.folioId)) as Doc<"folios"> | null;
    if (!folio || folio.propertyId !== auth.propertyId) throw new ConvexError("Folio not found");
    if (folio.status !== "open") throw new ConvexError("Folio is closed");
    if (!(args.unitKobo > 0)) throw new ConvexError("Unit amount must be positive");
    const result = await postChargeInternal(ctx, {
      propertyId: auth.propertyId,
      folioId: args.folioId,
      folioCode: folio.code,
      category: args.category,
      description: args.description,
      qty: args.qty,
      unitKobo: args.unitKobo,
      department: args.department,
      by: auth.name,
      addTaxAndService: args.addTaxAndService,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "folio.charge_posted",
      entity: "folio",
      entityId: folio.code,
      severity: "info",
      details: { category: args.category, amountKobo: result.chargeKobo, description: args.description },
      at: Date.now(),
    });
    return { ok: true as const, ...result };
  },
});

// ── Reads ────────────────────────────────────────────────────────────────
export const statement = query({
  args: { token: v.string(), folioId: v.id("folios") },
  handler: async (ctx, { token, folioId }) => {
    const auth = await requireAuth(ctx, token, "fin.folio.view");
    const folio = (await ctx.db.get(folioId)) as Doc<"folios"> | null;
    if (!folio || folio.propertyId !== auth.propertyId) throw new ConvexError("Folio not found");
    const lines = await ctx.db.query("folioLines").withIndex("by_folio", (q) => q.eq("folioId", folioId)).collect();
    const payments = await ctx.db
      .query("payments")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect();
    const res = (await ctx.db.get(folio.reservationId)) as Doc<"reservations"> | null;
    const guest = res ? ((await ctx.db.get(res.guestId)) as Doc<"guests"> | null) : null;
    lines.sort((a, b) => a.postedAt - b.postedAt);
    return {
      folio,
      lines,
      payments: payments.filter((p) => p.folioId === folioId),
      reservation: res,
      guestName: guest ? `${guest.firstName} ${guest.lastName}` : null,
      balanceKobo: folio.totalKobo - folio.paidKobo,
    };
  },
});

export const byReservation = query({
  args: { token: v.string(), reservationId: v.id("reservations") },
  handler: async (ctx, { token, reservationId }) => {
    const auth = await requireAuth(ctx, token, "fin.folio.view");
    const rows = await ctx.db
      .query("folios")
      .withIndex("by_reservation", (q) => q.eq("reservationId", reservationId))
      .collect();
    const row = rows.find((f) => f.propertyId === auth.propertyId) ?? null;
    if (!row) return null;
    return { folio: row, balanceKobo: row.totalKobo - row.paidKobo };
  },
});

export const openFolios = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "fin.folio.view");
    const rows = await ctx.db
      .query("folios")
      .withIndex("by_property_status", (q) => q.eq("propertyId", auth.propertyId).eq("status", "open"))
      .collect();
    return rows.map((f) => ({ ...f, balanceKobo: f.totalKobo - f.paidKobo }));
  },
});
