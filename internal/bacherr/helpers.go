package bacherr

import (
	"errors"
	"fmt"
)

func UsageErrorf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUsage}, args...)...)
}

func NotFoundf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrNotFound}, args...)...)
}

func ValidationFailedf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidationFailed}, args...)...)
}

func Unsupportedf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUnsupported}, args...)...)
}

func MissingInputf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrMissingInput}, args...)...)
}

func IsUsageError(err error) bool {
	return errors.Is(err, ErrUsage)
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsValidationFailed(err error) bool {
	return errors.Is(err, ErrValidationFailed)
}

func IsUnsupported(err error) bool {
	return errors.Is(err, ErrUnsupported)
}

func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}
