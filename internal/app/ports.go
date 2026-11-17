package app

import (
	"context"
	"time"

	"ride/internal/domain/ride"
)

// RideRepository persists and retrieves ride aggregates.
type RideRepository interface {
	Create(ctx context.Context, r *ride.Ride) error
	GetByID(ctx context.Context, id string) (*ride.Ride, error)
	// Update persists a transition using optimistic locking. Implementations
	// must ensure the update affects exactly one row matching id, expectedVersion
	// and the transition's source status; otherwise return
	// ride.ErrVersionConflict or ride.ErrIllegalTransition as applicable.
	Update(ctx context.Context, r *ride.Ride, expectedVersion int64) error
}

// DriverAvailability checks whether a driver can accept rides.
type DriverAvailability interface {
	IsAvailable(ctx context.Context, driverID string) (bool, error)
}

// DriverSnapshot provides driver display data captured at acceptance time.
type DriverSnapshot struct {
	DisplayName  string
	VehiclePlate string
}

// DriverInfo resolves snapshot information for a driver at acceptance.
// In early phases this may be backed by a stub.
type DriverInfo interface {
	GetSnapshot(ctx context.Context, driverID string) (DriverSnapshot, error)
}

// IdempotencyRecord stores a previously computed command response for replay.
type IdempotencyRecord struct {
	Endpoint     string
	ActorID      string
	Key          string
	RequestHash  string
	ResponseCode int
	ResponseBody []byte
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// IdempotencyStore persists and retrieves idempotency records.
type IdempotencyStore interface {
	Get(ctx context.Context, endpoint, actorID, key string) (*IdempotencyRecord, bool, error)

	// Claim atomically reserves the key for a new request. It must take over a
	// record whose expires_at is in the past.
	// When a live record already exists it returns it with claimed=false.
	// When the key is claimed it returns (nil, true, nil).
	Claim(ctx context.Context, rec IdempotencyRecord) (existing *IdempotencyRecord, claimed bool, err error)

	// Complete fills in the response of a record previously claimed by the
	// same request. It must run in the same transaction as the command effect.
	Complete(ctx context.Context, rec IdempotencyRecord) error
}

// TxManager runs a function within a transaction boundary when supported by adapters.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

// IDGenerator generates identifiers for new resources.
type IDGenerator interface {
	NewID() (string, error)
}
