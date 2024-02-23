package randutils

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

		nums := map[int]bool{}
		for _, n := range data {
			if nums[n] {
				t.Errorf("shuffle data %v error", data)
			}
			nums[n] = true
		}
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
		indexs := IndexN(data, -1)

		nums := map[int]bool{}
		for _, n := range indexs {
			if nums[n] || n < 0 || n >= len(data) {
				t.Errorf("index data %v result %d error", data, n)
			}
			nums[n] = true
		}
	}
}

func TestIndexFunc(t *testing.T) {
	var datalist [][][]int

	row1 := [][]int{
		{1, 1, 1},
		{2, 0, 3},
	}

	row2 := [][]int{
		{1, 1, 1},
		{2, 0, 3},
	}

	datalist = append(datalist, row1)
	datalist = append(datalist, row2)

	for _, data := range datalist {
		indexs := IndexFunc(data, -1, func(i int) int {
			size := len(data[i])
			return data[i][size-1]
		})

		nums := map[int]bool{}
		for _, n := range indexs {
			if nums[n] || n < 0 || n >= len(data) {
				t.Errorf("index data %v result %d error", data, n)
			}
			nums[n] = true
		}
	}
}
