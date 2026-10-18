// M-INV / M-PRC — stock ledger, purchase orders and the reorder loop.
//
// Every quantity change is a movement row (never a silent overwrite), receipts
// post inventory Dr / A/P Cr to the ledger, and after ANY movement the
// stock.movement event fans out to a reorder check: items at or below their
// reorder level that aren't already on an open PO get a draft PO generated
// automatically and publish stock.low for the console.

import { v, ConvexError } from "convex/values";
import { mutation, query, internalMutation, type MutationCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { requireProperty, todayPropertyLocal } from "./lib";
import { emit, EVENTS } from "./events";
import { postJournal, ACCOUNTS } from "./ledger";
import { requireAuth } from "./authz";

const FOOD_CATEGORIES = /food|beverage|breakfast|kitchen|drink|bar/i;

function inventoryAccountFor(category: string): string {
  return FOOD_CATEGORIES.test(category) ? ACCOUNTS.fbInventory : ACCOUNTS.suppliesInventory;
}

function expenseAccountFor(category: string): string {
  return FOOD_CATEGORIES.test(category) ? ACCOUNTS.costOfFood : ACCOUNTS.operatingSupplies;
}

interface MovementInput {
  propertyId: Id<"properties">;
  itemId: Id<"inventoryItems">;
  type: "receipt" | "issue" | "adjustment" | "waste";
  qty: number; // signed for adjustment/waste (negative = out), positive for receipt/issue direction handled internally
  reason?: string;
  ref?: string;
  by?: string;
  date?: string;
}

/**
 * Apply a stock movement atomically: update on-hand, write the movement row,
 * post the ledger entry for value flows, and publish stock.movement.
 */
export async function applyMovement(ctx: MutationCtx, input: MovementInput): Promise<number> {
  const item = (await ctx.db.get(input.itemId)) as Doc<"inventoryItems"> | null;
  if (!item) throw new ConvexError("Inventory item not found");
  if (item.propertyId !== input.propertyId) throw new ConvexError("Item belongs to another property");
  const property = (await ctx.db.get(input.propertyId)) as Doc<"properties">;

  const delta =
    input.type === "receipt" ? Math.abs(input.qty) : input.type === "issue" ? -Math.abs(input.qty) : Math.sign(input.qty) * Math.abs(input.qty);
  if (delta === 0) throw new ConvexError("Quantity must be non-zero");
  const qtyOnHand = item.qtyOnHand + delta;
  if (qtyOnHand < 0) {
    throw new ConvexError(`Insufficient stock for ${item.sku} — on hand ${item.qtyOnHand}, requested ${Math.abs(delta)}`);
  }

  await ctx.db.patch(item._id, { qtyOnHand });
  await ctx.db.insert("stockMovements", {
    propertyId: input.propertyId,
    itemId: input.itemId,
    type: input.type,
    qty: delta,
    reason: input.reason,
    ref: input.ref,
    at: Date.now(),
    by: input.by,
  });

  // Value flows hit the ledger: receipts → inventory Dr / A/P Cr;
  // issues & waste → expense Dr / inventory Cr (at moving cost).
  const date = input.date ?? todayPropertyLocal(property.timezone);
  const valueKobo = Math.round(Math.abs(delta) * item.unitCostKobo);
  if (input.type === "receipt" && valueKobo > 0) {
    await postJournal(ctx, {
      propertyId: input.propertyId,
      date,
      lines: [
        { accountCode: inventoryAccountFor(item.category), debitKobo: valueKobo, creditKobo: 0, description: `Receipt ${item.sku}${input.ref ? ` (${input.ref})` : ""}` },
        { accountCode: ACCOUNTS.accountsPayable, debitKobo: 0, creditKobo: valueKobo, description: `Receipt ${item.sku}${input.ref ? ` (${input.ref})` : ""}` },
      ],
      source: "inventory",
      ref: input.ref,
      by: input.by,
    });
  } else if ((input.type === "issue" || input.type === "waste") && valueKobo > 0) {
    await postJournal(ctx, {
      propertyId: input.propertyId,
      date,
      lines: [
        { accountCode: expenseAccountFor(item.category), debitKobo: valueKobo, creditKobo: 0, description: `${input.type === "waste" ? "Waste" : "Issue"} ${item.sku}${input.reason ? ` — ${input.reason}` : ""}` },
        { accountCode: inventoryAccountFor(item.category), debitKobo: 0, creditKobo: valueKobo, description: `${input.type === "waste" ? "Waste" : "Issue"} ${item.sku}${input.reason ? ` — ${input.reason}` : ""}` },
      ],
      source: "inventory",
      ref: input.ref,
      by: input.by,
    });
  }

  await emit(ctx, {
    propertyId: input.propertyId,
    type: EVENTS.stockMovement,
    source: "inventory",
    entityId: item.sku,
    payload: { propertyId: input.propertyId, itemId: input.itemId, sku: item.sku, type: input.type, delta, qtyOnHand, reorderLevel: item.reorderLevel },
    by: input.by,
  });
  return qtyOnHand;
}

// ── Subscriber: reorder check after every movement ───────────────────────
export const onStockMovement = internalMutation({
  args: { payload: v.any() },
  handler: async (ctx, { payload }) => {
    const p = payload as {
      propertyId: Id<"properties">;
      itemId: Id<"inventoryItems">;
      sku: string;
      qtyOnHand: number;
      reorderLevel: number;
    };
    if (!p?.propertyId || !p.itemId) return;
    if (p.qtyOnHand > p.reorderLevel) return; // healthy

    const item = (await ctx.db.get(p.itemId)) as Doc<"inventoryItems"> | null;
    if (!item) return;

    // Already on an open PO? → do not duplicate.
    const pos = await ctx.db
      .query("purchaseOrders")
      .withIndex("by_property_status", (q) => q.eq("propertyId", p.propertyId).eq("status", "draft"))
      .collect();
    const sent = await ctx.db
      .query("purchaseOrders")
      .withIndex("by_property_status", (q) => q.eq("propertyId", p.propertyId).eq("status", "sent"))
      .collect();
    const open = [...pos, ...sent];
    if (open.some((po) => po.items.some((li) => li.itemId === p.itemId))) return;

    // Draft a replenishment PO (2× reorder level less what's on hand).
    const targetQty = Math.max(1, p.reorderLevel * 2 - p.qtyOnHand);
    const counter = await ctx.db
      .query("counters")
      .withIndex("by_property_key", (q) => q.eq("propertyId", p.propertyId).eq("key", "po"))
      .unique();
    const value = (counter?.value ?? 0) + 1;
    if (counter) await ctx.db.patch(counter._id, { value });
    else await ctx.db.insert("counters", { propertyId: p.propertyId, key: "po", value });

    const supplierId = item.supplierId;
    if (!supplierId) {
      // No supplier on file — still raise the alert for purchasing to action.
      await emit(ctx, {
        propertyId: p.propertyId,
        type: EVENTS.stockLow,
        source: "inventory",
        entityId: item.sku,
        payload: { itemId: p.itemId, sku: item.sku, qtyOnHand: p.qtyOnHand, reorderLevel: p.reorderLevel, suggestedQty: targetQty, autoPoCreated: false },
      });
      return;
    }

    await ctx.db.insert("purchaseOrders", {
      propertyId: p.propertyId,
      poNo: `PO-${String(value).padStart(5, "0")}`,
      supplierId,
      status: "draft",
      items: [{ itemId: p.itemId, name: item.name, qty: targetQty, unitKobo: item.unitCostKobo }],
      totalKobo: Math.round(targetQty * item.unitCostKobo),
      createdBy: "auto-reorder",
      expectedAt: undefined,
    });
    await emit(ctx, {
      propertyId: p.propertyId,
      type: EVENTS.stockLow,
      source: "inventory",
      entityId: item.sku,
      payload: { itemId: p.itemId, sku: item.sku, qtyOnHand: p.qtyOnHand, reorderLevel: p.reorderLevel, suggestedQty: targetQty, autoPoCreated: true },
    });
  },
});

// ── Stock movements (console) ────────────────────────────────────────────
export const receiveStock = mutation({
  args: { token: v.string(), itemId: v.id("inventoryItems"), qty: v.number(), ref: v.optional(v.string()), unitCostKobo: v.optional(v.number()) },
  handler: async (ctx, { token, itemId, qty, ref, unitCostKobo }) => {
    const auth = await requireAuth(ctx, token, "inv.adjust");
    if (!(qty > 0)) throw new ConvexError("Quantity must be positive");
    const item = (await ctx.db.get(itemId)) as Doc<"inventoryItems"> | null;
    if (!item || item.propertyId !== auth.propertyId) throw new ConvexError("Item not found");
    // Weighted-average cost update when a new cost is supplied.
    if (unitCostKobo !== undefined && unitCostKobo > 0) {
      const totalCost = Math.round(item.qtyOnHand * item.unitCostKobo + qty * unitCostKobo);
      const newQty = item.qtyOnHand + qty;
      await ctx.db.patch(itemId, { unitCostKobo: Math.round(totalCost / Math.max(1, newQty)) });
    }
    const qtyOnHand = await applyMovement(ctx, { propertyId: auth.propertyId, itemId, type: "receipt", qty, ref, by: auth.name });
    return { ok: true as const, qtyOnHand };
  },
});

export const issueStock = mutation({
  args: { token: v.string(), itemId: v.id("inventoryItems"), qty: v.number(), reason: v.optional(v.string()) },
  handler: async (ctx, { token, itemId, qty, reason }) => {
    const auth = await requireAuth(ctx, token, "inv.adjust");
    const qtyOnHand = await applyMovement(ctx, { propertyId: auth.propertyId, itemId, type: "issue", qty, reason, by: auth.name });
    return { ok: true as const, qtyOnHand };
  },
});

export const wasteStock = mutation({
  args: { token: v.string(), itemId: v.id("inventoryItems"), qty: v.number(), reason: v.string() },
  handler: async (ctx, { token, itemId, qty, reason }) => {
    const auth = await requireAuth(ctx, token, "inv.adjust");
    const qtyOnHand = await applyMovement(ctx, { propertyId: auth.propertyId, itemId, type: "waste", qty, reason, by: auth.name });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "inv.waste_recorded",
      entity: "inventoryItem",
      entityId: itemId as string,
      severity: "warning",
      details: { qty, reason },
      at: Date.now(),
    });
    return { ok: true as const, qtyOnHand };
  },
});

export const adjustStock = mutation({
  args: { token: v.string(), itemId: v.id("inventoryItems"), delta: v.number(), reason: v.string() },
  handler: async (ctx, { token, itemId, reason, delta }) => {
    const auth = await requireAuth(ctx, token, "inv.adjust");
    if (delta === 0) throw new ConvexError("Delta must be non-zero");
    const qtyOnHand = await applyMovement(ctx, { propertyId: auth.propertyId, itemId, type: "adjustment", qty: delta, reason, by: auth.name });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "inv.adjusted",
      entity: "inventoryItem",
      entityId: itemId as string,
      severity: "warning",
      details: { delta, reason, qtyOnHand },
      at: Date.now(),
    });
    return { ok: true as const, qtyOnHand };
  },
});

// ── Purchase orders ──────────────────────────────────────────────────────
export const createPo = mutation({
  args: {
    token: v.string(),
    supplierId: v.id("suppliers"),
    items: v.array(v.object({ itemId: v.id("inventoryItems"), qty: v.number(), unitKobo: v.number() })),
    expectedAt: v.optional(v.string()),
  },
  handler: async (ctx, { token, supplierId, items, expectedAt }) => {
    const auth = await requireAuth(ctx, token, "inv.po.create");
    if (!items.length) throw new ConvexError("PO needs at least one line");
    const supplier = (await ctx.db.get(supplierId)) as Doc<"suppliers"> | null;
    if (!supplier || supplier.propertyId !== auth.propertyId) throw new ConvexError("Supplier not found");

    const lines = [];
    for (const li of items) {
      const item = (await ctx.db.get(li.itemId)) as Doc<"inventoryItems"> | null;
      if (!item || item.propertyId !== auth.propertyId) throw new ConvexError("Item not found");
      if (!(li.qty > 0)) throw new ConvexError("Quantity must be positive");
      lines.push({ itemId: li.itemId, name: item.name, qty: li.qty, unitKobo: li.unitKobo });
    }
    const counter = await ctx.db
      .query("counters")
      .withIndex("by_property_key", (q) => q.eq("propertyId", auth.propertyId).eq("key", "po"))
      .unique();
    const value = (counter?.value ?? 0) + 1;
    if (counter) await ctx.db.patch(counter._id, { value });
    else await ctx.db.insert("counters", { propertyId: auth.propertyId, key: "po", value });

    const poNo = `PO-${String(value).padStart(5, "0")}`;
    const poId = await ctx.db.insert("purchaseOrders", {
      propertyId: auth.propertyId,
      poNo,
      supplierId,
      status: "draft",
      items: lines,
      totalKobo: lines.reduce((s, li) => s + li.qty * li.unitKobo, 0),
      createdBy: auth.name,
      expectedAt,
    });
    return { ok: true as const, poId: poId as string, poNo };
  },
});

export const approvePo = mutation({
  args: { token: v.string(), poId: v.id("purchaseOrders") },
  handler: async (ctx, { token, poId }) => {
    const auth = await requireAuth(ctx, token, "inv.po.approve");
    const po = (await ctx.db.get(poId)) as Doc<"purchaseOrders"> | null;
    if (!po || po.propertyId !== auth.propertyId) throw new ConvexError("PO not found");
    if (po.status !== "draft") throw new ConvexError(`Cannot approve a PO in status "${po.status}"`);
    await ctx.db.patch(poId, { status: "sent", orderedAt: Date.now() });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "prc.po_approved",
      entity: "purchaseOrder",
      entityId: po.poNo,
      severity: "info",
      details: { totalKobo: po.totalKobo, lines: po.items.length },
      at: Date.now(),
    });
    return { ok: true as const };
  },
});

export const receivePo = mutation({
  args: {
    token: v.string(),
    poId: v.id("purchaseOrders"),
    receipts: v.array(v.object({ itemId: v.id("inventoryItems"), qty: v.number() })),
  },
  handler: async (ctx, { token, poId, receipts }) => {
    const auth = await requireAuth(ctx, token, "inv.po.receive");
    const property = await requireProperty(ctx);
    const po = (await ctx.db.get(poId)) as Doc<"purchaseOrders"> | null;
    if (!po || po.propertyId !== auth.propertyId) throw new ConvexError("PO not found");
    if (po.status !== "sent" && po.status !== "partial") throw new ConvexError(`Cannot receive a PO in status "${po.status}"`);
    if (!receipts.length) throw new ConvexError("No receipt lines supplied");

    let receivedValueKobo = 0;
    for (const r of receipts) {
      const line = po.items.find((li) => li.itemId === r.itemId);
      if (!line) throw new ConvexError("Item is not on this purchase order");
      if (!(r.qty > 0)) throw new ConvexError("Quantity must be positive");
      const today = todayPropertyLocal(property.timezone);
      await applyMovement(ctx, {
        propertyId: auth.propertyId,
        itemId: r.itemId,
        type: "receipt",
        qty: r.qty,
        ref: po.poNo,
        by: auth.name,
        date: today,
      });
      receivedValueKobo += Math.round(r.qty * line.unitKobo);
    }

    // Fully received when cumulative receipts cover every line.
    const movements = await ctx.db.query("stockMovements").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const fullyReceived = po.items.every((li) => {
      const received = movements.filter((m) => m.itemId === li.itemId && m.ref === po.poNo && m.type === "receipt").reduce((s, m) => s + m.qty, 0);
      return received >= li.qty;
    });
    await ctx.db.patch(poId, {
      status: fullyReceived ? "received" : "partial",
      receivedAt: fullyReceived ? Date.now() : po.receivedAt,
    });
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.poReceived,
      source: "procurement",
      entityId: po.poNo,
      payload: { poId, receivedValueKobo, fullyReceived },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "prc.po_received",
      entity: "purchaseOrder",
      entityId: po.poNo,
      severity: "info",
      details: { receivedValueKobo, fullyReceived },
      at: Date.now(),
    });
    return { ok: true as const, fullyReceived, receivedValueKobo };
  },
});

// ── Reads ────────────────────────────────────────────────────────────────
export const stock = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "inv.view");
    const [items, suppliers, movements] = await Promise.all([
      ctx.db.query("inventoryItems").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("suppliers").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
      ctx.db.query("stockMovements").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect(),
    ]);
    const lowStock = items.filter((i) => i.qtyOnHand <= i.reorderLevel);
    return {
      items,
      suppliers,
      lowStock,
      stockValueKobo: items.reduce((s, i) => s + i.qtyOnHand * i.unitCostKobo, 0),
      recentMovements: movements.sort((a, b) => b.at - a.at).slice(0, 50),
    };
  },
});

export const purchaseOrders = query({
  args: { token: v.string(), status: v.optional(v.string()) },
  handler: async (ctx, { token, status }) => {
    const auth = await requireAuth(ctx, token, "inv.view");
    const rows = await ctx.db
      .query("purchaseOrders")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect();
    const filtered = status ? rows.filter((r) => r.status === status) : rows;
    const suppliers = await ctx.db.query("suppliers").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const supplierById = new Map(suppliers.map((s) => [s._id, s.name]));
    return filtered
      .sort((a, b) => b.poNo.localeCompare(a.poNo))
      .map((po) => ({ ...po, supplierName: supplierById.get(po.supplierId) ?? "?" }));
  },
});
