import { defineSchema, defineTable } from "convex/server";
import { v } from "convex/values";

// INFINEON Enterprise Platform — full data model (ADR-0006 module hierarchy).
// Money is stored as integer kobo (1 NGN = 100 kobo) for exact decimal arithmetic.
// Dates are "YYYY-MM-DD" strings in property-local time (Africa/Lagos).
//
// NOTE: identity is a plain users table; the console uses the app-level
// auth provider, so no Convex Auth vendor tables are included.

export default defineSchema({
  users: defineTable({
    email: v.string(),
    name: v.string(),
    // PBKDF2-SHA256 encoded hash ("pbkdf2$iterations$salt$hash"). Absent only
    // for legacy demo accounts before their first password-verified login.
    passwordHash: v.optional(v.string()),
  }).index("by_email", ["email"]),

  // Server-side sessions for the console (M-SEC). Only the SHA-256 of the
  // bearer token is stored; the raw token lives in the client.
  sessions: defineTable({
    userId: v.id("users"),
    tokenHash: v.string(),
    roleKey: v.string(),
    propertyId: v.optional(v.id("properties")),
    createdAt: v.number(),
    expiresAt: v.number(),
    lastSeenAt: v.number(),
  })
    .index("by_token", ["tokenHash"])
    .index("by_user", ["userId"]),

  // ── M-PLT Platform core ────────────────────────────────────────────────
  counters: defineTable({
    propertyId: v.id("properties"),
    key: v.string(), // e.g. "reservation", "po", "folio"
    value: v.number(),
  })
    .index("by_property_key", ["propertyId", "key"]),

  profiles: defineTable({
    userId: v.id("users"),
    propertyId: v.optional(v.id("properties")),
    roleKey: v.string(), // owner, admin, manager, front_desk, housekeeping, fnb, finance, hr, guest
    displayName: v.string(),
    phone: v.optional(v.string()),
  }).index("by_user", ["userId"]),

  properties: defineTable({
    pk: v.string(), // stable property key, e.g. "grand-palm-lagos"
    name: v.string(),
    code: v.string(),
    address: v.string(),
    city: v.string(),
    country: v.string(),
    phone: v.string(),
    email: v.string(),
    timezone: v.string(),
    currency: v.string(),
    checkInTime: v.string(), // "14:00"
    checkOutTime: v.string(), // "12:00"
    taxRateBps: v.number(), // VAT in basis points (750 = 7.5%)
    serviceChargeBps: v.number(),
    active: v.boolean(),
    createdAt: v.number(),
  }).index("by_pk", ["pk"]),

  departments: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    code: v.string(),
  }).index("by_property", ["propertyId"]),

  roles: defineTable({
    propertyId: v.id("properties"),
    key: v.string(), // owner, admin, manager, front_desk, housekeeping, fnb, finance, hr
    name: v.string(),
    description: v.string(),
    permissions: v.array(v.string()), // module keys this role may open, e.g. ["lodging","fnb"]
  }).index("by_property", ["propertyId"]),

  settings: defineTable({
    propertyId: v.id("properties"),
    key: v.string(),
    value: v.string(),
    updatedBy: v.optional(v.string()),
    updatedAt: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_key", ["propertyId", "key"]),

  auditLogs: defineTable({
    propertyId: v.id("properties"),
    actorId: v.optional(v.id("users")),
    actorName: v.string(),
    action: v.string(), // reservation.created, room.checked_in, folio.payment_posted …
    entity: v.string(),
    entityId: v.optional(v.string()),
    severity: v.string(), // info | warning | critical
    details: v.optional(v.any()),
    at: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_at", ["propertyId", "at"]),

  // Cross-module domain event log (ADR-0006 contract bus). Every module
  // publishes here; subscribers are dispatched asynchronously (transactional
  // outbox) so producers never couple to consumers.
  domainEvents: defineTable({
    propertyId: v.id("properties"),
    type: v.string(), // reservation.checked_out, folio.charge_posted, stock.low …
    source: v.string(), // module that published the event
    entityId: v.optional(v.string()),
    payload: v.any(),
    status: v.string(), // pending, delivered, failed
    attempts: v.number(),
    error: v.optional(v.string()),
    emittedBy: v.optional(v.string()),
    at: v.number(),
    dispatchedAt: v.optional(v.number()),
  })
    .index("by_property_at", ["propertyId", "at"])
    .index("by_property_type", ["propertyId", "type"])
    .index("by_status", ["status"]),

  // ── M-SEC Security & staff ─────────────────────────────────────────────
  staff: defineTable({
    propertyId: v.id("properties"),
    userId: v.optional(v.id("users")),
    employeeNo: v.string(),
    firstName: v.string(),
    lastName: v.string(),
    email: v.string(),
    phone: v.optional(v.string()),
    roleKey: v.string(),
    department: v.string(),
    employmentType: v.string(), // full_time, part_time, contract
    status: v.string(), // active, on_leave, terminated
    hiredAt: v.string(),
  })
    .index("by_property", ["propertyId"])
    .index("by_user", ["userId"]),

  // ── M-LOD Lodging ──────────────────────────────────────────────────────
  roomTypes: defineTable({
    propertyId: v.id("properties"),
    code: v.string(),
    name: v.string(),
    description: v.string(),
    basePriceKobo: v.number(),
    maxOccupancy: v.number(),
    bedType: v.string(),
    sizeSqm: v.number(),
    amenities: v.array(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_code", ["propertyId", "code"]),

  ratePlans: defineTable({
    propertyId: v.id("properties"),
    roomTypeId: v.id("roomTypes"),
    code: v.string(),
    name: v.string(),
    priceKobo: v.number(),
    includesBreakfast: v.boolean(),
    refundable: v.boolean(),
    minNights: v.number(),
    active: v.boolean(),
  })
    .index("by_property", ["propertyId"])
    .index("by_room_type", ["roomTypeId"]),

  floors: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    level: v.number(),
  }).index("by_property", ["propertyId"]),

  rooms: defineTable({
    propertyId: v.id("properties"),
    number: v.string(),
    roomTypeId: v.id("roomTypes"),
    floorId: v.id("floors"),
    status: v.string(), // vacant_clean, vacant_dirty, occupied_clean, occupied_dirty, out_of_order
    hkStatus: v.string(), // clean, dirty, inspected
    oooReason: v.optional(v.string()),
    lastCleanedAt: v.optional(v.number()),
  })
    .index("by_property", ["propertyId"])
    .index("by_room_type", ["roomTypeId"]),

  roomBlocks: defineTable({
    propertyId: v.id("properties"),
    roomTypeId: v.id("roomTypes"),
    startDate: v.string(),
    endDate: v.string(),
    quantity: v.number(),
    reason: v.string(),
  }).index("by_property", ["propertyId"]),

  guests: defineTable({
    propertyId: v.id("properties"),
    firstName: v.string(),
    lastName: v.string(),
    email: v.string(),
    phone: v.string(),
    country: v.optional(v.string()),
    idType: v.optional(v.string()),
    idNumber: v.optional(v.string()),
    vip: v.boolean(),
    blacklisted: v.boolean(),
    notes: v.optional(v.string()),
    createdAt: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_email", ["propertyId", "email"]),

  reservations: defineTable({
    propertyId: v.id("properties"),
    code: v.string(), // confirmation code, e.g. RSV-2026-000123
    guestId: v.id("guests"),
    roomTypeId: v.id("roomTypes"),
    ratePlanId: v.optional(v.id("ratePlans")),
    status: v.string(), // pending, confirmed, checked_in, checked_out, cancelled, no_show
    source: v.string(), // website, direct, ota, corporate, walk_in
    checkInDate: v.string(),
    checkOutDate: v.string(),
    nights: v.number(),
    adults: v.number(),
    children: v.number(),
    roomsCount: v.number(),
    nightlyRateKobo: v.number(),
    totalKobo: v.number(),
    paidKobo: v.number(),
    paymentStatus: v.string(), // unpaid, partial, paid, refunded
    specialRequests: v.optional(v.string()),
    channelRef: v.optional(v.string()),
    checkedInAt: v.optional(v.number()),
    checkedOutAt: v.optional(v.number()),
    cancelledAt: v.optional(v.number()),
    cancellationReason: v.optional(v.string()),
    createdAt: v.number(),
    createdBy: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_code", ["propertyId", "code"])
    .index("by_property_status", ["propertyId", "status"])
    .index("by_property_checkin", ["propertyId", "checkInDate"])
    .index("by_room_type", ["roomTypeId"])
    .index("by_guest", ["guestId"]),

  roomAssignments: defineTable({
    propertyId: v.id("properties"),
    reservationId: v.id("reservations"),
    roomId: v.id("rooms"),
    assignedAt: v.number(),
    assignedBy: v.optional(v.string()),
  })
    .index("by_reservation", ["reservationId"])
    .index("by_room", ["roomId"])
    .index("by_property", ["propertyId"]),

  // ── Folios & payments (M-FIN) ──────────────────────────────────────────
  folios: defineTable({
    propertyId: v.id("properties"),
    reservationId: v.id("reservations"),
    code: v.string(),
    status: v.string(), // open, closed
    totalKobo: v.number(),
    paidKobo: v.number(),
    openedAt: v.number(),
    closedAt: v.optional(v.number()),
  })
    .index("by_property", ["propertyId"])
    .index("by_reservation", ["reservationId"])
    .index("by_property_status", ["propertyId", "status"]),

  folioLines: defineTable({
    propertyId: v.id("properties"),
    folioId: v.id("folios"),
    postedAt: v.number(),
    category: v.string(), // room, fnb, minibar, laundry, spa, tax, service, payment, adjustment
    description: v.string(),
    qty: v.number(),
    unitKobo: v.number(),
    amountKobo: v.number(),
    department: v.optional(v.string()),
    postedBy: v.optional(v.string()),
  })
    .index("by_folio", ["folioId"])
    .index("by_property", ["propertyId"]),

  payments: defineTable({
    propertyId: v.id("properties"),
    folioId: v.optional(v.id("folios")),
    reservationId: v.optional(v.id("reservations")),
    guestName: v.optional(v.string()),
    method: v.string(), // card, cash, transfer, pos_terminal, online
    amountKobo: v.number(),
    currency: v.string(),
    reference: v.string(), // internal reference
    gateway: v.string(), // internal, paystack, manual
    gatewayRef: v.optional(v.string()), // paystack transaction reference
    paystackInitRef: v.optional(v.string()),
    status: v.string(), // initialized, pending, success, failed, reversed
    channelResponse: v.optional(v.string()), // human-readable gateway status
    createdAt: v.number(),
    paidAt: v.optional(v.number()),
    receivedBy: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_reservation", ["reservationId"])
    .index("by_gateway_ref", ["gatewayRef"])
    .index("by_status", ["status"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── Housekeeping (M-LOD) & maintenance (M-BUS) ─────────────────────────
  hkTasks: defineTable({
    propertyId: v.id("properties"),
    roomId: v.id("rooms"),
    type: v.string(), // cleaning, inspection, turndown, maintenance_check
    priority: v.string(), // low, normal, high, urgent
    status: v.string(), // pending, in_progress, completed, inspected
    assignedTo: v.optional(v.string()),
    scheduledDate: v.string(),
    completedAt: v.optional(v.number()),
    inspectedAt: v.optional(v.number()),
    notes: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"])
    .index("by_room", ["roomId"]),

  maintenanceTickets: defineTable({
    propertyId: v.id("properties"),
    roomId: v.optional(v.id("rooms")),
    category: v.string(), // plumbing, electrical, hvac, it, furniture, other
    priority: v.string(), // low, medium, high, critical
    status: v.string(), // open, in_progress, resolved, closed
    title: v.string(),
    description: v.string(),
    reportedBy: v.optional(v.string()),
    reportedAt: v.number(),
    resolvedAt: v.optional(v.number()),
    resolutionNotes: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── M-FNB Food & Beverage ──────────────────────────────────────────────
  outlets: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    type: v.string(), // restaurant, bar, cafe, room_service, pool
    location: v.string(),
    active: v.boolean(),
  }).index("by_property", ["propertyId"]),

  menuCategories: defineTable({
    propertyId: v.id("properties"),
    outletId: v.id("outlets"),
    name: v.string(),
    sortOrder: v.number(),
  }).index("by_outlet", ["outletId"]),

  menuItems: defineTable({
    propertyId: v.id("properties"),
    outletId: v.id("outlets"),
    categoryId: v.id("menuCategories"),
    name: v.string(),
    description: v.optional(v.string()),
    priceKobo: v.number(),
    station: v.string(), // kitchen, bar, pastry
    available: v.boolean(),
    emoji: v.optional(v.string()),
  })
    .index("by_outlet", ["outletId"])
    .index("by_property", ["propertyId"]),

  posOrders: defineTable({
    propertyId: v.id("properties"),
    outletId: v.id("outlets"),
    orderNo: v.string(),
    tableLabel: v.optional(v.string()),
    status: v.string(), // open, sent, served, paid, void
    items: v.array(
      v.object({
        menuItemId: v.id("menuItems"),
        name: v.string(),
        qty: v.number(),
        unitKobo: v.number(),
        totalKobo: v.number(),
      }),
    ),
    subtotalKobo: v.number(),
    taxKobo: v.number(),
    serviceKobo: v.number(),
    totalKobo: v.number(),
    serverName: v.optional(v.string()),
    roomChargeFolioId: v.optional(v.id("folios")),
    paymentMethod: v.optional(v.string()),
    notes: v.optional(v.string()),
    openedAt: v.number(),
    closedAt: v.optional(v.number()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"])
    .index("by_outlet", ["outletId"]),

  // ── M-INV Inventory & procurement ──────────────────────────────────────
  suppliers: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    contactName: v.optional(v.string()),
    phone: v.optional(v.string()),
    email: v.optional(v.string()),
    paymentTerms: v.optional(v.string()),
  }).index("by_property", ["propertyId"]),

  inventoryItems: defineTable({
    propertyId: v.id("properties"),
    sku: v.string(),
    name: v.string(),
    category: v.string(),
    unit: v.string(),
    qtyOnHand: v.number(),
    reorderLevel: v.number(),
    unitCostKobo: v.number(),
    supplierId: v.optional(v.id("suppliers")),
    location: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_sku", ["propertyId", "sku"]),

  stockMovements: defineTable({
    propertyId: v.id("properties"),
    itemId: v.id("inventoryItems"),
    type: v.string(), // receipt, issue, adjustment, waste
    qty: v.number(),
    reason: v.optional(v.string()),
    ref: v.optional(v.string()),
    at: v.number(),
    by: v.optional(v.string()),
  })
    .index("by_item", ["itemId"])
    .index("by_property", ["propertyId"]),

  purchaseOrders: defineTable({
    propertyId: v.id("properties"),
    poNo: v.string(),
    supplierId: v.id("suppliers"),
    status: v.string(), // draft, sent, partial, received, cancelled
    items: v.array(
      v.object({
        itemId: v.id("inventoryItems"),
        name: v.string(),
        qty: v.number(),
        unitKobo: v.number(),
      }),
    ),
    totalKobo: v.number(),
    createdBy: v.optional(v.string()),
    orderedAt: v.optional(v.number()),
    expectedAt: v.optional(v.string()),
    receivedAt: v.optional(v.number()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── M-FIN General ledger ───────────────────────────────────────────────
  glAccounts: defineTable({
    propertyId: v.id("properties"),
    code: v.string(),
    name: v.string(),
    type: v.string(), // asset, liability, equity, revenue, expense
    subtype: v.string(),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_code", ["propertyId", "code"]),

  glEntries: defineTable({
    propertyId: v.id("properties"),
    date: v.string(),
    accountCode: v.string(),
    debitKobo: v.number(),
    creditKobo: v.number(),
    description: v.string(),
    ref: v.optional(v.string()),
    source: v.string(), // folio, pos, night_audit, manual
    at: v.number(),
    by: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_date", ["propertyId", "date"])
    .index("by_property_account", ["propertyId", "accountCode"]),

  // ── M-CRM Guest experience ─────────────────────────────────────────────
  loyaltyMembers: defineTable({
    propertyId: v.id("properties"),
    guestId: v.id("guests"),
    tier: v.string(), // bronze, silver, gold, platinum
    points: v.number(),
    lifetimeValueKobo: v.number(),
    joinedAt: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_guest", ["guestId"]),

  // Append-only loyalty point movements — every balance change is traceable.
  loyaltyLedger: defineTable({
    propertyId: v.id("properties"),
    guestId: v.id("guests"),
    reservationId: v.optional(v.id("reservations")),
    delta: v.number(),
    reason: v.string(), // stay, welcome, redemption, adjustment
    balanceAfter: v.number(),
    at: v.number(),
  })
    .index("by_guest", ["guestId"])
    .index("by_property", ["propertyId"]),

  campaigns: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    channel: v.string(), // email, sms, whatsapp
    segment: v.string(),
    status: v.string(), // draft, scheduled, sending, sent
    subject: v.optional(v.string()),
    audienceCount: v.number(),
    createdBy: v.optional(v.string()),
    sentAt: v.optional(v.number()),
  }).index("by_property", ["propertyId"]),

  feedback: defineTable({
    propertyId: v.id("properties"),
    guestId: v.optional(v.id("guests")),
    reservationId: v.optional(v.id("reservations")),
    rating: v.number(),
    category: v.string(),
    comment: v.string(),
    status: v.string(), // new, reviewed, resolved
    createdAt: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── M-EVT Events & conference ──────────────────────────────────────────
  venues: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    capacity: v.number(),
    areaSqm: v.number(),
    hourlyRateKobo: v.number(),
    dailyRateKobo: v.number(),
    features: v.array(v.string()),
    active: v.boolean(),
  }).index("by_property", ["propertyId"]),

  eventBookings: defineTable({
    propertyId: v.id("properties"),
    code: v.string(),
    venueId: v.id("venues"),
    clientName: v.string(),
    clientEmail: v.optional(v.string()),
    clientPhone: v.optional(v.string()),
    title: v.string(),
    type: v.string(), // conference, wedding, banquet, meeting, party
    startAt: v.number(),
    endAt: v.number(),
    attendees: v.number(),
    package: v.optional(v.string()),
    quotedKobo: v.number(),
    status: v.string(), // inquiry, tentative, confirmed, completed, cancelled
    notes: v.optional(v.string()),
    createdAt: v.number(),
  })
    .index("by_property", ["propertyId"])
    .index("by_venue", ["venueId"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── M-DST Distribution ─────────────────────────────────────────────────
  channels: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    type: v.string(), // ota, gds, direct, metasearch
    status: v.string(), // connected, disconnected, error
    commissionBps: v.number(),
    lastSyncAt: v.optional(v.number()),
  }).index("by_property", ["propertyId"]),

  channelBookings: defineTable({
    propertyId: v.id("properties"),
    channelName: v.string(),
    channelRef: v.string(),
    guestName: v.string(),
    roomTypeCode: v.string(),
    checkInDate: v.string(),
    checkOutDate: v.string(),
    nights: v.number(),
    totalKobo: v.number(),
    receivedAt: v.number(),
    imported: v.boolean(),
    importedReservationId: v.optional(v.id("reservations")),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_imported", ["propertyId", "imported"]),

  // ── M-HDW Hardware ─────────────────────────────────────────────────────
  devices: defineTable({
    propertyId: v.id("properties"),
    name: v.string(),
    type: v.string(), // pos_terminal, card_reader, pda, door_lock, printer, kds
    location: v.string(),
    status: v.string(), // online, offline, error, maintenance
    lastSeenAt: v.optional(v.number()),
    serial: v.optional(v.string()),
    firmware: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"]),

  // ── M-HR Human resources ───────────────────────────────────────────────
  employees: defineTable({
    propertyId: v.id("properties"),
    employeeNo: v.string(),
    firstName: v.string(),
    lastName: v.string(),
    position: v.string(),
    department: v.string(),
    employmentType: v.string(),
    status: v.string(), // active, on_leave, terminated
    hiredAt: v.string(),
    baseSalaryKobo: v.number(),
    leaveBalance: v.number(),
    phone: v.optional(v.string()),
    email: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_status", ["propertyId", "status"]),

  attendance: defineTable({
    propertyId: v.id("properties"),
    employeeId: v.id("employees"),
    date: v.string(),
    clockIn: v.optional(v.string()),
    clockOut: v.optional(v.string()),
    status: v.string(), // present, late, absent, leave
  })
    .index("by_employee", ["employeeId"])
    .index("by_property_date", ["propertyId", "date"]),

  payrollRuns: defineTable({
    propertyId: v.id("properties"),
    period: v.string(), // "2026-09"
    status: v.string(), // draft, approved, paid
    grossKobo: v.number(),
    deductionsKobo: v.number(),
    netKobo: v.number(),
    headcount: v.number(),
    runAt: v.optional(v.number()),
    approvedBy: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_period", ["propertyId", "period"]),

  // ── M-BUS Night audit ──────────────────────────────────────────────────
  nightAudits: defineTable({
    propertyId: v.id("properties"),
    businessDate: v.string(),
    runAt: v.number(),
    runBy: v.optional(v.string()),
    status: v.string(), // completed, failed
    occupancyPct: v.number(),
    adrKobo: v.number(),
    revparKobo: v.number(),
    roomRevenueKobo: v.number(),
    fnbRevenueKobo: v.number(),
    otherRevenueKobo: v.number(),
    transactionsCount: v.number(),
    notes: v.optional(v.string()),
  })
    .index("by_property", ["propertyId"])
    .index("by_property_date", ["propertyId", "businessDate"]),
  // NOTE: index name "by_state" is reserved for auth vendor tables — do not use it here.
});
