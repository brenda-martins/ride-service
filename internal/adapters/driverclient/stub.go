package driverclient

import (
	"context"

	"ride/internal/app"
)

// Stub is a Phase 1 in-memory stub for driver availability and snapshot info.
// It reports all drivers as available unless explicitly marked unavailable.
type Stub struct {
	Unavailable map[string]bool
	Snapshots   map[string]app.DriverSnapshot
}

func (s *Stub) IsAvailable(_ context.Context, driverID string) (bool, error) {
	if s != nil && s.Unavailable != nil && s.Unavailable[driverID] {
		return false, nil
	}
	return true, nil
}

func (s *Stub) GetSnapshot(_ context.Context, driverID string) (app.DriverSnapshot, error) {
	if s != nil && s.Snapshots != nil {
		if snap, ok := s.Snapshots[driverID]; ok {
			return snap, nil
		}
	}
	// Default placeholder snapshot for Phase 1 stub.
	return app.DriverSnapshot{DisplayName: "Driver " + driverID, VehiclePlate: "UNKNOWN"}, nil
}
