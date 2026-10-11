package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrMissingIdempotencyKey = errors.New("missing idempotency key")
	ErrIdempotencyConflict   = errors.New("idempotency key reused with different request")
	ErrIdempotencyInProgress = errors.New("idempotency request is already in progress")
)

type IdempotencyRecord struct {
	Key         string
	RequestHash string
	StatusCode  int
	Body        []byte
}

type IdempotencyStore interface {
	Find(
		ctx context.Context,
		propertyID uuid.UUID,
		actorID uuid.UUID,
		key string,
	) (*IdempotencyRecord, error)

	Store(
		ctx context.Context,
		propertyID uuid.UUID,
		actorID uuid.UUID,
		record IdempotencyRecord,
	) error
}

type IdempotencyDecision struct {
	Replay bool
	Record *IdempotencyRecord
}

func RequestID(r *http.Request) string {
	return r.Header.Get("X-Request-ID")
}

func RequestHash(r *http.Request, body []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(r.Method))
	_, _ = hash.Write([]byte(r.URL.Path))
	_, _ = hash.Write([]byte(r.URL.RawQuery))
	_, _ = hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}

func ReadBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func RequireIdempotencyKey(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", ErrMissingIdempotencyKey
	}

	if len(key) > 255 {
		return "", errors.New("idempotency key is too long")
	}

	return key, nil
}

func DecodeJSON[T any](r *http.Request, target *T) ([]byte, error) {
	body, err := ReadBody(r)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("request body is empty")
	}

	if err := json.Unmarshal(body, target); err != nil {
		return nil, err
	}

	return body, nil
}

func BeginIdempotentRequest(
	ctx context.Context,
	store IdempotencyStore,
	propertyID uuid.UUID,
	actorID uuid.UUID,
	key string,
	requestHash string,
) (*IdempotencyDecision, error) {
	if store == nil {
		return nil, errors.New("idempotency store is not configured")
	}

	record, err := store.Find(ctx, propertyID, actorID, key)
	if err == nil && record != nil {
		if record.RequestHash != requestHash {
			return nil, ErrIdempotencyConflict
		}

		if record.StatusCode > 0 && len(record.Body) > 0 {
			return &IdempotencyDecision{
				Replay: true,
				Record: record,
			}, nil
		}

		return nil, ErrIdempotencyInProgress
	}

	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return nil, err
	}

	err = store.Store(
		ctx,
		propertyID,
		actorID,
		IdempotencyRecord{
			Key:         key,
			RequestHash: requestHash,
		},
	)
	if err != nil {
		return nil, err
	}

	return &IdempotencyDecision{}, nil
}

func CompleteIdempotentRequest(
	ctx context.Context,
	store IdempotencyStore,
	propertyID uuid.UUID,
	actorID uuid.UUID,
	key string,
	requestHash string,
	statusCode int,
	body []byte,
) error {
	if store == nil {
		return errors.New("idempotency store is not configured")
	}

	if statusCode < 200 || statusCode > 599 {
		return errors.New("invalid idempotency response status")
	}

	return store.Store(
		ctx,
		propertyID,
		actorID,
		IdempotencyRecord{
			Key:         key,
			RequestHash: requestHash,
			StatusCode:  statusCode,
			Body:        append([]byte(nil), body...),
		},
	)
}

type idempotencyRecordNotFound struct{}

func (idempotencyRecordNotFound) Error() string {
	return "idempotency record not found"
}

var ErrRecordNotFound error = idempotencyRecordNotFound{}
