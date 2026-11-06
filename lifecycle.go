package main

import (
	"sync"
	"time"
)

// lifecycle tracks in-flight work for the idle auto-shutdown (-timeout).
//
// Semantics: the server shuts down once it has been idle (zero active
// requests/connections) for the full timeout. A transfer that is still running
// when the timer fires is never cut off — the timer is simply re-armed.
//
// All state is guarded by mu so that "check stopping + register work" is atomic
// with respect to "decide to stop", which removes the stop-flag / WaitGroup races.
type lifecycle struct {
	mu       sync.Mutex
	stopping bool
	active   int
	timeout  time.Duration
	timer    *time.Timer
	done     chan struct{}
	once     sync.Once
}

func newLifecycle() *lifecycle {
	return &lifecycle{done: make(chan struct{})}
}

// startIdleTimer arms the idle auto-shutdown. Safe to call once.
func (l *lifecycle) startIdleTimer(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeout = d
	l.timer = time.AfterFunc(d, l.onTimer)
}

func (l *lifecycle) onTimer() {
	l.mu.Lock()
	if l.stopping {
		l.mu.Unlock()
		return
	}
	if l.active > 0 {
		// Still busy: check again after another full timeout.
		l.timer.Reset(l.timeout)
		l.mu.Unlock()
		return
	}
	l.stopping = true
	l.mu.Unlock()
	l.shutdown()
}

// begin registers one unit of work. It returns false when the server is
// shutting down; the caller must then reject the work and must not call end.
func (l *lifecycle) begin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopping {
		return false
	}
	l.active++
	l.touchLocked()
	return true
}

// end marks one unit of work finished and restarts the idle clock.
func (l *lifecycle) end() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active > 0 {
		l.active--
	}
	l.touchLocked()
}

func (l *lifecycle) touchLocked() {
	if l.timer != nil && !l.stopping {
		l.timer.Reset(l.timeout)
	}
}

func (l *lifecycle) isStopping() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopping
}

func (l *lifecycle) activeCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active
}

// shutdown closes done exactly once; servers wait on Done() and then
// shut themselves down gracefully before main returns.
func (l *lifecycle) shutdown() {
	l.once.Do(func() { close(l.done) })
}

func (l *lifecycle) Done() <-chan struct{} { return l.done }
