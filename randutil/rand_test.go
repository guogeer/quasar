package randutil

import (
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
