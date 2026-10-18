package guestapp

import (
    "encoding/json"
    "net/http"
    
    "github.com/go-chi/chi/v5"
)

// GuestProfile represents guest information for mobile app
type GuestProfile struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Email        string `json:"email"`
    MobileNumber string `json:"mobile_number"`
    LoyaltyTier  string `json:"loyalty_tier"`
    CheckIns     int    `json:"check_ins"`
    Points       int    `json:"points"`
}

// GuestReservation represents reservation for mobile app
type GuestReservation struct {
    ID           string    `json:"id"`
    RoomID       string    `json:"room_id"`
    CheckIn      string    `json:"check_in"`
    CheckOut     string    `json:"check_out"`
    RoomType     string    `json:"room_type"`
    Status       string    `json:"status"`
    TotalAmount  float64   `json:"total_amount"`
}

// GetGuestProfile returns guest profile information
func GetGuestProfile(w http.ResponseWriter, r *http.Request) {
    // Get user info from context (authenticated)
    // Fetch profile from database
    profile := GuestProfile{
        ID:           "GUEST-001",
        Name:         "John Doe",
        Email:        "john.doe@example.com",
        MobileNumber: "+1234567890",
        LoyaltyTier:  "Gold",
        CheckIns:     15,
        Points:       2500,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(profile)
}

// GetGuestReservations retrieves guest's reservations
func GetGuestReservations(w http.ResponseWriter, r *http.Request) {
    // Get user ID from context
    // Fetch reservations from database
    reservations := []GuestReservation{
        {
            ID:           "RES-001",
            RoomID:       "ROOM-101",
            CheckIn:      "2023-12-01",
            CheckOut:     "2023-12-05",
            RoomType:     "Deluxe Suite",
            Status:       "confirmed",
            TotalAmount:  850.00,
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(reservations)
}

// Routes sets up mobile API routes
func Routes(r *chi.Mux) {
    r.Get("/guest/profile", GetGuestProfile)
    r.Get("/guest/reservations", GetGuestReservations)
}
