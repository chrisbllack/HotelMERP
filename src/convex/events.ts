// ADR-0006 cross-module contract bus.
//
// Producers call emit() inside their own transaction; delivery happens after
// commit through a scheduler-dispatched outbox (domainEvents table). Modules
// therefore interconnect without importing each other's mutations — a
// reservation checkout fans out to CRM loyalty, inventory fans out to
// procurement, and neither producer knows its consumers.

import { v } from "convex/values";
import { mutation, query, internalMutation, type MutationCtx } from "./_generated/server";
import { internal } from "./_generated/api";
import type { Id } from "./_generated/dataModel";
import { requireAuth } from "./authz";

/** Canonical event type names — the published contract vocabulary. */
export const EVENTS = {
  reservationCreated: "reservation.created",
  reservationCheckedIn: "reservation.checked_in",
  reservationCheckedOut: "reservation.checked_out",
  reservationCancelled: "reservation.cancelled",
  reservationNoShow: "reservation.no_show",
  roomStatusChanged: "room.status_changed",
  folioOpened: "folio.opened",
  folioChargePosted: "folio.charge_posted",
  folioSettled: "folio.settled",
  folioClosed: "folio.closed",
  paymentCaptured: "payment.captured",
  journalPosted: "journal.posted",
  orderClosed: "order.closed",
  stockMovement: "stock.movement",
  stockLow: "stock.low",
  poReceived: "po.received",
  payrollApproved: "payroll.approved",
  nightAuditClosed: "nightaudit.closed",
} as const;

export type EventType = (typeof EVENTS)[keyof typeof EVENTS];

export interface EmitInput {
  propertyId: Id<"properties">;
  type: EventType | string;
  source: string;
  entityId?: string;
  payload?: unknown;
  by?: string;
}

/** Publish a domain event within the caller's transaction. */
export async function emit(ctx: MutationCtx, input: EmitInput): Promise<Id<"domainEvents">> {
  const id = await ctx.db.insert("domainEvents", {
    propertyId: input.propertyId,
    type: input.type,
    source: input.source,
    entityId: input.entityId,
    payload: input.payload ?? {},
    status: "pending",
    attempts: 0,
    emittedBy: input.by,
    at: Date.now(),
  });
  await ctx.scheduler.runAfter(0, internal.events.dispatch, { eventId: id });
  return id;
}

// ── Subscriber registry ──────────────────────────────────────────────────
// Each handler is an internalMutation with args { payload: v.any() }.
type Subscriber = (ctx: unknown, args: { payload: unknown }) => Promise<unknown>;

const SUBSCRIBERS: Record<string, Subscriber[]> = {
  // M-FIN: every captured payment posts Dr cash/bank, Cr guest ledger.
  [EVENTS.paymentCaptured]: [internal.ledger.onPaymentCaptured as unknown as Subscriber],
  // M-LOD → M-CRM: post-stay loyalty accrual + lifetime value.
  [EVENTS.reservationCheckedOut]: [internal.crm.onStayCompleted as unknown as Subscriber],
  // M-LOD → M-CRM: welcome points for new guests.
  [EVENTS.reservationCheckedIn]: [internal.crm.onGuestCheckedIn as unknown as Subscriber],
  // M-INV → M-PRC: reorder check after each stock movement.
  [EVENTS.stockMovement]: [internal.inventory.onStockMovement as unknown as Subscriber],
};

export const dispatch = internalMutation({
  args: { eventId: v.id("domainEvents") },
  handler: async (ctx, { eventId }) => {
    const event = await ctx.db.get(eventId);
    if (!event || event.status === "delivered") return;
    const handlers = SUBSCRIBERS[event.type] ?? [];
    let error: string | undefined;
    for (const handler of handlers) {
      try {
        await handler(ctx, { payload: event.payload });
      } catch (err) {
        error = err instanceof Error ? err.message : String(err);
        break; // keep event in failed state for retry — handlers must be idempotent
      }
    }
    await ctx.db.patch(eventId, {
      status: error ? "failed" : "delivered",
      attempts: event.attempts + 1,
      error,
      dispatchedAt: Date.now(),
    });
  },
});

/** sec.admin manual re-delivery for failed events. */
export const retry = mutation({
  args: { token: v.string(), eventId: v.id("domainEvents") },
  handler: async (ctx, { token, eventId }) => {
    await requireAuth(ctx, token, "sec.admin");
    const event = await ctx.db.get(eventId);
    if (!event) throw new Error("Event not found");
    await ctx.db.patch(eventId, { status: "pending", error: undefined });
    await ctx.scheduler.runAfter(0, internal.events.dispatch, { eventId });
    return { ok: true as const };
  },
});

export const recent = query({
  args: { token: v.string(), limit: v.optional(v.number()) },
  handler: async (ctx, { token, limit }) => {
    const auth = await requireAuth(ctx, token, "dashboard.view");
    const rows = await ctx.db
      .query("domainEvents")
      .withIndex("by_property_at", (q) => q.eq("propertyId", auth.propertyId))
      .order("desc")
      .take(Math.min(limit ?? 50, 200));
    return rows;
  },
});

export const byType = query({
  args: { token: v.string(), type: v.string(), limit: v.optional(v.number()) },
  handler: async (ctx, { token, type, limit }) => {
    const auth = await requireAuth(ctx, token, "dashboard.view");
    return ctx.db
      .query("domainEvents")
      .withIndex("by_property_type", (q) => q.eq("propertyId", auth.propertyId).eq("type", type))
      .order("desc")
      .take(Math.min(limit ?? 50, 200));
  },
});

export const failed = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "sec.admin");
    const rows = await ctx.db
      .query("domainEvents")
      .withIndex("by_status", (q) => q.eq("status", "failed"))
      .order("desc")
      .take(100);
    return rows.filter((r) => r.propertyId === auth.propertyId);
  },
});
