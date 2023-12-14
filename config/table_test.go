package config

import (
	"encoding/json"
	"testing"
)

func TestMain(m *testing.M) {
	LoadLocalTables("testdata")
	m.Run()
}

func TestScan(t *testing.T) {
	var a int
	var b string
	var c bool
	var d float64
	var argn int

	argn, _ = Scan("test1", 1, "A,B,C,D", &a, &b, &c, &d)
	if !(argn == 4 && a == 10 && b == "HelloB1" && c && d == 10.1) {
		t.Error("scan config test1 row key:1 fail")
	}
	argn, _ = Scan("test1", 3, "A,B,C,D", &a, &b, &c, &d)
	if !(argn == 4 && a == 30 && b == "HelloB3" && !c && d == 30.1) {
		t.Error("scan config test1 row key:3 fail")
	}
	argn, _ = Scan("test1", 1, "PA,PB", &a, &b)
	if !(argn == 2 && a == 11 && b == "[10,11,12]") {
		t.Errorf("scan config test1 private data fail, 11!=%v [10,11,12]!=%v", a, b)
	}
	argn, _ = Scan("test1", 1, "PA,PB", &a, &b)
	if !(argn == 2 && a == 11 && b == "[10,11,12]") {
		t.Errorf("scan config test1 private data fail, 11!=%v [10,11,12]!=%v", a, b)
	}

	argn, _ = Scan("test1", RowId(0), "A,B,C,D", &a, &b, &c, &d)
	if !(argn == 4 && a == 10 && b == "HelloB1" && c && d == 10.1) {
		t.Error("scan config test1 RowId(0) fail")
	}
}

func TestFilterRows(t *testing.T) {
	rows1 := FilterRows("test2", "C1,C4", 11, "S1")
	if !(len(rows1) == 2 && rows1[0].n == 0 && rows1[1].n == 7) {
		t.Error("filter test2 11,S1 fail")
	}

	rows2 := FilterRows("test2", "C1,C4", 12, "S1")
	if len(rows2) != 0 {
		t.Error("filter test2 12,S1 fail")
	}
}

func TestScanTableGroup(t *testing.T) {
	a, ok := Int("test", 1, "A")
	if !(ok && a == 10) {
		t.Error("scan table group:test valid field fail")
	}
	a, ok = Int("test", 100, "A")
	if !(!ok && a == 0) {
		t.Error("scan table group:test empty field fail")
	}
}

func TestScanJSON(t *testing.T) {
	var arr []int
	Scan("test", 1, "E", &arr)
	if !(len(arr) == 2 && arr[0] == 1 && arr[1] == 1) {
		t.Errorf("scan table group:test (1,E) fail,[1,1] != %v", arr)
	}
}

func TestValidateConfigTable(t *testing.T) {
	contents := []string{
		`MyCol1[INT]	MyCol2[DATE]	MyCol3[DURATION]	MyCol4[JSON]
Col1	Col2	Col3	Col4
A	2021-01-01	100s	{"A":1,"B":1}
`,
		`MyCol1[INT]	MyCol2[DATE]	MyCol3[DURATION]	MyCol4[JSON]
Col1  Col2  Col3  Col4 
1	2021-01-32	100s	{"A":1,"B":1}
`,
		`MyCol1[INT]	MyCol2[DATE]	MyCol3[DURATION]	MyCol4[JSON]
Col1	Col2	Col3	Col4
1	2021-01-01	100a	{"A":1,"B":1}
`,
		`MyCol1[INT]	MyCol2[DATE]	MyCol3[DURATION]	MyCol4[JSON]
Col1	Col2	Col3 	Col4
1	2021-01-01	100a	{"A":1,"B":}
`,
	}
	for _, content := range contents {
		err := ValidateConfigTable([]byte(content))
		if err == nil {
			t.Error("check config table cell fail", content)
		}
	}
}

func TestExportConfigTable(t *testing.T) {
	contents := []string{
		`MyCol1[INT]	MyCol2[DATE|HIDE]	MyCol3[DURATION]	MyCol4[JSON]
Col1	Col2	Col3	.Private
A	2021-01-01	100s	{"A":1,"B":1}
`,
		`MyCol1[INT]	MyCol2[DATE]	MyVisible[HIDE]	MyCol4[JSON|HIDE]
Col1	Col2	Visible	.Private 
1	2021-01-01	HIDE	{"A":1,"B":1}
2	2021-01-02	HIDE	{}
3	2021-01-03	YES	{}
`,
	}
	exportTables := [][]map[string]any{
		{{
			"Col1":     "A",
			"Col3":     "100s",
			".Private": `{"A":1,"B":1}`,
			"A":        1,
			"B":        1,
		}},
		{{
			"Col1": 3,
			"Col2": "2021-01-03",
		}},
	}
	for i, content := range contents {
		exportJS := ExportConfigTable([]byte(content))
		expectJS, _ := json.Marshal(exportTables[i])
		if len(exportJS) != len(expectJS) {
			t.Errorf("export %s fail. export result: %s expect result %s", content, exportJS, expectJS)
		}
	}
}
