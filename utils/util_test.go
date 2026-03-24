package utils_test

import (
	"testing"
	"time"

	"github.com/guogeer/quasar/v2/utils"
)

func TestSkipPeriodTime(t *testing.T) {
	s := "2018-10-21 00:00:22"
	samples := [][3]string{
		{"2017-10-21 00:00:11", "1h", "2018-10-21 01:00:11"},
		{"2016-08-21 00:00:11", "30m", "2018-10-21 00:30:11"},
		{"2016-08-21 00:10:11", "10m", "2018-10-21 00:10:11"},
	}
	longDate := "2006-01-02 15:04:05"
	now, _ := time.Parse(longDate, s)
	for _, sample := range samples {
		t1, _ := time.Parse(longDate, sample[0])
		t2, _ := time.Parse(longDate, sample[2])
		d, _ := time.ParseDuration(sample[1])
		t3 := utils.SkipPeriodTime3(now, t1, d)
		if t2.Unix() != t3.Unix() {
			t.Error(sample, t2, t3)
		}
	}
}

func TestEqualJSON(t *testing.T) {
	a1 := map[string]any{
		"A": 1,
		"B": 2,
		"S": "s",
	}
	b1 := map[string]any{
		"A": 1,
		"B": 2,
		"S": "s",
	}
	if !utils.EqualJSON(a1, b1) {
		t.Error("deep equal result expect true")
	}
}
