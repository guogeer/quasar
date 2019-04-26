package config

// 配置表格式仅支持制表符TAB分隔的表格
// 表格第一行为字段解释
// 表格第二行为字段KEY
// 表格第一列默认索引
// Version 2.0.0 支持隐藏属性，属性格式：".Key"，其中".Private"私有属性

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/guogeer/husky/log"
	"github.com/guogeer/husky/util"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tableFileSuffix = ".tbl"
	attrTable       = "system_table_field"
)

var (
	gTableFiles sync.Map
	gTableRows  [255]*tableRow
)

func init() {
	for i := range gTableRows {
		gTableRows[i] = &tableRow{n: i}
	}
}

type tableFile struct {
	rowName map[string]int // 行名
	cells   []map[string]string
	name    string

	groups map[string]*tableGroup
}

type tableRow struct {
	n int
}

func NewTableFile(name string) *tableFile {
	return &tableFile{
		name:    name,
		rowName: make(map[string]int),
	}
}

func (f *tableFile) Load(buf []byte) error {
	// f.rowName = make(map[string]int)
	buf = bytes.Replace(buf, []byte("\r\n"), []byte("\n"), -1)
	// 尾部追加换行
	if n := len(buf); n > 0 && buf[n-1] != '\n' {
		buf = append(buf, '\n')
	}

	var colKeys [][]byte
	for rowID := 0; len(buf) > 0; rowID++ {
		line := buf
		if n := bytes.IndexByte(buf, '\n'); n >= 0 {
			line = buf[:n]
		}
		buf = buf[len(line)+1:]

		lineCells := bytes.Split(line, []byte("\t"))
		if len(lineCells) == 0 {
			log.Warnf("config %s:%d empty", f.name, rowID)
		}

		switch rowID {
		case 0: // 表格第一行为标题注释，忽略
		case 1: // 表格第二行列索引
			colKeys = lineCells
		default:
			rowName := string(lineCells[0])
			f.rowName[rowName] = rowID - 2

			cells := make(map[string]string)
			for k, cell := range lineCells {
				colKey := string(colKeys[k])
				if colKey == ".Private" && len(cell) > 0 {
					attrs := make(map[string]interface{})
					json.Unmarshal(cell, &attrs)
					for attrk, attrv := range attrs {
						cells[attrk] = fmt.Sprintf("%v", attrv)
					}
				}
				cells[colKey] = string(cell)
			}
			f.cells = append(f.cells, cells)
		}
	}
	return nil
}

func (f *tableFile) String(row, col interface{}) (string, bool) {
	if f == nil || row == nil || col == nil {
		return "", false
	}
	colKey := fmt.Sprintf("%v", col)

	rowN := -1
	if r, ok := row.(*tableRow); ok {
		rowN = r.n
	} else {
		rowKey := fmt.Sprintf("%v", row)
		if n, ok := f.rowName[rowKey]; ok {
			rowN = n
		}
	}
	if rowN >= 0 && rowN < len(f.cells) {
		s, ok := f.cells[rowN][colKey]
		return s, ok
	}
	return "", false
}

func (f *tableFile) Rows() []*tableRow {
	if f == nil {
		return nil
	}
	if len(f.cells) < len(gTableRows) {
		return gTableRows[:len(f.cells)]
	}

	rows := make([]*tableRow, 0, 32)
	for k := range f.cells {
		rows = append(rows, &tableRow{n: k})
	}
	return rows
}

type tableGroup struct {
	name    string
	members []string
}

func getTableGroup(name string) *tableGroup {
	f := getTableFile(attrTable)
	if f != nil {
		group, ok := f.groups[name]
		if ok == true {
			return group
		}
	}
	return &tableGroup{name: name}
}

func (g *tableGroup) String(row, col interface{}) (string, bool) {
	if s, ok := getTableFile(g.name).String(row, col); ok {
		return s, ok
	}
	for _, name := range g.members {
		if s, ok := getTableFile(name).String(row, col); ok {
			return s, ok
		}
	}
	return "", false
}

func scanOne(val reflect.Value, s string) {
	if s == "" {
		return
	}
	switch val.Kind() {
	default:
		panic("unsupport type" + val.Type().String())
	case reflect.Ptr:
		scanOne(val.Elem(), s)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Int:
		n, _ := strconv.ParseInt(s, 10, 64)
		val.SetInt(n)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uint:
		n, _ := strconv.ParseUint(s, 10, 64)
		val.SetUint(n)
	case reflect.Float32, reflect.Float64:
		a, _ := strconv.ParseFloat(s, 64)
		val.SetFloat(a)
	case reflect.String:
		val.SetString(s)
	case reflect.Slice:
		ss := util.ParseStrings(s)
		newval := reflect.MakeSlice(val.Type(), len(ss), len(ss))
		for i, s2 := range ss {
			scanOne(newval.Index(i), s2)
		}
		val.Set(newval)
	}
}

func (g *tableGroup) Scan(row, cols interface{}, args []interface{}) (int, error) {
	s := fmt.Sprintf("%v", cols)
	colkeys := strings.Split(s, ",")
	if len(colkeys) != len(args) {
		panic("args not match")
	}
	for i := range args {
		s, _ := g.String(row, colkeys[i])
		SScan(s, args[i])
	}
	return 0, nil
}

func readFile(path string, rc io.ReadCloser) {
	if rc == nil {
		return
	}
	defer rc.Close()

	ext := filepath.Ext(path)
	if ext != tableFileSuffix {
		log.Debugf("emit table file %s", path)
		return
	}

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		log.Fatal(err)
	}
	base := filepath.Base(path)
	name := base[:len(base)-len(ext)]
	if err := LoadTable(name, buf); err != nil {
		log.Fatal(err)
	}
}

// 加载tables下所有的tbl文件
func LoadLocalTables(fileName string) {
	// 第一步加载tables.zip
	zipFile := fileName + ".zip"
	if _, err := os.Stat(zipFile); err == nil {
		log.Infof("load tables %s", zipFile)
		r, err := zip.OpenReader(fileName + ".zip")
		if err != nil {
			panic(err)
		}
		defer r.Close()

		for _, f := range r.File {
			rc, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			readFile(f.Name, rc)
		}
	}
	// 第二部加载scripts/*.tbl
	fileInfo, err := os.Stat(fileName)
	if err == nil && fileInfo.IsDir() {
		log.Infof("load tables %s/*", fileName)
		files, err := ioutil.ReadDir(fileName)
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			path := fileName + "/" + f.Name()
			rc, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			readFile(path, rc)
		}
	}
}

func getTableFile(name string) *tableFile {
	if f, ok := gTableFiles.Load(name); ok {
		return f.(*tableFile)
	}
	if name != attrTable {
		log.Errorf("cannot find config table %s", name)
	}
	return nil
}

func Rows(name string) []*tableRow {
	return getTableFile(name).Rows()
}

func String(name string, row, col interface{}, def ...string) (string, bool) {
	return getTableGroup(name).String(row, col)
}

func Int(name string, row, col interface{}, def ...int64) (int64, bool) {
	s, ok := getTableGroup(name).String(row, col)
	if ok == false {
		for _, n := range def {
			return n, false
		}
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Errorf("cell %v:%v:%v[%v] invalid %v", name, row, col, s, err)
	}
	return n, ok
}

func Float(name string, row, col interface{}, def ...float64) (float64, bool) {
	s, ok := getTableGroup(name).String(row, col)
	if ok == false {
		for _, a := range def {
			return a, false
		}
		return 0.0, false
	}
	a, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Errorf("cellfloat %v:%v:%v[%v] invalid %v", name, row, col, s, err)
	}
	return a, ok
}

func Time(name string, row, col interface{}) (time.Time, bool) {
	if s, ok := getTableGroup(name).String(row, col); ok {
		if t, err := util.ParseTime(s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// 默认单位秒
// 120s、120m、120h、120d，分别表示秒，分，时，天
func Duration(name string, row, col interface{}) (time.Duration, bool) {
	if s, ok := getTableGroup(name).String(row, col); ok && len(s) > 0 {
		if d, err := time.ParseDuration(s); err == nil {
			return d, true
		}
	}
	return 0, false
}

func SScan(s string, arg interface{}) {
	switch arg.(type) {
	default:
		scanOne(reflect.ValueOf(arg), s)
	case *time.Duration:
		d, _ := time.ParseDuration(s)
		*(arg.(*time.Duration)) = d
	case *time.Time:
		t, _ := util.ParseTime(s)
		*(arg.(*time.Time)) = t
	}
}

func Scan(name string, row, colArgs interface{}, args ...interface{}) (int, error) {
	return getTableGroup(name).Scan(row, colArgs, args)
}

func Row(name string) int {
	f := getTableFile(name)
	if f == nil {
		return -1
	}
	return len(f.cells)
}

func RowId(n int) *tableRow {
	if n >= 0 && n < len(gTableRows) {
		return gTableRows[n]
	}
	return &tableRow{n: n}
}

// TODO 当前仅支持,分隔符
func IsPart(s string, match interface{}) bool {
	smatch := fmt.Sprintf("%v", match)
	return strings.Contains(","+s+",", ","+smatch+",")
}

func LoadTable(name string, buf []byte) error {
	t := NewTableFile(name)
	log.Infof("load table %s", name)
	if err := t.Load(buf); err != nil {
		return err
	}
	if name == attrTable {
		t.groups = make(map[string]*tableGroup)
		for _, row := range t.Rows() {
			s, _ := t.String(row, "Field")
			path := strings.Split(s, ".")
			if len(path) > 1 && path[1] == "*" {
				name, ok := t.String(row, "Group")
				if ok && name != "" {
					if _, ok := t.groups[name]; !ok {
						t.groups[name] = &tableGroup{name: name}
					}
					g := t.groups[name]
					g.members = append(g.members, path[0])
				}
			}
		}
	}
	gTableFiles.Store(name, t)
	return nil
}
