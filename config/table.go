package config

// 配置表格式仅支持制表符TAB分隔的表格
// 表格第一行为字段解释
// 表格第二行为字段KEY
// 表格第一列默认索引
// Version 1.0.0 支持隐藏属性，属性格式：".Key"，其中".Private"私有属性
// Version 1.1.0 列索引忽略大小写
// Version 1.2.0 表格名索引忽略大小写
// 2019-12-03 增加类型: INT、JSON、FLOAT、STRING，后续计划增强JSON支持
// 例如：列1[INT]	列2[JSON]	列3[FLOAT]
// Version 2.0.0 计划支持SQL语句

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guogeer/quasar/log"
	"github.com/guogeer/quasar/util"
)

const (
	tableFileSuffix   = ".tbl"
	attrTable         = "system_table_field"
	tableRowKeyPrefix = "_default_table_line_"
)

var (
	enableDebug   = false
	gTableFiles   sync.Map
	gTableRowKeys [255]*tableRow
)

func init() {
	for i := range gTableRowKeys {
		gTableRowKeys[i] = newTableRow(i)
	}
}

type tableFile struct {
	rowName  map[string]int // 行名
	cells    []map[string]string
	name     string
	colTypes map[string]string

	groups map[string]*tableGroup
}

type tableRow struct {
	n   int
	key string
}

func newTableRow(i int) *tableRow {
	return &tableRow{n: i, key: tableRowKeyPrefix + strconv.Itoa(i)}
}

func (r *tableRow) String() string {
	return r.key
}

func newTableFile(name string) *tableFile {
	name = strings.ToLower(name)
	return &tableFile{
		name:     name,
		rowName:  make(map[string]int),
		colTypes: make(map[string]string),
	}
}

func (f *tableFile) Load(buf []byte) error {
	// f.rowName = make(map[string]int)
	body := string(buf) + "\n"
	// windows 环境下环境"\r\n"替换为"\n"
	body = strings.ReplaceAll(body, "\r\n", "\n")

	var line0, line1 []string
	for rowID := 0; len(body) > 0; rowID++ {
		line := body
		if n := strings.Index(body, "\n"); n >= 0 {
			line = body[:n]
		}
		body = body[len(line)+1:]
		// 忽略空字符串
		if b, _ := regexp.MatchString(`\S`, line); !b {
			// log.Warnf("config %s:%d empty", f.name, rowID)
			continue
		}

		lineCells := strings.Split(line, "\t")
		switch rowID {
		// 表格第一行为标题注释，忽略
		// 2019-12-03 增加列字段数据类型
		case 0:
			line0 = lineCells
		case 1: // 表格第二行列索引
			line1 = lineCells
			for k, cell := range line0 {
				keys := regexp.MustCompile(`\[[^\]]+\]`).FindString(cell)
				types := regexp.MustCompile(`[A-Za-z0-9]+`).FindAllStringSubmatch(keys, -1)
				// TODO 支持多个关键字，当前仅考虑支持一个数据类型
				for _, typ := range types {
					if len(typ) > 0 {
						f.colTypes[line1[k]] = typ[0]
					}
				}
			}
		default:
			rowName := string(lineCells[0])
			f.rowName[rowName] = rowID - 2

			cells := make(map[string]string)
			for k, cell := range lineCells {
				colKey := strings.ToLower(string(line1[k]))
				if colKey == ".private" && len(cell) > 0 {
					attrs := make(map[string]json.RawMessage)
					json.Unmarshal([]byte(cell), &attrs)
					for attrk, attrv := range attrs {
						attrk = strings.ToLower(attrk)
						s := string(attrv)
						// 格式"message"移除前缀后缀
						if ok, _ := regexp.MatchString(`^".*"$`, s); ok {
							s = s[1 : len(s)-1]
						}
						cells[attrk] = s
					}
				}
				cells[colKey] = string(cell)
			}
			f.cells = append(f.cells, cells)
		}
	}
	// 列索引忽略大小写
	return nil
}

func (f *tableFile) String(row, col interface{}) (string, bool) {
	if row == nil || col == nil {
		return "", false
	}
	colKey := fmt.Sprintf("%v", col)
	colKey = strings.ToLower(colKey)

	rowN := -1
	rowKey := fmt.Sprintf("%v", row)
	if n := strings.Index(rowKey, tableRowKeyPrefix); n == 0 {
		rowN, _ = strconv.Atoi(rowKey[len(tableRowKeyPrefix):])
	}
	if n, ok := f.rowName[rowKey]; ok {
		rowN = n
	}

	if rowN >= 0 && rowN < len(f.cells) {
		s, ok := f.cells[rowN][colKey]
		return s, ok
	}
	return "", false
}

func (f *tableFile) Rows() []*tableRow {
	if len(f.cells) < len(gTableRowKeys) {
		return gTableRowKeys[:len(f.cells)]
	}

	rows := make([]*tableRow, 0, 32)
	for k := range f.cells {
		rows = append(rows, newTableRow(k))
	}
	return rows
}

type tableGroup struct {
	members []string
}

func getTableGroup(name string) *tableGroup {
	name = strings.ToLower(name)
	if f := getTableFile(attrTable); f != nil {
		if group, ok := f.groups[name]; ok {
			return group
		}
	}
	name = strings.ToLower(name)
	return &tableGroup{members: []string{name}}
}

func (g *tableGroup) Rows() []*tableRow {
	var rows []*tableRow
	for _, name := range g.members {
		if f := getTableFile(name); f != nil {
			if fileRows := f.Rows(); len(rows) < len(fileRows) {
				rows = fileRows
			}
		}
	}
	return rows
}

func (g *tableGroup) String(row, col interface{}) (string, bool) {
	for _, name := range g.members {
		if f := getTableFile(name); f != nil {
			if s, ok := f.String(row, col); ok {
				return s, ok
			}
		}
	}
	return "", false
}

func (g *tableGroup) Type(col string) (string, bool) {
	for _, name := range g.members {
		if f := getTableFile(name); f != nil {
			if s, ok := f.colTypes[col]; ok {
				return s, ok
			}
		}
	}
	return "", false
}

func scanOne(val reflect.Value, s string) {
	if s == "" {
		return
	}
	switch util.ConvertKind(val.Kind()) {
	default:
		panic("unsupport type" + val.Type().String())
	case reflect.Ptr:
		scanOne(val.Elem(), s)
	case reflect.Int64:
		n, _ := strconv.ParseInt(s, 10, 64)
		val.SetInt(n)
	case reflect.Uint64:
		n, _ := strconv.ParseUint(s, 10, 64)
		val.SetUint(n)
	case reflect.Float64:
		a, _ := strconv.ParseFloat(s, 64)
		val.SetFloat(a)
	case reflect.Bool:
		b, _ := strconv.ParseBool(s)
		val.SetBool(b)
	case reflect.String:
		val.SetString(s)
	case reflect.Slice:
		ss := ParseStrings(s)
		newval := reflect.MakeSlice(val.Type(), len(ss), len(ss))
		for i, s2 := range ss {
			scanOne(newval.Index(i), s2)
		}
		val.Set(newval)
	}
}

// 跳过表格不存在的元素
func (g *tableGroup) Scan(row, cols interface{}, args ...interface{}) (int, error) {
	s := fmt.Sprintf("%v", cols)
	colKeys := strings.Split(s, ",")
	if len(colKeys) != len(args) {
		panic("args not match")
	}

	counter := 0
	for i, arg := range args {
		colKey := strings.ToLower(colKeys[i])
		if s, exist := g.String(row, colKey); exist {
			counter++
			switch arg.(type) {
			case *time.Duration:
				arg = (*durationArg)(arg.(*time.Duration))
			case *time.Time:
				arg = (*timeArg)(arg.(*time.Time))
			}
			// 自动解析JSON
			if typ, ok := g.Type(colKey); ok && typ == "JSON" {
				arg = &jsonArg{value: arg}
			}

			if scanner, ok := arg.(Scanner); ok {
				scanner.Scan(s)
			} else {
				scanOne(reflect.ValueOf(arg), s)
			}
		}
	}
	return counter, nil
}

func readTableFile(path string, rc io.ReadCloser) {
	if rc == nil {
		return
	}
	defer rc.Close()

	ext := filepath.Ext(path)
	if ext != tableFileSuffix {
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
		if enableDebug {
			log.Infof("load tables %s", zipFile)
		}
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
			readTableFile(f.Name, rc)
		}
	}
	// 第二部加载scripts/*.tbl
	fileInfo, err := os.Stat(fileName)
	if err == nil && fileInfo.IsDir() {
		if enableDebug {
			log.Infof("load tables %s/*", fileName)
		}
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
			readTableFile(path, rc)
		}
	}
}

func getTableFile(name string) *tableFile {
	name = strings.ToLower(name)
	if f, ok := gTableFiles.Load(name); ok {
		return f.(*tableFile)
	}
	return nil
}

func Rows(name string) []*tableRow {
	if f := getTableFile(name); f != nil {
		return f.Rows()
	}
	return nil
}

func String(name string, row, col interface{}, def ...string) (string, bool) {
	var res string
	for _, v := range def {
		res = v
	}
	n, _ := getTableGroup(name).Scan(row, col, &res)
	return res, n == 1
}

func Int(name string, row, col interface{}, def ...int64) (int64, bool) {
	var res int64
	for _, v := range def {
		res = v
	}
	n, _ := getTableGroup(name).Scan(row, col, &res)
	return res, n == 1
}

func Float(name string, row, col interface{}, def ...float64) (float64, bool) {
	var res float64
	for _, v := range def {
		res = v
	}
	n, _ := getTableGroup(name).Scan(row, col, &res)
	return res, n == 1
}

func Time(name string, row, col interface{}, def ...time.Time) (time.Time, bool) {
	var res time.Time
	for _, v := range def {
		res = v
	}
	n, _ := getTableGroup(name).Scan(row, col, &res)
	return res, n == 1
}

// 默认单位秒
// 120s、120m、120h、120d，分别表示秒，分，时，天
func Duration(name string, row, col interface{}, def ...time.Duration) (time.Duration, bool) {
	var res time.Duration
	for _, v := range def {
		res = v
	}
	n, _ := getTableGroup(name).Scan(row, col, &res)
	return res, n == 1
}

func Scan(name string, row, colArgs interface{}, args ...interface{}) (int, error) {
	return getTableGroup(name).Scan(row, colArgs, args...)
}

func NumRow(name string) int {
	if f := getTableFile(name); f != nil {
		return len(f.cells)
	}
	return -1
}

func RowId(n int) *tableRow {
	if n >= 0 && n < len(gTableRowKeys) {
		return gTableRowKeys[n]
	}
	return newTableRow(n)
}

// TODO 当前仅支持,分隔符
func IsPart(s string, match interface{}) bool {
	smatch := fmt.Sprintf("%v", match)
	return strings.Contains(","+s+",", ","+smatch+",")
}

func LoadTable(name string, buf []byte) error {
	name = strings.ToLower(name)
	t := newTableFile(name)
	if enableDebug {
		log.Infof("load table %s", name)
	}
	if err := t.Load(buf); err != nil {
		return err
	}
	if name == attrTable {
		t.groups = make(map[string]*tableGroup)
		for _, row := range t.Rows() {
			s, _ := t.String(row, "Field")
			path := strings.Split(s, ".")
			if len(path) > 1 && path[1] == "*" {
				gname, ok := t.String(row, "Group")
				if ok && gname != "" {
					gname = strings.ToLower(gname)
					if _, ok := t.groups[gname]; !ok {
						// 2020-11-23 重新设定分组
						t.groups[gname] = &tableGroup{members: []string{gname}}
					}
					g := t.groups[gname]
					g.members = append(g.members, path[0])
				}
			}
		}
	}
	gTableFiles.Store(name, t)
	return nil
}

// 过滤表格行
// cols：多个列名。例如col1,col2,col3
// cells：过滤的值，对应列
func FilterRows(name string, cols string, vals ...interface{}) []*tableRow {
	colKeys := strings.Split(cols, ",")
	if len(colKeys) != len(vals) {
		panic("filter rows args not match")
	}

	var rows []*tableRow
	var sVals []string
	for _, v := range vals {
		sVals = append(sVals, fmt.Sprintf("%v", v))
	}
	tg := getTableGroup(name)
	for _, rowId := range tg.Rows() {
		isMatch := true
		for i := range colKeys {
			cell, _ := tg.String(rowId, colKeys[i])
			if cell != sVals[i] {
				isMatch = false
				break
			}
		}
		if isMatch == true {
			rows = append(rows, rowId)
		}
	}
	return rows
}
