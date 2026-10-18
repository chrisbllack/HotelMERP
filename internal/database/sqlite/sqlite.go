package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"os"
	"path/filepath"
	"sync"
	"time"
	"math"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB to provide a pgxpool.Pool-compatible interface for demo mode.
// This allows the entire INFINEON backend to run on SQLite without PostgreSQL.
type DB struct {
	*sql.DB
	path string
	mu   sync.RWMutex
}

// New creates or opens a SQLite database at the given path.
func New(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = filepath.Join(".", "infineon_demo.db")
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer
	db.SetMaxIdleConns(1)

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return &DB{DB: db, path: dbPath}, nil
}

// RunMigrations creates all tables needed for the demo.
func (d *DB) RunMigrations(ctx context.Context) error {
	migrations := []string{
		// ââ Users & Auth ââ
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			last_login_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// -- Channel Room Mappings --
		`CREATE TABLE IF NOT EXISTS channel_room_mappings (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			property_id TEXT NOT NULL,
			room_type_id TEXT NOT NULL,
			channel_room_id TEXT NOT NULL,
			channel_room_name TEXT,
			local_stock INTEGER DEFAULT 0,
			active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// -- Channel Rate Mappings --
		`CREATE TABLE IF NOT EXISTS channel_rate_mappings (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			property_id TEXT NOT NULL,
			room_type_id TEXT NOT NULL,
			rate_plan_id TEXT,
			channel_rate_id TEXT NOT NULL,
			channel_rate_name TEXT,
			base_price REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// -- Channel Sync Logs --
		`CREATE TABLE IF NOT EXISTS channel_sync_logs (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			property_id TEXT NOT NULL,
			sync_type TEXT NOT NULL,
			status TEXT NOT NULL,
			rooms_synced INTEGER DEFAULT 0,
			rates_synced INTEGER DEFAULT 0,
			bookings_pulled INTEGER DEFAULT 0,
			error_message TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,

		// -- Channel Bookings (OTA reservations pulled) --
		`CREATE TABLE IF NOT EXISTS channel_bookings (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			property_id TEXT NOT NULL,
			reservation_id TEXT,
			channel_booking_ref TEXT NOT NULL,
			guest_name TEXT,
			guest_email TEXT,
			room_type TEXT,
			check_in DATE,
			check_out DATE,
			nights INTEGER DEFAULT 1,
			adults INTEGER DEFAULT 1,
			children INTEGER DEFAULT 0,
			total_amount REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			status TEXT DEFAULT 'new',
			raw_payload TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Properties ââ
		`CREATE TABLE IF NOT EXISTS properties (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			code TEXT UNIQUE,
			address TEXT,
			city TEXT,
			state TEXT,
			country TEXT DEFAULT 'Nigeria',
			telephone TEXT,
			email TEXT,
			currency TEXT DEFAULT 'NGN',
			timezone TEXT DEFAULT 'Africa/Lagos',
			service_charge_rate REAL DEFAULT 10,
			vat_rate REAL DEFAULT 7.5,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Buildings & Floors ââ
		`CREATE TABLE IF NOT EXISTS buildings (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS floors (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			building_id TEXT REFERENCES buildings(id),
			name TEXT NOT NULL,
			level INTEGER NOT NULL,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Room Types ââ
		`CREATE TABLE IF NOT EXISTS room_types (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			max_occupancy INTEGER DEFAULT 2,
			adult_capacity INTEGER DEFAULT 2,
			child_capacity INTEGER DEFAULT 1,
			bed_configuration TEXT,
			room_size REAL,
			view TEXT,
			base_rate REAL DEFAULT 0,
			min_rate REAL DEFAULT 0,
			max_rate REAL DEFAULT 0,
			status TEXT DEFAULT 'active',
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			UNIQUE(property_id, code)
		)`,

		// ââ Rooms ââ
		`CREATE TABLE IF NOT EXISTS rooms (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			room_type_id TEXT NOT NULL REFERENCES room_types(id),
			number TEXT NOT NULL,
			building_id TEXT REFERENCES buildings(id),
			floor_id TEXT REFERENCES floors(id),
			is_smoking INTEGER DEFAULT 0,
			status TEXT DEFAULT 'vacant_dirty',
			housekeeping_status TEXT DEFAULT 'dirty',
			maintenance_status TEXT DEFAULT 'operational',
			status_changed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			UNIQUE(property_id, number)
		)`,

		// ââ Guests ââ
		`CREATE TABLE IF NOT EXISTS guests (
			id TEXT PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email TEXT,
			phone TEXT,
			nationality TEXT,
			country_id TEXT,
			id_type TEXT,
			id_number TEXT,
			id_expiry DATE,
			date_of_birth DATE,
			gender TEXT,
			address TEXT,
			city TEXT,
			company TEXT,
			notes TEXT,
			marketing_consent INTEGER DEFAULT 0,
			blacklisted INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Rate Plans ââ
		`CREATE TABLE IF NOT EXISTS rate_plans (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			room_type_id TEXT REFERENCES room_types(id),
			base_price REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			tax_inclusive INTEGER DEFAULT 0,
			cancellation_policy TEXT,
			min_stay INTEGER DEFAULT 1,
			max_stay INTEGER DEFAULT 365,
			advance_booking_days INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Reservations ââ
		`CREATE TABLE IF NOT EXISTS reservations (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			confirmation_number TEXT NOT NULL,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			room_type_id TEXT REFERENCES room_types(id),
			rate_plan_id TEXT REFERENCES rate_plans(id),
			room_id TEXT REFERENCES rooms(id),
			arrival_date DATE NOT NULL,
			departure_date DATE NOT NULL,
			nights INTEGER NOT NULL,
			adults INTEGER DEFAULT 1,
			children INTEGER DEFAULT 0,
			room_count INTEGER DEFAULT 1,
			status TEXT DEFAULT 'tentative',
			source TEXT DEFAULT 'direct',
			market_segment_id TEXT,
			channel_id TEXT,
			corporate_account_id TEXT,
			travel_agent_id TEXT,
			payment_method TEXT,
			deposit_amount REAL DEFAULT 0,
			deposit_paid REAL DEFAULT 0,
			total_estimated REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			exchange_rate REAL DEFAULT 1,
			is_vip INTEGER DEFAULT 0,
			special_requests TEXT,
			notes TEXT,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Guest Stays ââ
		`CREATE TABLE IF NOT EXISTS guest_stays (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			reservation_id TEXT NOT NULL REFERENCES reservations(id),
			guest_id TEXT NOT NULL REFERENCES guests(id),
			room_id TEXT REFERENCES rooms(id),
			expected_arrival DATE,
			expected_departure DATE,
			actual_arrival DATETIME,
			actual_departure DATETIME,
			adults INTEGER DEFAULT 1,
			children INTEGER DEFAULT 0,
			status TEXT DEFAULT 'expected',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Folios ââ
		`CREATE TABLE IF NOT EXISTS folios (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			guest_stay_id TEXT NOT NULL REFERENCES guest_stays(id),
			folio_number TEXT NOT NULL,
			status TEXT DEFAULT 'open',
			balance REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS folio_lines (
			id TEXT PRIMARY KEY,
			folio_id TEXT NOT NULL REFERENCES folios(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			type TEXT NOT NULL,
			charge_code TEXT NOT NULL,
			description TEXT,
			quantity REAL DEFAULT 1,
			unit_price REAL DEFAULT 0,
			amount REAL DEFAULT 0,
			tax_amount REAL DEFAULT 0,
			business_date DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ POS ââ
		`CREATE TABLE IF NOT EXISTS outlets (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS pos_tables (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			outlet_id TEXT NOT NULL REFERENCES outlets(id),
			number TEXT NOT NULL,
			capacity INTEGER DEFAULT 4,
			status TEXT DEFAULT 'available',
			section TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS menu_categories (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			outlet_id TEXT NOT NULL REFERENCES outlets(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			display_order INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS menu_items (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			category_id TEXT NOT NULL REFERENCES menu_categories(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			price REAL DEFAULT 0,
			tax_inclusive INTEGER DEFAULT 0,
			is_available INTEGER DEFAULT 1,
			preparation_time INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS pos_orders (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			outlet_id TEXT NOT NULL REFERENCES outlets(id),
			order_number TEXT NOT NULL,
			table_id TEXT REFERENCES pos_tables(id),
			room_id TEXT,
			guest_stay_id TEXT,
			status TEXT DEFAULT 'open',
			subtotal REAL DEFAULT 0,
			tax_amount REAL DEFAULT 0,
			discount_amount REAL DEFAULT 0,
			total REAL DEFAULT 0,
			payment_status TEXT DEFAULT 'unpaid',
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS pos_order_lines (
			id TEXT PRIMARY KEY,
			order_id TEXT NOT NULL REFERENCES pos_orders(id),
			menu_item_id TEXT NOT NULL REFERENCES menu_items(id),
			quantity REAL DEFAULT 1,
			unit_price REAL DEFAULT 0,
			discount REAL DEFAULT 0,
			amount REAL DEFAULT 0,
			notes TEXT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Inventory ââ
		`CREATE TABLE IF NOT EXISTS inventory_stores (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			type TEXT NOT NULL,
			location TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS inventory_products (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			sku TEXT NOT NULL,
			name TEXT NOT NULL,
			category TEXT,
			subcategory TEXT,
			unit_of_measure TEXT DEFAULT 'unit',
			cost_price REAL DEFAULT 0,
			selling_price REAL DEFAULT 0,
			reorder_level REAL DEFAULT 0,
			maximum_stock REAL DEFAULT 0,
			minimum_stock REAL DEFAULT 0,
			supplier_id TEXT,
			barcode TEXT,
			tax_rate REAL DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			UNIQUE(property_id, sku)
		)`,

		`CREATE TABLE IF NOT EXISTS inventory_stock (
			id TEXT PRIMARY KEY,
			product_id TEXT NOT NULL REFERENCES inventory_products(id),
			store_id TEXT NOT NULL REFERENCES inventory_stores(id),
			quantity REAL DEFAULT 0,
			cost_price REAL DEFAULT 0,
			batch_number TEXT DEFAULT 'DEFAULT',
			expiry_date DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(product_id, store_id, batch_number)
		)`,

		`CREATE TABLE IF NOT EXISTS inventory_movements (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			product_id TEXT NOT NULL REFERENCES inventory_products(id),
			from_store_id TEXT REFERENCES inventory_stores(id),
			to_store_id TEXT REFERENCES inventory_stores(id),
			quantity REAL NOT NULL,
			unit_cost REAL DEFAULT 0,
			reference_type TEXT,
			reference_id TEXT,
			reason TEXT,
			batch_number TEXT,
			performed_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Suppliers ââ
		`CREATE TABLE IF NOT EXISTS suppliers (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			contact_person TEXT,
			email TEXT,
			phone TEXT,
			address TEXT,
			tax_number TEXT,
			bank_name TEXT,
			bank_account TEXT,
			payment_terms_days INTEGER DEFAULT 30,
			credit_limit REAL DEFAULT 0,
			rating INTEGER DEFAULT 3,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Purchase Orders ââ
		`CREATE TABLE IF NOT EXISTS purchase_orders (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			po_number TEXT NOT NULL,
			supplier_id TEXT NOT NULL REFERENCES suppliers(id),
			order_date DATE,
			expected_date DATE,
			total REAL DEFAULT 0,
			status TEXT DEFAULT 'pending',
			approved_by TEXT,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS purchase_order_lines (
			id TEXT PRIMARY KEY,
			po_id TEXT NOT NULL REFERENCES purchase_orders(id),
			product_id TEXT NOT NULL REFERENCES inventory_products(id),
			quantity REAL NOT NULL,
			unit_price REAL DEFAULT 0,
			amount REAL DEFAULT 0,
			received_quantity REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Night Audit ââ
		`CREATE TABLE IF NOT EXISTS night_audits (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			business_date DATE NOT NULL,
			total_rooms INTEGER DEFAULT 0,
			occupied_rooms INTEGER DEFAULT 0,
			arrivals INTEGER DEFAULT 0,
			departures INTEGER DEFAULT 0,
			no_shows INTEGER DEFAULT 0,
			total_room_charges REAL DEFAULT 0,
			total_tax REAL DEFAULT 0,
			total_payments REAL DEFAULT 0,
			status TEXT DEFAULT 'pending',
			completed_by TEXT,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Chart of Accounts ââ
		`CREATE TABLE IF NOT EXISTS chart_of_accounts (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			account_code TEXT NOT NULL,
			account_name TEXT NOT NULL,
			account_class TEXT NOT NULL,
			account_type TEXT,
			description TEXT,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			UNIQUE(property_id, account_code)
		)`,

		// ââ Journal Entries ââ
		`CREATE TABLE IF NOT EXISTS journal_entries (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			entry_number TEXT NOT NULL,
			entry_date DATE NOT NULL,
			period_year INTEGER,
			period_month INTEGER,
			description TEXT,
			status TEXT DEFAULT 'draft',
			posted_by TEXT,
			posted_at DATETIME,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS journal_lines (
			id TEXT PRIMARY KEY,
			entry_id TEXT NOT NULL REFERENCES journal_entries(id),
			account_id TEXT NOT NULL REFERENCES chart_of_accounts(id),
			debit REAL DEFAULT 0,
			credit REAL DEFAULT 0,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Employees ââ
		`CREATE TABLE IF NOT EXISTS employees (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			employee_number TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			middle_name TEXT,
			email TEXT,
			phone TEXT,
			department_id TEXT,
			position TEXT,
			employment_date DATE,
			salary REAL DEFAULT 0,
			employment_status TEXT DEFAULT 'active',
			user_id TEXT,
			reporting_manager TEXT,
			hire_date DATE,
			status TEXT DEFAULT 'active',
			bank_name TEXT,
			bank_account TEXT,
			emergency_contact TEXT,
			emergency_phone TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			UNIQUE(property_id, employee_number)
		)`,

		// ââ Attendance ââ
		`CREATE TABLE IF NOT EXISTS attendance (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			date DATE NOT NULL,
			clock_in TIME,
			clock_out TIME,
			hours_worked REAL DEFAULT 0,
			overtime_hours REAL DEFAULT 0,
			status TEXT DEFAULT 'present',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ CRM / Loyalty ââ
		`CREATE TABLE IF NOT EXISTS guest_preferences (
			id TEXT PRIMARY KEY,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			preference_type TEXT NOT NULL,
			preference_value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS loyalty_tiers (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			min_points INTEGER DEFAULT 0,
			discount_percent REAL DEFAULT 0,
			benefits TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS loyalty_accounts (
			id TEXT PRIMARY KEY,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			tier_id TEXT REFERENCES loyalty_tiers(id),
			points_balance INTEGER DEFAULT 0,
			total_points_earned INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Audit Log ââ
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			module TEXT NOT NULL,
			action TEXT NOT NULL,
			entity_id TEXT,
			old_value TEXT,
			new_value TEXT,
			ip_address TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Notifications ââ
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			property_id TEXT,
			notification_type TEXT NOT NULL,
			template_code TEXT,
			recipient TEXT NOT NULL,
			subject TEXT,
			body TEXT,
			status TEXT DEFAULT 'pending',
			sent_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Cashier Shifts ââ
		`CREATE TABLE IF NOT EXISTS cashier_shifts (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			shift_number TEXT NOT NULL,
			user_id TEXT,
			opening_float REAL DEFAULT 0,
			closing_float REAL,
			total_sales REAL DEFAULT 0,
			total_refunds REAL DEFAULT 0,
			status TEXT DEFAULT 'open',
			opened_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		)`,

		// ââ Hardware Devices ââ
		`CREATE TABLE IF NOT EXISTS hardware_devices (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			device_type TEXT NOT NULL,
			vendor TEXT,
			model TEXT,
			serial_number TEXT,
			location TEXT,
			status TEXT DEFAULT 'active',
			last_health_check DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Workflow Approvals ââ
		`CREATE TABLE IF NOT EXISTS workflow_approvals (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			module TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			requested_by TEXT,
			approved_by TEXT,
			status TEXT DEFAULT 'pending',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		)`,

		// ââ Documents ââ
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			category TEXT,
			file_path TEXT,
			file_size INTEGER,
			mime_type TEXT,
			uploaded_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Channel Manager ââ
		`CREATE TABLE IF NOT EXISTS channels (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			type TEXT NOT NULL,
			api_key TEXT,
			last_sync_at DATETIME,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Expense Claims ââ
		`CREATE TABLE IF NOT EXISTS expense_claims (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			title TEXT NOT NULL,
			description TEXT,
			category TEXT,
			amount REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			status TEXT DEFAULT 'draft',
			submitted_by TEXT,
			approved_by TEXT,
			receipt_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Fixed Assets ââ
		`CREATE TABLE IF NOT EXISTS fixed_assets (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			asset_code TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT,
			location TEXT,
			department TEXT,
			purchase_date DATE,
			purchase_cost REAL DEFAULT 0,
			useful_life INTEGER DEFAULT 60,
			monthly_depreciation REAL DEFAULT 0,
			accumulated_depreciation REAL DEFAULT 0,
			current_value REAL DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Events ââ
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			event_type TEXT,
			client_id TEXT,
			venue_id TEXT,
			event_date DATE,
			start_time TIME,
			end_time TIME,
			guest_count INTEGER DEFAULT 0,
			package_name TEXT,
			total_amount REAL DEFAULT 0,
			status TEXT DEFAULT 'enquiry',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS venues (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			capacity INTEGER DEFAULT 100,
			hourly_rate REAL DEFAULT 0,
			status TEXT DEFAULT 'available',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		// ââ Recipes ââ
		`CREATE TABLE IF NOT EXISTS recipes (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			menu_item_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			serving_size REAL DEFAULT 1,
			cost_per_serving REAL DEFAULT 0,
			selling_price REAL DEFAULT 0,
			margin_percent REAL DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS recipe_ingredients (
			id TEXT PRIMARY KEY,
			recipe_id TEXT NOT NULL REFERENCES recipes(id),
			product_id TEXT NOT NULL REFERENCES inventory_products(id),
			quantity REAL DEFAULT 0,
			unit TEXT DEFAULT 'g',
			cost REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Wastage ââ
		`CREATE TABLE IF NOT EXISTS wastage_records (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			product_id TEXT NOT NULL REFERENCES inventory_products(id),
			store_id TEXT REFERENCES inventory_stores(id),
			quantity REAL NOT NULL,
			reason TEXT,
			cost REAL DEFAULT 0,
			reported_by TEXT,
			approved_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Bank Reconciliation ââ
		`CREATE TABLE IF NOT EXISTS bank_reconciliations (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			account_id TEXT NOT NULL,
			statement_date DATE,
			statement_balance REAL DEFAULT 0,
			system_balance REAL DEFAULT 0,
			difference REAL DEFAULT 0,
			status TEXT DEFAULT 'draft',
			reconciled_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Budgets ââ
		`CREATE TABLE IF NOT EXISTS budgets (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			account_id TEXT REFERENCES chart_of_accounts(id),
			period_year INTEGER NOT NULL,
			period_month INTEGER,
			budget_amount REAL DEFAULT 0,
			actual_amount REAL DEFAULT 0,
			variance REAL DEFAULT 0,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Payroll ââ
		`CREATE TABLE IF NOT EXISTS payroll_runs (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			run_number TEXT NOT NULL,
			period_start DATE,
			period_end DATE,
			total_gross REAL DEFAULT 0,
			total_deductions REAL DEFAULT 0,
			total_net REAL DEFAULT 0,
			status TEXT DEFAULT 'draft',
			processed_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Leave ââ
		`CREATE TABLE IF NOT EXISTS leave_types (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			days_per_year INTEGER DEFAULT 14,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS leave_requests (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			leave_type_id TEXT NOT NULL REFERENCES leave_types(id),
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			days_requested INTEGER DEFAULT 1,
			reason TEXT,
			status TEXT DEFAULT 'pending',
			approved_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Housekeeping Tasks ââ
		`CREATE TABLE IF NOT EXISTS housekeeping_tasks (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			room_id TEXT REFERENCES rooms(id),
			task_type TEXT NOT NULL,
			priority TEXT DEFAULT 'normal',
			status TEXT DEFAULT 'pending',
			assigned_to TEXT,
			assigned_date DATE,
			started_at DATETIME,
			completed_at DATETIME,
			inspected_by TEXT,
			inspected_at DATETIME,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Rate Plans ââ
		`CREATE TABLE IF NOT EXISTS rate_plans (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			rate_type TEXT DEFAULT 'standard',
			description TEXT,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Revenue Management ââ
		`CREATE TABLE IF NOT EXISTS revenue_forecasts (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			forecast_date DATE NOT NULL,
			forecast_type TEXT DEFAULT 'occupancy',
			metric_name TEXT NOT NULL,
			metric_value REAL DEFAULT 0,
			confidence REAL DEFAULT 0,
			factors TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Group Bookings ââ
		`CREATE TABLE IF NOT EXISTS group_bookings (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			group_name TEXT NOT NULL,
			company TEXT,
			contact_name TEXT,
			contact_email TEXT,
			contact_phone TEXT,
			arrival_date DATE,
			departure_date DATE,
			room_count INTEGER DEFAULT 0,
			rate_per_room REAL DEFAULT 0,
			total_estimated REAL DEFAULT 0,
			status TEXT DEFAULT 'tentative',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Events / Sales & Catering ââ
		`CREATE TABLE IF NOT EXISTS event_venues (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			capacity INTEGER DEFAULT 50,
			hourly_rate REAL DEFAULT 0,
			daily_rate REAL DEFAULT 0,
			amenities TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS event_bookings (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			venue_id TEXT REFERENCES event_venues(id),
			client_name TEXT NOT NULL,
			client_email TEXT,
			client_phone TEXT,
			event_name TEXT NOT NULL,
			event_type TEXT DEFAULT 'meeting',
			start_date DATETIME,
			end_date DATETIME,
			guest_count INTEGER DEFAULT 0,
			package_name TEXT,
			food_packages TEXT,
			equipment_needed TEXT,
			total_amount REAL DEFAULT 0,
			deposit_paid REAL DEFAULT 0,
			status TEXT DEFAULT 'inquiry',
			assigned_sales TEXT,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Loyalty Program ââ
		`CREATE TABLE IF NOT EXISTS loyalty_tiers (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			min_points INTEGER DEFAULT 0,
			benefits TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS loyalty_members (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			guest_id TEXT NOT NULL REFERENCES guests(id),
			member_number TEXT UNIQUE NOT NULL,
			tier_id TEXT REFERENCES loyalty_tiers(id),
			total_points INTEGER DEFAULT 0,
			lifetime_points INTEGER DEFAULT 0,
			enrolled_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS loyalty_transactions (
			id TEXT PRIMARY KEY,
			member_id TEXT NOT NULL REFERENCES loyalty_members(id),
			type TEXT NOT NULL,
			points INTEGER DEFAULT 0,
			description TEXT,
			reference_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Dynamic Pricing Rules ââ
		`CREATE TABLE IF NOT EXISTS pricing_rules (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			rule_type TEXT DEFAULT 'seasonal',
			min_occupancy REAL DEFAULT 0,
			max_occupancy REAL DEFAULT 100,
			min_advance_days INTEGER DEFAULT 0,
			max_advance_days INTEGER DEFAULT 365,
			price_multiplier REAL DEFAULT 1.0,
			start_date DATE,
			end_date DATE,
			days_of_week TEXT,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Guest Self-Service / Digital Check-in ââ
		`CREATE TABLE IF NOT EXISTS service_requests (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			guest_stay_id TEXT REFERENCES guest_stays(id),
			category TEXT NOT NULL,
			description TEXT,
			priority TEXT DEFAULT 'normal',
			status TEXT DEFAULT 'open',
			assigned_to TEXT,
			requested_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			notes TEXT
		)`,

		// ââ Shift Handover ââ
		`CREATE TABLE IF NOT EXISTS shift_handovers (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			shift_date DATE NOT NULL,
			shift_name TEXT DEFAULT 'morning',
			outgoing_staff TEXT,
			incoming_staff TEXT,
			summary TEXT,
			open_issues TEXT,
			vip_notes TEXT,
			revenue_summary TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Settings Tables ââ
		`CREATE TABLE IF NOT EXISTS tax_rates (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			rate REAL NOT NULL DEFAULT 0,
			type TEXT DEFAULT 'percentage',
			applies_to TEXT DEFAULT 'all',
			is_compound INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS payment_methods (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			type TEXT DEFAULT 'cash',
			is_active INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			requires_reference INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS departments (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			manager TEXT,
			email TEXT,
			phone TEXT,
			is_active INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS numbering_sequences (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			prefix TEXT DEFAULT '',
			suffix TEXT DEFAULT '',
			next_number INTEGER DEFAULT 1,
			padding INTEGER DEFAULT 6,
			format TEXT DEFAULT 'PREFIX-NNNNNN',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS system_settings (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			category TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT,
			data_type TEXT DEFAULT 'string',
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(property_id, category, key)
		)`,

		// ââ User Roles ââ
		`CREATE TABLE IF NOT EXISTS user_property_roles (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			role TEXT NOT NULL DEFAULT 'staff',
			permissions TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ââ Enterprise HR Schema ââ

		// Employee extended profile
		`CREATE TABLE IF NOT EXISTS employee_profiles (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			middle_name TEXT,
			preferred_name TEXT,
			date_of_birth TEXT,
			gender TEXT,
			nationality TEXT,
			marital_status TEXT,
			blood_group TEXT,
			religion TEXT,
			emergency_contact_name TEXT,
			emergency_contact_phone TEXT,
			emergency_contact_relation TEXT,
			next_of_kin_name TEXT,
			next_of_kin_phone TEXT,
			residential_address TEXT,
			city TEXT,
			state TEXT,
			country TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,


		// Employee contracts
		`CREATE TABLE IF NOT EXISTS employee_contracts (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			contract_type TEXT DEFAULT 'permanent',
			start_date TEXT NOT NULL,
			end_date TEXT,
			probation_end_date TEXT,
			probation_status TEXT DEFAULT 'active',
			notice_period_days INTEGER DEFAULT 30,
			renewal_auto INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee identification documents
		`CREATE TABLE IF NOT EXISTS employee_identifications (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			doc_type TEXT NOT NULL,
			doc_number TEXT,
			issue_date TEXT,
			expiry_date TEXT,
			issuing_authority TEXT,
			verified INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee bank accounts
		`CREATE TABLE IF NOT EXISTS employee_bank_accounts (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			bank_name TEXT,
			account_name TEXT,
			account_number TEXT,
			sort_code TEXT,
			is_primary INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Attendance devices (Dahua, ZKTeco, etc.)
		`CREATE TABLE IF NOT EXISTS attendance_devices (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			device_model TEXT,
			serial_number TEXT,
			ip_address TEXT,
			port INTEGER DEFAULT 8080,
			protocol TEXT DEFAULT 'dahua',
			username TEXT,
			password_encrypted TEXT,
			location TEXT,
			department TEXT,
			timezone TEXT DEFAULT 'Africa/Lagos',
			sync_interval_minutes INTEGER DEFAULT 5,
			status TEXT DEFAULT 'online',
			last_sync_at DATETIME,
			last_sync_error TEXT,
			capabilities TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee-to-device user mapping
		`CREATE TABLE IF NOT EXISTS attendance_device_users (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL REFERENCES attendance_devices(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			device_user_id TEXT NOT NULL,
			enrolled INTEGER DEFAULT 0,
			enrolled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, device_user_id)
		)`,

		// Raw biometric attendance events
		`CREATE TABLE IF NOT EXISTS attendance_raw_events (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL DEFAULT 'dahua',
			device_id TEXT REFERENCES attendance_devices(id),
			device_user_id TEXT NOT NULL,
			employee_id TEXT REFERENCES employees(id),
			event_timestamp DATETIME NOT NULL,
			received_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			event_type TEXT DEFAULT 'check_in',
			verification_type TEXT DEFAULT 'face',
			source_payload TEXT,
			event_hash TEXT UNIQUE,
			processed INTEGER DEFAULT 0,
			processing_status TEXT DEFAULT 'pending',
			processing_error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Attendance sync jobs
		`CREATE TABLE IF NOT EXISTS attendance_sync_jobs (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL REFERENCES attendance_devices(id),
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			records_fetched INTEGER DEFAULT 0,
			records_created INTEGER DEFAULT 0,
			duplicates_skipped INTEGER DEFAULT 0,
			errors INTEGER DEFAULT 0,
			status TEXT DEFAULT 'running',
			error_message TEXT,
			last_event_timestamp DATETIME
		)`,

		// Shift templates
		`CREATE TABLE IF NOT EXISTS shift_templates (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			break_minutes INTEGER DEFAULT 60,
			is_overnight INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			color TEXT DEFAULT '#3B82F6',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Rosters (schedule periods)
		`CREATE TABLE IF NOT EXISTS rosters (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			status TEXT DEFAULT 'draft',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Roster assignments (employee-shift per day)
		`CREATE TABLE IF NOT EXISTS roster_assignments (
			id TEXT PRIMARY KEY,
			roster_id TEXT NOT NULL REFERENCES rosters(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			shift_id TEXT NOT NULL REFERENCES shift_templates(id),
			assignment_date TEXT NOT NULL,
			status TEXT DEFAULT 'scheduled',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Attendance records (processed/validated)
		`CREATE TABLE IF NOT EXISTS attendance_records (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			record_date TEXT NOT NULL,
			shift_id TEXT REFERENCES shift_templates(id),
			clock_in DATETIME,
			clock_out DATETIME,
			break_out DATETIME,
			break_in DATETIME,
			gross_hours REAL DEFAULT 0,
			break_minutes REAL DEFAULT 0,
			net_hours REAL DEFAULT 0,
			overtime_hours REAL DEFAULT 0,
			late_minutes INTEGER DEFAULT 0,
			early_departure_minutes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'present',
			source TEXT DEFAULT 'biometric',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Attendance corrections
		`CREATE TABLE IF NOT EXISTS attendance_corrections (
			id TEXT PRIMARY KEY,
			attendance_id TEXT REFERENCES attendance_records(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			original_clock_in TEXT,
			original_clock_out TEXT,
			corrected_clock_in TEXT,
			corrected_clock_out TEXT,
			reason TEXT,
			requested_by TEXT,
			approved_by TEXT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Leave types
		`CREATE TABLE IF NOT EXISTS leave_types (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			code TEXT,
			days_per_year INTEGER DEFAULT 0,
			is_paid INTEGER DEFAULT 1,
			is_carry_forward INTEGER DEFAULT 0,
			is_encashable INTEGER DEFAULT 0,
			max_carry_forward INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Leave balances
		`CREATE TABLE IF NOT EXISTS leave_balances (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			leave_type_id TEXT NOT NULL REFERENCES leave_types(id),
			year INTEGER NOT NULL,
			entitled REAL DEFAULT 0,
			taken REAL DEFAULT 0,
			carried_forward REAL DEFAULT 0,
			balance REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(employee_id, leave_type_id, year)
		)`,

		// Leave requests
		`CREATE TABLE IF NOT EXISTS leave_requests (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			leave_type_id TEXT NOT NULL REFERENCES leave_types(id),
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			days_requested REAL DEFAULT 1,
			reason TEXT,
			status TEXT DEFAULT 'pending',
			approved_by TEXT,
			approved_at DATETIME,
			rejection_reason TEXT,
			property_id TEXT NOT NULL REFERENCES properties(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Public holidays
		`CREATE TABLE IF NOT EXISTS public_holidays (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			holiday_date TEXT NOT NULL,
			is_recurring INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Overtime requests
		`CREATE TABLE IF NOT EXISTS overtime_requests (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			overtime_date TEXT NOT NULL,
			hours REAL NOT NULL,
			type TEXT DEFAULT 'standard',
			reason TEXT,
			status TEXT DEFAULT 'pending',
			approved_by TEXT,
			approved_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Performance cycles
		`CREATE TABLE IF NOT EXISTS performance_cycles (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Performance goals/reviews
		`CREATE TABLE IF NOT EXISTS performance_reviews (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			cycle_id TEXT NOT NULL REFERENCES performance_cycles(id),
			reviewer_id TEXT REFERENCES employees(id),
			self_score REAL,
			manager_score REAL,
			final_score REAL,
			strengths TEXT,
			improvements TEXT,
			goals_next_period TEXT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Training courses
		`CREATE TABLE IF NOT EXISTS training_courses (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			category TEXT,
			description TEXT,
			duration_hours REAL DEFAULT 0,
			is_mandatory INTEGER DEFAULT 0,
			validity_months INTEGER DEFAULT 12,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Training enrollments
		`CREATE TABLE IF NOT EXISTS training_enrollments (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			course_id TEXT NOT NULL REFERENCES training_courses(id),
			enrollment_date TEXT NOT NULL,
			completion_date TEXT,
			score REAL,
			certificate_number TEXT,
			expiry_date TEXT,
			status TEXT DEFAULT 'enrolled',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee assets
		`CREATE TABLE IF NOT EXISTS employee_assets (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			asset_type TEXT NOT NULL,
			asset_name TEXT NOT NULL,
			serial_number TEXT,
			issue_date TEXT,
			return_date TEXT,
			condition_on_issue TEXT,
			condition_on_return TEXT,
			status TEXT DEFAULT 'assigned',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee loans/advances
		`CREATE TABLE IF NOT EXISTS employee_loans (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			loan_type TEXT DEFAULT 'advance',
			amount REAL NOT NULL,
			monthly_deduction REAL DEFAULT 0,
			repaid REAL DEFAULT 0,
			balance REAL NOT NULL,
			status TEXT DEFAULT 'active',
			approved_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee expenses
		`CREATE TABLE IF NOT EXISTS employee_expenses (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			expense_date TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT,
			amount REAL NOT NULL,
			receipt_reference TEXT,
			status TEXT DEFAULT 'pending',
			approved_by TEXT,
			reimbursed INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Disciplinary cases
		`CREATE TABLE IF NOT EXISTS disciplinary_cases (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL REFERENCES properties(id),
			case_type TEXT NOT NULL,
			severity TEXT DEFAULT 'minor',
			description TEXT,
			evidence TEXT,
			action_taken TEXT,
			status TEXT DEFAULT 'open',
			reported_by TEXT,
			resolved_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Compensation/salary structures
		`CREATE TABLE IF NOT EXISTS salary_structures (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			job_grade TEXT,
			min_salary REAL DEFAULT 0,
			max_salary REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Benefits
		`CREATE TABLE IF NOT EXISTS benefits (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL REFERENCES properties(id),
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			employer_contribution REAL DEFAULT 0,
			employee_contribution REAL DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Employee benefits enrollment
		`CREATE TABLE IF NOT EXISTS employee_benefits (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			benefit_id TEXT NOT NULL REFERENCES benefits(id),
			enrollment_date TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Onboarding tasks
		`CREATE TABLE IF NOT EXISTS onboarding_tasks (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			task_name TEXT NOT NULL,
			category TEXT,
			assigned_to TEXT,
			due_date TEXT,
			completed INTEGER DEFAULT 0,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ── Guest 360 ──
		`CREATE TABLE IF NOT EXISTS guest_stay_history (
			id TEXT PRIMARY KEY,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			property_id TEXT NOT NULL,
			reservation_id TEXT,
			room_number TEXT,
			room_type TEXT,
			arrival_date DATE,
			departure_date DATE,
			nights INTEGER DEFAULT 0,
			total_spend REAL DEFAULT 0,
			rate_paid REAL DEFAULT 0,
			source TEXT,
			market_segment TEXT,
			satisfaction_score INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS guest_service_requests (
			id TEXT PRIMARY KEY,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			reservation_id TEXT,
			property_id TEXT NOT NULL,
			request_type TEXT NOT NULL,
			description TEXT,
			priority TEXT DEFAULT 'normal',
			status TEXT DEFAULT 'open',
			assigned_to TEXT,
			department TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS guest_interactions (
			id TEXT PRIMARY KEY,
			guest_id TEXT NOT NULL REFERENCES guests(id),
			reservation_id TEXT,
			property_id TEXT NOT NULL,
			interaction_type TEXT NOT NULL,
			channel TEXT,
			subject TEXT,
			message TEXT,
			staff_name TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ── Traces / Tasks ──
		`CREATE TABLE IF NOT EXISTS traces (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			guest_id TEXT REFERENCES guests(id),
			reservation_id TEXT REFERENCES reservations(id),
			room_id TEXT REFERENCES rooms(id),
			trace_type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			department TEXT NOT NULL,
			priority TEXT DEFAULT 'normal',
			status TEXT DEFAULT 'pending',
			assigned_to TEXT,
			due_date DATE,
			due_time TIME,
			completed_by TEXT,
			completed_at DATETIME,
			verified_by TEXT,
			verified_at DATETIME,
			created_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS trace_notes (
			id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL REFERENCES traces(id),
			note TEXT NOT NULL,
			author TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ── Room Assignment Rules ──
		`CREATE TABLE IF NOT EXISTS room_assignment_rules (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			rule_name TEXT NOT NULL,
			weight_guest_preference REAL DEFAULT 0.30,
			weight_room_readiness REAL DEFAULT 0.20,
			weight_room_type REAL DEFAULT 0.20,
			weight_floor_preference REAL DEFAULT 0.10,
			weight_housekeeping REAL DEFAULT 0.10,
			weight_upgrade REAL DEFAULT 0.10,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS room_assignments (
			id TEXT PRIMARY KEY,
			reservation_id TEXT NOT NULL REFERENCES reservations(id),
			room_id TEXT NOT NULL REFERENCES rooms(id),
			property_id TEXT NOT NULL,
			assigned_by TEXT,
			assignment_score REAL DEFAULT 0,
			assignment_reasons TEXT,
			is_upgrade INTEGER DEFAULT 0,
			is_manual INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ── Room Queue ──
		`CREATE TABLE IF NOT EXISTS room_queue (
			id TEXT PRIMARY KEY,
			reservation_id TEXT NOT NULL REFERENCES reservations(id),
			property_id TEXT NOT NULL,
			priority INTEGER DEFAULT 0,
			status TEXT DEFAULT 'waiting',
			estimated_ready DATETIME,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Generator and Fuel Tracking (Africa-specific)
		`CREATE TABLE IF NOT EXISTS generators (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, model TEXT, power_rating_kw REAL DEFAULT 0, fuel_type TEXT DEFAULT 'diesel', status TEXT DEFAULT 'active', install_date DATE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS generator_logs (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, generator_id TEXT NOT NULL, start_time DATETIME NOT NULL, end_time DATETIME, run_hours REAL DEFAULT 0, fuel_consumed REAL DEFAULT 0, fuel_cost REAL DEFAULT 0, power_output_kw REAL DEFAULT 0, maintenance_notes TEXT, recorded_by TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS fuel_purchases (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, generator_id TEXT NOT NULL, quantity REAL NOT NULL, unit_price REAL NOT NULL, total_cost REAL NOT NULL, receipt_no TEXT, supplier TEXT, purchased_by TEXT, purchase_date DATE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS daily_revenue (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, date DATE NOT NULL, room_revenue REAL DEFAULT 0, pos_sales REAL DEFAULT 0, tax_revenue REAL DEFAULT 0, total_revenue REAL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, UNIQUE(property_id, date))`,
		// Staycore-inspired feature tables
		`CREATE TABLE IF NOT EXISTS approval_requests (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, type TEXT NOT NULL, target_type TEXT, target_id TEXT, amount REAL DEFAULT 0, reason TEXT, status TEXT DEFAULT 'pending', approved_by TEXT, approved_at DATETIME, audit_trail TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS shortlet_units (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, unit_type TEXT, bedrooms INTEGER DEFAULT 1, max_guests INTEGER DEFAULT 2, base_price REAL DEFAULT 0, status TEXT DEFAULT 'available', floor TEXT, building TEXT, owner_name TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS shortlet_bookings (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, unit_id TEXT NOT NULL, guest_name TEXT, guest_phone TEXT, guest_email TEXT, check_in DATE, check_out DATE, total_amount REAL DEFAULT 0, platform TEXT DEFAULT 'direct', status TEXT DEFAULT 'confirmed', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS guest_verifications (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, guest_id TEXT, verification_type TEXT NOT NULL, document_number TEXT, document_front_url TEXT, document_back_url TEXT, selfie_url TEXT, expiry_date TEXT, status TEXT DEFAULT 'pending_verification', verified_by TEXT, verified_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS pricing_rules (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, room_type_id TEXT NOT NULL, name TEXT NOT NULL, min_occupancy REAL DEFAULT 0, max_occupancy REAL DEFAULT 100, price_adjust REAL DEFAULT 0, day_of_week TEXT DEFAULT 'all', start_date TEXT, end_date TEXT, enabled INTEGER DEFAULT 1, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		// restaurant_tables is defined below in the Restaurant Management section
		// menu_items: restaurant_menu is defined in Restaurant Management section
		`CREATE TABLE IF NOT EXISTS expenses (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, category TEXT, description TEXT, amount REAL DEFAULT 0, vendor TEXT, reference TEXT, date DATE, approved_by TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS time_entries (id TEXT PRIMARY KEY, employee_id TEXT NOT NULL, property_id TEXT NOT NULL, date DATE NOT NULL, clock_in TEXT, clock_out TEXT, method TEXT DEFAULT 'manual', hours_worked REAL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS pos_order_items (id TEXT PRIMARY KEY, order_id TEXT NOT NULL, menu_item_id TEXT NOT NULL, quantity INTEGER DEFAULT 1, unit_price REAL DEFAULT 0, total REAL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
			// Enterprise Ops tables
		`CREATE TABLE IF NOT EXISTS cash_shifts (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, cashier_id TEXT, opening_cash REAL DEFAULT 0, closing_cash REAL DEFAULT 0, expected_cash REAL DEFAULT 0, variance REAL DEFAULT 0, card_total REAL DEFAULT 0, transfer_total REAL DEFAULT 0, total_sales REAL DEFAULT 0, pos_outlet TEXT, status TEXT DEFAULT 'open', notes TEXT, opened_at DATETIME, closed_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS company_profiles (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, contact_name TEXT, email TEXT, phone TEXT, address TEXT, tax_id TEXT, payment_term TEXT, credit_limit REAL DEFAULT 0, status TEXT DEFAULT 'active', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS travel_agent_profiles (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, contact_name TEXT, email TEXT, phone TEXT, address TEXT, commission_pct REAL DEFAULT 0, payment_term TEXT, agreement_number TEXT, status TEXT DEFAULT 'active', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS seasonal_rates (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, room_type_id TEXT, season_name TEXT, start_date DATE, end_date DATE, base_price REAL, min_stay INTEGER DEFAULT 1, tax_inclusive INTEGER DEFAULT 0, plan_code TEXT, enabled INTEGER DEFAULT 1, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS business_rates (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, source_id TEXT, source_type TEXT, room_type_id TEXT, rate_per_night REAL, tax_inclusive INTEGER DEFAULT 0, valid_from DATE, valid_to DATE, enabled INTEGER DEFAULT 1, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS card_keys (id TEXT PRIMARY KEY, reservation_id TEXT, room_number TEXT, key_code TEXT, valid_from TEXT, valid_to TEXT, status TEXT DEFAULT 'active', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS blacklisted_guests (id TEXT PRIMARY KEY, name TEXT, phone TEXT, reason TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS deposits (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, customer_id TEXT, sale_id TEXT, amount REAL DEFAULT 0, status TEXT DEFAULT 'active', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS serial_numbers (id TEXT PRIMARY KEY, product_id TEXT, serial_number TEXT, sale_id TEXT, property_id TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,

		// Accounting module enhancements
		`ALTER TABLE chart_of_accounts ADD COLUMN parent_id TEXT DEFAULT NULL`,
		`ALTER TABLE chart_of_accounts ADD COLUMN balance REAL DEFAULT 0`,
		`ALTER TABLE journal_entries ADD COLUMN reference TEXT DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS journal_entry_lines (id TEXT PRIMARY KEY, journal_entry_id TEXT NOT NULL, account_id TEXT NOT NULL, description TEXT, debit REAL DEFAULT 0, credit REAL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS account_transfers (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, from_account TEXT, to_account TEXT, amount REAL DEFAULT 0, description TEXT, status TEXT DEFAULT 'pending_approval', requested_by TEXT, approved_by TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS cheques (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, cheque_number TEXT, bank_account_id TEXT, payee_name TEXT, payee_address TEXT, amount REAL DEFAULT 0, issue_date DATE, pay_date DATE, clearance_date DATE, status TEXT DEFAULT 'issued', memo TEXT, reference TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`ALTER TABLE expenses ADD COLUMN status TEXT DEFAULT 'draft'`,
		`CREATE TABLE IF NOT EXISTS invoices (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, invoice_number TEXT UNIQUE NOT NULL, customer_id TEXT, customer_name TEXT, customer_email TEXT, reference TEXT, status TEXT DEFAULT 'draft', type TEXT DEFAULT 'invoice', issue_date DATE, due_date DATE, subtotal REAL DEFAULT 0, discount_pct REAL DEFAULT 0, discount_amount REAL DEFAULT 0, tax_amount REAL DEFAULT 0, wht_amount REAL DEFAULT 0, total REAL DEFAULT 0, amount_paid REAL DEFAULT 0, balance_due REAL DEFAULT 0, currency TEXT DEFAULT 'NGN', notes TEXT, payment_terms TEXT, paid_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS invoice_items (id TEXT PRIMARY KEY, invoice_id TEXT NOT NULL, product_id TEXT, description TEXT, quantity REAL DEFAULT 1, unit_price REAL DEFAULT 0, discount_pct REAL DEFAULT 0, tax_rate REAL DEFAULT 7.5, wht_rate REAL DEFAULT 0, amount REAL DEFAULT 0, tax_amount REAL DEFAULT 0, wht_amount REAL DEFAULT 0, line_total REAL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS invoice_customers (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, email TEXT, phone TEXT, address TEXT, city TEXT, country TEXT DEFAULT 'Nigeria', tax_id TEXT, type TEXT DEFAULT 'individual', status TEXT DEFAULT 'active', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS credit_notes (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, credit_number TEXT UNIQUE NOT NULL, invoice_id TEXT, customer_id TEXT, customer_name TEXT, amount REAL DEFAULT 0, reason TEXT, status TEXT DEFAULT 'issued', issue_date DATE, applied_date DATE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS bar_tabs (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, tab_number TEXT UNIQUE, table_id TEXT, guest_name TEXT, staff_id TEXT, status TEXT DEFAULT 'open', covers INTEGER DEFAULT 1, subtotal REAL DEFAULT 0, tax_amount REAL DEFAULT 0, discount_amount REAL DEFAULT 0, total REAL DEFAULT 0, paid_amount REAL DEFAULT 0, payment_method TEXT, opened_at DATETIME DEFAULT CURRENT_TIMESTAMP, closed_at DATETIME)`,
		`CREATE TABLE IF NOT EXISTS bar_tab_items (id TEXT PRIMARY KEY, tab_id TEXT NOT NULL, drink_id TEXT, drink_name TEXT, quantity REAL DEFAULT 1, unit_price REAL DEFAULT 0, discount_pct REAL DEFAULT 0, amount REAL DEFAULT 0, added_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS bar_drinks (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, category TEXT, price REAL DEFAULT 0, cost_price REAL DEFAULT 0, size TEXT, unit_type TEXT DEFAULT 'bottle', stock_qty REAL DEFAULT 0, min_stock REAL DEFAULT 0, abv REAL DEFAULT 0, is_active INTEGER DEFAULT 1, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS bar_tables (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, number TEXT, section TEXT DEFAULT 'Main', capacity INTEGER DEFAULT 4, status TEXT DEFAULT 'available', tab_id TEXT, staff_id TEXT, opened_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS bar_shifts (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, staff_id TEXT, staff_name TEXT, role TEXT, shift_start DATETIME, shift_end DATETIME, status TEXT DEFAULT 'clocked_in', total_sales REAL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS happy_hour_rules (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT, day_of_week TEXT DEFAULT 'all', start_time TEXT, end_time TEXT, discount_pct REAL DEFAULT 0, category TEXT DEFAULT 'all', is_active INTEGER DEFAULT 1, start_date DATE, end_date DATE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS bar_guests (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, phone TEXT, email TEXT, tier TEXT DEFAULT 'regular', total_visits INTEGER DEFAULT 0, total_spend REAL DEFAULT 0, favorite_drink TEXT, last_visit DATETIME, notes TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_orders (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, order_number TEXT UNIQUE, table_id TEXT, type TEXT DEFAULT 'dine_in', status TEXT DEFAULT 'new', priority TEXT DEFAULT 'normal', staff_id TEXT, staff_name TEXT, guest_name TEXT, subtotal REAL DEFAULT 0, tax_amount REAL DEFAULT 0, discount_amount REAL DEFAULT 0, total REAL DEFAULT 0, paid_amount REAL DEFAULT 0, payment_method TEXT, payment_status TEXT DEFAULT 'unpaid', notes TEXT, started_at DATETIME, served_at DATETIME, completed_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_order_items (id TEXT PRIMARY KEY, order_id TEXT NOT NULL, menu_item_id TEXT, name TEXT, category TEXT, quantity REAL DEFAULT 1, unit_price REAL DEFAULT 0, discount_pct REAL DEFAULT 0, amount REAL DEFAULT 0, status TEXT DEFAULT 'pending', notes TEXT)`,
		`CREATE TABLE IF NOT EXISTS restaurant_menu (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, category TEXT, description TEXT, price REAL DEFAULT 0, cost_price REAL DEFAULT 0, prep_time_min INTEGER DEFAULT 15, is_available INTEGER DEFAULT 1, tags TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_tables (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, number TEXT, section TEXT DEFAULT 'Main', capacity INTEGER DEFAULT 4, status TEXT DEFAULT 'available', waiter_id TEXT, order_id TEXT, guests INTEGER DEFAULT 0, occupied_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_deliveries (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, order_id TEXT, source TEXT, customer_name TEXT, customer_phone TEXT, address TEXT, total REAL DEFAULT 0, delivery_fee REAL DEFAULT 0, rider_id TEXT, rider_name TEXT, status TEXT DEFAULT 'pending', eta_minutes INTEGER DEFAULT 30, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_ingredients (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, category TEXT, unit TEXT DEFAULT 'kg', current_stock REAL DEFAULT 0, min_stock REAL DEFAULT 0, cost_per_unit REAL DEFAULT 0, supplier TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS restaurant_guests (id TEXT PRIMARY KEY, property_id TEXT NOT NULL, name TEXT NOT NULL, phone TEXT, email TEXT, tier TEXT DEFAULT 'regular', total_visits INTEGER DEFAULT 0, total_spend REAL DEFAULT 0, favorite_dish TEXT, dietary_notes TEXT, last_visit DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,

		// ── Payroll Engine Tables ──
		`CREATE TABLE IF NOT EXISTS payroll_lines (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES payroll_runs(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			basic_salary REAL DEFAULT 0,
			allowances_total REAL DEFAULT 0,
			overtime_pay REAL DEFAULT 0,
			bonus REAL DEFAULT 0,
			gross_pay REAL DEFAULT 0,
			income_tax REAL DEFAULT 0,
			pension_employee REAL DEFAULT 0,
			pension_employer REAL DEFAULT 0,
			nhf REAL DEFAULT 0,
			health_insurance REAL DEFAULT 0,
			loan_deduction REAL DEFAULT 0,
			other_deductions REAL DEFAULT 0,
			total_deductions REAL DEFAULT 0,
			net_pay REAL DEFAULT 0,
			tax_band TEXT DEFAULT '',
			country TEXT DEFAULT 'NG',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS payroll_overrides (
			id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			property_id TEXT NOT NULL,
			housing_allow REAL DEFAULT 0,
			transport_allow REAL DEFAULT 0,
			meal_allow REAL DEFAULT 0,
			loan_deduction REAL DEFAULT 0,
			bonus REAL DEFAULT 0,
			is_pension INTEGER DEFAULT 1,
			is_nhf INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(employee_id, property_id)
		)`,
	}

	for _, m := range migrations {
		if _, err := d.ExecContext(ctx, m); err != nil {
			if strings.Contains(err.Error(), "duplicate column") { continue }
			if len(m) > 80 { return fmt.Errorf("migration failed: %w\nSQL: %s", err, m[:80]) }; return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	// Engine-drift repair: databases created before the schema audit carry
	// reservations.total_estimate while the Postgres migrations, handlers and
	// JSON contract all use total_estimated. Rename in place when the legacy
	// column is present so existing installs keep booting.
	if err := d.renameLegacyColumn(ctx, "reservations", "total_estimate", "total_estimated"); err != nil {
		return fmt.Errorf("drift repair failed: %w", err)
	}

	log.Printf("[sqlite] All %d migrations applied successfully", len(migrations))
	return nil
}

// renameLegacyColumn renames oldName to newName on table when the legacy
// column exists and the replacement does not yet. It is a no-op on fresh
// databases, making it safe to call on every boot.
func (d *DB) renameLegacyColumn(ctx context.Context, table, oldName, newName string) error {
	rows, err := d.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("pragma %s: %w", table, err)
	}
	defer rows.Close()
	var hasOld, hasNew bool
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == oldName { hasOld = true }
		if name == newName { hasNew = true }
	}
	if err := rows.Err(); err != nil { return err }
	if !hasOld || hasNew { return nil }
	if _, err := d.ExecContext(ctx,
		`ALTER TABLE `+table+` RENAME COLUMN `+oldName+` TO `+newName); err != nil {
		return fmt.Errorf("rename %s.%s: %w", table, oldName, err)
	}
	log.Printf("[sqlite] Drift repair: %s.%s renamed to %s", table, oldName, newName)
	return nil
}

// SeedDemoData populates the database with realistic African hotel demo data.
func (d *DB) SeedDemoData(ctx context.Context) error {
	log.Println("[sqlite] Seeding demo data for 50-room Grand Palm Hotel Lagos...")

	// Property
	_, err := d.ExecContext(ctx, `
		INSERT OR IGNORE INTO properties (id, name, code, address, city, state, country, telephone, email, currency, timezone, service_charge_rate, vat_rate)
		VALUES ('00000000-0000-0000-0000-000000000001', 'Grand Palm Hotel Lagos', 'GPH-LAG', '15 Adeola Odeku VI', 'Lagos', 'Lagos', 'Nigeria', '+234-1-555-0100', 'info@grandpalm.ng', 'NGN', 'Africa/Lagos', 10, 7.5)
	`)
	if err != nil {
		return fmt.Errorf("seed property: %w", err)
	}

	// Admin user (password: admin123 â bcrypt hash of "admin123")
	_, err = d.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (id, name, email, password, status)
		VALUES ('00000000-0000-0000-0000-000000000002', 'System Administrator', 'admin@infineon.com', '$2a$10$1kflwkX6eDxjKMvaA40eQ.Fqwjd9Wn8HOPyUCO2arlPdAcpT8w6WS', 'active')
	`)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	// ââ Multi-role Demo Users ââ
	// All passwords follow pattern: <role>123 (e.g. reception123, cashier123)
	demoUsers := []struct {
		id, name, email, password, role string
		permissions                    string
	}{
		{
			"usr-reception-01", "Chioma Okoro", "receptionist@infineon.com",
			"$2a$10$Awr890F2BB0W6tDI1nnVMu0mL9FEnHOTfB4nS7gQvIKWVg2UZvf6u", "receptionist",
			"[\"reservations.create\",\"reservations.view\",\"front_desk.*\",\"check_in\",\"check_out\",\"folios.view\",\"guests.*\",\"rooms.view\",\"rooms.assign\"]",
		},
		{
			"usr-cashier-01", "Emeka Nwosu", "cashier@infineon.com",
			"$2a$10$tKFw2hOpVm5CU4Ttmrg4SeWJt8zLfhtT307.vblE8ZJsCvEoF3nyK", "cashier",
			"[\"pos.*\",\"cashier.*\",\"payments.create\",\"payments.view\",\"folios.view\",\"folios.post_charge\"]",
		},
		{
			"usr-housekeep-01", "Fatima Bello", "housekeeping@infineon.com",
			"$2a$10$LW0pR6evCmsgJrY.U1n8xO2GU.FesVPlGPm/o1xj.ib8IvjCIQ.aa", "housekeeping_supervisor",
			"[\"housekeeping.*\",\"rooms.view\",\"rooms.update_status\",\"maintenance.create\",\"maintenance.view\"]",
		},
		{
			"usr-kitchen-01", "Ibrahim Tanko", "kitchen@infineon.com",
			"$2a$10$UiYiyMQbghiDdi3ne3XIRO3XaNief0ejTn1AaPkt7IhsHfFPV5/xq", "restaurant_manager",
			"[\"pos.*\",\"kds.*\",\"menu.*\",\"inventory.view\",\"recipes.*\"]",
		},
		{
			"usr-account-01", "Aisha Abdullahi", "accountant@infineon.com",
			"$2a$10$VbPEiixPlDhWfl83HfXJXOJCEVHFmeO/W/skm/YlSKoKL58bekJQu", "accountant",
			"[\"finance.*\",\"accounting.*\",\"journal.*\",\"reports.financial\",\"bank_reconciliation\",\"accounts_receivable\",\"accounts_payable\"]",
		},
		{
			"usr-hr-01", "Blessing Eze", "hr@infineon.com",
			"$2a$10$qvjrowSth.5LcAHR1uccHOWh4Wo/n/8.8R6TvkmhzxzPKOpNfL/82", "hr_manager",
			"[\"hr.*\",\"employees.*\",\"attendance.*\",\"leave.*\",\"payroll.*\",\"reports.hr\"]",
		},
		{
			"usr-manager-01", "Oluwaseun Adeyemi", "gm@infineon.com",
			"$2a$10$XxEm19nm1XxBvhN0aDZGQ.D8cNmKQrJf6.Nm7qU6xESFps3MUXXgO", "general_manager",
			"[\"*\"]",
		},
		{
			"usr-waiter-01", "Tunde Bakare", "waiter@infineon.com",
			"$2a$10$zcvT9BuGV/Av0hHmHOm52e2nVVRDKMWtQOPyxYmeTsJsyO4fFICEW", "waiter",
			"[\"pos.create_order\",\"pos.view_menu\",\"pos.close_order\",\"kds.view\",\"rooms.view_guest\"]",
		},
	}

	pID := "00000000-0000-0000-0000-000000000001"
	for _, u := range demoUsers {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO users (id, name, email, password, status) VALUES (?, ?, ?, ?, 'active')`,
			u.id, u.name, u.email, u.password)
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO user_property_roles (id, user_id, property_id, role, permissions) VALUES (?, ?, ?, ?, ?)`,
			"role-"+u.id, u.id, pID, u.role, u.permissions)
	}

	// Also assign admin role to admin user
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO user_property_roles (id, user_id, property_id, role, permissions) VALUES (?, ?, ?, ?, ?)`,
		"role-admin-01", "00000000-0000-0000-0000-000000000002", pID, "admin",
		"[\"*\"]")

	// Room Types
	roomTypes := []struct {
		code, name string
		baseRate   float64
		adultCap   int
	}{
		{"STD", "Standard Room", 25000, 2},
		{"DLX", "Deluxe Room", 40000, 2},
		{"EXE", "Executive Suite", 65000, 2},
		{"PRM", "Presidential Suite", 150000, 4},
	}

	for _, rt := range roomTypes {
		_, err := d.ExecContext(ctx, `
			INSERT OR IGNORE INTO room_types (id, property_id, code, name, max_occupancy, adult_capacity, base_rate, min_rate, max_rate)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?, ?, ?)
		`, rt.code+"-TYPE", rt.code, rt.name, rt.adultCap, rt.adultCap, rt.baseRate, rt.baseRate*0.8, rt.baseRate*1.5)
		if err != nil {
			return fmt.Errorf("seed room type %s: %w", rt.code, err)
		}
	}

	// ââ Buildings & Floors (required by rooms FK) ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO buildings (id, property_id, name, code) VALUES ('bldg-main', '00000000-0000-0000-0000-000000000001', 'Main Building', 'MAIN')`)
	for f := 1; f <= 5; f++ {
		floorID := fmt.Sprintf("floor-%d", f)
		floorName := fmt.Sprintf("Floor %d", f)
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO floors (id, property_id, building_id, name, level) VALUES (?, '00000000-0000-0000-0000-000000000001', 'bldg-main', ?, ?)`, floorID, floorName, f)
	}

	// Create 50 rooms (40 Standard, 7 Deluxe, 2 Executive, 1 Presidential)
	roomNum := 1
	for i := 0; i < 40; i++ {
		num := fmt.Sprintf("%03d", roomNum)
		floor := (roomNum-1)/10 + 1
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO rooms (id, property_id, room_type_id, number, floor_id, status, housekeeping_status)
			VALUES (?, '00000000-0000-0000-0000-000000000001', 'STD-TYPE', ?, ?, 'vacant_dirty', 'dirty')
		`, fmt.Sprintf("room-%03d", roomNum), num, fmt.Sprintf("floor-%d", floor))
		roomNum++
	}
	for i := 0; i < 7; i++ {
		num := fmt.Sprintf("%03d", roomNum)
		floor := (roomNum-1)/10 + 1
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO rooms (id, property_id, room_type_id, number, floor_id, status, housekeeping_status)
			VALUES (?, '00000000-0000-0000-0000-000000000001', 'DLX-TYPE', ?, ?, 'vacant_dirty', 'dirty')
		`, fmt.Sprintf("room-%03d", roomNum), num, fmt.Sprintf("floor-%d", floor))
		roomNum++
	}
	for i := 0; i < 2; i++ {
		num := fmt.Sprintf("%03d", roomNum)
		floor := (roomNum-1)/10 + 1
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO rooms (id, property_id, room_type_id, number, floor_id, status, housekeeping_status)
			VALUES (?, '00000000-0000-0000-0000-000000000001', 'EXE-TYPE', ?, ?, 'occupied_dirty', 'dirty')
		`, fmt.Sprintf("room-%03d", roomNum), num, fmt.Sprintf("floor-%d", floor))
		roomNum++
	}
	_, _ = d.ExecContext(ctx, `
		INSERT OR IGNORE INTO rooms (id, property_id, room_type_id, number, floor_id, status, housekeeping_status)
		VALUES ('room-050', '00000000-0000-0000-0000-000000000001', 'PRM-TYPE', '050', 'floor-5', 'occupied_clean', 'clean')
	`)

	// Nigerian guests
	guests := []struct{ first, last, email, phone, company string }{
		{"Adaeze", "Okonkwo", "adaeze.okonkwo@mtngroup.com", "+234-803-123-4567", "MTN Nigeria"},
		{"Chukwuemeka", "Nwosu", "emeka.nwosu@shell.com.ng", "+234-806-234-5678", "Shell Nigeria"},
		{"Fatima", "Abubakar", "fatima@npa.gov.ng", "+234-809-345-6789", "NPA"},
		{"Oluwaseun", "Adeyemi", "seun@andela.com", "+234-812-456-7890", "Andela"},
		{"Ngozi", "Obi", "ngozi.obi@gtbank.com", "+234-701-567-8901", "GTBank"},
		{"Ibrahim", "Mohammed", "ibrahim@nnpc.gov.ng", "+234-805-678-9012", "NNPC"},
		{"Blessing", "Eke", "blessing.eke@jumia.com.ng", "+234-816-789-0123", "Jumia Nigeria"},
		{"Tunde", "Fashola", "tunde@lagosstate.gov.ng", "+234-807-890-1234", "Lagos State Govt"},
		{"Amara", "Eze", "amara.eze@pwc.com", "+234-818-901-2345", "PwC Nigeria"},
		{"Yusuf", "Bello", "yusuf.bello@dangote.com", "+234-802-012-3456", "Dangote Group"},
		{"Chidinma", "Ibrahim", "chidinma@kuda.com", "+234-908-111-2222", "Kuda Bank"},
		{"Olumide", "Sanusi", "olumide.sanusi@accessbank.com", "+234-909-222-3333", "Access Bank"},
		{"Halima", "Bello", "halima@zenithbank.com", "+234-905-333-4444", "Zenith Bank"},
		{"Emeka", "Uche", "emeka.uche@flourmills.com", "+234-906-444-5555", "Flour Mills"},
		{"Sade", "Ogunlesi", "sade@globacom.ng", "+234-808-555-6666", "Globacom"},
		{"Obinna", "Achebe", "obinna@lagosbiz.ng", "+234-815-666-7777", "Self-employed"},
		{"Funke", "Adepetu", "funke@multichoice.co.za", "+234-817-777-8888", "MultiChoice"},
		{"Babatunde", "Raji", "babatunde@stanbicibtc.com", "+234-810-888-9999", "Stanbic IBTC"},
		{"Kemi", "Adeosun", "kemi@oandoplc.com", "+234-811-999-0000", "Oando Plc"},
		{"Chidi", "Okpara", "chidi@intels.com.ng", "+234-813-000-1111", "Intels Nigeria"},
	}

	for _, g := range guests {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO guests (id, first_name, last_name, email, phone, nationality, company, id_type)
			VALUES (?, ?, ?, ?, ?, 'Nigerian', ?, 'national_id')
		`, fmt.Sprintf("guest-%s", g.last), g.first, g.last, g.email, g.phone, g.company)
	}

	// POS outlets
	_, _ = d.ExecContext(ctx, `
		INSERT OR IGNORE INTO outlets (id, property_id, name, code, type) VALUES
		('outlet-rest', '00000000-0000-0000-0000-000000000001', 'Palm Restaurant', 'REST', 'restaurant'),
		('outlet-bar', '00000000-0000-0000-0000-000000000001', 'Poolside Bar', 'BAR', 'bar'),
		('outlet-room', '00000000-0000-0000-0000-000000000001', 'Room Service', 'ROOM', 'room_service')
	`)

	// Menu items with Nigerian/African dishes
	menuItems := []struct {
		catName, catCode, item, code string
		price                       float64
	}{
		{"Nigerian Mains", "NG-M", "Jollof Rice & Chicken", "JOL-RIC", 4500},
		{"Nigerian Mains", "NG-M", "Pepper Soup (Goat Meat)", "PEP-GMT", 5500},
		{"Nigerian Mains", "NG-M", "Suya Platter", "SUY-PLT", 6000},
		{"Nigerian Mains", "NG-M", "Egusi Soup & Pounded Yam", "EGU-PY", 4000},
		{"Nigerian Mains", "NG-M", "Fried Rice & Plantain", "FRI-RIC", 3500},
		{"Nigerian Mains", "NG-M", "Banga Soup & Starch", "BAN-STR", 4500},
		{"Continental", "CON", "Grilled Tilapia", "GRA-TIL", 7000},
		{"Continental", "CON", "Steak & Chips", "STE-CHI", 8500},
		{"Continental", "CON", "Caesar Salad", "CAE-SAL", 3500},
		{"Snacks", "SNK", "Puff Puff & Sauce", "PUF-SAU", 1500},
		{"Snacks", "SNK", "Moi Moi", "MOI-MOI", 1200},
		{"Snacks", "SNK", "Spring Rolls (6 pcs)", "SPR-ROL", 2000},
		{"Breakfast", "BRK", "Full English Breakfast", "FUL-BRK", 5000},
		{"Breakfast", "BRK", "Nigerian Breakfast (Yam & Eggs)", "NIG-BRK", 3500},
		{"Breakfast", "BRK", "Fresh Fruit Platter", "FRU-PLT", 2500},
		{"Beverages", "BEV", "Chapman Cocktail", "CHA-COK", 2000},
		{"Beverages", "BEV", "Fresh Pineapple Juice", "PNA-JCE", 1500},
		{"Beverages", "BEV", "Zobo Drink", "ZOB-DRK", 1000},
		{"Beverages", "BEV", "Star Larger (Bottle)", "STA-LAG", 1500},
		{"Beverages", "BEV", "Guinness Stout (Bottle)", "GUI-STO", 1500},
		{"Desserts", "DES", "Pawpaw Ice Cream", "PAW-ICE", 2000},
		{"Desserts", "DES", "Chin Chin Platter", "CHN-PLT", 1500},
		{"Wine List", "WIN", "House Red Wine", "HSE-RED", 8000},
		{"Wine List", "WIN", "House White Wine", "HSE-WHT", 8000},
		{"Wine List", "WIN", "Hennessy XO", "HEN-XO", 45000},
		{"Wine List", "WIN", "Chivas Regal 12", "CHI-12", 25000},
		{"Wine List", "WIN", "Heineken (Bottle)", "HEI-BOT", 1200},
	}

	catID := 0
	lastCat := ""
	for _, mi := range menuItems {
		if mi.catName != lastCat {
			catID++
			lastCat = mi.catName
			_, _ = d.ExecContext(ctx, `
				INSERT OR IGNORE INTO menu_categories (id, property_id, outlet_id, name, code, display_order)
				VALUES (?, '00000000-0000-0000-0000-000000000001', 'outlet-rest', ?, ?, ?)
			`, fmt.Sprintf("cat-%d", catID), mi.catName, mi.catCode, catID)
		}
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO menu_items (id, property_id, category_id, name, code, price, preparation_time)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?)
		`, fmt.Sprintf("item-%s", mi.code), fmt.Sprintf("cat-%d", catID), mi.item, mi.code, mi.price, 15)
	}

	// Inventory stores
	_, _ = d.ExecContext(ctx, `
		INSERT OR IGNORE INTO inventory_stores (id, property_id, name, code, type) VALUES
		('store-main', '00000000-0000-0000-0000-000000000001', 'Main Store', 'MAIN', 'warehouse'),
		('store-kitchen', '00000000-0000-0000-0000-000000000001', 'Kitchen Store', 'KIT', 'department'),
		('store-bar', '00000000-0000-0000-0000-000000000001', 'Bar Store', 'BAR', 'department'),
		('store-housekeeping', '00000000-0000-0000-0000-000000000001', 'Housekeeping Store', 'HK', 'department')
	`)

	// Inventory products
	products := []struct {
		sku, name, category, uom string
		cost, sell, reorder      float64
	}{
		{"FOOD-001", "Basmati Rice (5kg)", "Food", "kg", 3500, 0, 50},
		{"FOOD-002", "Chicken Breast (kg)", "Food", "kg", 2800, 0, 30},
		{"FOOD-003", "Goat Meat (kg)", "Food", "kg", 4500, 0, 20},
		{"FOOD-004", "Fresh Tomatoes (kg)", "Food", "kg", 800, 0, 40},
		{"FOOD-005", "Onions (kg)", "Food", "kg", 500, 0, 30},
		{"FOOD-006", "Palm Oil (litre)", "Food", "litre", 1200, 0, 20},
		{"FOOD-007", "Garri (kg)", "Food", "kg", 400, 0, 50},
		{"FOOD-008", "Yam Tuber", "Food", "piece", 600, 0, 30},
		{"BEV-001", "Star Larger (crate)", "Beverage", "crate", 3600, 0, 20},
		{"BEV-002", "Guinness Stout (crate)", "Beverage", "crate", 3600, 0, 15},
		{"BEV-003", "Pineapple Juice (L)", "Beverage", "litre", 600, 0, 30},
		{"BEV-004", "Hennessy XO (bottle)", "Beverage", "bottle", 28000, 0, 5},
		{"SUP-001", "Toilet Paper (pack)", "Supplies", "pack", 1200, 0, 50},
		{"SUP-002", "Laundry Detergent (kg)", "Supplies", "kg", 2500, 0, 20},
		{"SUP-003", "Room Freshener", "Supplies", "unit", 800, 0, 30},
		{"SUP-004", "Hand Soap (L)", "Supplies", "litre", 1500, 0, 25},
		{"LIN-001", "Bath Towel", "Linen", "piece", 3000, 0, 100},
		{"LIN-002", "Bed Sheet (King)", "Linen", "piece", 4000, 0, 60},
		{"LIN-003", "Pillow Case", "Linen", "piece", 1500, 0, 80},
		{"CLE-001", "Floor Cleaner (L)", "Cleaning", "litre", 1000, 0, 30},
		{"CLE-002", "Bleach (L)", "Cleaning", "litre", 800, 0, 20},
		{"CLE-003", "Mop Head", "Cleaning", "piece", 500, 0, 15},
		{"ENG-001", "AC Filter", "Engineering", "piece", 5000, 0, 10},
		{"ENG-002", "Light Bulb (LED)", "Engineering", "piece", 1500, 0, 20},
		{"ENG-003", "Extension Socket", "Engineering", "piece", 3000, 0, 5},
	}

	for _, p := range products {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO inventory_products (id, property_id, sku, name, category, unit_of_measure, cost_price, reorder_level, maximum_stock, minimum_stock)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?, ?, ?, ?)
		`, fmt.Sprintf("prod-%s", p.sku), p.sku, p.name, p.category, p.uom, p.cost, p.reorder, p.reorder*3, p.reorder/2)
	}

	// Employees
	employees := []struct {
		num, first, last, dept, position string
		salary                           float64
	}{
		{"EMP-001", "Chidi", "Amadi", "Front Office", "Front Desk Manager", 350000},
		{"EMP-002", "Blessing", "Effiong", "Front Office", "Receptionist", 150000},
		{"EMP-003", "Tunde", "Bakare", "Food & Beverage", "Restaurant Manager", 300000},
		{"EMP-004", "Adaeze", "Igwe", "Food & Beverage", "Head Chef", 280000},
		{"EMP-005", "Kunle", "Ogundele", "Food & Beverage", "Waiter", 120000},
		{"EMP-006", "Funmi", "Adeyinka", "Food & Beverage", "Bartender", 120000},
		{"EMP-007", "Ibrahim", "Suleiman", "Housekeeping", "Executive Housekeeper", 280000},
		{"EMP-008", "Ngozi", "Okafor", "Housekeeping", "Room Attendant", 100000},
		{"EMP-009", "Emeka", "Chukwu", "Engineering", "Chief Engineer", 320000},
		{"EMP-010", "Yusuf", "Abdullahi", "Engineering", "Maintenance Technician", 150000},
		{"EMP-011", "Halima", "Bello", "Finance", "Finance Manager", 400000},
		{"EMP-012", "Olumide", "Sanusi", "Finance", "Accountant", 200000},
		{"EMP-013", "Sade", "Ogunlesi", "HR", "HR Manager", 350000},
		{"EMP-014", "Babatunde", "Raji", "Security", "Security Chief", 180000},
		{"EMP-015", "Chidinma", "Ike", "General", "General Manager", 600000},
	}

	for _, e := range employees {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO employees (id, property_id, employee_number, first_name, last_name, department_id, position, salary, employment_date)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?, ?, '2024-01-15')
		`, fmt.Sprintf("emp-%s", e.num), e.num, e.first, e.last, e.dept, e.position, e.salary)
	}

	// Link employees to user accounts for self-service
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-manager-01' WHERE id='emp-EMP-015'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-hr-01' WHERE id='emp-EMP-013'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-reception-01' WHERE id='emp-EMP-002'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-cashier-01' WHERE id='emp-EMP-012'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-housekeep-01' WHERE id='emp-EMP-007'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-kitchen-01' WHERE id='emp-EMP-003'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-account-01' WHERE id='emp-EMP-011'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET user_id='usr-waiter-01' WHERE id='emp-EMP-005'`)

	// Set reporting managers
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-015' WHERE id IN ('emp-EMP-001','emp-EMP-003','emp-EMP-007','emp-EMP-009','emp-EMP-011','emp-EMP-013','emp-EMP-014')`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-001' WHERE id='emp-EMP-002'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-003' WHERE id IN ('emp-EMP-004','emp-EMP-005','emp-EMP-006')`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-007' WHERE id='emp-EMP-008'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-009' WHERE id='emp-EMP-010'`)
	_, _ = d.ExecContext(ctx, `UPDATE employees SET reporting_manager='emp-EMP-011' WHERE id='emp-EMP-012'`)
	// ââ Rate Plans (inspired by Opera/eZee/Djubo) ââ
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	threeDaysAgo := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	ratePlans := []struct {
		id, code, name, rateType string
		rateMultiplier           float64
	}{
		{"rate-bar", "BAR", "Best Available Rate", "standard", 1.0},
		{"rate-corp", "CORP", "Corporate Rate", "corporate", 0.85},
		{"rate-govt", "GOVT", "Government Rate", "government", 0.75},
		{"rate-ota", "OTA", "OTA (Booking.com)", "ota", 0.90},
		{"rate-weekend", "WEEK", "Weekend Special", "promotional", 0.80},
		{"rate-longstay", "LONG", "Long Stay (7+ nights)", "promotional", 0.70},
		{"rate-earlybird", "EARL", "Early Bird (14+ days)", "promotional", 0.85},
		{"rate-group", "GRP", "Group/Conference Rate", "group", 0.80},
		{"rate-staff", "STF", "Staff Rate", "complimentary", 0.0},
		{"rate-comp", "CMP", "Complimentary", "complimentary", 0.0},
	}

	for _, rp := range ratePlans {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO rate_plans (id, property_id, code, name, base_price, status)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, 'active')
		`, rp.id, rp.code, rp.name, rp.rateMultiplier*25000)
	}

	// ââ Active Reservations (Opera-style: individual, corporate, OTA, walk-in) ââ
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	threeDaysLater := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	fiveDaysLater := time.Now().AddDate(0, 0, 5).Format("2006-01-02")
	sevenDaysLater := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	tenDaysLater := time.Now().AddDate(0, 0, 10).Format("2006-01-02")

	reservations := []struct {
		id, guestID, roomID, ratePlanID, status string
		arrival, departure                       string
		headlineRate                             float64
		adults, children, roomCount              int
		source, channel, company                 string
		isVip                                    bool
		specialRequests                          string
	}{
		// CHECKED-IN guests (in-house)
		{"res-001", "guest-Okonkwo", "room-001", "rate-corp", "checked_in", yesterday, fiveDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "MTN Nigeria", true, "Late check-in preferred"},
		{"res-002", "guest-Nwosu", "room-002", "rate-bar", "checked_in", yesterday, threeDaysLater, 25000, 1, 0, 1, "Direct", "direct", "Shell Nigeria", true, "Extra pillows"},
		{"res-003", "guest-Abubakar", "room-005", "rate-govt", "checked_in", twoDaysAgo, sevenDaysLater, 18750, 2, 1, 1, "Government", "direct", "NPA", false, "Government rate - breakfast included"},
		{"res-004", "guest-Adeyemi", "room-010", "rate-bar", "checked_in", yesterday, tomorrow, 25000, 1, 0, 1, "Website", "booking_engine", "Andela", false, "Quiet room please"},
		{"res-005", "guest-Obi", "room-015", "rate-corp", "checked_in", twoDaysAgo, fiveDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "GTBank", true, "Airport transfer needed"},
		{"res-006", "guest-Mohammed", "room-020", "rate-ota", "checked_in", yesterday, tenDaysLater, 22500, 2, 0, 1, "Booking.com", "channel_manager", "NNPC", false, "Late checkout if possible"},
		{"res-007", "guest-Eke", "room-025", "rate-corp", "checked_in", today, fiveDaysLater, 34000, 1, 0, 1, "Corporate", "direct", "Jumia Nigeria", false, "Meeting room on day 2"},
		{"res-008", "guest-Fashola", "room-041", "rate-corp", "checked_in", yesterday, threeDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "Lagos State Govt", true, "VIP - prefer floor 4"},
		{"res-009", "guest-Eze", "room-042", "rate-corp", "checked_in", twoDaysAgo, sevenDaysLater, 34000, 1, 0, 1, "Corporate", "direct", "PwC Nigeria", false, "Early breakfast 6:30am"},
		{"res-010", "guest-Bello", "room-050", "rate-bar", "checked_in", threeDaysAgo, tenDaysLater, 150000, 2, 1, 1, "Direct", "direct", "Dangote Group", true, "Presidential suite - full butler service"},
		// CONFIRMED arrivals today
		{"res-011", "guest-Ibrahim", "room-030", "rate-corp", "confirmed", today, threeDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "Kuda Bank", false, "Needs parking"},
		{"res-012", "guest-Sanusi", "room-031", "rate-corp", "confirmed", today, fiveDaysLater, 34000, 1, 0, 1, "Corporate", "direct", "Access Bank", false, "Conference attendee"},
		{"res-013", "guest-Bello-H", "room-032", "rate-ota", "confirmed", today, threeDaysLater, 22500, 2, 0, 1, "Booking.com", "channel_manager", "Zenith Bank", false, "Anniversary trip"},
		{"res-014", "guest-Uche", "room-033", "rate-corp", "confirmed", today, tomorrow, 34000, 1, 0, 1, "Corporate", "direct", "Flour Mills", false, "Just overnight"},
		{"res-015", "guest-Ogunlesi", "room-034", "rate-bar", "confirmed", today, threeDaysLater, 25000, 2, 1, 1, "Telephone", "direct", "Globacom", false, "Family room needed"},
		// CONFIRMED arrivals tomorrow
		{"res-016", "guest-Achebe", "room-003", "rate-weekend", "confirmed", tomorrow, threeDaysLater, 20000, 1, 0, 1, "Website", "booking_engine", "Self-employed", false, "Weekend getaway"},
		{"res-017", "guest-Adepetu", "room-006", "rate-corp", "confirmed", tomorrow, fiveDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "MultiChoice", false, "Executive floor preferred"},
		{"res-018", "guest-Raji", "room-007", "rate-ota", "confirmed", tomorrow, threeDaysLater, 22500, 1, 0, 1, "Expedia", "channel_manager", "Stanbic IBTC", false, ""},
		{"res-019", "guest-Adeosun", "room-008", "rate-longstay", "confirmed", tomorrow, sevenDaysLater, 17500, 2, 0, 1, "Direct", "direct", "Oando Plc", false, "Long stay discount applied"},
		{"res-020", "guest-Okpara", "room-043", "rate-corp", "confirmed", tomorrow, threeDaysLater, 34000, 2, 0, 1, "Corporate", "direct", "Intels Nigeria", false, "Crew rotation"},
		// GROUP BOOKING (Djubo/Opera group management)
		{"res-021", "guest-Okonkwo", "room-035", "rate-group", "confirmed", threeDaysLater, fiveDaysLater, 20000, 2, 0, 1, "Group", "direct", "MTN Nigeria", true, "MTN Board Meeting - Block of 5"},
		{"res-022", "guest-Nwosu", "room-036", "rate-group", "confirmed", threeDaysLater, fiveDaysLater, 20000, 2, 0, 1, "Group", "direct", "Shell Nigeria", true, "MTN Board Meeting"},
		{"res-023", "guest-Obi", "room-037", "rate-group", "confirmed", threeDaysLater, fiveDaysLater, 20000, 2, 0, 1, "Group", "direct", "GTBank", false, "MTN Board Meeting"},
		{"res-024", "guest-Eze", "room-038", "rate-group", "confirmed", threeDaysLater, fiveDaysLater, 20000, 1, 0, 1, "Group", "direct", "PwC Nigeria", false, "MTN Board Meeting"},
		{"res-025", "guest-Raji", "room-039", "rate-group", "confirmed", threeDaysLater, fiveDaysLater, 20000, 2, 0, 1, "Group", "direct", "Stanbic IBTC", false, "MTN Board Meeting"},
	}

	for _, res := range reservations {
		nights := 1
		// Calculate nights between arrival and departure
		arr, _ := time.Parse("2006-01-02", res.arrival)
		dep, _ := time.Parse("2006-01-02", res.departure)
		nights = int(dep.Sub(arr).Hours() / 24)
		if nights < 1 {
			nights = 1
		}
		confNum := fmt.Sprintf("CN-%s", res.id[4:])
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO reservations (id, property_id, confirmation_number, guest_id, rate_plan_id, room_id, status,
				arrival_date, departure_date, nights, adults, children, room_count,
				source, is_vip, special_requests, deposit_amount, deposit_paid, total_estimated)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, 1,
				?, ?, ?, ?, ?, ?)
		`, res.id, confNum, res.guestID, res.ratePlanID, res.roomID, res.status,
			res.arrival, res.departure, nights, res.adults, res.children,
			res.source, res.isVip, res.specialRequests, res.headlineRate, res.headlineRate, res.headlineRate*float64(nights))
	}

	// ââ Guest Stays (Opera-style: one stay per check-in) ââ
	stays := []struct {
		id, reservationID, guestID, roomID string
		checkIn, checkOut                  string
		status                            string
	}{
		{"stay-001", "res-001", "guest-Okonkwo", "room-001", yesterday, fiveDaysLater, "active"},
		{"stay-002", "res-002", "guest-Nwosu", "room-002", yesterday, threeDaysLater, "active"},
		{"stay-003", "res-003", "guest-Abubakar", "room-005", yesterday, sevenDaysLater, "active"},
		{"stay-004", "res-004", "guest-Adeyemi", "room-010", yesterday, tomorrow, "active"},
		{"stay-005", "res-005", "guest-Obi", "room-015", yesterday, fiveDaysLater, "active"},
		{"stay-006", "res-006", "guest-Mohammed", "room-020", yesterday, tenDaysLater, "active"},
		{"stay-007", "res-007", "guest-Eke", "room-025", today, fiveDaysLater, "active"},
		{"stay-008", "res-008", "guest-Fashola", "room-041", yesterday, threeDaysLater, "active"},
		{"stay-009", "res-009", "guest-Eze", "room-042", yesterday, sevenDaysLater, "active"},
		{"stay-010", "res-010", "guest-Bello", "room-050", threeDaysAgo, tenDaysLater, "active"},
	}

	for _, s := range stays {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO guest_stays (id, property_id, reservation_id, guest_id, room_id,
				expected_arrival, expected_departure, actual_arrival, status)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?, ?, ?)
		`, s.id, s.reservationID, s.guestID, s.roomID, s.checkIn, s.checkOut, s.checkIn, s.status)
	}

	// ââ Folios (Opera-style: guest billing) ââ
	for i, s := range stays {
		folioID := fmt.Sprintf("folio-%03d", i+1)
		roomRate := reservations[i].headlineRate
		nights := 1
		// Calculate charges
		ramCharge := roomRate * float64(nights)
		serviceCharge := ramCharge * 0.10
		vat := (ramCharge + serviceCharge) * 0.075
		totalCharges := ramCharge + serviceCharge + vat
	
depositPaid := roomRate * 1.5
		balance := totalCharges - depositPaid

		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO folios (id, property_id, guest_stay_id, folio_number, status, balance, currency)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, 'open', ?, 'NGN')
		`, folioID, s.id, fmt.Sprintf("F-%04d", i+1), balance)

		// Room charge
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date)
			VALUES (?, ?, '00000000-0000-0000-0000-000000000001', 'debit', 'ROOM', ?, ?, ?)
		`, fmt.Sprintf("fl-%03d-1", i+1), folioID, fmt.Sprintf("Room %s", reservations[i].roomID[5:]), ramCharge, today)
		// Service charge
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date)
			VALUES (?, ?, '00000000-0000-0000-0000-000000000001', 'debit', 'SVC', 'Service Charge (10%)', ?, ?)
		`, fmt.Sprintf("fl-%03d-2", i+1), folioID, serviceCharge, today)
		// VAT
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date)
			VALUES (?, ?, '00000000-0000-0000-0000-000000000001', 'debit', 'TAX', 'VAT (7.5%)', ?, ?)
		`, fmt.Sprintf("fl-%03d-3", i+1), folioID, vat, today)
		// Deposit
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date)
			VALUES (?, ?, '00000000-0000-0000-0000-000000000001', 'credit', 'DEP', 'Deposit Paid', ?, ?)
		`, fmt.Sprintf("fl-%03d-4", i+1), folioID, -depositPaid, today)
	}

	// ââ POS Orders (eZee-style: restaurant, bar, room service) ââ
	posOrders := []struct {
		outletID, guestStayID string
		totalAmount           float64
		status                string
		items                 []struct{ name string; qty int; price float64 }
	}{
		{
			"outlet-rest", "stay-001", 9500, "paid",
			[]struct{ name string; qty int; price float64 }{
				{"Jollof Rice & Chicken", 1, 4500},
				{"Chapman Cocktail", 2, 2000},
			},
		},
		{
			"outlet-bar", "stay-002", 14500, "paid",
			[]struct{ name string; qty int; price float64 }{
				{"Suya Platter", 1, 6000},
				{"Heineken (Bottle)", 3, 1200},
				{"Puff Puff & Sauce", 1, 1500},
			},
		},
		{
			"outlet-rest", "stay-003", 12000, "paid",
			[]struct{ name string; qty int; price float64 }{
				{"Grilled Tilapia", 1, 7000},
				{"Fresh Pineapple Juice", 2, 1500},
				{"Pawpaw Ice Cream", 1, 2000},
			},
		},
		{
			"outlet-room", "stay-010", 53000, "paid",
			[]struct{ name string; qty int; price float64 }{
				{"Steak & Chips", 1, 8500},
				{"Hennessy XO", 1, 45000},
			},
		},
		{
			"outlet-rest", "stay-005", 7500, "paid",
			[]struct{ name string; qty int; price float64 }{
				{"Egusi Soup & Pounded Yam", 1, 4000},
				{"Star Larger (Bottle)", 2, 1500},
			},
		},
		{
			"outlet-room", "stay-008", 8500, "pending",
			[]struct{ name string; qty int; price float64 }{
				{"Full English Breakfast", 1, 5000},
				{"Fresh Fruit Platter", 1, 2500},
				{"Zobo Drink", 1, 1000},
			},
		},
	}

	for oi, order := range posOrders {
		orderID := fmt.Sprintf("pos-order-%03d", oi+1)
		orderNum := fmt.Sprintf("ORD-%04d", oi+1)
		taxAmt := order.totalAmount * 0.075
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO pos_orders (id, property_id, outlet_id, order_number, guest_stay_id, subtotal, tax_amount, total, payment_status, created_at)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, orderID, order.outletID, orderNum, order.guestStayID, order.totalAmount, taxAmt, order.totalAmount+taxAmt, order.status)
		for ii, item := range order.items {
			// Find menu_item_id by name
			var menuItemID string
			_ = d.QueryRowContext(ctx, `SELECT id FROM menu_items WHERE name = ? LIMIT 1`, item.name).Scan(&menuItemID)
			if menuItemID == "" {
				menuItemID = fmt.Sprintf("item-missing-%d", ii)
			}
			_, _ = d.ExecContext(ctx, `
				INSERT OR IGNORE INTO pos_order_lines (id, order_id, menu_item_id, quantity, unit_price, amount, status)
				VALUES (?, ?, ?, ?, ?, ?, 'completed')
			`, fmt.Sprintf("pol-%03d-%d", oi+1, ii+1), orderID, menuItemID, item.qty, item.price, float64(item.qty)*item.price)
		}
	}

	// ââ Update room statuses for occupied rooms ââ
	_, _ = d.ExecContext(ctx, `UPDATE rooms SET status='occupied_clean', housekeeping_status='clean' WHERE id IN ('room-001','room-002','room-005','room-010','room-015','room-020','room-041','room-042')`)
	_, _ = d.ExecContext(ctx, `UPDATE rooms SET status='occupied_dirty', housekeeping_status='dirty' WHERE id IN ('room-025','room-050')`)
	_, _ = d.ExecContext(ctx, `UPDATE rooms SET status='vacant_clean', housekeeping_status='clean' WHERE id IN ('room-003','room-006','room-007','room-008','room-030','room-031','room-032','room-033','room-034')`)
	_, _ = d.ExecContext(ctx, `UPDATE rooms SET status='reserved', housekeeping_status='clean' WHERE id IN ('room-035','room-036','room-037','room-038','room-039','room-043')`)
	_, _ = d.ExecContext(ctx, `UPDATE rooms SET status='out_of_order' WHERE id IN ('room-045','room-046')`)

	// ââ Housekeeping Tasks (eZee-style: daily assignments) ââ
	housekeepingRooms := []struct{ roomID, task, priority string }{
		{"room-011", "Daily Cleaning", "normal"},
		{"room-012", "Daily Cleaning", "normal"},
		{"room-013", "Daily Cleaning", "normal"},
		{"room-014", "Daily Cleaning", "normal"},
		{"room-016", "Daily Cleaning", "normal"},
		{"room-017", "Daily Cleaning", "normal"},
		{"room-018", "Daily Cleaning", "normal"},
		{"room-019", "Daily Cleaning", "normal"},
		{"room-021", "Deep Cleaning", "high"},
		{"room-022", "Checkout Cleaning", "urgent"},
		{"room-023", "Daily Cleaning", "normal"},
		{"room-024", "Inspection Required", "high"},
		{"room-044", "Maintenance + Cleaning", "urgent"},
	}

	for hi, hk := range housekeepingRooms {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO housekeeping_tasks (id, property_id, room_id, task_type, priority, status, assigned_date)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, 'pending', ?)
		`, fmt.Sprintf("hk-%03d", hi+1), hk.roomID, hk.task, hk.priority, today)
	}

	// ââ Loyalty Tiers (Opera/Djubo-style loyalty program) ââ
	loyaltyTiers := []struct {
		id, name string
		minPts   int
		benefits string
	}{
		{"tier-member", "Member", 0, "Welcome drink, 5% F&B discount"},
		{"tier-silver", "Silver", 5000, "Late checkout, 10% discount, room upgrade"},
		{"tier-gold", "Gold", 15000, "Free breakfast, 15% discount, priority booking"},
		{"tier-platinum", "Platinum", 30000, "Suite upgrade, spa credit, airport transfer"},
		{"tier-vip", "VIP", 50000, "Personal concierge, complimentary minibar, VIP lounge"},
	}

	for _, lt := range loyaltyTiers {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO loyalty_tiers (id, property_id, name, code, min_points, benefits)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?)
		`, lt.id, lt.name, lt.name, lt.minPts, lt.benefits)
	}

	// ââ Event Venues (Sales & Catering - Opera-style) ââ
	venues := []struct {
		id, name string
		cap      int
		hRate    float64
		dRate    float64
		amenities string
	}{
		{"venue-palm", "Palm Ballroom", 300, 250000, 1500000, "Stage, AV system, dance floor, bar"},
		{"venue-ogba", "Ogba Conference Hall", 100, 100000, 600000, "Projector, screen, microphone, whiteboard"},
		{"venue-boards", "Boardroom", 20, 50000, 300000, "Video conferencing, smart TV, notepads"},
		{"venue-garden", "Garden Terrace", 150, 150000, 900000, "Open air, lighting, stage, generator backup"},
	}

	for _, v := range venues {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO event_venues (id, property_id, name, capacity, hourly_rate, daily_rate, amenities)
			VALUES (?, '00000000-0000-0000-0000-000000000001', ?, ?, ?, ?, ?)
		`, v.id, v.name, v.cap, v.hRate, v.dRate, v.amenities)
	}

	// ââ Event Bookings ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO event_bookings (id, property_id, venue_id, client_name, client_email, event_name, event_type, start_date, end_date, guest_count, package_name, total_amount, deposit_paid, status) VALUES
		('evt-001', '00000000-0000-0000-0000-000000000001', 'venue-palm', 'MTN Nigeria', 'events@mtnng.com', 'MTN Annual Board Meeting', 'conference', date('now', '+3 days'), date('now', '+5 days'), 45, 'Full Day Premium', 3000000, 1500000, 'confirmed'),
		('evt-002', '00000000-0000-0000-0000-000000000001', 'venue-ogba', 'GTBank', 'events@gtbank.com', 'GTBank Strategy Session', 'meeting', date('now', '+7 days'), date('now', '+7 days'), 25, 'Half Day Standard', 750000, 375000, 'tentative'),
		('evt-003', '00000000-0000-0000-0000-000000000001', 'venue-garden', 'Adaeze Okonkwo', 'adaeze.okonkwo@mtngroup.com', 'Adaeze & Emeka Wedding Reception', 'wedding', date('now', '+14 days'), date('now', '+14 days'), 200, 'Wedding Package', 8000000, 4000000, 'confirmed')
	`)

	// ââ Dynamic Pricing Rules (Opera/Djubo-style yield management) ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO pricing_rules (id, property_id, name, rule_type, min_occupancy, max_occupancy, price_multiplier, is_active) VALUES
		('pr-001', '00000000-0000-0000-0000-000000000001', 'Low Occupancy Discount', 'occupancy', 0, 40, 0.85, 1),
		('pr-002', '00000000-0000-0000-0000-000000000001', 'Medium Occupancy', 'occupancy', 40, 70, 1.0, 1),
		('pr-003', '00000000-0000-0000-0000-000000000001', 'High Occupancy Surge', 'occupancy', 70, 90, 1.25, 1),
		('pr-004', '00000000-0000-0000-0000-000000000001', 'Peak Demand Premium', 'occupancy', 90, 100, 1.50, 1),
		('pr-005', '00000000-0000-0000-0000-000000000001', 'Weekend Premium', 'seasonal', 0, 100, 1.15, 1),
		('pr-006', '00000000-0000-0000-0000-000000000001', 'Holiday Season Surge', 'seasonal', 0, 100, 1.40, 1)
	`)

	// ââ Group Booking ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO group_bookings (id, property_id, group_name, company, contact_name, contact_email, arrival_date, departure_date, room_count, rate_per_room, total_estimated, status) VALUES
		('grp-001', '00000000-0000-0000-0000-000000000001', 'MTN Board Meeting Block', 'MTN Nigeria', 'Adaeze Okonkwo', 'adaeze.okonkwo@mtngroup.com', date('now', '+3 days'), date('now', '+5 days'), 5, 20000, 200000, 'confirmed')
	`)

	// ââ Tax Rates ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO tax_rates (id, property_id, name, code, rate, type, applies_to, is_compound) VALUES
		('tax-vat', '00000000-0000-0000-0000-000000000001', 'Value Added Tax (VAT)', 'VAT', 7.5, 'percentage', 'all', 0),
		('tax-scvat', '00000000-0000-0000-0000-000000000001', 'Service Charge', 'SC', 10, 'percentage', 'all', 0),
		('tax-tourism', '00000000-0000-0000-0000-000000000001', 'Tourism Levy', 'TOUR', 2, 'percentage', 'accommodation', 0),
		('tax-luxury', '00000000-0000-0000-0000-000000000001', 'Luxury Tax', 'LUX', 5, 'percentage', 'room', 1),
		('tax-wholding', '00000000-0000-0000-0000-000000000001', 'Withholding Tax', 'WHT', 5, 'percentage', 'corporate', 0),
		('tax-lagos-tourism', '00000000-0000-0000-0000-000000000001', 'Lagos Tourism Development Levy', 'LTD', 1, 'percentage', 'all', 0)
	`)

	// ââ Payment Methods ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO payment_methods (id, property_id, name, code, type, sort_order, requires_reference) VALUES
		('pm-cash', '00000000-0000-0000-0000-000000000001', 'Cash', 'CASH', 'cash', 1, 0),
		('pm-pos', '00000000-0000-0000-0000-000000000001', 'POS Card Terminal', 'POS', 'card', 2, 1),
		('pm-visa', '00000000-0000-0000-0000-000000000001', 'Visa Card', 'VISA', 'card', 3, 1),
		('pm-mastercard', '00000000-0000-0000-0000-000000000001', 'Mastercard', 'MC', 'card', 4, 1),
		('pm-ussd', '00000000-0000-0000-0000-000000000001', 'Bank Transfer / USSD', 'USSD', 'transfer', 5, 1),
		('pm-paystack', '00000000-0000-0000-0000-000000000001', 'Paystack', 'PAYSK', 'online', 6, 1),
		('pm-flutterwave', '00000000-0000-0000-0000-000000000001', 'Flutterwave', 'FLW', 'online', 7, 1),
		('pm-opay', '00000000-0000-0000-0000-000000000001', 'OPay', 'OPAY', 'mobile', 8, 1),
		('pm-palmPay', '00000000-0000-0000-0000-000000000001', 'PalmPay', 'PALM', 'mobile', 9, 1),
		('pm-credit', '00000000-0000-0000-0000-000000000001', 'Room Charge / Credit', 'CREDIT', 'credit', 10, 0),
		('pm-complimentary', '00000000-0000-0000-0000-000000000001', 'Complimentary', 'COMP', 'complimentary', 11, 0)
	`)

	// ââ Departments ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO departments (id, property_id, name, code, sort_order) VALUES
		('dept-front', '00000000-0000-0000-0000-000000000001', 'Front Office', 'FO', 1),
		('dept-house', '00000000-0000-0000-0000-000000000001', 'Housekeeping', 'HK', 2),
		('dept-food', '00000000-0000-0000-0000-000000000001', 'Food & Beverage', 'FNB', 3),
		('dept-kitchen', '00000000-0000-0000-0000-000000000001', 'Kitchen', 'KIT', 4),
		('dept-bar', '00000000-0000-0000-0000-000000000001', 'Bar & Lounge', 'BAR', 5),
		('dept-spa', '00000000-0000-0000-0000-000000000001', 'Spa & Wellness', 'SPA', 6),
		('dept-acc', '00000000-0000-0000-0000-000000000001', 'Accounts & Finance', 'ACC', 7),
		('dept-hr', '00000000-0000-0000-0000-000000000001', 'Human Resources', 'HR', 8),
		('dept-eng', '00000000-0000-0000-0000-000000000001', 'Engineering & Maintenance', 'ENG', 9),
		('dept-store', '00000000-0000-0000-0000-000000000001', 'Stores & Procurement', 'STR', 10),
		('dept-sales', '00000000-0000-0000-0000-000000000001', 'Sales & Marketing', 'SMK', 11),
		('dept-security', '00000000-0000-0000-0000-000000000001', 'Security', 'SEC', 12),
		('dept-ict', '00000000-0000-0000-0000-000000000001', 'ICT & Systems', 'ICT', 13),
		('dept-events', '00000000-0000-0000-0000-000000000001', 'Events & Banqueting', 'EVT', 14)
	`)

	// ââ Numbering Sequences ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO numbering_sequences (id, property_id, name, prefix, next_number, padding, format) VALUES
		('seq-res', '00000000-0000-0000-0000-000000000001', 'Reservations', 'RES', 1025, 6, 'PREFIX-NNNNNN'),
		('seq-folio', '00000000-0000-0000-0000-000000000001', 'Folios', 'FOL', 1010, 6, 'PREFIX-NNNNNN'),
		('seq-guest', '00000000-0000-0000-0000-000000000001', 'Guests', 'GST', 1020, 6, 'PREFIX-NNNNNN'),
		('seq-pos', '00000000-0000-0000-0000-000000000001', 'POS Orders', 'POS', 1060, 6, 'PREFIX-NNNNNN'),
		('seq-po', '00000000-0000-0000-0000-000000000001', 'Purchase Orders', 'PO', 25, 6, 'PREFIX-NNNNNN'),
		('seq-inv', '00000000-0000-0000-0000-000000000001', 'Invoices', 'INV', 1010, 6, 'PREFIX-NNNNNN'),
		('seq-journal', '00000000-0000-0000-0000-000000000001', 'Journal Entries', 'JE', 50, 6, 'PREFIX-NNNNNN'),
		('seq-workorder', '00000000-0000-0000-0000-000000000001', 'Work Orders', 'WO', 12, 6, 'PREFIX-NNNNNN')
	`)

	// ââ System Settings ââ
	pID = "00000000-0000-0000-0000-000000000001"
	systemSettings := [][2]string{
		{"check_in_time", "14:00"}, {"check_out_time", "12:00"}, {"early_checkin_hours", "2"}, {"early_checkin_fee", "5000"},
		{"late_checkout_hours", "2"}, {"late_checkout_fee", "5000"}, {"max_occupancy_default", "4"}, {"default_deposit_pct", "50"},
		{"cancellation_hours", "24"}, {"cancellation_fee_pct", "100"}, {"no_show_fee_pct", "100"},
		{"currency_code", "NGN"}, {"currency_symbol", "â¦"}, {"decimal_places", "2"},
		{"date_format", "DD/MM/YYYY"}, {"time_format", "24h"}, {"fiscal_year_start", "01"},
		{"email_host", "smtp.gmail.com"}, {"email_port", "587"}, {"email_from", "reservations@grandpalm.ng"},
		{"sms_provider", "Termii"}, {"sms_from", "GrandPalm"},
		{"auto_room_charge", "true"}, {"night_audit_time", "02:00"},
		{"pos_auto_kot", "true"}, {"pos_receipt_footer", "Thank you for dining at Grand Palm Hotel!"},
		{"inventory_auto_reorder", "true"}, {"min_stock_alert", "true"},
		{"backup_enabled", "true"}, {"backup_interval", "daily"}, {"backup_retention_days", "30"},
		{"offline_sync_enabled", "true"}, {"offline_queue_max", "1000"},
		{"loyalty_points_per_naira", "1"}, {"loyalty_redemption_rate", "100"},
	}
	for i, kv := range systemSettings {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO system_settings (id, property_id, category, key, value, data_type) VALUES (?, ?, ?, ?, ?, 'string')`,
			fmt.Sprintf("sys-%03d", i+1), pID, "system", kv[0], kv[1])
	}

	// ââ Shift Templates ââ
	shifts := []struct{ id, name, code, start, end string; breakMin, overnight int }{
		{"shift-morning", "Morning Shift", "AM", "06:00", "14:00", 60, 0},
		{"shift-afternoon", "Afternoon Shift", "PM", "14:00", "22:00", 60, 0},
		{"shift-night", "Night Shift", "NT", "22:00", "06:00", 60, 1},
		{"shift-day", "Day Shift", "DY", "08:00", "17:00", 60, 0},
		{"shift-split", "Split Shift", "SP", "06:00", "22:00", 240, 0},
	}
	for _, s := range shifts {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO shift_templates (id, property_id, name, code, start_time, end_time, break_minutes, is_overnight) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			s.id, pID, s.name, s.code, s.start, s.end, s.breakMin, s.overnight)
	}

	// ââ Leave Types ââ
	leaveTypes := []struct{ id, name string; days int }{
		{"lt-annual", "Annual Leave", 21},
		{"lt-sick", "Sick Leave", 12},
		{"lt-maternity", "Maternity Leave", 90},
		{"lt-paternity", "Paternity Leave", 14},
		{"lt-compassionate", "Compassionate Leave", 5},
		{"lt-study", "Study Leave", 10},
		{"lt-unpaid", "Unpaid Leave", 30},
		{"lt-emergency", "Emergency Leave", 3},
	}
	for _, lt := range leaveTypes {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO leave_types (id, property_id, name, days_per_year, status) VALUES (?, ?, ?, ?, 'active')`,
			lt.id, pID, lt.name, lt.days)
	}
	holidays := []struct{ name, date string }{
		{"New Year's Day", "2026-01-01"}, {"Good Friday", "2026-04-03"},
		{"Easter Monday", "2026-04-06"}, {"Workers' Day", "2026-05-01"},
		{"Democracy Day", "2026-06-12"}, {"Eid el-Fitr", "2026-03-30"},
		{"Eid el-Kabir", "2026-06-06"}, {"Independence Day", "2026-10-01"},
		{"Christmas Day", "2026-12-25"}, {"Boxing Day", "2026-12-26"},
	}
	for _, h := range holidays {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO public_holidays (id, property_id, name, holiday_date, is_recurring) VALUES (?, ?, ?, ?, 1)`,
			"hol-"+h.date, pID, h.name, h.date)
	}

	// ââ Training Courses ââ
	courses := []struct{ id, name, cat string; hours float64; mandatory int }{
		{"tc-fire", "Fire Safety & Emergency Response", "Safety", 8, 1},
		{"tc-food", "Food Safety & Hygiene (HACCP)", "F&B", 16, 1},
		{"tc-guest", "Guest Service Excellence", "Hospitality", 8, 1},
		{"tc-security", "Security Awareness & Procedures", "Security", 4, 1},
		{"tc-data", "Data Protection & Privacy", "Compliance", 4, 0},
		{"tc-firstaid", "First Aid & CPR", "Safety", 8, 0},
		{"tc-leadership", "Leadership & Team Management", "Development", 16, 0},
		{"tc-pos", "POS System Operations", "Technical", 4, 0},
	}
	for _, c := range courses {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO training_courses (id, property_id, name, category, duration_hours, is_mandatory) VALUES (?, ?, ?, ?, ?, ?)`,
			c.id, pID, c.name, c.cat, c.hours, c.mandatory)
	}

	// ââ Employee Contracts for existing employees ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO employee_contracts (id, employee_id, property_id, contract_type, start_date, probation_end_date, probation_status) VALUES
		('ctr-001', 'emp-EMP-001', ?, 'permanent', '2024-03-15', '2024-09-15', 'completed'),
		('ctr-002', 'emp-EMP-002', ?, 'permanent', '2023-06-01', '2023-12-01', 'completed'),
		('ctr-003', 'emp-EMP-003', ?, 'permanent', '2024-01-10', '2024-07-10', 'completed'),
		('ctr-004', 'emp-EMP-004', ?, 'contract', '2025-01-01', '2025-07-01', 'completed'),
		('ctr-005', 'emp-EMP-005', ?, 'permanent', '2023-09-01', '2024-03-01', 'completed'),
		('ctr-006', 'emp-EMP-006', ?, 'permanent', '2024-06-15', '2024-12-15', 'completed'),
		('ctr-007', 'emp-EMP-007', ?, 'permanent', '2023-11-01', '2024-05-01', 'completed'),
		('ctr-008', 'emp-EMP-008', ?, 'permanent', '2024-02-01', '2024-08-01', 'completed'),
		('ctr-009', 'emp-EMP-009', ?, 'contract', '2025-06-01', '2025-12-01', 'active'),
		('ctr-010', 'emp-010', ?, 'permanent', '2023-04-01', '2023-10-01', 'completed'),
		('ctr-011', 'emp-011', ?, 'permanent', '2024-08-01', '2025-02-01', 'completed'),
		('ctr-012', 'emp-012', ?, 'casual', '2025-03-01', NULL, 'active'),
		('ctr-013', 'emp-013', ?, 'permanent', '2023-07-01', '2024-01-01', 'completed'),
		('ctr-014', 'emp-014', ?, 'permanent', '2024-04-01', '2024-10-01', 'completed'),
		('ctr-015', 'emp-015', ?, 'permanent', '2023-01-15', '2023-07-15', 'completed')
	`, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID)

	// ââ Performance Cycle ââ
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO performance_cycles (id, property_id, name, start_date, end_date, status) VALUES
		('pc-2026-h1', ?, '2026 H1 Performance Review', '2026-01-01', '2026-06-30', 'completed'),
		('pc-2026-h2', ?, '2026 H2 Performance Review', '2026-07-01', '2026-12-31', 'active')
	`, pID, pID)

	// ── Employee Profiles ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO employee_profiles (id, employee_id, gender, nationality, marital_status, emergency_contact_name, emergency_contact_phone, emergency_contact_relation, city, state, country) VALUES
		('ep-001', 'emp-EMP-001', 'Male', 'Nigerian', 'Married', 'Amadi Ngozi', '+234-801-234-5678', 'Spouse', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-002', 'emp-EMP-002', 'Female', 'Nigerian', 'Single', 'Effiong James', '+234-802-345-6789', 'Father', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-003', 'emp-EMP-003', 'Male', 'Nigerian', 'Married', 'Bakare Grace', '+234-803-456-7890', 'Spouse', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-004', 'emp-EMP-004', 'Female', 'Nigerian', 'Single', 'Igwe Obiora', '+234-804-567-8901', 'Brother', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-005', 'emp-EMP-005', 'Male', 'Nigerian', 'Married', 'Ogundele Yetunde', '+234-805-678-9012', 'Spouse', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-006', 'emp-EMP-006', 'Female', 'Nigerian', 'Single', 'Adeyinka Tunde', '+234-806-789-0123', 'Father', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-007', 'emp-EMP-007', 'Male', 'Nigerian', 'Married', 'Suleiman Fatima', '+234-807-890-1234', 'Spouse', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-008', 'emp-EMP-008', 'Female', 'Nigerian', 'Single', 'Okafor Chinedu', '+234-808-901-2345', 'Brother', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-009', 'emp-EMP-009', 'Male', 'Nigerian', 'Married', 'Chukwu Ada', '+234-809-012-3456', 'Spouse', 'Lagos', 'Lagos', 'Nigeria'),
		('ep-010', 'emp-EMP-010', 'Male', 'Nigerian', 'Single', 'Abdullahi Hauwa', '+234-810-123-4567', 'Sister', 'Lagos', 'Lagos', 'Nigeria')
	`, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID)

	// ── Attendance Records (last 7 days) ──
	for d2 := 0; d2 < 7; d2++ {
		date := time.Now().AddDate(0, 0, -d2).Format("2006-01-02")
		isWeekend := time.Now().AddDate(0, 0, -d2).Weekday() == time.Saturday || time.Now().AddDate(0, 0, -d2).Weekday() == time.Sunday
		for i := 1; i <= 15; i++ {
			empID := fmt.Sprintf("emp-EMP-%03d", i)
			attID := fmt.Sprintf("att-%s-%03d", date, i)
			clockInH, clockInM := 7+((i*3)%3), ((i*17)%50)
			clockIn := fmt.Sprintf("%s %02d:%02d:00", date, clockInH, clockInM)
			clockOut := fmt.Sprintf("%s %d:%02d:00", date, 16+((i*2)%3), ((i*13)%55))
			var status string
			if isWeekend && i > 10 {
				status = "absent"
				clockIn, clockOut = "", ""
			} else if i%7 == 0 {
				status = "late"
				clockIn = fmt.Sprintf("%s %02d:%02d:00", date, 9+((i*3)%2), clockInM)
			} else {
				status = "present"
				if i%5 == 0 {
				} else {
				}
			}
			lateMin := 0
			if status == "late" {
				lateMin = 15 + (i * 5)
			}
			overtimeH := 0.0
			if i%4 == 0 && !isWeekend {
				overtimeH = float64(1 + (i % 3))
			}
			_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO attendance_records (id, employee_id, property_id, record_date, clock_in, clock_out, gross_hours, net_hours, overtime_hours, late_minutes, status, source) VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, 'seed')`,
				attID, empID, pID, date, clockIn, clockOut, 8.0+overtimeH, 7.0+overtimeH, overtimeH, lateMin, status)
		}
	}

	// ── Leave Requests ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO leave_requests (id, employee_id, leave_type_id, start_date, end_date, days_requested, reason, status, property_id) VALUES
		('lr-001', 'emp-EMP-002', 'lt-annual', '2026-09-10', '2026-09-14', 5, 'Family vacation', 'approved', ?),
		('lr-002', 'emp-EMP-005', 'lt-sick', '2026-09-08', '2026-09-09', 2, 'Medical appointment', 'approved', ?),
		('lr-003', 'emp-EMP-008', 'lt-annual', '2026-09-15', '2026-09-19', 5, 'Personal leave', 'pending', ?),
		('lr-004', 'emp-EMP-003', 'lt-paternity', '2026-09-20', '2026-10-03', 14, 'New baby', 'approved', ?),
		('lr-005', 'emp-EMP-011', 'lt-maternity', '2026-10-01', '2026-12-30', 90, 'Maternity leave', 'approved', ?),
		('lr-006', 'emp-EMP-006', 'lt-study', '2026-09-12', '2026-09-17', 5, 'Professional certification exam', 'pending', ?),
		('lr-007', 'emp-EMP-004', 'lt-emergency', '2026-09-07', '2026-09-09', 3, 'Family emergency', 'approved', ?),
		('lr-008', 'emp-EMP-009', 'lt-compassionate', '2026-09-11', '2026-09-15', 5, 'Bereavement', 'approved', ?)
	`, pID, pID, pID, pID, pID, pID, pID, pID)

	// ── Training Enrollments ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO training_enrollments (id, employee_id, course_id, enrollment_date, status, score, property_id) VALUES
		('te-001', 'emp-EMP-001', 'tc-fire-safety', '2026-01-15', 'completed', 92, ?),
		('te-002', 'emp-EMP-002', 'tc-fire-safety', '2026-01-15', 'completed', 88, ?),
		('te-003', 'emp-EMP-003', 'tc-food-safety', '2026-02-01', 'completed', 95, ?),
		('te-004', 'emp-EMP-004', 'tc-food-safety', '2026-02-01', 'completed', 97, ?),
		('te-005', 'emp-EMP-005', 'tc-guest-service', '2026-03-01', 'completed', 85, ?),
		('te-006', 'emp-EMP-007', 'tc-fire-safety', '2026-01-15', 'completed', 90, ?),
		('te-007', 'emp-EMP-008', 'tc-hygiene', '2026-04-01', 'in_progress', NULL, ?),
		('te-008', 'emp-EMP-009', 'tc-maintenance', '2026-03-15', 'completed', 93, ?),
		('te-009', 'emp-EMP-010', 'tc-maintenance', '2026-03-15', 'enrolled', NULL, ?),
		('te-010', 'emp-EMP-011', 'tc-fire-safety', '2026-01-15', 'completed', 91, ?),
		('te-011', 'emp-EMP-012', 'tc-first-aid', '2026-05-01', 'enrolled', NULL, ?),
		('te-012', 'emp-EMP-013', 'tc-fire-safety', '2026-01-15', 'completed', 94, ?),
		('te-013', 'emp-EMP-014', 'tc-security', '2026-02-15', 'completed', 89, ?),
		('te-014', 'emp-EMP-015', 'tc-leadership', '2026-01-01', 'completed', 96, ?),
		('te-015', 'emp-EMP-006', 'tc-food-safety', '2026-02-01', 'completed', 87, ?)
	`, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID, pID)

	// ── Traces / Tasks ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO traces (id, property_id, guest_id, reservation_id, trace_type, title, description, department, priority, status, assigned_to, due_date, created_by) VALUES
		('tr-001', ?, 'guest-Okonkwo', 'res-001', 'amenity', 'Prepare Anniversary Amenities', 'Guest celebrating anniversary. Prepare champagne, flowers, and chocolate strawberries.', 'Housekeeping', 'high', 'pending', 'Ibrahim Suleiman', date('now', '+1 day'), 'System'),
		('tr-002', ?, 'guest-Nwosu', 'res-002', 'service', 'Call Guest at 6 PM', 'Guest requested wake-up call and restaurant recommendation.', 'Front Office', 'normal', 'pending', 'Chioma Okoro', date('now'), 'System'),
		('tr-003', ?, 'guest-Abubakar', 'res-003', 'transport', 'Airport Pickup 8:30 AM', 'VIP airport transfer scheduled for tomorrow. Black SUV confirmed.', 'Concierge', 'high', 'completed', 'Babatunde Raji', date('now', '+1 day'), 'System'),
		('tr-004', ?, NULL, NULL, 'maintenance', 'Room 405 Inspection', 'Guest reported AC issue in Room 405. Engineering to inspect before next check-in.', 'Engineering', 'urgent', 'in_progress', 'Emeka Chukwu', date('now'), 'System'),
		('tr-005', ?, 'guest-Obi', 'res-005', 'vip', 'VIP Welcome Package', 'Return guest (8th stay). Prepare personalized welcome card and preferred wine.', 'Guest Relations', 'high', 'pending', 'Chidi Amadi', date('now'), 'System'),
		('tr-006', ?, 'guest-Okonkwo', 'res-001', 'billing', 'Late Checkout Approval', 'Guest requested 2 PM late checkout. Manager approval needed.', 'Front Office', 'normal', 'pending', 'Chioma Okoro', date('now'), 'System'),
		('tr-007', ?, NULL, NULL, 'housekeeping', 'Deep Clean Suite 701', 'Suite 701 checkout today. Deep clean required before VIP arrival.', 'Housekeeping', 'high', 'pending', 'Ibrahim Suleiman', date('now'), 'System'),
		('tr-008', ?, 'guest-Fashola', 'res-008', 'dining', 'Dietary Requirements', 'Guest has nut allergy. Inform restaurant and kitchen staff.', 'Food & Beverage', 'urgent', 'completed', 'Tunde Bakare', date('now'), 'System')
	`, pID, pID, pID, pID, pID, pID, pID, pID)

	// ── Guest Service Requests ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO guest_service_requests (id, guest_id, reservation_id, property_id, request_type, description, priority, status, department) VALUES
		('gsr-001', 'guest-Okonkwo', 'res-001', ?, 'room_service', 'Extra towels and pillows', 'normal', 'completed', 'Housekeeping'),
		('gsr-002', 'guest-Nwosu', 'res-002', ?, 'concierge', 'Restaurant reservation for 2 at 8 PM', 'normal', 'completed', 'Concierge'),
		('gsr-003', 'guest-Abubakar', 'res-003', ?, 'maintenance', 'Air conditioning not cooling properly', 'high', 'open', 'Engineering'),
		('gsr-004', 'guest-Obi', 'res-005', ?, 'laundry', 'Express laundry service', 'normal', 'completed', 'Housekeeping'),
		('gsr-005', 'guest-Fashola', 'res-008', ?, 'spa', 'Book Swedish Massage for tomorrow at 2 PM', 'normal', 'pending', 'Spa'),
		('gsr-006', 'guest-Okonkwo', 'res-001', ?, 'transport', 'Airport transfer for 6 AM departure', 'high', 'pending', 'Concierge')
	`, pID, pID, pID, pID, pID, pID)

	// ── Guest Interactions ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO guest_interactions (id, guest_id, reservation_id, property_id, interaction_type, channel, subject, message, staff_name) VALUES
		('gi-001', 'guest-Okonkwo', 'res-001', ?, 'note', 'phone', 'Welcome Call', 'Called guest upon arrival. Confirmed anniversary celebration. VIP treatment activated.', 'Chioma Okoro'),
		('gi-002', 'guest-Nwosu', 'res-002', ?, 'complaint', 'email', 'WiFi Issue', 'Guest reported slow WiFi in Room 302. IT resolved within 30 minutes.', 'Emeka Chukwu'),
		('gi-003', 'guest-Abubakar', 'res-003', ?, 'request', 'in_person', 'Late Checkout', 'VIP guest requesting 3 PM late checkout. Approved by GM.', 'Chidi Amadi'),
		('gi-004', 'guest-Obi', 'res-005', ?, 'compliment', 'survey', 'Excellent Service', 'Guest praised housekeeping team. Rated 5/5 on satisfaction survey.', 'System'),
		('gi-005', 'guest-Fashola', 'res-008', ?, 'note', 'chat', 'Dietary Alert', 'Guest flagged nut allergy. Information shared with F&B team.', 'Tunde Bakare')
	`, pID, pID, pID, pID, pID)

	// ── Biometric Devices ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO attendance_devices (id, property_id, name, device_model, serial_number, ip_address, port, protocol, location, department, status, capabilities) VALUES
		('dev-001', ?, 'Main Lobby Face Terminal', 'Dahua ASA3223A-W', 'DHW20240815001', '192.168.1.101', 80, 'dahua', 'Main Lobby', 'Front Office', 'online', '["READ_ATTENDANCE","READ_USER","READ_DEVICE_INFO"]'),
		('dev-002', ?, 'Staff Entrance Face Terminal', 'Dahua ASI3201C-S', 'DHW20240815002', '192.168.1.102', 80, 'dahua', 'Staff Entrance', 'Security', 'online', '["READ_ATTENDANCE","WRITE_USER","READ_USER"]'),
		('dev-003', ?, 'Kitchen Access Terminal', 'ZKTeco uFace 800', 'ZKT20240815001', '192.168.1.103', 80, 'mock', 'Kitchen Entrance', 'Food & Beverage', 'offline', '["READ_ATTENDANCE","READ_USER"]')
	`, pID, pID, pID)

	// ── Device User Mappings ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO attendance_device_users (id, device_id, employee_id, device_user_id, enrolled) VALUES
		('du-001', 'dev-001', 'emp-EMP-001', '1001', 1),
		('du-002', 'dev-001', 'emp-EMP-002', '1002', 1),
		('du-003', 'dev-001', 'emp-EMP-003', '1003', 1),
		('du-004', 'dev-001', 'emp-EMP-007', '1007', 1),
		('du-005', 'dev-001', 'emp-EMP-009', '1009', 1),
		('du-006', 'dev-001', 'emp-EMP-013', '1013', 1),
		('du-007', 'dev-002', 'emp-EMP-004', '2004', 1),
		('du-008', 'dev-002', 'emp-EMP-005', '2005', 1),
		('du-009', 'dev-002', 'emp-EMP-006', '2006', 1),
		('du-010', 'dev-002', 'emp-EMP-010', '2010', 1),
		('du-011', 'dev-002', 'emp-EMP-014', '2014', 1),
		('du-012', 'dev-003', 'emp-EMP-004', '3004', 1),
		('du-013', 'dev-003', 'emp-EMP-005', '3005', 1),
		('du-014', 'dev-003', 'emp-EMP-006', '3006', 1)
	`)

	// ── Room Assignment Rules ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO room_assignment_rules (id, property_id, rule_name, weight_guest_preference, weight_room_readiness, weight_room_type, weight_floor_preference, weight_housekeeping, weight_upgrade) VALUES
		('rar-001', ?, 'Default Hotel Rules', 0.30, 0.20, 0.20, 0.10, 0.10, 0.10),
		('rar-002', ?, 'VIP Override', 0.40, 0.25, 0.15, 0.10, 0.05, 0.05),
		('rar-003', ?, 'High Occupancy Mode', 0.15, 0.25, 0.15, 0.05, 0.20, 0.20)
	`, pID, pID, pID)

	// ── Room Assignments ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO room_assignments (id, reservation_id, room_id, property_id, assigned_by, assignment_score, assignment_reasons, is_upgrade, is_manual) VALUES
		('ra-001', 'res-001', 'room-001', ?, 'System', 92.5, 'High floor preference matched, anniversary amenities pre-set, room type matched', 0, 0),
		('ra-002', 'res-002', 'room-002', ?, 'Chioma Okoro', 88.0, 'Guest floor preference 2nd floor matched, connecting room available', 0, 1),
		('ra-003', 'res-003', 'room-005', ?, 'System', 95.0, 'VIP status, top floor suite assigned, room readiness confirmed', 1, 0),
		('ra-004', 'res-005', 'room-015', ?, 'System', 97.0, 'Return guest preference: high floor, suite type, previous room matched', 0, 0)
	`, pID, pID, pID, pID)

	// ── Simulated OTA Channel Bookings ──
	// Booking.com bookings (15% commission)
	otaBookings := []struct {
		id, channelID, channelCode, otaRef, guestName, guestEmail string
		roomTypeID, checkIn, checkOut string
		nights int
		ratePerNight, totalAmount, commission, netAmount float64
		status, paymentStatus string
		numGuests int
	}{
		// Booking.com (15% commission)
		{"chb-001", "ch-bkng", "bkng", "BK-2847391", "James Mitchell", "james.m@gmail.com", "rt-standard", "2026-09-01", "2026-09-04", 3, 25000, 75000, 11250, 63750, "confirmed", "paid", 2},
		{"chb-002", "ch-bkng", "bkng", "BK-2847502", "Sarah Okonkwo", "sarah.o@yahoo.com", "rt-deluxe", "2026-09-02", "2026-09-05", 3, 35000, 105000, 15750, 89250, "confirmed", "paid", 2},
		{"chb-003", "ch-bkng", "bkng", "BK-2847805", "Ahmed Al-Hassan", "ahmed.h@outlook.com", "rt-executive", "2026-09-03", "2026-09-06", 3, 55000, 165000, 24750, 140250, "confirmed", "paid", 1},
		{"chb-004", "ch-bkng", "bkng", "BK-2848100", "Chioma Nwosu", "chioma.n@icloud.com", "rt-standard", "2026-09-05", "2026-09-07", 2, 25000, 50000, 7500, 42500, "confirmed", "paid", 2},
		{"chb-005", "ch-bkng", "bkng", "BK-2848320", "David Osei", "david.o@gmail.com", "rt-presidential", "2026-09-06", "2026-09-09", 3, 95000, 285000, 42750, 242250, "confirmed", "pending", 2},
		{"chb-006", "ch-bkng", "bkng", "BK-2848500", "Fatima Bello", "fatima.b@yahoo.com", "rt-standard", "2026-09-08", "2026-09-10", 2, 25000, 50000, 7500, 42500, "confirmed", "paid", 1},
		{"chb-007", "ch-bkng", "bkng", "BK-2848700", "Michael Adekunle", "michael.a@gmail.com", "rt-deluxe", "2026-09-10", "2026-09-13", 3, 35000, 105000, 15750, 89250, "confirmed", "paid", 3},
		// Expedia (18% commission)
		{"chb-008", "ch-expe", "expe", "EX-9023451", "Lisa Chang", "lisa.c@expedia.com", "rt-standard", "2026-09-01", "2026-09-03", 2, 26000, 52000, 9360, 42640, "confirmed", "paid", 1},
		{"chb-009", "ch-expe", "expe", "EX-9023700", "Oluwaseun Adewale", "seun.a@gmail.com", "rt-deluxe", "2026-09-02", "2026-09-05", 3, 37000, 111000, 19980, 91020, "confirmed", "paid", 2},
		{"chb-010", "ch-expe", "expe", "EX-9024000", "Rachel Okonjo", "rachel.o@outlook.com", "rt-executive", "2026-09-04", "2026-09-07", 3, 58000, 174000, 31320, 142680, "confirmed", "paid", 2},
		{"chb-011", "ch-expe", "expe", "EX-9024300", "Emmanuel Asante", "emmanuel.a@gmail.com", "rt-standard", "2026-09-05", "2026-09-06", 1, 26000, 26000, 4680, 21320, "confirmed", "paid", 1},
		{"chb-012", "ch-expe", "expe", "EX-9024500", "Grace Okwu", "grace.o@yahoo.com", "rt-deluxe", "2026-09-07", "2026-09-10", 3, 37000, 111000, 19980, 91020, "confirmed", "paid", 2},
		{"chb-013", "ch-expe", "expe", "EX-9024800", "Tunde Bakare", "tunde.b@gmail.com", "rt-standard", "2026-09-09", "2026-09-11", 2, 26000, 52000, 9360, 42640, "confirmed", "pending", 2},
		// Airbnb (14% commission)
		{"chb-014", "ch-airb", "airb", "HM-ABCD1234", "Yuki Tanaka", "yuki.t@airbnb.com", "rt-standard", "2026-09-01", "2026-09-05", 4, 22000, 88000, 12320, 75680, "confirmed", "paid", 1},
		{"chb-015", "ch-airb", "airb", "HM-EFGH5678", "Ngozi Eze", "ngozi.e@gmail.com", "rt-deluxe", "2026-09-03", "2026-09-06", 3, 32000, 96000, 13440, 82560, "confirmed", "paid", 2},
		{"chb-016", "ch-airb", "airb", "HM-IJKL9012", "Ben Thompson", "ben.t@outlook.com", "rt-standard", "2026-09-04", "2026-09-06", 2, 22000, 44000, 6160, 37840, "confirmed", "paid", 1},
		{"chb-017", "ch-airb", "airb", "HM-MNOP3456", "Amina Yusuf", "amina.y@gmail.com", "rt-executive", "2026-09-06", "2026-09-10", 4, 50000, 200000, 28000, 172000, "confirmed", "paid", 2},
		{"chb-018", "ch-airb", "airb", "HM-QRST7890", "Kevin Obi", "kevin.o@yahoo.com", "rt-standard", "2026-09-08", "2026-09-09", 1, 22000, 22000, 3080, 18920, "confirmed", "paid", 1},
	}

	for _, bk := range otaBookings {
		_, _ = d.ExecContext(ctx, `
			INSERT OR IGNORE INTO channel_bookings
				(id, channel_id, property_id, channel_booking_ref, channel_code, guest_name, guest_email,
				 room_type, check_in, check_out, nights, total_amount, currency, status,
				 rate_per_night, commission, net_amount, payment_status, number_of_guests, booked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'NGN', ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, bk.id, bk.channelID, pID, bk.otaRef, bk.channelCode, bk.guestName, bk.guestEmail,
			bk.roomTypeID, bk.checkIn, bk.checkOut, bk.nights, bk.totalAmount, bk.status,
			bk.ratePerNight, bk.commission, bk.netAmount, bk.paymentStatus, bk.numGuests)
	}

	// Update channel totals
	_, _ = d.ExecContext(ctx, `UPDATE channels SET total_bookings=7, total_revenue=835000 WHERE id='ch-bkng'`)
	_, _ = d.ExecContext(ctx, `UPDATE channels SET total_bookings=6, total_revenue=526000 WHERE id='ch-expe'`)
	_, _ = d.ExecContext(ctx, `UPDATE channels SET total_bookings=5, total_revenue=450000 WHERE id='ch-airb'`)

	// ── Restaurant Tables (16 tables: Main, Terrace, VIP, Private) ──
	rTables := []struct{ id, number, section string; capacity int }{
		{"rt-01", "1", "Main", 4}, {"rt-02", "2", "Main", 4}, {"rt-03", "3", "Main", 6},
		{"rt-04", "4", "Main", 2}, {"rt-05", "5", "Main", 4}, {"rt-06", "6", "Main", 8},
		{"rt-07", "7", "Main", 4}, {"rt-08", "8", "Main", 4}, {"rt-09", "9", "Terrace", 4},
		{"rt-10", "10", "Terrace", 6}, {"rt-11", "11", "Terrace", 4}, {"rt-12", "12", "Terrace", 2},
		{"rt-13", "V1", "VIP", 10}, {"rt-14", "V2", "VIP", 8}, {"rt-15", "P1", "Private", 12}, {"rt-16", "P2", "Private", 16},
	}
	for _, t := range rTables {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_tables (id, property_id, number, section, capacity, status) VALUES (?, ?, ?, ?, ?, 'available')`, t.id, pID, t.number, t.section, t.capacity)
	}

	// ── Restaurant Menu (Nigerian & International) ──
	rMenu := []struct{ name, cat, desc string; price, cost float64; prep int; tags string }{
		{"Jollof Rice", "main", "Classic Nigerian jollof with grilled chicken", 4500, 1200, 20, "spicy"},
		{"Fried Rice", "main", "Nigerian fried rice with mixed vegetables", 4500, 1100, 18, ""},
		{"Ofada Rice", "main", "Local ofada rice with ayamase sauce", 5500, 1800, 25, "spicy"},
		{"Egusi Soup & Pounded Yam", "main", "Melon seed soup with assorted meat", 5000, 2000, 25, ""},
		{"Banga Soup & Starch", "main", "Delta-style banga with fresh fish", 6000, 2500, 30, ""},
		{"Grilled Tilapia", "main", "Whole tilapia with pepper sauce & plantain", 7500, 3000, 30, ""},
		{"Suya Skewers", "main", "Spiced beef suya with onions & tomatoes", 3500, 1500, 15, "spicy"},
		{"Chicken Shawarma", "main", "Spiced chicken wrap with garlic sauce", 3000, 900, 12, ""},
		{"Pepper Soup", "starter", "Goat meat pepper soup", 3000, 1200, 20, "spicy"},
		{"Spring Rolls", "starter", "Vegetable spring rolls (6 pcs)", 2000, 500, 10, "vegetarian"},
		{"Puff Puff", "starter", "Nigerian doughnuts (8 pcs)", 1500, 300, 15, ""},
		{"Chin Chin", "starter", "Crunchy fried dough snack", 1000, 200, 10, ""},
		{"Gizdodo", "starter", "Gizzard & plantain pepper sauce", 3500, 1500, 15, "spicy"},
		{"Pancakes", "dessert", "Nigerian-style pancakes with honey", 2000, 400, 10, ""},
		{"Chin Chin Sundae", "dessert", "Ice cream topped with chin chin & chocolate", 3000, 800, 5, ""},
		{"Fruit Salad", "dessert", "Fresh tropical fruit salad", 2500, 700, 5, "vegetarian"},
		{"Supermalt", "drink", "Malt drink (bottle)", 800, 300, 1, ""},
		{"Chapman", "drink", "House cocktail (non-alcoholic)", 1500, 400, 5, ""},
		{"Star Lager", "drink", "Star beer (bottle)", 1000, 400, 1, ""},
		{"Guinness Stout", "drink", "Guinness Foreign Stout (bottle)", 1200, 500, 1, ""},
		{"H2O Water", "drink", "Bottled water 50cl", 300, 100, 1, ""},
		{"Coca-Cola", "drink", "Coke bottle", 500, 200, 1, ""},
		{"Fresh Orange Juice", "drink", "Freshly squeezed orange juice", 1500, 500, 3, ""},
		{"Café Latte", "drink", "Espresso with steamed milk", 1200, 300, 3, ""},
	}
	for i, m := range rMenu {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_menu (id, property_id, name, category, description, price, cost_price, prep_time_min, is_available, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			fmt.Sprintf("rm-%03d", i+1), pID, m.name, m.cat, m.desc, m.price, m.cost, m.prep, m.tags)
	}

	// ── Restaurant Ingredients ──
	ingredients := []struct{ name, cat, unit string; stock, minStock, cost float64; supplier string }{
		{"Basmati Rice", "grains", "kg", 50, 10, 1200, "Lagos Food Co"},
		{"Chicken (whole)", "protein", "kg", 30, 8, 3500, "Chicken Republic Supply"},
		{"Goat Meat", "protein", "kg", 15, 5, 5500, "Mile 12 Market"},
		{"Tilapia Fish", "protein", "kg", 20, 5, 4000, "Epe Fish Market"},
		{"Vegetable Oil", "cooking", "litre", 25, 10, 800, "Kings Oil"},
		{"Tomatoes", "produce", "kg", 15, 5, 600, "Lagos Farm Market"},
		{"Onions", "produce", "kg", 10, 3, 400, "Lagos Farm Market"},
		{"Peppers (Scotch Bonnet)", "produce", "kg", 8, 3, 700, "Lagos Farm Market"},
		{"Coca-Cola", "beverages", "bottles", 48, 12, 200, "Coca-Cola Nigeria"},
		{"Star Lager", "beverages", "bottles", 72, 24, 400, "Nigerian Breweries"},
		{"Guinness Stout", "beverages", "bottles", 36, 12, 500, "Nigerian Breweries"},
		{"Supermalt", "beverages", "bottles", 24, 8, 300, "Nigerian Breweries"},
		{"Ice", "cooking", "bags", 20, 5, 200, "Icy Express"},
		{"Garri", "grains", "kg", 40, 10, 500, "Ogun State Supply"},
		{"Plantain", "produce", "kg", 25, 8, 600, "Oshodi Market"},
	}
	for i, ing := range ingredients {
		ingrID := fmt.Sprintf("ing-%03d", i+1)
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_ingredients (id, property_id, name, category, unit, current_stock, min_stock, cost_per_unit, supplier) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ingrID, pID, ing.name, ing.cat, ing.unit, ing.stock, ing.minStock, ing.cost, ing.supplier)
	}

	// ── Restaurant Orders (8 demo orders across today) ──
	rOrders := []struct{ id, num, tableID, typ, status, staff string; items []struct{ name, cat string; qty, price float64 }; guest string }{
		{"ro-001", "ORD-120801", "rt-03", "dine_in", "completed", "Waiter Chidi",
			[]struct{ name, cat string; qty, price float64 }{
				{"Jollof Rice", "main", 2, 4500}, {"Star Lager", "drink", 2, 1000}, {"Pepper Soup", "starter", 1, 3000}},
			"Mr. Obi"},
		{"ro-002", "ORD-120901", "rt-06", "dine_in", "preparing", "Waiter Amara",
			[]struct{ name, cat string; qty, price float64 }{
				{"Grilled Tilapia", "main", 1, 7500}, {"Fried Rice", "main", 1, 4500}, {"Guinness Stout", "drink", 3, 1200}, {"Pancakes", "dessert", 2, 2000}},
			"Mrs. Adeyemi"},
		{"ro-003", "ORD-120902", "rt-10", "dine_in", "new", "Waiter Emeka",
			[]struct{ name, cat string; qty, price float64 }{
				{"Egusi Soup & Pounded Yam", "main", 2, 5000}, {"Chapman", "drink", 2, 1500}},
			"Dr. Okoro"},
		{"ro-004", "ORD-120903", "rt-01", "dine_in", "served", "Waiter Chidi",
			[]struct{ name, cat string; qty, price float64 }{
				{"Chicken Shawarma", "main", 3, 3000}, {"Coca-Cola", "drink", 3, 500}},
			"Team from MTN"},
		{"ro-005", "ORD-120904", "", "delivery", "new", "Kitchen",
			[]struct{ name, cat string; qty, price float64 }{
				{"Jollof Rice", "main", 5, 4500}, {"Suya Skewers", "main", 3, 3500}},
			"Lekki Office"},
		{"ro-006", "ORD-120905", "rt-13", "dine_in", "preparing", "Waiter Amara",
			[]struct{ name, cat string; qty, price float64 }{
				{"Banga Soup & Starch", "main", 2, 6000}, {"Suya Skewers", "main", 2, 3500}, {"Star Lager", "drink", 4, 1000}, {"Chin Chin Sundae", "dessert", 2, 3000}},
			"Chief Okafor"},
		{"ro-007", "ORD-120906", "rt-04", "dine_in", "completed", "Waiter Emeka",
			[]struct{ name, cat string; qty, price float64 }{
				{"Spring Rolls", "starter", 1, 2000}, {"Café Latte", "drink", 2, 1200}},
			"Visitor"},
		{"ro-008", "ORD-120907", "", "takeout", "new", "Kitchen",
			[]struct{ name, cat string; qty, price float64 }{
				{"Ofada Rice", "main", 2, 5500}, {"Gizdodo", "starter", 1, 3500}},
			"Pickup - Mr. Tunde"},
	}
	for _, o := range rOrders {
		sub := 0.0
		for _, it := range o.items { sub += it.qty * it.price }
		tax := math.Round(sub*0.075*100) / 100
		total := sub + tax
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_orders (id, property_id, order_number, table_id, type, status, staff_name, guest_name, subtotal, tax_amount, total, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			o.id, pID, o.num, o.tableID, o.typ, o.status, o.staff, o.guest, sub, tax, total)
		for j, it := range o.items {
			amt := it.qty * it.price
			_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_order_items (id, order_id, menu_item_id, name, category, quantity, unit_price, amount, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'served')`,
				fmt.Sprintf("%s-i%d", o.id, j+1), o.id, fmt.Sprintf("rm-%03d", j+1), it.name, it.cat, it.qty, it.price, amt)
		}
	}

	// ── Restaurant Guests ──
	rGuests := []struct{ id, name, phone, tier, favDish, dietary string; visits int; spend float64 }{
		{"rg-001", "Chief Okafor", "+234-803-123-4567", "vip", "Banga Soup", "Low salt", 28, 425000},
		{"rg-002", "Mrs. Adeyemi", "+234-805-234-5678", "gold", "Grilled Tilapia", "", 15, 187000},
		{"rg-003", "Mr. Obi", "+234-802-345-6789", "silver", "Jollof Rice", "", 8, 65000},
		{"rg-004", "Dr. Okoro", "+234-809-456-7890", "regular", "Egusi Soup", "Pescatarian", 5, 38000},
		{"rg-005", "Fatima Yusuf", "+234-810-567-8901", "regular", "Suya Skewers", "Halal", 3, 22000},
	}
	for _, g := range rGuests {
		_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_guests (id, property_id, name, phone, tier, favorite_dish, dietary_notes, total_visits, total_spend, last_visit) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			g.id, pID, g.name, g.phone, g.tier, g.favDish, g.dietary, g.visits, g.spend)
	}

	// ── Restaurant Deliveries ──
	_, _ = d.ExecContext(ctx, `INSERT OR IGNORE INTO restaurant_deliveries (id, property_id, order_id, source, customer_name, customer_phone, address, total, delivery_fee, status, eta_minutes) VALUES (?, ?, 'ro-005', 'whatsapp', 'Lekki Office', '+234-801-111-2222', '15 Admiralty Way, Lekki Phase 1', 33075, 1500, 'pending', 35)`, uuid.New().String(), pID)

	log.Println("[sqlite] Demo seed data complete: 1 property, 50 rooms, 20 guests, 25 reservations (10 in-house), 10 stays, 6 folios, 6 POS orders, 13 housekeeping tasks, 5 loyalty tiers, 4 venues, 3 events, 6 pricing rules, 6 tax rates, 11 payment methods, 14 departments, 8 numbering sequences, 33 system settings, 5 shifts, 8 leave types, 10 holidays, 8 training courses, 15 contracts, 2 performance cycles, 10 profiles, 105 attendance records, 8 leave requests, 15 training enrollments, 8 traces, 6 service requests, 5 interactions, 3 assignment rules, 4 room assignments, 18 OTA bookings, 16 restaurant tables, 24 menu items, 15 ingredients, 8 restaurant orders, 5 restaurant guests, 1 delivery")
	return nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}
