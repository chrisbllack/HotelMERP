package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════
// OFFLINE-FIRST SYNC ENGINE
// Core differentiator: "The only PMS that works when the network doesn't"
// ═══════════════════════════════════════════════════════════════

// SyncEngine manages offline queuing and sync for all modules
type SyncEngine struct {
	db           *sql.DB
	online       bool
	syncInterval time.Duration
	handlers     map[string]SyncHandler
}

// SyncHandler processes a specific entity type during sync
type SyncHandler interface {
	Push(ctx context.Context, event *SyncEvent) error
	Pull(ctx context.Context, since time.Time) ([]*SyncEvent, error)
}

// SyncEvent represents a single offline operation to be synced
type SyncEvent struct {
	ID         string                 `json:"id"`
	EntityType string                 `json:"entity_type"` // pos_order, reservation, room_status, housekeeping_task, folio_charge, etc.
	Action     string                 `json:"action"`      // create, update, delete
	EntityID   string                 `json:"entity_id"`
	Payload    map[string]interface{} `json:"payload"`
	DeviceID   string                 `json:"device_id"`
	UserID     string                 `json:"user_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Status     string                 `json:"status"` // pending, syncing, synced, failed, conflict
	RetryCount int                    `json:"retry_count"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
	PropertyID string                 `json:"property_id"`
}

// SyncStatus represents the overall sync state
type SyncStatus struct {
	Online          bool      `json:"online"`
	PendingCount    int       `json:"pending_count"`
	SyncingCount    int       `json:"syncing_count"`
	SyncedCount     int       `json:"synced_count"`
	FailedCount     int       `json:"failed_count"`
	ConflictCount   int       `json:"conflict_count"`
	LastSyncAt      time.Time `json:"last_sync_at"`
	LastSuccessful  time.Time `json:"last_successful_sync"`
	DeviceID        string    `json:"device_id"`
	UptimeSeconds   int       `json:"uptime_seconds"`
	QueueSize       int       `json:"queue_size_bytes"`
}

// NewSyncEngine creates the sync engine with SQLite-backed queue
func NewSyncEngine(db *sql.DB) *SyncEngine {
	se := &SyncEngine{
		db:           db,
		online:       true,
		syncInterval: 30 * time.Second,
		handlers:     make(map[string]SyncHandler),
	}
	se.createTables()
	return se
}

// createTables sets up the offline sync tables
func (se *SyncEngine) createTables() {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS sync_queue (
			id TEXT PRIMARY KEY,
			entity_type TEXT NOT NULL,
			action TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			payload TEXT NOT NULL,
			device_id TEXT,
			user_id TEXT,
			property_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			status TEXT DEFAULT 'pending',
			retry_count INTEGER DEFAULT 0,
			error_msg TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			synced_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON sync_queue(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_queue_entity ON sync_queue(entity_type, entity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_queue_device ON sync_queue(device_id)`,

		`CREATE TABLE IF NOT EXISTS sync_conflicts (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			conflict_type TEXT NOT NULL,
			local_version TEXT NOT NULL,
			remote_version TEXT NOT NULL,
			resolution TEXT,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS sync_devices (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			property_id TEXT NOT NULL,
			last_heartbeat DATETIME,
			last_sync_at DATETIME,
			firmware_version TEXT,
			ip_address TEXT,
			status TEXT DEFAULT 'online',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS offline_capabilities (
			id TEXT PRIMARY KEY,
			module TEXT NOT NULL,
			capability TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			max_queue_size INTEGER DEFAULT 10000,
			sync_priority INTEGER DEFAULT 5,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS write_ahead_log (
			id TEXT PRIMARY KEY,
			operation TEXT NOT NULL,
			table_name TEXT NOT NULL,
			row_id TEXT NOT NULL,
			before_image TEXT,
			after_image TEXT NOT NULL,
			xid TEXT,
			committed INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, t := range tables {
		_, _ = se.db.ExecContext(ctx, t)
	}

	// Seed offline capabilities for each module
	se.seedCapabilities(ctx)
}

// seedCapabilities registers which modules support offline operation
func (se *SyncEngine) seedCapabilities(ctx context.Context) {
	caps := []struct {
		module, cap, desc string
		priority          int
	}{
		{"pos", "create_order", "Create POS orders offline", 1},
		{"pos", "close_order", "Close/settle POS orders offline", 1},
		{"pos", "split_bill", "Split bills offline", 2},
		{"pos", "room_charge", "Post charges to guest folio offline", 1},
		{"front_desk", "check_in", "Process check-in offline", 1},
		{"front_desk", "check_out", "Process check-out offline", 1},
		{"front_desk", "room_assignment", "Assign rooms offline", 1},
		{"front_desk", "folio_charge", "Post folio charges offline", 1},
		{"front_desk", "walk_in", "Register walk-in guests offline", 1},
		{"housekeeping", "update_status", "Update room status offline", 1},
		{"housekeeping", "task_complete", "Complete cleaning tasks offline", 2},
		{"housekeeping", "maintenance_request", "Create maintenance tickets offline", 3},
		{"reservations", "create_booking", "Create reservations offline", 2},
		{"reservations", "modify_booking", "Modify reservations offline", 2},
		{"inventory", "stock_issue", "Issue stock offline", 3},
		{"inventory", "receive_stock", "Receive stock offline", 3},
	}

	for _, c := range caps {
		id := uuid.New().String()
		_, _ = se.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO offline_capabilities (id, module, capability, enabled, sync_priority, description) VALUES (?, ?, ?, 1, ?, ?)`,
			id, c.module, c.cap, c.priority, c.desc)
	}
}

// Enqueue adds an event to the offline sync queue
func (se *SyncEngine) Enqueue(ctx context.Context, entityType, action, entityID, deviceID, userID, propertyID string, payload interface{}) (string, error) {
	id := uuid.New().String()
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Write-ahead log entry
	se.writeAheadLog(ctx, action, entityType, entityID, string(payloadJSON))

	// Insert into sync queue
	_, err = se.db.ExecContext(ctx, `
		INSERT INTO sync_queue (id, entity_type, action, entity_id, payload, device_id, user_id, property_id, timestamp, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		id, entityType, action, entityID, string(payloadJSON), deviceID, userID, propertyID, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("enqueue: %w", err)
	}

	log.Printf("[SYNC] Enqueued %s:%s (id=%s, device=%s)", entityType, action, entityID, deviceID)
	return id, nil
}

// ProcessQueue processes pending sync events
func (se *SyncEngine) ProcessQueue(ctx context.Context) error {
	rows, err := se.db.QueryContext(ctx, `
		SELECT id, entity_type, action, entity_id, payload, device_id, user_id, property_id, timestamp, retry_count
		FROM sync_queue
		WHERE status = 'pending' AND retry_count < 5
		ORDER BY
			CASE entity_type
				WHEN 'pos_order' THEN 1
				WHEN 'folio_charge' THEN 1
				WHEN 'check_in' THEN 2
				WHEN 'check_out' THEN 2
				WHEN 'room_status' THEN 3
				WHEN 'housekeeping_task' THEN 4
				ELSE 5
			END,
			timestamp ASC
		LIMIT 50`)
	if err != nil {
		return fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var event SyncEvent
		var payloadStr string
		var ts time.Time
		rows.Scan(&event.ID, &event.EntityType, &event.Action, &event.EntityID,
			&payloadStr, &event.DeviceID, &event.UserID, &event.PropertyID, &ts, &event.RetryCount)
		event.Timestamp = ts

		if err := json.Unmarshal([]byte(payloadStr), &event.Payload); err != nil {
			se.markFailed(ctx, event.ID, "payload parse error")
			continue
		}

		// Try to push to the handler
		if handler, ok := se.handlers[event.EntityType]; ok {
			if err := handler.Push(ctx, &event); err != nil {
				se.incrementRetry(ctx, event.ID, err.Error())
				continue
			}
		} else {
			// Default: apply directly to DB
			if err := se.applyEvent(ctx, &event); err != nil {
				se.incrementRetry(ctx, event.ID, err.Error())
				continue
			}
		}

		se.markSynced(ctx, event.ID)
		processed++
	}

	if processed > 0 {
		log.Printf("[SYNC] Processed %d events from queue", processed)
	}
	return nil
}

// applyEvent applies a sync event to the database
func (se *SyncEngine) applyEvent(ctx context.Context, event *SyncEvent) error {
	payloadJSON, _ := json.Marshal(event.Payload)

	switch event.Action {
	case "create":
		// Generic insert using the entity type as table name
		log.Printf("[SYNC] Applying create for %s:%s", event.EntityType, event.EntityID)
	case "update":
		log.Printf("[SYNC] Applying update for %s:%s", event.EntityType, event.EntityID)
	case "delete":
		log.Printf("[SYNC] Applying delete for %s:%s", event.EntityType, event.EntityID)
	}

	// For demo mode, just log the application
	log.Printf("[SYNC] Event applied: %s %s %s (payload=%d bytes)", event.Action, event.EntityType, event.EntityID, len(payloadJSON))
	return nil
}

func (se *SyncEngine) writeAheadLog(ctx context.Context, operation, table, rowID, afterImage string) {
	_, _ = se.db.ExecContext(ctx,
		`INSERT INTO write_ahead_log (id, operation, table_name, row_id, after_image, xid, committed) VALUES (?, ?, ?, ?, ?, ?, 0)`,
		uuid.New().String(), operation, table, rowID, afterImage, fmt.Sprintf("xid-%d", time.Now().UnixNano()))
}

func (se *SyncEngine) markSynced(ctx context.Context, eventID string) {
	_, _ = se.db.ExecContext(ctx,
		`UPDATE sync_queue SET status='synced', synced_at=? WHERE id=?`,
		time.Now().UTC(), eventID)
}

func (se *SyncEngine) markFailed(ctx context.Context, eventID, errMsg string) {
	_, _ = se.db.ExecContext(ctx,
		`UPDATE sync_queue SET status='failed', error_msg=? WHERE id=?`,
		errMsg, eventID)
}

func (se *SyncEngine) incrementRetry(ctx context.Context, eventID, errMsg string) {
	_, _ = se.db.ExecContext(ctx,
		`UPDATE sync_queue SET retry_count=retry_count+1, error_msg=?, status=CASE WHEN retry_count+1>=5 THEN 'failed' ELSE 'pending' END WHERE id=?`,
		errMsg, eventID)
}

// GetStatus returns current sync engine status
func (se *SyncEngine) GetStatus(ctx context.Context, deviceID string) *SyncStatus {
	status := &SyncStatus{Online: se.online, DeviceID: deviceID}

	se.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue WHERE status='pending'`).Scan(&status.PendingCount)
	se.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue WHERE status='syncing'`).Scan(&status.SyncingCount)
	se.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue WHERE status='synced'`).Scan(&status.SyncedCount)
	se.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue WHERE status='failed'`).Scan(&status.FailedCount)
	se.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_conflicts WHERE resolution IS NULL`).Scan(&status.ConflictCount)
	se.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(synced_at),'1970-01-01') FROM sync_queue WHERE status='synced'`).Scan(&status.LastSyncAt)
	se.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(LENGTH(payload)),0) FROM sync_queue WHERE status='pending'`).Scan(&status.QueueSize)

	return status
}

// SetOnline sets the online/offline state
func (se *SyncEngine) SetOnline(online bool) {
	se.online = online
	log.Printf("[SYNC] Network status: %v", online)
}

// RegisterHandler registers a module-specific sync handler
func (se *SyncEngine) RegisterHandler(entityType string, handler SyncHandler) {
	se.handlers[entityType] = handler
}
