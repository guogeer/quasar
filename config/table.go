package config

// 配置表格式仅支持制表符TAB分隔的表格
// 表格第一行为字段解释
// 表格第二行为字段KEY
// 表格第一列默认索引
// Version 1.0.0 支持隐藏属性，属性格式：".Key"，其中".Private"私有属性
// Version 1.1.0 列索引忽略大小写
// Version 1.2.0 表格名索引忽略大小写
// 2019-12-03 增加类型: INT、JSON、FLOAT、STRING、JSON支持
// 例如：列1[INT]	列2[JSON]	列3[FLOAT]

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guogeer/quasar/log"
)

const (
	tableFileSuffix   = ".tbl"
	attrTable         = "system_table_field"
	tableRowKeyPrefix = "_default_table_line_"
	privateColKey     = ".private" // 该列可以直接访问
)

var (
	enableDebug   = false
	gTableFiles   sync.Map
	gTableRowKeys [255]*tableRow

	errUnsupportType = errors.New("unsupport table cell arg")
)

func init() {
	for i := range gTableRowKeys {
		gTableRowKeys[i] = newTableRow(i)
	}
}

type tableCell struct {
	s string
	n int64
	f float64
	b bool

	rawColKey string
}

type tableFile struct {
	rowName  map[string]int // 行名
	table    []map[string]*tableCell
	name     string
	colTypes map[string]string // 字段类型。Col1: JSON

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

func parseTable2Array(buf []byte) [][]string {
	body := strings.ReplaceAll(string(buf), "\r\n", "\n")
	body = strings.TrimRight(body, "\n")

	var cells [][]string
	for _, line := range strings.Split(body, "\n") {
		cells = append(cells, strings.Split(line, "\t"))
	}
	return cells
}

func parseColTypes(colName string) []string {
	colName = strings.ToLower(colName)
	keys := regexp.MustCompile(`\[[^\]]+\]`).FindString(colName)
	types := regexp.MustCompile(`[A-Za-z0-9]+`).FindAllString(keys, -1)
	return types
}

func loadTableFile(name string, buf []byte) (*tableFile, error) {
	name = strings.ToLower(name)
	f := &tableFile{
		name:     name,
		rowName:  make(map[string]int),
		colTypes: make(map[string]string),
	}
	cells := parseTable2Array(buf)

	var line0, line1 []string
	if len(cells) > 1 {
		line0, line1 = cells[0], cells[1]
	}

	for k := 0; k < len(line0) && k < len(line1); k++ {
		types := parseColTypes(line0[k])
		f.colTypes[strings.ToLower(line1[k])] = strings.Join(types, ",")
	}
	for rowID := 2; rowID < len(cells); rowID++ {
		lineCells := cells[rowID]
		rowName := string(lineCells[0])
		f.rowName[rowName] = rowID - 2

		cells := map[string]*tableCell{}
		for k, cell := range lineCells {
			colKey := strings.ToLower(line1[k])
			if colKey == privateColKey && len(cell) > 0 {
				attrs := make(map[string]json.RawMessage)
				json.Unmarshal([]byte(cell), &attrs)
				for attrk, attrv := range attrs {
					key := strings.ToLower(attrk)
					s := string(attrv)
					// 格式"message"移除前缀后缀
					if regexp.MustCompile(`^".*"$`).MatchString(s) {
						s = s[1 : len(s)-1]
					}
					cells[key] = &tableCell{s: s, rawColKey: attrk}
				}
			}
			cells[colKey] = &tableCell{s: cell, rawColKey: line1[k]}
		}
		for _, cell := range cells {
			cell.n, _ = strconv.ParseInt(cell.s, 10, 64)
			cell.f, _ = strconv.ParseFloat(cell.s, 64)
			cell.b, _ = strconv.ParseBool(cell.s)
		}
		f.table = append(f.table, cells)
	}
	return f, nil
}

func (f *tableFile) Cell(row, col interface{}) (*tableCell, bool) {
	if row == nil || col == nil {
		return nil, false
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

	if rowN >= 0 && rowN < len(f.table) {
		cell, ok := f.table[rowN][colKey]
		return cell, ok
	}
	return nil, false
}

func (f *tableFile) String(row, col interface{}) (string, bool) {
	if c, ok := f.Cell(row, col); ok {
		return c.s, true
	}
	return "", false
}

func (f *tableFile) Rows() []*tableRow {
	if len(f.table) < len(gTableRowKeys) {
		return gTableRowKeys[:len(f.table)]
	}

	var rows []*tableRow
	for k := range f.table {
		rows = append(rows, newTableRow(k))
	}
	return rows
}

// 每一行的数据个数不一定相同
func (f *tableFile) Cols(rowIndex int) []string {
	if rowIndex >= len(f.table) {
		return nil
	}

	var cols []string
	for _, cell := range f.table[rowIndex] {
		cols = append(cols, cell.rawColKey)
	}
	return cols
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

func (g *tableGroup) Cell(row, col interface{}) (*tableCell, bool) {
	for _, name := range g.members {
		if f := getTableFile(name); f != nil {
			if cell, ok := f.Cell(row, col); ok {
				return cell, ok
			}
		}
	}
	return nil, false
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

func (g *tableGroup) IsType(col string, typ string) bool {
	typ = strings.ToLower(typ)
	for _, name := range g.members {
		if f := getTableFile(name); f != nil {
			if s, ok := f.colTypes[col]; ok {
				return strings.Contains(","+s+",", ","+typ+",")
			}
		}
	}
	return false
}

func (cell *tableCell) Scan(arg interface{}) error {
	switch v := arg.(type) {
	default:
		if _, ok := arg.(Scanner); !ok {
			return errUnsupportType
		}
	case *time.Duration:
		arg = (*durationArg)(v)
	case *time.Time:
		arg = (*timeArg)(v)
	case *int8:
		*v = (int8)(cell.n)
	case *int16:
		*v = (int16)(cell.n)
	case *int32:
		*v = (int32)(cell.n)
	case *int64:
		*v = (int64)(cell.n)
	case *int:
		*v = (int)(cell.n)
	case *uint8:
		*v = (uint8)(cell.n)
	case *uint16:
		*v = (uint16)(cell.n)
	case *uint32:
		*v = (uint32)(cell.n)
	case *uint64:
		*v = (uint64)(cell.n)
	case *uint:
		*v = (uint)(cell.n)
	case *float32:
		*v = (float32)(cell.f)
	case *float64:
		*v = (float64)(cell.f)
	case *string:
		*v = cell.s
	case *bool:
		*v = cell.b
	}

	if scanner, ok := arg.(Scanner); ok {
		return scanner.Scan(cell.s)
	}
	return nil
}

// 跳过表格不存在的元素
func (g *tableGroup) Scan(row, cols interface{}, args ...interface{}) (int, error) {
	s := fmt.Sprintf("%v", cols)
	colKeys := strings.Split(s, ",")
	if len(colKeys) != len(args) {
		panic("args not match")
	}

	var counter int   // 读取成功的参数数量
	var lastErr error // 最后产生的错误
	for i, arg := range args {
		colKey := strings.ToLower(colKeys[i])
		if cell, exist := g.Cell(row, colKey); exist {
			// 优先匹配int/string等基本数据类型
			err := cell.Scan(arg)
			if err != nil {
				lastErr = err
			}
			if err == errUnsupportType {
				// 若列配置了[JSON]，则尝试解析成JSON类型
				if g.IsType(colKey, "JSON") {
					err = cell.Scan(JSON(arg))
				}
			}
			if err == nil {
				counter++
			} else {
				lastErr = err
			}
			// 配置读取错误容易引起BUG，直接异常退出
			if err == errUnsupportType {
				panic(err)
			}
		}
	}
	return counter, lastErr
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
// 120、120s、120m、120h，分别表示秒，分，时
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
		return len(f.table)
	}
	return -1
}

func RowId(n int) *tableRow {
	if n >= 0 && n < len(gTableRowKeys) {
		return gTableRowKeys[n]
	}
	return newTableRow(n)
}

func LoadTable(name string, buf []byte) error {
	name = strings.ToLower(name)
	if enableDebug {
		log.Infof("load table %s", name)
	}

	t, err := loadTableFile(name, buf)
	if err != nil {
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
		if isMatch {
			rows = append(rows, rowId)
		}
	}
	return rows
}

func Cols(name string, rowIndex int) []string {
	if f := getTableFile(name); f != nil {
		return f.Cols(rowIndex)
	}
	return nil
}
