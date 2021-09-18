package util

import (
	"bytes"
	"encoding/json"

	// "fmt"

	// "strconv"
	// "strings"
	"time"
)

func skipPeriodTime3(now, start time.Time, d time.Duration) time.Time {
	end := start
	if start.IsZero() {
		panic("start time is zero")
	}
	if diff := now.Sub(start); diff > 0 && d > 0 {
		end = start.Add(time.Duration((diff + d - 1) / d * d))
	}
	return end
}

func SkipPeriodTime(start time.Time, d time.Duration) time.Time {
	return skipPeriodTime3(time.Now(), start, d)
}

// compare a,b json string
func EqualJSON(a, b interface{}) bool {
	b1, _ := json.Marshal(a)
	b2, _ := json.Marshal(b)
	return bytes.Equal(b1, b2)
}
