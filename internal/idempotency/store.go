package idempotency

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/takemerightttt/cart-service/internal/postgres"
)

var (
	ErrInProgress = errors.New("a request with this idempotency key is still in progress")
	ErrKeyReused  = errors.New("idempotency key was already used with a different request")
)

type Response struct {
	Status int
	Body   []byte
}

type Store struct {
	db  postgres.DBTX
	ttl time.Duration
}

func NewStore(db postgres.DBTX, ttl time.Duration) *Store {
	return &Store{db: db, ttl: ttl}
}

func (s *Store) Begin(ctx context.Context, userID uuid.UUID, key string, hash []byte) (*Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		tag, err := s.db.Exec(ctx,
			`INSERT INTO idempotency_keys (user_id, key, request_hash) VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, key) DO NOTHING`, userID, key, hash)
		if err != nil {
			return nil, fmt.Errorf("insert idempotency key: %w", err)
		}
		if tag.RowsAffected() == 1 {
			return nil, nil
		}
		var (
			storedHash []byte
			status     *int
			body       []byte
			createdAt  time.Time
		)
		err = s.db.QueryRow(ctx,
			`SELECT request_hash, status_code, response_body, created_at
			 FROM idempotency_keys WHERE user_id = $1 AND key = $2`, userID, key).
			Scan(&storedHash, &status, &body, &createdAt)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("select idempotency key: %w", err)
		}
		if time.Since(createdAt) > s.ttl {
			if _, err := s.db.Exec(ctx,
				`DELETE FROM idempotency_keys WHERE user_id = $1 AND key = $2 AND created_at = $3`,
				userID, key, createdAt); err != nil {
				return nil, fmt.Errorf("expire idempotency key: %w", err)
			}
			continue
		}
		if !bytes.Equal(storedHash, hash) {
			return nil, ErrKeyReused
		}
		if status == nil {
			return nil, ErrInProgress
		}
		return &Response{Status: *status, Body: body}, nil
	}
	return nil, ErrInProgress
}

func (s *Store) Complete(ctx context.Context, userID uuid.UUID, key string, resp Response) error {
	_, err := s.db.Exec(ctx,
		`UPDATE idempotency_keys SET status_code = $3, response_body = $4 WHERE user_id = $1 AND key = $2`,
		userID, key, resp.Status, resp.Body)
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

func (s *Store) Release(ctx context.Context, userID uuid.UUID, key string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND key = $2 AND status_code IS NULL`, userID, key)
	if err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}
