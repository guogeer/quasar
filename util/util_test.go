package util

import (
	"strconv"
	"testing"
	"time"
)

func TestSkipPeriodTime(t *testing.T) {
	s := "2018-10-21 00:00:22"
	samples := [][3]string{
		{"2017-10-21 00:00:11", "1h", "2018-10-21 01:00:11"},
		{"2016-08-21 00:00:11", "30m", "2018-10-21 00:30:11"},
		{"2016-08-21 00:10:11", "10m", "2018-10-21 00:10:11"},
	}
	now, _ := ParseTime(s)
	for _, sample := range samples {
		t1, _ := ParseTime(sample[0])
		t2, _ := ParseTime(sample[2])
		d, _ := time.ParseDuration(sample[1])
		t3 := skipPeriodTime3(now, t1, d)
		if t2.Unix() != t3.Unix() {
			t.Error(sample, t2, t3)
		}
	}
}

func TestFormatMoney(t *testing.T) {
	samples := [][2]string{
		{"100", "100"},
		{"0", "0"},
		{"10000", "10,000"},
		{"-100000", "-100,000"},
		{"12345678", "12,345,678"},
	}
	for _, sample := range samples {
		n, _ := strconv.Atoi(sample[0])
		if FormatMoney(int64(n)) != sample[1] {
			t.Error(sample)
		}
	}
}
