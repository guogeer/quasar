package env

// 默认获取进程工作路径下config.xml

import (
	"encoding/xml"
	"io/ioutil"
	"third/log"
)

func ConfigLoad(path string, conf interface{}) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error(err)
		return err
	}
	if err = xml.Unmarshal(b, &conf); err != nil {
		log.Error(path, err)
		return err
	}
	return nil
}

func ConfigSave(path string, conf interface{}) error {
	b, err := xml.Marshal(conf)
	if err != nil {
		log.Error(err)
		return err
	}
	if err = ioutil.WriteFile(path, b, 0666); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

type DataSource struct {
	User     string `xml:"User"`
	Password string `xml:"Password"`
	Addr     string `xml:"Address"`
	Name     string `xml:"Name"`
}

type Server struct {
	Name   string
	Addr   string `xml:"Address"`
	SubIds []int  `xml:"Room>SubId"`
}

type Router struct {
	Addr string `xml:"Address"`
}

type XMLConfig struct {
	Version          int
	Environment      string
	Resource         string
	IconURL          string
	Sign             string
	DataSource       DataSource
	ManageDataSource DataSource
	SlaveDataSource  DataSource
	ServerList       []*Server `xml:"ServerList>Server"`
	Router           Router

	ProductKey  string
	ProductName string
	ServerID    string
	path        string
}

func (conf *XMLConfig) Server(name string) *Server {
	for _, server := range conf.ServerList {
		if server.Name == name {
			return server
		}
	}
	return nil
}

var defaultConfig *XMLConfig

func NewConfig(path string) *XMLConfig {
	conf := &XMLConfig{path: path}
	ConfigLoad(path, conf)
	return conf
}

func init() {
	path := "config.xml"
	log.Infof("load all config %s", path)
	defaultConfig = NewConfig(path)

	switch Config().Environment {
	case "test":
		log.SetLevel(log.LTest)
	case "release":
		log.SetLevel(log.LError)
	}
	// loadAllTables()
}

func Config() *XMLConfig {
	return defaultConfig
}
