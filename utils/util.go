package utils

import (
	"bytes"
	"encoding/json"
	"reflect"
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

func InArray(array any, some any) int {
	counter := 0
	someValues := reflect.ValueOf(some)
	arrayValues := reflect.ValueOf(array)
	for i := 0; i < arrayValues.Len(); i++ {
		switch someValues.Kind() {
		case reflect.Slice, reflect.Array:
			for k := 0; k < someValues.Len(); k++ {
				if reflect.DeepEqual(arrayValues.Index(i).Interface(), someValues.Index(k).Interface()) {
					counter++
				}
			}
		default:
			if reflect.DeepEqual(arrayValues.Index(i).Interface(), some) {
				counter++
			}
		}
	}
	return counter
}

// compare a,b json string
func EqualJSON(a, b any) bool {
	b1, _ := json.Marshal(a)
	b2, _ := json.Marshal(b)
	return bytes.Equal(b1, b2)
}
