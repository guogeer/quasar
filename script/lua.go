package script

import (
	// "archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/guogeer/quasar/log"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
	luahelper "layeh.com/gopher-luar"
)

const fileSuffix = ".lua"

var (
	ErrUnkownType  = errors.New("unknow lua type")
	ErrInvalidFile = errors.New("script file not exist")
	gScriptSet     = &scriptSet{
		files:   make(map[string]*scriptFile),
		loading: make(map[string]*scriptFile),
		modules: make(map[string]lua.LGFunction),
	}
)

type scriptFile struct {
	L    *lua.LState
	path string
	run  sync.RWMutex
}

func newScriptFile(path string) *scriptFile {
	f := &scriptFile{
		path: path,
		L:    lua.NewState(),
	}
	// 默认加载脚本同目录下模块
	dir := filepath.Dir(path)
	if dir != "" {
		code := fmt.Sprintf(`package.path="%s/?.lua;..package.path"`, dir)
		if err := f.L.DoString(code); err != nil {
			panic("try load lua package error")
		}
	}
	return f
}

type Result struct {
	rets []lua.LValue
	Err  error
}

type Scanner interface {
	Scan(v lua.LValue) error
}

type jsonArg struct {
	value interface{}
}

func JSON(v interface{}) *jsonArg {
	return &jsonArg{value: v}
}

func (arg *jsonArg) Scan(v lua.LValue) error {
	b, _ := luajson.Encode(v)
	return json.Unmarshal(b, arg.value)
}

func (res Result) Scan(args ...interface{}) error {
	maxn := len(res.rets)
	if maxn > len(args) {
		maxn = len(args)
	}
	for i := 0; i < maxn; i++ {
		arg := args[i]
		ret := res.rets[i]
		err := ErrUnkownType
		if b, ok := arg.(*bool); ok && ret.Type() == lua.LTBool {
			*b = lua.LVAsBool(ret)
		} else if scanner, ok := arg.(Scanner); ok {
			err = scanner.Scan(ret)
		} else if lua.LVCanConvToString(ret) {
			// 遇到分隔符会停止
			_, err = fmt.Sscanf(ret.String(), "%v", arg)
			if s, ok := arg.(*string); ok {
				*s = ret.String()
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (script *scriptFile) Call(funcName string, args ...interface{}) *Result {
	L := script.L
	largs := make([]lua.LValue, 0, 4)
	for _, arg := range args {
		largs = append(largs, luahelper.New(L, arg))
	}
	oldTop := L.GetTop()
	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal(funcName),
		NRet:    lua.MultRet,
		Protect: true,
	}, largs...); err != nil {
		return &Result{Err: err}
	}
	top := L.GetTop()
	rets := make([]lua.LValue, 0, 4)
	for i := oldTop; i < top; i++ {
		rets = append(rets, L.Get(i-top))
	}
	L.Pop(len(rets))
	return &Result{rets: rets}
}

func (script *scriptFile) Close() {
	script.run.Lock()
	if script.L != nil {
		script.L.Close()
	}
	script.L = nil
	script.run.Unlock()
}

type scriptSet struct {
	files   map[string]*scriptFile // 已加载的脚本
	loading map[string]*scriptFile // 正在加载的脚本
	modules map[string]lua.LGFunction
	mtx     sync.RWMutex
}

func (set *scriptSet) GetFileName(L *lua.LState) (string, bool) {
	set.mtx.RLock()
	defer set.mtx.RUnlock()

	for name, sf := range set.files {
		if sf.L == L {
			return name, true
		}
	}
	for name, sf := range set.loading {
		if sf.L == L {
			return name, true
		}
	}
	return "", false
}

func (set *scriptSet) PreloadModule(name string, loader lua.LGFunction) {
	set.mtx.Lock()
	defer set.mtx.Unlock()

	set.modules[name] = loader
}

// path: load dir files or single file
func (set *scriptSet) LoadString(path, body string) error {
	script := newScriptFile(path)
	_, fileName := filepath.Split(path)

	set.mtx.RLock()
	set.loading[fileName] = script
	for module, loader := range set.modules {
		script.L.PreloadModule(module, loader)
	}
	oldScript, ok := set.files[fileName]
	set.mtx.RUnlock()

	if err := script.L.DoString(body); err != nil {
		set.mtx.Lock()
		delete(set.loading, fileName)
		set.mtx.Unlock()
		script.Close()

		return fmt.Errorf("load scripts %s error: %w ", path, err)
	}

	set.mtx.Lock()
	set.files[fileName] = script
	delete(set.loading, fileName)
	set.mtx.Unlock()

	if ok == true {
		oldScript.Close()
	}

	return nil
}

func (set *scriptSet) Call(fileName, funcName string, args ...interface{}) *Result {
	set.mtx.RLock()
	script, ok := set.files[fileName]
	set.mtx.RUnlock()
	if ok == false {
		return &Result{Err: ErrInvalidFile}
	}

	script.run.RLock()
	defer script.run.RUnlock()
	return script.Call(funcName, args...)
}

func loadString(path, body string) error {
	return gScriptSet.LoadString(path, body)
}

func GetFileName(L *lua.LState) (string, bool) {
	return gScriptSet.GetFileName(L)
}

func Call(fileName, funcName string, args ...interface{}) *Result {
	res := gScriptSet.Call(fileName, funcName, args...)
	if res.Err != nil {
		log.Errorf("call script %s:%s error: %v", fileName, funcName, res.Err)
	}
	return res
}

func PreloadModule(name string, f lua.LGFunction) {
	gScriptSet.PreloadModule(name, f)
}

// 遇到脚本引入模块路径查找失败问题
// 简化设计，移除脚本压缩包加载支持
func LoadScripts(path string) error {
	log.Debugf("start load %s", path)
	// 脚本目录存在
	pathInfo, err := os.Stat(path)
	if err != nil {
		return errors.New("scripts is not exists")
	}

	dir, fileName := path, ""
	if !pathInfo.IsDir() {
		dir, fileName = filepath.Split(path)
	}
	return filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return errors.New("walk scripts error")
		}
		if f.IsDir() {
			return nil
		}
		_, name := filepath.Split(path)
		if filepath.Ext(name) != fileSuffix {
			return nil
		}
		if fileName != "" && name != fileName {
			return nil
		}
		// log.Infof("load script %s %s", path, name)
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		log.Debugf("load script %s from %s/", name, dir)
		if err := loadString(path, string(buf)); err != nil {
			return err
		}
		return nil
	})
}

// Depercated: use LoadScripts instead
func LoadLocalScripts(dir string) error {
	return LoadScripts(dir)
}

// Depercated: use LoadScripts instead
func LoadLocalScriptByName(dir, name string) error {
	return LoadScripts(dir + "/" + name)
}

// map[interface{}]interface{} to json
// map[1:1 2:2] to [1,2]
type GenericMap map[interface{}]interface{}

func (m GenericMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	dict := map[string]interface{}{}
	for k, v := range m {
		s := fmt.Sprintf("%v", k)
		if child, ok := v.(map[interface{}]interface{}); ok {
			buf, err := json.Marshal(GenericMap(child))
			if err != nil {
				return buf, err
			}
			v = json.RawMessage(buf)
		}
		dict[s] = v
	}

	isArray := true
	for i := 1; isArray && i <= len(m); i++ {
		s := fmt.Sprintf("%v", i)
		if _, ok := dict[s]; !ok {
			isArray = false
		}
	}
	if isArray == false {
		return json.Marshal(dict)
	}

	array := make([]interface{}, 0, 4)
	for i := 1; i <= len(dict); i++ {
		array = append(array, dict[strconv.Itoa(i)])
	}
	return json.Marshal(array)
}
