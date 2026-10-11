package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"infineon/internal/api"
)

var ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")
var ErrIdempotencyInProgress = errors.New("request with this idempotency key is already processing")

type IdempotencyStore struct {
	db *sql.DB
}

func NewIdempotencyStore(db *sql.DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

func (s *IdempotencyStore) Begin(
	r *http.Request,
	key string,
	requestHash string,
) (api.IdempotencyResult, error) {
	propertyID := r.Header.Get("X-Property-ID")
	userID := r.Context().Value("user_id")

	var existingHash string
	var state string
	var status sql.NullInt64
	var body []byte

	err := s.db.QueryRowContext(
		r.Context(),
		`
		SELECT request_hash, state, response_status, response_body
		FROM api_idempotency_keys
		WHERE property_id::text = $1
		  AND idempotency_key = $2
		  AND expires_at > NOW()
		`,
		propertyID,
		key,
	).Scan(&existingHash, &state, &status, &body)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = s.db.ExecContext(
			r.Context(),
			`
			INSERT INTO api_idempotency_keys (
				organisation_id,
				property_id,
				user_id,
				method,
				request_path,
				idempotency_key,
				request_hash,
				state,
				expires_at
			)
			VALUES (
				NULLIF($1, '')::integer,
				NULLIF($2, '')::integer,
				NULLIF($3, '')::integer,
				$4,
				$5,
				$6,
				$7,
				'processing',
				NOW() + INTERVAL '24 hours'
			)
			ON CONFLICT (property_id, idempotency_key) DO NOTHING
			`,
			"",
			propertyID,
			fmt.Sprint(userID),
			r.Method,
			r.URL.Path,
			key,
			requestHash,
		)
		if err != nil {
			return api.IdempotencyResult{}, err
		}

		return api.IdempotencyResult{}, nil

	case err != nil:
		return api.IdempotencyResult{}, err
	}

	if existingHash != requestHash {
		return api.IdempotencyResult{}, api.Conflict(
			"idempotency_conflict",
			ErrIdempotencyConflict.Error(),
		)
	}

	if state == "processing" {
		return api.IdempotencyResult{}, api.Conflict(
			"idempotency_in_progress",
			ErrIdempotencyInProgress.Error(),
		)
	}

	if state == "completed" && status.Valid {
		return api.IdempotencyResult{
			Replay:       true,
			Status:       int(status.Int64),
			ResponseBody: body,
		}, nil
	}

	return api.IdempotencyResult{}, nil
}

func (s *IdempotencyStore) Complete(
	r *http.Request,
	key string,
	status int,
	body []byte,
) error {
	_, err := s.db.ExecContext(
		r.Context(),
		`
		UPDATE api_idempotency_keys
		SET state = 'completed',
		    response_status = $1,
		    response_body = $2::jsonb,
		    completed_at = NOW()
		WHERE property_id::text = $3
		  AND idempotency_key = $4
		`,
		status,
		string(body),
		r.Header.Get("X-Property-ID"),
		key,
	)

	return err
}

func (s *IdempotencyStore) Fail(
	r *http.Request,
	key string,
) error {
	_, err := s.db.ExecContext(
		r.Context(),
		`
		DELETE FROM api_idempotency_keys
		WHERE property_id::text = $1
		  AND idempotency_key = $2
		`,
		r.Header.Get("X-Property-ID"),
		key,
	)

	return err
}

func CleanupIdempotencyKeys(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`DELETE FROM api_idempotency_keys WHERE expires_at <= $1`,
		time.Now().UTC(),
	)
	return err
}
