package bacherr

import (
	"errors"
	"testing"
)

func TestSentinelsAreDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrAlreadyExists) {
		t.Fatal("ErrNotFound should not match ErrAlreadyExists")
	}
	if errors.Is(ErrValidationFailed, ErrUnsupported) {
		t.Fatal("ErrValidationFailed should not match ErrUnsupported")
	}
}

func TestUsageErrorf(t *testing.T) {
	err := UsageErrorf("bach %s", "example")
	if !errors.Is(err, ErrUsage) {
		t.Fatal("UsageErrorf should wrap ErrUsage")
	}
	if got := err.Error(); got != "usage: bach example" {
		t.Fatalf("UsageErrorf.Error() = %q, want %q", got, "usage: bach example")
	}
}

func TestNotFoundf(t *testing.T) {
	err := NotFoundf("run %q", "abc")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("NotFoundf should wrap ErrNotFound")
	}
	if got := err.Error(); got != "not found: run \"abc\"" {
		t.Fatalf("NotFoundf.Error() = %q, want %q", got, "not found: run \"abc\"")
	}
}

func TestValidationFailedf(t *testing.T) {
	err := ValidationFailedf("missing title")
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatal("ValidationFailedf should wrap ErrValidationFailed")
	}
}

func TestUnsupportedf(t *testing.T) {
	err := Unsupportedf("provider %q", "x")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatal("Unsupportedf should wrap ErrUnsupported")
	}
}

func TestIsHelpers(t *testing.T) {
	if !IsUsageError(UsageErrorf("x")) {
		t.Fatal("IsUsageError should return true")
	}
	if !IsNotFound(NotFoundf("x")) {
		t.Fatal("IsNotFound should return true")
	}
	if !IsValidationFailed(ValidationFailedf("x")) {
		t.Fatal("IsValidationFailed should return true")
	}
	if !IsUnsupported(Unsupportedf("x")) {
		t.Fatal("IsUnsupported should return true")
	}
	if !IsCancelled(ErrCancelled) {
		t.Fatal("IsCancelled should return true for ErrCancelled")
	}
}
