package clock

import "time"

// Real returns the current UTC time.
type Real struct{}

// Now returns the current time in UTC.
func (Real) Now() time.Time { return time.Now().UTC() }
