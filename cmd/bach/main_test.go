package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/applauselab/bachkator/internal/cli"
)

func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version should not be empty")
	}
}

func TestExitCodeForUsageError(t *testing.T) {
	err := cli.UsageErrorf("usage: bach example")
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exitCodeFor(usage error) = %d, want 2", got)
	}
}

func TestExitCodeForWrappedUsageError(t *testing.T) {
	inner := cli.UsageErrorf("usage: bach example")
	wrapped := fmt.Errorf("wrapped: %w", inner)
	if got := exitCodeFor(wrapped); got != 2 {
		t.Fatalf("exitCodeFor(wrapped usage error) = %d, want 2", got)
	}
}

func TestExitCodeForGeneralError(t *testing.T) {
	if got := exitCodeFor(errors.New("something failed")); got != 1 {
		t.Fatalf("exitCodeFor(general error) = %d, want 1", got)
	}
}

func TestExitCodeForNil(t *testing.T) {
	if got := exitCodeFor(nil); got != 1 {
		t.Fatalf("exitCodeFor(nil) = %d, want 1", got)
	}
}
