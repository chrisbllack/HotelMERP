// M-SEC — identity, sessions and server-side RBAC.
//
// The console authenticates against this module: login issues a bearer token
// (only its SHA-256 is persisted), and every privileged surface calls
// requireAuth(ctx, token, permission) before touching data. Permissions are
// the ADR-0006 permission matrix enforced server-side — the UI merely mirrors
// what the backend already guarantees.

import { v, ConvexError } from "convex/values";
import { mutation, query, type MutationCtx, type QueryCtx } from "./_generated/server";
import { internal } from "./_generated/api";
import type { Doc, Id } from "./_generated/dataModel";
import { requireProperty } from "./lib";

const SESSION_TTL_MS = 24 * 60 * 60 * 1000; // 24h absolute lifetime
const PBKDF2_ITERATIONS = 100_000;
export const DEMO_PASSWORD = "demo1234";

// ── Permission catalogue ─────────────────────────────────────────────────
export const PERMISSIONS = [
  "dashboard.view",
  // M-LOD lodging
  "lodging.reservations.view",
  "lodging.reservations.create",
  "lodging.reservations.modify",
  "lodging.reservations.cancel",
  "lodging.frontdesk.check_in",
  "lodging.frontdesk.check_out",
  "lodging.frontdesk.assign_room",
  "lodging.frontdesk.no_show",
  "lodging.rooms.view",
  "lodging.rooms.edit",
  "lodging.hk.view",
  "lodging.hk.manage",
  "lodging.night_audit.run",
  // M-FNB
  "fnb.pos.view",
  "fnb.pos.order",
  "fnb.pos.void",
  "fnb.pos.settle",
  // M-FIN money
  "fin.folio.view",
  "fin.folio.charge",
  "fin.folio.settle",
  "fin.gl.view",
  "fin.gl.post",
  "fin.reports.view",
  // M-INV / M-PRC
  "inv.view",
  "inv.adjust",
  "inv.po.create",
  "inv.po.approve",
  "inv.po.receive",
  // M-CRM
  "crm.view",
  "crm.manage",
  // M-EVT / M-DST / M-BUS
  "evt.view",
  "evt.manage",
  "dst.view",
  "dst.manage",
  "bus.maintenance.view",
  "bus.maintenance.manage",
  // M-HR
  "hr.view",
  "hr.attendance.manage",
  "hr.payroll.run",
  "hr.payroll.approve",
  // platform
  "sec.admin",
] as const;

export type Permission = (typeof PERMISSIONS)[number];

const ALL: string[] = [...PERMISSIONS];
const ALL_BUT_ADMIN: string[] = ALL.filter((p) => p !== "sec.admin");

/** Server-side role → permission matrix (the single source of truth). */
export const ROLE_PERMISSIONS: Record<string, string[]> = {
  owner: ALL,
  admin: ALL,
  manager: ALL_BUT_ADMIN,
  front_desk: [
    "dashboard.view",
    "lodging.reservations.view",
    "lodging.reservations.create",
    "lodging.reservations.modify",
    "lodging.reservations.cancel",
    "lodging.frontdesk.check_in",
    "lodging.frontdesk.check_out",
    "lodging.frontdesk.assign_room",
    "lodging.frontdesk.no_show",
    "lodging.rooms.view",
    "lodging.hk.view",
    "lodging.night_audit.run",
    "fnb.pos.view",
    "fnb.pos.order",
    "fnb.pos.settle",
    "fin.folio.view",
    "fin.folio.charge",
    "fin.folio.settle",
    "fin.reports.view",
    "crm.view",
    "crm.manage",
    "evt.view",
    "dst.view",
    "inv.view",
    "bus.maintenance.view",
    "bus.maintenance.manage",
  ],
  housekeeping: ["dashboard.view", "lodging.rooms.view", "lodging.hk.view", "lodging.hk.manage", "bus.maintenance.view", "bus.maintenance.manage"],
  fnb: ["dashboard.view", "fnb.pos.view", "fnb.pos.order", "fnb.pos.void", "fnb.pos.settle", "fin.folio.view", "fin.folio.charge", "fin.reports.view", "inv.view", "bus.maintenance.view"],
  finance: [
    "dashboard.view",
    "fin.folio.view",
    "fin.folio.charge",
    "fin.folio.settle",
    "fin.gl.view",
    "fin.gl.post",
    "fin.reports.view",
    "inv.view",
    "inv.adjust",
    "inv.po.create",
    "inv.po.approve",
    "inv.po.receive",
    "hr.view",
    "hr.payroll.run",
    "hr.payroll.approve",
    "crm.view",
    "lodging.reservations.view",
    "lodging.rooms.view",
    "evt.view",
    "dst.view",
  ],
  hr: ["dashboard.view", "hr.view", "hr.attendance.manage", "hr.payroll.run", "hr.payroll.approve", "crm.view"],
  guest: [],
};

export const ROLE_KEYS = Object.keys(ROLE_PERMISSIONS);

export function permissionsFor(roleKey: string): string[] {
  return ROLE_PERMISSIONS[roleKey] ?? [];
}

export function can(roleKey: string, permission: string): boolean {
  return permissionsFor(roleKey).includes(permission);
}

// ── Crypto helpers ───────────────────────────────────────────────────────
function b64(bytes: Uint8Array): string {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

function unB64(s: string): Uint8Array<ArrayBuffer> {
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export async function sha256hex(input: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(input));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

export async function hashPassword(password: string): Promise<string> {
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(password), "PBKDF2", false, ["deriveBits"]);
  const bits = await crypto.subtle.deriveBits(
    { name: "PBKDF2", salt, iterations: PBKDF2_ITERATIONS, hash: "SHA-256" },
    key,
    256,
  );
  return `pbkdf2$${PBKDF2_ITERATIONS}$${b64(salt)}$${b64(new Uint8Array(bits))}`;
}

export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const parts = stored.split("$");
  if (parts.length !== 4 || parts[0] !== "pbkdf2") return false;
  const iterations = Number(parts[1]);
  if (!Number.isFinite(iterations) || iterations <= 0) return false;
  const salt = unB64(parts[2]);
  const expected = unB64(parts[3]);
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(password), "PBKDF2", false, ["deriveBits"]);
  const bits = new Uint8Array(
    await crypto.subtle.deriveBits({ name: "PBKDF2", salt, iterations, hash: "SHA-256" }, key, expected.length * 8),
  );
  if (bits.length !== expected.length) return false;
  let diff = 0;
  for (let i = 0; i < bits.length; i++) diff |= bits[i] ^ expected[i];
  return diff === 0;
}

function newToken(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// ── Guard ────────────────────────────────────────────────────────────────
export interface AuthContext {
  userId: Id<"users">;
  name: string;
  email: string;
  roleKey: string;
  propertyId: Id<"properties">;
  permissions: string[];
}

/**
 * Resolve a bearer session and assert `permission`. Read-safe (no writes) so
 * it can guard queries as well as mutations.
 */
export async function requireAuth(
  ctx: QueryCtx | MutationCtx,
  token: string,
  permission: Permission,
): Promise<AuthContext> {
  if (!token) throw new ConvexError("Authentication required");
  const property = await requireProperty(ctx);
  const tokenHash = await sha256hex(token);
  const session = await ctx.db
    .query("sessions")
    .withIndex("by_token", (q) => q.eq("tokenHash", tokenHash))
    .unique();
  if (!session) throw new ConvexError("Session invalid or expired — sign in again");
  if (session.expiresAt < Date.now()) throw new ConvexError("Session expired — sign in again");
  const user = await ctx.db.get(session.userId);
  if (!user) throw new ConvexError("Session invalid or expired — sign in again");
  const permissions = permissionsFor(session.roleKey);
  if (!permissions.includes(permission)) {
    throw new ConvexError(`Not permitted: ${permission} (role "${session.roleKey}")`);
  }
  return {
    userId: user._id,
    name: user.name,
    email: user.email,
    roleKey: session.roleKey,
    propertyId: property._id,
    permissions,
  };
}

// ── Bootstrap role mapping for the seeded demo accounts ──────────────────
const EMAIL_DEFAULT_ROLE: Record<string, string> = {
  "admin@infineon.com": "owner",
  "gm@infineon.com": "manager",
  "frontdesk@infineon.com": "front_desk",
  "housekeeping@infineon.com": "housekeeping",
  "finance@infineon.com": "finance",
};

async function loadSessionIdentity(
  ctx: QueryCtx | MutationCtx,
  token: string,
): Promise<{ user: Doc<"users">; session: Doc<"sessions">; permissions: string[] } | null> {
  if (!token) return null;
  const tokenHash = await sha256hex(token);
  const session = await ctx.db.query("sessions").withIndex("by_token", (q) => q.eq("tokenHash", tokenHash)).unique();
  if (!session || session.expiresAt < Date.now()) return null;
  const user = await ctx.db.get(session.userId);
  if (!user) return null;
  return { user, session, permissions: permissionsFor(session.roleKey) };
}

// ── Public surface ───────────────────────────────────────────────────────
export const login = mutation({
  args: { email: v.string(), password: v.string() },
  handler: async (ctx, { email, password }) => {
    await ctx.runMutation(internal.lib.ensureSeed, {});
    const normalized = email.trim().toLowerCase();
    let user =
      (await ctx.db.query("users").withIndex("by_email", (q) => q.eq("email", normalized)).unique()) ??
      (await ctx.db.query("users").withIndex("by_email", (q) => q.eq("email", email.trim())).unique());
    if (!user) throw new ConvexError("No account with that email.");

    let authenticated = false;
    if (user.passwordHash) {
      authenticated = await verifyPassword(password, user.passwordHash);
    } else if (password === DEMO_PASSWORD) {
      // First login for a seeded demo account — persist a real hash so the
      // plaintext demo password stops being the source of truth.
      await ctx.db.patch(user._id, { passwordHash: await hashPassword(password) });
      authenticated = true;
    }
    if (!authenticated) throw new ConvexError("Incorrect password.");

    const property = await requireProperty(ctx);
    let profile = (await ctx.db.query("profiles").withIndex("by_user", (q) => q.eq("userId", user._id)).unique()) ?? null;
    if (!profile) {
      const roleKey = EMAIL_DEFAULT_ROLE[user.email.toLowerCase()] ?? "guest";
      await ctx.db.insert("profiles", {
        userId: user._id,
        propertyId: property._id,
        roleKey,
        displayName: user.name,
      });
      profile = await ctx.db.query("profiles").withIndex("by_user", (q) => q.eq("userId", user._id)).unique();
    }

    const roleKey = profile?.roleKey ?? "guest";
    const token = newToken();
    const now = Date.now();
    await ctx.db.insert("sessions", {
      userId: user._id,
      tokenHash: await sha256hex(token),
      roleKey,
      propertyId: property._id,
      createdAt: now,
      expiresAt: now + SESSION_TTL_MS,
      lastSeenAt: now,
    });

    return {
      token,
      userId: user._id as string,
      email: user.email,
      name: user.name,
      displayName: profile?.displayName ?? user.name,
      roleKey,
      permissions: permissionsFor(roleKey),
    };
  },
});

export const logout = mutation({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const tokenHash = await sha256hex(token);
    const session = await ctx.db.query("sessions").withIndex("by_token", (q) => q.eq("tokenHash", tokenHash)).unique();
    if (session) await ctx.db.delete(session._id);
    return { ok: true as const };
  },
});

export const me = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const identity = await loadSessionIdentity(ctx, token);
    if (!identity) return null;
    const profile = (await ctx.db.query("profiles").withIndex("by_user", (q) => q.eq("userId", identity.user._id)).unique()) ?? null;
    return {
      userId: identity.user._id as string,
      email: identity.user.email,
      name: identity.user.name,
      displayName: profile?.displayName ?? identity.user.name,
      roleKey: identity.session.roleKey,
      permissions: identity.permissions,
      expiresAt: identity.session.expiresAt,
    };
  },
});

/** Staff provisioning (sec.admin only) — creates user + profile with role. */
export const createStaff = mutation({
  args: {
    token: v.string(),
    email: v.string(),
    name: v.string(),
    roleKey: v.string(),
    password: v.string(),
    department: v.optional(v.string()),
  },
  handler: async (ctx, { token, email, name, roleKey, password, department }) => {
    const auth = await requireAuth(ctx, token, "sec.admin");
    if (!ROLE_PERMISSIONS[roleKey]) throw new ConvexError(`Unknown role "${roleKey}"`);
    if (password.length < 8) throw new ConvexError("Password must be at least 8 characters");
    const normalized = email.trim().toLowerCase();
    if (await ctx.db.query("users").withIndex("by_email", (q) => q.eq("email", normalized)).unique()) {
      throw new ConvexError("An account with that email already exists");
    }
    const userId = await ctx.db.insert("users", {
      email: normalized,
      name,
      passwordHash: await hashPassword(password),
    });
    await ctx.db.insert("profiles", { userId, propertyId: auth.propertyId, roleKey, displayName: name });
    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "sec.staff_created",
      entity: "user",
      entityId: normalized,
      severity: "warning",
      details: { roleKey, department: department ?? null },
      at: Date.now(),
    });
    return { ok: true as const, userId: userId as string };
  },
});

export const listStaff = query({
  args: { token: v.string() },
  handler: async (ctx, { token }) => {
    const auth = await requireAuth(ctx, token, "sec.admin");
    const profiles = await ctx.db.query("profiles").collect();
    const rows: Array<{ userId: string; email: string; name: string; roleKey: string; displayName: string }> = [];
    for (const p of profiles) {
      if (p.propertyId !== auth.propertyId) continue;
      const u = await ctx.db.get(p.userId);
      if (!u) continue;
      rows.push({ userId: u._id as string, email: u.email, name: u.name, roleKey: p.roleKey, displayName: p.displayName });
    }
    return rows;
  },
});
