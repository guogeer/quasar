package utils

import (
	"testing"
)

func TestForbidWords(t *testing.T) {
	samples := [][]string{
		{"张麻子 CNM你好Fuck", "张** ***你好****", "CNM", "Fuck", "麻子", "麻花"},
		{"张麻子 CNM你好Fuck", "*** ***你好****", "CNM", "Fuck", "张麻子", "张麻"},
		{"张麻子 CNM你好Fuck", "**子 ***你好****", "CNM", "Fuck", "张麻花", "张麻"},
	}
	for i, sample := range samples {
		LoadForbidWords(sample[2:])
		res := ForbidWords(sample[0])
		if res != sample[1] {
			t.Errorf("sample:%d %v result: %s", i, sample, res)
		}
	}
}
