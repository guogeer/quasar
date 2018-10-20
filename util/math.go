package util

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
