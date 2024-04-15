package kvsrv

import(
	"sync"
)
type RequestLimiter struct {
    activeRequests int
    cond           *sync.Cond
}

func NewRequestLimiter() *RequestLimiter {
    return &RequestLimiter{
		activeRequests: 0,
        cond: sync.NewCond(&sync.Mutex{}),
    }
}

func (rl *RequestLimiter) TryIn() {
    rl.cond.L.Lock()
    for rl.activeRequests >= 2 {
        rl.cond.Wait()
    }

    rl.activeRequests++

    rl.cond.L.Unlock()
}

func (rl *RequestLimiter) Done() {
    rl.cond.L.Lock()

    rl.activeRequests--

    rl.cond.L.Unlock()

    rl.cond.Signal()
}