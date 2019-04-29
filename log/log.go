// log
// 2019-04-28
// 默认输出到标准输出，设置路径后同时输出到文件

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
	_ = iota
	LTest
	LDebug
	LInfo
	LWarn
	LError
	LFatal
	LAll
)

const maxFileNumPerDay = 1024

var (
	enableDebug = true
	logTags     = []string{
		"", "TEST", "DEBUG", "INFO", "WARN", "ERROR", "FATAL",
	}
	fileLog = &FileLog{
		level:       LDebug,
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
	fileLog.Create("log/{proc_name}/run.log")
}

type FileLog struct {
	path        string
	level       int
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

func (l *FileLog) Output(level int, s string) {
	if level <= 0 || level > LAll {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if level < l.level {
		return
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

	s = fmt.Sprintf("[%s] %s", logTags[level], s)
	l.logger.Output(3, s)
	if enableDebug {
		log.Output(3, s)
	}
}

func (l *FileLog) Create(path string) {
	procName := filepath.Base(string(os.Args[0]))
	path = strings.Replace(path, "{proc_name}", procName, -1)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.path == path {
		return
	}
	l.path = path
}

func (l *FileLog) SetLevel(level int) {
	if level <= 0 || level >= LAll {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func getLevelByTag(tag string) int {
	for k, v := range logTags {
		if v == tag {
			return k
		}
	}
	panic("invalid log tag: " + tag)
}

func Create(path string) {
	fileLog.Create(path)
}

func SetLevel(lv int) {
	fileLog.SetLevel(lv)
}

func SetLevelByTag(tag string) {
	lv := getLevelByTag(tag)
	fileLog.SetLevel(lv)
}

func Testf(format string, v ...interface{}) {
	fileLog.Output(LTest, fmt.Sprintf(format, v...))
}

func Test(v ...interface{}) {
	fileLog.Output(LTest, fmt.Sprintln(v...))
}

func Debugf(format string, v ...interface{}) {
	fileLog.Output(LDebug, fmt.Sprintf(format, v...))
}

func Debug(v ...interface{}) {
	fileLog.Output(LDebug, fmt.Sprintln(v...))
}

func Infof(format string, v ...interface{}) {
	fileLog.Output(LInfo, fmt.Sprintf(format, v...))
}

func Info(v ...interface{}) {
	fileLog.Output(LInfo, fmt.Sprintln(v...))
}

func Warnf(format string, v ...interface{}) {
	fileLog.Output(LWarn, fmt.Sprintf(format, v...))
}

func Warn(v ...interface{}) {
	fileLog.Output(LWarn, fmt.Sprintln(v...))
}

func Errorf(format string, v ...interface{}) {
	fileLog.Output(LError, fmt.Sprintf(format, v...))
}

func Error(v ...interface{}) {
	fileLog.Output(LError, fmt.Sprintln(v...))
}

func Fatalf(format string, v ...interface{}) {
	fileLog.Output(LFatal, fmt.Sprintf(format, v...))
	os.Exit(0)
}

func Fatal(v ...interface{}) {
	fileLog.Output(LFatal, fmt.Sprintln(v...))
	os.Exit(0)
}

func Printf(tag, format string, v ...interface{}) {
	tag = strings.ToUpper(tag)
	lv := getLevelByTag(tag)
	fileLog.Output(lv, fmt.Sprintf(format, v...))
}
