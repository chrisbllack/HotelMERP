package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CreateGuestRequest struct {
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	Nationality      string `json:"nationality"`
	CountryID        string `json:"country_id"`
	IDType           string `json:"id_type"`
	IDNumber         string `json:"id_number"`
	IDExpiry         string `json:"id_expiry"`
	DateOfBirth      string `json:"date_of_birth"`
	Gender           string `json:"gender"`
	Address          string `json:"address"`
	City             string `json:"city"`
	Company          string `json:"company"`
	Notes            string `json:"notes"`
	MarketingConsent bool   `json:"marketing_consent"`
}

func (h *Handler) ListGuests(w http.ResponseWriter, r *http.Request) {
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

	query := `SELECT id, first_name, last_name, email, phone, nationality, id_type, id_number, 
		date_of_birth, gender, company, city, created_at
		FROM guests WHERE deleted_at IS NULL`
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		query += ` AND (first_name ILIKE $` + strconv.Itoa(argIdx) + ` OR last_name ILIKE $` + strconv.Itoa(argIdx) +
			` OR email ILIKE $` + strconv.Itoa(argIdx) + ` OR phone ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM guests WHERE deleted_at IS NULL"
	var total int64
	if search != "" {
		h.db.QueryRow(context.Background(), countQuery+" AND (first_name ILIKE $1 OR last_name ILIKE $1 OR email ILIKE $1 OR phone ILIKE $1)",
			"%"+search+"%").Scan(&total)
	} else {
		h.db.QueryRow(context.Background(), countQuery).Scan(&total)
	}

	query += ` ORDER BY last_name, first_name LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query guests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var guests []map[string]interface{}
	for rows.Next() {
		var id string
		var firstName, lastName string
		var email, phone, nationality, idType, idNumber, dob, gender, company, city *string
		var createdAt interface{}

		if err := rows.Scan(&id, &firstName, &lastName, &email, &phone, &nationality, &idType, &idNumber,
			&dob, &gender, &company, &city, &createdAt); err != nil {
			continue
		}

		g := map[string]interface{}{
			"id": id, "first_name": firstName, "last_name": lastName,
		}
		if email != nil {
			g["email"] = *email
		}
		if phone != nil {
			g["phone"] = *phone
		}
		if nationality != nil {
			g["nationality"] = *nationality
		}
		if idType != nil {
			g["id_type"] = *idType
		}
		if idNumber != nil {
			g["id_number"] = *idNumber
		}
		if dob != nil {
			g["date_of_birth"] = *dob
		}
		if gender != nil {
			g["gender"] = *gender
		}
		if company != nil {
			g["company"] = *company
		}
		if city != nil {
			g["city"] = *city
		}
		guests = append(guests, g)
	}

	if guests == nil {
		guests = []map[string]interface{}{}
	}
	response.RespondPaginated(w, guests, total, page, perPage)
}

func (h *Handler) CreateGuest(w http.ResponseWriter, r *http.Request) {
	var req CreateGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FirstName == "" || req.LastName == "" {
		response.RespondError(w, "First name and last name are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	var countryID *string
	if req.CountryID != "" {
		countryID = &req.CountryID
	}

	_, err := h.db.Exec(context.Background(), `
		INSERT INTO guests (id, first_name, last_name, email, phone, nationality, country_id,
			id_type, id_number, id_expiry, date_of_birth, gender, address, city, company, notes, marketing_consent)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7,
			NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, '')::date, NULLIF($11, '')::date, 
			NULLIF($12, ''), NULLIF($13, ''), NULLIF($14, ''), NULLIF($15, ''), NULLIF($16, ''), $17)
	`, id, req.FirstName, req.LastName, req.Email, req.Phone, req.Nationality, countryID,
		req.IDType, req.IDNumber, req.IDExpiry, req.DateOfBirth, req.Gender, req.Address,
		req.City, req.Company, req.Notes, req.MarketingConsent)

	if err != nil {
		response.RespondError(w, "Failed to create guest", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{
		"id": id, "first_name": req.FirstName, "last_name": req.LastName,
	}, "Guest created successfully", http.StatusCreated)
}

func (h *Handler) GetGuest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var firstName, lastName string
	var email, phone, nationality, idType, idNumber, dob, gender, company, city, address, notes *string
	var countryID *uuid.UUID
	var marketingConsent, blacklisted bool

	err := h.db.QueryRow(context.Background(), `
		SELECT first_name, last_name, email, phone, nationality, country_id, id_type, id_number,
			date_of_birth, gender, address, city, company, notes, marketing_consent, blacklisted
		FROM guests WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&firstName, &lastName, &email, &phone, &nationality, &countryID, &idType, &idNumber,
		&dob, &gender, &address, &city, &company, &notes, &marketingConsent, &blacklisted)

	if err != nil {
		response.RespondError(w, "Guest not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id": id, "first_name": firstName, "last_name": lastName,
		"marketing_consent": marketingConsent, "blacklisted": blacklisted,
	}
	if email != nil {
		result["email"] = *email
	}
	if phone != nil {
		result["phone"] = *phone
	}
	if nationality != nil {
		result["nationality"] = *nationality
	}
	if countryID != nil {
		result["country_id"] = *countryID
	}
	if idType != nil {
		result["id_type"] = *idType
	}
	if idNumber != nil {
		result["id_number"] = *idNumber
	}
	if dob != nil {
		result["date_of_birth"] = *dob
	}
	if gender != nil {
		result["gender"] = *gender
	}
	if address != nil {
		result["address"] = *address
	}
	if city != nil {
		result["city"] = *city
	}
	if company != nil {
		result["company"] = *company
	}
	if notes != nil {
		result["notes"] = *notes
	}

	response.RespondSuccess(w, result, "Guest retrieved successfully")
}

func (h *Handler) UpdateGuest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CreateGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE guests SET
			first_name = COALESCE(NULLIF($2, ''), first_name),
			last_name = COALESCE(NULLIF($3, ''), last_name),
			email = NULLIF($4, ''),
			phone = NULLIF($5, ''),
			nationality = NULLIF($6, ''),
			country_id = NULLIF($7, '')::uuid,
			id_type = NULLIF($8, ''),
			id_number = NULLIF($9, ''),
			id_expiry = NULLIF($10, '')::date,
			date_of_birth = NULLIF($11, '')::date,
			gender = NULLIF($12, ''),
			address = NULLIF($13, ''),
			city = NULLIF($14, ''),
			company = NULLIF($15, ''),
			notes = NULLIF($16, ''),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, req.FirstName, req.LastName, req.Email, req.Phone, req.Nationality, req.CountryID,
		req.IDType, req.IDNumber, req.IDExpiry, req.DateOfBirth, req.Gender, req.Address,
		req.City, req.Company, req.Notes)

	if err != nil {
		response.RespondError(w, "Failed to update guest", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Guest updated successfully")
}

func (h *Handler) DeleteGuest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := h.db.Exec(context.Background(), "UPDATE guests SET deleted_at = NOW() WHERE id = $1", id)
	if err != nil {
		response.RespondError(w, "Failed to delete guest", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, nil, "Guest deleted successfully")
}

// GuestSearch is a quick search endpoint for autocomplete
func (h *Handler) GuestSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		response.RespondSuccess(w, []interface{}{}, "Search query too short")
		return
	}

	rows, err := h.db.Query(context.Background(), `
		SELECT id, first_name, last_name, email, phone, id_type, id_number
		FROM guests WHERE deleted_at IS NULL AND blacklisted = FALSE
			AND (first_name ILIKE $1 OR last_name ILIKE $1 OR email ILIKE $1 OR phone ILIKE $1 OR id_number ILIKE $1)
		ORDER BY last_name LIMIT 20
	`, "%"+q+"%")

	if err != nil {
		response.RespondError(w, "Search failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id string
		var firstName, lastName string
		var email, phone, idType, idNumber *string
		rows.Scan(&id, &firstName, &lastName, &email, &phone, &idType, &idNumber)

		g := map[string]interface{}{
			"id": id, "first_name": firstName, "last_name": lastName,
			"full_name": firstName + " " + lastName,
		}
		if email != nil {
			g["email"] = *email
		}
		if phone != nil {
			g["phone"] = *phone
		}
		results = append(results, g)
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	response.RespondSuccess(w, results, "Search completed")
}
