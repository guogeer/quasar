package util

import (
	"bytes"
	"encoding/json"

	// "fmt"
	"reflect"
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

// Monday,Thursday...
func GetFirstWeekday(t time.Time) time.Time {
	weekDays := ((int)(t.Weekday()) + 6) % 7
	firstDay := t.Add(-time.Duration(weekDays) * 24 * time.Hour)
	y, h, d := firstDay.Date()
	firstDay = time.Date(y, h, d, 0, 0, 0, 0, t.Location())
	return firstDay
}

func InArray(array interface{}, some interface{}) int {
	counter := 0
	someValues := reflect.ValueOf(some)
	arrayValues := reflect.ValueOf(array)
	for i := 0; i < arrayValues.Len(); i++ {
		if someValues.Kind() == reflect.Slice {
			for k := 0; k < someValues.Len(); k++ {
				if reflect.DeepEqual(arrayValues.Index(i).Interface(), someValues.Index(k).Interface()) {
					counter++
				}
			}
		}
		if reflect.DeepEqual(arrayValues.Index(i).Interface(), some) {
			counter++
		}
	}
	return counter
}

// compare a,b json string
// TODO  ignore struct or map field order
func EqualJSON(a, b interface{}) bool {
	b1, _ := json.Marshal(a)
	b2, _ := json.Marshal(b)
	return bytes.Equal(b1, b2)
}
