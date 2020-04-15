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
	gRuntime       = &Runtime{
		files:   make(map[string]*scriptFile),
		modules: make(map[string]lua.LGFunction),
	}
)

type scriptFile struct {
	L    *lua.LState
	path string
	run  sync.RWMutex
}

func NewScriptFile(root, path string) *scriptFile {
	f := &scriptFile{
		path: path,
		L:    lua.NewState(),
	}
	// 默认加载脚本同目录下模块
	if root != "" {
		code := fmt.Sprintf(`package.path="%s/?.lua;..package.path"`, root)
		// fmt.Println(code)
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
		if scanner, ok := arg.(Scanner); ok {
			err = scanner.Scan(ret)
		} else if lua.LVCanConvToString(ret) {
			s := ret.String()
			if sp, ok := arg.(*string); ok {
				*sp, err = s, nil
			} else {
				// 遇到分隔符会停止
				_, err = fmt.Sscanf(s, "%v", arg)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// 返回值通过参数传入
// TODO 传入参数仅支持数值、字符串
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
	// TODO 返回值暂时不考虑
	top := L.GetTop()
	rets := make([]lua.LValue, 0, 4)
	for i := oldTop; i < top; i++ {
		rets = append(rets, L.Get(-1))
		L.Pop(1)
	}
	return &Result{rets: rets}
}

type Runtime struct {
	files   map[string]*scriptFile
	modules map[string]lua.LGFunction
	mtx     sync.RWMutex
}

func (rt *Runtime) PreloadModule(name string, loader lua.LGFunction) {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	rt.modules[name] = loader
}

func (rt *Runtime) LoadString(root, path, body string) error {
	script := NewScriptFile(root, path)
	rt.mtx.RLock()
	for module, loader := range rt.modules {
		script.L.PreloadModule(module, loader)
	}
	oldScript, ok := rt.files[path]
	rt.mtx.RUnlock()
	if err := script.L.DoString(body); err != nil {
		log.Info("load script", err)
		return err
	}

	rt.mtx.Lock()
	rt.files[path] = script
	rt.mtx.Unlock()

	if ok == true {
		oldScript.run.Lock()
		if oldScript.L != nil {
			oldScript.L.Close()
		}
		oldScript.L = nil
		oldScript.run.Unlock()
	}

	return nil
}

func (rt *Runtime) Call(fileName, funcName string, args ...interface{}) *Result {
	rt.mtx.RLock()
	script, ok := rt.files[fileName]
	rt.mtx.RUnlock()
	if ok == false {
		return &Result{Err: ErrInvalidFile}
	}

	script.run.RLock()
	defer script.run.RUnlock()
	return script.Call(funcName, args...)
}

func loadString(root, path, body string) error {
	return gRuntime.LoadString(root, path, body)
}

func Call(fileName, funcName string, args ...interface{}) *Result {
	res := gRuntime.Call(fileName, funcName, args...)
	if res.Err != nil {
		log.Errorf("call script %s:%s error: %v", fileName, funcName, res.Err)
	}
	return res
}

func PreloadModule(name string, f lua.LGFunction) {
	gRuntime.PreloadModule(name, f)
}

// 遇到脚本引入模块路径查找失败问题
// 简化设计，移除脚本压缩包加载支持
func loadScripts(dir, filename string) error {
	log.Debugf("start load %s/%s", dir, filename)
	// 脚本目录存在
	if fileInfo, err := os.Stat(dir); err != nil || !fileInfo.IsDir() {
		return errors.New("scripts is not exists")
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
		if filename != "" && filename != name {
			return nil
		}
		// log.Infof("load script %s %s", path, name)
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		log.Debugf("load script %s from %s/", name, dir)
		if err := loadString(dir, name, string(buf)); err != nil {
			return err
		}
		return nil
	})
}

func LoadLocalScripts(dir string) error {
	return loadScripts(dir, "")
}

func LoadLocalScriptByName(dir, name string) error {
	return loadScripts(dir, name)
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
		if _, ok := m[i]; !ok {
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
