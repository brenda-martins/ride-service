package ride

import "errors"

var (
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrInvariantViolation = errors.New("invariant violation")
	ErrIllegalTransition  = errors.New("illegal transition")
	ErrVersionConflict    = errors.New("version conflict")
	ErrDriverUnavailable  = errors.New("driver unavailable")
	ErrAlreadyAccepted    = errors.New("ride already accepted")
	ErrTerminal           = errors.New("ride is in a terminal state")
	ErrNotFound           = errors.New("ride not found")
)
