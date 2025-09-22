package cardhopper

import (
	"bytes"
	"github.com/nvx/go-rfid/pm3"
	"io"
)

type acker struct {
	rw         io.ReadWriter
	pendingAck bool
}

func NewAcker(rw io.ReadWriter) io.ReadWriter {
	return &acker{rw: rw}
}

var _ io.ReadWriter = (*acker)(nil)

func (a *acker) Read(p []byte) (n int, err error) {
	if a.pendingAck && len(p) > 0 {
		a.pendingAck = false
		p[0] = 0xFE
		return 1, nil
	}
	return a.rw.Read(p)
}

func (a *acker) Write(p []byte) (n int, err error) {
	n, err = a.rw.Write(p)
	a.pendingAck = err == nil && !bytes.Equal(pm3.CommandEnterStandalone.Bytes(), p)
	return
}
