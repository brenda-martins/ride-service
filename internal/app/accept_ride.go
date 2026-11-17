package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride/internal/domain/ride"
)

const EndpointAcceptRide = "POST /v1/rides/{rideId}/accept"

// AcceptRideInput contains the data required to accept a ride.
type AcceptRideInput struct {
	RideID          string
	DriverID        string
	ExpectedVersion int64
	ActorID         string
	IdempotencyKey  string
}

// AcceptRideOutput is the result of accepting (or replaying acceptance of) a ride.
type AcceptRideOutput struct {
	Ride         *ride.Ride
	Replayed     bool
	ResponseCode int
	ResponseBody []byte
}

// AcceptRide handles the REQUESTED -> ACCEPTED transition with idempotency support.
type AcceptRide struct {
	Rides        RideRepository
	Availability DriverAvailability
	DriverInfo   DriverInfo
	Store        IdempotencyStore
	Tx           TxManager
	Clock        Clock
	TTL          time.Duration
}

func (uc *AcceptRide) Execute(ctx context.Context, in AcceptRideInput) (AcceptRideOutput, error) {
	actorID := in.ActorID
	if actorID == "" {
		actorID = in.DriverID
	}
	hash, err := HashRequest(struct {
		DriverID        string `json:"driverId"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}{DriverID: in.DriverID, ExpectedVersion: in.ExpectedVersion})
	if err != nil {
		return AcceptRideOutput{}, err
	}

	cfg := IdempotencyConfig{
		Store:    uc.Store,
		Clock:    uc.Clock,
		Endpoint: EndpointAcceptRide,
		ActorID:  actorID,
		Key:      in.IdempotencyKey,
		TTL:      uc.TTL,
	}
	if rec, replay, err := BeginIdempotency(ctx, cfg, hash); err != nil {
		return AcceptRideOutput{}, err
	} else if replay {
		var r ride.Ride
		if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
			return AcceptRideOutput{}, err
		}
		return AcceptRideOutput{Ride: &r, Replayed: true, ResponseCode: 200, ResponseBody: rec.ResponseBody}, nil
	}

	if in.RideID == "" {
		return AcceptRideOutput{}, fmt.Errorf("%w: rideId is required", ride.ErrInvalidArgument)
	}

	// Execute within a transaction: load, check availability, apply domain transition, persist with optimistic locking.
	var result *ride.Ride
	run := func(ctx context.Context) error {
		r, err := uc.Rides.GetByID(ctx, in.RideID)
		if err != nil {
			return err
		}
		available, err := uc.Availability.IsAvailable(ctx, in.DriverID)
		if err != nil {
			return err
		}
		var snap DriverSnapshot
		if uc.DriverInfo != nil {
			snap, err = uc.DriverInfo.GetSnapshot(ctx, in.DriverID)
			if err != nil {
				return err
			}
		}
		now := uc.Clock.Now()
		if err := r.Accept(in.DriverID, in.ExpectedVersion, available, snap.DisplayName, snap.VehiclePlate, now); err != nil {
			return err
		}
		if err := uc.Rides.UpdateOnAccept(ctx, r, in.ExpectedVersion); err != nil {
			return err
		}
		body, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if err := SaveIdempotency(ctx, cfg, hash, 200, body); err != nil {
			return err
		}
		result = r
		return nil
	}
	if uc.Tx != nil {
		if err := uc.Tx.WithinTx(ctx, run); err != nil {
			return AcceptRideOutput{}, err
		}
	} else {
		if err := run(ctx); err != nil {
			return AcceptRideOutput{}, err
		}
	}
	body, err := json.Marshal(result)
	if err != nil {
		return AcceptRideOutput{}, err
	}
	return AcceptRideOutput{Ride: result, Replayed: false, ResponseCode: 200, ResponseBody: body}, nil
}
