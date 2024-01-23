// timer
// 2019-08-16 增加分组，分组定时器可全部关闭

package util

import (
	"container/heap"
	"time"

	"quasar/log"
)

type timerHeap []*Timer

func (h timerHeap) Len() int           { return len(h) }
func (h timerHeap) Less(i, j int) bool { return h[i].t.Before(h[j].t) }
func (h timerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].pos, h[j].pos = i, j
}

func (h *timerHeap) Push(x any) {
	timer := x.(*Timer)
	*h = append(*h, timer)
	timer.pos = len(*h) - 1
}

func (h *timerHeap) Pop() any {
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

// 批量处理到期的定时器
func (tm *timerSet) RunOnce() {
	now := time.Now()
	for i := 0; tm.h.Len() > 0; i++ {
		top := tm.h[0]
		// 处理当前的定时器任务时，新创建的任务放到下一个周期再处理
		if now.Before(top.t) {
			break
		}
		if i > 4096 {
			log.Warnf("too much timer, handle %d left %d", i, tm.h.Len())
		}
		// 分组已失效
		if top.group != nil && top.group.isClose {
			top.f = nil
			top.period = 0
		}

		f := top.f
		if period := top.period; period > 0 {
			top.repeat++
			period = SkipPeriodTime(top.startTime, top.period).Sub(now)
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

func (tm *timerSet) NewPeriodTimer(f func(), start time.Time, period time.Duration) *Timer {
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

func (g *TimerGroup) NewTimer(f func(), d time.Duration) *Timer {
	set := g.set
	// 默认全局定时器集
	if set == nil {
		set = defaultTimerSet
	}

	t := set.NewTimer(f, d)
	t.group = g
	return t
}

func (g *TimerGroup) NewPeriodTimer(f func(), start time.Time, period time.Duration) *Timer {
	set := g.set
	// 默认全局定时器集
	if set == nil {
		set = defaultTimerSet
	}

	t := set.NewPeriodTimer(f, start, period)
	t.group = g
	return t
}

func (g *TimerGroup) ResetTimer(t **Timer, f func(), d time.Duration) {
	StopTimer(*t)
	*t = g.NewTimer(f, d)
}

func (g *TimerGroup) StopAllTimer() {
	g.isClose = true
}

var defaultTimerSet = NewTimerSet()

func GetTimerSet() *timerSet {
	return defaultTimerSet
}

// 关闭定时器
func StopTimer(t *Timer) {
	GetTimerSet().StopTimer(t)
}

// 重制定时器过期时间
func ResetTimer(t *Timer, d time.Duration) {
	GetTimerSet().ResetTimer(t, d)
}

// 创建仅执行一次的定时器
func NewTimer(f func(), d time.Duration) *Timer {
	return GetTimerSet().NewTimer(f, d)
}

// 创建重复执行的定时器
func NewPeriodTimer(f func(), start time.Time, period time.Duration) *Timer {
	return GetTimerSet().NewPeriodTimer(f, start, period)
}
