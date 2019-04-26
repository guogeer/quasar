package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	LoadLocalTables(".")

	type struct1 struct {
		A1    int64
		A1ok  bool
		C2    int64
		P11   int64
		PS1   string
		P12   int64
		P12ok bool
	}
	var st1 struct1
	var st2 = struct1{
		A1: 1, C2: 12, P11: 1, PS1: "S", P12: 0,
		A1ok: true,
	}
	st1.A1, st1.A1ok = Int("test1", 1, "A")
	st1.C2, _ = Int("test1", 2, "C")
	st1.P11, _ = Int("test1", 1, "P1")
	st1.PS1, _ = String("test1", 1, "PS")
	st1.P12, st1.P12ok = Int("test1", 2, "P1")
	b1, _ := json.Marshal(st1)
	b2, _ := json.Marshal(st2)
	if string(b1) != string(b2) {
		t.Error("Load error")
	}
}

func TestScan(t *testing.T) {
	type dataset struct {
		N8  int8
		N16 int16
		N32 int32
		N64 int64
		N   int
		U8  uint8
		U16 uint16
		U32 uint32
		U64 uint64
		U   uint
		S   string
		SS  string
		NN  []int
		NN8 []int8
		F32 float32
		F64 float64
		FF  []float32
	}
	data1 := dataset{}
	data2 := dataset{
		N16: 16,
		N64: 64,
		N:   32,
		U16: 16,
		U64: 64,
		U:   32,
		S:   "s",
		SS:  "ss",
		NN:  []int{1, 2, 3},
		NN8: []int8{1, 2, 3},
		F32: 32,
		F64: 64,
		FF:  []float32{1.1, 2.2},
	}
	scanOne(reflect.ValueOf(&data1.N16), "16")
	scanOne(reflect.ValueOf(&data1.N64), "64")
	scanOne(reflect.ValueOf(&data1.N), "32")
	scanOne(reflect.ValueOf(&data1.U16), "16")
	scanOne(reflect.ValueOf(&data1.U64), "64")
	scanOne(reflect.ValueOf(&data1.U), "32")
	scanOne(reflect.ValueOf(&data1.S), "s")
	scanOne(reflect.ValueOf(&data1.SS), "ss")
	scanOne(reflect.ValueOf(&data1.NN), "1,2,3")
	scanOne(reflect.ValueOf(&data1.NN8), "1,2,3")
	scanOne(reflect.ValueOf(&data1.F32), "32")
	scanOne(reflect.ValueOf(&data1.F64), "64")
	scanOne(reflect.ValueOf(&data1.FF), "1.1,2.2")
	b1, _ := json.Marshal(data1)
	b2, _ := json.Marshal(data2)
	if string(b1) != string(b2) {
		t.Error("scan error")
	}
}
