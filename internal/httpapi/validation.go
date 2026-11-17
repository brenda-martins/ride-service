package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// maxBodyBytes caps request bodies. Ride commands are small; 1 MiB is generous
// and protects the service from oversized payloads.
const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeProblem(w, r, http.StatusRequestEntityTooLarge, "Payload Too Large", "PAYLOAD_TOO_LARGE",
				fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes))
			return false
		}
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "invalid JSON body")
		return false
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "INVALID_REQUEST", "request body must contain a single JSON object")
		return false
	}
	return true
}

// validateUUID rejects malformed identifiers at the boundary, before they reach
// the database. It requires the canonical 36-character form.
func validateUUID(field, value string) error {
	if len(value) != 36 {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	return nil
}

// requiredID trims and validates a required actor identifier.
func requiredID(field, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if err := validateUUID(field, v); err != nil {
		return "", err
	}
	return v, nil
}
