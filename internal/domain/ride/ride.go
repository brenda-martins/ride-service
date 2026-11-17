package ride

import (
	"fmt"
	"time"
)

// Ride is the aggregate root for the ride context.
// Key invariants: valid transitions (INV-2), at most one driver assigned on
// acceptance (INV-1), version for optimistic locking, and immutability after a
// terminal state (INV-5).
type Ride struct {
	ID     string `json:"id"`
	Status Status `json:"status"`

	RiderID  string `json:"riderId"`
	DriverID string `json:"driverId,omitempty"`

	Pickup  GeoPoint `json:"pickup"`
	Dropoff GeoPoint `json:"dropoff"`

	Fare Fare `json:"fare"`

	DriverDisplayName string `json:"driverDisplayName,omitempty"`
	VehiclePlate      string `json:"vehiclePlate,omitempty"`

	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	AcceptedAt  *time.Time `json:"acceptedAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// NewRequested creates a ride in the REQUESTED state (start of the lifecycle).
// The caller is responsible for providing id, timestamps, and the estimated fare.
func NewRequested(id, riderID string, pickup, dropoff GeoPoint, estimatedFare Money, now time.Time) (*Ride, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidArgument)
	}
	if riderID == "" {
		return nil, fmt.Errorf("%w: riderId is required", ErrInvalidArgument)
	}
	if err := pickup.Validate(); err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	if err := dropoff.Validate(); err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	if err := estimatedFare.Validate(); err != nil {
		return nil, fmt.Errorf("estimated fare: %w", err)
	}
	if now.IsZero() {
		return nil, fmt.Errorf("%w: createdAt is required", ErrInvalidArgument)
	}

	fare := Fare{Estimated: estimatedFare}
	if err := fare.Validate(); err != nil {
		return nil, err
	}

	return &Ride{
		ID:        id,
		Status:    StatusRequested,
		RiderID:   riderID,
		Pickup:    pickup,
		Dropoff:   dropoff,
		Fare:      fare,
		Version:   1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}, nil
}

// Accept applies the REQUESTED -> ACCEPTED transition.
// expectedVersion implements optimistic locking and must match the current version.
// driverAvailable reflects the availability check.
// driverDisplayName and vehiclePlate are historical snapshots captured at acceptance.
func (r *Ride) Accept(driverID string, expectedVersion int64, driverAvailable bool, driverDisplayName, vehiclePlate string, now time.Time) error {
	if r.Status.IsTerminal() {
		return fmt.Errorf("%w: ride is %s", ErrTerminal, r.Status)
	}
	if r.Status == StatusAccepted {
		return fmt.Errorf("%w: ride is already assigned to a driver", ErrAlreadyAccepted)
	}
	if r.Status != StatusRequested {
		return fmt.Errorf("%w: %s cannot transition to ACCEPTED", ErrIllegalTransition, r.Status)
	}
	if expectedVersion != r.Version {
		return ErrVersionConflict
	}
	if driverID == "" {
		return fmt.Errorf("%w: driverId is required", ErrInvalidArgument)
	}
	if !driverAvailable {
		return ErrDriverUnavailable
	}
	if now.IsZero() {
		return fmt.Errorf("%w: acceptedAt is required", ErrInvalidArgument)
	}

	t := now.UTC()
	r.Status = StatusAccepted
	r.DriverID = driverID
	r.DriverDisplayName = driverDisplayName
	r.VehiclePlate = vehiclePlate
	r.AcceptedAt = &t
	r.Version++
	r.UpdatedAt = t
	return nil
}

// CanBeAcceptedBy reports whether the ride is eligible for acceptance without
// applying side effects. Useful for precondition checks.
func (r *Ride) CanBeAcceptedBy(driverID string, expectedVersion int64) error {
	if r.Status != StatusRequested {
		return ErrIllegalTransition
	}
	if expectedVersion != r.Version {
		return ErrVersionConflict
	}
	if driverID == "" {
		return fmt.Errorf("%w: driverId is required", ErrInvalidArgument)
	}
	return nil
}
