package postgres

import (
	"context"
	"database/sql"
	"errors"

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

func (s *IdempotencyStore) Save(ctx context.Context, rec app.IdempotencyRecord) error {
	ex := execFromContext(ctx, s.DB)
	const q = `
		INSERT INTO idempotency_keys (
			endpoint, actor_id, idempotency_key, request_hash, response_status, response_body, created_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (endpoint, actor_id, idempotency_key) DO NOTHING`
	_, err := ex.ExecContext(ctx, q,
		rec.Endpoint, rec.ActorID, rec.Key, rec.RequestHash, rec.ResponseCode, rec.ResponseBody, rec.CreatedAt, rec.ExpiresAt,
	)
	return err
}
