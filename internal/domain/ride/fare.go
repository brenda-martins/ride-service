package ride

import "fmt"

// Fare holds the estimated fare and, when completed, the final fare.
// Currency is fixed at creation (from the estimated fare).
type Fare struct {
	Estimated Money  `json:"estimated"`
	Final     *Money `json:"final,omitempty"`
}

// Validate checks fare consistency.
func (f Fare) Validate() error {
	if err := f.Estimated.Validate(); err != nil {
		return fmt.Errorf("estimated fare: %w", err)
	}
	if f.Final != nil {
		if err := f.Final.Validate(); err != nil {
			return fmt.Errorf("final fare: %w", err)
		}
		if f.Final.Currency != f.Estimated.Currency {
			return fmt.Errorf("%w: final fare currency differs from estimated currency", ErrInvalidArgument)
		}
		if f.Final.AmountMinor < f.Estimated.AmountMinor {
			// INV-4: fare may only grow up to the final value (final >= estimated when present).
			return fmt.Errorf("%w: final fare must not be less than estimated fare", ErrInvariantViolation)
		}
	}
	return nil
}
