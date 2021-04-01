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

// 随机打乱数组/切片
func Shuffle(a interface{}) {
	lst := reflect.ValueOf(a)
	ShuffleN(a, lst.Len())
}

// 随机打乱切片slice前n个元素
func ShuffleN(a interface{}, n int) {
	if a == nil {
		return
	}

	swap := reflect.Swapper(a)
	size := reflect.ValueOf(a).Len()
	for i := 0; i+1 < size && i < n; i++ {
		r := rand.Intn(size-i) + i
		// 使用reflect.Swapper进行切片元素交换，效率更高
		swap(i, r)
	}
}

func IndexN(a interface{}, num int) []int {
	var numbers []int64
	var vals = reflect.ValueOf(a)

	if num == -1 {
		num = vals.Len()
	}
	for i := 0; i < vals.Len(); i++ {
		s := fmt.Sprintf("%v", vals.Index(i))
		f, _ := strconv.ParseFloat(s, 64)
		numbers = append(numbers, int64(f*sampleSize))
	}

	res := make([]int, 0, 4)
	for try := 0; try < num; try++ {
		var part, sum int64
		for _, n := range numbers {
			sum += n
		}

		if sum > 0 {
			r := rand.Int63n(sum)
			for i, n := range numbers {
				part += n
				if r < part {
					res = append(res, i)
					numbers[i] = 0
					break
				}
			}
		}
	}
	return res
}

// 根据a[i]比重随机下标i
func Index(a interface{}) int {
	res := IndexN(a, 1)
	for _, v := range res {
		return v
	}
	return -1
}
