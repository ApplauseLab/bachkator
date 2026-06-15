package cli

import (
	"testing"
	"time"
)

func TestParseSinceWithNowUsesInjectedClockForDurations(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	got, err := parseSinceWithNow("2h", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseSinceWithNow() = %s, want %s", got, want)
	}
}
