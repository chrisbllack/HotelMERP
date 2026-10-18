package api

import (
	"errors"
	"log"
	"net/http"

	"infineon/internal/response"

	"github.com/google/uuid"
)

// apiError carries an HTTP status and a machine-readable code so business
// errors surface with the right status through HandleError.
type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.message }

// Conflict builds a 409 conflict error (idempotency, state clashes).
func Conflict(code, message string) error {
	return &apiError{status: http.StatusConflict, code: code, message: message}
}

// BadRequest builds a 400 validation error.
func BadRequest(code, message string) error {
	return &apiError{status: http.StatusBadRequest, code: code, message: message}
}

// requestID returns the inbound request id, or a fresh one.
func requestID(r *http.Request) string {
	if r != nil {
		if id := r.Header.Get("X-Request-ID"); id != "" {
			return id
		}
	}
	return uuid.New().String()
}

// WriteError writes the standard API error envelope with a machine-readable
// error code and optional structured details.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	response.RespondJSON(w, status, response.APIResponse{
		Success:   false,
		Message:   message,
		Errors:    []string{message},
		Code:      code,
		Details:   details,
		Meta:      response.Meta{RequestID: requestID(r)},
		Timestamp: nowRFC3339(),
	})
}

// WriteSuccess writes the standard API success envelope with an explicit
// status code, correlating the response to the inbound request id.
func WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	response.RespondJSON(w, status, response.APIResponse{
		Success:   true,
		Message:   "Operation completed successfully",
		Data:      data,
		Meta:      response.Meta{RequestID: requestID(r)},
		Timestamp: nowRFC3339(),
	})
}

// HandleError maps an error onto the API error envelope. Typed errors built
// by Conflict/BadRequest keep their status and code; anything else is logged
// and reported as an internal error without leaking internals.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var typed *apiError
	if errors.As(err, &typed) {
		WriteError(w, r, typed.status, typed.code, typed.message, nil)
		return
	}
	log.Printf("[api] internal error: %v", err)
	WriteError(w, r, http.StatusInternalServerError, "internal_error",
		"An unexpected error occurred", nil)
}
