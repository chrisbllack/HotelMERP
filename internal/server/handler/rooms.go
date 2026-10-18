package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── Room Types ──────────────────────────────────────────────

type CreateRoomTypeRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	MaxOccupancy  int     `json:"max_occupancy"`
	AdultCapacity int     `json:"adult_capacity"`
	ChildCapacity int     `json:"child_capacity"`
	BedConfig     string  `json:"bed_configuration"`
	RoomSize      float64 `json:"room_size"`
	View          string  `json:"view"`
	BaseRate      float64 `json:"base_rate"`
	MinRate       float64 `json:"min_rate"`
	MaxRate       float64 `json:"max_rate"`
	Status        string  `json:"status"`
}

func (h *Handler) ListRoomTypes(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	query := `SELECT id, code, name, description, max_occupancy, adult_capacity, child_capacity, 
		bed_configuration, room_size, view, base_rate, min_rate, max_rate, status, sort_order, created_at
		FROM room_types WHERE property_id = $1 AND deleted_at IS NULL ORDER BY sort_order, name`

	rows, err := h.db.Query(context.Background(), query, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to query room types", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var roomTypes []map[string]interface{}
	for rows.Next() {
		var id string
		var code, name, status string
		var description, bedConfig, view *string
		var maxOcc, adultCap, childCap, sortOrder int
		var roomSize, baseRate, minRate, maxRate *float64
		var createdAt interface{}

		if err := rows.Scan(&id, &code, &name, &description, &maxOcc, &adultCap, &childCap,
			&bedConfig, &roomSize, &view, &baseRate, &minRate, &maxRate, &status, &sortOrder, &createdAt); err != nil {
			continue
		}

		rt := map[string]interface{}{
			"id": id, "code": code, "name": name, "max_occupancy": maxOcc,
			"adult_capacity": adultCap, "child_capacity": childCap, "status": status,
		}
		if description != nil {
			rt["description"] = *description
		}
		if bedConfig != nil {
			rt["bed_configuration"] = *bedConfig
		}
		if roomSize != nil {
			rt["room_size"] = *roomSize
		}
		if view != nil {
			rt["view"] = *view
		}
		if baseRate != nil {
			rt["base_rate"] = *baseRate
		}
		if minRate != nil {
			rt["min_rate"] = *minRate
		}
		if maxRate != nil {
			rt["max_rate"] = *maxRate
		}
		rt["sort_order"] = sortOrder

		roomTypes = append(roomTypes, rt)
	}

	if roomTypes == nil {
		roomTypes = []map[string]interface{}{}
	}
	response.RespondSuccess(w, roomTypes, "Room types retrieved successfully")
}

func (h *Handler) CreateRoomType(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	var req CreateRoomTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.Name == "" {
		response.RespondError(w, "Code and name are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO room_types (id, property_id, code, name, description, max_occupancy, adult_capacity, child_capacity,
			bed_configuration, room_size, view, base_rate, min_rate, max_rate, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, id, propertyID, req.Code, req.Name, req.Description, req.MaxOccupancy, req.AdultCapacity, req.ChildCapacity,
		req.BedConfig, req.RoomSize, req.View, req.BaseRate, req.MinRate, req.MaxRate,
		statusOrDefault(req.Status, "active"))

	if err != nil {
		response.RespondError(w, "Failed to create room type (code may already exist)", http.StatusConflict)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id, "code": req.Code, "name": req.Name}, "Room type created successfully", http.StatusCreated)
}

func (h *Handler) GetRoomType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var code, name, status string
	var description, bedConfig, view *string
	var maxOcc, adultCap, childCap, sortOrder int
	var roomSize, baseRate, minRate, maxRate *float64

	err := h.db.QueryRow(context.Background(), `
		SELECT code, name, description, max_occupancy, adult_capacity, child_capacity,
			bed_configuration, room_size, view, base_rate, min_rate, max_rate, status, sort_order
		FROM room_types WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, id, propertyID).Scan(&code, &name, &description, &maxOcc, &adultCap, &childCap,
		&bedConfig, &roomSize, &view, &baseRate, &minRate, &maxRate, &status, &sortOrder)

	if err != nil {
		response.RespondError(w, "Room type not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id": id, "code": code, "name": name, "max_occupancy": maxOcc,
		"adult_capacity": adultCap, "child_capacity": childCap, "status": status, "sort_order": sortOrder,
	}
	if description != nil {
		result["description"] = *description
	}
	if bedConfig != nil {
		result["bed_configuration"] = *bedConfig
	}
	if roomSize != nil {
		result["room_size"] = *roomSize
	}
	if view != nil {
		result["view"] = *view
	}
	if baseRate != nil {
		result["base_rate"] = *baseRate
	}
	if minRate != nil {
		result["min_rate"] = *minRate
	}
	if maxRate != nil {
		result["max_rate"] = *maxRate
	}

	response.RespondSuccess(w, result, "Room type retrieved successfully")
}

func (h *Handler) UpdateRoomType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var req CreateRoomTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE room_types SET 
			code = COALESCE(NULLIF($3, ''), code),
			name = COALESCE(NULLIF($4, ''), name),
			description = $5,
			max_occupancy = $6, adult_capacity = $7, child_capacity = $8,
			bed_configuration = $9, room_size = $10, view = $11,
			base_rate = $12, min_rate = $13, max_rate = $14,
			updated_at = NOW()
		WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, id, propertyID, req.Code, req.Name, req.Description, req.MaxOccupancy, req.AdultCapacity,
		req.ChildCapacity, req.BedConfig, req.RoomSize, req.View, req.BaseRate, req.MinRate, req.MaxRate)

	if err != nil {
		response.RespondError(w, "Failed to update room type", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Room type updated successfully")
}

func (h *Handler) DeleteRoomType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	_, err := h.db.Exec(context.Background(),
		"UPDATE room_types SET deleted_at = NOW() WHERE id = $1 AND property_id = $2", id, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to delete room type", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, nil, "Room type deleted successfully")
}

// ── Rooms ──────────────────────────────────────────────────

type CreateRoomRequest struct {
	RoomTypeID string `json:"room_type_id"`
	Number     string `json:"number"`
	BuildingID string `json:"building_id"`
	FloorID    string `json:"floor_id"`
	IsSmoking  bool   `json:"is_smoking"`
	Status     string `json:"status"`
}

func (h *Handler) ListRooms(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	// Parse query params for filtering
	statusFilter := r.URL.Query().Get("status")
	floorFilter := r.URL.Query().Get("floor_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	query := `SELECT r.id, r.number, r.status, r.housekeeping_status, r.maintenance_status, r.is_smoking,
		r.room_type_id, rt.name as room_type_name, r.building_id, r.floor_id, r.created_at
		FROM rooms r
		JOIN room_types rt ON r.room_type_id = rt.id
		WHERE r.property_id = $1 AND r.deleted_at IS NULL`

	args := []interface{}{propertyID}
	argIdx := 2

	if statusFilter != "" {
		query += ` AND r.status = $` + strconv.Itoa(argIdx)
		args = append(args, statusFilter)
		argIdx++
	}
	if floorFilter != "" {
		query += ` AND r.floor_id = $` + strconv.Itoa(argIdx)
		args = append(args, floorFilter)
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM rooms WHERE property_id = $1 AND deleted_at IS NULL"
	var total int64
	h.db.QueryRow(context.Background(), countQuery, propertyID).Scan(&total)

	query += ` ORDER BY r.number LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query rooms", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var rooms []map[string]interface{}
	for rows.Next() {
		var id, roomTypeID, number, roomStatus, hkStatus, maintStatus string
		var isSmoking interface{}

		var roomTypeName string
		var buildingID, floorID *string
		var createdAt interface{}

		if err := rows.Scan(&id, &number, &roomStatus, &hkStatus, &maintStatus, &isSmoking,
			&roomTypeID, &roomTypeName, &buildingID, &floorID, &createdAt); err != nil {
			continue
		}

		rm := map[string]interface{}{
			"id": id, "number": number, "status": roomStatus,
			"housekeeping_status": hkStatus, "maintenance_status": maintStatus,
			"is_smoking": isSmoking, "room_type_id": roomTypeID, "room_type_name": roomTypeName,
		}
		if buildingID != nil {
			rm["building_id"] = *buildingID
		}
		if floorID != nil {
			rm["floor_id"] = *floorID
		}
		rooms = append(rooms, rm)
	}

	if rooms == nil {
		rooms = []map[string]interface{}{}
	}
	response.RespondPaginated(w, rooms, total, page, perPage)
}

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RoomTypeID == "" || req.Number == "" {
		response.RespondError(w, "Room type and number are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	roomStatus := statusOrDefault(req.Status, "vacant_dirty")

	_, err := h.db.Exec(context.Background(), `
		INSERT INTO rooms (id, property_id, room_type_id, number, building_id, floor_id, is_smoking, status, housekeeping_status, status_changed_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, $7, $8, 'dirty', NOW())
	`, id, propertyID, req.RoomTypeID, req.Number, req.BuildingID, req.FloorID, req.IsSmoking, roomStatus)

	if err != nil {
		response.RespondError(w, "Failed to create room (number may already exist)", http.StatusConflict)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id, "number": req.Number}, "Room created successfully", http.StatusCreated)
}

func (h *Handler) GetRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var number, roomStatus, hkStatus, maintStatus string
	var isSmoking bool
	var roomTypeID string
	var roomTypeName string
	var buildingID, floorID *string

	err := h.db.QueryRow(context.Background(), `
		SELECT r.number, r.status, r.housekeeping_status, r.maintenance_status, r.is_smoking,
			r.room_type_id, rt.name, r.building_id, r.floor_id
		FROM rooms r JOIN room_types rt ON r.room_type_id = rt.id
		WHERE r.id = $1 AND r.property_id = $2 AND r.deleted_at IS NULL
	`, id, propertyID).Scan(&number, &roomStatus, &hkStatus, &maintStatus, &isSmoking,
		&roomTypeID, &roomTypeName, &buildingID, &floorID)

	if err != nil {
		response.RespondError(w, "Room not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id": id, "number": number, "status": roomStatus,
		"housekeeping_status": hkStatus, "maintenance_status": maintStatus,
		"is_smoking": isSmoking, "room_type_id": roomTypeID, "room_type_name": roomTypeName,
	}
	if buildingID != nil {
		result["building_id"] = *buildingID
	}
	if floorID != nil {
		result["floor_id"] = *floorID
	}

	response.RespondSuccess(w, result, "Room retrieved successfully")
}

func (h *Handler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE rooms SET
			room_type_id = COALESCE(NULLIF($3, '')::uuid, room_type_id),
			number = COALESCE(NULLIF($4, ''), number),
			building_id = NULLIF($5, '')::uuid,
			floor_id = NULLIF($6, '')::uuid,
			is_smoking = $7,
			updated_at = NOW()
		WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, id, propertyID, req.RoomTypeID, req.Number, req.BuildingID, req.FloorID, req.IsSmoking)

	if err != nil {
		response.RespondError(w, "Failed to update room", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Room updated successfully")
}

func (h *Handler) UpdateRoomStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var req struct {
		Status             string `json:"status"`
		HousekeepingStatus string `json:"housekeeping_status"`
		MaintenanceStatus  string `json:"maintenance_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status != "" {
		validStatuses := map[string]bool{
			"vacant_clean": true, "vacant_dirty": true, "occupied_clean": true, "occupied_dirty": true,
			"inspected": true, "reserved": true, "due_in": true, "due_out": true,
			"out_of_order": true, "out_of_service": true, "blocked": true, "house_use": true, "complimentary": true,
		}
		if !validStatuses[req.Status] {
			response.RespondError(w, "Invalid room status", http.StatusBadRequest)
			return
		}

		_, err := h.db.Exec(context.Background(), `
			UPDATE rooms SET status = $3, status_changed_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
		`, id, propertyID, req.Status)
		if err != nil {
			response.RespondError(w, "Failed to update room status", http.StatusInternalServerError)
			return
		}
	}

	if req.HousekeepingStatus != "" {
		h.db.Exec(context.Background(), `
			UPDATE rooms SET housekeeping_status = $3, updated_at = NOW()
			WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
		`, id, propertyID, req.HousekeepingStatus)
	}

	if req.MaintenanceStatus != "" {
		h.db.Exec(context.Background(), `
			UPDATE rooms SET maintenance_status = $3, updated_at = NOW()
			WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
		`, id, propertyID, req.MaintenanceStatus)
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Room status updated successfully")
}

func (h *Handler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	_, err := h.db.Exec(context.Background(),
		"UPDATE rooms SET deleted_at = NOW() WHERE id = $1 AND property_id = $2", id, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to delete room", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, nil, "Room deleted successfully")
}

func statusOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
