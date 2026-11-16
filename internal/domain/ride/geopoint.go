package ride

import "fmt"

// GeoPoint represents a geographic coordinate. Address is optional (display only).
type GeoPoint struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address,omitempty"`
}

// Validate reports whether the coordinate is within valid bounds.
func (p GeoPoint) Validate() error {
	if p.Lat < -90 || p.Lat > 90 {
		return fmt.Errorf("%w: latitude out of range [-90,90]", ErrInvalidArgument)
	}
	if p.Lng < -180 || p.Lng > 180 {
		return fmt.Errorf("%w: longitude out of range [-180,180]", ErrInvalidArgument)
	}
	return nil
}
