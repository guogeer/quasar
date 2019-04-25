package config

// 默认获取进程工作路径下config.xml

import (
	"encoding/json"
	"encoding/xml"
	"github.com/go-yaml/yaml"
	"github.com/guogeer/husky/log"
	"io/ioutil"
	"os"
	pathlib "path"
	"regexp"
	"strings"
)

func LoadConfig(path string, conf interface{}) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	switch pathlib.Ext(path) {
	default:
		panic("only support xml|json|yaml")
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
	Sign       string
	ProductKey string
	ServerList []server `xml:"ServerList>Server"`
	path       string
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
	path := ParseArgs(os.Args[1:], "config")
	LoadConfig(path, &defaultConfig)
	defaultConfig.path = path

	tag := ParseArgs(os.Args[1:], "log")
	log.SetLevelByTag(tag)
}

func Config() Env {
	return defaultConfig
}

// 解析命令行参数，支持4种格式
// -name=value -name value --name=value --name value
func ParseArgs(args []string, name string) string {
	s := " " + strings.Join(args, " ")
	re := regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))\S+`)

	s = re.FindString(s)
	re = regexp.MustCompile(`\s+[-]{1,2}` + name + `(=|(\s+))`)

	prefix := re.FindString(s)
	return s[len(prefix):]
}
