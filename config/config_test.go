package config

import (
	"testing"

	"quasar/util"
)

func TestLoadConfig(t *testing.T) {
	env1 := &Env{
		ClientKey: "helloworld!",
		ServerList: []server{
			{Name: "router", Addr: "127.0.0.1:9003"},
		},
	}
	env2 := &Env{}
	LoadFile("testdata/config.xml", env2)
	if !util.EqualJSON(env1, env2) {
		t.Error("not equal")
	}
}
