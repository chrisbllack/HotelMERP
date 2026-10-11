package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidRefreshToken = errors.New("invalid or revoked refresh token")

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

type RefreshSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

func randomRefreshToken() (string, error) {
	value := make([]byte, 48)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *SessionStore) Create(
	ctx context.Context,
	userID uuid.UUID,
	duration time.Duration,
) (string, error) {
	token, err := randomRefreshToken()
	if err != nil {
		return "", err
	}

	_, err = s.db.ExecContext(
		ctx,
		`
		INSERT INTO refresh_tokens (
			id,
			user_id,
			token_hash,
			expires_at,
			created_at
		)
		VALUES ($1,$2,$3,$4,NOW())
		`,
		uuid.New(),
		userID,
		tokenHash(token),
		time.Now().UTC().Add(duration),
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *SessionStore) Rotate(
	ctx context.Context,
	token string,
	duration time.Duration,
) (uuid.UUID, string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback()

	var sessionID uuid.UUID
	var userID uuid.UUID
	var expiresAt time.Time

	err = tx.QueryRowContext(
		ctx,
		`
		SELECT id, user_id, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		FOR UPDATE
		`,
		tokenHash(token),
	).Scan(&sessionID, &userID, &expiresAt)
	if err != nil {
		return uuid.Nil, "", ErrInvalidRefreshToken
	}

	if time.Now().UTC().After(expiresAt) {
		return uuid.Nil, "", ErrInvalidRefreshToken
	}

	nextToken, err := randomRefreshToken()
	if err != nil {
		return uuid.Nil, "", err
	}

	_, err = tx.ExecContext(
		ctx,
		`
		UPDATE refresh_tokens
		SET revoked_at = NOW(),
		    replaced_by = $1
		WHERE id = $2
		`,
		uuid.New(),
		sessionID,
	)
	if err != nil {
		return uuid.Nil, "", err
	}

	nextID := uuid.New()
	_, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO refresh_tokens (
			id,
			user_id,
			token_hash,
			expires_at,
			created_at
		)
		VALUES ($1,$2,$3,$4,NOW())
		`,
		nextID,
		userID,
		tokenHash(nextToken),
		time.Now().UTC().Add(duration),
	)
	if err != nil {
		return uuid.Nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, "", err
	}

	return userID, nextToken, nil
}

func (s *SessionStore) Revoke(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(
		ctx,
		`
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		`,
		tokenHash(token),
	)
	return err
}
