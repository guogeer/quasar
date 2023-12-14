package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 校验配置表数据格式有效性
func ValidateConfigTable(buf []byte) error {
	cells := parseTable2Array(buf)
	if len(cells) < 2 {
		return errors.New("row line need more than 2")
	}
	for i := 1; i < len(cells); i++ {
		if len(cells[i]) != len(cells[0]) {
			return fmt.Errorf("col nums between row %d and row 1 is different %d!=%d", i+1, len(cells[i]), len(cells[0]))
		}
	}

	for i := 2; i < len(cells); i++ {
		lineId := i + 1
		lineCells := cells[i]
		if lineCells[0] == "" {
			return fmt.Errorf("row %d empty", lineId)
		}

		for k, cell := range lineCells {
			colTypes := parseColTypes(cells[0][k])

			var err error
			for _, typ := range colTypes {
				if cell == "" {
					break
				}
				switch strings.ToUpper(typ) {
				case "INT":
					_, err = strconv.ParseInt(cell, 10, 64)
				case "JSON":
					if !json.Valid([]byte(cell)) {
						err = errors.New("invalid JSON")
					}
				case "DURATION":
					if regexp.MustCompile(`[0-9]$`).MatchString(cell) {
						cell = cell + "s"
					}
					_, err = time.ParseDuration(cell)
				case "DATE":
					_, err = ParseTime(cell)
				case "FLOAT":
					_, err = strconv.ParseFloat(cell, 64)
				case "BOOL":
					_, err = strconv.ParseBool(cell)
				case "CLOCK":
					_, err = time.ParseInLocation(cell, "15:04", time.UTC)
				}
				if err != nil {
					return fmt.Errorf("invalid cell(%d,%d) data format [%s]: %v", lineId, k+1, typ, err.Error())
				}
			}
		}
	}
	return nil
}

// 导出JSON格式配置表
func ExportConfigTable(buf []byte) []byte {
	cells := parseTable2Array(buf)

	visibleCol := -1
	for i, colKey := range cells[1] {
		if strings.ToUpper(colKey) == ".VISIBLE" {
			visibleCol = i
		}
	}
	hideCols := map[string]bool{}
	for i := range cells[0] {
		colTypes := parseColTypes(cells[0][i])
		for _, typ := range colTypes {
			if visible := strings.ToUpper(typ); visible == "HIDE" {
				hideCols[cells[1][i]] = true
			}
		}
	}

	var tableRows []any
	for i := 2; i < len(cells); i++ {
		lineCells := cells[i]
		// hide row
		if visibleCol >= 0 && cells[i][visibleCol] == "HIDE" {
			continue
		}

		tableRow := map[string]any{}
		for k, cell := range lineCells {
			if hideCols[cells[1][k]] {
				continue
			}
			if strings.ToLower(cells[1][k]) == privateColKey && len(cell) > 0 {
				attrs := make(map[string]json.RawMessage)
				json.Unmarshal([]byte(cell), &attrs)
				for attrk, attrv := range attrs {
					s := string(attrv)
					// 格式"message"移除前缀后缀
					if regexp.MustCompile(`^".*"$`).MatchString(s) {
						s = s[1 : len(s)-1]
					}
					tableRow[attrk] = s
				}
			}
			tableRow[cells[1][k]] = cell
		}
		for key := range tableRow {
			cell := fmt.Sprintf("%v", tableRow[key])
			if f, err := strconv.ParseFloat(cell, 64); err == nil {
				tableRow[key] = f
			}
		}
		tableRows = append(tableRows, tableRow)
	}
	b, _ := json.Marshal(tableRows)
	return b
}
