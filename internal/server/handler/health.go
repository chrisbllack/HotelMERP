package handler

import (
	"net/http"

	"infineon/internal/response"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		response.RespondError(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	if err := h.db.Ping(r.Context()); err != nil {
		response.RespondError(w, "Database unreachable", http.StatusServiceUnavailable)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"status":"healthy","version":"0.1.0","database":"connected"}, "System operational")
}
