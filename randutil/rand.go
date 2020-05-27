package randutil

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strconv"

	"github.com/guogeer/quasar/util"
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

// TODO 伪随机
func PseudoRand(percent float64, max int) *util.Bitmap {
	var x, y int
	var minMistake = 100.0
	for i := 2; i <= max; i++ {
		for j := 1; j <= i; j++ {
			mistake := math.Abs(percent - float64(j)/float64(i)*100)
			if minMistake > mistake {
				x, y = j, i
				minMistake = mistake
			}
		}
	}
	if y == 0 {
		y = 10
	}

	var samples []int
	for i := 0; i < x; i++ {
		samples = append(samples, i)
	}
	ShuffleN(samples, x)

	bm := util.NewBitmap(y)
	for i := 0; i < x; i++ {
		bm.Set(samples[i], 1)
	}
	return bm
}
