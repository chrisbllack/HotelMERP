package health

import (
	"net/http"

	"infineon/internal/api"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Public(w http.ResponseWriter, r *http.Request) {
	result := h.service.CheckApplication(r.Context())

	status := http.StatusOK
	if result.Database == Failed {
		status = http.StatusServiceUnavailable
	}

	api.WriteSuccess(w, r, status, result)
}

func (h *Handler) Protected(w http.ResponseWriter, r *http.Request) {
	propertyID, ok := api.PropertyID(r.Context())
	if !ok {
		api.WriteError(
			w,
			r,
			http.StatusBadRequest,
			"property_context_required",
			"Property context is required",
			nil,
		)
		return
	}

	result := h.service.Check(r.Context(), int(propertyID.ID()))
	status := http.StatusOK
	if result.Database == Failed {
		status = http.StatusServiceUnavailable
	}

	api.WriteSuccess(w, r, status, result)
}
