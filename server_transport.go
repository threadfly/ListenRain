package listenrain

import (
	"fmt"
	"log"
	"sync"
)

type ServerResponse interface {
	Response(message interface{}) error
	// Close current server transport
	Close()
}

type CmdMethoder interface {
	Cmd() int
}

type serverTransport struct {
	ch       Channel
	q        Queue
	edP      EnDecPacket
	edM      EnDecMessage
	cg       ChannelGenerator
	executor Executor
	close    bool
	err      error
	wg       sync.WaitGroup
	router   ServerRouter
}

func newServerTransport(ch Channel, transportKey TransportKey, pt *protocolType, cg ChannelGenerator) (*serverTransport, error) {
	q, err := pt.QueueGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	exe, err := pt.ExecutorGenerator(transportKey)
	if err != nil {
		return nil, err
	}

	transport := &serverTransport{
		ch:       ch,
		q:        q,
		edP:      pt.EdP,
		edM:      pt.EdM,
		cg:       cg,
		executor: exe,
		router:   pt.ServerRouter,
	}

	return transport, nil
}

// This logic is actually very similar to client transport, and can be unified in the follow-up
func (t *serverTransport) runloop() error {
	t.wg.Add(2)
	var closewg sync.WaitGroup
	closewg.Add(1)
	go func() {
		// receive request from client
		for {
			rcvPayload, err := t.edP.DecodePacket(t.ch)
			if t.close || t.err != nil {
				break
			}

			if err != nil {
				// TODO
				t.err = err
				break
			}

			t.executor.Process(t, rcvPayload)
		}
		t.wg.Done()
		closewg.Done()
	}()

	for {
		// deal with response from server
		payload := t.q.Pop()
		err := t.edP.EncodePacket(t.ch, payload)
		if err != nil {
			t.err = err
		}

		if t.close || t.err != nil {
			break
		}
	}

	if t.close {
		closewg.Wait()
		for payload := t.q.PopNoBlocking(); payload != nil; {
			err := t.edP.EncodePacket(t.ch, payload)
			if err != nil {
				// callback app layer
				log.Printf("server transport encode packet from %s failed, %s", t.ch.PeerInfo(), err)
			}
		}
	}

	if t.err != nil {
		log.Printf("server transport(peer:%s) closed for %s", t.ch.PeerInfo(), t.err)
	}

	t.wg.Done()
	return t.err
}

func (t *serverTransport) Process(payload []byte) {
	v, msgId, err := t.edM.DecodeMessage(payload)
	if err != nil {
		log.Printf("server transport msgId:%s decode, %s", msgId, err)
		return
	}

	var cmdNo int = -19900405
	if t.router == nil {
		log.Printf("server transport not register router function")
		return
	} else if cmd, ok := v.(CmdMethoder); ok {
		cmdNo = cmd.Cmd()
	}

	err = t.router(t, msgId, cmdNo, v)
	if err != nil {
		log.Printf("server transport router function, %s", err)
	}
}

func (t *serverTransport) Response(message interface{}) error {
	if t.close {
		return fmt.Errorf("channel of to [%s] is closed", t.ch.PeerInfo())
	}

	payload, _, err := t.edM.EncodeMessage(message)
	if err != nil {
		return err
	}

	t.q.Push(payload)
	return nil
}

func (t *serverTransport) Close() {
	if t.close {
		return
	}

	t.close = true
	t.wg.Wait()
}

func (t *serverTransport) Timeout(msgId string) {
	// nothing to do
	// TODO: Automatic response timeout
}
