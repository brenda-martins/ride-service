package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ride/internal/domain/ride"
)

const EndpointCreateRide = "POST /v1/rides"

// CreateRideInput contains the data required to request a new ride.
type CreateRideInput struct {
	RiderID        string
	Pickup         ride.GeoPoint
	Dropoff        ride.GeoPoint
	ActorID        string
	IdempotencyKey string
}

// CreateRideOutput is the result of creating (or replaying) a ride request.
type CreateRideOutput struct {
	Ride         *ride.Ride
	Replayed     bool
	ResponseCode int
	ResponseBody []byte
}

// CreateRide handles ride creation with idempotency support.
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
	actorID := in.ActorID
	if actorID == "" {
		actorID = in.RiderID
	}
	hash, err := HashRequest(struct {
		RiderID string        `json:"riderId"`
		Pickup  ride.GeoPoint `json:"pickup"`
		Dropoff ride.GeoPoint `json:"dropoff"`
	}{RiderID: in.RiderID, Pickup: in.Pickup, Dropoff: in.Dropoff})
	if err != nil {
		return CreateRideOutput{}, err
	}

	cfg := IdempotencyConfig{
		Store:    uc.Store,
		Clock:    uc.Clock,
		Endpoint: EndpointCreateRide,
		ActorID:  actorID,
		Key:      in.IdempotencyKey,
		TTL:      uc.TTL,
	}
	if rec, replay, err := BeginIdempotency(ctx, cfg, hash); err != nil {
		return CreateRideOutput{}, err
	} else if replay {
		var r ride.Ride
		if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
			return CreateRideOutput{}, err
		}
		return CreateRideOutput{Ride: &r, Replayed: true, ResponseCode: 200, ResponseBody: rec.ResponseBody}, nil
	}

	estimated, err := uc.Estimate(in.Pickup, in.Dropoff)
	if err != nil {
		return CreateRideOutput{}, err
	}
	id, err := uc.IDs.NewID()
	if err != nil {
		return CreateRideOutput{}, err
	}
	now := uc.Clock.Now()
	r, err := ride.NewRequested(id, in.RiderID, in.Pickup, in.Dropoff, estimated, now)
	if err != nil {
		return CreateRideOutput{}, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return CreateRideOutput{}, err
	}

	run := func(ctx context.Context) error {
		if err := uc.Rides.Create(ctx, r); err != nil {
			return err
		}
		return SaveIdempotency(ctx, cfg, hash, 201, body)
	}
	if uc.Tx != nil {
		err = uc.Tx.WithinTx(ctx, run)
	} else {
		err = run(ctx)
	}
	if err != nil {
		return CreateRideOutput{}, err
	}
	return CreateRideOutput{Ride: r, Replayed: false, ResponseCode: 201, ResponseBody: body}, nil
}
