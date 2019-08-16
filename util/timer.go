// timer
// 2019-08-16 增加分组，分组定时器可全部关闭

package util

import (
	"container/heap"
	// "github.com/guogeer/husky/log"
	"time"
)

type timerHeap []*Timer

func (h timerHeap) Len() int           { return len(h) }
func (h timerHeap) Less(i, j int) bool { return h[i].t.Before(h[j].t) }
func (h timerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].pos, h[j].pos = i, j
}

func (h *timerHeap) Push(x interface{}) {
	timer := x.(*Timer)
	*h = append(*h, timer)
	timer.pos = len(*h) - 1
}

func (h *timerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	timer := old[n-1]
	*h = old[:n-1]

	timer.f = nil
	timer.pos = -1
	timer.period = 0 // 清理周期
	timer.repeat = 0
	timer.group = nil
	return timer
}

type Timer struct {
	f         func()
	t         time.Time
	pos       int
	startTime time.Time
	period    time.Duration
	repeat    int
	group     *TimerGroup
}

func (timer *Timer) Expire() time.Time {
	return timer.t
}

func (timer *Timer) IsValid() bool {
	return timer != nil && timer.pos >= 0
}

type timerSet struct {
	h timerHeap
}

func NewTimerSet() *timerSet {
	tm := new(timerSet)
	heap.Init(&tm.h)
	return tm
}

func (tm *timerSet) RunOnce() {
	now := time.Now()
	for i := 0; i < 64 && tm.h.Len() > 0; i++ {
		top := tm.h[0]
		if now.Before(top.t) {
			break
		}
		// 分组已失效
		if top.group != nil && top.group.isClose {
			top.f = nil
		}

		f := top.f
		if period := top.period; period > 0 {
			top.repeat++
			// 纠正误差
			if top.repeat%1000 == 0 {
				period = SkipPeriodTime(top.startTime, top.period).Sub(now)
			}
			tm.ResetTimer(top, period)
		} else {
			tm.StopTimer(top)
		}

		// call
		if f != nil {
			f()
		}
	}
}

func (tm *timerSet) StopTimer(timer *Timer) {
	if timer == nil {
		return
	}
	pos := timer.pos
	if pos < 0 || pos >= tm.h.Len() {
		return
	}
	if tm.h[pos] != timer {
		return
	}
	heap.Remove(&tm.h, timer.pos)
}

func (tm *timerSet) ResetTimer(timer *Timer, d time.Duration) {
	if timer == nil {
		return
	}

	pos := timer.pos
	if pos < 0 || pos >= tm.h.Len() {
		return
	}
	if tm.h[pos] != timer {
		return
	}

	timer.t = time.Now().Add(d)
	heap.Fix(&tm.h, timer.pos)
}

func (tm *timerSet) NewTimer(f func(), d time.Duration) *Timer {
	timer := &Timer{
		f: f,
		t: time.Now().Add(d),
	}
	heap.Push(&tm.h, timer)
	return timer
}

func (tm *timerSet) NewPeriodTimer(f func(), startTime string, period time.Duration) *Timer {
	start, err := ParseTime(startTime)
	if err != nil {
		panic(err)
	}

	timer := &Timer{
		f:         f,
		t:         SkipPeriodTime(start, period),
		startTime: start,
		period:    period,
	}
	heap.Push(&tm.h, timer)
	return timer
}

type TimerGroup struct {
	isClose bool
	set     *timerSet
}

func NewTimerGroup(set ...*timerSet) *TimerGroup {
	g := &TimerGroup{}
	for _, v := range set {
		g.set = v
	}
	if g.set == nil {
		g.set = GetTimerSet()
	}
	return g
}

func (g *TimerGroup) NewTimer(f func(), d time.Duration) *Timer {
	t := g.set.NewTimer(f, d)
	t.group = g
	return t
}

func (g *TimerGroup) StopAllTimer() {
	g.isClose = true
}

var defaultTimerSet = NewTimerSet()

func GetTimerSet() *timerSet {
	return defaultTimerSet
}

func StopTimer(t *Timer) {
	GetTimerSet().StopTimer(t)
}

func ResetTimer(t *Timer, d time.Duration) {
	GetTimerSet().ResetTimer(t, d)
}

func NewTimer(f func(), d time.Duration) *Timer {
	return GetTimerSet().NewTimer(f, d)
}

func NewPeriodTimer(f func(), startTime string, period time.Duration) *Timer {
	return GetTimerSet().NewPeriodTimer(f, startTime, period)
}
