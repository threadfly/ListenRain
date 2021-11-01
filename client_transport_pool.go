package listenrain

import (
	"sync"
)

type DefaultTransportPool struct {
	m   sync.Map
	mtx sync.Mutex
	q   map[string]chan struct{}
}

func NewDefaultTransportPool() *DefaultTransportPool {
	return &DefaultTransportPool{
		q: make(map[string]chan struct{}),
	}
}

func (p *DefaultTransportPool) Get(transportKey TransportKey,
	typ *protocolType) (*Transport, error) {
	v, exist := p.m.Load(transportKey.Key())
	if !exist {
	InitTransport:
		// Queue
		p.mtx.Lock()
		c, exist2 := p.q[transportKey.Key()]
		if !exist2 {
			c = make(chan struct{})
			p.q[transportKey.Key()] = c
		}
		p.mtx.Unlock()

		v, exist = p.m.Load(transportKey.Key())
		if exist {
			p.mtx.Lock()
			c, exist2 = p.q[transportKey.Key()]
			if exist2 {
				close(c)
				delete(p.q, transportKey.Key())
			}
			p.mtx.Unlock()
			return v.(*Transport), nil
		}

		if exist2 {
			<-c
			v, exist2 := p.m.Load(transportKey.Key())
			if exist2 {
				return v.(*Transport), nil
			} else {
				goto InitTransport
			}
		} else {
			transport, err := NewTransport(transportKey, typ)
			if err != nil {
				p.mtx.Lock()
				close(p.q[transportKey.Key()])
				delete(p.q, transportKey.Key())
				p.mtx.Unlock()
				return nil, err
			} else {
				p.mtx.Lock()
				p.m.Store(transportKey.Key(), transport)
				close(p.q[transportKey.Key()])
				delete(p.q, transportKey.Key())
				p.mtx.Unlock()
				return transport, nil
			}
		}

		// TODO
		// Burst requests lead to a large number of links?
		//transport, err := NewTransport(transportKey, typ)
		//if err != nil {
		//	return nil, err
		//}

		//ac, loaded := p.m.LoadOrStore(transportKey.Key(), transport)
		//if loaded {
		//	transport.Close()
		//	return ac, nil
		//} else {
		//	return transport, nil
		//}
	}
	return v.(*Transport), nil
}
