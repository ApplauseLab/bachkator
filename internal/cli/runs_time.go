package cli

import (
	"fmt"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
)

const timeFormat = "2006-01-02T15:04:05Z07:00"

func parseSince(value string) (time.Time, error) {
	return parseSinceWithNow(value, clock.SystemNow())
}

func parseSinceWithNow(value string, now time.Time) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return now.UTC().Add(-duration), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid --since %q: use a duration like 24h or an RFC3339 time",
			value,
		)
	}
	return parsed, nil
}
