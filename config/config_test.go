package config

import (
	"github.com/guogeer/quasar/utils"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	env1 := &Env{
		ClientKey: "helloworld!",
		ServerList: []server{
			{Name: "router", Addr: "127.0.0.1:9003"},
		},
	}
	env2 := &Env{}
	LoadFile("testdata/config.yaml", env2)
	if !utils.EqualJSON(env1, env2) {
		t.Error("not equal")
	}
}
