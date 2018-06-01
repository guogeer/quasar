package util

import (
	"math/rand"
)

type random struct{ sampleSize int }

var defaultRandom = &random{sampleSize: 100 * 10000} // 100W

func Random() *random {
	return defaultRandom
}

// 随机
func (obj *random) Array(array interface{}) int {
	t := obj.Int()

	switch array.(type) {
	case []int:
		for i, n := range array.([]int) {
			if t < n {
				return i
			}
		}
	case []int64:
		for i, n := range array.([]int64) {
			if t < int(n) {
				return i
			}
		}
	}
	panic("random in empty array")
}

func (obj *random) Range() int {
	return obj.sampleSize
}

func (obj *random) Int() int {
	return rand.Intn(obj.Range())
}

func (obj *random) IsNice(n int) bool {
	return obj.Int() < n
}

func (obj *random) IsPercentNice(percent float64) bool {
	n := obj.FromPercent(percent)
	return obj.IsNice(n)
}

func (obj *random) FromPercent(percent float64) int {
	return int(float64(obj.sampleSize/100) * percent)
}

func (obj *random) ToPercent(n int) float64 {
	return float64(n) / float64(obj.sampleSize/100)
}

func (obj *random) Shuffle(a []int) {
	size := len(a)
	for i := 0; i+1 < size; i++ {
		r := rand.Intn(size-i) + i
		a[i], a[r] = a[r], a[i]
	}
}

// 组合
func Combine(a []int, n int) [][]int {
	var one []int
	var result [][]int
	var recurse func(int)

	recurse = func(k int) {
		if k+n > len(one)+len(a) {
			return
		}
		if len(one) >= n {
			temp := make([]int, len(one))
			copy(temp, one)
			result = append(result, temp)
			return
		}

		one = append(one, a[k])
		recurse(k + 1)
		one = one[:len(one)-1]

		recurse(k + 1)
	}
	recurse(0)
	return result
}
