package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"ride/internal/app"
	"ride/internal/domain/ride"
)

type geoPointRequest struct {
	Lat     *float64 `json:"lat"`
	Lng     *float64 `json:"lng"`
	Address string   `json:"address"`
}

func (g geoPointRequest) toDomain() (ride.GeoPoint, error) {
	if g.Lat == nil || g.Lng == nil {
		return ride.GeoPoint{}, errors.New("lat and lng are required")
	}
	return ride.GeoPoint{Lat: *g.Lat, Lng: *g.Lng, Address: g.Address}, nil
}

type createRideRequest struct {
	RiderID string          `json:"riderId"`
	Pickup  geoPointRequest `json:"pickup"`
	Dropoff geoPointRequest `json:"dropoff"`
}

type acceptRideRequest struct {
	DriverID        string `json:"driverId"`
	ExpectedVersion *int64 `json:"expectedVersion"`
}

func (a *API) createRide(w http.ResponseWriter, r *http.Request) {
	if a.createRideUC == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "SERVICE_UNAVAILABLE", "ride creation is not configured")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if err := validateIdempotencyKey(key); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", err.Error())
		return
	}

	var req createRideRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.RiderID) == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "riderId is required")
		return
	}
	pickup, err := req.Pickup.toDomain()
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "pickup: "+err.Error())
		return
	}
	dropoff, err := req.Dropoff.toDomain()
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "dropoff: "+err.Error())
		return
	}

	out, err := a.createRideUC.Execute(r.Context(), app.CreateRideInput{
		RiderID:        req.RiderID,
		Pickup:         pickup,
		Dropoff:        dropoff,
		ActorID:        req.RiderID,
		IdempotencyKey: key,
	})
	if err != nil {
		writeCreateRideError(w, r, err)
		return
	}

	// Use the serialized body from the use case for faithful replay/original response.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(out.ResponseCode)
	_, _ = w.Write(out.ResponseBody)
}

func validateIdempotencyKey(key string) error {
	if key == "" {
		return errors.New("Idempotency-Key header is required")
	}
	if len(key) < 16 || len(key) > 128 {
		return errors.New("Idempotency-Key must be between 16 and 128 characters")
	}
	return nil
}

func (a *API) getRide(w http.ResponseWriter, r *http.Request) {
	if a.getRideUC == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "SERVICE_UNAVAILABLE", "ride retrieval is not configured")
		return
	}
	rideID := r.PathValue("rideId")
	if rideID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "rideId is required")
		return
	}
	res, err := a.getRideUC.Execute(r.Context(), rideID)
	if err != nil {
		if errors.Is(err, ride.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "RIDE_NOT_FOUND", "ride not found")
			return
		}
		a.logger.Error("get ride failed", "err", err, "rideId", rideID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR", "unexpected error")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeCreateRideError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrIdempotencyKeyReused):
		writeProblem(w, r, http.StatusConflict, "Conflict", "IDEMPOTENCY_KEY_REUSED", "idempotency key was already used with a different request body")
	case errors.Is(err, ride.ErrInvalidArgument):
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", err.Error())
	case errors.Is(err, ride.ErrInvariantViolation):
		writeProblem(w, r, http.StatusUnprocessableEntity, "Unprocessable Entity", "DOMAIN_INVARIANT_VIOLATION", err.Error())
	default:
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR", "unexpected error")
	}
}
