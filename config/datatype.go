package config

import (
	"encoding/json"
	"regexp"
	"time"
)

type Scanner interface {
	Scan(s string) error
}

type jsonArg struct {
	value any
}

func JSON(v any) *jsonArg {
	return &jsonArg{value: v}
}

func (arg *jsonArg) Scan(s string) error {
	return json.Unmarshal([]byte(s), arg.value)
}

// 解析时间，格式："2019-10-25 01:02:03" "2019-10-25"
type timeArg time.Time

func ParseTime(s string) (time.Time, error) {
	match := "2006-01-02 15:04:05"
	if form := "2006-01-02"; len(form) == len(s) {
		match = form
	}
	return time.ParseInLocation(match, s, time.Local)
}

func (arg *timeArg) Scan(s string) error {
	t, err := ParseTime(s)
	*arg = timeArg(t)
	return err
}

func parseDuration(s string) (time.Duration, error) {
	if b, _ := regexp.MatchString(`[0-9]$`, s); b {
		s = s + "s"
	}
	return time.ParseDuration(s)
}

// 格式同https://golang.google.cn/pkg/time/#ParseDuration
type durationArg time.Duration

func (arg *durationArg) Scan(s string) error {
	d, err := parseDuration(s)
	*arg = durationArg(d)
	return err
}
