package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ride/internal/domain/ride"
)

type RideRepository struct {
	DB *sql.DB
}

func (r *RideRepository) Create(ctx context.Context, rd *ride.Ride) error {
	ex := execFromContext(ctx, r.DB)
	const q = `
		INSERT INTO rides (
			id, status, rider_id, driver_id,
			pickup_lat, pickup_lng, pickup_address,
			dropoff_lat, dropoff_lng, dropoff_address,
			fare_estimated_amount_minor, fare_estimated_currency,
			fare_final_amount_minor, fare_final_currency,
			driver_display_name, vehicle_plate,
			cancellation_actor_type, cancellation_actor_id, cancellation_reason, cancellation_cancelled_at,
			version, created_at, updated_at, accepted_at, started_at, completed_at
		) VALUES (
			$1,$2,$3,$4,
			$5,$6,$7,
			$8,$9,$10,
			$11,$12,
			$13,$14,
			$15,$16,
			$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26
		)`
	_, err := ex.ExecContext(ctx, q,
		rd.ID, string(rd.Status), rd.RiderID, nullString(rd.DriverID),
		rd.Pickup.Lat, rd.Pickup.Lng, nullString(rd.Pickup.Address),
		rd.Dropoff.Lat, rd.Dropoff.Lng, nullString(rd.Dropoff.Address),
		rd.Fare.Estimated.AmountMinor, rd.Fare.Estimated.Currency,
		nullInt64Ptr(moneyAmount(rd.Fare.Final)), nullStringPtr(moneyCurrency(rd.Fare.Final)),
		nullString(rd.DriverDisplayName), nullString(rd.VehiclePlate),
		nil, nil, nil, nil,
		rd.Version, rd.CreatedAt, rd.UpdatedAt, rd.AcceptedAt, rd.StartedAt, rd.CompletedAt,
	)
	return err
}

func (r *RideRepository) GetByID(ctx context.Context, id string) (*ride.Ride, error) {
	ex := execFromContext(ctx, r.DB)
	const q = `
		SELECT
			id, 
			status, 
			rider_id, 
			driver_id,
			pickup_lat, 
			pickup_lng, 
			pickup_address,
			dropoff_lat, 
			dropoff_lng, 
			dropoff_address,
			fare_estimated_amount_minor, 
			fare_estimated_currency,
			fare_final_amount_minor, 
			fare_final_currency,
			driver_display_name, 
			vehicle_plate,
			version, 
			created_at, 
			updated_at, 
			accepted_at, 
			started_at, 
			completed_at
		FROM rides WHERE id = $1`
	row := ex.QueryRowContext(ctx, q, id)

	var rd ride.Ride
	var status string
	var driverID, pickupAddr, dropoffAddr, driverName, vehiclePlate sql.NullString
	var finalAmount sql.NullInt64
	var finalCurrency sql.NullString
	var acceptedAt, startedAt, completedAt sql.NullTime

	err := row.Scan(
		&rd.ID, &status, &rd.RiderID, &driverID,
		&rd.Pickup.Lat, &rd.Pickup.Lng, &pickupAddr,
		&rd.Dropoff.Lat, &rd.Dropoff.Lng, &dropoffAddr,
		&rd.Fare.Estimated.AmountMinor, &rd.Fare.Estimated.Currency,
		&finalAmount, &finalCurrency,
		&driverName, &vehiclePlate,
		&rd.Version, &rd.CreatedAt, &rd.UpdatedAt, &acceptedAt, &startedAt, &completedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ride.ErrNotFound
		}
		return nil, err
	}

	rd.Status = ride.Status(status)
	rd.DriverID = driverID.String
	rd.Pickup.Address = pickupAddr.String
	rd.Dropoff.Address = dropoffAddr.String
	rd.DriverDisplayName = driverName.String
	rd.VehiclePlate = vehiclePlate.String
	if finalAmount.Valid && finalCurrency.Valid {
		rd.Fare.Final = &ride.Money{AmountMinor: finalAmount.Int64, Currency: finalCurrency.String}
	}
	if acceptedAt.Valid {
		t := acceptedAt.Time
		rd.AcceptedAt = &t
	}
	if startedAt.Valid {
		t := startedAt.Time
		rd.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		rd.CompletedAt = &t
	}
	return &rd, nil
}

// Update persists the REQUESTED -> ACCEPTED transition using optimistic
// locking. It only updates a row that is still REQUESTED and matches
// expectedVersion; otherwise it reports the appropriate domain error.
func (r *RideRepository) Update(ctx context.Context, rd *ride.Ride, expectedVersion int64) error {
	ex := execFromContext(ctx, r.DB)
	const q = `
		UPDATE rides SET
			status = $2,
			driver_id = $3,
			driver_display_name = $4,
			vehicle_plate = $5,
			version = $6,
			updated_at = $7,
			accepted_at = $8
		WHERE id = $1 AND version = $9 AND status = $10`
	res, err := ex.ExecContext(ctx, q,
		rd.ID, string(rd.Status), nullString(rd.DriverID),
		nullString(rd.DriverDisplayName), nullString(rd.VehiclePlate),
		rd.Version, rd.UpdatedAt, rd.AcceptedAt,
		expectedVersion, string(ride.StatusRequested),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		current, err := r.GetByID(ctx, rd.ID)
		if err != nil {
			return err
		}
		switch {
		case current.Status.IsTerminal():
			return fmt.Errorf("%w: ride is %s", ride.ErrTerminal, current.Status)
		case current.Status == ride.StatusAccepted:
			return fmt.Errorf("%w: ride is already assigned to a driver", ride.ErrAlreadyAccepted)
		case current.Status != ride.StatusRequested:
			return fmt.Errorf("%w: %s cannot transition to ACCEPTED", ride.ErrIllegalTransition, current.Status)
		default:
			return ride.ErrVersionConflict
		}
	}
	return nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullStringPtr(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func nullInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func moneyAmount(m *ride.Money) *int64 {
	if m == nil {
		return nil
	}
	v := m.AmountMinor
	return &v
}

func moneyCurrency(m *ride.Money) *string {
	if m == nil {
		return nil
	}
	v := m.Currency
	return &v
}
