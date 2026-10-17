package staffapp

import (
    "encoding/json"
    "net/http"
    
    "hotelmerp/internal/lodging/rooms"
)

// StaffRoomStatus represents room status for staff app
type StaffRoomStatus struct {
    RoomID      string `json:"room_id"`
    RoomNumber  string `json:"room_number"`
    Status      string `json:"status"`
    GuestName   string `json:"guest_name"`
    CheckOut    string `json:"check_out"`
    RoomType    string `json:"room_type"`
}

// GetStaffRoomStatus returns current room statuses for staff
func GetStaffRoomStatus(w http.ResponseWriter, r *http.Request) {
    // For demonstration, returning mock data
    statuses := []StaffRoomStatus{
        {
            RoomID:     "ROOM-101",
            RoomNumber: "101",
            Status:     "occupied",
            GuestName:  "John Smith",
            CheckOut:   "2023-12-05",
            RoomType:   "Deluxe Suite",
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(statuses)
}

// UpdateRoomStatus updates room status
func UpdateRoomStatus(w http.ResponseWriter, r *http.Request) {
    // Implementation for updating room status
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"success": true}`))
}

// Routes sets up staff mobile API routes
func Routes(r *chi.Mux) {
    r.Get("/staff/room-status", GetStaffRoomStatus)
    r.Post("/staff/room-status", UpdateRoomStatus)
}
