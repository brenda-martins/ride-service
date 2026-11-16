package ride

import "fmt"

// Money represents a monetary value in minor units (e.g. cents).
// Never use floating point for money.
type Money struct {
	AmountMinor int64
	Currency    string
}

// Validate checks basic monetary invariants.
func (m Money) Validate() error {
	if m.AmountMinor < 0 {
		return fmt.Errorf("%w: amountMinor must not be negative", ErrInvalidArgument)
	}
	if len(m.Currency) != 3 {
		return fmt.Errorf("%w: currency must be ISO 4217 (3 letters)", ErrInvalidArgument)
	}
	for i := 0; i < len(m.Currency); i++ {
		c := m.Currency[i]
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("%w: currency must be uppercase ISO 4217", ErrInvalidArgument)
		}
	}
	return nil
}
