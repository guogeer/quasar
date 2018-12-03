package randutil

import (
	"math/rand"
	"reflect"
)

const sampleSize = 100 * 10000 // 100W

// 随机
func Array(array interface{}) int {
	t := rand.Intn(sampleSize)

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

func IsNice(n int) bool {
	return rand.Intn(sampleSize) < n
}

func IsPercentNice(percent float64) bool {
	n := FromPercent(percent)
	return IsNice(n)
}

func FromPercent(percent float64) int {
	return int(float64(sampleSize/100) * percent)
}

func ToPercent(n int) float64 {
	return float64(n) / float64(sampleSize/100)
}

func Shuffle(a interface{}) {
	lst := reflect.ValueOf(a)
	ShuffleN(a, lst.Len())
}

func ShuffleN(a interface{}, n int) {
	lst := reflect.ValueOf(a)
	size := lst.Len()
	for i := 0; i+1 < size && i < n; i++ {
		r := rand.Intn(size-i) + i
		temp := lst.Index(r)
		temp = reflect.New(temp.Type()).Elem()
		temp.Set(lst.Index(i))
		lst.Index(i).Set(lst.Index(r))
		lst.Index(r).Set(temp)
	}
}

// 根据a[i]比重随机下标i
func Index(a []int) int {
	var part, sum int
	for _, n := range a {
		sum += n
	}
	if sum <= 0 {
		return -1
	}
	r := rand.Intn(sum)

	for i, n := range a {
		part += n
		if r < part {
			return i
		}
	}
	panic("invalid rand array")
}
