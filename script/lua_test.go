package script

import (
	"github.com/guogeer/husky/util"
	"github.com/yuin/gopher-lua"
	"testing"
)

func loadTestScripts() {
	PreloadModule("testpkg", externScript)
	LoadLocalScripts(".")
}

type testStruct struct {
	I32 int32
	I   int
	I64 int64
	F32 float32
	F64 float64
	S   string
	AI2 []int
	AS2 []string
}

func TestCall(t *testing.T) {
	loadTestScripts()

	p := &testStruct{
		AI2: []int{1, 2, 3},
		AS2: []string{"S", "B"},
	}
	p2 := testStruct{
		I32: 3032,
		I:   3064,
		I64: 3164,
		F32: 3032.3200,
		F64: 3064.6400,
		S:   "hello test3",
		AI2: []int{10, 10, 10},
		AS2: []string{"ss", "ss"},
	}
	res := Call("test1.lua", "testcall", p)
	if err := res.Err; err != nil {
		t.Error(err)
	}
	if !util.DeepEqual(p, p2) {
		t.Error("not equal", p, p2)
	}
	var s string
	res.Scan(&s)
	if s != "123" {
		t.Error("return", res)
	}
}

func callSum(L *lua.LState) int {
	m := L.ToInt(1)
	n := L.ToInt(2)
	L.Push(lua.LNumber(m + n))
	return 1
}

func externScript(L *lua.LState) int {
	exports := map[string]lua.LGFunction{
		"sum": callSum,
	}
	mod := L.SetFuncs(L.NewTable(), exports)
	L.Push(mod)
	return 1
}

func TestPreloadModule(t *testing.T) {
	LoadLocalScripts(".")

	var n int
	Call("test1.lua", "test_sum", 1, 2).Scan(&n)
	if n != 3 {
		t.Error("fail 1+2=3")
	}
}
