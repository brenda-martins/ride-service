package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var ErrIdempotencyKeyReused = errors.New("idempotency key reused with different request")

type IdempotencyConfig struct {
	Store    IdempotencyStore
	Clock    Clock
	Endpoint string
	ActorID  string
	Key      string
	TTL      time.Duration
}

// HashRequest returns a stable hash of the canonical JSON encoding of v.
func HashRequest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// LookupIdempotency returns the stored response for a previous request with the
// same key and hash. It is a read-only fast path that keeps pure replays out of
// the write transaction; the unique index still serializes concurrent claims.
func LookupIdempotency(ctx context.Context, cfg IdempotencyConfig, requestHash string) (*IdempotencyRecord, bool, error) {
	if cfg.Store == nil || cfg.Key == "" {
		return nil, false, nil
	}
	rec, found, err := cfg.Store.Get(ctx, cfg.Endpoint, cfg.ActorID, cfg.Key)
	if err != nil || !found {
		return nil, false, err
	}
	if rec.RequestHash != requestHash {
		return rec, false, ErrIdempotencyKeyReused
	}
	return rec, true, nil
}

// ClaimIdempotency reserves the key inside the command transaction. It returns
// (nil, true, nil) when this request owns the key and must execute the effect.
// It returns (record, false, nil) when a live record already exists and the
// caller must replay it instead of executing the effect.
func ClaimIdempotency(ctx context.Context, cfg IdempotencyConfig, requestHash string) (*IdempotencyRecord, bool, error) {
	if cfg.Store == nil || cfg.Key == "" {
		return nil, true, nil
	}
	existing, claimed, err := cfg.Store.Claim(ctx, newIdempotencyRecord(cfg, requestHash))
	if err != nil {
		return nil, false, err
	}
	if claimed {
		return nil, true, nil
	}
	if existing == nil {
		return nil, false, errors.New("idempotency: claim returned neither ownership nor a record")
	}
	if existing.RequestHash != requestHash {
		return existing, false, ErrIdempotencyKeyReused
	}
	// A live record with the same hash is a replay, not a claim.
	return existing, false, nil
}

// CompleteIdempotency stores the response for a key owned by this request.
// It must run in the same transaction as the effect.
func CompleteIdempotency(ctx context.Context, cfg IdempotencyConfig, requestHash string, status int, body []byte) error {
	if cfg.Store == nil || cfg.Key == "" {
		return nil
	}
	return cfg.Store.Complete(ctx, IdempotencyRecord{
		Endpoint:     cfg.Endpoint,
		ActorID:      cfg.ActorID,
		Key:          cfg.Key,
		RequestHash:  requestHash,
		ResponseCode: status,
		ResponseBody: body,
	})
}

// RequireTransaction fails closed when idempotency is enabled without a
// transaction manager: the claim, the effect and the completed response would
// not commit atomically.
func RequireTransaction(cfg IdempotencyConfig, tx TxManager) error {
	if cfg.Store == nil || cfg.Key == "" {
		return nil
	}
	if tx == nil {
		return errors.New("idempotency requires a transaction manager")
	}
	return nil
}

func newIdempotencyRecord(cfg IdempotencyConfig, requestHash string) IdempotencyRecord {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := cfg.Clock.Now().UTC()
	return IdempotencyRecord{
		Endpoint:     cfg.Endpoint,
		ActorID:      cfg.ActorID,
		Key:          cfg.Key,
		RequestHash:  requestHash,
		ResponseCode: 0,
		// Placeholder for the NOT NULL JSONB column; Complete fills the real
		// response before the transaction commits.
		ResponseBody: []byte("{}"),
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
	}
}
