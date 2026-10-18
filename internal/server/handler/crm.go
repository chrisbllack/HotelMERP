package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── Guest Profile (CRM) ──

type GuestProfile struct {
	ID               string                 `json:"id"`
	GuestID          string                 `json:"guest_id"`
	FirstName        string                 `json:"first_name"`
	LastName         string                 `json:"last_name"`
	Email            string                 `json:"email"`
	Phone            string                 `json:"phone"`
	Nationality      string                 `json:"nationality"`
	TotalStays       int                    `json:"total_stays"`
	TotalSpent       float64                `json:"total_spent"`
	AverageRating    float64                `json:"average_rating"`
	LastStayDate     string                 `json:"last_stay_date"`
	LoyaltyTier      string                 `json:"loyalty_tier"`
	LoyaltyPoints    int                    `json:"loyalty_points"`
	Preferences      map[string]interface{} `json:"preferences"`
	MarketingConsent bool                   `json:"marketing_consent"`
	Tags             []string               `json:"tags"`
}

// GetGuestProfile returns the full CRM profile for a guest. Loyalty figures
// are read from the M-CRM loyalty engine (loyalty_members / loyalty_tiers),
// which is the source of truth the checkout subscriber writes to; the
// lifetime value is the mirrored kobo accrual.
func (h *Handler) GetGuestProfile(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "id")
	ctx := r.Context()

	// Get guest basic info
	var profile GuestProfile
	err := h.db.QueryRow(ctx, `
		SELECT g.id, g.first_name, g.last_name, COALESCE(g.email, ''), COALESCE(g.phone, ''),
		       COALESCE(g.nationality, ''),
		       COALESCE((SELECT COUNT(*) FROM reservations WHERE guest_id = g.id), 0),
		       COALESCE((SELECT lm.lifetime_value_kobo FROM loyalty_members lm
		                 WHERE lm.guest_id = g.id AND lm.property_id = $2
		                 ORDER BY lm.last_activity DESC LIMIT 1), 0) / 100.0,
		       COALESCE((SELECT MAX(res.departure_date)::text FROM reservations res WHERE res.guest_id = g.id), ''),
		       COALESCE((SELECT t.name FROM loyalty_members lm
		                 LEFT JOIN loyalty_tiers t ON t.id = lm.tier_id
		                 WHERE lm.guest_id = g.id AND lm.property_id = $2
		                 ORDER BY lm.last_activity DESC LIMIT 1), 'member'),
		       COALESCE((SELECT lm.total_points FROM loyalty_members lm
		                 WHERE lm.guest_id = g.id AND lm.property_id = $2
		                 ORDER BY lm.last_activity DESC LIMIT 1), 0),
		       COALESCE(g.marketing_consent, false)
		FROM guests g WHERE g.id = $1
	`, guestID, ctx.Value("property_id")).Scan(
		&profile.ID, &profile.FirstName, &profile.LastName, &profile.Email,
		&profile.Phone, &profile.Nationality, &profile.TotalStays, &profile.TotalSpent,
		&profile.LastStayDate, &profile.LoyaltyTier, &profile.LoyaltyPoints,
		&profile.MarketingConsent,
	)
	if err != nil {
		jsonError(w, "Guest profile not found", http.StatusNotFound)
		return
	}
	profile.GuestID = profile.ID

	// Get stay history
	rows, err := h.db.Query(ctx, `
		SELECT res.id, res.arrival_date::text, res.departure_date::text,
		       COALESCE(rt.name, ''), res.total_estimated, res.status
		FROM reservations res
		LEFT JOIN room_types rt ON rt.id = res.room_type_id
		WHERE res.guest_id = $1
		ORDER BY res.arrival_date DESC LIMIT 20
	`, guestID)
	if err == nil {
		defer rows.Close()
		type StayHistory struct {
			ReservationID string  `json:"reservation_id"`
			CheckIn       string  `json:"check_in"`
			CheckOut      string  `json:"check_out"`
			RoomType      string  `json:"room_type"`
			TotalSpent    float64 `json:"total_spent"`
			Status        string  `json:"status"`
		}
		var history []StayHistory
		for rows.Next() {
			var s StayHistory
			if err := rows.Scan(&s.ReservationID, &s.CheckIn, &s.CheckOut, &s.RoomType, &s.TotalSpent, &s.Status); err != nil {
				continue
			}
			history = append(history, s)
		}
		profile.Preferences = map[string]interface{}{
			"stay_history": history,
		}
	}

	jsonSuccess(w, profile)
}

// UpdateGuestPreferences updates guest CRM preferences
func (h *Handler) UpdateGuestPreferences(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "id")
	var req struct {
		PreferredRoomType string `json:"preferred_room_type"`
		PreferredFloor    string `json:"preferred_floor"`
		PreferredBed      string `json:"preferred_bed"`
		FoodPreferences   string `json:"food_preferences"`
		Allergies         string `json:"allergies"`
		MarketingConsent  *bool  `json:"marketing_consent"`
		SpecialOccasions  string `json:"special_occasions"`
		Notes             string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		UPDATE guests SET
			preferred_room_type = COALESCE(NULLIF($1, ''), preferred_room_type),
			preferred_floor = COALESCE(NULLIF($2, ''), preferred_floor),
			preferred_bed = COALESCE(NULLIF($3, ''), preferred_bed),
			food_preferences = COALESCE(NULLIF($4, ''), food_preferences),
			allergies = COALESCE(NULLIF($5, ''), allergies),
			special_occasions = COALESCE(NULLIF($6, ''), special_occasions),
			notes = COALESCE(NULLIF($7, ''), notes),
			updated_at = NOW()
		WHERE id = $8
	`, req.PreferredRoomType, req.PreferredFloor, req.PreferredBed,
		req.FoodPreferences, req.Allergies, req.SpecialOccasions, req.Notes,
		guestID)
	if err != nil {
		jsonError(w, "Failed to update preferences", http.StatusInternalServerError)
		return
	}

	if req.MarketingConsent != nil {
		_, _ = h.db.Exec(ctx, `UPDATE guests SET marketing_consent = $1 WHERE id = $2`, *req.MarketingConsent, guestID)
	}

	jsonSuccess(w, map[string]interface{}{"message": "Preferences updated"})
}

// ── Loyalty Program ──

// GetLoyaltyTiers returns configured loyalty tiers
func (h *Handler) GetLoyaltyTiers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, name, min_points, discount_percent, COALESCE(benefits, ''), COALESCE(color, '')
		FROM loyalty_tiers WHERE property_id = $1 ORDER BY min_points
	`, ctx.Value("property_id"))
	if err != nil {
		jsonError(w, "Failed to load loyalty tiers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Tier struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		MinPoints       int     `json:"min_points"`
		DiscountPercent float64 `json:"discount_percent"`
		Benefits        string  `json:"benefits"`
		Color           string  `json:"color"`
	}

	var tiers []Tier
	for rows.Next() {
		var t Tier
		if err := rows.Scan(&t.ID, &t.Name, &t.MinPoints, &t.DiscountPercent, &t.Benefits, &t.Color); err != nil {
			continue
		}
		tiers = append(tiers, t)
	}

	// Default tiers if none configured
	if len(tiers) == 0 {
		tiers = []Tier{
			{"1", "Member", 0, 0, "Welcome drink, Late checkout (subject to availability)", "#6B7280"},
			{"2", "Silver", 500, 5, "10% F&B discount, Priority check-in, Room upgrade (subject to availability)", "#9CA3AF"},
			{"3", "Gold", 1500, 10, "15% F&B discount, Free breakfast, Guaranteed late checkout, Complimentary parking", "#F59E0B"},
			{"4", "Platinum", 5000, 15, "20% F&B discount, Free spa access, Suite upgrade, Personal concierge, Free laundry", "#7C3AED"},
			{"5", "VIP", 10000, 20, "All Platinum benefits + Complimentary airport transfer, Private dining, Personal butler", "#EC4899"},
		}
	}

	jsonSuccess(w, tiers)
}

// AwardLoyaltyPoints adds points to a guest. Points post through the M-CRM
// loyalty engine (loyalty_transactions is the audit trail, loyalty_members
// holds the balance) and the denormalized guests mirror is kept in sync so
// legacy readers see the same tier.
func (h *Handler) AwardLoyaltyPoints(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID string  `json:"guest_id"`
		Points  int     `json:"points"`
		Reason  string  `json:"reason"`
		Amount  float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.GuestID == "" || req.Points == 0 {
		jsonError(w, "guest_id and a non-zero points value are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	prop := getPropertyID2(r)

	memberID, _, err := h.loadOrCreateLoyaltyMember(ctx, prop, req.GuestID)
	if err != nil {
		jsonError(w, "Failed to resolve loyalty member", http.StatusInternalServerError)
		return
	}

	// Engine: transaction (audit) then member balance.
	if _, err := h.db.Exec(ctx, `
		INSERT INTO loyalty_transactions (id, member_id, type, points, description, reference_id, created_at)
		VALUES ($1, $2, 'adjustment', $3, $4, NULL, NOW())
	`, uuid.New().String(), memberID, req.Points, req.Reason); err != nil {
		jsonError(w, "Failed to record loyalty transaction", http.StatusInternalServerError)
		return
	}
	if _, err := h.db.Exec(ctx, `
		UPDATE loyalty_members SET total_points = total_points + $1,
			lifetime_points = lifetime_points + $1, last_activity = NOW()
		WHERE id = $2
	`, req.Points, memberID); err != nil {
		jsonError(w, "Failed to award points", http.StatusInternalServerError)
		return
	}

	// Denormalized mirror + tier recompute on the guest record.
	if _, err := h.db.Exec(ctx, `
		UPDATE guests SET loyalty_points = COALESCE(loyalty_points, 0) + $1,
			loyalty_tier = CASE
				WHEN COALESCE(loyalty_points, 0) + $1 >= 10000 THEN 'vip'
				WHEN COALESCE(loyalty_points, 0) + $1 >= 5000 THEN 'platinum'
				WHEN COALESCE(loyalty_points, 0) + $1 >= 1500 THEN 'gold'
				WHEN COALESCE(loyalty_points, 0) + $1 >= 500 THEN 'silver'
				ELSE 'member'
			END,
			updated_at = NOW()
		WHERE id = $2
	`, req.Points, req.GuestID); err != nil {
		jsonError(w, "Failed to award points", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{
		"message":   "Points awarded",
		"member_id": memberID,
		"points":    req.Points,
		"reason":    req.Reason,
	})
}

// ── Feedback ──

// SubmitFeedback records guest feedback
func (h *Handler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID        string `json:"guest_id"`
		ReservationID  string `json:"reservation_id"`
		OverallRating  int    `json:"overall_rating"`
		RoomRating     int    `json:"room_rating"`
		ServiceRating  int    `json:"service_rating"`
		FoodRating     int    `json:"food_rating"`
		Comments       string `json:"comments"`
		WouldRecommend bool   `json:"would_recommend"`
		Issues         string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.OverallRating < 1 || req.OverallRating > 5 {
		jsonError(w, "Rating must be between 1 and 5", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		INSERT INTO guest_feedback (
			guest_id, reservation_id, overall_rating, room_rating,
			service_rating, food_rating, comments, would_recommend,
			issues, property_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`, req.GuestID, req.ReservationID, req.OverallRating, req.RoomRating,
		req.ServiceRating, req.FoodRating, req.Comments, req.WouldRecommend,
		req.Issues, ctx.Value("property_id"))
	if err != nil {
		jsonError(w, "Failed to submit feedback", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{
		"message": "Thank you for your feedback!",
		"rating":  req.OverallRating,
	})
}

// ListFeedback returns all guest feedback
func (h *Handler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT gf.id, COALESCE(g.first_name || ' ' || g.last_name, 'Anonymous'),
		       gf.overall_rating, gf.comments, gf.would_recommend, gf.created_at::text
		FROM guest_feedback gf
		LEFT JOIN guests g ON g.id = gf.guest_id
		WHERE gf.property_id = $1
		ORDER BY gf.created_at DESC LIMIT 50
	`, ctx.Value("property_id"))
	if err != nil {
		jsonError(w, "Failed to load feedback", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type FeedbackItem struct {
		ID             string `json:"id"`
		GuestName      string `json:"guest_name"`
		OverallRating  int    `json:"overall_rating"`
		Comments       string `json:"comments"`
		WouldRecommend bool   `json:"would_recommend"`
		CreatedAt      string `json:"created_at"`
	}

	var items []FeedbackItem
	for rows.Next() {
		var f FeedbackItem
		rows.Scan(&f.ID, &f.GuestName, &f.OverallRating, &f.Comments, &f.WouldRecommend, &f.CreatedAt)
		items = append(items, f)
	}
	jsonSuccess(w, items)
}

// ── Guest Search (CRM-enhanced) ──

// CRMGuestSearch searches guests with CRM enrichment
func (h *Handler) CRMGuestSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		jsonError(w, "Search query required", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT g.id, g.first_name, g.last_name, COALESCE(g.email, ''), COALESCE(g.phone, ''),
		       COALESCE(g.nationality, ''),
		       COALESCE((SELECT COUNT(*) FROM reservations WHERE guest_id = g.id), 0),
		       COALESCE(g.loyalty_tier, 'member'),
		       COALESCE(g.loyalty_points, 0)
		FROM guests g
		WHERE (
			g.first_name ILIKE '%' || $1 || '%'
			OR g.last_name ILIKE '%' || $1 || '%'
			OR g.email ILIKE '%' || $1 || '%'
			OR g.phone ILIKE '%' || $1 || '%'
		)
		ORDER BY g.first_name
		LIMIT $2 OFFSET $3
	`, query, limit, offset)
	if err != nil {
		jsonError(w, "Search failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type SearchResult struct {
		ID            string `json:"id"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
		Email         string `json:"email"`
		Phone         string `json:"phone"`
		Nationality   string `json:"nationality"`
		TotalStays    int    `json:"total_stays"`
		LoyaltyTier   string `json:"loyalty_tier"`
		LoyaltyPoints int    `json:"loyalty_points"`
	}

	var results []SearchResult
	for rows.Next() {
		var s SearchResult
		rows.Scan(&s.ID, &s.FirstName, &s.LastName, &s.Email, &s.Phone,
			&s.Nationality, &s.TotalStays, &s.LoyaltyTier, &s.LoyaltyPoints)
		results = append(results, s)
	}
	jsonSuccess(w, results)
}
