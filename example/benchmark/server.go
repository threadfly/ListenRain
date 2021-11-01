package main

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

const (
	Listen_Addr = "0.0.0.0"
)

type ServerManager struct {
	count, initPort int
	tks             []listenrain.TCPTransportKey
	pollIdx         int64
}

func NewServerManager(count, initPort int) *ServerManager {
	return &ServerManager{
		count:    count,
		initPort: initPort,
		tks:      make([]listenrain.TCPTransportKey, count),
		pollIdx:  -1,
	}
}

func (sm *ServerManager) Do() {
	for i := range sm.tks {
		sm.tks[i].Ip = Listen_Addr
		sm.tks[i].Port = sm.initPort
		sm.initPort++
		go func(idx int) {
			time.Sleep(time.Second)
			log.Printf("ServerManager Do %d", idx)
			err := lrain.Listen(serverMsgProto, &sm.tks[idx])
			if err != nil {
				panic(fmt.Sprintf("server listen, %s\n", err))
			}
		}(i)
	}
	//time.Sleep(time.Second * 20)
	//log.Printf("ServerManager Do() Done")
}

func (sm *ServerManager) SelectTransportKey() *listenrain.TCPTransportKey {
	new := atomic.AddInt64(&sm.pollIdx, 1)
	return &(sm.tks[new%int64(sm.count)])
}
