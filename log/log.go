// log
// 2019-04-28
// 默认输出到标准输出，设置路径后同时输出到文件
// 2019-07-22
// 默认输出到标准输出，配置日志文件后才输出文件
// 2019-12-05
// 移除日志Tag。上个版本日志等级是整形序列1，2，3，……，现在改动为"INFO"，"TEST"

package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LvTest  = "TEST"
	LvDebug = "DEBUG"
	LvInfo  = "INFO"
	LvWarn  = "WARN"
	LvError = "ERROR"
	LvFatal = "FATAL"
)

const maxFileNumPerDay = 1024

var (
	enableDebug = true
	logLevels   = []string{
		LvTest,
		LvDebug,
		LvInfo,
		LvWarn,
		LvError,
		LvError,
	}
	fileLog = &FileLog{
		level:       LvDebug,
		maxSaveDays: 10,
		maxFileSize: 512 * 1024 * 1024, // 512M
	}
)

func init() {
	f, _ := os.Open(os.DevNull)
	fileLog.logger = log.New(f, "", log.Lshortfile|log.LstdFlags)
	if enableDebug {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Lshortfile | log.LstdFlags)
	}
	// fileLog.Create("log/{proc_name}/run.log")
}

type FileLog struct {
	path        string
	level       string
	maxSaveDays int
	maxFileSize int64

	t       time.Time
	f       *os.File
	logger  *log.Logger
	mu      sync.Mutex
	newPath string

	lines int
	size  int64
}

// 将oldPath移动至newPath并创建新oldPath
func (l *FileLog) moveFile(oldPath, newPath string) {
	if l.newPath == newPath {
		return
	}
	if l.f != nil {
		l.f.Close()
	}
	l.f = nil
	os.Rename(oldPath, newPath)

	dir := filepath.Dir(newPath)
	mode := os.O_WRONLY | os.O_CREATE | os.O_APPEND

	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(oldPath, mode, 0664)
	if err != nil {
		panic("create log file error")
	}
	l.f = f
	l.size = 0
	l.lines = 0
	l.newPath = newPath
	l.logger.SetOutput(f)
}

// 清理过期文件
func (l *FileLog) cleanFiles(path string) {
	if l.maxSaveDays == 0 || path == "" {
		return
	}
	for try := 0; try < maxFileNumPerDay; try++ {
		path2 := fmt.Sprintf("%s.%d", path, try)
		if try == 0 {
			path2 = path
		}
		if _, err := os.Stat(path2); os.IsNotExist(err) {
			break
		}
		os.Remove(path2)
	}
}

func (l *FileLog) Output(level, s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, level2 := range logLevels {
		if level2 == level {
			return
		}
		if level2 == l.level {
			break
		}
	}

	now := time.Now()
	datePath := fmt.Sprintf("%s.%02d-%02d", l.path, now.Month(), now.Day())

	newPath := l.path
	if l.path != "" && l.t.YearDay() != now.YearDay() {
		newPath = datePath
		l.t = now

		t := now.Add(-time.Duration(l.maxSaveDays+1) * 24 * time.Hour)
		path2 := fmt.Sprintf("%s.%02d-%02d", l.path, t.Month(), t.Day())
		l.cleanFiles(path2)
	}

	if l.lines&(l.lines-1) == 0 {
		if info, err := os.Stat(datePath); err == nil {
			l.size = info.Size()
		}
	}
	l.lines += 1
	if l.size > l.maxFileSize {
		for try := 1; try < maxFileNumPerDay; try++ {
			newPath = fmt.Sprintf("%s.%d", datePath, try)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if l.path != "" && newPath != l.path {
		l.moveFile(datePath, newPath)
	}

	s = fmt.Sprintf("[%s] %s", level, s)
	l.logger.Output(3, s)
	if enableDebug {
		log.Output(3, s)
	}
}

func (l *FileLog) Create(path string) {
	if path == "" {
		return
	}
	procName := filepath.Base(string(os.Args[0]))
	path = strings.Replace(path, "{proc_name}", procName, -1)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.path == path {
		return
	}
	l.path = path
}

func (l *FileLog) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func Create(path string) {
	fileLog.Create(path)
}

func SetLevel(lv string) {
	fileLog.SetLevel(lv)
}

func Testf(format string, v ...interface{}) {
	fileLog.Output(LvTest, fmt.Sprintf(format, v...))
}

func Test(v ...interface{}) {
	fileLog.Output(LvTest, fmt.Sprintln(v...))
}

func Debugf(format string, v ...interface{}) {
	fileLog.Output(LvDebug, fmt.Sprintf(format, v...))
}

func Debug(v ...interface{}) {
	fileLog.Output(LvDebug, fmt.Sprintln(v...))
}

func Infof(format string, v ...interface{}) {
	fileLog.Output(LvInfo, fmt.Sprintf(format, v...))
}

func Info(v ...interface{}) {
	fileLog.Output(LvInfo, fmt.Sprintln(v...))
}

func Warnf(format string, v ...interface{}) {
	fileLog.Output(LvWarn, fmt.Sprintf(format, v...))
}

func Warn(v ...interface{}) {
	fileLog.Output(LvWarn, fmt.Sprintln(v...))
}

func Errorf(format string, v ...interface{}) {
	fileLog.Output(LvError, fmt.Sprintf(format, v...))
}

func Error(v ...interface{}) {
	fileLog.Output(LvError, fmt.Sprintln(v...))
}

func Fatalf(format string, v ...interface{}) {
	fileLog.Output(LvFatal, fmt.Sprintf(format, v...))
	os.Exit(0)
}

func Fatal(v ...interface{}) {
	fileLog.Output(LvFatal, fmt.Sprintln(v...))
	os.Exit(0)
}

func Printf(level, format string, v ...interface{}) {
	level = strings.ToUpper(level)
	fileLog.Output(level, fmt.Sprintf(format, v...))
}
