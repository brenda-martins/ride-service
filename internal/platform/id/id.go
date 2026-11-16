package id

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// UUIDv7 generates UUIDv7 identifiers (time-ordered) as required by project conventions.
type UUIDv7 struct{}

// NewID returns a new UUIDv7 string.
func (UUIDv7) NewID() (string, error) {
	return NewUUIDv7(time.Now())
}

// NewUUIDv7 generates a UUIDv7 using the provided timestamp.
func NewUUIDv7(t time.Time) (string, error) {
	var b [16]byte
	ms := uint64(t.UnixMilli())
	binary.BigEndian.PutUint32(b[0:4], uint32(ms>>16))
	binary.BigEndian.PutUint16(b[4:6], uint16(ms))
	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	// version 7
	b[6] = (b[6] & 0x0f) | 0x70
	// RFC 4122 variant
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
