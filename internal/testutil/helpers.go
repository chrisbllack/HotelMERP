package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"infineon/internal/auth"
	"infineon/internal/database"
	"infineon/internal/server/handler"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestServer wraps the handler for testing without a real database.
type TestServer struct {
	Handler *handler.Handler
	Mux     http.Handler
}

// SetupTestHandler creates a handler with a real JWT service for testing.
// Pass nil for db if you only need to test auth/crypto logic.
func SetupTestHandler(t *testing.T, db *pgxpool.Pool) *TestServer {
	t.Helper()
	jwt := auth.NewJWTService([]byte("test-secret-for-unit-tests-2024"))
	var pool database.Pool
	if db != nil {
		pool = database.NewPGXPool(db)
	}
	h := handler.New(pool, jwt)
	return &TestServer{Handler: h}
}

// MakeRequest creates an HTTP request and records the response.
func MakeRequest(method, path string, body interface{}, headers map[string]string) (*httptest.ResponseRecorder, *http.Request) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Property-ID", "00000000-0000-0000-0000-000000000001")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	return w, req
}

// MakeAuthRequest creates an authenticated HTTP request.
func MakeAuthRequest(method, path string, body interface{}, jwtSecret []byte) (*httptest.ResponseRecorder, *http.Request) {
	jwtSvc := auth.NewJWTService(jwtSecret)
	token, _ := jwtSvc.GenerateToken(
		[16]byte{1}, // userID
		[16]byte{1}, // propertyID
		[]string{"admin"},
		[]string{"*"},
	)

	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}
	return MakeRequest(method, path, body, headers)
}

// ParseResponse extracts the JSON response body.
func ParseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return result
}

// AssertSuccess checks that the response indicates success.
func AssertSuccess(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	resp := ParseResponse(t, w)
	success, ok := resp["success"].(bool)
	if !ok || !success {
		t.Errorf("expected success response, got: %v", resp)
	}
	return resp
}

// AssertError checks that the response indicates an error.
func AssertError(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	if w.Code != expectedCode {
		t.Errorf("expected status %d, got %d", expectedCode, w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	success, _ := resp["success"].(bool)
	if success {
		t.Error("expected error response, got success")
	}
}

// AssertStatusCode checks the HTTP status code.
func AssertStatusCode(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d. Body: %s", expected, w.Code, w.Body.String())
	}
}

// TokenFor generates a test JWT token.
func TokenFor(t *testing.T) string {
	t.Helper()
	jwtSvc := auth.NewJWTService([]byte("test-secret-for-unit-tests-2024"))
	token, err := jwtSvc.GenerateToken(
		[16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[]string{"admin"},
		[]string{"*"},
	)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}
