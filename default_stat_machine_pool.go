package listenrain

import (
	"sync"
)

// simple implementation for fast iteration
type DefaultStatMachinePool struct {
	mtx sync.Mutex
	c   map[string]StatMachine
}

func (p *DefaultStatMachinePool) Put(msgId string, sm StatMachine) {
	p.mtx.Lock()
	p.c[msgId] = sm
	p.mtx.Unlock()
}

func (p *DefaultStatMachinePool) Pop(msgId string) StatMachine {
	p.mtx.Lock()
	sm := p.c[msgId]
	delete(p.c, msgId)
	p.mtx.Unlock()
	return sm
}

func DefaultStatMachinePoolGenerator(key TransportKey) (StatMachinePool, error) {
	return &DefaultStatMachinePool{
		c: make(map[string]StatMachine),
	}, nil
}
