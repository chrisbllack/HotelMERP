package response

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// APIResponse is the standard JSON envelope for all API responses.
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Code      string      `json:"code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Meta      Meta        `json:"meta,omitempty"`
	Errors    []string    `json:"errors,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// Meta carries request metadata and pagination.
type Meta struct {
	RequestID   string      `json:"request_id"`
	ExecutionMs int64       `json:"execution_time_ms,omitempty"`
	Pagination  *Pagination `json:"pagination,omitempty"`
}

// Pagination holds paging information for list endpoints.
type Pagination struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
	LastPage    int   `json:"last_page"`
}

// RespondJSON writes a raw JSON response with the given status code.
func RespondJSON(w http.ResponseWriter, code int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", resp.Meta.RequestID)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// RespondSuccess writes a success response with optional custom status code.
func RespondSuccess(w http.ResponseWriter, data interface{}, message string, code ...int) {
	status := http.StatusOK
	if len(code) > 0 {
		status = code[0]
	}
	RespondJSON(w, status, APIResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Meta:      Meta{RequestID: uuid.New().String()},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// RespondError writes an error response.
func RespondError(w http.ResponseWriter, message string, code int) {
	RespondJSON(w, code, APIResponse{
		Success:   false,
		Message:   message,
		Errors:    []string{message},
		Meta:      Meta{RequestID: uuid.New().String()},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// RespondPaginated writes a paginated success response.
func RespondPaginated(w http.ResponseWriter, data interface{}, total int64, page, perPage int) {
	lastPage := int(total) / perPage
	if int(total)%perPage > 0 {
		lastPage++
	}
	RespondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Operation completed successfully",
		Data:    data,
		Meta: Meta{
			RequestID: uuid.New().String(),
			Pagination: &Pagination{
				CurrentPage: page,
				PerPage:     perPage,
				Total:       total,
				LastPage:    lastPage,
			},
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// DecodeJSON is a convenience wrapper that decodes a JSON request body into dst.
func DecodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
