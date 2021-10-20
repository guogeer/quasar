package util

import (
	"testing"
)

type AA struct {
	N1  int64
	N2  int32
	N21 *int
	S1  string
	F1  float32
	B1  bool
	B2  bool
	M1  map[string]string
	M2  map[string]string
	A1  []int
}

type AB struct {
	N1  int32
	N2  int64
	N21 *int
	S1  string
	F1  float64
	B1  bool
	M1  map[string]string
	M2  map[string]string
	N3  int
	A1  []int
}

type A struct {
	N1  int64
	N2  int32
	N21 *int
	S1  string
	F1  float32
	B1  bool
	M3  map[string]string
	M4  map[string]string
	AA1 AA
	AA2 *AA
	AA3 *AA
	AA4 [2]AA
	AA5 []*AA
	AA6 []int
}

func TestSructCopy(t *testing.T) {
	aa3 := AA{N1: 31, N2: 32, N21: new(int), S1: "AAS3", B1: true}
	*aa3.N21 = 210
	aa4 := AA{N1: 41, N2: 42, S1: "AAS4", B1: false}
	a := &A{
		N1: 1, N2: 2, N21: new(int),
		S1:  "S1",
		AA1: AA{N1: 11, N2: 12, S1: "AAS1", B1: true},
		AA3: &AA{N1: 21, N2: 22, S1: "AAS2", B1: false},
		AA4: [2]AA{aa3, aa4},
		AA5: []*AA{&aa3, &aa4},
		AA6: []int{1, 2, 3},
	}
	*a.N21 = 123321
	DeepCopy(nil, a)
	DeepCopy(nil, nil)

	a2 := &A{}
	DeepCopy(a2, a)
	// t.Log("deep copy test", *a2.AA4[0].N21)
	if !EqualJSON(a2, a) {
		t.Error("deep copy error", a2, a)
	}
}

func BenchmarkCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		aa3 := AA{N1: 31, N2: 32, N21: new(int), S1: "AAS3", B1: true}
		*aa3.N21 = 210
		aa4 := AA{N1: 41, N2: 42, S1: "AAS4", B1: false}
		a := &A{
			N1: 1, N2: 2, N21: new(int),
			S1:  "S1",
			AA1: AA{N1: 11, N2: 12, S1: "AAS1", B1: true},
			AA3: &AA{N1: 21, N2: 22, S1: "AAS2", B1: false},
			AA4: [2]AA{aa3, aa4},
			AA5: []*AA{&aa3, &aa4},
			AA6: []int{1, 2, 3},
		}
		*a.N21 = 123321
		DeepCopy(nil, a)
		DeepCopy(nil, nil)

		a2 := &A{}
		DeepCopy(a2, a)
		// t.Log("deep copy test", *a2.AA4[0].N21)
		if !EqualJSON(a2, a) {
			b.Error("deep copy error", a2, a)
		}
	}
}

type Child struct {
	N int
	S string
}

type GrandPa struct {
	GrandAliasId int `alias:"TestGrandAliasId"`
}

type Father struct {
	GrandPa
	Id      int `alias:"UId" json:"UId"`
	AliasId int `alias:"TestAliasId"`
	UId2    int
	N2      int
	S2      string
	Child
}

type Father2 struct {
	GrandPa
	UId     int
	AliasId int `alias:"TestAliasId"`
	Id2     int `alias:"UId2" json:"UId2"`
	N2      int
	S2      string
	N       int
	S       string
}

func TestInheritSructCopy(t *testing.T) {
	f := &Father{S2: "sb2"}
	f2 := &Father2{
		N2: 22, S2: "", N: 11, S: "sb", UId: 100, Id2: 200, AliasId: 300,
		GrandPa: GrandPa{GrandAliasId: 400},
	}
	DeepCopy(f, f2)
	if EqualJSON(f, f2) == false {
		t.Error("deep copy inherit", f, f2)
	}

	f2 = &Father2{}
	f = &Father{
		N2: 22, S2: "sb2", Id: 100, UId2: 200,
		Child: Child{N: 100, S: "sb"},
	}
	DeepCopy(f2, f)
	if EqualJSON(f, f2) == false {
		t.Error("deep copy inherit", f, f2)
	}
}

type TestArray struct {
	A [3]int
	B []int `json:"-"`
}

type TestSlice struct {
	A []int
	B int `json:"-"`
}

func TestArraySliceCopy(t *testing.T) {
	fromS := &TestSlice{A: []int{1, 2, 3}}
	toA := &TestArray{}

	DeepCopy(toA, fromS)
	if !EqualJSON(fromS, toA) {
		t.Error("deep copy slice to array error", fromS, toA)
	}

	fromA := &TestArray{A: [3]int{1, 2, 3}, B: []int{100}}
	toS := &TestSlice{A: []int{0, 0, 0}}
	DeepCopy(toA, fromS)
	if !EqualJSON(fromS, toA) {
		t.Error("deep copy array to slice error", fromA, toS)
	}

	DeepCopy(fromA, toS)
}
