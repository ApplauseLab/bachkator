package id

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewReturnsUUIDv7(t *testing.T) {
	value, err := New()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("version = %d, want 7", parsed.Version())
	}
}
