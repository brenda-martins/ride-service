package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride/internal/domain/ride"
)

const EndpointCreateRide = "POST /v1/rides"

type CreateRideInput struct {
	RiderID        string
	Pickup         ride.GeoPoint
	Dropoff        ride.GeoPoint
	ActorID        string
	IdempotencyKey string
}

type CreateRideOutput struct {
	Ride         *ride.Ride
	Replayed     bool
	ResponseCode int
	ResponseBody []byte
}

type CreateRide struct {
	Rides    RideRepository
	Store    IdempotencyStore
	Tx       TxManager
	Clock    Clock
	IDs      IDGenerator
	TTL      time.Duration
	Estimate func(pickup, dropoff ride.GeoPoint) (ride.Money, error)
}

func (uc *CreateRide) Execute(ctx context.Context, in CreateRideInput) (CreateRideOutput, error) {
	if uc.Estimate == nil {
		return CreateRideOutput{}, fmt.Errorf("fare estimator is required")
	}

	hash, err := createRideRequestHash(in)
	if err != nil {
		return CreateRideOutput{}, err
	}
	cfg := uc.idempotencyConfig(in)
	if err := RequireTransaction(cfg, uc.Tx); err != nil {
		return CreateRideOutput{}, err
	}

	if rec, found, err := LookupIdempotency(ctx, cfg, hash); err != nil {
		return CreateRideOutput{}, err
	} else if found {
		return replayCreateRide(rec)
	}

	r, body, err := uc.buildRide(in)
	if err != nil {
		return CreateRideOutput{}, err
	}

	var out CreateRideOutput
	err = withinTx(ctx, uc.Tx, func(ctx context.Context) error {
		existing, claimed, err := ClaimIdempotency(ctx, cfg, hash)
		if err != nil {
			return err
		}
		if !claimed {
			out, err = replayCreateRide(existing)
			return err
		}
		if err := uc.Rides.Create(ctx, r); err != nil {
			return err
		}
		if err := CompleteIdempotency(ctx, cfg, hash, 201, body); err != nil {
			return err
		}
		out = CreateRideOutput{Ride: r, Replayed: false, ResponseCode: 201, ResponseBody: body}
		return nil
	})
	if err != nil {
		return CreateRideOutput{}, err
	}
	return out, nil
}

func createRideRequestHash(in CreateRideInput) (string, error) {
	return HashRequest(struct {
		RiderID string        `json:"riderId"`
		Pickup  ride.GeoPoint `json:"pickup"`
		Dropoff ride.GeoPoint `json:"dropoff"`
	}{RiderID: in.RiderID, Pickup: in.Pickup, Dropoff: in.Dropoff})
}

func (uc *CreateRide) idempotencyConfig(in CreateRideInput) IdempotencyConfig {
	actorID := in.ActorID
	if actorID == "" {
		actorID = in.RiderID
	}
	return IdempotencyConfig{
		Store:    uc.Store,
		Clock:    uc.Clock,
		Endpoint: EndpointCreateRide,
		ActorID:  actorID,
		Key:      in.IdempotencyKey,
		TTL:      uc.TTL,
	}
}

func (uc *CreateRide) buildRide(in CreateRideInput) (*ride.Ride, []byte, error) {
	estimated, err := uc.Estimate(in.Pickup, in.Dropoff)
	if err != nil {
		return nil, nil, err
	}
	id, err := uc.IDs.NewID()
	if err != nil {
		return nil, nil, err
	}
	r, err := ride.NewRequested(id, in.RiderID, in.Pickup, in.Dropoff, estimated, uc.Clock.Now())
	if err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, nil, err
	}
	return r, body, nil
}

func replayCreateRide(rec *IdempotencyRecord) (CreateRideOutput, error) {
	var r ride.Ride
	if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
		return CreateRideOutput{}, err
	}
	return CreateRideOutput{
		Ride:         &r,
		Replayed:     true,
		ResponseCode: 200,
		ResponseBody: rec.ResponseBody,
	}, nil
}
