package log

import (
	"os"
	// "math/rand"
	"testing"
)

func TestUpdateLogPath(t *testing.T) {
	path := updateLogPath("log/{proc_name}/run.log")
	t.Log(updateLogPath(path))
}

func TestNoFileLog(t *testing.T) {
	// fileLog.Path = ""
	fileLog.MaxFileSize = 64
	for i := 0; i < 1; i++ {
		Debugf("%d", i)
	}
}

// 测试Stat耗时
func TestStat(t *testing.T) {
	var n int64
	for i := 0; i < 1000000; i++ {
		info, _ := os.Stat("log.go")
		n += info.Size()
	}
	t.Log(n)
}

func TestDebug(t *testing.T) {
	fileLog.MaxFileSize = 64
	for i := 0; i < 16; i++ {
		Debugf("%d", i)
	}
}
