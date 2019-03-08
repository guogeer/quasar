package randutil

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
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
func Index(a interface{}) int {
	var numbers []int64
	var vals = reflect.ValueOf(a)
	for i := 0; i < vals.Len(); i++ {
		s := fmt.Sprintf("%v", vals.Index(i))
		f, _ := strconv.ParseFloat(s, 64)
		numbers = append(numbers, int64(f*sampleSize))
	}

	var part, sum int64
	for _, n := range numbers {
		sum += n
	}

	if sum <= 0 {
		return -1
	}

	r := rand.Int63n(sum)
	for i, n := range numbers {
		part += n
		if r < part {
			return i
		}
	}
	panic("invalid rand array")
}
