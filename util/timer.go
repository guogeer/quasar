// timer

package util

import (
	"container/heap"
	"github.com/guogeer/husky/log"
	"time"
)

type TimerHeap []*Timer

func (h TimerHeap) Len() int           { return len(h) }
func (h TimerHeap) Less(i, j int) bool { return h[i].t.Before(h[j].t) }
func (h TimerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].pos, h[j].pos = i, j
}

func (h *TimerHeap) Push(x interface{}) {
	timer := x.(*Timer)
	*h = append(*h, timer)
	timer.pos = len(*h) - 1
}

func (h *TimerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	timer := old[n-1]
	*h = old[:n-1]

	timer.f = nil
	timer.pos = -1
	timer.period = 0 // 清理周期
	timer.repeat = 0
	return timer
}

type Timer struct {
	f         func()
	t         time.Time
	pos       int
	startTime time.Time
	period    time.Duration
	repeat    int
}

func (timer *Timer) Expire() time.Time {
	return timer.t
}

func (timer *Timer) IsValid() bool {
	return timer != nil && timer.pos >= 0
}

type timerManage struct {
	h TimerHeap
}

func NewTimerManage() *timerManage {
	tm := new(timerManage)
	heap.Init(&tm.h)
	return tm
}

var defaultTimerManage = NewTimerManage()

func GetTimerManage() *timerManage {
	return defaultTimerManage
}

func (tm *timerManage) Run() {
	now := time.Now()
	for i := 0; i < 64 && tm.h.Len() > 0; i++ {
		top := tm.h[0]
		if now.Before(top.t) {
			break
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

func (tm *timerManage) StopTimer(timer *Timer) {
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

func (tm *timerManage) ResetTimer(timer *Timer, d time.Duration) {
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

func (tm *timerManage) NewTimer(f func(), d time.Duration) *Timer {
	timer := &Timer{
		f: f,
		t: time.Now().Add(d),
	}
	heap.Push(&tm.h, timer)
	return timer
}

func (tm *timerManage) NewPeriodTimer(f func(), startTimeString string, period time.Duration) *Timer {
	startTime, err := ParseTime(startTimeString)
	if err != nil {
		log.Errorf("new period timer %v", err)
		return nil
	}

	timer := &Timer{
		f:         f,
		t:         SkipPeriodTime(startTime, period),
		startTime: startTime,
		period:    period,
	}
	heap.Push(&tm.h, timer)
	return timer
}

func StopTimer(t *Timer) {
	GetTimerManage().StopTimer(t)
}

func ResetTimer(t *Timer, d time.Duration) {
	GetTimerManage().ResetTimer(t, d)
}

func NewTimer(f func(), d time.Duration) *Timer {
	return GetTimerManage().NewTimer(f, d)
}

func NewPeriodTimer(f func(), startTimeString string, period time.Duration) *Timer {
	return GetTimerManage().NewPeriodTimer(f, startTimeString, period)
}

func TickTimerRun() {
	GetTimerManage().Run()
}
