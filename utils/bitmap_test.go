package utils

import (
	"math/rand"
	"testing"
	"time"
)

func TestSetBitmap(t *testing.T) {
	bm := NewBitmap(9)
	bm.Set(8, 1)
	bm.Set(7, 1)
	bm.Set(1, 1)
	if len(bm.Bits) != 2 {
		t.Error("invalid bit map length", bm.Bits)
	}
	if bm.Bits[0] != 130 {
		t.Error("invalid bit 0", bm.Bits)
	}
	if bm.Bits[1] != 1 {
		t.Error("invalid bit 1", bm.Bits)
	}
	rand.Seed(time.Now().Unix())

	k := bm.Rand()
	t.Log(k)
	if k == 1 || k == 7 || k == 8 {
		t.Error("invalid rand", bm.Bits)
	}
}

func TestBitmapMarshalJSON(t *testing.T) {
	bm := NewBitmap(9)
	bm.Set(8, 1)
	bm.Set(2, 1)
	fake := fakeBitmap{Num: 9, Bits: []uint32{4, 1}}
	if !EqualJSON(bm, fake) {
		t.Error("bit map marshal json")
	}
}
