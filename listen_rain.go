package listenrain

import (
	"errors"
	"io"
	"log"
	"sync"
	"time"
)

var (
	ErrInvalidTransport = errors.New("client transport is invalid")
)

type StatMachine interface {
	Process(msgId string, v interface{})
	Timeout(msgId string)
}

type StatMachinePool interface {
	Put(msgId string, sm StatMachine)
	Pop(msgId string) StatMachine
}

type EnDecMessage interface {
	EncodeMessage(message interface{}) (payload []byte, msgId string, err error)
	DecodeMessage(payload []byte) (message interface{}, msgId string, err error)
}

type Queue interface {
	Push(payload []byte)
	Pop() (payload []byte)
	PopNoBlocking() (payload []byte)
	Drop()
}

type EnDecPacket interface {
	EncodePacket(w io.Writer, payload []byte) error
	DecodePacket(r io.Reader) ([]byte, error)
}

type Channel interface {
	io.ReadWriteCloser
	IsActive() bool
	PeerInfo() string
}

type ChannelGenerator interface {
	Next() (Channel, error)
	GC(Channel)
	IsTry(error) bool
}

type ProtocolType int

type ServerRouter func(response ServerResponse, msgId string, cmd int, message interface{}) error

type protocolType struct {
	EdM                      EnDecMessage
	EdP                      EnDecPacket
	Timeout                  func() time.Duration
	ChannelGenerator         func(TransportKey) (ChannelGenerator, error)
	QueueGenerator           func(TransportKey) (Queue, error)
	ExecutorGenerator        func(TransportKey) (Executor, error)
	StatMachinePoolGenerator func(TransportKey) (StatMachinePool, error)
	ServerRouter             ServerRouter
	Name                     string
}

type TransportKey interface {
	Key() string
}

type TransportPool interface {
	Get(TransportKey, *protocolType) (*Transport, error)
	Drop(TransportKey)
}

// implements by Transport
type processRunner interface {
	Process(payload []byte)
}

type timeoutRunner interface {
	Timeout(msgId string)
}

// concurrency pool
type Executor interface {
	Process(r processRunner, payload []byte)
	Timeout(r timeoutRunner, msgId string)
}

type ListenRain struct {
	protoTyps     []*protocolType
	transportPool TransportPool
	ssmPool       *sync.Pool
}

func NewListenRain(transportPool TransportPool) *ListenRain {
	return &ListenRain{
		protoTyps:     make([]*protocolType, 0, 5),
		transportPool: transportPool,
		ssmPool: &sync.Pool{
			New: func() interface{} {
				return &SyncStatMachine{
					s: SSM_INIT,
				}
			},
		},
	}
}

// register protocol type before send
func (lr *ListenRain) RegisterProtocol(edm EnDecMessage, edp EnDecPacket,
	timeout func() time.Duration,
	channelGenerator func(TransportKey) (ChannelGenerator, error),
	queueGenerator func(TransportKey) (Queue, error),
	executor func(TransportKey) (Executor, error),
	statMachinePoolGenerator func(TransportKey) (StatMachinePool, error)) ProtocolType {
	lr.protoTyps = append(lr.protoTyps, &protocolType{
		EdM:                      edm,
		EdP:                      edp,
		Timeout:                  timeout,
		ChannelGenerator:         channelGenerator,
		QueueGenerator:           queueGenerator,
		ExecutorGenerator:        executor,
		StatMachinePoolGenerator: statMachinePoolGenerator,
	})

	return ProtocolType(len(lr.protoTyps) - 1)
}

func (lr *ListenRain) RegisterServerProtocol(edm EnDecMessage, edp EnDecPacket,
	timeout func() time.Duration,
	channelGenerator func(TransportKey) (ChannelGenerator, error),
	queueGenerator func(TransportKey) (Queue, error),
	executor func(TransportKey) (Executor, error),
	serverRouter ServerRouter,
	name string) ProtocolType {

	ptindex := lr.RegisterProtocol(edm, edp, timeout, channelGenerator,
		queueGenerator, executor, nil)
	pt := lr.ProtocolType(ptindex)
	pt.ServerRouter = serverRouter
	pt.Name = name
	return ptindex
}

func (lr *ListenRain) ProtocolType(ptyp ProtocolType) *protocolType {
	return lr.protoTyps[ptyp]
}

func (lr *ListenRain) Send(ptyp ProtocolType, sm StatMachine, key TransportKey, msg interface{}) error {
	protoTyps := lr.protoTyps[ptyp]
	transport, err := lr.transportPool.Get(key, protoTyps)
	if err != nil {
		return err
	}

	if transport.state == TRANSPORT_DOWN {
		lr.transportPool.Drop(key)
		go func() {
			// delay background gc
			time.Sleep(10 * time.Second)
			transport.Drop()
		}()
		return ErrInvalidTransport
	}

	err = transport.Send(sm, key, msg)
	if err != nil {
		return err
	}
	return nil
}

func (lr *ListenRain) SyncSend(ptyp ProtocolType, key TransportKey, msg interface{}) (interface{}, error) {
	protoTyps := lr.protoTyps[ptyp]
	transport, err := lr.transportPool.Get(key, protoTyps)
	if err != nil {
		return nil, err
	}

	if transport.state == TRANSPORT_DOWN {
		lr.transportPool.Drop(key)
		go func() {
			// delay background gc
			time.Sleep(10 * time.Second)
			transport.Drop()
		}()
		return nil, ErrInvalidTransport
	}

	ssm := lr.ssmPool.Get().(*SyncStatMachine)
	ssm.Fire()
	err = transport.Send(ssm, key, msg)
	if err != nil {
		ssm.ShutDown()
		lr.ssmPool.Put(ssm)
		return nil, err
	}

	v, err := ssm.Return()
	lr.ssmPool.Put(ssm)
	return v, err
}

func (lr *ListenRain) Listen(ptyp ProtocolType, key TransportKey) error {
	protoTyps := lr.protoTyps[ptyp]
	cg, err := protoTyps.ChannelGenerator(key)
	if err != nil {
		return err
	}

	for {
		ch, err := cg.Next()
		if err != nil {
			cg.GC(ch)
			log.Printf("next ch err, %s", err)
			if !cg.IsTry(err) {
				// TODO
				// close running channel
				break
			}
			continue
		}

		if !ch.IsActive() {
			log.Printf("ch not active")
			cg.GC(ch)
			continue
		}

		transport, err := newServerTransport(ch, key, protoTyps, cg)
		if err != nil {
			log.Printf("new server transport, %s", err)
			cg.GC(ch)
			continue
		}

		go func() {
			log.Printf("new transport")
			err := transport.runloop()
			if err != nil {
				log.Printf("server transport(peer:%s) runloop over, %s", transport.ch.PeerInfo(), err)
			}
			cg.GC(transport.ch)
		}()
	}
	return nil
}
