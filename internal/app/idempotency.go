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

// HashRequest returns a stable hash of the canonical JSON encoding of v.
func HashRequest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// IdempotencyConfig configures idempotency behavior for a command.
type IdempotencyConfig struct {
	Store    IdempotencyStore
	Clock    Clock
	Endpoint string
	ActorID  string
	Key      string
	TTL      time.Duration
}

// BeginIdempotency checks for an existing idempotency record.
// It returns (record, true, nil) when a replay is available for the same request hash.
// It returns (_, _, ErrIdempotencyKeyReused) when the key exists with a different hash.
func BeginIdempotency(ctx context.Context, cfg IdempotencyConfig, requestHash string) (*IdempotencyRecord, bool, error) {
	if cfg.Store == nil || cfg.Key == "" {
		return nil, false, nil
	}
	rec, found, err := cfg.Store.Get(ctx, cfg.Endpoint, cfg.ActorID, cfg.Key)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	if rec.RequestHash != requestHash {
		return rec, false, ErrIdempotencyKeyReused
	}
	return rec, true, nil
}

// SaveIdempotency stores a command response for future replays.
func SaveIdempotency(ctx context.Context, cfg IdempotencyConfig, requestHash string, status int, body []byte) error {
	if cfg.Store == nil || cfg.Key == "" {
		return nil
	}
	now := cfg.Clock.Now().UTC()
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return cfg.Store.Save(ctx, IdempotencyRecord{
		Endpoint:     cfg.Endpoint,
		ActorID:      cfg.ActorID,
		Key:          cfg.Key,
		RequestHash:  requestHash,
		ResponseCode: status,
		ResponseBody: body,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
	})
}
