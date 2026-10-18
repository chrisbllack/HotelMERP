package handler

// M-FIN ledger HTTP surface (port of src/convex/ledger.ts queries). All
// monetary values cross this API as integer kobo; the finance console and
// the booking engine consume the same contract.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"infineon/internal/ledger"
)

// ListGLAccounts handles GET /gl/accounts — the property's standard chart
// (created on first read).
func (h *Handler) ListGLAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := ledger.Accounts(r.Context(), h.db, getPropertyID2(r))
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, accounts)
}

// GLTrialBalance handles GET /gl/trial-balance?from=&to= — derived, never
// stored; totals must be equal when the books balance.
func (h *Handler) GLTrialBalance(w http.ResponseWriter, r *http.Request) {
	res, err := ledger.TrialBalance(r.Context(), h.db, getPropertyID2(r),
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, res)
}

// GLProfitAndLoss handles GET /gl/profit-and-loss?from=&to=.
func (h *Handler) GLProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	res, err := ledger.ProfitAndLoss(r.Context(), h.db, getPropertyID2(r),
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, res)
}

// GLEntryList handles GET /gl/entries?account_code=&from=&to= — one
// account's posting history.
func (h *Handler) GLEntryList(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("account_code")
	if code == "" {
		jsonError(w, "account_code is required", http.StatusBadRequest)
		return
	}
	entries, err := ledger.AccountLedger(r.Context(), h.db, getPropertyID2(r), code,
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, entries)
}

// postGLJournalRequest is the manual journal contract (balanced,
// period-checked).
type postGLJournalRequest struct {
	Date  string               `json:"date"`
	Ref   string               `json:"ref"`
	Lines []ledger.JournalLine `json:"lines"`
}

// PostGLJournal handles POST /gl/journal — the Finance console's manual
// double entry. Validation failures are 400, a closed period 409.
func (h *Handler) PostGLJournal(w http.ResponseWriter, r *http.Request) {
	var req postGLJournalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Date == "" {
		jsonError(w, "date is required", http.StatusBadRequest)
		return
	}

	in := ledger.JournalInput{
		PropertyID: getPropertyID2(r),
		Date:       req.Date,
		Lines:      req.Lines,
		Source:     "manual",
		Ref:        req.Ref,
		By:         eventActor(r),
	}
	if err := ledger.Validate(in); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	count, err := ledger.PostJournal(r.Context(), h.db, in)
	if err != nil {
		if errors.Is(err, ledger.ErrPeriodLocked) {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	auditLog(h, r, "finance", "gl_manual_posted", req.Date,
		fmt.Sprintf("%d lines via %s", count, in.By))
	jsonSuccess(w, map[string]interface{}{"ok": true, "lines": count})
}
