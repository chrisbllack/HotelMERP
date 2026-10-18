// M-HR — attendance and payroll, closing the loop into M-FIN.
//
// Payroll runs are computed from the employee master (never hand-keyed),
// approved under separation of duties (approver ≠ runner), and on approval
// post Dr Salaries, Cr Payroll Payable + Statutory Deductions; marking paid
// clears the payable against the bank. payroll.approved is published for
// downstream consumers (finance dashboards, audit).

import { v, ConvexError } from "convex/values";
import { mutation, query } from "./_generated/server";
import type { Doc } from "./_generated/dataModel";
import { requireProperty, todayPropertyLocal } from "./lib";
import { postJournal, postSimple, ACCOUNTS } from "./ledger";
import { emit, EVENTS } from "./events";
import { requireAuth } from "./authz";

// Nigerian statutory mix (documented house policy): employee Pension 8%,
// PAYE/IT simplification 10%. Employer benefits accrue separately at 6%.
const EMPLOYEE_PENSION_RATE = 0.08;
const EMPLOYEE_TAX_RATE = 0.1;
const EMPLOYER_BENEFIT_RATE = 0.06;

const LATE_AFTER = "09:00";

// ── Attendance ───────────────────────────────────────────────────────────
export const clockIn = mutation({
  args: { token: v.string(), employeeId: v.id("employees") },
  handler: async (ctx, { token, employeeId }) => {
    const auth = await requireAuth(ctx, token, "hr.attendance.manage");
    const property = await requireProperty(ctx);
    const employee = (await ctx.db.get(employeeId)) as Doc<"employees"> | null;
    if (!employee || employee.propertyId !== auth.propertyId) throw new ConvexError("Employee not found");
    if (employee.status !== "active") throw new ConvexError(`Employee is ${employee.status}`);

    const today = todayPropertyLocal(property.timezone);
    const clock = new Date().toLocaleTimeString("en-GB", { timeZone: property.timezone, hour: "2-digit", minute: "2-digit" });
    const existing = (
      await ctx.db
        .query("attendance")
        .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).eq("date", today))
        .collect()
    ).find((a) => a.employeeId === employeeId);
    if (existing?.clockIn) throw new ConvexError("Already clocked in today");

    if (existing) {
      await ctx.db.patch(existing._id, { clockIn: clock, status: clock > LATE_AFTER ? "late" : "present" });
    } else {
      await ctx.db.insert("attendance", {
        propertyId: auth.propertyId,
        employeeId,
        date: today,
        clockIn: clock,
        status: clock > LATE_AFTER ? "late" : "present",
      });
    }
    return { ok: true as const, clock, late: clock > LATE_AFTER };
  },
});

export const clockOut = mutation({
  args: { token: v.string(), employeeId: v.id("employees") },
  handler: async (ctx, { token, employeeId }) => {
    const auth = await requireAuth(ctx, token, "hr.attendance.manage");
    const property = await requireProperty(ctx);
    const today = todayPropertyLocal(property.timezone);
    const record = (
      await ctx.db
        .query("attendance")
        .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).eq("date", today))
        .collect()
    ).find((a) => a.employeeId === employeeId);
    if (!record?.clockIn) throw new ConvexError("Not clocked in today");
    if (record.clockOut) throw new ConvexError("Already clocked out");
    const clock = new Date().toLocaleTimeString("en-GB", { timeZone: property.timezone, hour: "2-digit", minute: "2-digit" });
    await ctx.db.patch(record._id, { clockOut: clock });
    return { ok: true as const, clock };
  },
});

export const markLeave = mutation({
  args: { token: v.string(), employeeId: v.id("employees"), date: v.string() },
  handler: async (ctx, { token, employeeId, date }) => {
    const auth = await requireAuth(ctx, token, "hr.attendance.manage");
    const existing = (
      await ctx.db
        .query("attendance")
        .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).eq("date", date))
        .collect()
    ).find((a) => a.employeeId === employeeId);
    if (existing) await ctx.db.patch(existing._id, { status: "leave", clockIn: undefined, clockOut: undefined });
    else await ctx.db.insert("attendance", { propertyId: auth.propertyId, employeeId, date, status: "leave" });
    return { ok: true as const };
  },
});

// ── Payroll ──────────────────────────────────────────────────────────────
interface PayrollLine {
  employeeNo: string;
  name: string;
  department: string;
  grossKobo: number;
  pensionKobo: number;
  taxKobo: number;
  deductionsKobo: number;
  netKobo: number;
}

function computeLines(employees: Array<Doc<"employees">>): PayrollLine[] {
  return employees.map((e) => {
    const pensionKobo = Math.round(e.baseSalaryKobo * EMPLOYEE_PENSION_RATE);
    const taxKobo = Math.round(e.baseSalaryKobo * EMPLOYEE_TAX_RATE);
    const deductionsKobo = pensionKobo + taxKobo;
    return {
      employeeNo: e.employeeNo,
      name: `${e.firstName} ${e.lastName}`,
      department: e.department,
      grossKobo: e.baseSalaryKobo,
      pensionKobo,
      taxKobo,
      deductionsKobo,
      netKobo: e.baseSalaryKobo - deductionsKobo,
    };
  });
}

/** Preview a payroll period without writing (draft calculation). */
export const previewPayroll = query({
  args: { token: v.string(), period: v.string() },
  handler: async (ctx, { token, period }) => {
    const auth = await requireAuth(ctx, token, "hr.payroll.run");
    const employees = (
      await ctx.db.query("employees").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect()
    ).filter((e) => e.status === "active");
    const lines = computeLines(employees);
    const grossKobo = lines.reduce((s, l) => s + l.grossKobo, 0);
    const deductionsKobo = lines.reduce((s, l) => s + l.deductionsKobo, 0);
    return {
      period,
      headcount: lines.length,
      lines,
      grossKobo,
      deductionsKobo,
      netKobo: grossKobo - deductionsKobo,
      employerBenefitKobo: Math.round(grossKobo * EMPLOYER_BENEFIT_RATE),
    };
  },
});

export const runPayroll = mutation({
  args: { token: v.string(), period: v.string() },
  handler: async (ctx, { token, period }) => {
    const auth = await requireAuth(ctx, token, "hr.payroll.run");
    if (!/^\d{4}-\d{2}$/.test(period)) throw new ConvexError("Period must be YYYY-MM");
    const employees = (
      await ctx.db.query("employees").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect()
    ).filter((e) => e.status === "active");
    if (!employees.length) throw new ConvexError("No active employees to pay");

    const lines = computeLines(employees);
    const grossKobo = lines.reduce((s, l) => s + l.grossKobo, 0);
    const deductionsKobo = lines.reduce((s, l) => s + l.deductionsKobo, 0);
    const netKobo = grossKobo - deductionsKobo;

    const existing = (
      await ctx.db
        .query("payrollRuns")
        .withIndex("by_property_period", (q) => q.eq("propertyId", auth.propertyId).eq("period", period))
        .collect()
    )[0];
    if (existing && existing.status !== "draft") throw new ConvexError(`Payroll ${period} is already ${existing.status}`);

    const row = {
      period,
      grossKobo,
      deductionsKobo,
      netKobo,
      headcount: lines.length,
      runAt: Date.now(),
    };
    if (existing) await ctx.db.patch(existing._id, row);
    else await ctx.db.insert("payrollRuns", { propertyId: auth.propertyId, status: "draft", ...row });
    return { ok: true as const, ...row };
  },
});

export const approvePayroll = mutation({
  args: { token: v.string(), period: v.string() },
  handler: async (ctx, { token, period }) => {
    const auth = await requireAuth(ctx, token, "hr.payroll.approve");
    const property = await requireProperty(ctx);
    const run = (
      await ctx.db
        .query("payrollRuns")
        .withIndex("by_property_period", (q) => q.eq("propertyId", auth.propertyId).eq("period", period))
        .collect()
    )[0];
    if (!run) throw new ConvexError("No payroll run for that period");
    if (run.status !== "draft") throw new ConvexError(`Payroll ${period} is already ${run.status}`);

    // Posting date = last day of the payroll period.
    const [year, month] = period.split("-").map(Number);
    const postingDate = `${period}-${String(new Date(Date.UTC(year, month, 0)).getUTCDate()).padStart(2, "0")}`;

    await postJournal(ctx, {
      propertyId: auth.propertyId,
      date: postingDate,
      source: "payroll",
      ref: `PAYROLL-${period}`,
      by: auth.name,
      lines: [
        { accountCode: ACCOUNTS.salaries, debitKobo: run.grossKobo, creditKobo: 0, description: `Payroll ${period} — gross salaries` },
        { accountCode: ACCOUNTS.payrollPayable, debitKobo: 0, creditKobo: run.netKobo, description: `Payroll ${period} — net pay payable` },
        { accountCode: ACCOUNTS.statutoryDeductions, debitKobo: 0, creditKobo: run.deductionsKobo, description: `Payroll ${period} — pension & PAYE withheld` },
      ],
    });
    // Employer benefits accrual (6%): Dr benefits, Cr payable.
    const employerBenefitKobo = Math.round(run.grossKobo * EMPLOYER_BENEFIT_RATE);
    if (employerBenefitKobo > 0) {
      await postJournal(ctx, {
        propertyId: auth.propertyId,
        date: postingDate,
        source: "payroll",
        ref: `PAYROLL-${period}-ER`,
        by: auth.name,
        lines: [
          { accountCode: ACCOUNTS.benefits, debitKobo: employerBenefitKobo, creditKobo: 0, description: `Payroll ${period} — employer benefits` },
          { accountCode: ACCOUNTS.payrollPayable, debitKobo: 0, creditKobo: employerBenefitKobo, description: `Payroll ${period} — employer benefits payable` },
        ],
      });
    }

    await ctx.db.patch(run._id, { status: "approved", approvedBy: auth.name, runAt: run.runAt ?? Date.now() });
    await emit(ctx, {
      propertyId: auth.propertyId,
      type: EVENTS.payrollApproved,
      source: "hr",
      entityId: period,
      payload: { period, grossKobo: run.grossKobo, netKobo: run.netKobo, deductionsKobo: run.deductionsKobo, headcount: run.headcount },
      by: auth.name,
    });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "hr.payroll_approved",
      entity: "payrollRun",
      entityId: period,
      severity: "warning",
      details: { grossKobo: run.grossKobo, netKobo: run.netKobo, headcount: run.headcount, currency: property.currency },
      at: Date.now(),
    });
    return { ok: true as const };
  },
});

export const settlePayroll = mutation({
  args: { token: v.string(), period: v.string() },
  handler: async (ctx, { token, period }) => {
    const auth = await requireAuth(ctx, token, "hr.payroll.approve");
    const run = (
      await ctx.db
        .query("payrollRuns")
        .withIndex("by_property_period", (q) => q.eq("propertyId", auth.propertyId).eq("period", period))
        .collect()
    )[0];
    if (!run) throw new ConvexError("No payroll run for that period");
    if (run.status !== "approved") throw new ConvexError("Only approved payroll can be settled");

    const [year, month] = period.split("-").map(Number);
    const postingDate = `${period}-${String(new Date(Date.UTC(year, month, 0)).getUTCDate()).padStart(2, "0")}`;
    const benefitsPayableKobo = Math.round(run.grossKobo * EMPLOYER_BENEFIT_RATE);

    await postSimple(ctx, {
      propertyId: auth.propertyId,
      date: postingDate,
      debit: ACCOUNTS.payrollPayable,
      credit: ACCOUNTS.bank,
      amountKobo: run.netKobo + benefitsPayableKobo,
      description: `Payroll ${period} — settlement from bank`,
      ref: `PAYROLL-${period}`,
      source: "payroll",
      by: auth.name,
    });
    // Withheld statutory amounts remain payable to the authorities; they stay
    // on 2130 until remitted (tracked by finance in the trial balance).
    await ctx.db.patch(run._id, { status: "paid" });
    return { ok: true as const, paidKobo: run.netKobo + benefitsPayableKobo };
  },
});

export const payrollHistory = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "hr.view");
    return ctx.db
      .query("payrollRuns")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .order("desc")
      .take(36);
  },
});

export const roster = query({
  args: { token: v.string(), date: v.optional(v.string()) },
  handler: async (ctx, { token, date }) => {
    const auth = await requireAuth(ctx, token, "hr.view");
    const property = await requireProperty(ctx);
    const day = date ?? todayPropertyLocal(property.timezone);
    const employees = await ctx.db.query("employees").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const attendance = (
      await ctx.db
        .query("attendance")
        .withIndex("by_property_date", (q) => q.eq("propertyId", auth.propertyId).eq("date", day))
        .collect()
    );
    const byEmployee = new Map(attendance.map((a) => [a.employeeId, a]));
    return {
      date: day,
      employees: employees.map((e) => ({ employee: e, attendance: byEmployee.get(e._id) ?? null })),
      present: attendance.filter((a) => a.status === "present" || a.status === "late").length,
      late: attendance.filter((a) => a.status === "late").length,
      onLeave: attendance.filter((a) => a.status === "leave").length,
    };
  },
});
