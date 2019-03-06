package config

// 默认获取进程工作路径下config.xml

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"github.com/go-yaml/yaml"
	"io/ioutil"
	"os"
	pathlib "path"
)

func LoadConfig(path string, conf interface{}) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	switch pathlib.Ext(path) {
	default:
		err = errors.New("only support xml|json|yaml")
	case ".xml":
		err = xml.Unmarshal(b, &conf)
	case ".json":
		err = json.Unmarshal(b, &conf)
	case ".yaml":
		err = yaml.Unmarshal(b, &conf)
	}
	if err != nil {
		panic(err)
	}
	return nil
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
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	f, _ := os.Open(os.DevNull)
	fs.SetOutput(f) // 不打印错误信息
	path := fs.String("config", "config.xml", "config file path")
	fs.Parse(os.Args[1:])
	f.Close()

	LoadConfig(*path, &defaultConfig)
	defaultConfig.path = *path
}

func Config() Env {
	return defaultConfig
}
