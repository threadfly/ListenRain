package listenrain

import (
	"time"
)

const (
	DEFAULT_QUEUE_CAP = 1 << 7  // 128
	MAX_QUEUE_CAP     = 1 << 12 // 4k
)

type DefaultQueue struct {
	q            chan []byte
	ticker       *time.Ticker
	lastPopCount int
}

func NewDefaultQueue(cap int) *DefaultQueue {
	if cap > MAX_QUEUE_CAP {
		cap = DEFAULT_QUEUE_CAP
	}

	dq := &DefaultQueue{
		q: make(chan []byte, cap),
	}

	return dq
}

func NewDefaultQueueV2() *DefaultQueue {
	return NewDefaultQueue(DEFAULT_QUEUE_CAP)
}

func (q *DefaultQueue) Push(payload []byte) {
	q.q <- payload
}

func (q *DefaultQueue) Pop() (payload []byte) {
	return <-q.q
}

// What's the problem with implementation?
func (q *DefaultQueue) PopNoBlocking() (payload []byte) {
	if q.ticker == nil {
		q.ticker = time.NewTicker(50 * time.Millisecond)
	}

	stop := func() bool {
		if q.lastPopCount >= len(q.q) {
			q.lastPopCount = 0
			q.ticker.Stop()
			q.ticker = nil
			return true
		}
		return false
	}

	if stop() {
		return nil
	}

	for {
		q.lastPopCount++
		select {
		case p := <-q.q:
			return p
		case <-q.ticker.C:
		}

		if stop() {
			return nil
		}
	}
}

func (q *DefaultQueue) Drop() {
	close(q.q)
}

func DefaultQueueGenerator(key TransportKey) (Queue, error) {
	return NewDefaultQueueV2(), nil
}
