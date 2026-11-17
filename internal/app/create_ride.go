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

	hash, err := createRideRequestHash(in)
	if err != nil {
		return CreateRideOutput{}, err
	}
	cfg := uc.idempotencyConfig(in)

	if out, replayed, err := uc.replay(ctx, cfg, hash); err != nil {
		return CreateRideOutput{}, err
	} else if replayed {
		return out, nil
	}

	r, body, err := uc.buildRide(in)
	if err != nil {
		return CreateRideOutput{}, err
	}
	if err := uc.persist(ctx, cfg, hash, r, body); err != nil {
		return CreateRideOutput{}, err
	}

	return CreateRideOutput{Ride: r, Replayed: false, ResponseCode: 201, ResponseBody: body}, nil
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

func (uc *CreateRide) replay(ctx context.Context, cfg IdempotencyConfig, hash string) (CreateRideOutput, bool, error) {
	rec, replayed, err := BeginIdempotency(ctx, cfg, hash)
	if err != nil || !replayed {
		return CreateRideOutput{}, false, err
	}
	var r ride.Ride
	if err := json.Unmarshal(rec.ResponseBody, &r); err != nil {
		return CreateRideOutput{}, false, err
	}
	return CreateRideOutput{
		Ride:         &r,
		Replayed:     true,
		ResponseCode: 200,
		ResponseBody: rec.ResponseBody,
	}, true, nil
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

func (uc *CreateRide) persist(ctx context.Context, cfg IdempotencyConfig, hash string, r *ride.Ride, body []byte) error {
	return withinTx(ctx, uc.Tx, func(ctx context.Context) error {
		if err := uc.Rides.Create(ctx, r); err != nil {
			return err
		}
		return SaveIdempotency(ctx, cfg, hash, 201, body)
	})
}
