package listenrain

import (
	"container/heap"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	DEFAULT_TIMEOUT = 10 // sec
)

type Transport struct {
	q               Queue
	edP             EnDecPacket
	edM             EnDecMessage
	cg              ChannelGenerator
	wg              sync.WaitGroup
	close           bool
	err             error
	ch              Channel
	pt              *protocolType
	executor        Executor
	statmachinePool StatMachinePool
	t               *timer
	tq              chan string
	timeout         time.Duration
}

func NewTransport(transportKey TransportKey, pt *protocolType) (*Transport, error) {
	q, err := pt.QueueGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	cg, err := pt.ChannelGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	exe, err := pt.ExecutorGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	smp, err := pt.StatMachinePoolGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	ch, err := cg.Next()
	if err != nil {
		return nil, err
	}

	if !ch.IsActive() {
		cg.GC(ch)
		return nil, errors.New("channel is empty")
	}

	transport := &Transport{
		q:               q,
		edP:             pt.EdP,
		edM:             pt.EdM,
		cg:              cg,
		ch:              ch,
		pt:              pt,
		executor:        exe,
		statmachinePool: smp,
		t:               NewTimer(),
		tq:              make(chan string, 128),
	}

	transport.init()
	return transport, nil
}

func (t *Transport) init() {
	go t.runLoop()
	go t.startTimer()
}

func (t *Transport) runLoop() {
	var (
		sndPayload []byte
		closewg    = new(sync.WaitGroup)
	)
	closewg.Add(1)
	for {
		t.wg.Add(2)
		go func() {
			for {
				if sndPayload == nil {
					sndPayload = t.q.Pop()
				}
				err := t.edP.EncodePacket(t.ch, sndPayload)
				if err != nil {
					t.err = err
					// TODO
					// callback app layer
					break
				} else {
					sndPayload = nil
				}

				if t.close || t.err != nil {
					break
				}
			}

			if t.close {
				// process request that on send queue
				for sndPayload = t.q.PopNoBlocking(); sndPayload != nil; {
					err := t.edP.EncodePacket(t.ch, sndPayload)
					if err != nil {
						// callback app layer
						log.Printf("client transport encode packet to %s failed, %s", t.ch.PeerInfo(), err)
					}
				}
				closewg.Done()
			}
			t.wg.Done()
		}()

		go func() {
			for {
				rcvPayload, err := t.edP.DecodePacket(t.ch)
				if t.close || t.err != nil {
					break
				}

				if err != nil {
					// TODO
					log.Printf("client transport decode packet from %s failed, %s", t.ch.PeerInfo(), err)
					t.err = err
					break
				}

				t.executor.Process(t, rcvPayload)
			}

			if t.close {
				closewg.Wait()
				// just begin
				time.Sleep(t.timeout / 2)
				for {
					rcvPayload, err := t.edP.DecodePacket(t.ch)
					if err != nil {
						// callback app layer
						log.Printf("client transport decode packet from %s failed, %s", t.ch.PeerInfo(), err)
					}

					if rcvPayload == nil {
						break
					}

					t.executor.Process(t, rcvPayload)
				}
			}

			t.wg.Done()
		}()

		t.wg.Wait()
		if t.close {
			break
		}

		if t.err != nil && t.cg.IsTry(t.err) {
			exit := false
			ch, err := t.cg.Next()
			if err != nil {
				exit = true
			} else if !ch.IsActive() {
				exit = true
				err = errors.New("client channel is not active")
			}

			if exit {
				// 主动退出
				t.close = true
				t.err = err
				t.cg.GC(ch)
				t.cg.GC(t.ch)
				t.ch = nil
				break
			} else {
				t.cg.GC(t.ch)
				t.ch = ch
			}

			t.err = nil
			continue
		}

		break
	}
}

// TODO 从池中剔除
func (t *Transport) Close() error {
	if !t.close {
		t.close = true
		t.wg.Wait()
		t.cg.GC(t.ch)
	}

	return t.err
}

func (t *Transport) Error() error {
	return t.err
}

func (t *Transport) Send(sm StatMachine, key TransportKey, msg interface{}) error {
	if t.close {
		return fmt.Errorf("closed transport:%s", key.Key())
	}

	payload, msgId, err := t.edM.EncodeMessage(msg)
	if err != nil {
		return err
	}

	t.statmachinePool.Put(msgId, sm)
	t.q.Push(payload) // TODO how to deal with blocking?
	t.tq <- msgId
	return nil
}

func (t *Transport) startTimer() {
	tc := time.NewTicker(100 * time.Millisecond)
	var (
		index    int8 = -1
		q        [TENT_QSIZE]*tentry
		overflow bool = true
		now      time.Time
	)

	push := func(q []*tentry, index *int8, e *tentry) {
		*index = *index + 1
		q[*index] = e
	}

	fillq := func(timer time.Time) {
		for {
			v := t.t.Top()
			if v == nil {
				break
			}

			e := (v).(*tentry)
			if e.timeout.Before(timer) {
				heap.Pop(t.t)
				push(q[:], &index, e)
			} else {
				break
			}

			if index+1 >= TENT_QSIZE {
				overflow = true
				break
			}
		}
	}

	for {

		select {
		case now = <-tc.C:
			fillq(now)
			if index < 0 {
				continue
			}
		case msgId := <-t.tq:
			te := timeEntPool.Get().(*tentry)
			te.msgId = msgId
			te.timeout = time.Now().Add(t.pt.Timeout())
			heap.Push(t.t, te)
			continue
		}

		for {
			for i := int8(0); i <= index; i++ {
				t.Timeout(q[i].msgId)
				timeEntPool.Put(q[i])
				q[i] = nil // help gc
			}
			index = -1

			if overflow {
				overflow = false
				fillq(now)
			}

			if index >= 0 {
				continue
			}

			break
		}

		if t.close {
			break
		}
	}

	tc.Stop()
}

func (t *Transport) Process(payload []byte) {
	v, msgId, err := t.edM.DecodeMessage(payload)
	if err != nil {
		log.Printf("msgId:%s decode, %s", msgId, err)
		// leak sm? no, by timer gc
		return
	}

	sm := t.statmachinePool.Pop(msgId)
	if sm == nil {
		// maybe timeout
		log.Printf("msgId:%s maybe statmachine timeout", msgId)
		return
	}

	sm.Process(msgId, v)
}

func (t *Transport) Timeout(msgId string) {
	sm := t.statmachinePool.Pop(msgId)
	if sm == nil {
		return
	}
	log.Printf("msgId:%s The state machine timed out", msgId)
	t.executor.Timeout(sm, msgId)
}
