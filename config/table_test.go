package config

import (
	"testing"
)

func TestMain(m *testing.M) {
	LoadLocalTables("test_tables")
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
		t.Errorf("scan config test1 row key:1 fail")
	}
	argn, _ = Scan("test1", 3, "A,B,C,D", &a, &b, &c, &d)
	if !(argn == 4 && a == 30 && b == "HelloB3" && !c && d == 30.1) {
		t.Errorf("scan config test1 row key:3 fail")
	}

	argn, _ = Scan("test1", 1, "PA,PB", &a, &b)
	if !(argn == 2 && a == 11 && b == "[10,11,12]") {
		t.Errorf("scan config test1 private data fail")
	}

	argn, _ = Scan("test1", RowId(0), "A,B,C,D", &a, &b, &c, &d)
	if !(argn == 4 && a == 10 && b == "HelloB1" && c && d == 10.1) {
		t.Errorf("scan config test1 RowId(0) fail")
	}
}

func TestFilterRows(t *testing.T) {
	rows1 := FilterRows("test2", "C1,C4", 11, "S1")
	if !(len(rows1) == 2 && rows1[0].n == 0 && rows1[1].n == 7) {
		t.Errorf("filter test2 11,S1 fail")
	}

	rows2 := FilterRows("test2", "C1,C4", 12, "S1")
	if len(rows2) != 0 {
		t.Errorf("filter test2 12,S1 fail")
	}
}

func TestScanTableGroup(t *testing.T) {
	a, ok := Int("test", 1, "A")
	if !(ok && a == 10) {
		t.Errorf("scan table group:test valid field fail")
	}
	a, ok = Int("test", 100, "A")
	if !(!ok && a == 0) {
		t.Errorf("scan table group:test empty field fail")
	}
}
