package randutil

import (
	"testing"
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
		{0, 8000, 1000, 1000, 0, 0, 0},
	}
	for _, data := range datalist {
		n := Index(data[1:])
		if n != data[0] {
			t.Error("fail index", data)
		}
	}
}
