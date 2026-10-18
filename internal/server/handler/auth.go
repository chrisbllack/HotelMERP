package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"infineon/internal/api"
	"infineon/internal/auth"
)

type AuthHandler struct {
	jwtSvc       *auth.JWTService
	sessionStore *auth.SessionStore
}

func NewAuthHandler(
	jwtSvc *auth.JWTService,
	sessionStore *auth.SessionStore,
) *AuthHandler {
	return &AuthHandler{
		jwtSvc:       jwtSvc,
		sessionStore: sessionStore,
	}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.RefreshToken) == "" {
		api.WriteError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_request",
			"refresh_token is required",
			nil,
		)
		return
	}

	userID, nextRefreshToken, err := h.sessionStore.Rotate(
		r.Context(),
		req.RefreshToken,
		30*24*time.Hour,
	)
	if err != nil {
		api.WriteError(
			w,
			r,
			http.StatusUnauthorized,
			"invalid_refresh_token",
			"Refresh token is invalid or expired",
			nil,
		)
		return
	}

	token, err := h.jwtSvc.GenerateToken(
		userID,
		[16]byte{},
		nil,
		nil,
	)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	api.WriteSuccess(w, r, http.StatusOK, map[string]string{
		"token":         token,
		"refresh_token": nextRefreshToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.Header.Get("X-Refresh-Token"))
	if token != "" {
		_ = h.sessionStore.Revoke(r.Context(), token)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		api.WriteError(
			w,
			r,
			http.StatusUnauthorized,
			"unauthorized",
			"Authentication is required",
			nil,
		)
		return
	}

	api.WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":          claims.UserID,
			"roles":       claims.Roles,
			"permissions": claims.Permissions,
		},
		"properties": []interface{}{
			claims.PropertyID,
		},
	})
}
