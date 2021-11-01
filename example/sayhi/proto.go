package main

import (
	"fmt"
	"io"

	"encoding/binary"
)

const (
	SAYHI_REQUEST_CMD = iota
	SAYHI_RESPONSE_CMD
)

type Serializer interface {
	Size() uint32
	Serialize([]byte) (uint32, error)
	Unserialize([]byte) (uint32, error)
}

////// declare of proto
type Header struct {
	Cmd   uint32
	MsgId string
}

type SayHiReq struct {
	Name string
}

type SayHiResp struct {
	Name string
}

type Message struct {
	Header
	Serializer
}

////// implement of Header
// length of Header packet
func (h *Header) Size() uint32 {
	return uint32(4 + 4 + len(h.MsgId))
}

// format:
//		+----------------------+-----------------------------------------------------+
//		|payload size of header|				header payload															 |
//		+----------------------+-----------------------------------------------------+
//    |                      |  Cmd field     |     MsgId field										 |
//		+----------------------+-----------------------------------------------------+
//		|      4 byte          |   4 byte       |    (*(payload size) - 4) byte   	 |
//		+----------------------+-----------------------------------------------------+
func (h *Header) Serialize(payload []byte) (uint32, error) {
	hsize := h.Size()
	if uint32(len(payload)) < h.Size() {
		return 0, fmt.Errorf("Not enough loading space")
	}

	binary.BigEndian.PutUint32(payload[:4], hsize-4)
	binary.BigEndian.PutUint32(payload[4:8], h.Cmd)
	copy(payload[8:], h.MsgId)
	return hsize, nil
}

func (h *Header) Unserialize(payload []byte) (uint32, error) {
	if uint32(len(payload)) < h.Size() {
		return 0, fmt.Errorf("payload is deformed for header, size:%d", len(payload))
	}

	rsize := binary.BigEndian.Uint32(payload[:4])
	if rsize < 4 || rsize+4 > uint32(len(payload)) {
		return 0, fmt.Errorf("header package is deformed, size:%d", rsize)
	}

	//log.Printf("Header Unserialize, rsize:%d", rsize)

	h.Cmd = binary.BigEndian.Uint32(payload[4:8])
	leftSize := rsize - 4
	if leftSize < 1024 {
		// Deep copy
		h.MsgId = string(payload[8 : 4+rsize])
	} else {
		// TODO
		h.MsgId = string(payload[8 : 4+rsize])
	}

	return rsize + 4, nil
}

////// implement of SayHiReq
// length of SayHiReq packet
func (req *SayHiReq) Size() uint32 {
	return uint32(4 + len(req.Name))
}

// format:
//	sayhireq length (4 byte) | name ((sayhireq length) byte)
func (req *SayHiReq) Serialize(payload []byte) (uint32, error) {
	if len(payload) < int(req.Size()) {
		return 0, fmt.Errorf("payload is deformed for sayhireq, payload size:%d,  but name:%s", len(payload), req.Name)
	}
	binary.BigEndian.PutUint32(payload[:4], uint32(len(req.Name)))
	copy(payload[4:], req.Name)
	return req.Size(), nil
}

func (req *SayHiReq) Unserialize(payload []byte) (uint32, error) {
	if len(payload) < int(req.Size()) {
		return 0, fmt.Errorf("payload is deformed for sayhireq, size:%d", len(payload))
	}

	rsize := binary.BigEndian.Uint32(payload[:4])
	if rsize+4 > uint32(len(payload)) {
		return 0, fmt.Errorf("sayhireq package is deformed, size:%d", rsize)
	}

	if rsize == 0 {
		return 4 + rsize, nil
	}

	if rsize < 1024 {
		req.Name = string(payload[4 : 4+rsize])
	}
	return 4 + rsize, nil
}

////// implement of SayHiResp
// length of SayHiResp packet
func (resp *SayHiResp) Size() uint32 {
	return uint32(4 + len(resp.Name))
}

// format:
//	sayhiresp length (4 byte) | name ((sayhiresp length) byte)
func (resp *SayHiResp) Serialize(payload []byte) (uint32, error) {
	if len(payload) < int(resp.Size()) {
		return 0, fmt.Errorf("payload is deformed for sayhiresp, payload size:%d,  but name:%s", len(payload), resp.Name)
	}
	binary.BigEndian.PutUint32(payload[:4], uint32(len(resp.Name)))
	copy(payload[4:], resp.Name)
	return uint32(4 + len(resp.Name)), nil
}

func (resp *SayHiResp) Unserialize(payload []byte) (uint32, error) {
	if len(payload) < 4 {
		return 0, fmt.Errorf("payload is deformed for sayhiresp, size:%d", len(payload))
	}

	rsize := binary.BigEndian.Uint32(payload[:4])
	//log.Printf("SayHiResp, rsize:%d", rsize)
	if rsize+4 > uint32(len(payload)) {
		return 0, fmt.Errorf("sayhiresp package is deformed, size:%d", rsize)
	}

	if rsize == 0 {
		return 4 + rsize, nil
	}

	if rsize < 1024 {
		resp.Name = string(payload[4 : 4+rsize])
	}
	return 4 + rsize, nil
}

////// implement of Message
func (m *Message) Size() uint32 {
	if m.Serializer != nil {
		return m.Header.Size() + m.Serializer.Size()
	}

	return m.Header.Size()
}

func (m *Message) EncodeMessage() ([]byte, string, error) {
	rsize := m.Size()
	// TODO allocate from mem pool
	payload := make([]byte, rsize)
	hsize, err := m.Header.Serialize(payload)
	if err != nil {
		return nil, "", err
	}
	//log.Printf("Message EncodeMessage, hsize:%d", hsize)
	_, err = m.Serializer.Serialize(payload[hsize:])
	if err != nil {
		return nil, "", err
	}
	//log.Printf("Message EncodeMessage, ssize:%d", ssize)

	return payload, m.Header.MsgId, nil
}

func (m *Message) DecodeMessage(payload []byte) error {
	iter, err := m.Header.Unserialize(payload)
	if err != nil {
		return err
	}

	//log.Printf("Message DecodeMessage, iter:%d", iter)

	switch m.Cmd {
	case SAYHI_REQUEST_CMD:
		m.Serializer = &SayHiReq{}
	case SAYHI_RESPONSE_CMD:
		m.Serializer = &SayHiResp{}
	default:
		return fmt.Errorf("not support cmd no:%d", m.Cmd)
	}

	_, err = m.Serializer.Unserialize(payload[iter:])
	if err != nil {
		return err
	}
	return nil
}

////// implement of machine that encode and decode message
type EnDecMessage struct {
}

func (ed *EnDecMessage) EncodeMessage(v interface{}) (payload []byte, msgId string, err error) {
	switch msg := v.(type) {
	case *Message:
		return msg.EncodeMessage()
	default:
		err = fmt.Errorf("not supported protocol")
	}
	return
}

func (ed *EnDecMessage) DecodeMessage(payload []byte) (v interface{}, msgId string, err error) {
	// TODO allocate from mem pool
	msg := &Message{}
	err = msg.DecodeMessage(payload)
	if err != nil {
		return
	}
	return msg, msg.Header.MsgId, nil
}

////// implement of machine that encode and decode packet
const (
	DEFAULT_PACKET_HEAD_BYTE_SIZE = 4
)

type EnDecPacket struct {
}

func (ed *EnDecPacket) EncodePacket(w io.Writer, payload []byte) error {
	var (
		length uint32 = uint32(len(payload))
		head   [DEFAULT_PACKET_HEAD_BYTE_SIZE]byte
	)
	binary.BigEndian.PutUint32(head[:], length)
	l, err := w.Write(head[:])
	if err != nil {
		return err
	}

	if l < DEFAULT_PACKET_HEAD_BYTE_SIZE {
		return fmt.Errorf("write packet header no complete, %d", l)
	}

	var offset int
	for {
		n, err := w.Write(payload[offset:])
		offset += n
		if err != nil {
			return err
		}
		if offset >= int(length) {
			break
		}
	}
	return nil
}

func (ed *EnDecPacket) DecodePacket(r io.Reader) ([]byte, error) {
	var head [DEFAULT_PACKET_HEAD_BYTE_SIZE]byte
	n, err := io.ReadFull(r, head[:])
	if err != nil {
		return nil, err
	}

	if n < DEFAULT_PACKET_HEAD_BYTE_SIZE {
		return nil, fmt.Errorf("decode packet size no complete, size is %d", n)
	}

	size := binary.BigEndian.Uint32(head[:])
	if size == 0 {
		return nil, nil
	}

	// TODO allocate from mem pool
	buf := make([]byte, size)
	n, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	//log.Printf("EnDecPacket DecodePacket size:%d", size)
	if n != int(size) {
		return nil, fmt.Errorf("read body expect:%d,but have %d byte", size, n)
	}
	return buf, nil
}
