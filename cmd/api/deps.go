package main

import (
	"database/sql"
	"time"

	"ride/internal/adapters/driverclient"
	ridepg "ride/internal/adapters/postgres"
	"ride/internal/app"
	"ride/internal/domain/ride"
	"ride/internal/httpapi"
	"ride/internal/platform/clock"
	"ride/internal/platform/id"
)

const idempotencyTTL = 24 * time.Hour

func buildDependencies(db *sql.DB) httpapi.Dependencies {
	clk := clock.Real{}
	ids := id.UUIDv7{}

	ridesRepo := &ridepg.RideRepository{DB: db}
	idemStore := &ridepg.IdempotencyStore{DB: db}
	txMgr := &ridepg.TxManager{DB: db}
	driverStub := &driverclient.Stub{}

	return httpapi.Dependencies{
		CreateRide: &app.CreateRide{
			Rides:    ridesRepo,
			Store:    idemStore,
			Tx:       txMgr,
			Clock:    clk,
			IDs:      ids,
			TTL:      idempotencyTTL,
			Estimate: fixedFareEstimator,
		},
		GetRide: &app.GetRide{Rides: ridesRepo},
		AcceptRide: &app.AcceptRide{
			Rides:        ridesRepo,
			Availability: driverStub,
			DriverInfo:   driverStub,
			Store:        idemStore,
			Tx:           txMgr,
			Clock:        clk,
			TTL:          idempotencyTTL,
		},
	}
}

func fixedFareEstimator(_, _ ride.GeoPoint) (ride.Money, error) {
	return ride.Money{AmountMinor: 2500, Currency: "BRL"}, nil
}
