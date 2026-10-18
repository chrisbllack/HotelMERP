// M-PRC — Payments & settlement (public website checkout + webhook).
//
// Vendor-neutral adapter over Paystack's REST API (AGENTS.md rule 7 — no SDK):
// when PAYSTACK_SECRET_KEY is present, checkout initializes on Paystack and
// payment is confirmed via server-verified webhook / verify endpoint.
// Without a key, the site runs in "pay at front desk" mode: the booking is
// confirmed with a house payment record and settleable in-console.
//
// Public surface (via http.ts):
//   POST /payments/initialize
//   POST /payments/verify/:reference      (client polls after gateway redirect)
//   POST /paystack/webhook                (server-to-server, HMAC-SHA512 verified)

import { v } from "convex/values";
import { ConvexError } from "convex/values";
import { action, internalAction, internalMutation, type MutationCtx } from "./_generated/server";
import { internal } from "./_generated/api";
import type { Doc } from "./_generated/dataModel";
import { emit, EVENTS } from "./events";
import { todayPropertyLocal } from "./lib";

const PAYSTACK_INIT_URL = "https://api.paystack.co/transaction/initialize";
const PAYSTACK_VERIFY_URL = (ref: string) => `https://api.paystack.co/transaction/verify/${encodeURIComponent(ref)}`;
const SEED_PK = "grand-palm-lagos";

interface InitializeResponse {
  status: boolean;
  message: string;
  data?: { authorization_url?: string; reference?: string };
}
interface VerifyResponse {
  status: boolean;
  message: string;
  data?: { status?: string; amount?: number; reference?: string };
}

export interface InitializeResult {
  mode: "paystack" | "demo";
  reference: string | null;
  authorizationUrl: string | null;
}

export const initializePayment = action({
  args: {
    code: v.string(),
    email: v.string(),
    amountKobo: v.number(),
  },
  handler: async (ctx, { code, email, amountKobo }): Promise<InitializeResult> => {
    if (amountKobo <= 0) throw new ConvexError("Amount must be positive");
    const secret = process.env.PAYSTACK_SECRET_KEY;

    // Demo mode (no key): confirm the booking with a house payment record.
    if (!secret) {
      await ctx.runMutation(internal.payments.applyDemoPayment, { code, amountKobo });
      return { mode: "demo", reference: null, authorizationUrl: null };
    }

    const res = await fetch(PAYSTACK_INIT_URL, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${secret}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email,
        amount: amountKobo, // Paystack expects kobo
        reference: `INF-${code}-${Date.now().toString(36).toUpperCase()}`,
        metadata: { bookingCode: code, purpose: "booking_deposit" },
      }),
    });
    const body = (await res.json()) as InitializeResponse;
    if (!res.ok || !body.status || !body.data?.authorization_url) {
      throw new ConvexError(body.message || "Payment gateway rejected the request");
    }
    await ctx.runMutation(internal.payments.recordInitialized, {
      code,
      reference: body.data.reference ?? `INF-${code}`,
      amountKobo,
    });
    return { mode: "paystack", reference: body.data.reference ?? null, authorizationUrl: body.data.authorization_url };
  },
});

/** Client polls this after the Paystack redirect; also used by ops tooling. */
export const verifyAndApply = internalAction({
  args: { reference: v.string() },
  handler: async (ctx, { reference }): Promise<{ ok: boolean; mode: string; applied: boolean; gatewayStatus?: string; error?: string }> => {
    const secret = process.env.PAYSTACK_SECRET_KEY;
    if (!secret) {
      // Demo mode has nothing to verify remotely — payment already applied.
      return { ok: true, mode: "demo", applied: false };
    }
    const res = await fetch(PAYSTACK_VERIFY_URL(reference), {
      headers: { Authorization: `Bearer ${secret}` },
    });
    const body = (await res.json()) as VerifyResponse;
    if (!res.ok || !body.status) {
      return { ok: false, mode: "paystack", applied: false, error: body.message ?? "Verification failed" };
    }
    const status = body.data?.status;
    const amount = body.data?.amount ?? 0;
    if (status !== "success") {
      return { ok: true, mode: "paystack", applied: false, gatewayStatus: status ?? "unknown" };
    }
    const paid = await ctx.runMutation(internal.payments.recordGatewaySuccess, { reference, amount });
    return { ok: true, mode: "paystack", applied: paid };
  },
});

/** Server-to-server webhook: Paystack signs the raw body with HMAC-SHA512. */
export const handleWebhook = internalAction({
  args: { signature: v.string(), rawBody: v.string() },
  handler: async (ctx, { signature, rawBody }): Promise<{ ok: boolean; error?: string; ignored?: string; applied?: boolean }> => {
    const secret = process.env.PAYSTACK_SECRET_KEY;
    if (!secret) return { ok: false, error: "Webhook received but no gateway key configured" };

    const key = await crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(secret),
      { name: "HMAC", hash: "SHA-512" },
      false,
      ["sign"],
    );
    const mac = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(rawBody));
    const expected = Array.from(new Uint8Array(mac))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    if (signature !== expected) return { ok: false, error: "Invalid signature" };

    const event = JSON.parse(rawBody) as { event?: string; data?: { reference?: string; amount?: number; status?: string } };
    if (event.event !== "charge.success") return { ok: true, ignored: event.event };
    const reference = event.data?.reference;
    if (!reference) return { ok: false, error: "Missing reference" };
    const paid = await ctx.runMutation(internal.payments.recordGatewaySuccess, {
      reference,
      amount: event.data?.amount ?? 0,
    });
    return { ok: true, applied: paid };
  },
});

// ── internal mutations ───────────────────────────────────────────────────

/** Demo-mode: mark deposit as paid at the house (no external gateway). */
export const applyDemoPayment = internalMutation({
  args: { code: v.string(), amountKobo: v.number() },
  handler: async (ctx, { code, amountKobo }): Promise<void> => {
    const res = await reservationByCode(ctx, code);
    if (!res) throw new ConvexError("Booking not found");
    const property = await propertyById(ctx, res.propertyId);
    const now = Date.now();
    const reference = `DEMO-${res.code}-${now}`;
    const paidKobo = Math.min(res.totalKobo, res.paidKobo + amountKobo);
    await ctx.db.patch(res._id, {
      paidKobo,
      paymentStatus: paidKobo >= res.totalKobo ? "paid" : paidKobo > 0 ? "partial" : "unpaid",
      status: res.status === "pending" ? "confirmed" : res.status,
    });
    await ctx.db.insert("payments", {
      propertyId: res.propertyId,
      reservationId: res._id,
      method: "online",
      amountKobo,
      currency: "NGN",
      reference,
      gateway: "internal",
      status: "success",
      channelResponse: "Demo checkout — pay at front desk mode",
      createdAt: now,
      paidAt: now,
    });
    // Contract: every captured payment posts through M-FIN (outbox → GL).
    await emit(ctx, {
      propertyId: res.propertyId,
      type: EVENTS.paymentCaptured,
      source: "website",
      entityId: res.code,
      payload: {
        propertyId: res.propertyId,
        date: todayPropertyLocal(property.timezone),
        method: "online",
        amountKobo,
        reference,
        description: `Website deposit ${res.code}`,
      },
      by: "website checkout",
    });
    await ctx.db.insert("auditLogs", {
      propertyId: res.propertyId,
      actorName: "website checkout (demo)",
      action: "folio.payment_posted",
      entity: "reservation",
      entityId: res.code,
      severity: "info",
      details: { amountKobo, gateway: "internal" },
      at: now,
    });
  },
});

export const recordInitialized = internalMutation({
  args: { code: v.string(), reference: v.string(), amountKobo: v.number() },
  handler: async (ctx, { code, reference, amountKobo }): Promise<void> => {
    const res = await reservationByCode(ctx, code);
    if (!res) throw new ConvexError("Booking not found");
    await ctx.db.insert("payments", {
      propertyId: res.propertyId,
      reservationId: res._id,
      method: "online",
      amountKobo,
      currency: "NGN",
      reference,
      gateway: "paystack",
      paystackInitRef: reference,
      status: "initialized",
      createdAt: Date.now(),
    });
  },
});

export const recordGatewaySuccess = internalMutation({
  args: { reference: v.string(), amount: v.number() },
  handler: async (ctx, { reference, amount }): Promise<boolean> => {
    const pay = await ctx.db
      .query("payments")
      .withIndex("by_gateway_ref", (q) => q.eq("gatewayRef", reference))
      .unique();
    if (!pay) return false; // unknown reference — left for manual reconciliation
    if (pay.status === "success") return false; // idempotent
    const now = Date.now();
    await ctx.db.patch(pay._id, {
      status: "success",
      channelResponse: "Gateway confirmed payment",
      paidAt: now,
    });
    const res = pay.reservationId ? await ctx.db.get(pay.reservationId) : null;
    if (res) {
      const paidKobo = Math.min(res.totalKobo, res.paidKobo + amount);
      await ctx.db.patch(res._id, {
        paidKobo,
        paymentStatus: paidKobo >= res.totalKobo ? "paid" : "partial",
        status: res.status === "pending" ? "confirmed" : res.status,
      });
    }
    const property = await propertyById(ctx, pay.propertyId);
    await emit(ctx, {
      propertyId: pay.propertyId,
      type: EVENTS.paymentCaptured,
      source: "website",
      entityId: reference,
      payload: {
        propertyId: pay.propertyId,
        date: todayPropertyLocal(property.timezone),
        method: "online",
        amountKobo: amount,
        reference,
        description: `Gateway capture ${reference}`,
      },
      by: "payment gateway",
    });
    await ctx.db.insert("auditLogs", {
      propertyId: pay.propertyId,
      actorName: "payment gateway",
      action: "folio.payment_posted",
      entity: "payment",
      entityId: reference,
      severity: "info",
      details: { gateway: "paystack", amount },
      at: now,
    });
    return true;
  },
});

// ── helpers ──────────────────────────────────────────────────────────────

async function propertyById(ctx: MutationCtx, propertyId: Doc<"properties">["_id"]): Promise<Doc<"properties">> {
  return (await ctx.db.get(propertyId)) as Doc<"properties">;
}

async function reservationByCode(ctx: MutationCtx, code: string): Promise<Doc<"reservations"> | null> {
  const property = await ctx.db
    .query("properties")
    .withIndex("by_pk", (q) => q.eq("pk", SEED_PK))
    .unique();
  if (!property) return null;
  return ctx.db
    .query("reservations")
    .withIndex("by_property_code", (q) => q.eq("propertyId", property._id).eq("code", code.trim().toUpperCase()))
    .unique();
}
