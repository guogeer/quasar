package util

import (
	"testing"
)

type A struct {
	X int
	Y int
	S string
}

type B struct {
	Y int64
	S string
	B string
}

var a = A{X: 1, Y: 2, S: "zzz"}
var b = B{}

func TestSructCopy(t *testing.T) {
	t.Log(a, b)
	DeepCopy(&b, &a)
	t.Log(a, b)
}
