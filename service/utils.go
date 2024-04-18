package service

import "time"

func IsUnixZero(t time.Time) bool {
	return t.Equal(time.Unix(0, 0))
}

func IsToday(t time.Time, timezone *time.Location) bool {
	return t.In(timezone).Format("2006-01-02") == time.Now().In(timezone).Format("2006-01-02")
}
