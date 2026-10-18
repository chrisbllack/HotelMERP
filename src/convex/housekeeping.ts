// M-LOD housekeeping — task board driving room status.
//
// Interlocks: check-out creates cleaning tasks (stays.ts); completing one
// flips the room back to sellable and publishes room.status_changed; failing
// one opens a maintenance ticket and can take the room out of order.

import { v, ConvexError } from "convex/values";
import { mutation, query, type MutationCtx } from "./_generated/server";
import type { Doc, Id } from "./_generated/dataModel";
import { requireProperty, todayPropertyLocal } from "./lib";
import { emit, EVENTS } from "./events";
import { requireAuth, type AuthContext } from "./authz";

async function loadTask(ctx: MutationCtx, auth: AuthContext, taskId: Id<"hkTasks">): Promise<Doc<"hkTasks">> {
  const task = (await ctx.db.get(taskId)) as Doc<"hkTasks"> | null;
  if (!task || task.propertyId !== auth.propertyId) throw new ConvexError("Task not found");
  return task;
}

async function setRoom(
  ctx: MutationCtx,
  auth: AuthContext,
  roomId: Id<"rooms">,
  patch: Partial<Doc<"rooms">>,
): Promise<Doc<"rooms">> {
  const room = (await ctx.db.get(roomId)) as Doc<"rooms"> | null;
  if (!room) throw new ConvexError("Room not found");
  await ctx.db.patch(roomId, patch);
  await emit(ctx, {
    propertyId: auth.propertyId,
    type: EVENTS.roomStatusChanged,
    source: "housekeeping",
    entityId: room.number,
    payload: { roomId, number: room.number, ...patch, previous: room.status },
    by: auth.name,
  });
  return { ...room, ...patch } as Doc<"rooms">;
}

export const createTask = mutation({
  args: {
    token: v.string(),
    roomId: v.id("rooms"),
    type: v.string(), // cleaning, inspection, turndown, maintenance_check
    priority: v.optional(v.string()),
    notes: v.optional(v.string()),
    scheduledDate: v.optional(v.string()),
  },
  handler: async (ctx, { token, roomId, type, priority, notes, scheduledDate }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.manage");
    const property = await requireProperty(ctx);
    const room = (await ctx.db.get(roomId)) as Doc<"rooms"> | null;
    if (!room || room.propertyId !== auth.propertyId) throw new ConvexError("Room not found");
    const taskId = await ctx.db.insert("hkTasks", {
      propertyId: auth.propertyId,
      roomId,
      type,
      priority: priority ?? "normal",
      status: "pending",
      scheduledDate: scheduledDate ?? todayPropertyLocal(property.timezone),
      notes,
    });
    return { ok: true as const, taskId: taskId as string };
  },
});

export const assign = mutation({
  args: { token: v.string(), taskId: v.id("hkTasks"), attendant: v.string() },
  handler: async (ctx, { token, taskId, attendant }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.manage");
    const task = await loadTask(ctx, auth, taskId);
    if (task.status === "completed" || task.status === "inspected") throw new ConvexError("Task already finished");
    await ctx.db.patch(taskId, { assignedTo: attendant, status: task.status === "pending" ? "in_progress" : task.status });
    return { ok: true as const };
  },
});

export const start = mutation({
  args: { token: v.string(), taskId: v.id("hkTasks") },
  handler: async (ctx, { token, taskId }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.manage");
    const task = await loadTask(ctx, auth, taskId);
    if (task.status !== "pending" && task.status !== "in_progress") {
      throw new ConvexError(`Cannot start from status "${task.status}"`);
    }
    await ctx.db.patch(taskId, { status: "in_progress" });
    return { ok: true as const };
  },
});

export const complete = mutation({
  args: { token: v.string(), taskId: v.id("hkTasks"), notes: v.optional(v.string()) },
  handler: async (ctx, { token, taskId, notes }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.manage");
    const property = await requireProperty(ctx);
    const task = await loadTask(ctx, auth, taskId);
    if (task.status === "completed" || task.status === "inspected") throw new ConvexError("Task already completed");
    const now = Date.now();
    await ctx.db.patch(taskId, { status: "completed", completedAt: now, notes: notes ?? task.notes });

    const room = (await ctx.db.get(task.roomId)) as Doc<"rooms"> | null;
    if (!room) throw new ConvexError("Room not found");

    // Cleaning done → room becomes sellable again (unless out of order).
    if (task.type === "cleaning" && room.status !== "out_of_order") {
      await setRoom(ctx, auth, room._id, {
        hkStatus: "clean",
        status: room.status.startsWith("occupied") ? "occupied_clean" : "vacant_clean",
        lastCleanedAt: now,
      });
    } else if (task.type === "inspection") {
      await ctx.db.patch(taskId, { status: "inspected", inspectedAt: now });
    }

    // Housekeeping found a problem → raise a maintenance ticket for it.
    if (notes && /broken|leak|damage|fault|not working|issue/i.test(notes)) {
      await ctx.db.insert("maintenanceTickets", {
        propertyId: auth.propertyId,
        roomId: room._id,
        category: "other",
        priority: "medium",
        status: "open",
        title: `Housekeeping issue — room ${room.number}`,
        description: notes,
        reportedBy: auth.name,
        reportedAt: now,
      });
    }

    await ctx.db.insert("auditLogs", {
      propertyId: auth.propertyId,
      actorId: auth.userId,
      actorName: auth.name,
      action: "hk.task_completed",
      entity: "hkTask",
      entityId: task._id as string,
      severity: "info",
      details: { room: room.number, type: task.type, completedOn: todayPropertyLocal(property.timezone) },
      at: now,
    });
    return { ok: true as const };
  },
});

export const inspect = mutation({
  args: { token: v.string(), taskId: v.id("hkTasks"), passed: v.boolean(), notes: v.optional(v.string()) },
  handler: async (ctx, { token, taskId, passed, notes }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.manage");
    const task = await loadTask(ctx, auth, taskId);
    if (task.status !== "completed") throw new ConvexError("Only completed tasks can be inspected");
    const now = Date.now();
    if (!passed) {
      // Failed inspection → back to the board as urgent.
      await ctx.db.patch(taskId, { status: "pending", inspectedAt: now, priority: "urgent", notes: notes ?? task.notes });
      return { ok: true as const, passed: false as const };
    }
    await ctx.db.patch(taskId, { status: "inspected", inspectedAt: now, notes: notes ?? task.notes });
    const room = (await ctx.db.get(task.roomId)) as Doc<"rooms"> | null;
    if (room && room.status !== "out_of_order") {
      await setRoom(ctx, auth, room._id, {
        hkStatus: "inspected",
        status: room.status.startsWith("occupied") ? "occupied_clean" : "vacant_clean",
      });
    }
    return { ok: true as const, passed: true as const };
  },
});

/** Take a room out of order (and block inventory immediately). */
export const takeOutOfOrder = mutation({
  args: { token: v.string(), roomId: v.id("rooms"), reason: v.string() },
  handler: async (ctx, { token, roomId, reason }) => {
    const auth = await requireAuth(ctx, token, "lodging.rooms.edit");
    const room = (await ctx.db.get(roomId)) as Doc<"rooms"> | null;
    if (!room || room.propertyId !== auth.propertyId) throw new ConvexError("Room not found");
    if (room.status === "out_of_order") return { ok: true as const };
    await setRoom(ctx, auth, roomId, { status: "out_of_order", oooReason: reason });
    await ctx.db.insert("maintenanceTickets", {
      propertyId: auth.propertyId,
      roomId,
      category: "other",
      priority: "high",
      status: "open",
      title: `Room ${room.number} out of order`,
      description: reason,
      reportedBy: auth.name,
      reportedAt: Date.now(),
    });
    return { ok: true as const };
  },
});

export const returnToService = mutation({
  args: { token: v.string(), roomId: v.id("rooms") },
  handler: async (ctx, { token, roomId }) => {
    const auth = await requireAuth(ctx, token, "lodging.rooms.edit");
    const room = (await ctx.db.get(roomId)) as Doc<"rooms"> | null;
    if (!room || room.propertyId !== auth.propertyId) throw new ConvexError("Room not found");
    await setRoom(ctx, auth, roomId, {
      status: room.hkStatus === "dirty" ? "vacant_dirty" : "vacant_clean",
      oooReason: undefined,
    });
    return { ok: true as const };
  },
});

// ── Board ────────────────────────────────────────────────────────────────
export const board = query({
  args: { token: v.string(), date: v.optional(v.string()) },
  handler: async (ctx, { token, date }) => {
    const auth = await requireAuth(ctx, token, "lodging.hk.view");
    const property = await requireProperty(ctx);
    const day = date ?? todayPropertyLocal(property.timezone);
    const tasks = (await ctx.db
      .query("hkTasks")
      .withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId))
      .collect()).filter((t) => t.scheduledDate === day);
    const rooms = await ctx.db.query("rooms").withIndex("by_property", (q) => q.eq("propertyId", auth.propertyId)).collect();
    const roomById = new Map(rooms.map((r) => [r._id, r]));
    const tickets = (
      await ctx.db
        .query("maintenanceTickets")
        .withIndex("by_property_status", (q) => q.eq("propertyId", auth.propertyId).eq("status", "open"))
        .collect()
    ).length;

    const groups: Record<string, Array<Record<string, unknown>>> = { pending: [], in_progress: [], completed: [], inspected: [] };
    for (const t of tasks.slice().sort((a, b) => a.priority.localeCompare(b.priority))) {
      (groups[t.status] ??= []).push({ ...t, roomNumber: roomById.get(t.roomId)?.number ?? "?" });
    }
    return {
      date: day,
      tasks,
      groups,
      roomsNeedingService: rooms.filter((r) => r.hkStatus === "dirty"),
      dirtyCount: rooms.filter((r) => r.hkStatus === "dirty").length,
      outOfOrder: rooms.filter((r) => r.status === "out_of_order"),
      openTickets: tickets,
    };
  },
});
