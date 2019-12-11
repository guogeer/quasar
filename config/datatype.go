package config

import (
	"encoding/json"
	"regexp"
	"strconv"
	"time"
)

type Scanner interface {
	Scan(s string) error
}

type jsonArg struct {
	value interface{}
}

func JSON(v interface{}) *jsonArg {
	return &jsonArg{value: v}
}

func (arg *jsonArg) Scan(s string) error {
	return json.Unmarshal([]byte(s), arg.value)
}

// 解析时间，格式："2019-10-25 01:02:03" "2019-10-25"
type timeArg time.Time

func ParseTime(s string) (time.Time, error) {
	loc, _ := time.LoadLocation("Local")
	match := "2006-01-02 15:04:05"
	if form := "2006-01-02"; len(form) == len(s) {
		match = form
	}
	return time.ParseInLocation(match, s, loc)
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

// Version 1.0.0 配置表的数组分隔符支持多个[;-~/\,]
func ParseStrings(s string) []string {
	return regexp.MustCompile(`[,;\-~/\\]`).Split(s, -1)
}

func ParseInts(s string) []int64 {
	chips := make([]int64, 0, 8)
	for _, v := range ParseStrings(s) {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			chips = append(chips, n)
		}
	}
	return chips
}
