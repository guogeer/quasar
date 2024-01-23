package config

// 进程启动时预加载的配置
// 默认获取进程工作路径下config.xml

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"quasar/log"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	gLogLevel   = flag.String("log-level", "DEBUG", "log DEBUG|INFO|ERROR")
	gLogPath    = flag.String("log-path", "log/{proc_name}/run.log", "log path")
	gConfigPath = flag.String("config-file", "", "config xml|json|yaml")
)

// 加载xml/json/yaml格式配置文件
func LoadFile(path string, conf any) error {
	b, err := os.ReadFile(path)
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
	Name string `yaml:"name"`
	Addr string `yaml:"address" xml:"Address"`
}

type Env struct {
	path string

	ProductKey string   `yaml:"productKey"`
	ServerList []server `yaml:"serverList" xml:"ServerList>Server"`
	Log        struct {
		Path  string `yaml:"path"`
		Level string `yaml:"level"`
	} `yaml:"log"`
	EnableDebug bool `yaml:"enableDebug"` // 开启调试，将输出消息统计日志等
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
	logPath, logLevel, path := *gLogPath, *gLogLevel, *gConfigPath
	defaultConfig.path = parseCommandLine(os.Args[1:], "config-file", path)
	// 未指定配置路径，默认加载当前目录下config.xml
	if defaultConfig.path == "" {
		path := "config.yaml"
		if _, err := os.Stat(path); err == nil {
			defaultConfig.path = path
		}
	}
	// 无配置
	if defaultConfig.path == "" {
		return
	}

	err := LoadFile(defaultConfig.path, &defaultConfig)
	if err != nil {
		log.Errorf("load config %s error %v", defaultConfig.path, err)
	}
	if logPath == "" {
		logPath = defaultConfig.Log.Path
	}
	if logLevel == "" {
		logLevel = defaultConfig.Log.Level
	}
	defaultConfig.Log.Level = parseCommandLine(os.Args[1:], "log-level", logLevel)
	defaultConfig.Log.Path = parseCommandLine(os.Args[1:], "log-path", logPath)

	log.Create(defaultConfig.Log.Path)
	log.SetLevel(defaultConfig.Log.Level)
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
