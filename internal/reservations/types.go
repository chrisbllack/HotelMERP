package reservations

import (
	"database/sql"
	"time"
)

// Status is a reservation lifecycle state.
type Status string

// Reservation lifecycle states.
const (
	StatusTentative  Status = "tentative"
	StatusConfirmed  Status = "confirmed"
	StatusCheckedIn  Status = "checked_in"
	StatusInHouse    Status = "in_house"
	StatusCheckedOut Status = "checked_out"
	StatusCancelled  Status = "cancelled"
	StatusNoShow     Status = "no_show"
)

// CreateRequest captures the inputs for booking a reservation. Dates are
// ISO-8601 strings (YYYY-MM-DD); nil room pointers mean "any".
type CreateRequest struct {
	GuestID       int
	RoomID        *int
	RoomTypeID    *int
	ArrivalDate   string
	DepartureDate string
	Source        string
}

// Reservation is a persisted booking in money-minor units.
type Reservation struct {
	ID                 int
	PropertyID         int
	GuestID            int
	RoomID             sql.NullInt64
	RoomTypeID         sql.NullInt64
	ConfirmationNumber string
	ArrivalDate        string
	DepartureDate      string
	Status             Status
	Source             string
	BalanceMinor       int64
	PaymentStatus      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
