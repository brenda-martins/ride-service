package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ride/internal/app"
)

// IdempotencyStore implements app.IdempotencyStore using PostgreSQL.
type IdempotencyStore struct {
	DB *sql.DB
}

func (s *IdempotencyStore) Get(ctx context.Context, endpoint, actorID, key string) (*app.IdempotencyRecord, bool, error) {
	ex := execFromContext(ctx, s.DB)
	const q = `
		SELECT endpoint, actor_id, idempotency_key, request_hash, response_status, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE endpoint = $1 AND actor_id = $2 AND idempotency_key = $3 AND expires_at > NOW()`
	row := ex.QueryRowContext(ctx, q, endpoint, actorID, key)
	var rec app.IdempotencyRecord
	if err := row.Scan(&rec.Endpoint, &rec.ActorID, &rec.Key, &rec.RequestHash, &rec.ResponseCode, &rec.ResponseBody, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &rec, true, nil
}

func (s *IdempotencyStore) Claim(ctx context.Context, rec app.IdempotencyRecord) (*app.IdempotencyRecord, bool, error) {
	ex := execFromContext(ctx, s.DB)
	const q = `
		INSERT INTO idempotency_keys (
			endpoint, actor_id, idempotency_key, request_hash, response_status, response_body, created_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (endpoint, actor_id, idempotency_key) DO UPDATE SET
			request_hash = EXCLUDED.request_hash,
			response_status = EXCLUDED.response_status,
			response_body = EXCLUDED.response_body,
			created_at = EXCLUDED.created_at,
			expires_at = EXCLUDED.expires_at
		WHERE idempotency_keys.expires_at <= NOW()
		RETURNING idempotency_key`

	for attempt := 0; attempt < 3; attempt++ {
		var key string
		err := ex.QueryRowContext(ctx, q,
			rec.Endpoint, rec.ActorID, rec.Key, rec.RequestHash,
			rec.ResponseCode, rec.ResponseBody, rec.CreatedAt, rec.ExpiresAt,
		).Scan(&key)
		if err == nil {
			return nil, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}

		existing, found, err := s.Get(ctx, rec.Endpoint, rec.ActorID, rec.Key)
		if err != nil {
			return nil, false, err
		}
		if found {
			return existing, false, nil
		}
	}
	return nil, false, errors.New("postgres: idempotency claim could not be resolved")
}

func (s *IdempotencyStore) Complete(ctx context.Context, rec app.IdempotencyRecord) error {
	ex := execFromContext(ctx, s.DB)
	const q = `
		UPDATE idempotency_keys
		SET response_status = $4, response_body = $5
		WHERE endpoint = $1 AND actor_id = $2 AND idempotency_key = $3 AND request_hash = $6`
	res, err := ex.ExecContext(ctx, q,
		rec.Endpoint, rec.ActorID, rec.Key, rec.ResponseCode, rec.ResponseBody, rec.RequestHash,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("postgres: idempotency claim not found for %s/%s/%s", rec.Endpoint, rec.ActorID, rec.Key)
	}
	return nil
}
