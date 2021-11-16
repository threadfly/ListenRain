package listenrain

import (
	"sync"
)

type DefaultTransportPool struct {
	m   sync.Map
	mtx sync.Mutex
	q   map[string]*sync.Mutex
}

func NewDefaultTransportPool() *DefaultTransportPool {
	return &DefaultTransportPool{
		q: make(map[string]*sync.Mutex),
	}
}

func (p *DefaultTransportPool) Get(transportKey TransportKey,
	typ *protocolType) (*Transport, error) {
	v, exist := p.m.Load(transportKey.Key())
	if exist {
		// fast path
		return v.(*Transport), nil
	}
	// slow path
	// Queue
	p.mtx.Lock()
	kmtx, exist := p.q[transportKey.Key()]
	if !exist {
		kmtx = new(sync.Mutex)
		p.q[transportKey.Key()] = kmtx
	}
	p.mtx.Unlock()

	kmtx.Lock()
	defer kmtx.Unlock()
	// init transport and set in map
	v, exist = p.m.Load(transportKey.Key())
	if exist {
		return v.(*Transport), nil
	}

	transport, err := NewTransport(transportKey, typ)
	if err != nil {
		return nil, err
	}

	p.m.Store(transportKey.Key(), transport)
	return transport, nil
}

func (p *DefaultTransportPool) Drop(transportKey TransportKey) {
	p.m.Delete(transportKey.Key())
}
