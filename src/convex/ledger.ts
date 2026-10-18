// M-FIN — double-entry general ledger.
//
// Every module posts money through this engine (ADR-0006: "money always
// posts through M-FIN"). Journals are validated for balance, chart membership
// and period locks before a single line is written. Folio charges, POS
// settlement, stock receipts, payroll runs and payment captures all land
// here — the trial balance is therefore always derived, never hand-maintained.

import { v, ConvexError } from "convex/values";
import { mutation, query, internalMutation, type MutationCtx, type QueryCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { requireProperty } from "./lib";
import { requireAuth } from "./authz";

// ── Standard hotel chart of accounts ─────────────────────────────────────
export const STANDARD_COA: Array<{ code: string; name: string; type: string; subtype: string }> = [
  { code: "1000", name: "Cash on Hand", type: "asset", subtype: "cash" },
  { code: "1010", name: "Bank Account — Current", type: "asset", subtype: "cash" },
  { code: "1100", name: "Guest Ledger (Receivable)", type: "asset", subtype: "receivable" },
  { code: "1110", name: "City Ledger (Receivable)", type: "asset", subtype: "receivable" },
  { code: "1200", name: "F&B Inventory", type: "asset", subtype: "inventory" },
  { code: "1210", name: "Supplies Inventory", type: "asset", subtype: "inventory" },
  { code: "1500", name: "Furniture, Fixtures & Equipment", type: "asset", subtype: "fixed" },
  { code: "2000", name: "Accounts Payable", type: "liability", subtype: "payable" },
  { code: "2100", name: "VAT Payable", type: "liability", subtype: "tax" },
  { code: "2110", name: "Service Charge Pool", type: "liability", subtype: "accrued" },
  { code: "2120", name: "Payroll Payable", type: "liability", subtype: "accrued" },
  { code: "2130", name: "Statutory Deductions Payable", type: "liability", subtype: "tax" },
  { code: "2200", name: "Guest Deposits Held", type: "liability", subtype: "deferred" },
  { code: "3000", name: "Owner's Equity", type: "equity", subtype: "capital" },
  { code: "4000", name: "Room Revenue", type: "revenue", subtype: "rooms" },
  { code: "4010", name: "Food & Beverage Revenue", type: "revenue", subtype: "fnb" },
  { code: "4020", name: "Events & Banqueting Revenue", type: "revenue", subtype: "events" },
  { code: "4030", name: "Other Operating Revenue", type: "revenue", subtype: "other" },
  { code: "4040", name: "Discounts & Allowances", type: "revenue", subtype: "contra" },
  { code: "5000", name: "Salaries & Wages", type: "expense", subtype: "payroll" },
  { code: "5010", name: "Employee Benefits", type: "expense", subtype: "payroll" },
  { code: "5100", name: "Utilities", type: "expense", subtype: "operating" },
  { code: "5110", name: "Repairs & Maintenance", type: "expense", subtype: "operating" },
  { code: "5120", name: "Housekeeping & Operating Supplies", type: "expense", subtype: "operating" },
  { code: "5200", name: "Cost of Food Sold", type: "expense", subtype: "cost" },
  { code: "5300", name: "Sales & Marketing", type: "expense", subtype: "operating" },
  { code: "5310", name: "OTA Commission Expense", type: "expense", subtype: "distribution" },
  { code: "5500", name: "Administrative & General", type: "expense", subtype: "operating" },
];

const COA_BY_CODE = new Map(STANDARD_COA.map((a) => [a.code, a]));

export const ACCOUNTS = {
  cash: "1000",
  bank: "1010",
  guestLedger: "1100",
  cityLedger: "1110",
  fbInventory: "1200",
  suppliesInventory: "1210",
  accountsPayable: "2000",
  vatPayable: "2100",
  serviceCharge: "2110",
  payrollPayable: "2120",
  statutoryDeductions: "2130",
  roomRevenue: "4000",
  fbRevenue: "4010",
  eventsRevenue: "4020",
  otherRevenue: "4030",
  discounts: "4040",
  salaries: "5000",
  benefits: "5010",
  utilities: "5100",
  repairs: "5110",
  operatingSupplies: "5120",
  costOfFood: "5200",
  marketing: "5300",
  otaCommission: "5310",
  adminGeneral: "5500",
} as const;

/** Debit-side account for a settlement method. */
export function paymentAccount(method: string): string {
  return method === "cash" ? ACCOUNTS.cash : ACCOUNTS.bank;
}

/** Create any missing standard accounts for the property (idempotent). */
export async function ensureChartOfAccounts(ctx: MutationCtx, propertyId: Id<"properties">): Promise<void> {
  const rows = await ctx.db.query("glAccounts").withIndex("by_property", (q) => q.eq("propertyId", propertyId)).collect();
  const have = new Set(rows.map((r) => r.code));
  for (const account of STANDARD_COA) {
    if (!have.has(account.code)) await ctx.db.insert("glAccounts", { propertyId, ...account });
  }
}

export interface JournalLine {
  accountCode: string;
  debitKobo: number;
  creditKobo: number;
  description: string;
  ref?: string;
}

export interface JournalInput {
  propertyId: Id<"properties">;
  date: string; // business date (YYYY-MM-DD)
  lines: JournalLine[];
  source: string; // folio | pos | night_audit | inventory | payroll | payment | manual
  ref?: string;
  by?: string;
  allowClosedPeriod?: boolean;
}

async function assertPeriodOpen(ctx: QueryCtx | MutationCtx, propertyId: Id<"properties">, date: string, allowClosed: boolean): Promise<void> {
  if (allowClosed) return;
  const lock = await ctx.db
    .query("settings")
    .withIndex("by_property_key", (q) => q.eq("propertyId", propertyId).eq("key", "gl.locked_through"))
    .unique();
  if (lock && date <= lock.value) {
    throw new ConvexError(`Ledger period closed through ${lock.value} — ${date} is locked`);
  }
}

/**
 * Post a balanced journal. Throws before writing anything if unbalanced,
 * unknown-account or period-locked — the whole call is one transaction.
 */
export async function postJournal(ctx: MutationCtx, input: JournalInput): Promise<number> {
  const { propertyId, date, lines, source, ref, by, allowClosedPeriod = false } = input;
  if (!lines.length) throw new ConvexError("Journal has no lines");
  let debit = 0;
  let credit = 0;
  for (const line of lines) {
    if (line.debitKobo < 0 || line.creditKobo < 0) throw new ConvexError("Journal lines must be non-negative");
    if (line.debitKobo > 0 && line.creditKobo > 0) throw new ConvexError("A journal line cannot debit and credit simultaneously");
    if (!COA_BY_CODE.has(line.accountCode)) throw new ConvexError(`Unknown account code "${line.accountCode}"`);
    debit += line.debitKobo;
    credit += line.creditKobo;
  }
  if (debit !== credit) throw new ConvexError(`Unbalanced journal — debits ${debit} ≠ credits ${credit}`);
  if (debit <= 0) throw new ConvexError("Journal must move a positive amount");
  await assertPeriodOpen(ctx, propertyId, date, allowClosedPeriod);
  await ensureChartOfAccounts(ctx, propertyId);

  const now = Date.now();
  for (const line of lines) {
    await ctx.db.insert("glEntries", {
      propertyId,
      date,
      accountCode: line.accountCode,
      debitKobo: line.debitKobo,
      creditKobo: line.creditKobo,
      description: line.description,
      ref: line.ref ?? ref,
      source,
      at: now,
      by,
    });
  }
  return lines.length;
}

/** Two-sided convenience wrapper (Dr one account, Cr another). */
export async function postSimple(
  ctx: MutationCtx,
  input: Omit<JournalInput, "lines"> & { debit: string; credit: string; amountKobo: number; description: string },
): Promise<number> {
  const { debit, credit, amountKobo, description, ...rest } = input;
  return postJournal(ctx, {
    ...rest,
    lines: [
      { accountCode: debit, debitKobo: amountKobo, creditKobo: 0, description },
      { accountCode: credit, debitKobo: 0, creditKobo: amountKobo, description },
    ],
  });
}

// ── payment.captured subscriber: cash/bank Dr, Guest Ledger Cr ───────────
export const onPaymentCaptured = internalMutation({
  args: { payload: v.any() },
  handler: async (ctx, { payload }) => {
    const { propertyId, date, method, amountKobo, reference, description } = payload as {
      propertyId: Id<"properties">;
      date: string;
      method: string;
      amountKobo: number;
      reference: string;
      description?: string;
    };
    if (!propertyId || !date || !(amountKobo > 0)) return;
    await postSimple(ctx, {
      propertyId,
      date,
      debit: paymentAccount(method),
      credit: ACCOUNTS.guestLedger,
      amountKobo,
      description: description ?? `Payment received — ${reference}`,
      ref: reference,
      source: "payment",
    });
  },
});

// ── Reporting ────────────────────────────────────────────────────────────
export interface TrialBalanceRow {
  accountCode: string;
  name: string;
  type: string;
  debitKobo: number;
  creditKobo: number;
  balanceKobo: number;
}

async function collectEntries(
  ctx: QueryCtx,
  propertyId: Id<"properties">,
  from?: string,
  to?: string,
): Promise<Array<Pick<Doc<"glEntries">, "accountCode" | "debitKobo" | "creditKobo" | "date">>> {
  const all = from && to
    ? await ctx.db
        .query("glEntries")
        .withIndex("by_property_date", (q) => q.eq("propertyId", propertyId).gte("date", from).lte("date", to))
        .collect()
    : await ctx.db.query("glEntries").withIndex("by_property", (q) => q.eq("propertyId", propertyId)).collect();
  return all;
}

export const trialBalance = query({
  args: { token: v.string(), from: v.optional(v.string()), to: v.optional(v.string()) },
  handler: async (ctx, { token, from, to }): Promise<{ rows: TrialBalanceRow[]; totalDebitKobo: number; totalCreditKobo: number }> => {
    const auth = await requireAuth(ctx, token, "fin.gl.view");
    const accounts = await ctx.db.query("glAccounts").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const entries = await collectEntries(ctx, auth.propertyId, from, to);
    const byCode = new Map<string, { debit: number; credit: number }>();
    for (const e of entries) {
      const agg = byCode.get(e.accountCode) ?? { debit: 0, credit: 0 };
      agg.debit += e.debitKobo;
      agg.credit += e.creditKobo;
      byCode.set(e.accountCode, agg);
    }
    const rows: TrialBalanceRow[] = accounts
      .map((a) => {
        const agg = byCode.get(a.code) ?? { debit: 0, credit: 0 };
        const debitFirst = a.type === "asset" || a.type === "expense";
        return {
          accountCode: a.code,
          name: a.name,
          type: a.type,
          debitKobo: agg.debit,
          creditKobo: agg.credit,
          balanceKobo: debitFirst ? agg.debit - agg.credit : agg.credit - agg.debit,
        };
      })
      .sort((x, y) => x.accountCode.localeCompare(y.accountCode));
    return {
      rows,
      totalDebitKobo: rows.reduce((s, r) => s + r.debitKobo, 0),
      totalCreditKobo: rows.reduce((s, r) => s + r.creditKobo, 0),
    };
  },
});

export const profitAndLoss = query({
  args: { token: v.string(), from: v.string(), to: v.string() },
  handler: async (ctx, { token, from, to }) => {
    const auth = await requireAuth(ctx, token, "fin.reports.view");
    const accounts = await ctx.db.query("glAccounts").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const entries = await collectEntries(ctx, auth.propertyId, from, to);
    const byCode = new Map<string, { debit: number; credit: number }>();
    for (const e of entries) {
      const agg = byCode.get(e.accountCode) ?? { debit: 0, credit: 0 };
      agg.debit += e.debitKobo;
      agg.credit += e.creditKobo;
      byCode.set(e.accountCode, agg);
    }
    const rows = accounts
      .filter((a) => a.type === "revenue" || a.type === "expense")
      .map((a) => {
        const agg = byCode.get(a.code) ?? { debit: 0, credit: 0 };
        const amountKobo = a.type === "revenue" ? agg.credit - agg.debit : agg.debit - agg.credit;
        return { accountCode: a.code, name: a.name, type: a.type, amountKobo };
      })
      .filter((r) => r.amountKobo !== 0)
      .sort((x, y) => x.accountCode.localeCompare(y.accountCode));
    const revenueKobo = rows.filter((r) => r.type === "revenue").reduce((s, r) => s + r.amountKobo, 0);
    const expenseKobo = rows.filter((r) => r.type === "expense").reduce((s, r) => s + r.amountKobo, 0);
    return { rows, revenueKobo, expenseKobo, netKobo: revenueKobo - expenseKobo };
  },
});

export const accountLedger = query({
  args: { token: v.string(), accountCode: v.string(), from: v.optional(v.string()), to: v.optional(v.string()) },
  handler: async (ctx, { token, accountCode, from, to }) => {
    const auth = await requireAuth(ctx, token, "fin.gl.view");
    const entries = await collectEntries(ctx, auth.propertyId, from, to);
    return entries.filter((e) => e.accountCode === accountCode);
  },
});

export const listAccounts = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "fin.gl.view");
    const accounts = await ctx.db.query("glAccounts").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    return accounts.sort((a, b) => a.code.localeCompare(b.code));
  },
});

/** Manual journal entry from the Finance console (balanced, period-checked). */
export const postManual = mutation({
  args: {
    token: v.string(),
    date: v.string(),
    lines: v.array(
      v.object({
        accountCode: v.string(),
        debitKobo: v.number(),
        creditKobo: v.number(),
        description: v.string(),
      }),
    ),
  },
  handler: async (ctx, { token, date, lines }) => {
    const auth = await requireAuth(ctx, token, "fin.gl.post");
    const property = await requireProperty(ctx);
    const count = await postJournal(ctx, {
      propertyId: auth.propertyId,
      date,
      lines,
      source: "manual",
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "gl.manual_posted",
      entity: "glEntry",
      entityId: date,
      severity: "warning",
      details: { lines: count, currency: property.currency },
      at: Date.now(),
    });
    return { ok: true as const, lines: count };
  },
});
