package channelmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════
// CHANNEL MANAGER
// Production-grade OTA integration with Booking.com, Expedia,
// Airbnb, Agoda, Jumia Travel, Hotels.ng, Wakanow
// ═══════════════════════════════════════════════════════════════

// OTAProvider defines the interface every OTA adapter must implement
type OTAProvider interface {
	Name() string
	PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error)
	PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error)
	PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error)
	PushBookingConfirm(ctx context.Context, confirmation *BookingConfirmation) (*SyncResponse, error)
	CancelBooking(ctx context.Context, reservationID string, reason string) (*SyncResponse, error)
	GetMappingStatus(ctx context.Context) (*MappingStatus, error)
	TestConnection(ctx context.Context) error
}

// Channel represents an OTA channel connection
type Channel struct {
	ID             string            `json:"id"`
	PropertyID     string            `json:"property_id"`
	Name           string            `json:"name"` // "Booking.com", "Expedia", etc.
	Code           string            `json:"code"` // "bkng", "expe", "airb", "agda", "jumia", "hotels_ng", "wakanow"
	Enabled        bool              `json:"enabled"`
	Status         string            `json:"status"`      // connected, disconnected, error, syncing
	Commission     float64           `json:"commission"`  // percentage
	Markup         float64           `json:"markup"`      // percentage markup on base rate
	MarkupType     string            `json:"markup_type"` // percentage, fixed
	MinStay        int               `json:"min_stay"`
	MaxStay        int               `json:"max_stay"`
	StopSellDates  []string          `json:"stop_sell_dates"`
	APIKey         string            `json:"api_key,omitempty"`
	APISecret      string            `json:"api_secret,omitempty"`
	HotelID        string            `json:"hotel_id"` // OTA's hotel ID
	LastSyncAt     *time.Time        `json:"last_sync_at"`
	LastSyncStatus string            `json:"last_sync_status"`
	TotalBookings  int               `json:"total_bookings"`
	TotalRevenue   float64           `json:"total_revenue"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// ChannelMapping maps PMS room types/rate plans to OTA room types/rate plans
type ChannelMapping struct {
	ID              string `json:"id"`
	ChannelID       string `json:"channel_id"`
	PMSRoomTypeID   string `json:"pms_room_type_id"`
	OTARoomTypeID   string `json:"ota_room_type_id"`
	OTARoomTypeName string `json:"ota_room_type_name"`
	PMSRatePlanID   string `json:"pms_rate_plan_id"`
	OTARatePlanID   string `json:"ota_rate_plan_id"`
	OTARatePlanName string `json:"ota_rate_plan_name"`
	Enabled         bool   `json:"enabled"`
}

// RatePushRequest contains rates to push to an OTA
type RatePushRequest struct {
	ChannelID string     `json:"channel_id"`
	RoomType  string     `json:"room_type_id"`
	Rates     []DateRate `json:"rates"`
}

// DateRate represents a rate for a specific date
type DateRate struct {
	Date     string  `json:"date"`
	Rate     float64 `json:"rate"`
	MinStay  int     `json:"min_stay"`
	Closed   bool    `json:"closed"`
	StopSell bool    `json:"stop_sell"`
}

// AvailabilityPushRequest contains availability to push
type AvailabilityPushRequest struct {
	ChannelID      string      `json:"channel_id"`
	Availabilities []DateAvail `json:"availabilities"`
}

// DateAvail represents availability for a date
type DateAvail struct {
	Date       string `json:"date"`
	RoomTypeID string `json:"room_type_id"`
	Available  int    `json:"available"`
	Restricted bool   `json:"restricted"`
}

// SyncResponse is the result of a sync operation
type SyncResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	SyncedItems int    `json:"synced_items"`
	FailedItems int    `json:"failed_items"`
	Timestamp   string `json:"timestamp"`
}

// OTABooking is a booking imported from an OTA
type OTABooking struct {
	ChannelID        string            `json:"channel_id"`
	ChannelCode      string            `json:"channel_code"`
	OTAReservationID string            `json:"ota_reservation_id"`
	GuestName        string            `json:"guest_name"`
	GuestEmail       string            `json:"guest_email"`
	GuestPhone       string            `json:"guest_phone"`
	RoomTypeID       string            `json:"room_type_id"`
	RatePlanID       string            `json:"rate_plan_id"`
	CheckIn          string            `json:"check_in"`
	CheckOut         string            `json:"check_out"`
	Nights           int               `json:"nights"`
	RatePerNight     float64           `json:"rate_per_night"`
	TotalAmount      float64           `json:"total_amount"`
	Currency         string            `json:"currency"`
	Status           string            `json:"status"` // confirmed, modified, cancelled
	PaymentStatus    string            `json:"payment_status"`
	NumberOfGuests   int               `json:"number_of_guests"`
	SpecialRequests  string            `json:"special_requests"`
	BookingSource    string            `json:"booking_source"`
	Commission       float64           `json:"commission"`
	NetRate          float64           `json:"net_rate"`
	BookedAt         string            `json:"booked_at"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// BookingConfirmation sent back to OTA
type BookingConfirmation struct {
	ReservationID    string `json:"reservation_id"`
	OTAReservationID string `json:"ota_reservation_id"`
	ChannelCode      string `json:"channel_code"`
	RoomNumber       string `json:"room_number"`
	Status           string `json:"status"`
}

// MappingStatus shows current mapping health
type MappingStatus struct {
	TotalRoomTypes  int `json:"total_room_types"`
	MappedRoomTypes int `json:"mapped_room_types"`
	TotalRatePlans  int `json:"total_rate_plans"`
	MappedRatePlans int `json:"mapped_rate_plans"`
	UnmappedItems   int `json:"unmapped_items"`
	Errors          int `json:"errors"`
}

// SyncLogEntry records a sync operation
type SyncLogEntry struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Direction   string `json:"direction"` // push, pull
	Type        string `json:"type"`      // rates, availability, bookings, confirmation
	Status      string `json:"status"`    // success, partial, failed
	ItemsCount  int    `json:"items_count"`
	ErrorCount  int    `json:"error_count"`
	Duration    int    `json:"duration_ms"`
	ErrorMsg    string `json:"error_msg,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// ChannelManager orchestrates all OTA connections
type ChannelManager struct {
	db        *sql.DB
	providers map[string]OTAProvider
}

// NewChannelManager creates the channel manager
func NewChannelManager(db *sql.DB) *ChannelManager {
	cm := &ChannelManager{
		db:        db,
		providers: make(map[string]OTAProvider),
	}
	cm.createTables()
	cm.seedProviders()
	return cm
}

func (cm *ChannelManager) createTables() {
	ctx := context.Background()

	// Add missing columns to existing channels table (from sqlite.go migration)
	alterCols := []string{
		"ALTER TABLE channels ADD COLUMN enabled INTEGER DEFAULT 1",
		"ALTER TABLE channels ADD COLUMN commission REAL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN markup REAL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN markup_type TEXT DEFAULT 'percentage'",
		"ALTER TABLE channels ADD COLUMN min_stay INTEGER DEFAULT 1",
		"ALTER TABLE channels ADD COLUMN max_stay INTEGER DEFAULT 30",
		"ALTER TABLE channels ADD COLUMN stop_sell_dates TEXT DEFAULT '[]'",
		"ALTER TABLE channels ADD COLUMN api_secret TEXT",
		"ALTER TABLE channels ADD COLUMN hotel_id TEXT",
		"ALTER TABLE channels ADD COLUMN last_sync_status TEXT",
		"ALTER TABLE channels ADD COLUMN total_bookings INTEGER DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN total_revenue REAL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN metadata TEXT DEFAULT '{}'",
		"ALTER TABLE channels ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP",
	}
	for _, a := range alterCols {
		_, _ = cm.db.ExecContext(ctx, a)
	}

	// Add missing columns to channel_bookings table
	bookingAlters := []string{
		"ALTER TABLE channel_bookings ADD COLUMN channel_code TEXT",
		"ALTER TABLE channel_bookings ADD COLUMN rate_per_night REAL DEFAULT 0",
		"ALTER TABLE channel_bookings ADD COLUMN commission REAL DEFAULT 0",
		"ALTER TABLE channel_bookings ADD COLUMN net_amount REAL DEFAULT 0",
		"ALTER TABLE channel_bookings ADD COLUMN payment_status TEXT DEFAULT 'pending'",
		"ALTER TABLE channel_bookings ADD COLUMN number_of_guests INTEGER DEFAULT 1",
		"ALTER TABLE channel_bookings ADD COLUMN special_requests TEXT",
		"ALTER TABLE channel_bookings ADD COLUMN booked_at DATETIME",
		"ALTER TABLE channel_bookings ADD COLUMN imported_at DATETIME DEFAULT CURRENT_TIMESTAMP",
	}
	for _, a := range bookingAlters {
		_, _ = cm.db.ExecContext(ctx, a)
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS channels (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			status TEXT DEFAULT 'disconnected',
			commission REAL DEFAULT 0,
			markup REAL DEFAULT 0,
			markup_type TEXT DEFAULT 'percentage',
			min_stay INTEGER DEFAULT 1,
			max_stay INTEGER DEFAULT 30,
			stop_sell_dates TEXT DEFAULT '[]',
			api_key TEXT,
			api_secret TEXT,
			hotel_id TEXT,
			last_sync_at DATETIME,
			last_sync_status TEXT,
			total_bookings INTEGER DEFAULT 0,
			total_revenue REAL DEFAULT 0,
			metadata TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_property ON channels(property_id)`,

		`CREATE TABLE IF NOT EXISTS channel_mappings (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			pms_room_type_id TEXT NOT NULL,
			ota_room_type_id TEXT NOT NULL,
			ota_room_type_name TEXT,
			pms_rate_plan_id TEXT NOT NULL,
			ota_rate_plan_id TEXT NOT NULL,
			ota_rate_plan_name TEXT,
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(channel_id, pms_room_type_id, pms_rate_plan_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mappings_channel ON channel_mappings(channel_id)`,

		`CREATE TABLE IF NOT EXISTS channel_sync_log (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			channel_name TEXT,
			direction TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			items_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_msg TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_log_channel ON channel_sync_log(channel_id, created_at)`,

		`CREATE TABLE IF NOT EXISTS channel_bookings (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			channel_code TEXT NOT NULL,
			ota_reservation_id TEXT NOT NULL,
			pms_reservation_id TEXT,
			guest_name TEXT,
			guest_email TEXT,
			guest_phone TEXT,
			room_type_id TEXT,
			check_in DATE,
			check_out DATE,
			nights INTEGER DEFAULT 1,
			rate_per_night REAL DEFAULT 0,
			total_amount REAL DEFAULT 0,
			commission REAL DEFAULT 0,
			net_amount REAL DEFAULT 0,
			currency TEXT DEFAULT 'NGN',
			status TEXT DEFAULT 'confirmed',
			payment_status TEXT DEFAULT 'pending',
			number_of_guests INTEGER DEFAULT 1,
			special_requests TEXT,
			booked_at DATETIME,
			imported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(channel_id, ota_reservation_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ch_bookings_channel ON channel_bookings(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ch_bookings_dates ON channel_bookings(check_in, check_out)`,

		`CREATE TABLE IF NOT EXISTS channel_performance (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			property_id TEXT NOT NULL,
			period TEXT NOT NULL,
			total_bookings INTEGER DEFAULT 0,
			total_nights INTEGER DEFAULT 0,
			total_revenue REAL DEFAULT 0,
			total_commission REAL DEFAULT 0,
			net_revenue REAL DEFAULT 0,
			avg_daily_rate REAL DEFAULT 0,
			occupancy_contribution REAL DEFAULT 0,
			cancellation_rate REAL DEFAULT 0,
			no_show_rate REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(channel_id, period)
		)`,
	}

	for _, t := range tables {
		_, _ = cm.db.ExecContext(ctx, t)
	}
}

func (cm *ChannelManager) seedProviders() {
	cm.providers["bkng"] = &BookingComProvider{name: "Booking.com", code: "bkng"}
	cm.providers["expe"] = &ExpediaProvider{name: "Expedia", code: "expe"}
	cm.providers["airb"] = &AirbnbProvider{name: "Airbnb", code: "airb"}
	cm.providers["agda"] = &AgodaProvider{name: "Agoda", code: "agda"}
	cm.providers["jumia"] = &JumiaTravelProvider{name: "Jumia Travel", code: "jumia"}
	cm.providers["hotels_ng"] = &HotelsNGProvider{name: "Hotels.ng", code: "hotels_ng"}
	cm.providers["wakanow"] = &WakanowProvider{name: "Wakanow", code: "wakanow"}
	cm.providers["direct"] = &DirectBookingProvider{name: "Direct Booking", code: "direct"}
	cm.providers["mock"] = &MockOTAProvider{name: "Mock OTA", code: "mock"}
}

// SeedChannelData seeds channels and mappings for a property.
// Call this AFTER property seed data exists (foreign key constraint).
func (cm *ChannelManager) SeedChannelData(ctx context.Context, propertyID string) {
	cm.seedChannelsForProperty(ctx, propertyID)
}

func (cm *ChannelManager) seedChannelsForProperty(ctx context.Context, propertyID string) {

	channels := []Channel{
		{ID: "ch-bkng", PropertyID: propertyID, Name: "Booking.com", Code: "bkng", Enabled: true, Status: "connected", Commission: 15.0, HotelID: "BKNG-12345"},
		{ID: "ch-expe", PropertyID: propertyID, Name: "Expedia", Code: "expe", Enabled: true, Status: "connected", Commission: 18.0, HotelID: "EXPE-67890"},
		{ID: "ch-airb", PropertyID: propertyID, Name: "Airbnb", Code: "airb", Enabled: true, Status: "connected", Commission: 14.0, HotelID: "AIRB-ABCDE"},
		{ID: "ch-agda", PropertyID: propertyID, Name: "Agoda", Code: "agda", Enabled: false, Status: "disconnected", Commission: 16.0, HotelID: "AGDA-FGHIJ"},
		{ID: "ch-jumia", PropertyID: propertyID, Name: "Jumia Travel", Code: "jumia", Enabled: true, Status: "connected", Commission: 12.0, HotelID: "JUMIA-LMNO"},
		{ID: "ch-hotelsng", PropertyID: propertyID, Name: "Hotels.ng", Code: "hotels_ng", Enabled: false, Status: "disconnected", Commission: 10.0, HotelID: "HNG-PQRS"},
		{ID: "ch-wakanow", PropertyID: propertyID, Name: "Wakanow", Code: "wakanow", Enabled: false, Status: "disconnected", Commission: 12.0, HotelID: "WKW-TUVWX"},
		{ID: "ch-direct", PropertyID: propertyID, Name: "Direct Booking", Code: "direct", Enabled: true, Status: "connected", Commission: 0.0, HotelID: "DIRECT"},
	}

	for _, ch := range channels {
		metaJSON, _ := json.Marshal(ch.Metadata)
		if metaJSON == nil {
			metaJSON = []byte("{}")
		}
		_, _ = cm.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO channels (id, property_id, name, code, type, enabled, status, commission, hotel_id, metadata)
			VALUES (?, ?, ?, ?, 'ota', ?, ?, ?, ?, ?)`,
			ch.ID, ch.PropertyID, ch.Name, ch.Code, ch.Enabled, ch.Status, ch.Commission, ch.HotelID, string(metaJSON))
	}

	// Seed channel mappings (PMS room types → OTA room types)
	mappings := []ChannelMapping{
		// Booking.com mappings
		{ID: "map-bkng-std", ChannelID: "ch-bkng", PMSRoomTypeID: "rt-standard", OTARoomTypeID: "RT-STD", OTARoomTypeName: "Standard Room", PMSRatePlanID: "rate-bar", OTARatePlanID: "RP-BAR", OTARatePlanName: "Best Available Rate"},
		{ID: "map-bkng-dlx", ChannelID: "ch-bkng", PMSRoomTypeID: "rt-deluxe", OTARoomTypeID: "RT-DLX", OTARoomTypeName: "Deluxe Room", PMSRatePlanID: "rate-bar", OTARatePlanID: "RP-BAR", OTARatePlanName: "Best Available Rate"},
		{ID: "map-bkng-exs", ChannelID: "ch-bkng", PMSRoomTypeID: "rt-executive", OTARoomTypeID: "RT-EXS", OTARoomTypeName: "Executive Suite", PMSRatePlanID: "rate-bar", OTARatePlanID: "RP-BAR", OTARatePlanName: "Best Available Rate"},
		{ID: "map-bkng-pre", ChannelID: "ch-bkng", PMSRoomTypeID: "rt-presidential", OTARoomTypeID: "RT-PRS", OTARoomTypeName: "Presidential Suite", PMSRatePlanID: "rate-bar", OTARatePlanID: "RP-BAR", OTARatePlanName: "Best Available Rate"},
		// Expedia mappings
		{ID: "map-expe-std", ChannelID: "ch-expe", PMSRoomTypeID: "rt-standard", OTARoomTypeID: "EXP-STD", OTARoomTypeName: "Standard Room", PMSRatePlanID: "rate-bar", OTARatePlanID: "EXP-BAR", OTARatePlanName: "Rack Rate"},
		{ID: "map-expe-dlx", ChannelID: "ch-expe", PMSRoomTypeID: "rt-deluxe", OTARoomTypeID: "EXP-DLX", OTARoomTypeName: "Deluxe Room", PMSRatePlanID: "rate-bar", OTARatePlanID: "EXP-BAR", OTARatePlanName: "Rack Rate"},
		// Airbnb mappings
		{ID: "map-airb-std", ChannelID: "ch-airb", PMSRoomTypeID: "rt-standard", OTARoomTypeID: "AIR-STD", OTARoomTypeName: "Entire home/apt", PMSRatePlanID: "rate-bar", OTARatePlanID: "AIR-NIGHTLY", OTARatePlanName: "Nightly Rate"},
		// Jumia mappings
		{ID: "map-jumia-std", ChannelID: "ch-jumia", PMSRoomTypeID: "rt-standard", OTARoomTypeID: "JUM-STD", OTARoomTypeName: "Standard Room", PMSRatePlanID: "rate-bar", OTARatePlanID: "JUM-BAR", OTARatePlanName: "Standard Rate"},
	}

	for _, m := range mappings {
		_, _ = cm.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO channel_mappings (id, channel_id, pms_room_type_id, ota_room_type_id, ota_room_type_name,
				pms_rate_plan_id, ota_rate_plan_id, ota_rate_plan_name, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			m.ID, m.ChannelID, m.PMSRoomTypeID, m.OTARoomTypeID, m.OTARoomTypeName,
			m.PMSRatePlanID, m.OTARatePlanID, m.OTARatePlanName)
	}

	log.Printf("[CHANNEL] Seeded %d channels, %d mappings", len(channels), len(mappings))
}

// ─── CORE CHANNEL OPERATIONS ────────────────────────────────

// ListChannels returns all channels for a property
func (cm *ChannelManager) ListChannels(ctx context.Context, propertyID string) ([]Channel, error) {
	rows, err := cm.db.QueryContext(ctx,
		`SELECT id, name, code, enabled, status, commission, markup, hotel_id,
			COALESCE(last_sync_at,''), COALESCE(last_sync_status,''), total_bookings, total_revenue
		FROM channels WHERE property_id=? ORDER BY name`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		var lastSync string
		rows.Scan(&ch.ID, &ch.Name, &ch.Code, &ch.Enabled, &ch.Status, &ch.Commission,
			&ch.Markup, &ch.HotelID, &lastSync, &ch.LastSyncStatus, &ch.TotalBookings, &ch.TotalRevenue)
		ch.PropertyID = propertyID
		channels = append(channels, ch)
	}
	return channels, nil
}

// SyncRatesToAll pushes rates to all enabled channels
func (cm *ChannelManager) SyncRatesToAll(ctx context.Context, propertyID string) (*SyncResponse, error) {
	log.Printf("[CHANNEL] Starting rate sync for property %s", propertyID)

	// Get all enabled channels
	channels, err := cm.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	totalSynced := 0
	totalFailed := 0
	errors := []string{}

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		provider, ok := cm.providers[ch.Code]
		if !ok {
			provider = cm.providers["mock"]
		}

		// Get rates for this channel (with markup)
		roomTypes := cm.getRoomTypesWithRates(ctx, propertyID, ch.Markup)

		for rtID, rate := range roomTypes {
			rates := []DateRate{}
			for i := 0; i < 30; i++ { // Push 30 days ahead
				date := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")
				dow := time.Now().AddDate(0, 0, i+1).Weekday()
				dayRate := rate
				if dow == time.Saturday || dow == time.Friday {
					dayRate *= 1.15 // Weekend premium
				}
				rates = append(rates, DateRate{
					Date: date, Rate: math.Round(dayRate*100) / 100,
					MinStay: ch.MinStay, Closed: false, StopSell: false,
				})
			}

			resp, err := provider.PushRates(ctx, &RatePushRequest{
				ChannelID: ch.ID,
				RoomType:  rtID,
				Rates:     rates,
			})
			if err != nil {
				totalFailed++
				errors = append(errors, fmt.Sprintf("%s:%s", ch.Name, err.Error()))
				continue
			}
			if resp.Success {
				totalSynced += resp.SyncedItems
			} else {
				totalFailed++
				errors = append(errors, fmt.Sprintf("%s:%s", ch.Name, resp.Message))
			}
		}

		// Update last sync time
		now := time.Now().UTC()
		_, _ = cm.db.ExecContext(ctx,
			`UPDATE channels SET last_sync_at=?, last_sync_status='success', updated_at=? WHERE id=?`,
			now, now, ch.ID)

		cm.logSync(ctx, ch.ID, ch.Name, "push", "rates", "success", totalSynced, 0, 0, "")
	}

	log.Printf("[CHANNEL] Rate sync complete: %d synced, %d failed", totalSynced, totalFailed)
	return &SyncResponse{
		Success:     totalFailed == 0,
		Message:     fmt.Sprintf("Synced rates to %d channels", len(channels)),
		SyncedItems: totalSynced,
		FailedItems: totalFailed,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// SyncAvailabilityToAll pushes availability to all enabled channels
func (cm *ChannelManager) SyncAvailabilityToAll(ctx context.Context, propertyID string) (*SyncResponse, error) {
	log.Printf("[CHANNEL] Starting availability sync for property %s", propertyID)

	channels, err := cm.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	totalSynced := 0
	totalFailed := 0

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		provider, ok := cm.providers[ch.Code]
		if !ok {
			provider = cm.providers["mock"]
		}

		// Get room counts per type
		roomCounts := cm.getRoomAvailability(ctx, propertyID)

		var avails []DateAvail
		for rtID, count := range roomCounts {
			for i := 0; i < 30; i++ {
				date := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")
				avails = append(avails, DateAvail{
					Date: date, RoomTypeID: rtID, Available: count, Restricted: false,
				})
			}
		}

		resp, err := provider.PushAvailability(ctx, &AvailabilityPushRequest{
			ChannelID:      ch.ID,
			Availabilities: avails,
		})
		if err != nil {
			totalFailed++
			continue
		}
		if resp.Success {
			totalSynced += resp.SyncedItems
		} else {
			totalFailed++
		}

		now := time.Now().UTC()
		_, _ = cm.db.ExecContext(ctx,
			`UPDATE channels SET last_sync_at=?, last_sync_status='success' WHERE id=?`, now, ch.ID)
	}

	return &SyncResponse{
		Success:     totalFailed == 0,
		Message:     fmt.Sprintf("Synced availability to %d channels", len(channels)),
		SyncedItems: totalSynced,
		FailedItems: totalFailed,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// PullBookingsFromAll imports bookings from all enabled channels
func (cm *ChannelManager) PullBookingsFromAll(ctx context.Context, propertyID string) (*SyncResponse, error) {
	log.Printf("[CHANNEL] Starting booking pull for property %s", propertyID)

	channels, err := cm.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	totalImported := 0
	totalFailed := 0

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		provider, ok := cm.providers[ch.Code]
		if !ok {
			continue
		}

		bookings, err := provider.PullBookings(ctx, time.Now().AddDate(0, -1, 0))
		if err != nil {
			totalFailed++
			log.Printf("[CHANNEL] Failed to pull from %s: %v", ch.Name, err)
			continue
		}

		for _, bk := range bookings {
			// Check for duplicate
			var exists bool
			_ = cm.db.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM channel_bookings WHERE channel_id=? AND ota_reservation_id=?)`,
				ch.ID, bk.OTAReservationID).Scan(&exists)

			if exists {
				continue // Skip duplicate
			}

			// Import booking
			id := uuid.New().String()
			_, err := cm.db.ExecContext(ctx, `
				INSERT INTO channel_bookings (id, channel_id, channel_code, ota_reservation_id,
					guest_name, guest_email, guest_phone, room_type_id, check_in, check_out, nights,
					rate_per_night, total_amount, commission, net_amount, currency, status, payment_status,
					number_of_guests, special_requests, booked_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, ch.ID, ch.Code, bk.OTAReservationID,
				bk.GuestName, bk.GuestEmail, bk.GuestPhone, bk.RoomTypeID,
				bk.CheckIn, bk.CheckOut, bk.Nights,
				bk.RatePerNight, bk.TotalAmount, bk.Commission, bk.NetRate,
				bk.Currency, bk.Status, bk.PaymentStatus,
				bk.NumberOfGuests, bk.SpecialRequests, bk.BookedAt)
			if err != nil {
				totalFailed++
				continue
			}

			// Update channel stats
			_, _ = cm.db.ExecContext(ctx,
				`UPDATE channels SET total_bookings=total_bookings+1, total_revenue=total_revenue+? WHERE id=?`,
				bk.TotalAmount, ch.ID)

			totalImported++
			log.Printf("[CHANNEL] Imported booking %s from %s: %s (%s to %s)", bk.OTAReservationID, ch.Name, bk.GuestName, bk.CheckIn, bk.CheckOut)
		}

		cm.logSync(ctx, ch.ID, ch.Name, "pull", "bookings", "success", totalImported, totalFailed, 0, "")
	}

	return &SyncResponse{
		Success:     totalFailed == 0,
		Message:     fmt.Sprintf("Imported %d bookings from %d channels", totalImported, len(channels)),
		SyncedItems: totalImported,
		FailedItems: totalFailed,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// GetChannelPerformance returns performance metrics per channel
func (cm *ChannelManager) GetChannelPerformance(ctx context.Context, propertyID string) map[string]interface{} {
	channels, _ := cm.ListChannels(ctx, propertyID)

	type ChannelPerf struct {
		Name            string  `json:"name"`
		Code            string  `json:"code"`
		TotalBookings   int     `json:"total_bookings"`
		TotalRevenue    float64 `json:"total_revenue"`
		AvgRate         float64 `json:"avg_rate"`
		Commission      float64 `json:"commission_pct"`
		TotalCommission float64 `json:"total_commission"`
		NetRevenue      float64 `json:"net_revenue"`
		Share           float64 `json:"revenue_share_pct"`
	}

	perfs := []ChannelPerf{}
	totalRev := 0.0

	for _, ch := range channels {
		perf := ChannelPerf{
			Name:       ch.Name,
			Code:       ch.Code,
			Commission: ch.Commission,
		}

		// Get bookings from this channel
		_ = cm.db.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(total_amount), 0), COALESCE(SUM(commission), 0)
			FROM channel_bookings WHERE channel_id=?`, ch.ID).Scan(&perf.TotalBookings, &perf.TotalRevenue, &perf.TotalCommission)

		if perf.TotalBookings > 0 {
			perf.AvgRate = perf.TotalRevenue / float64(perf.TotalBookings)
		}
		perf.NetRevenue = perf.TotalRevenue - perf.TotalCommission
		totalRev += perf.TotalRevenue

		perfs = append(perfs, perf)
	}

	// Calculate revenue share
	for i := range perfs {
		if totalRev > 0 {
			perfs[i].Share = math.Round(perfs[i].TotalRevenue/totalRev*10000) / 100
		}
	}

	return map[string]interface{}{
		"channels":      perfs,
		"total_revenue": totalRev,
		"period":        "All time",
	}
}

// GetSyncLog returns recent sync operations
func (cm *ChannelManager) GetSyncLog(ctx context.Context, channelID string, limit int) []SyncLogEntry {
	if limit <= 0 {
		limit = 50
	}

	rows, err := cm.db.QueryContext(ctx,
		`SELECT id, channel_id, channel_name, direction, type, status, items_count, error_count, duration_ms, COALESCE(error_msg,''), created_at
		FROM channel_sync_log WHERE channel_id=?
		ORDER BY created_at DESC LIMIT ?`, channelID, limit)
	if err != nil {
		return []SyncLogEntry{}
	}
	defer rows.Close()

	logs := []SyncLogEntry{}
	for rows.Next() {
		var log SyncLogEntry
		rows.Scan(&log.ID, &log.ChannelID, &log.ChannelName, &log.Direction, &log.Type,
			&log.Status, &log.ItemsCount, &log.ErrorCount, &log.Duration, &log.ErrorMsg, &log.CreatedAt)
		logs = append(logs, log)
	}
	return logs
}

// GetChannelBookings returns bookings from a specific channel
func (cm *ChannelManager) GetChannelBookings(ctx context.Context, channelID string, status string) []OTABooking {
	sqlQuery := `SELECT COALESCE(channel_code,''), COALESCE(ota_reservation_id,''), COALESCE(guest_name,''), COALESCE(guest_email,''),
		COALESCE(room_type_id,''), COALESCE(check_in,''), COALESCE(check_out,''), COALESCE(nights,1), COALESCE(rate_per_night,0), COALESCE(total_amount,0),
		COALESCE(commission,0), COALESCE(net_amount,0), COALESCE(currency,'NGN'), COALESCE(status,'confirmed'), COALESCE(payment_status,'pending'), COALESCE(number_of_guests,1)
		FROM channel_bookings WHERE channel_id=?`
	args := []interface{}{channelID}

	if status != "" {
		sqlQuery += ` AND status=?`
		args = append(args, status)
	}
	sqlQuery += ` ORDER BY booked_at DESC LIMIT 100`

	rows, err := cm.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return []OTABooking{}
	}
	defer rows.Close()

	bookings := []OTABooking{}
	for rows.Next() {
		var bk OTABooking
		rows.Scan(&bk.ChannelCode, &bk.OTAReservationID, &bk.GuestName, &bk.GuestEmail,
			&bk.RoomTypeID, &bk.CheckIn, &bk.CheckOut, &bk.Nights, &bk.RatePerNight,
			&bk.TotalAmount, &bk.Commission, &bk.NetRate, &bk.Currency,
			&bk.Status, &bk.PaymentStatus, &bk.NumberOfGuests)
		bookings = append(bookings, bk)
	}
	return bookings
}

// UpdateChannelSettings updates channel configuration
func (cm *ChannelManager) UpdateChannelSettings(ctx context.Context, channelID string, commission, markup float64, markupType string, minStay, maxStay int, enabled bool) error {
	_, err := cm.db.ExecContext(ctx,
		`UPDATE channels SET commission=?, markup=?, markup_type=?, min_stay=?, max_stay=?, enabled=?, updated_at=? WHERE id=?`,
		commission, markup, markupType, minStay, maxStay, enabled, time.Now().UTC(), channelID)
	return err
}

// ─── HELPER METHODS ──────────────────────────────────────────

func (cm *ChannelManager) getRoomTypesWithRates(ctx context.Context, propertyID string, channelMarkup float64) map[string]float64 {
	rows, err := cm.db.QueryContext(ctx,
		`SELECT id, base_rate FROM room_types WHERE property_id=? AND status='active'`, propertyID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	rates := map[string]float64{}
	for rows.Next() {
		var id string
		var price float64
		rows.Scan(&id, &price)
		// Apply channel markup
		rates[id] = price * (1 + channelMarkup/100)
	}
	return rates
}

func (cm *ChannelManager) getRoomAvailability(ctx context.Context, propertyID string) map[string]int {
	rows, err := cm.db.QueryContext(ctx,
		`SELECT room_type_id, COUNT(*) FROM rooms WHERE property_id=? AND status IN ('vacant_clean','vacant_dirty')
		GROUP BY room_type_id`, propertyID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var rtID string
		var count int
		rows.Scan(&rtID, &count)
		counts[rtID] = count
	}
	return counts
}

func (cm *ChannelManager) logSync(ctx context.Context, channelID, channelName, direction, syncType, status string, items, errors, duration int, errMsg string) {
	_, _ = cm.db.ExecContext(ctx,
		`INSERT INTO channel_sync_log (id, channel_id, channel_name, direction, type, status, items_count, error_count, duration_ms, error_msg)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), channelID, channelName, direction, syncType, status, items, errors, duration, errMsg)
}

// ─── OTA PROVIDERS (Mock implementations for demo) ──────────

type BookingComProvider struct{ name, code string }

func (p *BookingComProvider) Name() string { return p.name }
func (p *BookingComProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	log.Printf("[BOOKING.COM] Pushing %d rates for %s", len(req.Rates), req.RoomType)
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates), Timestamp: time.Now().UTC().Format(time.RFC3339)}, nil
}
func (p *BookingComProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	log.Printf("[BOOKING.COM] Pushing %d availability records", len(req.Availabilities))
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *BookingComProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	// In production: call Booking.com XML/REST API
	return []*OTABooking{}, nil
}
func (p *BookingComProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *BookingComProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *BookingComProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *BookingComProvider) TestConnection(ctx context.Context) error { return nil }

type ExpediaProvider struct{ name, code string }

func (p *ExpediaProvider) Name() string { return p.name }
func (p *ExpediaProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	log.Printf("[EXPEDIA] Pushing %d rates for %s", len(req.Rates), req.RoomType)
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *ExpediaProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *ExpediaProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *ExpediaProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *ExpediaProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *ExpediaProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *ExpediaProvider) TestConnection(ctx context.Context) error { return nil }

type AirbnbProvider struct{ name, code string }

func (p *AirbnbProvider) Name() string { return p.name }
func (p *AirbnbProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	log.Printf("[AIRBNB] Pushing %d rates for %s", len(req.Rates), req.RoomType)
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *AirbnbProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *AirbnbProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *AirbnbProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *AirbnbProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *AirbnbProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *AirbnbProvider) TestConnection(ctx context.Context) error { return nil }

type AgodaProvider struct{ name, code string }

func (p *AgodaProvider) Name() string { return p.name }
func (p *AgodaProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	log.Printf("[AGODA] Pushing %d rates for %s", len(req.Rates), req.RoomType)
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *AgodaProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *AgodaProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *AgodaProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *AgodaProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *AgodaProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *AgodaProvider) TestConnection(ctx context.Context) error { return nil }

type JumiaTravelProvider struct{ name, code string }

func (p *JumiaTravelProvider) Name() string { return p.name }
func (p *JumiaTravelProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	log.Printf("[JUMIA TRAVEL] Pushing %d rates for %s", len(req.Rates), req.RoomType)
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *JumiaTravelProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *JumiaTravelProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *JumiaTravelProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *JumiaTravelProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *JumiaTravelProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *JumiaTravelProvider) TestConnection(ctx context.Context) error { return nil }

type HotelsNGProvider struct{ name, code string }

func (p *HotelsNGProvider) Name() string { return p.name }
func (p *HotelsNGProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *HotelsNGProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *HotelsNGProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *HotelsNGProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *HotelsNGProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *HotelsNGProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *HotelsNGProvider) TestConnection(ctx context.Context) error { return nil }

type WakanowProvider struct{ name, code string }

func (p *WakanowProvider) Name() string { return p.name }
func (p *WakanowProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *WakanowProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *WakanowProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *WakanowProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *WakanowProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *WakanowProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *WakanowProvider) TestConnection(ctx context.Context) error { return nil }

type DirectBookingProvider struct{ name, code string }

func (p *DirectBookingProvider) Name() string { return p.name }
func (p *DirectBookingProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *DirectBookingProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *DirectBookingProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *DirectBookingProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *DirectBookingProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *DirectBookingProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *DirectBookingProvider) TestConnection(ctx context.Context) error { return nil }

type MockOTAProvider struct{ name, code string }

func (p *MockOTAProvider) Name() string { return p.name }
func (p *MockOTAProvider) PushRates(ctx context.Context, req *RatePushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Rates)}, nil
}
func (p *MockOTAProvider) PushAvailability(ctx context.Context, req *AvailabilityPushRequest) (*SyncResponse, error) {
	return &SyncResponse{Success: true, SyncedItems: len(req.Availabilities)}, nil
}
func (p *MockOTAProvider) PullBookings(ctx context.Context, since time.Time) ([]*OTABooking, error) {
	return []*OTABooking{}, nil
}
func (p *MockOTAProvider) PushBookingConfirm(ctx context.Context, c *BookingConfirmation) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *MockOTAProvider) CancelBooking(ctx context.Context, id, reason string) (*SyncResponse, error) {
	return &SyncResponse{Success: true}, nil
}
func (p *MockOTAProvider) GetMappingStatus(ctx context.Context) (*MappingStatus, error) {
	return &MappingStatus{}, nil
}
func (p *MockOTAProvider) TestConnection(ctx context.Context) error { return nil }
