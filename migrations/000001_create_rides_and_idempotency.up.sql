-- Base schema for ride-service (Phase 1, slice C).
-- Money is stored in minor units (integer) + ISO 4217 currency.
-- Optional fields are NULL columns; the API layer omits nulls from JSON.

CREATE TABLE IF NOT EXISTS rides (
    id                   UUID PRIMARY KEY,
    status               TEXT NOT NULL,
    rider_id             UUID NOT NULL,
    driver_id            UUID,

    pickup_lat           DOUBLE PRECISION NOT NULL,
    pickup_lng           DOUBLE PRECISION NOT NULL,
    pickup_address       TEXT,
    dropoff_lat          DOUBLE PRECISION NOT NULL,
    dropoff_lng          DOUBLE PRECISION NOT NULL,
    dropoff_address      TEXT,

    fare_estimated_amount_minor BIGINT NOT NULL,
    fare_estimated_currency     CHAR(3) NOT NULL,
    fare_final_amount_minor     BIGINT,
    fare_final_currency         CHAR(3),

    driver_display_name  TEXT,
    vehicle_plate        TEXT,

    cancellation_actor_type TEXT,
    cancellation_actor_id   UUID,
    cancellation_reason     TEXT,
    cancellation_cancelled_at TIMESTAMPTZ,

    version              BIGINT NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL,
    accepted_at          TIMESTAMPTZ,
    started_at           TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,

    CONSTRAINT rides_status_check CHECK (status IN (
        'REQUESTED', 'ACCEPTED', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED'
    )),
    CONSTRAINT rides_version_positive CHECK (version >= 1),
    CONSTRAINT rides_fare_estimated_nonneg CHECK (fare_estimated_amount_minor >= 0),
    CONSTRAINT rides_fare_final_nonneg CHECK (
        fare_final_amount_minor IS NULL OR fare_final_amount_minor >= 0
    ),
    CONSTRAINT rides_final_fare_consistent CHECK (
        (fare_final_amount_minor IS NULL AND fare_final_currency IS NULL) OR
        (fare_final_amount_minor IS NOT NULL AND fare_final_currency IS NOT NULL)
    ),
    CONSTRAINT rides_pickup_lat_range CHECK (pickup_lat BETWEEN -90 AND 90),
    CONSTRAINT rides_pickup_lng_range CHECK (pickup_lng BETWEEN -180 AND 180),
    CONSTRAINT rides_dropoff_lat_range CHECK (dropoff_lat BETWEEN -90 AND 90),
    CONSTRAINT rides_dropoff_lng_range CHECK (dropoff_lng BETWEEN -180 AND 180)
);

CREATE INDEX IF NOT EXISTS idx_rides_rider_id ON rides (rider_id);
CREATE INDEX IF NOT EXISTS idx_rides_driver_id ON rides (driver_id);
CREATE INDEX IF NOT EXISTS idx_rides_status ON rides (status);

-- Command idempotency: scoped by endpoint + actor + key.
-- request_hash distinguishes improper reuse of the same key with a different body.
-- response_body stores the serialized response for faithful replay.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    endpoint         TEXT NOT NULL,
    actor_id         TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    request_hash     TEXT NOT NULL,
    response_status  INTEGER NOT NULL,
    response_body    JSONB NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (endpoint, actor_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);
