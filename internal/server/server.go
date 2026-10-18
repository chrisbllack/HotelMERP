package server

import (
	"context"
	"database/sql"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"infineon/internal/auth"
	"infineon/internal/config"
	"infineon/internal/database"
	"infineon/internal/events"
	"infineon/internal/payment"
	"infineon/internal/server/handler"
	"infineon/internal/server/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func New(cfg *config.Config, db database.Pool, staticFiles fs.FS) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CorrelationID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Property-ID", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Auth & handlers
	jwtService := auth.NewJWTService([]byte(cfg.JWTSecret))
	h := handler.New(db, jwtService)

	// ADR-0006 contract bus: transactional outbox with in-process delivery.
	// Subscribers register before the boot requeue so recovered events from
	// a previous run dispatch immediately.
	outbox := events.NewOutbox(db)
	h.Outbox = outbox
	h.RegisterSubscribers(outbox)
	if n := outbox.RequeuePending(context.Background()); n > 0 {
		log.Printf("[events] Requeued %d pending domain event(s) from previous run", n)
	}

	// Initialize payment integration (Paystack + Stripe + Mock)
	if sqldb, ok := db.(interface{ RawDB() *sql.DB }); ok {
		payMgr := payment.NewManager(sqldb.RawDB(), payment.Config{
			ActiveProvider: cfg.PaymentProvider, // paystack, stripe, or mock
			Paystack: payment.PaystackConfig{
				SecretKey:     cfg.PaystackSecretKey,
				WebhookSecret: cfg.PaystackWebhookSecret,
			},
			Stripe: payment.StripeConfig{
				SecretKey:     cfg.StripeSecretKey,
				WebhookSecret: cfg.StripeWebhookSecret,
			},
		})
		h.PayV2 = handler.NewPaymentIntegration(payMgr)
		log.Printf("[server] Payment integration initialized: provider=%s", cfg.PaymentProvider)

		// payment.captured origin for the GL: fires exactly once per status
		// transition (verify or webhook, whichever lands second is a no-op).
		payMgr.OnCaptured(func(ctx context.Context, info payment.CaptureInfo) {
			pid := info.PropertyID
			if pid == "" {
				pid = "00000000-0000-0000-0000-000000000001"
			}
			id, err := outbox.Emit(ctx, db, events.EventInput{
				PropertyID: pid,
				Type:       events.PaymentCaptured,
				Source:     "payments",
				EntityID:   info.ReservationID,
				Payload: map[string]interface{}{
					"propertyId":    pid,
					"date":          info.PaidAt.Format("2006-01-02"),
					"method":        info.PaymentMethod,
					"amountKobo":    info.AmountKobo,
					"reference":     info.Reference,
					"description":   info.Provider + " payment — " + info.Reference,
					"reservationId": info.ReservationID,
				},
				EmittedBy: "system:payment",
			})
			if err != nil {
				log.Printf("[events] payment.captured emit failed for %s: %v", info.Reference, err)
				return
			}
			outbox.Schedule(id)
		})
	}

	// Serve static files if provided
	if staticFiles != nil {
		if sub, err := fs.Sub(staticFiles, "frontend/dist"); err == nil {
			fileServer := http.FileServer(http.FS(sub))
			r.Handle("/*", spaHandler(fileServer, sub))
		}
	}

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health
		r.Get("/health", h.Health)

		// Public (rate-limited credential endpoints)
		loginLimiter := middleware.NewIPRateLimiter(10, time.Minute)
		limited := func(next http.Handler) http.Handler {
			return middleware.LoginRateLimit(loginLimiter, next)
		}
		r.With(limited).Post("/auth/login", h.Login)
		r.With(limited).Post("/auth/register", h.Register)
		r.Post("/booking/search", h.SearchAvailability)
		r.Post("/booking/create", h.CreateBooking)
		r.Post("/ai/chat", h.ChatWithAI)

		// Payment webhooks (public — no auth, verified by signature)
		if h.PayV2 != nil {
			r.Post("/payments/webhook/paystack", h.PayV2.PaystackWebhook)
			r.Post("/payments/webhook/stripe", h.PayV2.StripeWebhook)
		}

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtService))
			if cfg.Mode == "demo" {
				r.Use(middleware.DemoPropertyID("00000000-0000-0000-0000-000000000001"))
			}
			// Pin tenancy to the JWT (header verified + injected), then
			// enforce module-level permission boundaries on every route.
			r.Use(middleware.PropertyScope())
			r.Use(middleware.ModuleRBAC())

			// M-FIN ledger (integer kobo, contract-posted money)
			r.Route("/gl", func(r chi.Router) {
				r.Get("/accounts", h.ListGLAccounts)
				r.Get("/trial-balance", h.GLTrialBalance)
				r.Get("/profit-and-loss", h.GLProfitAndLoss)
				r.Get("/entries", h.GLEntryList)
				r.Post("/journal", h.PostGLJournal)
			})

			// ADR-0006 contract bus observability (sec.admin inbox)
			r.Route("/domain-events", func(r chi.Router) {
				r.Get("/", h.ListDomainEvents)
				r.Get("/failed", h.ListFailedDomainEvents)
				r.Post("/{id}/retry", h.RetryDomainEvent)
			})

			// Users
			r.Route("/users", func(r chi.Router) {
				r.Get("/", h.ListUsers)
				r.Post("/", h.CreateUser)
				r.Get("/{id}", h.GetUser)
				r.Put("/{id}", h.UpdateUser)
				r.Delete("/{id}", h.DeleteUser)
			})

			// Guests
			r.Route("/guests", func(r chi.Router) {
				r.Get("/", h.ListGuests)
				r.Post("/", h.CreateGuest)
				r.Get("/search", h.GuestSearch)
				r.Get("/{id}", h.GetGuest)
				r.Put("/{id}", h.UpdateGuest)
				r.Delete("/{id}", h.DeleteGuest)
			})

			// Rooms
			r.Route("/rooms", func(r chi.Router) {
				r.Get("/", h.ListRooms)
				r.Post("/", h.CreateRoom)
				r.Get("/{id}", h.GetRoom)
				r.Put("/{id}", h.UpdateRoom)
				r.Delete("/{id}", h.DeleteRoom)
				r.Put("/{id}/status", h.UpdateRoomStatus)
			})

			// Room Types
			r.Route("/room-types", func(r chi.Router) {
				r.Get("/", h.ListRoomTypes)
				r.Post("/", h.CreateRoomType)
				r.Get("/{id}", h.GetRoomType)
				r.Put("/{id}", h.UpdateRoomType)
				r.Delete("/{id}", h.DeleteRoomType)
			})

			// Reservations
			r.Route("/reservations", func(r chi.Router) {
				r.Get("/", h.ListReservations)
				r.Post("/", h.CreateReservation)
				r.Get("/{id}", h.GetReservation)
				r.Put("/{id}", h.UpdateReservation)
				r.Post("/{id}/confirm", h.ConfirmReservation)
				r.Post("/{id}/cancel", h.CancelReservation)
				r.Post("/{id}/check-in", h.CheckIn)
				r.Post("/{id}/check-out", h.CheckOut)
				r.Post("/{id}/no-show", h.NoShow)
			})

			// Folios
			r.Route("/folios", func(r chi.Router) {
				r.Get("/stay/{stayId}", h.GetFolio)
				r.Post("/{folioId}/charges", h.PostCharge)
			})

			// Availability
			r.Get("/availability/check", h.CheckAvailability)

			// Front Desk
			r.Get("/front-desk/dashboard", h.FrontDeskDashboard)

			// Properties
			r.Route("/properties", func(r chi.Router) {
				r.Get("/", h.ListProperties)
				r.Post("/", h.CreateProperty)
				r.Get("/{id}", h.GetProperty)
				r.Put("/{id}", h.UpdateProperty)
			})

			// POS
			r.Route("/pos", func(r chi.Router) {
				r.Route("/outlets", func(r chi.Router) {
					r.Get("/", h.ListOutlets)
					r.Post("/", h.CreateOutlet)
				})
				r.Route("/tables", func(r chi.Router) {
					r.Get("/", h.ListRestaurantTables)
				})
				r.Route("/menu", func(r chi.Router) {
					r.Get("/categories", h.ListMenuCategories)
					r.Get("/items", h.ListMenuItems)
				})
				r.Route("/orders", func(r chi.Router) {
					r.Post("/", h.CreatePOSOrder)
					r.Get("/{orderId}", h.GetPOSOrder)
					r.Post("/{orderId}/close", h.ClosePOSOrder)
				})
			})

			// KDS
			r.Get("/kds/orders", h.GetKDSOrders)
			r.Put("/kds/orders/status", h.UpdateKDSOrderStatus)

			// Night Audit
			r.Post("/night-audit/run", h.RunNightAudit)
			r.Get("/night-audit", h.GetNightAuditStatus)

			// Cashier
			r.Post("/cashier/shift/open", h.OpenCashierShift)
			r.Post("/cashier/shift/close", h.CloseCashierShift)

			// Inventory
			r.Route("/inventory", func(r chi.Router) {
				r.Route("/products", func(r chi.Router) {
					r.Get("/", h.ListProducts)
					r.Post("/", h.CreateProduct)
					r.Get("/{id}", h.GetProduct)
					r.Put("/{id}", h.UpdateProduct)
					r.Delete("/{id}", h.DeleteProduct)
				})
				r.Route("/stock", func(r chi.Router) {
					r.Get("/", h.GetStock)
					r.Post("/movement", h.RecordStockMovement)
				})
				r.Route("/stores", func(r chi.Router) {
					r.Get("/", h.ListStores)
					r.Post("/", h.CreateStore)
				})
				r.Route("/suppliers", func(r chi.Router) {
					r.Get("/", h.ListSuppliers)
					r.Post("/", h.CreateSupplier)
				})
				r.Route("/purchase-orders", func(r chi.Router) {
					r.Get("/", h.ListPurchaseOrders)
					r.Post("/", h.CreatePurchaseOrder)
				})
			})

			// Procurement
			r.Route("/procurement", func(r chi.Router) {
				r.Get("/grn", h.ListGoodsReceipts)
				r.Post("/grn", h.CreateGoodsReceipt)
				r.Post("/three-way-match", h.ThreeWayMatch)
				r.Post("/stock-count", h.CreateStockCount)
				r.Post("/stock-count/{id}/submit", h.SubmitStockCount)
				r.Get("/valuation", h.GetInventoryValuation)
			})

			// Recipes
			r.Route("/recipes", func(r chi.Router) {
				r.Get("/", h.ListRecipes)
				r.Get("/ingredients", h.GetRecipe)
				r.Post("/save", h.SaveRecipe)
				r.Get("/costing-report", h.GetMenuCostingReport)
			})

			// Wastage
			r.Route("/wastage", func(r chi.Router) {
				r.Get("/", h.ListWastage)
				r.Post("/", h.RecordWastage)
			})

			// Finance
			r.Route("/finance", func(r chi.Router) {
				r.Route("/accounts", func(r chi.Router) {
					r.Get("/classes", h.ListAccountClasses)
					r.Get("/", h.ListAccounts)
					r.Post("/", h.CreateAccount)
					r.Get("/{id}", h.GetAccount)
				})
				r.Route("/journal", func(r chi.Router) {
					r.Get("/", h.ListJournalEntries)
					r.Post("/", h.CreateJournalEntry)
					r.Get("/{id}", h.GetJournalEntry)
					r.Post("/{id}/post", h.PostJournalEntry)
				})
				r.Route("/ar", func(r chi.Router) {
					r.Get("/", h.ListARInvoices)
					r.Post("/", h.CreateARInvoice)
				})
				r.Route("/ap", func(r chi.Router) {
					r.Get("/", h.ListAPInvoices)
					r.Post("/", h.CreateAPInvoice)
				})
				r.Route("/bank", func(r chi.Router) {
					r.Get("/", h.ListBankAccounts)
					r.Post("/", h.CreateBankAccount)
				})
				r.Route("/budgets", func(r chi.Router) {
					r.Get("/", h.ListBudgets)
					r.Post("/", h.CreateBudget)
				})
				r.Get("/profit-loss", h.GetProfitLoss)
			})

			// ══ Comprehensive Accounting Module ══
			// 1. Chart of Accounts (hierarchy)
			r.Get("/finance/coa", h.ListChartOfAccounts)

			// 2. Journal Entries v2 (debit/credit with audit trail)
			r.Post("/finance/journal-entries", h.CreateJournalEntryV2)

			// 3. Transactions (categorized, searchable)
			r.Get("/finance/transactions", h.ListTransactions)

			// 4. Account Reconciliation
			r.Get("/finance/reconciliation/{bankId}", h.GetReconciliation)
			r.Post("/finance/reconciliation/match", h.MatchReconItem)

			// 5. Transfer Accounts (with approval workflow)
			r.Get("/finance/transfers", h.ListTransfers)
			r.Post("/finance/transfers", h.CreateTransfer)
			r.Post("/finance/transfers/{id}/approve", h.ApproveTransfer)

			// 6. Banking Reconciliation (bank feed sync)
			r.Post("/finance/bank-sync", h.SyncBankFeed)

			// 7. Budget Management (per department, actual vs planned)
			r.Get("/finance/budgets-v2", h.ListBudgetsV2)

			// 8. Register (running balance per account)
			r.Get("/finance/register/{accountId}", h.GetAccountRegister)

			// 9. Profit & Loss v2
			r.Get("/finance/profit-loss-v2", h.GetProfitLossV2)

			// 10. Balance Sheet
			r.Get("/finance/balance-sheet", h.GetBalanceSheet)

			// 11. General Ledger (filterable)
			r.Get("/finance/general-ledger", h.GetGeneralLedger)

			// 12. Trial Balance
			r.Get("/finance/trial-balance", h.GetTrialBalance)

			// 13. Statement of Cash Flows
			r.Get("/finance/cash-flow", h.GetCashFlowStatement)

			// 14. Cheque Details
			r.Get("/finance/cheques", h.ListCheques)
			r.Post("/finance/cheques", h.IssueCheque)
			r.Put("/finance/cheques/{id}/status", h.UpdateChequeStatus)

			// 15. AR Aging
			r.Get("/finance/ar-aging", h.GetARAging)

			// 16. AP Aging
			r.Get("/finance/ap-aging", h.GetAPAging)

			// 17. Sales Tax Reports (VAT/FIRS/LIRS)
			r.Get("/finance/sales-tax", h.GetSalesTaxReport)

			// ══ Expense Management Module ══
			r.Get("/finance/expenses/categories", h.ListExpenseCategories)
			r.Get("/finance/expenses", h.ListExpensesV2)
			r.Post("/finance/expenses", h.CreateExpense)
			r.Post("/finance/expenses/bulk", h.BulkCreateExpenses)
			r.Post("/finance/expenses/{id}/approve", h.ApproveExpenseV2)
			r.Delete("/finance/expenses/{id}", h.DeleteExpense)
			r.Get("/finance/expenses/summary", h.GetExpenseSummary)
			r.Get("/finance/expenses/by-department", h.GetExpenseByDepartment)
			r.Get("/finance/expenses/trend", h.GetExpenseTrend)

			// ══ Invoicing Module ══
			r.Post("/invoices", h.CreateInvoiceV2)
			r.Get("/invoices", h.ListInvoicesV2)
			r.Get("/invoices/{id}", h.GetInvoiceDetail)
			r.Post("/invoices/{id}/pay", h.RecordInvoicePayment)
			r.Post("/invoices/{id}/void", h.VoidInvoice)
			r.Get("/invoices/{id}/pdf", h.ExportInvoicePDF)
			r.Post("/invoices/{id}/convert-quote", h.ConvertQuoteToInvoice)
			r.Get("/invoices/analytics", h.GetInvoiceAnalytics)
			r.Post("/invoices/customers", h.CreateInvoiceCustomer)
			r.Get("/invoices/customers", h.ListInvoiceCustomers)
			r.Post("/invoices/credit-notes", h.CreateCreditNote)
			r.Get("/invoices/credit-notes", h.ListCreditNotes)

			// ══ Bar Management Module ══
			r.Route("/bar", func(r chi.Router) {
				// Tabs
				r.Post("/tabs", h.OpenBarTab)
				r.Get("/tabs", h.ListOpenTabs)
				r.Get("/tabs/{tabId}", h.GetBarTab)
				r.Post("/tabs/{tabId}/items", h.AddTabItem)
				r.Post("/tabs/{tabId}/close", h.CloseBarTab)
				r.Post("/tabs/{tabId}/split", h.SplitBarTab)

				// Drinks
				r.Get("/drinks", h.ListDrinkMenu)
				r.Post("/drinks", h.CreateDrink)

				// Stock
				r.Get("/inventory", h.GetBarInventory)
				r.Post("/pour", h.RecordBarPour)
				r.Get("/variance", h.GetBarVarianceReport)

				// Tables
				r.Get("/floor-plan", h.GetBarFloorPlan)
				r.Post("/tables", h.CreateBarTable)

				// Staff
				r.Post("/staff/clock-in", h.BarStaffClockIn)
				r.Post("/staff/{shiftId}/clock-out", h.BarStaffClockOut)
				r.Get("/staff/shifts", h.ListBarShifts)

				// Pricing
				r.Get("/happy-hour", h.ListHappyHourRules)
				r.Post("/happy-hour", h.CreateHappyHourRule)

				// Recipes
				r.Get("/recipes", h.GetDrinkRecipes)

				// Analytics
				r.Get("/analytics", h.GetBarAnalytics)

				// CRM
				r.Get("/guests", h.ListBarGuests)
				r.Post("/guests", h.CreateBarGuest)
			})

			// ══ Restaurant Management Module ══
			r.Route("/restaurant", func(r chi.Router) {
				// POS & Orders
				r.Post("/orders", h.CreateRestaurantOrder)
				r.Get("/orders", h.ListRestaurantOrders)
				r.Post("/orders/{orderId}/status", h.UpdateOrderStatus)
				r.Post("/orders/{orderId}/pay", h.ProcessRestaurantPayment)

				// Kitchen Display System
				r.Get("/kds", h.GetKitchenDisplay)

				// Floor Plan
				r.Get("/floor-plan", h.GetRestaurantFloorPlan)
				r.Post("/tables/{tableId}/waiter", h.UpdateTableWaiter)

				// Menu
				r.Get("/menu", h.ListRestaurantMenu)
				r.Post("/menu", h.CreateMenuItem)

				// Delivery
				r.Post("/deliveries", h.CreateDeliveryOrder)
				r.Post("/deliveries/{deliveryId}/assign", h.AssignDeliveryRider)
				r.Get("/deliveries", h.ListDeliveries)

				// Ingredients
				r.Get("/ingredients", h.ListIngredients)
				r.Post("/ingredients", h.CreateIngredient)

				// Analytics
				r.Get("/analytics", h.GetRestaurantAnalytics)

				// CRM
				r.Get("/guests", h.ListRestaurantGuests)
				r.Post("/guests", h.CreateRestaurantGuest)

				// Supplier Invoices
				r.Get("/supplier-invoices", h.ListSupplierInvoices)
			})

			// Bank Reconciliation
			r.Route("/bank-rec", func(r chi.Router) {
				r.Get("/", h.ListBankReconciliations)
				r.Post("/", h.CreateBankReconciliation)
			})

			// Fixed Assets
			r.Route("/assets", func(r chi.Router) {
				r.Get("/", h.ListFixedAssets)
				r.Post("/", h.CreateFixedAsset)
				r.Post("/depreciation", h.RunDepreciation)
			})

			// Expenses
			r.Route("/expenses", func(r chi.Router) {
				r.Get("/", h.ListExpenseClaims)
				r.Post("/", h.CreateExpenseClaim)
				r.Put("/{id}/approve", h.ApproveExpense)
			})

			// HR
			r.Route("/hr", func(r chi.Router) {
				r.Route("/employees", func(r chi.Router) {
					r.Get("/", h.ListEmployees)
					r.Post("/", h.CreateEmployee)
					r.Get("/{id}", h.GetEmployee)
					r.Put("/{id}", h.UpdateEmployee)
				})
				r.Route("/attendance", func(r chi.Router) {
					r.Get("/", h.ListAttendance)
					r.Post("/clock-in", h.ClockIn)
					r.Post("/clock-out", h.ClockOut)
				})
				r.Route("/leave", func(r chi.Router) {
					r.Get("/types", h.ListLeaveTypes)
					r.Get("/requests", h.ListLeaveRequests)
					r.Post("/requests", h.CreateLeaveRequest)
				})
				r.Route("/payroll", func(r chi.Router) {
					r.Get("/", h.ListPayrollRunsEnhanced)
					r.Post("/run", h.RunPayroll)
					r.Get("/{id}", h.GetPayrollRun)
					r.Post("/{runId}/approve", h.ApprovePayroll)
					r.Post("/{runId}/pay", h.ProcessPayrollPayment)
					r.Get("/{runId}/payslips", h.GeneratePayslips)
					r.Get("/{runId}/payslips/{empId}", h.GetPayslip)
					r.Get("/{runId}/export", h.ExportBankPaymentFile)
					r.Get("/{runId}/report", h.PayrollSummaryReport)
					r.Get("/{runId}/cost-by-dept", h.CostByDepartmentReport)
					r.Post("/{runId}/post-ledger", h.PostPayrollToLedger)
					r.Get("/config", h.GetPayrollConfig)
					r.Put("/config", h.UpdatePayrollConfig)
					r.Get("/multi-branch/{runId}", h.MultiBranchPayrollSummary)
				})
				r.Get("/expenses", h.ListExpenseClaims)

				// ── Enterprise HR Extensions ──
				r.Get("/live-attendance", h.LiveAttendanceDashboard)
				r.Get("/department-staffing", h.DepartmentStaffing)
				r.Get("/dashboard", h.HRDashboard)
				r.Get("/employees-dir", h.ListEmployeesDir)
				r.Get("/employees/{id}/detail", h.GetEmployeeDetail)
				r.Route("/shifts", func(r chi.Router) {
					r.Get("/", h.ListCashShifts)
					r.Post("/", h.CreateShift)
				})
				r.Route("/rosters", func(r chi.Router) {
					r.Get("/", h.ListRosters)
					r.Get("/{id}/assignments", h.GetRosterAssignments)
				})
				r.Route("/devices", func(r chi.Router) {
					r.Get("/", h.ListAttendanceDevices)
					r.Post("/", h.CreateAttendanceDevice)
					r.Post("/{id}/sync", h.SyncAttendanceDevice)
					r.Get("/sync-jobs", h.ListSyncJobs)
				})
				r.Route("/attendance-records", func(r chi.Router) {
					r.Get("/", h.ListAttendanceRecords)
				})
				r.Put("/leave/requests/{id}/approve", h.ApproveLeaveRequest)
				r.Get("/holidays", h.ListPublicHolidays)
				r.Get("/leave-types-all", h.ListLeaveTypesAll)
				r.Get("/leave-requests-all", h.ListLeaveRequestsAll)
				r.Get("/training", h.ListTrainingCourses)
				r.Get("/performance", h.ListPerformanceCycles)
			})

			// ── Employee Self-Service (ESS) ──
			r.Route("/ess", func(r chi.Router) {
				r.Get("/my-profile", h.MyProfile)
				r.Get("/my-attendance", h.GetMyAttendance)
				r.Get("/my-leaves", h.MyLeaveRequests)
				r.Post("/my-leaves", h.SubmitLeaveRequest)
				r.Get("/my-payslips", h.GetMyPayslips)
				r.Get("/my-leave-balance", h.GetMyLeaveBalance)
			})

			// ── Manager Self-Service (MSS) ──
			r.Route("/mss", func(r chi.Router) {
				r.Get("/my-team", h.MyTeam)
				r.Get("/my-team/attendance", h.MyTeamAttendance)
				r.Get("/pending-leaves", h.PendingLeaveApprovals)
				r.Post("/approve-leave/{id}", h.ApproveLeave)
			})

			// CRM
			r.Route("/crm", func(r chi.Router) {
				r.Get("/guests", h.CRMGuestSearch)
				r.Get("/guests/{id}/profile", h.GetGuestProfile)
				r.Put("/guests/{id}/preferences", h.UpdateGuestPreferences)
				r.Get("/loyalty/tiers", h.GetLoyaltyTiers)
				r.Post("/loyalty/award", h.AwardLoyaltyPoints)
				r.Post("/feedback", h.SubmitFeedback)
				r.Get("/feedback", h.ListFeedback)
			})

			// Front Desk Intelligence
			r.Get("/front-desk/intelligence", h.FrontDeskIntelligence)
			r.Get("/front-desk/chart", h.ReservationChart)
			r.Post("/front-desk/move-reservation", h.DragDropMoveReservation)
			r.Get("/guests/{id}/360", h.GetGuest360)
			r.Route("/traces", func(r chi.Router) {
				r.Get("/", h.ListTraces)
				r.Post("/", h.CreateTrace)
				r.Put("/{id}/status", h.UpdateTraceStatus)
			})
			r.Get("/reservations/{id}/suggest-room", h.SuggestRoomAssignment)
			r.Post("/reservations/assign-room", h.AssignRoom)
			r.Route("/room-queue", func(r chi.Router) {
				r.Get("/", h.ListRoomQueue)
				r.Post("/", h.AddToRoomQueue)
			})

			// Events
			r.Route("/events", func(r chi.Router) {
				r.Get("/", h.ListEvents)
				r.Post("/", h.CreateEvent)
				r.Put("/{id}/status", h.UpdateEventStatus)
				r.Get("/venues", h.ListEventVenues)
			})

			// Channels (legacy route kept, comprehensive routes below)

			// Payments (legacy)
			r.Route("/payments", func(r chi.Router) {
				r.Post("/initiate", h.InitiatePayment)
				r.Post("/refund", h.ProcessRefund)
			})

			// Payments v2 (Production-grade with Paystack/Stripe integration)
			if h.PayV2 != nil {
				r.Route("/payments/v2", func(r chi.Router) {
					r.Post("/initiate", h.PayV2.InitiatePayment)
					r.Get("/verify/{reference}", h.PayV2.VerifyPayment)
					r.Post("/refund", h.PayV2.ProcessRefund)
					r.Get("/transactions", h.PayV2.ListTransactions)
					r.Get("/summary", h.PayV2.PaymentSummary)
					r.Get("/methods", h.PayV2.ListPaymentMethods)
					r.Post("/simulate-webhook", h.PayV2.SimulateWebhook)
				})
			}

			// Hardware
			r.Route("/hardware", func(r chi.Router) {
				r.Get("/devices", h.ListDevices)
				r.Post("/devices", h.RegisterDevice)
				r.Get("/health", h.DeviceHealthCheck)
				r.Post("/door-lock/encode", h.EncodeKeyCard)
				r.Get("/logs", h.HardwareLogs)
			})

			// Notifications
			r.Route("/notifications", func(r chi.Router) {
				r.Post("/send", h.SendNotification)
				r.Get("/", h.ListNotifications)
			})

			// Workflow
			r.Route("/workflow", func(r chi.Router) {
				r.Get("/approvals", h.ListPendingApprovals)
				r.Post("/approvals", h.ApproveRevenueRequest)
			})

			// Documents
			r.Route("/documents", func(r chi.Router) {
				r.Post("/upload", h.UploadDocument)
				r.Get("/", h.ListDocuments)
			})

			// Backups
			r.Route("/backups", func(r chi.Router) {
				r.Post("/create", h.CreateBackup)
				r.Get("/", h.ListBackups)
				r.Post("/restore", h.RestoreBackup)
				r.Get("/export", h.ExportData)
			})

			// Audit
			r.Get("/audit-logs", h.ListAuditLogs)

			// Config
			r.Route("/config", func(r chi.Router) {
				r.Get("/", h.GetConfig)
				r.Put("/", h.UpdateConfig)
			})

			// ── Settings Management ──
			r.Route("/settings", func(r chi.Router) {
				r.Get("/property", h.GetPropertySettings)
				r.Put("/property", h.UpdatePropertySettings)
				r.Get("/system-info", h.GetSystemInfo)
				r.Get("/db-stats", h.GetDatabaseStats)
				r.Get("/users", h.ListAllUsers)
				r.Get("/tax-rates", h.ListTaxRates)
				r.Post("/tax-rates", h.CreateTaxRate)
				r.Put("/tax-rates/{id}", h.UpdateTaxRate)
				r.Delete("/tax-rates/{id}", h.DeleteTaxRate)
				r.Get("/payment-methods", h.ListPaymentMethods)
				r.Post("/payment-methods", h.CreatePaymentMethod)
				r.Put("/payment-methods/{id}", h.UpdatePaymentMethod)
				r.Delete("/payment-methods/{id}", h.DeletePaymentMethod)
				r.Get("/departments", h.ListDepartments)
				r.Post("/departments", h.CreateDepartment)
				r.Put("/departments/{id}", h.UpdateDepartment)
				r.Delete("/departments/{id}", h.DeleteDepartment)
				r.Get("/numbering", h.ListNumberingSequences)
				r.Put("/numbering/{id}", h.UpdateNumberingSequence)
				r.Get("/system", h.ListSystemSettings)
				r.Put("/system", h.UpdateSystemSettings)
			})

			// ── Revenue Management (Opera/Djubo-style) ──
			r.Route("/revenue", func(r chi.Router) {
				r.Get("/dashboard", h.RevenueDashboard)
				r.Get("/forecast", h.GetRevenueForecast)
				r.Route("/pricing-rules", func(r chi.Router) {
					r.Get("/", h.ListPricingRules)
					r.Post("/", h.CreateDynamicPricingRule)
				})
			})

			// ── Group Bookings (Opera-style) ──
			r.Route("/group-bookings", func(r chi.Router) {
				r.Get("/", h.ListGroupBookings)
				r.Post("/", h.CreateGroupBooking)
				r.Get("/{id}", h.GetGroupBooking)
				r.Put("/{id}/status", h.UpdateGroupBookingStatus)
			})

			// ── Sales & Catering (Opera-style) ──
			r.Route("/event-venues", func(r chi.Router) {
				r.Get("/", h.ListEventVenues)
			})
			r.Route("/event-bookings", func(r chi.Router) {
				r.Get("/", h.ListEventBookings)
				r.Post("/", h.CreateEventBooking)
				r.Put("/{id}/status", h.UpdateEventBookingStatus)
			})

			// ── Guest Self-Service (eZee-style) ──
			r.Route("/service-requests", func(r chi.Router) {
				r.Get("/", h.ListServiceRequests)
				r.Post("/", h.CreateServiceRequest)
				r.Put("/{id}", h.UpdateServiceRequest)
			})

			// ── Shift Handover (Opera-style) ──
			r.Route("/shift-handovers", func(r chi.Router) {
				r.Get("/", h.ListShiftHandovers)
				r.Post("/", h.CreateShiftHandover)
			})

			// ── Loyalty Program (Opera/Djubo-style) ──
			r.Route("/loyalty", func(r chi.Router) {
				r.Get("/dashboard", h.LoyaltyDashboard)
				r.Route("/tiers", func(r chi.Router) {
					r.Get("/", h.ListLoyaltyTiers)
				})
				r.Route("/members", func(r chi.Router) {
					r.Get("/", h.ListLoyaltyMembers)
					r.Post("/enroll", h.EnrollLoyaltyMember)
					r.Put("/{id}/points", h.AddLoyaltyPoints)
				})
			})

			// Reports (25+ comprehensive reports)
			r.Route("/reports", func(r chi.Router) {
				r.Get("/dashboard", h.GetDashboard)
				r.Get("/occupancy", h.ReportOccupancySummary)
				r.Get("/occupancy/forecast", h.ReportOccupancyForecast)
				r.Get("/revenue", h.ReportDailyRevenue)
				r.Get("/revenue/room-type", h.ReportRevenueByRoomType)
				r.Get("/revenue/source", h.ReportRevenueBySource)
				r.Get("/pos-sales", h.ReportPOSSales)
				r.Get("/pos-sales/outlet", h.ReportPOSSalesByOutlet)
				r.Get("/reservations/source", h.ReportReservationsBySource)
				r.Get("/arrivals", h.ReportArrivals)
				r.Get("/departures", h.ReportDepartures)
				r.Get("/no-shows", h.ReportNoShows)
				r.Get("/cancellations", h.ReportCancellations)
				r.Get("/guest-ledger", h.ReportGuestLedger)
				r.Get("/guest-demographics", h.ReportGuestDemographics)
				r.Get("/vip-guests", h.ReportVIPGuests)
				r.Get("/housekeeping", h.ReportHousekeepingStatus)
				r.Get("/housekeeping/tasks", h.ReportHousekeepingTasks)
				r.Get("/inventory", h.ReportInventorySummary)
				r.Get("/inventory/low-stock", h.ReportLowStock)
				r.Get("/employee-headcount", h.ReportEmployeeHeadcount)
				r.Get("/payroll-summary", h.ReportPayrollSummary)
				r.Get("/attendance", h.ReportAttendance)
				r.Get("/leave", h.ReportLeaveSummary)
				r.Get("/loyalty", h.ReportLoyalty)
				r.Get("/events", h.ReportEventRevenue)

				// Night Audit
				r.Post("/night-audit/run", h.RunNightAudit)
				r.Get("/night-audit/status", h.GetNightAuditStatus)

				// Generator & Fuel Tracking (Africa-specific)
				r.Post("/facilities/generator-log", h.CreateGeneratorLog)
				r.Get("/facilities/generator-logs", h.ListGeneratorLogs)
				r.Post("/facilities/fuel-purchase", h.CreateFuelPurchase)
				r.Get("/facilities/fuel-summary", h.GeneratorFuelSummary)

				// Service Charge Distribution
				r.Get("/hr/service-charge-pool", h.ServiceChargePool)
				// Revenue Protection (Staycore-style)
				r.Post("/approval-request", h.CreateApprovalRequest)
				r.Get("/approval-requests", h.ListApprovalRequests)
				r.Post("/approve-request", h.ApproveRevenueRequest)

				// Shortlet Management
				r.Get("/shortlets", h.ListShortlets)
				r.Post("/shortlet-booking", h.CreateShortletBooking)
				r.Get("/shortlet-payouts", h.ShortletOwnerPayouts)

				// Guest Verification
				r.Post("/guest-verification", h.SubmitGuestVerification)
				r.Get("/guest-verifications", h.ListGuestVerifications)

				// Dynamic Pricing
				r.Get("/dynamic-rate", h.GetDynamicRate)
				r.Post("/pricing-rule", h.CreateDynamicPricingRule)

				// Forecasting
				r.Get("/revenue-forecast", h.RevenueForecast)

				// Restaurant Tables
				r.Get("/tables", h.ListRestaurantTables)
				r.Post("/table-status", h.UpdateTableStatus)

				// Owner Portal
				r.Get("/owner-dashboard", h.OwnerDashboard)
				r.Get("/owner-statement", h.OwnerStatement)

				// Recipe Costing
				r.Get("/recipe-costs", h.RecipeCostReport)

				// Tip Pool & Staff Scorecards
				r.Get("/tip-pool", h.TipPoolReport)
				r.Get("/staff-scorecards", h.StaffScorecard)

				// Time Tracking
				r.Post("/clock-in-out", h.ClockInOut)
				r.Get("/timesheet", h.Timesheet)
			})

			// Channel Manager (OTA Integration)
			r.Get("/channels", h.ListChannels)
			r.Get("/channels/dashboard", h.ChannelDashboard)
			r.Post("/channels/sync-rates", h.SyncRates)
			r.Post("/channels/sync-availability", h.SyncAvailability)
			r.Post("/channels/pull-bookings", h.PullChannelBookings)
			r.Get("/channels/performance", h.ChannelPerformance)
			r.Get("/channels/sync-log", h.ChannelSyncLog)
			r.Get("/channels/bookings", h.ChannelBookings)
			r.Put("/channels/settings", h.UpdateChannelSettings)
			// RBAC & Access Control
			r.Get("/permissions", h.ListPermissions)
			r.Get("/roles", h.ListRoles)

			// Shift & Cash Management
			r.Post("/shift/open", h.OpenShift)
			r.Post("/shift/close", h.CloseShift)
			r.Get("/shifts", h.ListCashShifts)

			// Folio Enhancements
			r.Post("/folio/split", h.SplitFolio)
			r.Post("/folio/transfer", h.TransferFolio)

			// Rate Management
			r.Post("/seasonal-rate", h.CreateSeasonalRate)
			r.Post("/business-rate", h.CreateBusinessSourceRate)
			r.Get("/best-rate", h.GetBestRate)

			// Profile Management
			r.Post("/company-profile", h.CreateCompanyProfile)
			r.Post("/travel-agent-profile", h.CreateTravelAgentProfile)
			r.Get("/company-profiles", h.ListCompanyProfiles)

			// Reservation Enhancements
			r.Get("/reservations/search", h.SearchReservations)
			r.Post("/reservations/group-checkout", h.GroupCheckOut)

			// Check-in/Check-out Enhancements
			r.Post("/check-in/early", h.ProcessEarlyCheckIn)
			r.Post("/check-out/late", h.ProcessLateCheckOut)
			r.Post("/day-use", h.ProcessDayUse)
			r.Post("/card-key", h.GenerateCardKey)
			r.Post("/reservation/void", h.VoidReservation)

			// Retail POS
			r.Post("/retail-sale", h.CreateRetailSale)
			r.Post("/retail-void", h.VoidSale)
			r.Get("/customer-balance", h.GetCustomerBalance)
			r.Get("/receipt/reprint", h.ReprintReceipt)
			r.Get("/executive", h.ExecutiveDashboard)
			r.Get("/export/csv", h.ExportReportCSV)
			r.Get("/export/pdf", h.ExportReportPDF)
			r.Get("/departments", h.ListDepartmentsOverview)
			r.Get("/departments/{department}", h.DepartmentReport)
		})
	})

	return r
}

func spaHandler(fileServer http.Handler, fs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}
		clean := strings.TrimPrefix(path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Open(clean); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
