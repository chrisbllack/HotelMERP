package core

import "errors"

var ErrReservationConflict = errors.New(
	"reservation conflicts with existing room assignment",
)
