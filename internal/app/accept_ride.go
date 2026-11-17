package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride/internal/domain/ride"
)

const endpointAcceptRideTemplate = "POST /v1/rides/%s/accept"

type AcceptRideInput struct {
	RideID          string
	DriverID        string
	ExpectedVersion int64
	ActorID         string
	IdempotencyKey  string
}

type AcceptRideOutput struct {
	Ride         *ride.Ride
	Replayed     bool
	ResponseCode int
	ResponseBody []byte
}

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
	if in.RideID == "" {
		return AcceptRideOutput{}, fmt.Errorf("%w: rideId is required", ride.ErrInvalidArgument)
	}
	if uc.Availability == nil {
		return AcceptRideOutput{}, fmt.Errorf("driver availability is required")
	}

	hash, err := acceptRideRequestHash(in)
	if err != nil {
		return AcceptRideOutput{}, err
	}
	cfg := uc.idempotencyConfig(in)
	if err := RequireTransaction(cfg, uc.Tx); err != nil {
		return AcceptRideOutput{}, err
	}

	if rec, found, err := LookupIdempotency(ctx, cfg, hash); err != nil {
		return AcceptRideOutput{}, err
	} else if found {
		return replayAcceptRide(rec)
	}

	available, err := uc.Availability.IsAvailable(ctx, in.DriverID)
	if err != nil {
		return AcceptRideOutput{}, err
	}
	snap, err := uc.driverSnapshot(ctx, in.DriverID)
	if err != nil {
		return AcceptRideOutput{}, err
	}

	var out AcceptRideOutput
	err = withinTx(ctx, uc.Tx, func(ctx context.Context) error {
		existing, claimed, err := ClaimIdempotency(ctx, cfg, hash)
		if err != nil {
			return err
		}
		if !claimed {
			out, err = replayAcceptRide(existing)
			return err
		}

		r, err := uc.applyAcceptance(ctx, in, available, snap)
		if err != nil {
			return err
		}
		if err := uc.Rides.Update(ctx, r, in.ExpectedVersion); err != nil {
			return err
		}
		body, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if err := CompleteIdempotency(ctx, cfg, hash, 200, body); err != nil {
			return err
		}
		out = AcceptRideOutput{Ride: r, Replayed: false, ResponseCode: 200, ResponseBody: body}
		return nil
	})
	if err != nil {
		return AcceptRideOutput{}, err
	}
	return out, nil
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
		Endpoint: fmt.Sprintf(endpointAcceptRideTemplate, in.RideID),
		ActorID:  actorID,
		Key:      in.IdempotencyKey,
		TTL:      uc.TTL,
	}
}

func (uc *AcceptRide) applyAcceptance(ctx context.Context, in AcceptRideInput, available bool, snap DriverSnapshot) (*ride.Ride, error) {
	r, err := uc.Rides.GetByID(ctx, in.RideID)
	if err != nil {
		return nil, err
	}
	if err := r.Accept(in.DriverID, in.ExpectedVersion, available, snap.DisplayName, snap.VehiclePlate, uc.Clock.Now()); err != nil {
		return nil, err
	}
	return r, nil
}

func replayAcceptRide(rec *IdempotencyRecord) (AcceptRideOutput, error) {
	var r ride.Ride
	if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
		return AcceptRideOutput{}, err
	}
	return AcceptRideOutput{
		Ride:         &r,
		Replayed:     true,
		ResponseCode: 200,
		ResponseBody: rec.ResponseBody,
	}, nil
}

func (uc *AcceptRide) driverSnapshot(ctx context.Context, driverID string) (DriverSnapshot, error) {
	if uc.DriverInfo == nil {
		return DriverSnapshot{}, nil
	}
	return uc.DriverInfo.GetSnapshot(ctx, driverID)
}
