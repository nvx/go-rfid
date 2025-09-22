package cardhopper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/nvx/go-rfid"
	"io"
)

type Packet []byte

var (
	MagicRead    = Packet("READ")
	MagicCard    = Packet("CARD")
	MagicRestart = Packet("RESTART")
	MagicEnd     = Packet("\xFFEND")
	MagicErr     = Packet("\xFFERR")
)

var (
	ErrPacketTooBig = errors.New("packet too big")
)

func (p *Packet) Bytes() []byte {
	return append([]byte{byte(len(*p))}, *p...)
}

func (p *Packet) WriteToIgnoreAck(writer io.Writer) (n int64, err error) {
	defer rfid.DeferWrap(context.Background(), &err)
	if len(*p) > 255 {
		err = ErrPacketTooBig
		return
	}

	written, err := writer.Write(p.Bytes())
	n += int64(written)
	if err != nil {
		return
	}

	return
}

func (p *Packet) WriteTo(writer io.Writer, reader io.Reader) (n int64, err error) {
	defer rfid.DeferWrap(context.Background(), &err)

	n, err = p.WriteToIgnoreAck(writer)
	if err != nil {
		return
	}

	// Read ACK
	var ackBuf [1]byte
	_, err = reader.Read(ackBuf[:])
	if err != nil {
		return
	}

	if ackBuf[0] != 0xFE {
		err = fmt.Errorf("bad ack: %X", ackBuf[:])
		return
	}

	return
}

func (p *Packet) ReadFrom(reader io.Reader) (n int64, err error) {
	defer rfid.DeferWrap(context.Background(), &err)

	var packetLenBuf [1]byte
	var read int
	read, err = reader.Read(packetLenBuf[:])
	n += int64(read)
	if err != nil || read == 0 {
		return
	}
	packetLen := int(packetLenBuf[0])

	if cap(*p) < packetLen {
		*p = make([]byte, packetLen)
	} else {
		*p = (*p)[:packetLen]
	}

	if packetLen == 0 {
		return
	}

	read, err = io.ReadFull(reader, *p)
	n += int64(read)
	if err != nil {
		return
	}

	// ACK is only processed by relay server
	return
}

func (p *Packet) Write(b []byte) (n int, err error) {
	if len(*p)+len(b) > 255 {
		err = ErrPacketTooBig
		return
	}

	*p = append(*p, b...)
	return len(b), nil
}

func (p *Packet) Truncate(n int) {
	if n < 0 || n > len(*p) {
		panic("cardhopper.Packet: truncation out of range")
	}
	*p = (*p)[:n]
}

func (p *Packet) Reset() {
	*p = (*p)[:0]
}

func (p *Packet) Equal(other Packet) bool {
	return bytes.Equal(*p, other)
}
