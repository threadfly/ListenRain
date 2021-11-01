package listenrain

import (
	"sync"
	"time"
)

const (
	TENT_QSIZE = 16
)

var (
	timeEntPool *sync.Pool
)

func init() {
	timeEntPool = &sync.Pool{
		New: func() interface{} {
			return &tentry{}
		},
	}
}

type tentry struct {
	msgId   string
	timeout time.Time
}

type timer struct {
	c map[int]*tentry
}

func (t *timer) Top() interface{} {
	if len(t.c) == 0 {
		return nil
	}

	return t.c[0]
}

func (t *timer) Push(x interface{}) {
	idx := len(t.c)
	t.c[idx] = x.(*tentry)
}

func (t *timer) Pop() interface{} {
	e := t.c[len(t.c)-1]
	if e == nil {
		return nil
	}
	delete(t.c, len(t.c)-1)
	return e
}

func (t *timer) Len() int {
	return len(t.c)
}

func (t *timer) Less(i, j int) bool {
	return t.c[i].timeout.Before(t.c[j].timeout)
}

func (t *timer) Swap(i, j int) {
	if i < 0 || j < 0 {
		return
	}
	t.c[i], t.c[j] = t.c[j], t.c[i]
}

func NewTimer() *timer {
	return &timer{
		c: make(map[int]*tentry),
	}
}
