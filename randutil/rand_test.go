package randutil

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestIndex(t *testing.T) {
	var datalist = [][]int{
		{0, 1},
		{0, 1, 0},
		{0, 10000, 1},
		{1, 0, 1, 0},
		{2, 0, 1, 100000},
		{2, 0, 0, 100000},
		{2, 0, 0, 1},
		{0, 8000, 1, 1, 0, 0, 0},
	}
	for _, data := range datalist {
		n := Index(data[1:])
		if n != data[0] {
			t.Error("fail index", data)
		}
	}
	var floats = [][]float64{
		{0, 1.0},
		{0, 1.2, 0},
		{0, 10000.001, 1},
		{1, 0, 1, 0},
		{2, 0, 1, 100000},
		{2, 0, 0, 100000.3},
		{2, 0, 0, 1.3},
		{0, 8000, 1.00, 1, 0, 0, 0},
	}
	for _, data := range floats {
		n := Index(data[1:])
		if n != int(data[0]) {
			t.Error("fail index", data)
		}
	}

}

func TestShuffeN(t *testing.T) {
	rand.Seed(time.Now().Unix())
	var datalist = [][]int{
		{},
		{0},
		{0, 1},
		{0, 1, 2},
		{0, 1, 2, 3},
		{0, 1, 2, 3, 4},
	}
	for _, data := range datalist {
		Shuffle(data)
		t.Log(data)
	}
}

func TestIndexN(t *testing.T) {
	var datalist = [][]int{
		{1000, 1, 1, 1},
		{1, 0, 3, 5},
		{10000, 1},
		{0, 1, 0},
		{0, 1, 100000},
		{0, 0, 100000},
		{0, 0, 1},
		{8000, 1, 1, 0, 0, 0},
	}
	for _, data := range datalist {
		res := IndexN(data, -1)
		t.Logf("IndexN: %v %v", data, res)
	}
}

func TestPseudoRand(t *testing.T) {
	for i := 0; i <= 100; i++ {
		bm := PseudoRand(float64(i), 33)
		zeroNum := bm.ZeroNum()
		per := float64(bm.Num-zeroNum) / float64(bm.Num) * 100
		if math.Abs(per-float64(i)) > 100.0/33 {
			t.Error("rand", i, per)
		}
	}
}
