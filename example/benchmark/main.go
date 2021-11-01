package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"time"
)

type Perf struct {
	theme    string
	sum      int32
	interval time.Duration
}

func NewPerf(theme string, interval time.Duration) *Perf {
	return &Perf{
		theme:    theme,
		interval: interval,
	}
}

func (p *Perf) AtomicIncr() {
	atomic.AddInt32(&p.sum, 1)
}

func (p *Perf) Incr() {
	p.sum++
}

func (p *Perf) Start() {
	ticker := time.NewTicker(p.interval)
	for {
		select {
		case <-ticker.C:
			log.Printf("%s %f", p.theme, float64(p.sum)/float64(float64(p.interval)/float64(time.Second)))
			p.sum = 0
		}
	}
}

var (
	lrain          *listenrain.ListenRain
	clientMsgProto listenrain.ProtocolType
	serverMsgProto listenrain.ProtocolType
	gperf          *Perf
	serverManager  *ServerManager
)

type Client struct {
	originMsgID string
	initUUID    int
	perf        *Perf
}

func NewClient(originMsgID string, initUUID int, perf *Perf) *Client {
	return &Client{
		originMsgID: originMsgID,
		initUUID:    initUUID,
		perf:        perf,
	}
}

func (c *Client) NewMsg(msg BMMessage) BMMessage {
	uuid := NewUUID(&c.initUUID)
	//log.Printf("NewMsg %d", c.initUUID)
	payload := ResetBytes(msg.payload, MessageIDSize)
	copy(payload[:MessageIDSize], StringToBytes(uuid))
	return msg
}

func (c *Client) FirstMsg() BMMessage {
	var msg BMMessage
	msg.msgId = NewMessageID(c.originMsgID)
	payload := PayloadBufferPool.Get().([]byte)
	copy(payload[:MessageIDSize], StringToBytes(c.originMsgID))
	msg.payload = payload[MessageIDSize:]
	return msg
}

func (c *Client) Do() {
	var (
		old BMMessage = c.FirstMsg()
		new interface{}
		err error
	)
	for {
		new, err = lrain.SyncSend(clientMsgProto, serverManager.SelectTransportKey(), old)
		if err != nil {
			log.Printf("sync send msgId:%s, %s, try again", old.MsgID(), err)
			continue
		} else {
			old = c.NewMsg(new.(BMMessage))
		}

		gperf.AtomicIncr()
		c.perf.Incr()
	}
}

func ServerRouter(response listenrain.ServerResponse, msgId string, cmd int, message interface{}) error {
	//log.Printf("ServerRouter msgId:%s", msgId)
	return response.Response(message.(BMMessage))
}

func init() {
	endecPacket := &BMEnDecPacket{}
	endecPacket.init()
	lrain = listenrain.NewListenRain(listenrain.NewDefaultTransportPool())
	clientMsgProto = lrain.RegisterProtocol(&BMEnDecMessage{},
		endecPacket,
		func() time.Duration { return 5 * time.Second },
		listenrain.NewTcpClientChannelGeneratorV2,
		listenrain.DefaultQueueGenerator,
		listenrain.DefaultExecutorGenerator,
		listenrain.DefaultStatMachinePoolGenerator)
	serverMsgProto = lrain.RegisterServerProtocol(&BMEnDecMessage{},
		endecPacket,
		func() time.Duration { return 5 * time.Second },
		listenrain.NewTcpServerChannleGenerator,
		listenrain.DefaultQueueGenerator,
		listenrain.DefaultExecutorGenerator,
		listenrain.ServerRouter(ServerRouter),
		"Benchmark Server")

}

func pprof() {
	err := http.ListenAndServe("0.0.0.0:8090", nil)
	if err != nil {
		log.Fatalf("listen and server, %s", err)
	}
}

var (
	paralle     = flag.Int("paralle", 1, "")
	payloadSize = flag.Int("payloadSize", 4096, "")
	port        = flag.Int("port", 8899, "")
	serverCount = flag.Int("sc", 1, "server count")
)

func init2() {
	var sec int = *paralle
	if sec > 10 {
		sec = 10
	}
	gperf = NewPerf("global qps:", time.Duration(sec)*time.Second)
	go gperf.Start()
	ResetVar(*payloadSize)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	go pprof()

	serverManager = NewServerManager(*serverCount, *port)
	serverManager.Do()
	go func() {
		http.ListenAndServe("0.0.0.0:8888", nil)
	}()
}

func main() {
	flag.Parse()
	init2()

	var (
		wg sync.WaitGroup
	)

	wg.Add(*paralle)
	time.Sleep(time.Second)
	for i := 0; i < *paralle; i++ {
		var (
			gap  int   = 10000000000 // Tens of billions
			uuid int   = i * gap
			perf *Perf = NewPerf(fmt.Sprintf("cli(%d) qps:", i), time.Second)
		)
		go perf.Start()
		cli := NewClient(NewUUID(&uuid), uuid, perf)
		go cli.Do()
	}
	wg.Wait()
}
