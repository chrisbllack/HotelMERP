package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"infineon/internal/auth"
	"infineon/internal/events"
	"infineon/internal/ledger"
	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Valid reservation status transitions
var validTransitions = map[string][]string{
	"inquiry":     {"tentative", "confirmed", "cancelled", "expired"},
	"tentative":   {"confirmed", "cancelled", "expired"},
	"confirmed":   {"checked_in", "cancelled", "no_show"},
	"checked_in":  {"in_house"},
	"in_house":    {"checked_out"},
	"checked_out": {},
	"cancelled":   {},
	"no_show":     {},
	"expired":     {},
}

type CreateReservationRequest struct {
	GuestID            string  `json:"guest_id"`
	RatePlanID         string  `json:"rate_plan_id"`
	RoomTypeID         string  `json:"room_type_id"`
	RoomID             string  `json:"room_id"`
	ArrivalDate        string  `json:"arrival_date"`
	DepartureDate      string  `json:"departure_date"`
	Adults             int     `json:"adults"`
	Children           int     `json:"children"`
	RoomCount          int     `json:"room_count"`
	Source             string  `json:"source"`
	MarketSegmentID    string  `json:"market_segment_id"`
	ChannelID          string  `json:"channel_id"`
	CorporateAccountID string  `json:"corporate_account_id"`
	TravelAgentID      string  `json:"travel_agent_id"`
	DepositAmount      float64 `json:"deposit_amount"`
	TotalEstimate      float64 `json:"total_estimated"`
	Currency           string  `json:"currency"`
	SpecialRequests    string  `json:"special_requests"`
	InternalNotes      string  `json:"internal_notes"`
	IsVIP              bool    `json:"is_vip"`
}

func generateConfirmationNumber() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func (h *Handler) ListReservations(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")
	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	query := `SELECT res.id, res.confirmation_number, res.guest_id, 
		g.first_name || ' ' || g.last_name as guest_name,
		res.room_type_id, rt.name as room_type_name, res.room_id, r.number as room_number,
		res.arrival_date, res.departure_date, res.nights, res.adults, res.children,
		res.status, res.deposit_amount, res.total_estimated, res.source,
		res.is_vip, res.special_requests, res.created_at
		FROM reservations res
		JOIN guests g ON res.guest_id = g.id
		JOIN room_types rt ON res.room_type_id = rt.id
		LEFT JOIN rooms r ON res.room_id = r.id
		WHERE res.property_id = $1 AND res.deleted_at IS NULL`
	args := []interface{}{propertyID}
	argIdx := 2

	if statusFilter != "" {
		query += ` AND res.status = $` + strconv.Itoa(argIdx)
		args = append(args, statusFilter)
		argIdx++
	}
	if dateFrom != "" {
		query += ` AND res.arrival_date >= $` + strconv.Itoa(argIdx)
		args = append(args, dateFrom)
		argIdx++
	}
	if dateTo != "" {
		query += ` AND res.departure_date <= $` + strconv.Itoa(argIdx)
		args = append(args, dateTo)
		argIdx++
	}
	if search != "" {
		query += ` AND (g.first_name ILIKE $` + strconv.Itoa(argIdx) + ` OR g.last_name ILIKE $` + strconv.Itoa(argIdx) +
			` OR res.confirmation_number ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Count
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM reservations res JOIN guests g ON res.guest_id = g.id WHERE res.property_id = $1 AND res.deleted_at IS NULL`)
	var total int64
	h.db.QueryRow(context.Background(), countQuery, propertyID).Scan(&total)

	query += ` ORDER BY res.created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query reservations", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var reservations []map[string]interface{}
	for rows.Next() {
		var id, guestID, roomTypeID uuid.UUID
		var confirmationNumber, guestName, roomTypeName, status, source string
		var roomID, roomNumber *uuid.UUID
		var arrivalDate, departureDate time.Time
		var nights, adults, children int
		var depositAmount, totalEstimate float64
		var isVIP bool
		var specialRequests *string
		var createdAt interface{}

		err := rows.Scan(&id, &confirmationNumber, &guestID, &guestName,
			&roomTypeID, &roomTypeName, &roomID, &roomNumber,
			&arrivalDate, &departureDate, &nights, &adults, &children,
			&status, &depositAmount, &totalEstimate, &source,
			&isVIP, &specialRequests, &createdAt)
		if err != nil {
			continue
		}

		res := map[string]interface{}{
			"id": id, "confirmation_number": confirmationNumber, "guest_id": guestID,
			"guest_name": guestName, "room_type_id": roomTypeID, "room_type_name": roomTypeName,
			"arrival_date": arrivalDate.Format("2006-01-02"), "departure_date": departureDate.Format("2006-01-02"),
			"nights": nights, "adults": adults, "children": children,
			"status": status, "deposit_amount": depositAmount, "total_estimated": totalEstimate,
			"source": source, "is_vip": isVIP,
		}
		if roomID != nil {
			res["room_id"] = *roomID
		}
		if roomNumber != nil {
			res["room_number"] = *roomNumber
		}
		if specialRequests != nil {
			res["special_requests"] = *specialRequests
		}
		reservations = append(reservations, res)
	}

	if reservations == nil {
		reservations = []map[string]interface{}{}
	}
	response.RespondPaginated(w, reservations, total, page, perPage)
}

func (h *Handler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	var req CreateReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.GuestID == "" || req.RatePlanID == "" || req.RoomTypeID == "" || req.ArrivalDate == "" || req.DepartureDate == "" {
		response.RespondError(w, "guest_id, rate_plan_id, room_type_id, arrival_date, and departure_date are required", http.StatusBadRequest)
		return
	}

	if req.Adults < 1 {
		req.Adults = 1
	}
	if req.RoomCount < 1 {
		req.RoomCount = 1
	}

	arrival, err := time.Parse("2006-01-02", req.ArrivalDate)
	if err != nil {
		response.RespondError(w, "Invalid arrival_date format (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	departure, err := time.Parse("2006-01-02", req.DepartureDate)
	if err != nil {
		response.RespondError(w, "Invalid departure_date format (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	nights := int(departure.Sub(arrival).Hours() / 24)
	if nights < 1 {
		response.RespondError(w, "Departure date must be after arrival date", http.StatusBadRequest)
		return
	}

	// Check availability
	available := h.checkAvailability(r.Context(), propertyID, req.RoomTypeID, req.ArrivalDate, req.DepartureDate, req.RoomCount)
	if !available {
		response.RespondError(w, "No rooms available for the selected dates and type", http.StatusConflict)
		return
	}

	// Get base rate from rate plan
	var basePrice float64
	h.db.QueryRow(context.Background(), "SELECT base_price FROM rate_plans WHERE id = $1 AND is_active = TRUE", req.RatePlanID).Scan(&basePrice)

	totalEstimate := basePrice * float64(nights) * float64(req.RoomCount)
	if req.TotalEstimate > 0 {
		totalEstimate = req.TotalEstimate
	}

	// Get user ID from context
	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	id := uuid.New()
	confirmationNumber := generateConfirmationNumber()
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	_, err = h.db.Exec(context.Background(), `
		INSERT INTO reservations (id, property_id, confirmation_number, guest_id, rate_plan_id, room_type_id, room_id,
			arrival_date, departure_date, nights, adults, children, room_count, source, market_segment_id, channel_id,
			corporate_account_id, travel_agent_id, status, deposit_amount, deposit_paid, total_estimated, currency,
			special_requests, internal_notes, is_vip, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, 'confirmed', $19, 0, $20, $21, $22, $23, $24, $25)
	`, id, propertyID, confirmationNumber, req.GuestID, req.RatePlanID, req.RoomTypeID,
		uuidOrEmpty(req.RoomID), req.ArrivalDate, req.DepartureDate, nights, req.Adults, req.Children,
		req.RoomCount, req.Source, uuidOrEmpty(req.MarketSegmentID), uuidOrEmpty(req.ChannelID),
		uuidOrEmpty(req.CorporateAccountID), uuidOrEmpty(req.TravelAgentID),
		req.DepositAmount, totalEstimate, currency,
		nullString(req.SpecialRequests), nullString(req.InternalNotes), req.IsVIP, createdBy)

	if err != nil {
		response.RespondError(w, "Failed to create reservation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record status history
	h.db.Exec(context.Background(), `
		INSERT INTO reservation_status_history (reservation_id, to_status, changed_by, created_at)
		VALUES ($1, 'confirmed', $2, NOW())
	`, id, createdBy)

	response.RespondSuccess(w, map[string]interface{}{
		"id": id, "confirmation_number": confirmationNumber, "status": "confirmed",
		"arrival_date": req.ArrivalDate, "departure_date": req.DepartureDate, "nights": nights,
	}, "Reservation created successfully", http.StatusCreated)
}

func (h *Handler) GetReservation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var confirmationNumber, status, source, currency string
	var guestID, roomTypeID uuid.UUID
	var roomID *uuid.UUID
	var guestName, roomTypeName string
	var arrivalDate, departureDate time.Time
	var nights, adults, children, roomCount int
	var depositAmount, depositPaid, totalEstimate, exchangeRate float64
	var isVIP, isComplimentary, isHouseUse bool
	var specialRequests, internalNotes *string
	var createdAt interface{}

	err := h.db.QueryRow(context.Background(), `
		SELECT res.confirmation_number, res.guest_id, g.first_name || ' ' || g.last_name,
			res.rate_plan_id, res.room_type_id, rt.name, res.room_id,
			res.arrival_date, res.departure_date, res.nights, res.adults, res.children, res.room_count,
			res.source, res.status, res.deposit_amount, res.deposit_paid, res.total_estimated,
			res.currency, res.exchange_rate, res.special_requests, res.internal_notes,
			res.is_vip, res.is_complimentary, res.is_house_use, res.created_at
		FROM reservations res
		JOIN guests g ON res.guest_id = g.id
		JOIN room_types rt ON res.room_type_id = rt.id
		WHERE res.id = $1 AND res.property_id = $2 AND res.deleted_at IS NULL
	`, id, propertyID).Scan(&confirmationNumber, &guestID, &guestName,
		&uuid.UUID{}, &roomTypeID, &roomTypeName, &roomID,
		&arrivalDate, &departureDate, &nights, &adults, &children, &roomCount,
		&source, &status, &depositAmount, &depositPaid, &totalEstimate,
		&currency, &exchangeRate, &specialRequests, &internalNotes,
		&isVIP, &isComplimentary, &isHouseUse, &createdAt)

	if err != nil {
		response.RespondError(w, "Reservation not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id": id, "confirmation_number": confirmationNumber, "guest_id": guestID, "guest_name": guestName,
		"room_type_id": roomTypeID, "room_type_name": roomTypeName,
		"arrival_date": arrivalDate.Format("2006-01-02"), "departure_date": departureDate.Format("2006-01-02"),
		"nights": nights, "adults": adults, "children": children, "room_count": roomCount,
		"source": source, "status": status, "deposit_amount": depositAmount, "deposit_paid": depositPaid,
		"total_estimated": totalEstimate, "currency": currency, "exchange_rate": exchangeRate,
		"is_vip": isVIP, "is_complimentary": isComplimentary, "is_house_use": isHouseUse,
	}
	if roomID != nil {
		result["room_id"] = *roomID
	}
	if specialRequests != nil {
		result["special_requests"] = *specialRequests
	}
	if internalNotes != nil {
		result["internal_notes"] = *internalNotes
	}

	response.RespondSuccess(w, result, "Reservation retrieved successfully")
}

func (h *Handler) UpdateReservation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var req CreateReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE reservations SET
			guest_id = NULLIF($3, '')::uuid,
			rate_plan_id = NULLIF($4, '')::uuid,
			room_type_id = NULLIF($5, '')::uuid,
			room_id = NULLIF($6, '')::uuid,
			arrival_date = COALESCE(NULLIF($7, '')::date, arrival_date),
			departure_date = COALESCE(NULLIF($8, '')::date, departure_date),
			adults = $9, children = $10,
			special_requests = $11, internal_notes = $12,
			is_vip = $13, updated_at = NOW()
		WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, id, propertyID, req.GuestID, req.RatePlanID, req.RoomTypeID, req.RoomID,
		req.ArrivalDate, req.DepartureDate, req.Adults, req.Children,
		nullString(req.SpecialRequests), nullString(req.InternalNotes), req.IsVIP)

	if err != nil {
		response.RespondError(w, "Failed to update reservation", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Reservation updated successfully")
}

func (h *Handler) ConfirmReservation(w http.ResponseWriter, r *http.Request) {
	h.changeReservationStatus(w, r, "confirmed")
}

func (h *Handler) CancelReservation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var currentStatus string
	err := h.db.QueryRow(context.Background(),
		"SELECT status FROM reservations WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL",
		id, propertyID).Scan(&currentStatus)

	if err != nil {
		response.RespondError(w, "Reservation not found", http.StatusNotFound)
		return
	}

	if !canTransition(currentStatus, "cancelled") {
		response.RespondError(w, fmt.Sprintf("Cannot cancel reservation in '%s' status", currentStatus), http.StatusConflict)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	h.db.Exec(context.Background(), `
		UPDATE reservations SET status = 'cancelled', cancellation_reason = $3, 
			cancelled_at = NOW(), cancelled_by = $4, updated_at = NOW()
		WHERE id = $1 AND property_id = $2
	`, id, propertyID, nullString(req.Reason), createdBy)

	h.db.Exec(context.Background(), `
		INSERT INTO reservation_status_history (reservation_id, from_status, to_status, reason, changed_by, created_at)
		VALUES ($1, $2, 'cancelled', $3, $4, NOW())
	`, id, currentStatus, nullString(req.Reason), createdBy)

	// Free up room if assigned
	h.db.Exec(context.Background(), `
		UPDATE rooms SET status = 'vacant_dirty', status_changed_at = NOW(), updated_at = NOW()
		WHERE id = (SELECT room_id FROM reservations WHERE id = $1) AND status = 'reserved'
	`, id)

	response.RespondSuccess(w, nil, "Reservation cancelled successfully")
}

func (h *Handler) changeReservationStatus(w http.ResponseWriter, r *http.Request, newStatus string) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var currentStatus string
	err := h.db.QueryRow(context.Background(),
		"SELECT status FROM reservations WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL",
		id, propertyID).Scan(&currentStatus)

	if err != nil {
		response.RespondError(w, "Reservation not found", http.StatusNotFound)
		return
	}

	if !canTransition(currentStatus, newStatus) {
		response.RespondError(w, fmt.Sprintf("Cannot transition from '%s' to '%s'", currentStatus, newStatus), http.StatusConflict)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	h.db.Exec(context.Background(), `
		UPDATE reservations SET status = $3, updated_at = NOW() WHERE id = $1 AND property_id = $2
	`, id, propertyID, newStatus)

	h.db.Exec(context.Background(), `
		INSERT INTO reservation_status_history (reservation_id, from_status, to_status, changed_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, currentStatus, newStatus, createdBy)

	response.RespondSuccess(w, map[string]interface{}{
		"id": id, "from_status": currentStatus, "to_status": newStatus,
	}, fmt.Sprintf("Reservation status changed to %s", newStatus))
}

func (h *Handler) NoShow(w http.ResponseWriter, r *http.Request) {
	h.changeReservationStatus(w, r, "no_show")
}

// ── Availability ──────────────────────────────────────────

func (h *Handler) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	roomTypeID := r.URL.Query().Get("room_type_id")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	if propertyID == "" || roomTypeID == "" || dateFrom == "" || dateTo == "" {
		response.RespondError(w, "room_type_id, date_from, and date_to are required", http.StatusBadRequest)
		return
	}

	available := h.checkAvailability(r.Context(), propertyID, roomTypeID, dateFrom, dateTo, 1)

	// Get total rooms of this type
	var totalRooms int
	h.db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rooms WHERE room_type_id = $1 AND deleted_at IS NULL
		AND status NOT IN ('out_of_order', 'blocked')
	`, roomTypeID).Scan(&totalRooms)

	// Get booked rooms for these dates
	var bookedRooms int
	h.db.QueryRow(context.Background(), `
		SELECT COUNT(DISTINCT room_id) FROM reservations 
		WHERE room_type_id = $1 AND status IN ('confirmed', 'checked_in', 'in_house')
		AND arrival_date < $3 AND departure_date > $2
		AND room_id IS NOT NULL
	`, roomTypeID, dateFrom, dateTo).Scan(&bookedRooms)

	response.RespondSuccess(w, map[string]interface{}{
		"available":       available,
		"total_rooms":     totalRooms,
		"booked_rooms":    bookedRooms,
		"available_rooms": totalRooms - bookedRooms,
	}, "Availability checked successfully")
}

func (h *Handler) checkAvailability(ctx context.Context, propertyID, roomTypeID, dateFrom, dateTo string, roomCount int) bool {
	var availableRooms int
	h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms r
		WHERE r.property_id = $1 AND r.room_type_id = $2 AND r.deleted_at IS NULL
		AND r.status NOT IN ('out_of_order', 'out_of_service', 'blocked')
		AND r.id NOT IN (
			SELECT res.room_id FROM reservations res
			WHERE res.room_id IS NOT NULL
			AND res.status IN ('confirmed', 'checked_in', 'in_house')
			AND res.room_type_id = $2
			AND res.arrival_date < $4 AND res.departure_date > $3
		)
	`, propertyID, roomTypeID, dateFrom, dateTo).Scan(&availableRooms)

	return availableRooms >= roomCount
}

// ── Check-in ──────────────────────────────────────────────

type CheckInRequest struct {
	RoomID      string  `json:"room_id"`
	IDType      string  `json:"id_type"`
	IDNumber    string  `json:"id_number"`
	DepositPaid float64 `json:"deposit_paid"`
}

func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	propertyID := r.Header.Get("X-Property-ID")

	var req CheckInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get reservation
	var currentStatus, guestID, roomTypeID, ratePlanID, currency string
	var nights, adults, children int
	var arrivalDate time.Time
	var depositAmount, depositPaid float64
	var roomID *uuid.UUID
	var isVIP bool

	err := h.db.QueryRow(context.Background(), `
		SELECT status, guest_id, room_type_id, rate_plan_id, room_id, arrival_date, 
			nights, adults, children, deposit_amount, deposit_paid, currency, is_vip
		FROM reservations WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, id, propertyID).Scan(&currentStatus, &guestID, &roomTypeID, &ratePlanID, &roomID,
		&arrivalDate, &nights, &adults, &children, &depositAmount, &depositPaid, &currency, &isVIP)

	if err != nil {
		response.RespondError(w, "Reservation not found", http.StatusNotFound)
		return
	}

	if currentStatus != "confirmed" {
		response.RespondError(w, "Only confirmed reservations can be checked in", http.StatusConflict)
		return
	}

	// Determine room
	assignRoomID := req.RoomID
	if assignRoomID == "" && roomID != nil {
		assignRoomID = roomID.String()
	}
	if assignRoomID == "" {
		response.RespondError(w, "Room ID is required for check-in", http.StatusBadRequest)
		return
	}

	// Verify room is available
	var roomStatus string
	err = h.db.QueryRow(context.Background(),
		"SELECT status FROM rooms WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL",
		assignRoomID, propertyID).Scan(&roomStatus)
	if err != nil {
		response.RespondError(w, "Room not found", http.StatusNotFound)
		return
	}
	if roomStatus != "vacant_clean" && roomStatus != "inspected" && roomStatus != "reserved" {
		response.RespondError(w, fmt.Sprintf("Room is not available (status: %s)", roomStatus), http.StatusConflict)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	// Start transaction
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		response.RespondError(w, "Transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// Update reservation status
	_, err = tx.Exec(r.Context(), `
		UPDATE reservations SET status = 'checked_in', room_id = $3, updated_at = NOW()
		WHERE id = $1 AND property_id = $2
	`, id, propertyID, assignRoomID)
	if err != nil {
		response.RespondError(w, "Failed to update reservation", http.StatusInternalServerError)
		return
	}

	// Update room status
	_, err = tx.Exec(r.Context(), `
		UPDATE rooms SET status = 'occupied_dirty', status_changed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, assignRoomID)
	if err != nil {
		response.RespondError(w, "Failed to update room status", http.StatusInternalServerError)
		return
	}

	// Create guest stay
	stayID := uuid.New()
	expectedDeparture := arrivalDate.AddDate(0, 0, nights)
	_, err = tx.Exec(r.Context(), `
		INSERT INTO guest_stays (id, reservation_id, property_id, guest_id, room_id, actual_arrival, 
			expected_departure, status, is_vip, created_by)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, 'in_house', $7, $8)
	`, stayID, id, propertyID, guestID, assignRoomID, expectedDeparture.Format("2006-01-02"), isVIP, createdBy)
	if err != nil {
		response.RespondError(w, "Failed to create guest stay", http.StatusInternalServerError)
		return
	}

	// Create folio
	folioNumber := fmt.Sprintf("F-%s", generateConfirmationNumber()[:8])
	folioID := uuid.New()
	_, err = tx.Exec(r.Context(), `
		INSERT INTO folios (id, guest_stay_id, property_id, folio_number, type, status, balance, currency)
		VALUES ($1, $2, $3, $4, 'main', 'open', 0, $5)
	`, folioID, stayID, propertyID, folioNumber, currency)
	if err != nil {
		response.RespondError(w, "Failed to create folio", http.StatusInternalServerError)
		return
	}

	// Post initial charges (first night room charge + taxes)
	var basePrice float64
	h.db.QueryRow(r.Context(), "SELECT base_price FROM rate_plans WHERE id = $1", ratePlanID).Scan(&basePrice)

	// Get tax rate
	var taxRate float64
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(rt.tax_rate, 0) FROM rate_plans rp 
		LEFT JOIN tax_rates rt ON rp.tax_rate_id = rt.id 
		WHERE rp.id = $1
	`, ratePlanID).Scan(&taxRate)

	roomCharge := basePrice
	taxAmount := roomCharge * taxRate / 100

	// Post room charge
	_, err = tx.Exec(r.Context(), `
		INSERT INTO folio_lines (id, folio_id, property_id, type, charge_code, description, quantity, unit_price, amount, tax_amount, business_date, posted_by)
		VALUES ($1, $2, $3, 'debit', 'ROOM', 'Room charge - Night 1', 1, $4, $5, $6, CURRENT_DATE, $7)
	`, uuid.New(), folioID, propertyID, roomCharge, roomCharge, taxAmount, createdBy)
	if err != nil {
		response.RespondError(w, "Failed to post room charge", http.StatusInternalServerError)
		return
	}

	// M-FIN contract: recognize room revenue atomically with the charge
	// (Dr Guest Ledger, Cr Room Revenue + VAT). Money always posts through
	// the ledger — check-in rolls back if it cannot.
	if _, err := ledger.PostFolioCharge(r.Context(), tx, propertyID, "ROOM",
		roomCharge, taxAmount, "Room charge - Night 1", fmt.Sprintf("CHKIN-%s", id), eventActor(r)); err != nil {
		response.RespondError(w, "Ledger posting failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record status history
	tx.Exec(r.Context(), `
		INSERT INTO reservation_status_history (reservation_id, from_status, to_status, changed_by, created_at)
		VALUES ($1, 'confirmed', 'checked_in', $2, NOW())
	`, id, createdBy)

	// ADR-0006 outbox: publish inside the transaction so the events commit
	// with the state change; deliver only after commit.
	checkedInEv, err := h.emitDomainEvent(r.Context(), tx, propertyID, eventActor(r),
		events.ReservationCheckedIn, "reservations", id, map[string]interface{}{
			"propertyId":    propertyID,
			"guestId":       guestID,
			"reservationId": id,
		})
	if err != nil {
		response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
		return
	}
	folioEv, err := h.emitDomainEvent(r.Context(), tx, propertyID, eventActor(r),
		events.FolioOpened, "reservations", folioID.String(), map[string]interface{}{
			"propertyId":    propertyID,
			"guestId":       guestID,
			"reservationId": id,
			"folioId":       folioID.String(),
			"folioNumber":   folioNumber,
		})
	if err != nil {
		response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		response.RespondError(w, "Check-in transaction failed", http.StatusInternalServerError)
		return
	}
	h.ScheduleEvent(checkedInEv)
	h.ScheduleEvent(folioEv)

	response.RespondSuccess(w, map[string]interface{}{
		"reservation_id": id, "guest_stay_id": stayID, "folio_id": folioID,
		"folio_number": folioNumber, "room_id": assignRoomID, "status": "checked_in",
	}, "Guest checked in successfully")
}

// ── Check-out ─────────────────────────────────────────────

type CheckOutRequest struct {
	PaymentMethod string  `json:"payment_method"`
	PaymentAmount float64 `json:"payment_amount"`
	PaymentRef    string  `json:"payment_ref"`
	Notes         string  `json:"notes"`
}

func (h *Handler) CheckOut(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id") // reservation ID
	propertyID := r.Header.Get("X-Property-ID")

	var req CheckOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get reservation and stay info
	var currentStatus, roomID, guestID, stayID, folioID, folioNumber string
	var balance float64
	var nights int

	err := h.db.QueryRow(context.Background(), `
		SELECT res.status, res.room_id::text, res.guest_id::text, gs.id::text, f.id::text, f.folio_number, f.balance, res.nights
		FROM reservations res
		JOIN guest_stays gs ON gs.reservation_id = res.id
		JOIN folios f ON f.guest_stay_id = gs.id
		WHERE res.id = $1 AND res.property_id = $2 AND res.deleted_at IS NULL
		AND gs.status = 'in_house'
	`, id, propertyID).Scan(&currentStatus, &roomID, &guestID, &stayID, &folioID, &folioNumber, &balance, &nights)

	if err != nil {
		response.RespondError(w, "No active stay found for this reservation", http.StatusNotFound)
		return
	}

	if currentStatus != "checked_in" && currentStatus != "in_house" {
		response.RespondError(w, "Reservation is not in-house", http.StatusConflict)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	// Start transaction
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		response.RespondError(w, "Transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// Post final payment if provided
	paymentEv := ""
	if req.PaymentAmount > 0 {
		method := req.PaymentMethod
		if method == "" {
			method = "cash"
		}
		reference := req.PaymentRef
		if reference == "" {
			reference = fmt.Sprintf("CO-%s", folioNumber)
		}

		_, err := tx.Exec(r.Context(), `
			INSERT INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date, posted_by)
			VALUES ($1, $2, $3, 'credit', 'PAYMENT', $4 || ' payment', $5, CURRENT_DATE, $6)
		`, uuid.New(), folioID, propertyID, method, req.PaymentAmount, createdBy)
		if err != nil {
			response.RespondError(w, "Failed to post checkout payment", http.StatusInternalServerError)
			return
		}
		// ADR-0006: the settlement travels the contract bus atomically with
		// the credit — M-FIN posts Dr cash/bank, Cr Guest Ledger.
		paymentEv, err = h.emitDomainEvent(r.Context(), tx, propertyID, eventActor(r),
			events.PaymentCaptured, "frontdesk", id, map[string]interface{}{
				"propertyId":  propertyID,
				"date":        time.Now().Format("2006-01-02"),
				"method":      method,
				"amountKobo":  ledger.NairaToKobo(req.PaymentAmount),
				"reference":   reference,
				"description": fmt.Sprintf("Check-out settlement — folio %s", folioNumber),
			})
		if err != nil {
			response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
			return
		}
	}

	// Close folio
	tx.Exec(r.Context(), `
		UPDATE folios SET status = 'closed', closed_at = NOW(), updated_at = NOW() WHERE id = $1
	`, folioID)

	// Update guest stay
	tx.Exec(r.Context(), `
		UPDATE guest_stays SET actual_departure = NOW(), status = 'checked_out', updated_at = NOW()
		WHERE id = $1
	`, stayID)

	// Update reservation status
	tx.Exec(r.Context(), `
		UPDATE reservations SET status = 'checked_out', updated_at = NOW() WHERE id = $1
	`, id)

	// Update room status to vacant_dirty
	roomUUID, _ := uuid.Parse(roomID)
	tx.Exec(r.Context(), `
		UPDATE rooms SET status = 'vacant_dirty', status_changed_at = NOW(), updated_at = NOW() WHERE id = $1
	`, roomUUID)

	// Record status history
	tx.Exec(r.Context(), `
		INSERT INTO reservation_status_history (reservation_id, from_status, to_status, changed_by, created_at)
		VALUES ($1, 'checked_in', 'checked_out', $2, NOW())
	`, id, createdBy)

	// Record actual departure in guest stay
	tx.Exec(r.Context(), `
		UPDATE guest_stays SET actual_departure = NOW(), updated_at = NOW()
		WHERE reservation_id = $1
	`, id)

	// Total debited on the folio — M-CRM accrues points from this value.
	var totalCharges float64
	if err := tx.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(amount), 0) FROM folio_lines
		WHERE folio_id = $1 AND type = 'debit' AND is_voided = FALSE
	`, folioID).Scan(&totalCharges); err != nil {
		response.RespondError(w, "Failed to total folio charges", http.StatusInternalServerError)
		return
	}

	checkedOutEv, err := h.emitDomainEvent(r.Context(), tx, propertyID, eventActor(r),
		events.ReservationCheckedOut, "reservations", id, map[string]interface{}{
			"propertyId":    propertyID,
			"guestId":       guestID,
			"reservationId": id,
			"nights":        nights,
			"totalKobo":     ledger.NairaToKobo(totalCharges),
			"folioCode":     folioNumber,
		})
	if err != nil {
		response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		response.RespondError(w, "Check-out transaction failed", http.StatusInternalServerError)
		return
	}
	h.ScheduleEvent(paymentEv)
	h.ScheduleEvent(checkedOutEv)

	response.RespondSuccess(w, map[string]interface{}{
		"reservation_id": id, "folio_number": folioNumber, "balance": balance,
		"status": "checked_out",
	}, "Guest checked out successfully")
}

// ── Folio ─────────────────────────────────────────────────

func (h *Handler) GetFolio(w http.ResponseWriter, r *http.Request) {
	stayID := chi.URLParam(r, "stayId")
	propertyID := r.Header.Get("X-Property-ID")

	// Get folio
	var folioID, folioNumber, folioType, folioStatus, currency string
	var balance float64

	err := h.db.QueryRow(context.Background(), `
		SELECT id, folio_number, type, status, balance, currency
		FROM folios WHERE guest_stay_id = $1 AND property_id = $2
	`, stayID, propertyID).Scan(&folioID, &folioNumber, &folioType, &folioStatus, &balance)

	if err != nil {
		response.RespondError(w, "Folio not found", http.StatusNotFound)
		return
	}

	// Get folio lines
	rows, err := h.db.Query(context.Background(), `
		SELECT fl.id, fl.type, fl.charge_code, fl.description, fl.quantity, fl.unit_price, 
			fl.amount, fl.tax_amount, fl.business_date, fl.created_at,
			COALESCE(u.name, 'System') as posted_by_name
		FROM folio_lines fl
		LEFT JOIN users u ON fl.posted_by = u.id
		WHERE fl.folio_id = $1 AND fl.is_voided = FALSE
		ORDER BY fl.created_at
	`, folioID)
	if err != nil {
		response.RespondError(w, "Failed to query folio lines", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lines []map[string]interface{}
	var totalDebits, totalCredits float64
	for rows.Next() {
		var lineID uuid.UUID
		var lineType, chargeCode, description string
		var quantity, unitPrice, amount, taxAmount float64
		var businessDate, createdAt interface{}
		var postedByName string

		rows.Scan(&lineID, &lineType, &chargeCode, &description, &quantity, &unitPrice,
			&amount, &taxAmount, &businessDate, &createdAt, &postedByName)

		line := map[string]interface{}{
			"id": lineID, "type": lineType, "charge_code": chargeCode,
			"description": description, "quantity": quantity, "unit_price": unitPrice,
			"amount": amount, "tax_amount": taxAmount, "posted_by": postedByName,
		}
		lines = append(lines, line)

		if lineType == "debit" {
			totalDebits += amount
		} else {
			totalCredits += amount
		}
	}

	if lines == nil {
		lines = []map[string]interface{}{}
	}

	response.RespondSuccess(w, map[string]interface{}{
		"folio_id": folioID, "folio_number": folioNumber, "type": folioType,
		"status": folioStatus, "balance": balance, "currency": currency,
		"total_debits": totalDebits, "total_credits": totalCredits,
		"lines": lines,
	}, "Folio retrieved successfully")
}

type PostChargeRequest struct {
	Type        string  `json:"type"`
	ChargeCode  string  `json:"charge_code"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	TaxAmount   float64 `json:"tax_amount"`
}

func (h *Handler) PostCharge(w http.ResponseWriter, r *http.Request) {
	folioID := chi.URLParam(r, "folioId")
	propertyID := r.Header.Get("X-Property-ID")

	var req PostChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChargeCode == "" || req.Description == "" {
		response.RespondError(w, "charge_code and description are required", http.StatusBadRequest)
		return
	}

	if req.Quantity < 1 {
		req.Quantity = 1
	}

	lineType := req.Type
	if lineType == "" {
		lineType = "debit"
	}

	amount := req.Amount
	if amount == 0 {
		amount = req.Quantity * req.UnitPrice
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	lineID := uuid.New()

	// Transactional: the folio line, its revenue recognition and its outbox
	// event commit together — or not at all.
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		response.RespondError(w, "Transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(), `
		INSERT INTO folio_lines (id, folio_id, property_id, type, charge_code, description, quantity, unit_price, amount, tax_amount, business_date, posted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_DATE, $11)
	`, lineID, folioID, propertyID, lineType, req.ChargeCode, req.Description,
		req.Quantity, req.UnitPrice, amount, req.TaxAmount, createdBy)

	if err != nil {
		response.RespondError(w, "Failed to post charge", http.StatusInternalServerError)
		return
	}

	// M-FIN contract: recognize revenue for debit lines (Dr Guest Ledger,
	// Cr revenue by charge code, +Cr VAT). Credit lines are settlements, not
	// revenue — cash arrives through payment.captured instead.
	if lineType == "debit" && amount+req.TaxAmount > 0 {
		if _, err := ledger.PostFolioCharge(r.Context(), tx, propertyID, req.ChargeCode,
			amount, req.TaxAmount, req.Description, fmt.Sprintf("FOLIO-%s", folioID), eventActor(r)); err != nil {
			response.RespondError(w, "Ledger posting failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// ADR-0006: record the charge on the contract bus atomically with it.
	chargeEv, err := h.emitDomainEvent(r.Context(), tx, propertyID, eventActor(r),
		events.FolioChargePosted, "folio", lineID.String(), map[string]interface{}{
			"propertyId": propertyID,
			"folioId":    folioID,
			"chargeCode": req.ChargeCode,
			"type":       lineType,
			"amountKobo": ledger.NairaToKobo(amount),
			"taxKobo":    ledger.NairaToKobo(req.TaxAmount),
		})
	if err != nil {
		response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		response.RespondError(w, "Failed to post charge", http.StatusInternalServerError)
		return
	}
	h.ScheduleEvent(chargeEv)

	response.RespondSuccess(w, map[string]interface{}{
		"id": lineID, "amount": amount, "type": lineType,
	}, "Charge posted successfully", http.StatusCreated)
}

// ── Front Desk Dashboard ──────────────────────────────────

func (h *Handler) FrontDeskDashboard(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	today := time.Now().Format("2006-01-02")

	var arrivals, departures, inHouse, availableRooms, occupiedRooms int

	// Expected arrivals (confirmed + today)
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM reservations 
		WHERE property_id = $1 AND status IN ('confirmed') AND arrival_date = $2 AND deleted_at IS NULL
	`, propertyID, today).Scan(&arrivals)

	// Expected departures (in-house + today)
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM reservations 
		WHERE property_id = $1 AND status IN ('checked_in', 'in_house') AND departure_date = $2 AND deleted_at IS NULL
	`, propertyID, today).Scan(&departures)

	// In-house guests
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM reservations 
		WHERE property_id = $1 AND status IN ('checked_in', 'in_house') AND deleted_at IS NULL
	`, propertyID).Scan(&inHouse)

	// Available rooms
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM rooms 
		WHERE property_id = $1 AND deleted_at IS NULL 
		AND status IN ('vacant_clean', 'inspected')
	`, propertyID).Scan(&availableRooms)

	// Occupied rooms
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM rooms 
		WHERE property_id = $1 AND deleted_at IS NULL 
		AND status IN ('occupied_clean', 'occupied_dirty')
	`, propertyID).Scan(&occupiedRooms)

	// Room status breakdown
	type roomStatusCount struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	statusRows, _ := h.db.Query(r.Context(), `
		SELECT status, COUNT(*) as count FROM rooms 
		WHERE property_id = $1 AND deleted_at IS NULL
		GROUP BY status ORDER BY status
	`, propertyID)
	defer statusRows.Close()

	var statusBreakdown []roomStatusCount
	for statusRows.Next() {
		var sc roomStatusCount
		statusRows.Scan(&sc.Status, &sc.Count)
		statusBreakdown = append(statusBreakdown, sc)
	}

	// VIP arrivals today
	var vipArrivals int
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM reservations 
		WHERE property_id = $1 AND status IN ('confirmed') AND arrival_date = $2 
		AND is_vip = TRUE AND deleted_at IS NULL
	`, propertyID, today).Scan(&vipArrivals)

	// Outstanding balances
	var outstandingBalance float64
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(f.balance), 0) FROM folios f
		JOIN guest_stays gs ON f.guest_stay_id = gs.id
		WHERE gs.property_id = $1 AND f.status = 'open' AND f.balance > 0
	`, propertyID).Scan(&outstandingBalance)

	response.RespondSuccess(w, map[string]interface{}{
		"arrivals":              arrivals,
		"departures":            departures,
		"in_house":              inHouse,
		"available_rooms":       availableRooms,
		"occupied_rooms":        occupiedRooms,
		"vip_arrivals":          vipArrivals,
		"outstanding_balance":   outstandingBalance,
		"room_status_breakdown": statusBreakdown,
	}, "Front desk dashboard retrieved successfully")
}

// ── Properties ─────────────────────────────────────────────

func (h *Handler) ListProperties(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, trading_name, code, city, country_id, phone, email, timezone, status, created_at
		FROM properties WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		response.RespondError(w, "Failed to query properties", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var properties []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, code, status string
		var tradingName, city *string
		var countryID *uuid.UUID
		var phone, email, timezone *string
		var createdAt interface{}

		rows.Scan(&id, &name, &tradingName, &code, &city, &countryID, &phone, &email, &timezone, &status, &createdAt)

		p := map[string]interface{}{
			"id": id, "name": name, "code": code, "status": status,
		}
		if tradingName != nil {
			p["trading_name"] = *tradingName
		}
		if city != nil {
			p["city"] = *city
		}
		if countryID != nil {
			p["country_id"] = *countryID
		}
		if phone != nil {
			p["phone"] = *phone
		}
		if email != nil {
			p["email"] = *email
		}
		if timezone != nil {
			p["timezone"] = *timezone
		}
		properties = append(properties, p)
	}

	if properties == nil {
		properties = []map[string]interface{}{}
	}
	response.RespondSuccess(w, properties, "Properties retrieved successfully")
}

func (h *Handler) CreateProperty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CompanyID   string `json:"company_id"`
		Name        string `json:"name"`
		TradingName string `json:"trading_name"`
		Code        string `json:"code"`
		Address     string `json:"address"`
		City        string `json:"city"`
		State       string `json:"state"`
		Phone       string `json:"phone"`
		Email       string `json:"email"`
		Timezone    string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.CompanyID == "" || req.Name == "" || req.Code == "" {
		response.RespondError(w, "company_id, name, and code are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO properties (id, company_id, name, trading_name, code, address, city, state, phone, email, timezone, status)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), $11, 'active')
	`, id, req.CompanyID, req.Name, req.TradingName, req.Code, req.Address, req.City, req.State,
		req.Phone, req.Email, req.Timezone)

	if err != nil {
		response.RespondError(w, "Failed to create property", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Property created successfully", http.StatusCreated)
}

func (h *Handler) GetProperty(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var name, code, status string
	var tradingName, address, city, state, phone, email, timezone *string
	var countryID *uuid.UUID
	var companyId uuid.UUID

	err := h.db.QueryRow(context.Background(), `
		SELECT company_id, name, trading_name, code, address, city, state, country_id, phone, email, timezone, status
		FROM properties WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&companyId, &name, &tradingName, &code, &address, &city, &state, &countryID, &phone, &email, &timezone, &status)

	if err != nil {
		response.RespondError(w, "Property not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id": id, "company_id": companyId, "name": name, "code": code, "status": status,
	}
	if tradingName != nil {
		result["trading_name"] = *tradingName
	}
	if address != nil {
		result["address"] = *address
	}
	if city != nil {
		result["city"] = *city
	}
	if state != nil {
		result["state"] = *state
	}
	if countryID != nil {
		result["country_id"] = *countryID
	}
	if phone != nil {
		result["phone"] = *phone
	}
	if email != nil {
		result["email"] = *email
	}
	if timezone != nil {
		result["timezone"] = *timezone
	}

	response.RespondSuccess(w, result, "Property retrieved successfully")
}

func (h *Handler) UpdateProperty(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Name        string `json:"name"`
		TradingName string `json:"trading_name"`
		Address     string `json:"address"`
		City        string `json:"city"`
		State       string `json:"state"`
		Phone       string `json:"phone"`
		Email       string `json:"email"`
		Timezone    string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE properties SET
			name = COALESCE(NULLIF($2, ''), name),
			trading_name = NULLIF($3, ''),
			address = NULLIF($4, ''),
			city = NULLIF($5, ''),
			state = NULLIF($6, ''),
			phone = NULLIF($7, ''),
			email = NULLIF($8, ''),
			timezone = NULLIF($9, ''),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, req.Name, req.TradingName, req.Address, req.City, req.State, req.Phone, req.Email, req.Timezone)

	if err != nil {
		response.RespondError(w, "Failed to update property", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Property updated successfully")
}

// ── Audit Logs ─────────────────────────────────────────────

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	module := r.URL.Query().Get("module")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50
	offset := (page - 1) * perPage

	query := `SELECT id, user_id, module, action, auditable_type, auditable_id, created_at
		FROM audit_logs WHERE property_id = $1`
	args := []interface{}{propertyID}
	argIdx := 2

	if module != "" {
		query += ` AND module = $` + strconv.Itoa(argIdx)
		args = append(args, module)
		argIdx++
	}

	var total int64
	h.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM audit_logs WHERE property_id = $1", propertyID).Scan(&total)

	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query audit logs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var userID *uuid.UUID
		var module, action, auditableType string
		var auditableID uuid.UUID
		var createdAt time.Time
		rows.Scan(&id, &userID, &module, &action, &auditableType, &auditableID, &createdAt)

		l := map[string]interface{}{
			"id": id, "module": module, "action": action,
			"auditable_type": auditableType, "auditable_id": auditableID,
			"created_at": createdAt.Format(time.RFC3339),
		}
		if userID != nil {
			l["user_id"] = *userID
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	response.RespondPaginated(w, logs, total, page, perPage)
}

// ── Config ─────────────────────────────────────────────────

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	response.RespondSuccess(w, map[string]interface{}{
		"version":  "0.1.0",
		"mode":     "on-premise",
		"database": "postgresql",
	}, "Configuration retrieved successfully")
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	response.RespondSuccess(w, nil, "Configuration updated successfully")
}

// ── Helpers ────────────────────────────────────────────────

func canTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func uuidOrEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return u
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return sql.NullString{String: s, Valid: true}
}
