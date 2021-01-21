package script

import (
	"encoding/json"
	"testing"

	"github.com/guogeer/quasar/util"
	lua "github.com/yuin/gopher-lua"
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
	B   bool
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
	if !util.EqualJSON(p, p2) {
		t.Error("not equal", p, p2)
	}

	ret := &testStruct{}
	expectRet := &testStruct{I: 123, S: "Hello World", B: true}
	res.Scan(&ret.I, &ret.S, &ret.B)
	if !util.EqualJSON(ret, expectRet) {
		t.Errorf("return %v, expect %v", ret, expectRet)
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
	Call("test1.lua", "test_sum", 1, 2, []int{4, 5, 6}).Scan(&n)
	if n != 18 {
		t.Error("fail sum", n)
	}
	type json1 struct {
		A int
		B int
		S string
	}
	var data1, data2 json1
	Call("test1.lua", "test_json").Scan(JSON(&data1))
	data2 = json1{
		A: 1,
		B: 2,
		S: "hello world",
	}
	if !util.EqualJSON(data1, data2) {
		t.Error("scan json", data1, data2)
	}
}

func TestGenericMap(t *testing.T) {
	genericMap := map[interface{}]interface{}{
		"S1": 1,
		"A1": map[interface{}]interface{}{
			"1": "abc",
			"2": "cde",
		},
	}
	expectMap := map[string]interface{}{
		"S1": 1,
		"A1": []string{"abc", "cde"},
	}
	if !util.EqualJSON((GenericMap)(genericMap), expectMap) {
		buf, _ := json.Marshal((GenericMap)(genericMap))
		t.Error("encode generic map", genericMap, expectMap, string(buf))
	}
}

type InheritA struct {
	A1 int
}

type InheritB struct {
	InheritA
	B1 int
}

type InheritC struct {
	InheritB
	C1 int
}

func TestInherit(t *testing.T) {
	c := &InheritC{}
	Call("test1.lua", "set_inherit", c)
	if c.A1 != 10 {
		t.Error("set fail", c.A1)
	}
}

type testRegistery struct {
	N int
}

func (t *testRegistery) Add(n int) *testRegistery {
	t.N = t.N + n
	return t
}

func TestRegistryOverflow(t *testing.T) {
	c := &testRegistery{}
	for i := 0; i < 10_0000; i++ {
		Call("test1.lua", "test_registry_overflow", c, i)
	}
}
