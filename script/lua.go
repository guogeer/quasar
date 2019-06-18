package script

import (
	"archive/zip"
	"errors"
	"fmt"
	"github.com/guogeer/husky/log"
	"github.com/yuin/gopher-lua"
	"io/ioutil"
	luahelper "layeh.com/gopher-luar"
	"os"
	"path/filepath"
	"sync"
)

const fileSuffix = ".lua"

var (
	ErrInvalidFile = errors.New("script file not exist")
	gRuntime       = &Runtime{
		files:   make(map[string]*scriptFile),
		modules: make(map[string]lua.LGFunction),
	}
)

type scriptFile struct {
	L    *lua.LState
	Path string
	run  sync.RWMutex
}

func NewScriptFile(path string) *scriptFile {
	f := &scriptFile{
		Path: path,
		L:    lua.NewState(),
	}
	return f
}

type Result []lua.LValue

func (res Result) Scan(args ...interface{}) int {
	maxn := len(res)
	if maxn > len(args) {
		maxn = len(args)
	}
	for i := 0; i < maxn; i++ {
		fmt.Sscanf(res[i].String(), "%v", args[i])
	}
	return maxn
}

// 返回值通过参数传入
// TODO 传入参数仅支持数值、字符串
func (script *scriptFile) Call(funcName string, args ...interface{}) (Result, error) {
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
		return nil, err
	}
	// TODO 返回值暂时不考虑
	top := L.GetTop()
	res := make([]lua.LValue, 0, 4)
	for i := oldTop; i < top; i++ {
		res = append(res, L.Get(-1))
		L.Pop(1)
	}
	return res, nil
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

func (rt *Runtime) LoadString(path, body string) error {
	script := NewScriptFile(path)
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

func (rt *Runtime) Call(fileName, funcName string, args ...interface{}) (Result, error) {
	rt.mtx.RLock()
	script, ok := rt.files[fileName]
	rt.mtx.RUnlock()
	if ok == false {
		return nil, ErrInvalidFile
	}

	script.run.RLock()
	defer script.run.RUnlock()
	return script.Call(funcName, args...)
}

func LoadString(path, body string) error {
	return gRuntime.LoadString(path, body)
}

func Call(fileName, funcName string, args ...interface{}) (Result, error) {
	return gRuntime.Call(fileName, funcName, args...)
}

func PreloadModule(name string, f lua.LGFunction) {
	gRuntime.PreloadModule(name, f)
}

func loadScripts(dir, filename string) error {
	log.Debugf("start load %s/%s", dir, filename)

	// 首先判断{dir}.zip
	if _, err := os.Stat(dir + ".zip"); err == nil {
		r, err := zip.OpenReader(dir + ".zip")
		if err != nil {
			return err
		}
		defer r.Close()

		for _, f := range r.File {
			_, name := filepath.Split(f.Name)
			if filepath.Ext(name) != fileSuffix {
				continue
			}
			if filename != "" && name != filename {
				continue
			}

			rc, err := f.Open()
			if err != nil {
				return err
			}
			buf, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}
			rc.Close()

			log.Debugf("load script %s from %s.zip", name, dir)
			if err := LoadString(name, string(buf)); err != nil {
				return err
			}
		}
		return nil
	}

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
		if err := LoadString(name, string(buf)); err != nil {
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
