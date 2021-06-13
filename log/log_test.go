package log

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	// "math/rand"
	"testing"
)

func TestNoFileLog(t *testing.T) {
	// fileLog.Path = ""
	fileLog.maxFileSize = 64
	for i := 0; i < 1; i++ {
		Debugf("%d", i)
	}
}

// 测试Stat耗时
func BenchmarkStat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Stat("log.go")
	}
}

func TestDebug(t *testing.T) {
	fileLog.maxFileSize = 99
	fileLog.disableStdout = true
	defer func() { fileLog.disableStdout = false }()

	os.RemoveAll("log")
	for i := 0; i < 16; i++ {
		if i == 8 {
			fileLog.Create("log/{proc_name}/run.log")
		}
		Debugf("%d", i)
		Debug(i)
	}

	re := regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} log_test.go:\d+: \[DEBUG\] \d+\n\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} log_test.go:\d+: \[DEBUG\] \d+\n`)

	err := filepath.Walk("log/log.test", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		buf, _ := os.ReadFile(path)
		if info.Size() > fileLog.maxFileSize {
			return errors.New("log file size more than limit")
		}

		if !re.MatchString(string(buf)) {
			return errors.New("log file content not match")
		}
		return nil
	})
	fileLog.f.Close()
	os.RemoveAll("log")

	if err != nil {
		t.Errorf("walk log files error: %v\n", err)
	}
}
