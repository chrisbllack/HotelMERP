// Shared domain types — mirror the platform data model (ADR-0006 submodules)

export type RoomStatus =
  | "occupied_clean"
  | "occupied_dirty"
  | "vacant_clean"
  | "vacant_dirty"
  | "reserved"
  | "out_of_order"
  | "out_of_service";

export interface Room {
  id: string;
  number: string;
  roomTypeId: string;
  floorId: string;
  status: RoomStatus;
  housekeepingStatus: "clean" | "dirty" | "inspected";
  wing?: "East" | "West";
  bedType?: "King" | "Twin" | "Queen" | "Suite";
  features?: string[];
  connectingRoomId?: string | null;
  oooReason?: string;
  oooReturnDate?: string;
  assignedHousekeeper?: string | null;
  cleaningCredits?: number;
  dndActive?: boolean;
  murActive?: boolean;
}

export interface RoomType {
  id: string;
  code: string;
  name: string;
  baseRate: number;
  maxOccupancy: number;
  description?: string;
  amenities?: string[];
}

export type GuestFlag = "VIP" | "Blacklist" | "High-Spend";

export interface Guest {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  phone: string;
  nationality: string;
  company: string;
  idType?: string;
  idNumber?: string;
  vipTier?: "Standard" | "Silver" | "Gold" | "Platinum" | "Diamond";
  totalStays?: number;
  lifetimeSpend?: number;
  preferences?: string[];
  flags?: GuestFlag[];
  blacklistReason?: string;
  flagNotes?: string;
}

export type ReservationStatus =
  | "confirmed"
  | "tentative"
  | "checked_in"
  | "checked_out"
  | "cancelled"
  | "no_show";

export interface Reservation {
  id: string;
  confirmationNumber: string;
  guestId: string;
  roomTypeId: string | null;
  roomId: string | null;
  arrivalDate: string;
  departureDate: string;
  nights: number;
  adults: number;
  children: number;
  status: ReservationStatus;
  source: string;
  totalEstimate: number;
  depositPaid: number;
  isVip: boolean;
  ratePlanCode?: string;
  ratePerNight?: number;
  marketSegment?: string;
  specialRequests?: string;
  keyCardsIssued?: number;
  assignedRoomNumber?: string;
  checkedInAt?: string;
  checkedOutAt?: string;
}

export interface GuestStay {
  id: string;
  reservationId: string;
  guestId: string;
  roomId: string;
  expectedArrival: string;
  expectedDeparture: string;
  status: "active" | "departed";
  actualArrival?: string;
  actualDeparture?: string;
}

export interface Folio {
  id: string;
  guestStayId: string;
  folioNumber: string;
  status: "open" | "closed";
  balance: number; // negative = guest owes (debits exceed credits)
  currency: string;
  guestName?: string;
  roomNumber?: string;
}

export interface FolioLine {
  id?: string;
  folioId: string;
  type: "debit" | "credit";
  description: string;
  amount: number;
  window?: number; // 1: Room & Tax, 2: Extras & Incidental, 3: Direct Bill
  date?: string;
  department?: string;
  reference?: string;
  taxAmount?: number;
  subtotal?: number;
  paymentMethod?: string;
}

export interface GuestTrace {
  id: string;
  reservationId: string;
  guestName: string;
  roomNumber: string;
  department: "front_desk" | "housekeeping" | "room_service" | "concierge" | "maintenance";
  message: string;
  time: string;
  status: "pending" | "resolved";
  resolvedBy?: string;
}

export interface KeyCardRecord {
  id: string;
  reservationId: string;
  roomNumber: string;
  guestName: string;
  keyType: "guest" | "duplicate" | "master" | "mobile_nfc";
  issuedAt: string;
  validUntil: string;
  cardUid: string;
  status: "active" | "revoked";
}

export interface NightAudit {
  id: string;
  businessDate: string;
  totalRooms: number;
  occupiedRooms: number;
  arrivals: number;
  departures: number;
  noShows: number;
  totalRoomCharges: number;
  totalTax: number;
  totalPayments: number;
  status: "completed" | "pending";
}

export interface Outlet {
  id: string;
  name: string;
  code: string;
  type: "restaurant" | "bar" | "room_service";
  status: "active" | "inactive";
}

export interface MenuCategory {
  id: string;
  name: string;
  outletId: string;
}

export interface MenuItem {
  id: string;
  categoryId: string;
  name: string;
  code: string;
  price: number;
  isAvailable: boolean;
  preparationTime: number;
}

export interface PosOrder {
  id: string;
  outletId: string;
  orderNumber: string;
  status: "open" | "closed" | "voided";
  subtotal: number;
  taxAmount: number;
  discountAmount: number;
  total: number;
  paymentStatus: "paid" | "pending" | "voided";
  createdAt: string;
}

export interface PosOrderLine {
  orderId: string;
  menuItemId: string;
  quantity: number;
  unitPrice: number;
  amount: number;
  status: "completed" | "pending" | "voided";
}

export type HkTaskStatus = "pending" | "in_progress" | "completed" | "inspected";
export type HkPriority = "normal" | "high" | "urgent";

export interface HousekeepingTask {
  id: string;
  roomId: string;
  taskType: string;
  priority: HkPriority;
  status: HkTaskStatus;
  assignedTo: string | null;
}

export interface Employee {
  id: string;
  employeeNumber: string;
  firstName: string;
  lastName: string;
  email: string | null;
  phone: string | null;
  departmentId: string;
  position: string;
  salary: number;
  status: "active" | "on_leave" | "terminated";
}

export interface Department {
  id: string;
  name: string;
  code: string;
}

export interface GlAccount {
  id: string;
  accountCode: string;
  accountName: string;
  accountClass: "asset" | "liability" | "equity" | "revenue" | "expense";
  accountType: string;
}

export interface JournalEntry {
  id: string;
  entryNumber: string;
  entryDate: string;
  description: string;
  status: "draft" | "posted" | "reversed";
}

export interface CashierShift {
  id: string;
  shiftNumber: string;
  openingFloat: number;
  closingFloat: number | null;
  totalSales: number;
  totalRefunds: number;
  status: "open" | "closed";
}

export interface LoyaltyTier {
  id: string;
  name: string;
  code: string;
  minPoints: number;
  discountPercent: number;
  benefits: string;
  status: "active" | "inactive";
}

export interface PaymentMethod {
  name: string;
  code: string;
  type: "cash" | "card" | "transfer" | "online" | "mobile" | "credit" | "complimentary";
  isActive: boolean;
}

export interface EventBooking {
  id: string;
  name: string;
  eventType: string;
  clientName: string;
  venueId: string | null;
  eventDate: string;
  startTime: string;
  endTime: string;
  guestCount: number;
  packageName: string;
  totalAmount: number;
  status: "inquiry" | "tentative" | "confirmed" | "completed" | "cancelled";
}

export interface Venue {
  id: string;
  name: string;
  capacity: number;
  hourlyRate: number;
  status: "available" | "maintenance";
}

export interface Channel {
  id: string;
  name: string;
  code: string;
  type: "ota" | "gds" | "direct" | "wholesaler";
  status: "connected" | "disconnected" | "error";
  lastSyncAt: string | null;
}

export interface HardwareDevice {
  id: string;
  deviceType: "door_lock" | "printer" | "pos_terminal" | "kds" | "scanner" | "iptv" | "biometric";
  vendor: string;
  model: string;
  serialNumber: string;
  location: string;
  status: "online" | "offline" | "error";
  lastHealthCheck: string | null;
}

export interface User {
  id: string;
  name: string;
  email: string;
  role: Role;
  status: "active" | "suspended";
}

export type Role =
  | "admin"
  | "general_manager"
  | "front_desk"
  | "cashier"
  | "housekeeping"
  | "fnb"
  | "accountant"
  | "hr";

export const ROLE_LABELS: Record<Role, string> = {
  admin: "System Administrator",
  general_manager: "General Manager",
  front_desk: "Front Desk",
  cashier: "Cashier",
  housekeeping: "Housekeeping",
  fnb: "Food & Beverage",
  accountant: "Accountant",
  hr: "Human Resources",
};

export interface AuditLogEntry {
  id: string;
  userId: string;
  userName: string;
  module: string;
  action: string;
  entityId: string;
  createdAt: string;
}

export interface InventoryProduct {
  id: string;
  sku: string;
  name: string;
  category: string;
  unit: string;
  costPrice: number;
  reorderLevel: number;
  valuationMethod?: ValuationMethod;
  storeStock?: StoreStockLevel[];
  batches?: ProductBatch[];
}

export interface StockLevel {
  productId: string;
  storeId: string;
  quantity: number;
}

export interface Store {
  id: string;
  name: string;
  type: "main" | "kitchen" | "bar" | "housekeeping";
}

export interface Supplier {
  id: string;
  name: string;
  contactPerson: string;
  email: string;
  phone: string;
  paymentTerms: string;
}

export interface PurchaseOrder {
  id: string;
  poNumber: string;
  supplierId: string;
  orderDate: string;
  status: "draft" | "pending_approval" | "approved" | "received" | "cancelled";
  total: number;
}

// ----------------- Shift Handover -----------------
export interface ShiftHandoverTask {
  id: string;
  description: string;
  category: "checkout" | "vip_arrival" | "wakeup_call" | "luggage" | "maintenance" | "key_audit" | "other";
  status: "pending" | "completed" | "in_progress";
  priority: "normal" | "high" | "urgent";
  dueTime?: string;
}

export interface ShiftGuestNote {
  id: string;
  guestName: string;
  roomNumber: string;
  note: string;
  severity: "info" | "attention" | "urgent";
  actionRequired?: string;
}

export interface ShiftHandoverRecord {
  id: string;
  handoverNumber: string;
  shiftType: "morning_am" | "evening_pm" | "night_graveyard";
  shiftDate: string;
  departingStaffName: string;
  incomingStaffName: string;
  status: "draft" | "submitted" | "acknowledged";
  submittedAt: string;
  acknowledgedAt?: string;
  
  // Cash Drawer Counts & Float
  openingFloat: number;
  expectedCashDrop: number;
  countedCashTotal: number;
  cashVariance: number;
  varianceExplanation?: string;
  denominations: {
    n1000: number;
    n500: number;
    n200: number;
    n100: number;
    n50: number;
  };
  posCardBatchesSettled: boolean;
  totalCardPayments: number;
  
  // Tasks & Responsibilities
  tasks: ShiftHandoverTask[];
  
  // Critical Guest Notes & Incident Log
  guestNotes: ShiftGuestNote[];
  
  // Security Verification
  departingStaffPinVerified: boolean;
  incomingStaffPinVerified: boolean;
  handoverSummaryNotes: string;
}

// ----------------- Preventative Maintenance -----------------
export interface PreventativeMaintenanceChecklistItem {
  id: string;
  item: string;
  passed?: boolean;
  reading?: string;
  notes?: string;
}

export interface PreventativeMaintenanceSchedule {
  id: string;
  scheduleCode: string;
  title: string;
  equipmentType: "HVAC & Air Con" | "Fire & Smoke Safety" | "Plumbing & Water Heaters" | "RFID Door Locks" | "Electrical & Backup Generator" | "Elevators & Lifts" | "Refrigeration & Minibars" | "Windows & Balconies";
  targetType: "room" | "floor" | "common_area" | "central_plant";
  targetIds: string[];
  frequency: "weekly" | "bi_weekly" | "monthly" | "quarterly" | "semi_annual" | "annual";
  assignedEngineer: string;
  lastCompletedDate: string | null;
  nextDueDate: string;
  status: "scheduled" | "due_soon" | "overdue" | "in_progress" | "completed";
  checklist: PreventativeMaintenanceChecklistItem[];
  complianceScore: number;
  notes?: string;
}

export interface PmInspectionLog {
  id: string;
  scheduleId: string;
  scheduleTitle: string;
  equipmentType: string;
  inspectedTarget: string;
  technicianName: string;
  inspectedAt: string;
  result: "passed" | "passed_with_notes" | "failed_repair_needed";
  readings: Record<string, string>;
  findings: string;
  spawnedWorkOrderId?: string;
}

// ----------------- Yield & Revenue Management -----------------
export interface CompetitorRate {
  competitorName: string;
  roomType: string;
  rate: number;
  varianceVsUs: number;
  channel: string;
}

export interface MarketEvent {
  id: string;
  name: string;
  startDate: string;
  endDate: string;
  impactLevel: "low" | "medium" | "high" | "compression";
  estimatedInflow: string;
}

export interface DynamicRateRecommendation {
  roomTypeId: string;
  roomTypeName: string;
  currentBaseRate: number;
  recommendedRate: number;
  variancePercent: number;
  confidenceScore: number;
  demandFactor: "High Weekend Pace" | "Compression Event" | "Low Mid-week Demand" | "Competitor Undercut" | "Surge Occupancy";
  suggestedAction: "Increase Rate" | "Decrease Rate" | "Hold Base" | "Attach Min Length of Stay";
  minStayRequirement?: number;
  projectedRevPARGain: number;
  applied: boolean;
}

export interface YieldRule {
  id: string;
  name: string;
  condition: string;
  action: string;
  isActive: boolean;
  lastTriggered?: string;
}

// ----------------- Night Audit -----------------
export interface NightAuditReport {
  id: string;
  businessDate: string;
  completedAt: string;
  auditorName: string;
  roomsCharged: number;
  totalRoomRevenue: number;
  totalTaxPosted: number;
  totalServiceChargePosted: number;
  totalFnbRevenue: number;
  totalOtherRevenue: number;
  grandTotalRevenue: number;
  totalPaymentsCollected: number;
  cityLedgerTransfers: number;
  cashierVariancesTotal: number;
  noShowsCount: number;
  roomsOooCount: number;
  endingOccupancyRate: number;
  adr: number;
  revpar: number;
  trialBalanceBalanced: boolean;
  status: "balanced" | "warnings" | "posted";
}

// ----------------- Guest Sentiment & CRM -----------------
export interface GuestSurveyResponse {
  id: string;
  guestId: string;
  guestName: string;
  stayId: string;
  roomNumber: string;
  checkoutDate: string;
  submittedAt: string;
  channel: "Post-Stay Email" | "In-Room QR / Tablet" | "WhatsApp Concierge" | "OTA Direct Review" | "Google Review";
  overallScore: number;
  npsType: "promoter" | "passive" | "detractor";
  categories: {
    frontDesk: number;
    cleanliness: number;
    dining: number;
    roomComfort: number;
    value: number;
  };
  sentiment: "positive" | "neutral" | "negative";
  comment: string;
  highlights: string[];
  serviceRecoveryTicketId?: string;
}

export interface ServiceRecoveryTicket {
  id: string;
  surveyId: string;
  guestName: string;
  guestPhone: string;
  guestEmail: string;
  roomNumber: string;
  issueSummary: string;
  department: "Front Desk" | "Housekeeping" | "Food & Beverage" | "Maintenance" | "Billing";
  priority: "high" | "urgent";
  status: "open" | "contacted" | "compensation_offered" | "resolved";
  assignedManager: string;
  compensationOffered?: string;
  resolutionNotes?: string;
  createdAt: string;
  resolvedAt?: string;
}

// ----------------- Lost and Found (M-HKP) -----------------
export type LostAndFoundStatus = "logged" | "claimed" | "returned" | "disposed";

export interface LostAndFoundItem {
  id: string;
  itemNumber: string;
  name: string;
  category: "Electronics" | "Jewelry & Valuables" | "Clothing & Apparel" | "Documents & Passports" | "Bags & Luggage" | "Keys & Cards" | "Other";
  description: string;
  roomNumber?: string;
  locationFound: string;
  foundByStaffName: string;
  foundDate: string;
  estimatedValue?: number;
  storageLocker: string;
  status: LostAndFoundStatus;
  guestId?: string;
  guestName?: string;
  guestEmail?: string;
  guestPhone?: string;
  claimedAt?: string;
  claimedBy?: string;
  returnedAt?: string;
  returnedByStaff?: string;
  disposedAt?: string;
  disposalReason?: string;
  emailSentAt?: string;
  notes?: string;
}

// ----------------- Room Energy Consumption & BMS (M-ENG) -----------------
export interface RoomEnergyReading {
  roomId: string;
  roomNumber: string;
  floor: number;
  occupancyStatus: "occupied" | "vacant";
  keycardInserted: boolean;
  powerDrawKw: number;
  dailyKwh: number;
  currentTemp: number; // in Celsius
  targetTemp: number;
  hvacState: "cooling" | "heating" | "fan_only" | "eco_standby" | "off";
  lightingState: "all_off" | "partial" | "all_on";
  waterFlowLpm: number;
  highConsumptionAlert: boolean;
  alertReason?: string;
  ecoModeEngaged: boolean;
  lastTelemetryTime: string;
}

// ----------------- Smart Room Controls & Guest Portal (M-GST) -----------------
export interface SmartRoomControl {
  roomId: string;
  roomNumber: string;
  reservationId?: string;
  guestName?: string;
  dndActive: boolean; // Do Not Disturb
  murActive: boolean; // Make Up Room
  targetTemperature: number; // 16 - 28 Celsius
  currentTemperature: number;
  hvacMode: "cool" | "heat" | "fan" | "eco" | "off";
  fanSpeed: "auto" | "low" | "medium" | "high";
  lightingScene: "all_off" | "relax" | "bright" | "night" | "work" | "ambient";
  lights: {
    master: boolean;
    entryway: boolean;
    bedroom: boolean;
    bathroom: boolean;
    nightstand: boolean;
  };
  doorLocked: boolean;
  privacyLatchEngaged: boolean;
  lastUpdated: string;
}

// ----------------- Hospitality Finance & Accounting (USALI) -----------------
export interface USALILineItem {
  code: string;
  name: string;
  monthActual: number;
  monthBudget: number;
  monthVariance: number;
  ytdActual: number;
  ytdBudget: number;
  pctRevenue: number;
}

export interface USALIDepartmentStatement {
  department: "Rooms" | "Food & Beverage" | "Other Operated" | "Undistributed A&G" | "Undistributed POM" | "Undistributed Utilities" | "Non-Operating / GOP";
  title: string;
  grossRevenue: number;
  directExpenses: number;
  departmentalProfit: number;
  marginPct: number;
  lines: USALILineItem[];
}

export interface CityLedgerInvoice {
  id: string;
  invoiceNumber: string;
  issueDate: string;
  dueDate: string;
  guestName: string;
  folioNumber: string;
  amount: number;
  whtAmount: number; // Withholding tax 5%
  netPayable: number;
  status: "unbilled" | "invoiced" | "partial" | "settled" | "overdue";
  daysOverdue: number;
  settledAt?: string;
}

export interface CityLedgerAccount {
  id: string;
  accountNumber: string;
  companyName: string;
  contactPerson: string;
  email: string;
  phone: string;
  creditLimit: number;
  currentBalance: number;
  aging: {
    current: number;    // 0-30 days
    days31to60: number; // 31-60 days
    days61to90: number; // 61-90 days
    over90: number;     // >90 days
  };
  invoices: CityLedgerInvoice[];
  status: "active" | "on_hold" | "suspended";
}

export interface AccountsPayableInvoice {
  id: string;
  invoiceNumber: string;
  vendorId: string;
  vendorName: string;
  category: "Food & Beverage" | "Guest Supplies & Amenities" | "Linen & Laundry" | "Engineering & Energy" | "IT & Licenses" | "Professional Services";
  poNumber: string;
  grnNumber: string;
  threeWayMatched: boolean;
  invoiceDate: string;
  dueDate: string;
  grossAmount: number;
  vatAmount: number; // 7.5%
  whtRate: number;   // 5% or 10%
  whtAmount: number;
  netPayable: number;
  status: "pending_3way" | "approved" | "scheduled" | "paid";
  paymentBatchId?: string;
  paymentVoucherRef?: string;
}

export interface TaxRemittanceSchedule {
  id: string;
  taxType: "VAT (7.5%)" | "Tourism & Consumption (5%)" | "Withholding Tax (WHT 5%)" | "PAYE Payroll Tax";
  period: string;
  taxableBase: number;
  taxPayable: number;
  collectedToDate: number;
  dueDate: string;
  status: "accruing" | "reconciled" | "remitted";
  filingRef?: string;
}

// ----------------- Food & Beverage Operations (M-FNB) -----------------
export type TableStatus = "available" | "seated" | "order_placed" | "billed";
export type RestaurantZone = "Main Dining Room" | "Terrace & Poolside" | "Executive Lounge" | "In-Room Dining (IRDs)";

export interface RestaurantTable {
  id: string;
  number: string;
  zone: RestaurantZone;
  capacity: number;
  status: TableStatus;
  currentOrderId?: string;
  currentOrderNumber?: string;
  guestName?: string;
  roomNumber?: string;
  serverName?: string;
  seatedAt?: string;
  totalAmount?: number;
  activeCovers?: number;
}

export type KitchenStation = "Hot Kitchen" | "Grill & Broiler" | "Pantry & Cold Salads" | "Pastry & Dessert" | "Bar Dispenser";
export type KotStatus = "queued" | "cooking" | "ready" | "served";

export interface KotItem {
  id: string;
  menuItemId: string;
  name: string;
  quantity: number;
  modifiers?: string[];
  course: "Starters" | "Mains" | "Dessert" | "Beverage";
  status: KotStatus;
  station: KitchenStation;
}

export interface KitchenOrderTicket {
  id: string;
  ticketNumber: string;
  outletName: string;
  tableOrRoom: string;
  serverName: string;
  createdAt: string;
  orderType: "dine_in" | "room_service" | "poolside" | "takeaway";
  guestName?: string;
  roomNumber?: string;
  status: KotStatus;
  elapsedMinutes: number;
  items: KotItem[];
  specialInstructions?: string;
}

export interface RecipeIngredient {
  productId: string;
  name: string;
  portionQty: number;
  unit: string;
  unitCost: number;
  lineCost: number;
}

export interface RecipeCostingItem {
  id: string;
  menuItemId: string;
  name: string;
  category: string;
  sellingPrice: number;
  foodCost: number;
  foodCostPct: number; // e.g. 28.5%
  targetCostPct: number; // e.g. 30.0%
  grossMargin: number;
  engineeringClass: "Star (High Margin / Popular)" | "Plowhorse (Low Margin / Popular)" | "Puzzle (High Margin / Niche)" | "Dog (Low Margin / Low Popularity)";
  popularityScore: number; // monthly sales count
  ingredients: RecipeIngredient[];
}

export interface BanquetEventOrder {
  id: string;
  beoNumber: string;
  eventTitle: string;
  clientName: string;
  clientCompany?: string;
  eventDate: string;
  startTime: string;
  endTime: string;
  hallName: string;
  expectedCovers: number;
  guaranteedCovers: number;
  mealServiceType: "International Buffet" | "Plated 4-Course VIP" | "Cocktail & Canapés" | "Working Executive Lunch";
  menuHighlights: string[];
  beveragePackage: "Full Open Premium Bar" | "Wine & Soft Drinks Package" | "On Consumption";
  specialDietaryNotes?: string;
  totalBudget: number;
  depositReceived: number;
  status: "tentative" | "contracted" | "food_tasting_done" | "in_preparation" | "completed";
}

export interface KitchenWasteLog {
  id: string;
  date: string;
  station: KitchenStation;
  itemName: string;
  quantity: number;
  unit: string;
  costLoss: number;
  reason: "Burnt / Overcooked" | "Expired / Spoilage" | "Prep Trimmings" | "Buffet Surplus" | "Guest Return";
  loggedByChef: string;
  actionTaken: string;
}

// ----------------- Supply Chain & Inventory (M-INV) -----------------
export interface SupplyChainStore {
  id: string;
  code: string;
  name: string;
  type: "Central Warehouse" | "Kitchen Dry Store" | "Walk-in Cold Room" | "Wine & Beverage Cellar" | "Housekeeping Linen & Amenities" | "Engineering Spares";
  managerName: string;
  location: string;
  activeSkus: number;
  totalValue: number;
}

export interface SupplyChainProduct {
  id: string;
  sku: string;
  name: string;
  category: "Perishables & Produce" | "Meat & Seafood" | "Dry Provisions" | "Beverage & Spirits" | "Guest Amenities" | "Linen & Textiles" | "Chemicals & Cleaning" | "Engineering Spares";
  unit: string;
  costPrice: number;
  parLevel: number;
  reorderPoint: number;
  safetyStock: number;
  storageCondition: "Ambient Dry" | "Refrigerated (+2°C to +4°C)" | "Deep Freeze (-18°C)" | "Secure Locked Vault";
  supplierName: string;
  leadTimeDays: number;
  stockByStore: Record<string, number>; // storeId -> quantity
  totalQuantity: number;
  totalValue: number;
  status: "healthy" | "low_stock" | "critical";
  valuationMethod?: ValuationMethod;
  storeStock?: StoreStockLevel[];
  batches?: ProductBatch[];
  bookValue?: number;
}

export interface RequisitionItem {
  productId: string;
  name: string;
  quantity: number;
  unit: string;
  estimatedCost: number;
  purpose: string;
}

export interface PurchaseRequisition {
  id: string;
  requisitionNumber: string;
  department: "Food & Beverage - Kitchen" | "Front Office & Guest Services" | "Housekeeping" | "Maintenance & Engineering" | "Food & Beverage - Bar";
  requestedBy: string;
  requestDate: string;
  urgency: "Routine Par Fill" | "Event Demand" | "Emergency Rush";
  items: RequisitionItem[];
  totalEstimate: number;
  status: "draft" | "submitted" | "hod_approved" | "procurement_po_created" | "rejected";
  poNumber?: string;
  notes?: string;
}

export interface GrnLineItem {
  productId: string;
  name: string;
  orderedQty: number;
  receivedQty: number;
  rejectedQty: number;
  unit: string;
  unitCost: number;
  rejectionReason?: string;
}

export interface GoodsReceivedNote {
  id: string;
  grnNumber: string;
  poNumber: string;
  supplierName: string;
  receivedDate: string;
  receivedBy: string;
  storeTarget: string;
  deliveryNoteNumber: string;
  temperatureOnArrival?: string; // e.g. "-19.2°C"
  hygieneCondition: "Passed Inspection" | "Requires Washing" | "Rejected";
  items: GrnLineItem[];
  totalAcceptedValue: number;
  hasVariance: boolean;
  status: "inspected" | "posted_to_inventory" | "rejected";
}

export interface StoreTransferRecord {
  id: string;
  transferNumber: string;
  transferDate: string;
  fromStoreName: string;
  toStoreName: string;
  requestedBy: string;
  authorizedBy: string;
  items: Array<{
    productId: string;
    name: string;
    quantity: number;
    unit: string;
  }>;
  status: "pending_dispatch" | "in_transit" | "received";
  notes?: string;
}

export interface StocktakeAuditRecord {
  id: string;
  auditNumber: string;
  storeName: string;
  auditDate: string;
  auditorName: string;
  countedBy?: string;
  variance?: number;
  categoriesAudited: string[];
  totalSkusCounted: number;
  totalSystemValue: number;
  totalPhysicalValue: number;
  varianceValue: number;
  variancePct: number;
  discrepanciesCount: number;
  status: "in_progress" | "submitted" | "approved" | "rejected" | "reconciled" | "posted_adjustment";
  itemsSample: Array<{
    sku: string;
    name: string;
    systemQty: number;
    physicalQty: number;
    varianceQty: number;
    unitCost: number;
    varianceCost: number;
    reason?: string;
  }>;
  lines?: Array<{
    productId: string;
    name: string;
    expected: number;
    counted: number;
    variance: number;
    unitCost: number;
  }>;
}

// ----------------- Inventory Depth Extensions (Multi-Store, Batches, Valuation) -----------------
export type ValuationMethod = "FIFO" | "LIFO" | "WeightedAvg" | "Standard";
export type StoreLocation =
  | "main"
  | "fnb"
  | "engineering"
  | "housekeeping"
  | "spa"
  | "minibar";

export interface StoreStockLevel {
  storeId: string;
  productId: string;
  onHand: number;
  reserved: number;
  available: number;
  reorderPoint: number;
  parLevel: number;
  minLevel: number;
  maxLevel: number;
  lastCountedAt: string;
}

export interface ProductBatch {
  id: string;
  productId: string;
  batchNumber: string;
  expiryDate: string;
  manufactureDate: string;
  supplierId: string;
  quantityReceived: number;
  quantityRemaining: number;
  costPerUnit: number;
  grnId?: string;
  location: StoreLocation;
}

export interface SupplierScorecard {
  supplierId: string;
  supplierName: string;
  onTimeDeliveryPct: number;
  qualityRejectPct: number;
  avgLeadTimeDays: number;
  totalSpendLast90Days: number;
  rating: "A+" | "A" | "B" | "C";
}

export interface ReorderAlert {
  productId: string;
  productName: string;
  storeId: string;
  storeName: string;
  currentLevel: number;
  reorderPoint: number;
  suggestedOrderQty: number;
  urgency: "low" | "medium" | "high" | "critical";
  leadTimeDays: number;
}

// ---------------------------------------------------------------------------
// Cash Flow Projections & 30-Day Liquidity Forecast
// ---------------------------------------------------------------------------

export interface DailyCashFlowEntry {
  date: string;
  dayNumber: number;
  dayName: string;
  inflows: {
    roomRevenueSettlements: number;
    roomAdvanceDeposits: number;
    cityLedgerCollections: number;
    fnbAndOutletsRevenue: number;
    otherOperatingInflows: number;
    totalInflow: number;
  };
  outflows: {
    payrollAndWages: number;
    vendorAccountsPayable: number;
    utilitiesAndDiesel: number;
    statutoryTaxes: number;
    loanServiceAndCapex: number;
    totalOutflow: number;
  };
  netCashFlow: number;
  openingBalance: number;
  closingBalance: number;
  isMinReserveBreached: boolean;
  notes?: string;
}

export interface CashFlowScenario {
  occupancyAdjustmentPct: number;
  arCollectionDelayDays: number;
  capexFreeze: boolean;
}

// ---------------------------------------------------------------------------
// Supplier Contract Management & SLAs
// ---------------------------------------------------------------------------

export type SupplierContractCategory =
  | "fnb_perishables"
  | "fnb_beverages"
  | "fnb_dry_goods"
  | "facility_hvac_power"
  | "facility_fire_life_safety"
  | "housekeeping_linen_chem"
  | "it_telecom";

export type SupplierContractStatus =
  | "active"
  | "expiring_soon"
  | "in_renewal"
  | "expired"
  | "terminated";

export type RenewalType =
  | "auto_renew_30d"
  | "auto_renew_60d"
  | "manual_negotiation"
  | "tender_required";

export interface SupplierContractSla {
  deliveryTimelinessPct: number;
  currentTimelinessPct: number;
  fillRatePct: number;
  currentFillRatePct: number;
  tempQualityCompliancePct: number;
  currentQualityCompliancePct: number;
  emergencyResponseTimeMinutes?: number;
  overallScore: number;
  grade: "A+" | "A" | "B" | "C" | "F";
  penaltyClausesActive: boolean;
  lastAuditDate: string;
}

export interface SupplierContract {
  id: string;
  contractNumber: string;
  supplierId: string;
  supplierName: string;
  category: SupplierContractCategory;
  title: string;
  annualValue: number;
  paymentTerms: string;
  startDate: string;
  endDate: string;
  noticePeriodDays: number;
  renewalType: RenewalType;
  status: SupplierContractStatus;
  sla: SupplierContractSla;
  keyContacts: {
    accountManager: string;
    phone: string;
    email: string;
  };
  deliverablesSummary: string;
  renewalAlertDays: number;
  lastRenewalNoticeSent?: string;
  notes?: string;
}

// ---------------------------------------------------------------------------
// Kitchen Prep & Recipe Scaling
// ---------------------------------------------------------------------------

export type KitchenPrepStation =
  | "hot_line"
  | "cold_pantry"
  | "butchery"
  | "bakery_pastry"
  | "banquet_prep";

export type KitchenShift = "breakfast" | "lunch" | "dinner" | "banquet" | "all_day";

export type PrepItemStatus = "pending" | "in_prep" | "completed" | "qc_verified";

export interface ScaledIngredientRequirement {
  ingredientId: string;
  name: string;
  baseQty: number;
  scaledQty: number;
  unit: string;
  inventoryStore: string;
  currentStockQty: number;
  stockStatus: "sufficient" | "low_stock" | "stockout";
}

export interface KitchenPrepItem {
  id: string;
  dishName: string;
  category: string;
  mealPeriod: KitchenShift;
  station: KitchenPrepStation;
  baseYieldPortions: number;
  forecastedPortions: number;
  scalingFactor: number;
  leadTimeMinutes: number;
  assignedChef: string;
  status: PrepItemStatus;
  ingredients: ScaledIngredientRequirement[];
  criticalAllergens: string[];
  storageLocation: string;
  notes?: string;
}

export interface KitchenOccupancyForecast {
  hotelOccupancyRatePct: number;
  inHouseGuests: number;
  expectedArrivalGuests: number;
  forecastedBreakfastCovers: number;
  forecastedLunchCovers: number;
  forecastedDinnerCovers: number;
  forecastedBanquetCovers: number;
  totalForecastedCovers: number;
}

// ---------------------------------------------------------------------------
// Maintenance Audit Trail (Assets Module)
// ---------------------------------------------------------------------------

export interface ReplacedPart {
  id: string;
  partNumber: string;
  partName: string;
  category: "Mechanical" | "Electrical" | "Plumbing" | "HVAC & Refrigeration" | "Electronics" | "Consumables";
  quantity: number;
  unitCost: number;
  totalCost: number;
  supplierOrBatch?: string;
  warrantyExpiry?: string;
}

export interface MaintenanceAuditRecord {
  id: string;
  workOrderNumber: string;
  repairTicketCode: string;
  assetTag: string;
  assetName: string;
  assetCategory: "HVAC & Climate" | "Generators & Power" | "Chillers & Refrigeration" | "Elevators & Escalators" | "Commercial Kitchen" | "Plumbing & Boilers" | "Fire Safety & Access";
  location: string;
  failureReported: string;
  diagnosis: string;
  actionTaken: string;
  
  // Technician Accountability
  technicianId: string;
  technicianName: string;
  technicianCertification: string;
  technicianSignatureDate: string;
  
  // Supervisor / QC Verification
  supervisorName: string;
  supervisorApprovalStatus: "verified_passed" | "conditional_pass" | "rework_required" | "pending_qc";
  supervisorSignOffDate?: string;
  supervisorNotes?: string;

  // Parts Replaced
  partsReplaced: ReplacedPart[];
  totalPartsCost: number;

  // Time Accountability & Labor
  taskStartedAt: string;
  taskCompletedAt: string;
  timeTakenMinutes: number;
  technicianHourlyRate: number;
  laborCost: number;
  totalRepairCost: number;

  // Asset Condition & Impact
  preRepairCondition: "Critical Failure / Inoperable" | "Degraded Performance" | "Safety Hazard" | "Intermittent Fault";
  postRepairCondition: "Operational (Optimal Efficiency)" | "Operational (Monitoring Required)" | "Temporary Fix Applied";
  downtimeHours: number;
  preventativeAdvice?: string;
  status: "completed" | "verified" | "in_review";
}

// ---------------------------------------------------------------------------
// Personalized Preference Log (Guest 360 Profile)
// ---------------------------------------------------------------------------

export type PillowType =
  | "Feather & Down"
  | "Memory Foam"
  | "Buckwheat Hull"
  | "Firm Hypoallergenic"
  | "Silk & Microfiber"
  | "Contour Ergonomic";

export type DietaryRestriction =
  | "Gluten-Free"
  | "Strict Halal"
  | "Shellfish Allergy"
  | "Tree Nut Allergy"
  | "Peanut Allergy"
  | "Vegan"
  | "Vegetarian"
  | "Lactose-Free"
  | "Diabetic Friendly"
  | "Low Sodium"
  | "Kosher";

export interface GuestPreferenceLogEntry {
  id: string;
  timestamp: string;
  category: "climate" | "bedding" | "dietary" | "amenity" | "room_location";
  changeDescription: string;
  source: "Guest App" | "Front Desk Check-in" | "Concierge Trace" | "Post-Stay Survey" | "VIP Butler";
  staffMember?: string;
}

export interface GuestPersonalizedPreferences {
  guestId: string;
  guestName: string;
  
  // Room Temperature & Climate
  preferredTemperatureCelsius: number; // 16.0 - 28.0
  preferredAcMode: "cool" | "heat" | "fan" | "eco";
  preferredFanSpeed: "auto" | "low" | "medium" | "high";
  
  // Bedding & Sleep Comfort
  pillowType: PillowType;
  pillowCount: number;
  mattressTopperPreference?: "Extra Soft Plush" | "Firm Orthopedic" | "Featherbed" | "Standard Luxury";
  duvetWeight?: "Lightweight Summer" | "All-Season Goose Down" | "Heavy Weight";

  // Dietary Restrictions & Dining
  dietaryRestrictions: DietaryRestriction[];
  dietaryNotes?: string;
  beveragePreference?: "Perrier Sparkling" | "San Pellegrino" | "Fresh Coconut Water" | "English Breakfast Tea" | "Hibiscus Zobo" | "Espresso Double";
  winePreference?: "Cabernet Sauvignon" | "Chardonnay Oak" | "Champagne Brut" | "Non-Alcoholic";

  // Room Location & Amenities
  floorPreference: "High Floor (Floors 5-7)" | "Low Floor (Floors 1-2)" | "Near Elevator" | "Quiet End of Hall" | "Lagoon / Ocean View";
  turndownPreference: "18:00 Standard" | "19:30 Evening" | "21:00 Late Night" | "Declined";
  newspaperPreference?: "Financial Times" | "The Guardian Nigeria" | "BusinessDay" | "Digital Only";
  roomFragrance?: "White Tea & Thyme" | "Lavender Relaxing" | "Fresh Citrus Bergamot" | "Unscented Hypoallergenic";

  // Pre-Arrival Automation Engine
  preArrivalAutomation: {
    autoPreSetEnabled: boolean;
    assignedRoomNumber?: string;
    lastDispatchedAt?: string;
    status: "idle" | "ready_for_dispatch" | "controls_active" | "pre_conditioned";
    dispatchedActions: Array<{
      id: string;
      timestamp: string;
      system: "IoT Climate Control" | "Housekeeping Linen Dispatch" | "F&B Kitchen Alert" | "VIP Welcome Amenity";
      detail: string;
      status: "applied" | "in_progress" | "verified";
    }>;
  };

  // Preference Log History
  history: GuestPreferenceLogEntry[];
}

// ---------------------------------------------------------------------------
// Inventory Threshold Alerts & Burn Rate System
// ---------------------------------------------------------------------------

export interface CoreInventoryThresholdItem {
  id: string;
  productId: string;
  sku: string;
  name: string;
  category: "Meat & Seafood" | "Dairy & Cold-Chain" | "Fresh Produce" | "Dry Provisions" | "Beverages & Wines" | "Bakery & Pastry";
  storeLocation: string;
  unit: string;
  currentStock: number;
  dailyBurnRate: number;
  burnRateBasis: string;
  minStockDaysThreshold: number;
  criticalStockDaysThreshold: number;
  parTargetLevel: number;
  costPerUnit: number;
  leadTimeDays: number;
  primarySupplierName: string;
  supplierContact: string;
  supplierEmail: string;
  status: "healthy" | "warning_low" | "critical_depletion" | "stockout";
  isAutoAlertEnabled: boolean;
  alertRecipientEmails: string[];
  alertRecipientRoles: string[];
  lastAlertSentAt?: string;
  openPoNumber?: string;
}

export interface InventoryAlertNotification {
  id: string;
  alertCode: string;
  timestamp: string;
  itemId: string;
  itemName: string;
  sku: string;
  severity: "critical" | "warning";
  channel: "email" | "push" | "both";
  currentStockFormatted: string;
  dailyBurnRateFormatted: string;
  runwayDays: number;
  emailSubject: string;
  emailBody: string;
  pushTitle: string;
  pushMessage: string;
  recipients: string[];
  deliveryStatus: "delivered" | "dispatched" | "failed";
  actionStatus: "pending_reorder" | "po_created" | "acknowledged" | "dismissed";
  linkedPoNumber?: string;
}



