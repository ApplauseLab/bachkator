package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/applauselab/bachkator/internal/bacherr"
)

func TestUsageErrorfImplementsError(t *testing.T) {
	err := UsageErrorf("bach %s", "example")
	if got := err.Error(); got != "usage: bach example" {
		t.Fatalf("UsageErrorf.Error() = %q, want %q", got, "usage: bach example")
	}
}

func TestIsUsageErrorDetectsUsageError(t *testing.T) {
	if !IsUsageError(UsageErrorf("bach example")) {
		t.Fatal("IsUsageError should return true for a usage error")
	}
}

func TestIsUsageErrorRejectsPlainError(t *testing.T) {
	if IsUsageError(fmt.Errorf("something failed")) {
		t.Fatal("IsUsageError should return false for a plain error")
	}
}

func TestIsUsageErrorDetectsWrappedUsageError(t *testing.T) {
	inner := UsageErrorf("bach example")
	wrapped := fmt.Errorf("wrapped: %w", inner)
	if !IsUsageError(wrapped) {
		t.Fatal("IsUsageError should return true for a wrapped usage error")
	}
}

func TestIsUsageErrorRejectsNil(t *testing.T) {
	if IsUsageError(nil) {
		t.Fatal("IsUsageError should return false for nil")
	}
}

func TestUsageErrorMatchesBacherrErrUsage(t *testing.T) {
	err := UsageErrorf("bach example")
	if !errors.Is(err, bacherr.ErrUsage) {
		t.Fatal("UsageErrorf should wrap bacherr.ErrUsage")
	}
}
