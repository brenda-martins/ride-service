package ride

// Status represents the ride state in the domain state machine.
type Status string

const (
	StatusRequested  Status = "REQUESTED"
	StatusAccepted   Status = "ACCEPTED"
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
	StatusCancelled  Status = "CANCELLED"
)

// IsTerminal reports whether the status is final (immutable).
func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCancelled
}

// IsValid reports whether the value is a known status.
func (s Status) IsValid() bool {
	switch s {
	case StatusRequested, StatusAccepted, StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}
