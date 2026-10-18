// M-FNB — point-of-sale order lifecycle.
//
// The settlement decision is where F&B interconnects with the rest of the
// platform: "charge to room" routes through the folio engine (guest ledger +
// revenue GL, settled with the stay), while cash/card settlement posts
// Dr cash/bank directly. Either way the sale lands in the ledger — and
// order.closed feeds the night audit's F&B revenue line.

import { v, ConvexError } from "convex/values";
import { mutation, query, type MutationCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { pctKobo, requireProperty, todayPropertyLocal } from "./lib";
import { emit, EVENTS } from "./events";
import { postChargeInternal, ensureForReservation } from "./folio";
import { postJournal, paymentAccount, ACCOUNTS } from "./ledger";
import { requireAuth, type AuthContext } from "./authz";

async function loadOrder(ctx: MutationCtx, auth: AuthContext, orderId: Id<"posOrders">): Promise<Doc<"posOrders">> {
  const order = (await ctx.db.get(orderId)) as Doc<"posOrders"> | null;
  if (!order || order.propertyId !== auth.propertyId) throw new ConvexError("Order not found");
  return order;
}

function recomputeTotals(
  items: Array<{ qty: number; unitKobo: number }>,
  taxBps: number,
  serviceBps: number,
): { subtotalKobo: number; taxKobo: number; serviceKobo: number; totalKobo: number } {
  const subtotalKobo = items.reduce((s, i) => s + Math.round(i.qty * i.unitKobo), 0);
  const taxKobo = pctKobo(subtotalKobo, taxBps / 10_000);
  const serviceKobo = pctKobo(subtotalKobo, serviceBps / 10_000);
  return { subtotalKobo, taxKobo, serviceKobo, totalKobo: subtotalKobo + taxKobo + serviceKobo };
}

// ── Order creation & lifecycle ───────────────────────────────────────────
export const openOrder = mutation({
  args: {
    token: v.string(),
    outletId: v.id("outlets"),
    tableLabel: v.optional(v.string()),
    items: v.array(v.object({ menuItemId: v.id("menuItems"), qty: v.number() })),
    serverName: v.optional(v.string()),
  },
  handler: async (ctx, { token, outletId, tableLabel, items, serverName }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.order");
    const property = await requireProperty(ctx);
    if (!items.length) throw new ConvexError("Order needs at least one item");
    const outlet = (await ctx.db.get(outletId)) as Doc<"outlets"> | null;
    if (!outlet || outlet.propertyId !== auth.propertyId || !outlet.active) throw new ConvexError("Outlet unavailable");

    const menu = await ctx.db.query("menuItems").withIndex("by_outlet", (q) => q.eq("outletId", outletId)).collect();
    const menuById = new Map(menu.map((m) => [m._id, m]));
    const orderItems = items.map(({ menuItemId, qty }) => {
      const item = menuById.get(menuItemId);
      if (!item) throw new ConvexError("Menu item not found for this outlet");
      if (!item.available) throw new ConvexError(`${item.name} is unavailable`);
      if (!(qty > 0)) throw new ConvexError("Quantity must be positive");
      return { menuItemId, name: item.name, qty, unitKobo: item.priceKobo, totalKobo: Math.round(qty * item.priceKobo) };
    });
    const totals = recomputeTotals(orderItems, property.taxRateBps, property.serviceChargeBps);

    const counter = await ctx.db
      .query("counters")
      .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", "pos_order"))
      .unique();
    const value = (counter?.value ?? 0) + 1;
    if (counter) await ctx.db.patch(counter._id, { value });
    else await ctx.db.insert("counters", { propertyId: auth.propertyId, key: "pos_order", value });

    const orderId = await ctx.db.insert("posOrders", {
      propertyId: auth.propertyId,
      outletId,
      orderNo: `ORD-${String(value).padStart(5, "0")}`,
      tableLabel,
      status: "open",
      items: orderItems,
      ...totals,
      serverName,
      openedAt: Date.now(),
    });
    return { ok: true as const, orderId: orderId as string, orderNo: `ORD-${String(value).padStart(5, "0")}`, ...totals };
  },
});

export const addItems = mutation({
  args: { token: v.string(), orderId: v.id("posOrders"), items: v.array(v.object({ menuItemId: v.id("menuItems"), qty: v.number() })) },
  handler: async (ctx, { token, orderId, items }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.order");
    const property = await requireProperty(ctx);
    const order = await loadOrder(ctx, auth, orderId);
    if (order.status !== "open") throw new ConvexError(`Cannot modify an order in status "${order.status}"`);

    const menu = await ctx.db.query("menuItems").withIndex("by_outlet", (q) => q.eq("outletId", order.outletId)).collect();
    const menuById = new Map(menu.map((m) => [m._id, m]));
    const next = [...order.items];
    for (const { menuItemId, qty } of items) {
      const item = menuById.get(menuItemId);
      if (!item) throw new ConvexError("Menu item not found for this outlet");
      if (!(qty > 0)) throw new ConvexError("Quantity must be positive");
      const existing = next.find((i) => i.menuItemId === menuItemId);
      if (existing) {
        existing.qty += qty;
        existing.totalKobo = Math.round(existing.qty * existing.unitKobo);
      } else {
        next.push({ menuItemId, name: item.name, qty, unitKobo: item.priceKobo, totalKobo: Math.round(qty * item.priceKobo) });
      }
    }
    const totals = recomputeTotals(next, property.taxRateBps, property.serviceChargeBps);
    await ctx.db.patch(orderId, { items: next, ...totals });
    return { ok: true as const, ...totals };
  },
});

export const voidItem = mutation({
  args: { token: v.string(), orderId: v.id("posOrders"), menuItemId: v.id("menuItems") },
  handler: async (ctx, { token, orderId, menuItemId }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.void");
    const property = await requireProperty(ctx);
    const order = await loadOrder(ctx, auth, orderId);
    if (order.status !== "open" && order.status !== "sent") throw new ConvexError(`Cannot void items on "${order.status}"`);
    const next = order.items.filter((i) => i.menuItemId !== menuItemId);
    if (next.length === order.items.length) throw new ConvexError("Item not on this order");
    if (!next.length) throw new ConvexError("Cannot void the last item — void the whole order");
    const totals = recomputeTotals(next, property.taxRateBps, property.serviceChargeBps);
    await ctx.db.patch(orderId, { items: next, ...totals });
    return { ok: true as const, ...totals };
  },
});

export const advanceStatus = mutation({
  args: { token: v.string(), orderId: v.id("posOrders"), to: v.string() }, // sent | served
  handler: async (ctx, { token, orderId, to }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.order");
    const order = await loadOrder(ctx, auth, orderId);
    const allowed: Record<string, string[]> = { open: ["sent"], sent: ["served"], served: ["sent"] };
    if (!allowed[order.status]?.includes(to)) throw new ConvexError(`Cannot move "${order.status}" → "${to}"`);
    await ctx.db.patch(orderId, { status: to });
    return { ok: true as const };
  },
});

// ── Settlement: the folio interconnect ───────────────────────────────────
export const closeOrder = mutation({
  args: {
    token: v.string(),
    orderId: v.id("posOrders"),
    mode: v.string(), // room | cash | card | transfer
    reservationId: v.optional(v.id("reservations")),
    reference: v.optional(v.string()),
  },
  handler: async (ctx, { token, orderId, mode, reservationId, reference }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.settle");
    const property = await requireProperty(ctx);
    const order = await loadOrder(ctx, auth, orderId);
    if (order.status === "paid") throw new ConvexError("Order already settled");
    if (order.status === "void") throw new ConvexError("Order is void");
    const now = Date.now();

    if (mode === "room") {
      // Charge to the guest folio — one line per order, tax & service split,
      // all posting to the guest ledger through the folio engine.
      if (!reservationId) throw new ConvexError("Room charge requires a reservation");
      const res = (await ctx.db.get(reservationId)) as Doc<"reservations"> | null;
      if (!res || res.propertyId !== auth.propertyId) throw new ConvexError("Reservation not found");
      if (res.status !== "checked_in") throw new ConvexError("Guest is not in house — room charge unavailable");
      const folio = await ensureForReservation(ctx, res);
      const outlet = (await ctx.db.get(order.outletId)) as Doc<"outlets"> | null;
      await postChargeInternal(ctx, {
        propertyId: auth.propertyId,
        folioId: folio._id,
        folioCode: folio.code,
        category: "fnb",
        description: `${order.orderNo} · ${outlet?.name ?? "Outlet"}${order.tableLabel ? ` · ${order.tableLabel}` : ""}`,
        qty: 1,
        unitKobo: order.subtotalKobo,
        department: "fnb",
        addTaxAndService: false, // tax/service follow as their own lines below
        by: auth.name,
      });
      if (order.taxKobo > 0) {
        await postChargeInternal(ctx, {
          propertyId: auth.propertyId,
          folioId: folio._id,
          folioCode: folio.code,
          category: "tax",
          description: `VAT · ${order.orderNo}`,
          qty: 1,
          unitKobo: order.taxKobo,
          by: auth.name,
        });
      }
      if (order.serviceKobo > 0) {
        await postChargeInternal(ctx, {
          propertyId: auth.propertyId,
          folioId: folio._id,
          folioCode: folio.code,
          category: "service",
          description: `Service charge · ${order.orderNo}`,
          qty: 1,
          unitKobo: order.serviceKobo,
          by: auth.name,
        });
      }
      await ctx.db.patch(orderId, { status: "paid", closedAt: now, roomChargeFolioId: folio._id, paymentMethod: "room" });
      await ctx.db.insert("auditLogs", {
        propertyId: auth.propertyId,
        actorId: auth.userId,
        actorName: auth.name,
        action: "pos.charged_to_folio",
        entity: "posOrder",
        entityId: order.orderNo,
        severity: "info",
        details: { folio: folio.code, reservation: res.code, totalKobo: order.totalKobo },
        at: now,
      });
      await emit(ctx, {
        propertyId: auth.propertyId,
        type: EVENTS.orderClosed,
        source: "pos",
        entityId: order.orderNo,
        payload: { orderId, mode: "room", folioCode: folio.code, subtotalKobo: order.subtotalKobo, totalKobo: order.totalKobo },
        by: auth.name,
      });
      return { ok: true as const, mode, folioCode: folio.code };
    }

    // Direct settlement: Dr cash/bank, Cr F&B revenue + VAT + service pool.
    if (mode !== "cash" && mode !== "card" && mode !== "transfer") {
      throw new ConvexError(`Unknown settlement mode "${mode}"`);
    }
    const description = `${order.orderNo} · settlement`;
    const lines = [
      { accountCode: paymentAccount(mode), debitKobo: order.totalKobo, creditKobo: 0, description },
      { accountCode: ACCOUNTS.fbRevenue, debitKobo: 0, creditKobo: order.subtotalKobo, description },
      ...(order.taxKobo > 0 ? [{ accountCode: ACCOUNTS.vatPayable, debitKobo: 0, creditKobo: order.taxKobo, description }] : []),
      ...(order.serviceKobo > 0 ? [{ accountCode: ACCOUNTS.serviceCharge, debitKobo: 0, creditKobo: order.serviceKobo, description }] : []),
    ];
    await postJournal(ctx, {
      propertyId: auth.propertyId,
      date: todayPropertyLocal(property.timezone),
      lines,
      source: "pos",
      ref: order.orderNo,
      by: auth.name,
    });
    await ctx.db.insert("payments", {
      propertyId: auth.propertyId,
      method: mode,
      guestName: order.tableLabel,
      amountKobo: order.totalKobo,
      currency: property.currency,
      reference: reference ?? order.orderNo,
      gateway: "internal",
      status: "success",
      channelResponse: "POS settlement",
      createdAt: now,
      paidAt: now,
      receivedBy: auth.name,
    });
    await ctx.db.patch(orderId, { status: "paid", closedAt: now, paymentMethod: mode });
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.orderClosed,
      source: "pos",
      entityId: order.orderNo,
      payload: { orderId, mode, subtotalKobo: order.subtotalKobo, taxKobo: order.taxKobo, serviceKobo: order.serviceKobo, totalKobo: order.totalKobo },
      by: auth.name,
    });
    return { ok: true as const, mode, totalKobo: order.totalKobo };
  },
});

export const voidOrder = mutation({
  args: { token: v.string(), orderId: v.id("posOrders"), reason: v.string() },
  handler: async (ctx, { token, orderId, reason }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.void");
    const order = await loadOrder(ctx, auth, orderId);
    if (order.status === "paid") throw new ConvexError("Settled orders cannot be voided — post a refund instead");
    if (order.status === "void") return { ok: true as const };
    await ctx.db.patch(orderId, { status: "void", closedAt: Date.now(), notes: reason });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "pos.order_voided",
      entity: "posOrder",
      entityId: order.orderNo,
      severity: "warning",
      details: { reason, totalKobo: order.totalKobo },
      at: Date.now(),
    });
    return { ok: true as const };
  },
});

// ── Reads ────────────────────────────────────────────────────────────────
export const activeOrders = query({
  args: { token: v.string(), outletId: v.optional(v.id("outlets")) },
  handler: async (ctx, { token, outletId }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.view");
    const rows = await ctx.db
      .query("posOrders")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect();
    return rows
      .filter((o) => o.status !== "paid" && o.status !== "void" && (outletId ? o.outletId === outletId : true))
      .sort((a, b) => b.openedAt - a.openedAt);
  },
});

export const salesSummary = query({
  args: { token: v.string(), date: v.optional(v.string()) },
  handler: async (ctx, { token, date }) => {
    const auth = await requireAuth(ctx, token, "fin.reports.view");
    const property = await requireProperty(ctx);
    const day = date ?? todayPropertyLocal(property.timezone);
    const start = Date.parse(`${day}T00:00:00+01:00`);
    const end = start + 86_400_000;
    const paid = (await ctx.db.query("posOrders").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect()).filter(
      (o) => o.status === "paid" && o.closedAt !== undefined && o.closedAt >= start && o.closedAt < end,
    );
    const byPayment: Record<string, number> = {};
    for (const o of paid) {
      const key = o.paymentMethod ?? "unknown";
      byPayment[key] = (byPayment[key] ?? 0) + o.totalKobo;
    }
    return {
      date: day,
      orders: paid.length,
      subtotalKobo: paid.reduce((s, o) => s + o.subtotalKobo, 0),
      taxKobo: paid.reduce((s, o) => s + o.taxKobo, 0),
      serviceKobo: paid.reduce((s, o) => s + o.serviceKobo, 0),
      totalKobo: paid.reduce((s, o) => s + o.totalKobo, 0),
      byPayment,
      roomChargeKobo: paid.filter((o) => o.roomChargeFolioId).reduce((s, o) => s + o.totalKobo, 0),
    };
  },
});

export const outlets = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "fnb.pos.view");
    const outletRows = await ctx.db.query("outlets").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const menu = await ctx.db.query("menuItems").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const categories = await ctx.db.query("menuCategories").collect();
    return {
      outlets: outletRows,
      menu: menu.filter((m) => outletRows.some((o) => o._id === m.outletId)),
      categories: categories.filter((c) => outletRows.some((o) => o._id === c.outletId)),
    };
  },
});
