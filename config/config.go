package config

// 默认获取进程工作路径下config.xml

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"github.com/go-yaml/yaml"
	"github.com/guogeer/husky/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var gLogTag = flag.String("log", "DEBUG", "log DEBUG|INFO|ERROR")
var gLogPath = flag.String("logpath", "log/{proc_name}/run.log", "log path")
var gConfigPath = flag.String("config", "config.xml", "config xml|json|yaml")

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

	Sign            string
	ProductKey      string
	ServerList      []server `xml:"ServerList>Server"`
	CompressPackage int
	LogPath         string `xml:"Log>Path"`
	LogTag          string `xml:"Log>Tag"`
}

func (cf Env) Server(name string) server {
	for _, s := range cf.ServerList {
		if s.Name == name {
			return s
		}
	}
	return server{Name: name}
}

func (cf Env) Path() string {
	return cf.path
}

var defaultConfig Env

func init() {
	// 命令行参数优先
	logPath, logTag, path := *gLogPath, *gLogTag, *gConfigPath
	defaultConfig.path = ParseCmdArgs(os.Args[1:], "config", path)

	if err := LoadFile(defaultConfig.path, &defaultConfig); err != nil {
		log.Errorf("load config %s error %v", defaultConfig.path, err)
	}
	if logPath == "" {
		logPath = defaultConfig.LogPath
	}
	if logTag == "" {
		logTag = defaultConfig.LogTag
	}
	defaultConfig.LogTag = ParseCmdArgs(os.Args[1:], "log", logTag)
	defaultConfig.LogPath = ParseCmdArgs(os.Args[1:], "logpath", logPath)

	log.Create(defaultConfig.LogPath)
	log.SetLevelByTag(defaultConfig.LogTag)
}

func Config() Env {
	return defaultConfig
}

// 解析命令行参数，支持4种格式
// -name=value -name value --name=value --name value
func ParseCmdArgs(args []string, name, def string) string {
	s := " " + strings.Join(args, " ")
	re := regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))\S+`)

	s = re.FindString(s)
	if s == "" {
		return def
	}
	re = regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))`)

	prefix := re.FindString(s)
	return s[len(prefix):]
}
