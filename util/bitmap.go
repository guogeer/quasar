package util

import (
	"encoding/json"
	"math/rand"
)

type Bitmap struct {
	Num  int
	Bits []uint8 // 低位在前
}

type fakeBitmap struct {
	Num  int
	Bits []uint32
}

func NewBitmap(n int) *Bitmap {
	return &Bitmap{
		Num:  n,
		Bits: make([]uint8, (n+7)/8),
	}
}

func (bm Bitmap) Set(k, n int) {
	if n&1 != n {
		panic("must 0 or 1")
	}
	x, y := uint(k/8), uint(k%8)
	if k < bm.Num {
		bm.Bits[x] |= uint8(1 << y)
		bm.Bits[x] |= uint8(uint(n) << y)
	}
}

func (bm Bitmap) Rand() int {
	a := make([]int, 0, 8)
	for k, b := range bm.Bits {
		for i := 0; i < 8; i++ {
			if 1<<uint(i)&b == 0 && k*8+i < bm.Num {
				a = append(a, k*8+i)
			}
		}
	}
	if len(a) > 0 {
		return a[rand.Intn(len(a))]
	}
	return -1
}

func (bm *Bitmap) ZeroNum() int {
	num := 0
	for k := 0; k < bm.Num; k++ {
		x, y := k/8, uint(k%8)
		if bm.Bits[x]&(1<<y) == 0 {
			num++
		}
	}
	return num
}

func (bm *Bitmap) MarshalJSON() ([]byte, error) {
	fake := fakeBitmap{
		Num: bm.Num,
	}
	for _, u8 := range bm.Bits {
		fake.Bits = append(fake.Bits, uint32(u8))
	}
	return json.Marshal(fake)
}

func (bm *Bitmap) UnmarshalJSON(buf []byte) error {
	fake := fakeBitmap{}
	err := json.Unmarshal(buf, &fake)
	bm.Num, bm.Bits = fake.Num, nil
	for _, u32 := range bm.Bits {
		bm.Bits = append(bm.Bits, uint8(u32))
	}
	return err
}
