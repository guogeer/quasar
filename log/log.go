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

const MaxFileNumPerDay = 1024

var (
	enableDebug = true
	logTags     = []string{
		"", "TEST", "DEBUG", "INFO", "WARN", "ERROR", "FATAL",
	}
	MyFileLog = FileLog{
		Path:        "log/{proc_name}/run.log",
		Level:       LDebug,
		MaxSaveDays: 10,
		MaxFileSize: 512 * 1024 * 1024, // 512M
	}
	fileLog = &MyFileLog
)

func updateLogPath(path string) string {
	procPath := string(os.Args[0])
	n := strings.LastIndexByte(procPath, os.PathSeparator)
	procName := procPath[n+1:]
	return strings.Replace(path, "{proc_name}", procName, -1)
}

type FileLog struct {
	Path        string
	Level       int
	MaxSaveDays int
	MaxFileSize int64

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

	if n := strings.LastIndexByte(newPath, os.PathSeparator); n >= 0 {
		dir := newPath[:n]
		os.MkdirAll(dir, 0755)
	}
	f, err := os.OpenFile(oldPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		panic("create log file error")
	}
	l.f = f
	l.size = 0
	l.lines = 0
	l.newPath = newPath
	if l.logger == nil {
		l.logger = log.New(f, "", log.Lshortfile|log.LstdFlags)
		if enableDebug {
			log.SetOutput(os.Stdout)
			log.SetFlags(log.Lshortfile | log.LstdFlags)
		}
	}
	l.logger.SetOutput(f)
}

// 清理过期文件
func (l *FileLog) cleanFiles(path string) {
	if l.MaxSaveDays == 0 {
		return
	}
	for try := 0; try < MaxFileNumPerDay; try++ {
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
	if level < l.Level || l.Path == "" {
		return
	}

	now := time.Now()
	path := updateLogPath(l.Path)
	newPath := path
	datePath := fmt.Sprintf("%s.%02d-%02d", path, now.Month(), now.Day())

	datePath = updateLogPath(datePath)
	if l.t.YearDay() != now.YearDay() {
		newPath = datePath
		l.t = now

		t := now.Add(-time.Duration(l.MaxSaveDays+1) * 24 * time.Hour)
		path2 := fmt.Sprintf("%s.%02d-%02d", path, t.Month(), t.Day())
		l.cleanFiles(path2)
	}

	if l.lines&(l.lines-1) == 0 {
		if info, err := os.Stat(datePath); err == nil {
			l.size = info.Size()
		}
	}
	l.lines += 1
	if l.size > l.MaxFileSize {
		for try := 1; try < MaxFileNumPerDay; try++ {
			newPath = fmt.Sprintf("%s.%d", datePath, try)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if newPath != path {
		l.moveFile(datePath, newPath)
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
