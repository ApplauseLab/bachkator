package id

import (
	"fmt"

	"github.com/google/uuid"
)

// New returns a Bach public identifier. All Bach-generated public IDs use UUIDv7.
func New() (string, error) {
	value, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuidv7: %w", err)
	}
	return value.String(), nil
}

// MustNew returns a Bach public identifier or panics. Use only where callers cannot
// recover cleanly from system randomness failure.
func MustNew() string {
	value, err := New()
	if err != nil {
		panic(err)
	}
	return value
}
