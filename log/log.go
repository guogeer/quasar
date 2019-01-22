// log

package log

import (
	"fmt"
	"log"
	"os"
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

var (
	enableDebug     = true
	defaultLogPath  = getDefaultLogPath()
	defaultLogLevel = LDebug
	fileLog         = NewFileLog(defaultLogPath, defaultLogLevel)
	logTags         = []string{
		"", "TEST", "DEBUG", "INFO", "WARN", "ERROR", "FATAL",
	}
)

// default path
func getDefaultLogPath() string {
	procPath := string(os.Args[0])
	n := strings.LastIndexByte(procPath, os.PathSeparator)
	procName := procPath[n+1:]
	return fmt.Sprintf("log%c%s%crun.log", os.PathSeparator, procName, os.PathSeparator)
}

type FileLog struct {
	path   string
	cycle  int
	Level  int
	t      time.Time
	f      *os.File
	logger *log.Logger
	mu     sync.RWMutex
}

func NewFileLog(path string, level int) *FileLog {
	l := &FileLog{
		path:  path,
		Level: level,
		cycle: 10,
	}
	l.t, _ = time.Parse("2006-01-01", "1900-01-01")
	f, _ := os.Open(os.DevNull)
	l.logger = log.New(f, "", log.Lshortfile|log.LstdFlags)
	if enableDebug {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Lshortfile | log.LstdFlags)
	}
	return l
}

func (l *FileLog) NewFile(newPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	// prevent enter again on one day
	if l.t.YearDay() == now.YearDay() {
		return nil
	}
	if l.f != nil {
		l.f.Close()
		os.Rename(l.path, newPath)
	}
	n := strings.LastIndexByte(l.path, os.PathSeparator)
	if n >= 0 {
		dir := l.path[:n]
		os.MkdirAll(dir, 0755)
	}
	deadline := now.Add(-time.Duration(l.cycle) * time.Hour * 24)
	deadlinePath := fmt.Sprintf("%s.%02d-%02d", l.path, deadline.Month(), deadline.Day())
	// fmt.Println(deadlinePath)
	if _, err := os.Lstat(deadlinePath); err == nil {
		os.Remove(deadlinePath)
	}
	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		panic("create log file error")
	}
	l.f = f
	l.t = now
	l.logger.SetOutput(f)
	return nil
}

func (l *FileLog) Output(level int, s string) {
	if level <= 0 || level > LAll {
		return
	}

	l.mu.RLock()
	t, minLevel, path := l.t, l.Level, l.path
	l.mu.RUnlock()
	if level < minLevel {
		return
	}
	now := time.Now()
	if t.YearDay() != now.YearDay() {
		newPath := fmt.Sprintf("%s.%02d-%02d", path, t.Month(), t.Day())
		l.NewFile(newPath)
	}
	if l.logger == nil {
		panic("Log output path unset")
	}
	s = fmt.Sprintf("[%s] %s", logTags[level], s)
	l.logger.Output(3, s)
	if enableDebug {
		log.Output(3, s)
	}
}

func (l *FileLog) SetLevel(level int) {
	if level <= 0 || level >= LAll {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Level = level
}

func getLevelByTag(tag string) int {
	for k, v := range logTags {
		if v == tag {
			return k
		}
	}
	panic("invalid log tag: " + tag)
}

func SetLevel(lv int) {
	fileLog.SetLevel(lv)
}

func SetLevelByTag(tag string) {
	lv := getLevelByTag(tag)
	fileLog.SetLevel(lv)
}

func SetOutput(path string) {
	if fileLog.path == path {
		return
	}
	fileLog.NewFile(path)
	fileLog.path = path
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
