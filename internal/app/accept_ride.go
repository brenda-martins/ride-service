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
	hash, err := acceptRideRequestHash(in)
	if err != nil {
		return AcceptRideOutput{}, err
	}
	cfg := uc.idempotencyConfig(in)

	if out, replayed, err := uc.replay(ctx, cfg, hash); err != nil {
		return AcceptRideOutput{}, err
	} else if replayed {
		return out, nil
	}

	if in.RideID == "" {
		return AcceptRideOutput{}, fmt.Errorf("%w: rideId is required", ride.ErrInvalidArgument)
	}

	r, body, err := uc.accept(ctx, cfg, hash, in)
	if err != nil {
		return AcceptRideOutput{}, err
	}
	return AcceptRideOutput{Ride: r, Replayed: false, ResponseCode: 200, ResponseBody: body}, nil
}

func acceptRideRequestHash(in AcceptRideInput) (string, error) {
	return HashRequest(struct {
		RideID          string `json:"rideId"`
		DriverID        string `json:"driverId"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}{RideID: in.RideID, DriverID: in.DriverID, ExpectedVersion: in.ExpectedVersion})
}

func (uc *AcceptRide) idempotencyConfig(in AcceptRideInput) IdempotencyConfig {
	actorID := in.ActorID
	if actorID == "" {
		actorID = in.DriverID
	}
	return IdempotencyConfig{
		Store:    uc.Store,
		Clock:    uc.Clock,
		Endpoint: EndpointAcceptRide,
		ActorID:  actorID,
		Key:      in.IdempotencyKey,
		TTL:      uc.TTL,
	}
}

func (uc *AcceptRide) replay(ctx context.Context, cfg IdempotencyConfig, hash string) (AcceptRideOutput, bool, error) {
	rec, replayed, err := BeginIdempotency(ctx, cfg, hash)
	if err != nil || !replayed {
		return AcceptRideOutput{}, false, err
	}
	var r ride.Ride
	if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
		return AcceptRideOutput{}, false, err
	}
	return AcceptRideOutput{
		Ride:         &r,
		Replayed:     true,
		ResponseCode: 200,
		ResponseBody: rec.ResponseBody,
	}, true, nil
}

func (uc *AcceptRide) accept(ctx context.Context, cfg IdempotencyConfig, hash string, in AcceptRideInput) (*ride.Ride, []byte, error) {
	var (
		result *ride.Ride
		body   []byte
	)
	err := withinTx(ctx, uc.Tx, func(ctx context.Context) error {
		r, err := uc.applyAcceptance(ctx, in)
		if err != nil {
			return err
		}
		if err := uc.Rides.Update(ctx, r, in.ExpectedVersion); err != nil {
			return err
		}
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if err := SaveIdempotency(ctx, cfg, hash, 200, b); err != nil {
			return err
		}
		result, body = r, b
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return result, body, nil
}

func (uc *AcceptRide) applyAcceptance(ctx context.Context, in AcceptRideInput) (*ride.Ride, error) {
	r, err := uc.Rides.GetByID(ctx, in.RideID)
	if err != nil {
		return nil, err
	}

	available, err := uc.Availability.IsAvailable(ctx, in.DriverID)
	if err != nil {
		return nil, err
	}

	snap, err := uc.driverSnapshot(ctx, in.DriverID)
	if err != nil {
		return nil, err
	}

	if err := r.Accept(in.DriverID, in.ExpectedVersion, available, snap.DisplayName, snap.VehiclePlate, uc.Clock.Now()); err != nil {
		return nil, err
	}
	return r, nil
}

func (uc *AcceptRide) driverSnapshot(ctx context.Context, driverID string) (DriverSnapshot, error) {
	if uc.DriverInfo == nil {
		return DriverSnapshot{}, nil
	}
	return uc.DriverInfo.GetSnapshot(ctx, driverID)
}
