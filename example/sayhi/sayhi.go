package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Response listenrain.ServerResponse

var (
	lrain          *listenrain.ListenRain
	clientMsgProto listenrain.ProtocolType
	serverMsgProto listenrain.ProtocolType
	serverEndpoint *listenrain.TCPTransportKey
)

type Client struct {
	sync.WaitGroup
}

type Server struct {
	respNo int
}

var (
	cli *Client = new(Client)
	srv *Server = new(Server)
)

// sync request
func (c *Client) SayHiReq() {
	req := SayHiReq{Name: "I am Sync Client"}
	message := Message{
		Header: Header{
			Cmd:   SAYHI_REQUEST_CMD,
			MsgId: "1111-1111-2222-2222",
		},
		Serializer: &req,
	}
	c.Add(1)
	resp, err := lrain.SyncSend(clientMsgProto, serverEndpoint, &message)
	if err != nil {
		panic(err)
	}

	c.Process("", resp)
}

// async request
func (c *Client) AsyncSayHiReq() {
	req := SayHiReq{Name: "I am Async Client"}
	message := Message{
		Header: Header{
			Cmd:   SAYHI_REQUEST_CMD,
			MsgId: "3333-3333-4444-4444",
		},
		Serializer: &req,
	}
	c.Add(1)
	err := lrain.Send(clientMsgProto, c, serverEndpoint, &message)
	if err != nil {
		panic(fmt.Sprintf("client async send, %s", err))
	}
}

// implement of listenrain.StatMachine interface
func (c *Client) Process(msgId string, v interface{}) {
	msg, ok := v.(*Message)
	if !ok {
		panic(fmt.Sprintf("no support response type, %#v", v))
	}

	switch msg.Header.Cmd {
	case SAYHI_RESPONSE_CMD:
		log.Printf("client: receive sayhi response, name:%s\n", msg.Serializer.(*SayHiResp).Name)
	default:
		panic(fmt.Sprintf("no support response cmd, %#v", msg.Header.Cmd))
	}
	c.Done()
}

// implement of listenrain.StatMachine interface
func (c *Client) Timeout(msgId string) {
	log.Printf("client request, msgId:%s timeout\n", msgId)
}

func (s *Server) SayHiResponse(response Response, msgId string, request *SayHiReq) {
	log.Printf("server: welcome %s\n", request.Name)
	log.Printf("server: msg id: %s\n", msgId)
	s.respNo++
	resp := SayHiResp{
		Name: fmt.Sprintf("Server Response No.%d", s.respNo),
	}
	message := Message{
		Header: Header{
			Cmd:   SAYHI_RESPONSE_CMD,
			MsgId: msgId,
		},
		Serializer: &resp,
	}
	err := response.Response(&message)
	if err != nil {
		log.Printf("server response, %s\n", err)
	}
}

func ServerRouter(response listenrain.ServerResponse, msgId string, cmd int, message interface{}) error {
	msg, ok := message.(*Message)
	if !ok {
		return fmt.Errorf("is not message proto")
	}
	switch msg.Header.Cmd {
	case SAYHI_REQUEST_CMD:
		srv.SayHiResponse(response, msg.Header.MsgId, msg.Serializer.(*SayHiReq))
	default:
		return fmt.Errorf("not support cmd type:%d", msg.Header.Cmd)
	}
	return nil
}

func init() {
	lrain = listenrain.NewListenRain(listenrain.NewDefaultTransportPool())
	clientMsgProto = lrain.RegisterProtocol(&EnDecMessage{},
		&EnDecPacket{},
		func() time.Duration { return 5 * time.Second },
		listenrain.NewTcpClientChannelGeneratorV2,
		listenrain.DefaultQueueGenerator,
		listenrain.DefaultExecutorGenerator,
		listenrain.DefaultStatMachinePoolGenerator)
	serverMsgProto = lrain.RegisterServerProtocol(&EnDecMessage{},
		&EnDecPacket{},
		func() time.Duration { return 5 * time.Second },
		listenrain.NewTcpServerChannleGenerator,
		listenrain.DefaultQueueGenerator,
		listenrain.DefaultExecutorGenerator,
		listenrain.ServerRouter(ServerRouter),
		"SayHi Server")
	serverEndpoint = &listenrain.TCPTransportKey{}
}

func main() {
	serverEndpoint.Ip = "0.0.0.0"
	serverEndpoint.Port = 8899
	go func() {
		err := lrain.Listen(serverMsgProto, serverEndpoint)
		if err != nil {
			panic(fmt.Sprintf("server listen, %s\n", err))
		}
	}()

	time.Sleep(time.Second)
	// sync request
	cli.SayHiReq()
	cli.AsyncSayHiReq()
	cli.Wait()
}
