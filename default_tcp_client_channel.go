package listenrain

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type TCPEndpointStat uint8

const (
	TCPEndpointStat_NORMAL TCPEndpointStat = iota
	TCPEndpointStat_EXCEPTION
)

var (
	TCP_TRANSPORTKEY_NOT_FOUND_CHANNEL_ERROR = errors.New("No available channel found")
)

type TCPEndpoint struct {
	IKey string
	Ip   string
	Port int
	stat TCPEndpointStat
}

func (k *TCPEndpoint) Key() string {
	if k.IKey != "" {
		return k.IKey
	}

	k.IKey = fmt.Sprintf("%s:%d", k.Ip, k.Port)
	return k.IKey
}

type TCPTransportKey struct {
	TCPEndpoint
}

func (k *TCPTransportKey) Key() string {
	return k.TCPEndpoint.Key()
}

type TcpChannel struct {
	net.Conn
}

func (tc *TcpChannel) IsActive() bool {
	return tc.RemoteAddr() != nil
}

func (tc *TcpChannel) PeerInfo() string {
	addr := tc.RemoteAddr()
	return fmt.Sprintf("%s:%s", addr.Network(), addr.String())
}

type TcpClientChannelGenerator struct {
	net.Addr
	key string
}

func NewTcpClientChannelGeneratorV2(key TransportKey) (ChannelGenerator, error) {
	switch k := key.(type) {
	case *TCPTransportKey:
		return NewTcpClientChannelGenerator(k.Ip, k.Port)
	case *HATCPTransportKey:
		return NewHATcpClientChannelGenerator(k)
	}
	return nil, errors.New("no supported tcp transport key type")
}

func NewTcpClientChannelGenerator(ip string, port int) (ChannelGenerator, error) {
	address := fmt.Sprintf("%s:%d", ip, port)
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	return &TcpClientChannelGenerator{
		Addr: addr,
		key:  address,
	}, nil
}

func (tcg *TcpClientChannelGenerator) Next() (Channel, error) {
	c, err := net.DialTimeout(tcg.Addr.Network(), tcg.Addr.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	return &TcpChannel{c}, nil
}

func (tcg *TcpClientChannelGenerator) IsTry(err error) bool {
	if err != nil {
		log.Printf("last time, tcp channel, %s", err)
	}
	return true
}

func (tcg *TcpClientChannelGenerator) GC(ch Channel) {
	if ch == nil {
		log.Printf("TcpClientChannelGenerator ch is nil")
		return
	}

	tcpC, ok := ch.(*TcpChannel)
	if ok {
		addr := tcpC.LocalAddr()
		log.Printf("TcpClientChannelGenerator GC tcp channel local:%s ", fmt.Sprintf("%s:%s", addr.Network(), addr.String()))
	}

	log.Printf("TcpClientChannelGenerator GC tcp channel peer:%s", tcpC.PeerInfo())
	err := ch.Close()
	if err != nil {
		log.Printf("TcpClientChannelGenerator gc channel, close %s", err)
	}
}

/*
 *	HATCPTransportKey is a highly available tcp component that provides active and standby
 *	access endpoints, so that when part of the access endpoints are abnormal, it is possible
 *	to switch between different access points within the framework without affecting the
 *	business level.
 */
type HATCPTransportKey struct {
	endpoints []TCPEndpoint
}

func (k *HATCPTransportKey) SetActive(ip string, port int) {
	if k.endpoints == nil {
		k.endpoints = make([]TCPEndpoint, 1, 3)
	}

	k.endpoints[0].Ip = ip
	k.endpoints[0].Port = port
	k.endpoints[0].IKey = fmt.Sprintf("%s:%d", ip, port)
}

func (k *HATCPTransportKey) SetStandBy(ip string, port int) {
	if k.endpoints == nil {
		k.SetActive(ip, port)
		return
	}

	k.endpoints = append(k.endpoints, TCPEndpoint{
		Ip:   ip,
		Port: port,
		IKey: fmt.Sprintf("%s:%d", ip, port),
	})
}

func (k *HATCPTransportKey) Key() string {
	if k.endpoints == nil {
		panic("transport key is not set active endpoint")
	}

	return k.endpoints[0].Key()
}

type HATcpClientChannelGenerator struct {
	key   *HATCPTransportKey
	addrs []net.Addr
	point int
}

func NewHATcpClientChannelGenerator(key *HATCPTransportKey) (ChannelGenerator, error) {
	hatcg := &HATcpClientChannelGenerator{
		key:   key,
		addrs: make([]net.Addr, len(key.endpoints)),
		point: -1,
	}

	for i := range key.endpoints {
		addr, err := net.ResolveTCPAddr("tcp", (&key.endpoints[i]).Key())
		if err != nil {
			return nil, err
		}
		hatcg.addrs[i] = addr
	}

	return hatcg, nil
}

func (hatcg *HATcpClientChannelGenerator) Next() (Channel, error) {
	var searchPointStart int = hatcg.point
	hatcg.point++
	for {
		if hatcg.point >= len(hatcg.addrs) {
			hatcg.point = 0
		}

		if hatcg.point == searchPointStart {
			return nil, TCP_TRANSPORTKEY_NOT_FOUND_CHANNEL_ERROR
		}

		if hatcg.key.endpoints[hatcg.point].stat == TCPEndpointStat_EXCEPTION {
			hatcg.point++
			continue
		}
		break
	}

	c, err := net.DialTimeout(hatcg.addrs[hatcg.point].Network(),
		hatcg.addrs[hatcg.point].String(),
		10*time.Second)
	if err != nil {
		return nil, err
	}

	return &TcpChannel{c}, nil
}

func (hatcg *HATcpClientChannelGenerator) IsTry(err error) bool {
	if err != nil {
		log.Printf("last time, tcp channel, %s", err)
		errMsg := strings.ToLower(err.Error())
		exist1 := strings.Contains(errMsg, "connection")
		exist2 := strings.Contains(errMsg, "refuse")
		if exist2 && exist1 {
			hatcg.key.endpoints[hatcg.point].stat = TCPEndpointStat_EXCEPTION
			for i := 0; i < len(hatcg.addrs); i++ {
				if hatcg.key.endpoints[i].stat == TCPEndpointStat_NORMAL {
					return true
				}
			}
			return false
		}
	}
	return true
}

func (hatcg *HATcpClientChannelGenerator) GC(ch Channel) {
	if ch == nil {
		log.Printf("HATcpClientChannelGenerator GC ch is nil")
		return
	}

	tcpChannel, ok := ch.(*TcpChannel)
	if ok {
		return
	}

	err := tcpChannel.Close()
	if err != nil {
		log.Printf("gc tcp channel, close %s", err)
	}
}
