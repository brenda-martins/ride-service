package app

import (
	"context"

	"ride/internal/domain/ride"
)

type GetRide struct {
	Rides RideRepository
}

func (uc *GetRide) Execute(ctx context.Context, id string) (*ride.Ride, error) {
	return uc.Rides.GetByID(ctx, id)
}
