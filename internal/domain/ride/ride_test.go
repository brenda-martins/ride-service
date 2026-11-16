package ride

import (
	"errors"
	"testing"
	"time"
)

func testNow() time.Time {
	return time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
}

func validPickup() GeoPoint  { return GeoPoint{Lat: -23.55, Lng: -46.63, Address: "Origem"} }
func validDropoff() GeoPoint { return GeoPoint{Lat: -23.56, Lng: -46.64, Address: "Destino"} }
func validFare() Money       { return Money{AmountMinor: 2500, Currency: "BRL"} }

func newRequestedRide(t *testing.T) *Ride {
	t.Helper()
	r, err := NewRequested("ride-1", "rider-1", validPickup(), validDropoff(), validFare(), testNow())
	if err != nil {
		t.Fatalf("NewRequested: %v", err)
	}
	return r
}

func TestNewRequested(t *testing.T) {
	r := newRequestedRide(t)

	if r.Status != StatusRequested {
		t.Fatalf("status = %s, want %s", r.Status, StatusRequested)
	}
	if r.Version != 1 {
		t.Fatalf("version = %d, want 1", r.Version)
	}
	if r.RiderID != "rider-1" {
		t.Fatalf("riderId = %q, want rider-1", r.RiderID)
	}
	if r.Fare.Estimated.AmountMinor != 2500 || r.Fare.Estimated.Currency != "BRL" {
		t.Fatalf("estimated fare = %+v, want 2500 BRL", r.Fare.Estimated)
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		t.Fatal("timestamps must be set")
	}
	if r.DriverID != "" || r.AcceptedAt != nil {
		t.Fatal("new requested ride must not have driver assignment")
	}
}

func TestNewRequestedValidation(t *testing.T) {
	now := testNow()
	tests := []struct {
		name    string
		id      string
		riderID string
		pickup  GeoPoint
		dropoff GeoPoint
		fare    Money
		at      time.Time
	}{
		{"missing id", "", "rider-1", validPickup(), validDropoff(), validFare(), now},
		{"missing rider", "ride-1", "", validPickup(), validDropoff(), validFare(), now},
		{"invalid pickup lat", "ride-1", "rider-1", GeoPoint{Lat: 91, Lng: 0}, validDropoff(), validFare(), now},
		{"invalid fare currency", "ride-1", "rider-1", validPickup(), validDropoff(), Money{AmountMinor: 1, Currency: "brl"}, now},
		{"negative fare", "ride-1", "rider-1", validPickup(), validDropoff(), Money{AmountMinor: -1, Currency: "BRL"}, now},
		{"zero time", "ride-1", "rider-1", validPickup(), validDropoff(), validFare(), time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRequested(tt.id, tt.riderID, tt.pickup, tt.dropoff, tt.fare, tt.at)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got %v", err)
			}
		})
	}
}

func TestAcceptSuccess(t *testing.T) {
	r := newRequestedRide(t)
	at := testNow().Add(2 * time.Minute)

	err := r.Accept("driver-1", 1, true, "Maria", "ABC-1234", at)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if r.Status != StatusAccepted {
		t.Fatalf("status = %s, want %s", r.Status, StatusAccepted)
	}
	if r.DriverID != "driver-1" {
		t.Fatalf("driverId = %q, want driver-1", r.DriverID)
	}
	if r.DriverDisplayName != "Maria" || r.VehiclePlate != "ABC-1234" {
		t.Fatalf("snapshot = (%q,%q), want (Maria,ABC-1234)", r.DriverDisplayName, r.VehiclePlate)
	}
	if r.Version != 2 {
		t.Fatalf("version = %d, want 2", r.Version)
	}
	if r.AcceptedAt == nil || !r.AcceptedAt.Equal(at.UTC()) {
		t.Fatalf("acceptedAt = %v, want %v", r.AcceptedAt, at.UTC())
	}
}

func TestAcceptTransitionTable(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*Ride)
		driverID        string
		expectedVersion int64
		available       bool
		wantErr         error
	}{
		{
			name:            "version conflict",
			driverID:        "driver-1",
			expectedVersion: 999,
			available:       true,
			wantErr:         ErrVersionConflict,
		},
		{
			name:            "driver unavailable",
			driverID:        "driver-1",
			expectedVersion: 1,
			available:       false,
			wantErr:         ErrDriverUnavailable,
		},
		{
			name:            "missing driver id",
			driverID:        "",
			expectedVersion: 1,
			available:       true,
			wantErr:         ErrInvalidArgument,
		},
		{
			name: "illegal transition from accepted",
			setup: func(r *Ride) {
				r.Status = StatusAccepted
				r.Version = 2
			},
			driverID:        "driver-2",
			expectedVersion: 2,
			available:       true,
			wantErr:         ErrIllegalTransition,
		},
		{
			name: "illegal transition from completed",
			setup: func(r *Ride) {
				r.Status = StatusCompleted
			},
			driverID:        "driver-1",
			expectedVersion: 1,
			available:       true,
			wantErr:         ErrIllegalTransition,
		},
		{
			name: "illegal transition from cancelled",
			setup: func(r *Ride) {
				r.Status = StatusCancelled
			},
			driverID:        "driver-1",
			expectedVersion: 1,
			available:       true,
			wantErr:         ErrIllegalTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRequestedRide(t)
			if tt.setup != nil {
				tt.setup(r)
			}
			err := r.Accept(tt.driverID, tt.expectedVersion, tt.available, "Name", "PLATE", testNow().Add(time.Minute))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatusTerminal(t *testing.T) {
	if !StatusCompleted.IsTerminal() || !StatusCancelled.IsTerminal() {
		t.Fatal("completed and cancelled must be terminal")
	}
	for _, s := range []Status{StatusRequested, StatusAccepted, StatusInProgress} {
		if s.IsTerminal() {
			t.Fatalf("%s must not be terminal", s)
		}
	}
}
