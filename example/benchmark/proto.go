package main

import (
	"fmt"
	"io"
	"log"
	"sync"

	listenrain "github.com/threadfly/ListenRain"
)

const (
	MessageIDSize = 36
)

var (
	PayloadSize       = 4 * 1024
	PayloadBufferSize = MessageIDSize + PayloadSize
)

func ResetVar(payloadSize int) {
	PayloadSize = payloadSize
	PayloadBufferSize = MessageIDSize + PayloadSize
}

func NewUUID(num *int) string {
	// The display width is the same as MessageIDSize
	uuid := fmt.Sprintf("%36d", *num)
	*num += 1
	return uuid
}

var (
	PayloadBufferPool *sync.Pool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, PayloadBufferSize)
		},
	}
)

type MessageID []byte

func NewMessageID(c string) MessageID {
	cs := StringToBytes(c)
	if len(cs) != MessageIDSize {
		log.Fatalf("message id 's size:%d is invalid", len(cs))
	}
	return MessageID(cs)
}

func (m MessageID) Set(msgId string) {
	copy([]byte(m), StringToBytes(msgId))
}

func (m MessageID) String() string {
	//return BytesToString(m.Bytes())
	return string(m.Bytes())
}

func (m MessageID) Bytes() []byte {
	return []byte(m)
}

type BMMessage struct {
	msgId   MessageID
	payload []byte
}

func (m BMMessage) MsgID() string {
	return m.msgId.String()
}

func (m BMMessage) EncodeMessage() (payload []byte, msgId string, err error) {
	payload = ResetBytes(m.payload, MessageIDSize)
	return payload, m.msgId.String(), nil
}

type BMEnDecMessage struct {
}

func (ed *BMEnDecMessage) DecodeMessage(payload []byte) (v interface{}, msgId string, err error) {
	msg := BMMessage{
		msgId:   MessageID(payload[:MessageIDSize]),
		payload: payload[MessageIDSize:],
	}
	return msg, msg.MsgID(), nil
}

func (ed *BMEnDecMessage) EncodeMessage(v interface{}) (payload []byte, msgId string, err error) {
	msg := v.(BMMessage)
	return msg.EncodeMessage()
}

type BMEnDecPacket struct {
	listenrain.DefaultEnDecPacket
}

func (ed *BMEnDecPacket) init() {
	ed.DefaultEnDecPacket.AllocatePacketBuffer = ed.AllocatePacketBuffer
}

func (ed *BMEnDecPacket) AllocatePacketBuffer(size uint32) ([]byte, error) {
	return PayloadBufferPool.Get().([]byte), nil
}

func (ed *BMEnDecPacket) EncodePacket(w io.Writer, payload []byte) error {
	err := ed.DefaultEnDecPacket.EncodePacket(w, payload)
	if err != nil {
		return err
	}
	PayloadBufferPool.Put(payload)
	return nil
}
