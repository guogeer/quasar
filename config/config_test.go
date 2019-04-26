package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Log("load config.xml", defaultConfig)
}

func TestParseArgs(t *testing.T) {
	samples := [][]string{
		{"1", "-test=1", "-test2=2"},
		{"", "-test2=1", "-test3=2"},
		{"1", "-test", "1", "-test3=2"},
		{"1", "-test2=1", "-test", "1", "-test3=2"},
		{"abcde1", "-test2", "1", "-test", "abcde1", "-test3=2"},
	}
	for _, sample := range samples {
		v := ParseArgs(sample[1:], "test", "")
		if v != sample[0] {
			t.Errorf("parse %v result: %s", sample, v)
		}
	}
}
