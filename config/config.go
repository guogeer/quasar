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

	"gopkg.in/yaml.v3"
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
		err = xml.Unmarshal(b, conf)
	case ".json":
		err = json.Unmarshal(b, conf)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(b, conf)
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
		Path  string `yaml:"path" long:"log-path" default:"DEBUG" description:"log DEBUG|INFO|ERROR"`
		Level string `yaml:"level"  long:"log-level" default:"log/{proc_name}/run.log" description:"log file path"`
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
	conf := Config()
	// 参数优先级：命令行>配置文件
	newFlag := flag.NewFlagSet("init config", flag.ContinueOnError)
	configPath := newFlag.String("config", "config.yaml", "config file path")
	logLevel := newFlag.String("log-level", "DEBUG", "log DEBUG|INFO|ERROR")
	logPath := newFlag.String("log-path", "log/{proc_name}/run.log", "log path")

	newFlag.Parse(flag.CommandLine.Args())
	if *configPath != "" {
		conf.path = *configPath
	}
	conf.Log.Path = *logPath
	conf.Log.Level = *logLevel

	if conf.path != "" {
		err := LoadFile(conf.path, &conf)
		if err != nil {
			log.Errorf("load config %s error %v", conf.path, err)
		}
	}

	log.Create(conf.Log.Path)
	log.SetLevel(conf.Log.Level)
}
