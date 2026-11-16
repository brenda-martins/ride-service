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
	// UpdateOnAccept persists an acceptance transition using optimistic locking.
	// Implementations must ensure the update affects exactly one row matching
	// id, expectedVersion and REQUESTED status; otherwise return ride.ErrVersionConflict
	// or ride.ErrIllegalTransition/appropriate domain error context as applicable.
	// UpdateOnAccept(ctx context.Context, r *ride.Ride, expectedVersion int64) error
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
	Save(ctx context.Context, rec IdempotencyRecord) error
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
