package clock

import "time"

type NowFunc func() time.Time

func SystemNow() time.Time {
	return time.Now().UTC()
}

func UTC(now NowFunc) time.Time {
	if now == nil {
		return SystemNow()
	}
	return now().UTC()
}
