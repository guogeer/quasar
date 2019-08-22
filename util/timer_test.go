package util

import (
	"fmt"
	"testing"
	"time"
)

func TestTimerGroup(t *testing.T) {
	isNice := true
	g1 := TimerGroup{}
	g2 := TimerGroup{}
	for i := 0; i < 100; i++ {
		t1, t2 := i, i
		g1.NewTimer(func() { fmt.Printf("g1 %d\n", t1) }, time.Duration(i)*time.Second)
		g2.NewTimer(func() { fmt.Printf("g2 %d\n", t2) }, time.Duration(i)*time.Second)
	}
	g1.NewTimer(func() { g1.StopAllTimer() }, 10*time.Second)
	g2.NewTimer(func() { g1.StopAllTimer() }, 20*time.Second)
	NewTimer(func() { isNice = false }, 30*time.Second)
	NewPeriodTimer(func() { fmt.Println("period 1") }, "2018-01-01 00:00:00", 5*time.Second)
	NewPeriodTimer(func() { fmt.Println("period 2") }, "2020-01-01 00:00:00", 5*time.Second)
	for isNice {
		GetTimerSet().RunOnce()
		time.Sleep(100 * time.Millisecond)
	}
}
