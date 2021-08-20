// 日志
// 1、保存最近15天的日志
// 2、日志按照日期命名，如run.log.06-10，run.log.06-11。单个日志文件最大限制500M，超过后命名为新文件run.log.06-11.1，新的日志写入run.log.06-11
// 3、日志同时输出到文件和标准输出

package log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

const (
	maxFileNumPerDay = 1024
)

var (
	logLevels = []string{
		LvTest,
		LvDebug,
		LvInfo,
		LvWarn,
		LvError,
		LvError,
	}
	fileLog = &FileLog{
		level:       LvDebug,
		maxSaveDays: 15,
		maxFileSize: 512 * 1024 * 1024, // 512M
	}
)

type FileLog struct {
	mu         sync.Mutex
	path       string // 日志路径
	newPath    string
	f          *os.File // 日志输出的文件
	createTime time.Time
	size       int64 // 当前打印的文件大小

	level         string // 打印的日志最低等级
	maxSaveDays   int    // 文件最大保存天数
	maxFileSize   int64  // 文件最大限制
	disableStdout bool   // 屏蔽标准输出
}

// 将oldPath移动至newPath并创建新oldPath
func (l *FileLog) moveFileLocked(oldPath, newPath string) {
	if l.newPath == newPath {
		return
	}
	if l.f != nil {
		l.f.Close()
	}
	os.Rename(oldPath, newPath)

	dir := filepath.Dir(newPath)
	mode := os.O_WRONLY | os.O_CREATE | os.O_APPEND

	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(oldPath, mode, 0664)
	if err != nil {
		panic("create log file error")
	}
	stat, _ := f.Stat()
	l.f = f
	l.size = stat.Size()
	l.newPath = newPath
	l.createTime = time.Now()
}

// 清理过期文件
func (l *FileLog) cleanFilesLocked(path string) {
	if l.maxSaveDays == 0 || path == "" {
		return
	}
	for try := 0; try < maxFileNumPerDay; try++ {
		morePath := fmt.Sprintf("%s.%d", path, try) // run.log.2021-06-04.1
		if try == 0 {
			morePath = path
		}
		if _, err := os.Stat(morePath); os.IsNotExist(err) {
			break
		}
		os.Remove(morePath)
	}
}

func (l *FileLog) Output(level, s string) {
	// 比较等级
	l.mu.Lock()
	for i := range logLevels {
		if logLevels[i] == l.level {
			break
		}
		if logLevels[i] == level {
			l.mu.Unlock()
			return
		}
	}

	_, codePath, codeLine, ok := runtime.Caller(2)
	if !ok {
		codePath = "???"
	}
	codeFile := filepath.Base(codePath)

	now := time.Now()
	datePath := fmt.Sprintf("%s.%02d-%02d", l.path, now.Month(), now.Day())

	newPath := l.path
	if l.path != "" && l.createTime.YearDay() != now.YearDay() {
		newPath = datePath
		l.createTime = now

		t := now.Add(-time.Duration(l.maxSaveDays+1) * 24 * time.Hour)
		path2 := fmt.Sprintf("%s.%02d-%02d", l.path, t.Month(), t.Day())
		l.cleanFilesLocked(path2)
	}

	// 2021/06/03 23:01:39 message.go:165: [DEBUG] log message
	outStr := fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d %s:%d: [%s] %s\n",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(),
		codeFile, codeLine, level, s,
	)
	if l.size+int64(len(outStr)) > l.maxFileSize {
		for try := 1; try < maxFileNumPerDay; try++ {
			newPath = fmt.Sprintf("%s.%d", datePath, try)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				break
			}
		}
	}
	if l.path != "" && newPath != l.path {
		l.moveFileLocked(datePath, newPath)
	}
	if l.f != nil {
		l.size += int64(len(outStr))
		l.f.WriteString(outStr)
	}
	if !fileLog.disableStdout {
		os.Stdout.WriteString(outStr)
	}
	l.mu.Unlock()
}

func (l *FileLog) Create(path string) {
	if path == "" {
		return
	}
	procName := filepath.Base(string(os.Args[0]))
	// windows系统移除".exe"后缀
	if ext := filepath.Ext(procName); ext == ".exe" {
		procName = procName[:len(procName)-len(ext)]
	}
	path = strings.Replace(path, "{proc_name}", procName, -1)

	l.mu.Lock()
	defer l.mu.Unlock()
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

// fmt.Sprint：string类型参数前后不会插入空格
// fmt.Sprintln：参数之间都会插入空格，需移除串尾换行
func sprintf(v ...interface{}) string {
	s := fmt.Sprintln(v...)
	return s[:len(s)-1]
}

func Testf(format string, v ...interface{}) {
	fileLog.Output(LvTest, fmt.Sprintf(format, v...))
}

func Test(v ...interface{}) {
	fileLog.Output(LvTest, sprintf(v...))
}

func Debugf(format string, v ...interface{}) {
	fileLog.Output(LvDebug, fmt.Sprintf(format, v...))
}

func Debug(v ...interface{}) {
	fileLog.Output(LvDebug, sprintf(v...))
}

func Infof(format string, v ...interface{}) {
	fileLog.Output(LvInfo, fmt.Sprintf(format, v...))
}

func Info(v ...interface{}) {
	fileLog.Output(LvInfo, sprintf(v...))
}

func Warnf(format string, v ...interface{}) {
	fileLog.Output(LvWarn, fmt.Sprintf(format, v...))
}

func Warn(v ...interface{}) {
	fileLog.Output(LvWarn, sprintf(v...))
}

func Errorf(format string, v ...interface{}) {
	fileLog.Output(LvError, fmt.Sprintf(format, v...))
}

func Error(v ...interface{}) {
	fileLog.Output(LvError, sprintf(v...))
}

func Fatalf(format string, v ...interface{}) {
	fileLog.Output(LvFatal, fmt.Sprintf(format, v...))
	os.Exit(0)
}

func Fatal(v ...interface{}) {
	fileLog.Output(LvFatal, sprintf(v...))
	os.Exit(0)
}

func Printf(level, format string, v ...interface{}) {
	level = strings.ToUpper(level)
	fileLog.Output(level, fmt.Sprintf(format, v...))
}
