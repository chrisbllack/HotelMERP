package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"

	"infineon/internal/auth"

	"github.com/google/uuid"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	PropertyID string `json:"property_id"`
}

// userPrincipal is the authenticated identity handed back to clients.
type userPrincipal struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	PropertyID  string   `json:"property_id"`
}

// Login verifies email + password and issues a JWT carrying the caller's
// roles and merged per-property permissions (user_property_roles bridge).
//
// Validation happens before any database access so malformed requests fail
// fast with 400 even when the storage backend is unavailable.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Request body must be valid JSON", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || strings.TrimSpace(req.Password) == "" {
		jsonError(w, "Email and password are required", http.StatusBadRequest)
		return
	}
	if h.db == nil || h.jwt == nil {
		jsonError(w, "Authentication backend unavailable", http.StatusServiceUnavailable)
		return
	}

	var (
		userID   string
		name     string
		password string
		status   string
	)
	err := h.db.QueryRow(r.Context(), `
		SELECT id, name, password, status
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, req.Email).Scan(&userID, &name, &password, &status)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("[auth] login query failed: %v", err)
		jsonError(w, "Login failed", http.StatusInternalServerError)
		return
	}
	if status != "active" {
		jsonError(w, "Account is disabled", http.StatusForbidden)
		return
	}
	if !auth.CheckPassword(req.Password, password) {
		jsonError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	roles, permissions, propertyID, err := h.loadIdentity(r.Context(), userID)
	if err != nil {
		log.Printf("[auth] identity load failed: %v", err)
		jsonError(w, "Login failed", http.StatusInternalServerError)
		return
	}

	token, err := h.jwt.GenerateToken(
		userIDAsUUID(userID), userIDAsUUID(propertyID), roles, permissions,
	)
	if err != nil {
		log.Printf("[auth] token issue failed: %v", err)
		jsonError(w, "Login failed", http.StatusInternalServerError)
		return
	}

	// Best-effort bookkeeping — never blocks authentication.
	_, _ = h.db.Exec(r.Context(), `
		UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = $1
	`, userID)
	auditLog(h, r, "auth", "login", userID, "Successful login")

	jsonSuccess(w, map[string]interface{}{
		"token":       token,
		"token_type":  "Bearer",
		"expires_in":  8 * 60 * 60,
		"property_id": propertyID,
		"user": userPrincipal{
			ID:          userID,
			Name:        name,
			Email:       req.Email,
			Roles:       roles,
			Permissions: permissions,
			PropertyID:  propertyID,
		},
	})
}

// Register creates an account and its per-property role assignment.
// Open registration on an on-premise instance yields a least-privilege
// front-office account; elevated roles are granted by an administrator.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Request body must be valid JSON", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || req.Email == "" || strings.TrimSpace(req.Password) == "" {
		jsonError(w, "Name, email and password are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if h.db == nil {
		jsonError(w, "Authentication backend unavailable", http.StatusServiceUnavailable)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("[auth] password hash failed: %v", err)
		jsonError(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO users (id, name, email, password, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, req.Name, req.Email, hash)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			jsonError(w, "An account with this email already exists", http.StatusConflict)
			return
		}
		log.Printf("[auth] register insert failed: %v", err)
		jsonError(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "receptionist"
	}
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO user_property_roles (id, user_id, property_id, role, permissions)
		VALUES ($1, $2, NULLIF($3, ''), $4, '[]')
	`, uuid.New().String(), userID, strings.TrimSpace(req.PropertyID), role)
	if err != nil {
		// The user exists even if the role bridge write fails; log loudly.
		log.Printf("[auth] role assignment failed for %s: %v", userID, err)
	}
	auditLog(h, r, "auth", "register", userID, "Account created")

	SendCreated(w, map[string]interface{}{
		"id":    userID,
		"name":  req.Name,
		"email": req.Email,
		"role":  role,
	})
}

// loadIdentity resolves the caller's roles, merged permission set and default
// property from the user_property_roles bridge table.
func (h *Handler) loadIdentity(ctx context.Context, userID string) ([]string, []string, string, error) {
	rows, err := h.db.Query(ctx, `
		SELECT role, permissions, property_id
		FROM user_property_roles
		WHERE user_id = $1
		ORDER BY property_id
	`, userID)
	if err != nil {
		return nil, nil, "", err
	}
	defer rows.Close()

	roleSet := map[string]bool{}
	permSet := map[string]bool{}
	propertyID := ""
	for rows.Next() {
		var (
			role     string
			rawPerms sql.NullString
			prop     sql.NullString
		)
		if err := rows.Scan(&role, &rawPerms, &prop); err != nil {
			return nil, nil, "", err
		}
		if role != "" {
			roleSet[role] = true
		}
		if rawPerms.Valid && strings.TrimSpace(rawPerms.String) != "" {
			var perms []string
			if err := json.Unmarshal([]byte(rawPerms.String), &perms); err == nil {
				for _, p := range perms {
					if p != "" {
						permSet[p] = true
					}
				}
			}
		}
		if propertyID == "" && prop.Valid && strings.TrimSpace(prop.String) != "" {
			propertyID = strings.TrimSpace(prop.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, "", err
	}

	// Fallback: no bridge rows yet — authenticated, but permission-less.
	if len(roleSet) == 0 {
		roleSet["authenticated"] = true
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}
	sort.Strings(perms)
	return roles, perms, propertyID, nil
}

// userIDAsUUID maps legacy non-UUID user ids (seeded demo ids such as
// "usr-cashier-01") onto a stable UUIDv5 so JWT subjects always parse.
func userIDAsUUID(raw string) uuid.UUID {
	if raw == "" {
		return uuid.Nil
	}
	if parsed, err := uuid.Parse(raw); err == nil {
		return parsed
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(raw))
}
