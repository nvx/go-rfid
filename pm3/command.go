package pm3

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/nvx/go-rfid"
	"io"
)

const (
	commandPreambleMagic  = uint32(0x61334D50) // PM3a
	commandPostambleMagic = uint16(0x3361)     // a3
)

var (
	CommandEnterStandalone = Command{
		NG:      true,
		Command: 0x0115,
		Data:    []byte{0x01},
	}
)

type Command struct {
	NG      bool
	Command uint16
	Data    []byte
}

func (p Command) WriteTo(w io.Writer) (_ int64, err error) {
	defer rfid.DeferWrap(context.Background(), &err)

	n, err := w.Write(p.Bytes())
	return int64(n), err
}

func (p Command) Bytes() []byte {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.LittleEndian, commandPreambleMagic)
	if err != nil {
		panic(err)
	}

	lenNg := uint16(len(p.Data) & 0x7FFF)
	if p.NG {
		lenNg |= 1 << 15
	}
	err = binary.Write(buf, binary.LittleEndian, lenNg)
	if err != nil {
		panic(err)
	}

	err = binary.Write(buf, binary.LittleEndian, p.Command)
	if err != nil {
		panic(err)
	}

	_, err = buf.Write(p.Data)
	if err != nil {
		panic(err)
	}

	err = binary.Write(buf, binary.LittleEndian, commandPostambleMagic)
	if err != nil {
		panic(err)
	}

	return buf.Bytes()
}
