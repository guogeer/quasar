package util

import (
	"bytes"
	"encoding/json"
	"testing"
)

type AA struct {
	N1 int64
	N2 int32
	S1 string
	F1 float32
	B1 bool
	B2 bool
	M1 map[string]string
	M2 map[string]string
}

type AB struct {
	N1 int32
	N2 int64
	S1 string
	F1 float64
	B1 bool
	M1 map[string]string
	M2 map[string]string
}

type A struct {
	N1  int64
	N2  int32
	S1  string
	F1  float32
	B1  bool
	M3  map[string]string
	M4  map[string]string
	AA1 AA
	AA3 AA
}

type B struct {
	N1  int64
	N3  int64
	S1  string
	F1  float32
	F2  float64
	M4  map[string]string
	AA1 *AB
	AA2 AB
	AA3 AA
}

func TestSructCopy(t *testing.T) {
	a := &A{
		N1: 1, N2: 2,
		S1:  "S1",
		AA1: AA{N1: 11, N2: 12, S1: "AAS1", B1: true},
		AA3: AA{N1: 21, N2: 22, S1: "AAS2", B1: false},
	}
	b1 := &B{}
	b2 := &B{AA1: &AB{}}
	DeepCopy(b1, a)
	s1, _ := json.Marshal(b1)
	s2, _ := json.Marshal(a)
	json.Unmarshal(s2, b2)
	s2, _ = json.Marshal(b2)
	if bytes.Compare(s1, s2) != 0 {
		t.Error("deep copy error", b1, b2)
	}
}
