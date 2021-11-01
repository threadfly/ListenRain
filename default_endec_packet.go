// The default implementation to solve the sticky packet problem,
// by adding 4 bytes to the message synchronization to indicate
// the message length
package listenrain

import (
	"fmt"
	"io"

	"encoding/binary"
)

const (
	DEFAULT_PACKET_HEAD_BYTE_SIZE = 4
)

func defaultAllocatePacketBuffer(size uint32) ([]byte, error) {
	return make([]byte, size), nil
}

type DefaultEnDecPacket struct {
	AllocatePacketBuffer func(size uint32) ([]byte, error)
}

func (ed *DefaultEnDecPacket) EncodePacket(w io.Writer, payload []byte) error {
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

func (ed *DefaultEnDecPacket) DecodePacket(r io.Reader) ([]byte, error) {
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

	if ed.AllocatePacketBuffer == nil {
		ed.AllocatePacketBuffer = defaultAllocatePacketBuffer
	}

	buf, err := ed.AllocatePacketBuffer(size)
	if err != nil {
		return nil, err
	}

	n, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	if n != int(size) {
		return nil, fmt.Errorf("read body expect:%d,but have %d byte", size, n)
	}
	return buf, nil
}
