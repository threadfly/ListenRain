package listenrain

import (
	"errors"
	"fmt"
	"log"
	"net"
)

const (
	SERVER_ACCEPT_MAX_ERROR_RETRY = 3
)

func NewTcpServerChannleGenerator(key TransportKey) (ChannelGenerator, error) {
	switch k := key.(type) {
	case *TCPTransportKey:
		g := &TcpServerChannelGenerator{}
		return g, g.listen(k.Ip, k.Port)
	}

	return nil, errors.New("no supported tcp transport key type")
}

type TcpServerChannelGenerator struct {
	net.Listener
	key string
}

func (tcg *TcpServerChannelGenerator) listen(ip string, port int) error {
	tcg.key = fmt.Sprintf("%s:%d", ip, port)
	addr, err := net.ResolveTCPAddr("tcp", tcg.key)
	if err != nil {
		return err
	}

	tcg.Listener, err = net.Listen(addr.Network(), addr.String())
	if err != nil {
		return err
	}

	return nil
}

func (tcg *TcpServerChannelGenerator) Next() (Channel, error) {
	var retry int
	for {
		c, err := tcg.Listener.Accept()
		if err != nil {
			nerr, ok := err.(net.Error)
			if ok {
				if retry >= SERVER_ACCEPT_MAX_ERROR_RETRY {
					return nil, err
				}
				if nerr.Timeout() {
					retry++
					continue
				} else if nerr.Temporary() {
					retry++
					continue
				}
			}
			return nil, err
		}

		return &TcpChannel{c}, nil
	}
}

func (tcg *TcpServerChannelGenerator) IsTry(err error) bool {
	return false
}

func (tcg *TcpServerChannelGenerator) GC(ch Channel) {
	tcpChannel, ok := ch.(*TcpChannel)
	if !ok {
		log.Printf("TcpServerChannelGenerator GC ch type Assert fail")
		return
	}

	err := tcpChannel.Close()
	if err != nil {
		log.Printf("TcpServerChannelGenerator gc channel, close %s", err)
	} else {
		log.Printf("TcpServerChannelGenerator gc channel peer info:%s", tcpChannel.PeerInfo())
	}
}
