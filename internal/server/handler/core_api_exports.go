package handler

import "net/http"

func (h *CoreAPI) ListReservations(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.listReservations(w, r)
}

func (h *CoreAPI) CreateReservation(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.createReservation(w, r)
}

func (h *CoreAPI) GetReservation(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.getReservation(w, r)
}

func (h *CoreAPI) CheckIn(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.checkIn(w, r)
}

func (h *CoreAPI) CheckOut(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.checkOut(w, r)
}

func (h *CoreAPI) GetFolio(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.getFolio(w, r)
}

func (h *CoreAPI) PostCharge(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.postCharge(w, r)
}

func (h *CoreAPI) InitiatePayment(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.initiatePayment(w, r)
}

func (h *CoreAPI) VerifyPayment(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.verifyPayment(w, r)
}

func (h *CoreAPI) NightAuditStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.nightAuditStatus(w, r)
}

func (h *CoreAPI) RunNightAudit(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.runNightAudit(w, r)
}
