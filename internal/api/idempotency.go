package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type IdempotencyStore interface {
	Begin(r *http.Request, key string, requestHash string) (IdempotencyResult, error)
	Complete(r *http.Request, key string, status int, body []byte) error
	Fail(r *http.Request, key string) error
}

type IdempotencyResult struct {
	Replay       bool
	Status       int
	ResponseBody []byte
}

type idempotencyResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *idempotencyResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *idempotencyResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	_, _ = w.body.Write(body)
	return w.ResponseWriter.Write(body)
}

func RequireIdempotencyKey(store IdempotencyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

			if key == "" {
				WriteError(
					w,
					r,
					http.StatusBadRequest,
					"idempotency_key_required",
					"Idempotency-Key is required for this command",
					nil,
				)
				return
			}

			if len(key) > 128 {
				WriteError(
					w,
					r,
					http.StatusBadRequest,
					"invalid_idempotency_key",
					"Idempotency-Key is too long",
					nil,
				)
				return
			}

			if _, err := uuid.Parse(key); err != nil {
				WriteError(
					w,
					r,
					http.StatusBadRequest,
					"invalid_idempotency_key",
					"Idempotency-Key must be a UUID",
					nil,
				)
				return
			}

			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
			if err != nil {
				WriteError(
					w,
					r,
					http.StatusBadRequest,
					"invalid_request",
					"Unable to read request body",
					nil,
				)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := sha256.Sum256(append(
				[]byte(r.Method+"\x00"+r.URL.Path+"\x00"),
				body...,
			))
			requestHash := hex.EncodeToString(hash[:])

			result, err := store.Begin(r, key, requestHash)
			if err != nil {
				HandleError(w, r, err)
				return
			}

			if result.Replay {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(result.Status)
				_, _ = w.Write(result.ResponseBody)
				return
			}

			recorder := &idempotencyResponseWriter{
				ResponseWriter: w,
			}

			next.ServeHTTP(recorder, r)

			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}

			if status >= http.StatusInternalServerError {
				_ = store.Fail(r, key)
				return
			}

			if err := store.Complete(r, key, status, recorder.body.Bytes()); err != nil {
				_ = store.Fail(r, key)
			}
		})
	}
}
