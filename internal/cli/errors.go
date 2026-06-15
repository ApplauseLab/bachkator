package cli

import (
	"github.com/applauselab/bachkator/internal/bacherr"
)

func UsageErrorf(format string, args ...any) error {
	return bacherr.UsageErrorf(format, args...)
}

func IsUsageError(err error) bool {
	return bacherr.IsUsageError(err)
}
