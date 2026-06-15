package runner

import (
	"context"
	"fmt"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
)

func shouldRetryAttempt(err error, retry model.RetryPolicy) bool {
	if quality.IsParseError(err) {
		return false
	}
	if quality.IsGateError(err) {
		return retry.RetryOnQualityGateFailure
	}
	return true
}

func targetRuntimeContext(
	ctx context.Context,
	target *Target,
) (context.Context, context.CancelFunc) {
	timeout := target.Spec().Runtime.Timeout
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func targetRuntimeError(ctx context.Context, target *Target) error {
	if ctx.Err() == context.DeadlineExceeded {
		if target.Spec().Runtime.Timeout <= 0 {
			return ctx.Err()
		}
		return fmt.Errorf(
			"target %q timed out after %s",
			target.Name,
			target.Spec().Runtime.Timeout,
		)
	}
	return ctx.Err()
}
