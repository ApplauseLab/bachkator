package clock

import (
	"testing"
	"time"
)

func TestUTCUsesSystemClockWhenUnset(t *testing.T) {
	got := UTC(nil)
	if got.IsZero() {
		t.Fatal("UTC(nil) returned zero time")
	}
	if got.Location() != time.UTC {
		t.Fatalf("UTC(nil) location = %v, want UTC", got.Location())
	}
}

func TestUTCNormalizesInjectedClock(t *testing.T) {
	zone := time.FixedZone("offset", 2*60*60)
	input := time.Date(2026, 6, 14, 10, 0, 0, 0, zone)
	got := UTC(func() time.Time { return input })
	want := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("UTC(injected) = %v, want %v in UTC", got, want)
	}
}
