package util

import (
	"testing"
	"time"
)

func TestSkipPeriodTime(t *testing.T) {
	s1, _ := ParseTime("2017-10-21 00:00:11")
	t1 := SkipPeriodTime(s1, time.Duration(25*time.Minute))
	s2, _ := ParseTime("2016-08-21 00:00:11")
	t2 := SkipPeriodTime(s2, time.Duration(60*time.Minute))
	t.Log(t1, t2)
}

func TestCombine(t *testing.T) {
	t.Log(Combine([]int{1, 3, 5}, 2))
}

func TestFormatMoney(t *testing.T) {
	t.Log(FormatMoney(100))
	t.Log(FormatMoney(0))
	t.Log(FormatMoney(10000))
	t.Log(FormatMoney(-100000))
	t.Log(FormatMoney(12345678))
}
