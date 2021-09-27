package config

// 进程启动时预加载的配置
// 默认获取进程工作路径下config.xml

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"github.com/go-yaml/yaml"
	"github.com/guogeer/quasar/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var gLogTag = flag.String("log", "DEBUG", "log DEBUG|INFO|ERROR")
var gLogPath = flag.String("logpath", "log/{proc_name}/run.log", "log path")
var gConfigPath = flag.String("config", "", "config xml|json|yaml")

func LoadFile(path string, conf interface{}) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	switch filepath.Ext(path) {
	default:
		return errors.New("only support xml|json|yaml")
	case ".xml":
		err = xml.Unmarshal(b, &conf)
	case ".json":
		err = json.Unmarshal(b, &conf)
	case ".yaml":
		err = yaml.Unmarshal(b, &conf)
	}
	return err
}

type server struct {
	Name string
	Addr string `xml:"Address"`
}

type Env struct {
	path string

	ProductKey  string
	ServerList  []server `xml:"ServerList>Server"`
	LogPath     string   `xml:"Log>Path"`
	LogTag      string   `xml:"Log>Tag"`
	EnableDebug bool     // 开启调试，将输出消息统计日志等
}

func (env *Env) Path() string {
	return env.path
}

func (env *Env) Server(name string) server {
	for _, srv := range env.ServerList {
		if srv.Name == name {
			return srv
		}
	}
	return server{}
}

var defaultConfig Env

func Config() *Env {
	return &defaultConfig
}

func init() {
	// 命令行参数优先
	logPath, logTag, path := *gLogPath, *gLogTag, *gConfigPath
	defaultConfig.path = parseCommandLine(os.Args[1:], "config", path)
	// 未指定配置路径，默认加载当前目录下config.xml
	if defaultConfig.path == "" {
		path := "config.xml"
		if _, err := os.Stat(path); err == nil {
			defaultConfig.path = path
		}
	}
	// 未要求加载配置
	if defaultConfig.path == "" {
		return
	}

	err := LoadFile(defaultConfig.path, &defaultConfig)
	if err != nil {
		log.Errorf("load config %s error %v", defaultConfig.path, err)
	}
	if logPath == "" {
		logPath = defaultConfig.LogPath
	}
	if logTag == "" {
		logTag = defaultConfig.LogTag
	}
	defaultConfig.LogTag = parseCommandLine(os.Args[1:], "log", logTag)
	defaultConfig.LogPath = parseCommandLine(os.Args[1:], "logpath", logPath)

	log.Create(defaultConfig.LogPath)
	log.SetLevel(defaultConfig.LogTag)
}

// 解析命令行参数，支持4种格式
// 1、-name=value
// 2、-name value
// 3、--name=value
// 4、--name value
func parseCommandLine(args []string, name, def string) string {
	s := " " + strings.Join(args, " ")
	re := regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))\S+`)

	// 匹配的子串
	s = re.FindString(s)
	if s == "" {
		return def
	}
	re = regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))`)

	// 前缀关键字
	prefix := re.FindString(s)
	return s[len(prefix):]
}
