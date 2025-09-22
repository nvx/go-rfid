package type4

import (
	"context"
	"errors"
	"github.com/nvx/go-rfid"
	"hash/crc32"
)

var _ Handler = (*Emulator)(nil)

type Handler interface {
	rfid.Exchanger
	Reset(context.Context)
}

type Emulator struct {
	UID     []byte
	SAK     byte
	ATQA    []byte
	ATR     []byte
	ATS     []byte
	Handler Handler
	Tracer  Tracer
}

func (t *Emulator) Exchange(ctx context.Context, capdu []byte) (_ []byte, err error) {
	defer rfid.DeferWrap(ctx, &err)

	if t.Tracer != nil {
		checksum := crc32.ChecksumIEEE(capdu)
		t.Tracer.Reader(ctx, capdu)
		if checksum != crc32.ChecksumIEEE(capdu) {
			err = errors.New("reader tracer munged data")
			return
		}
	}

	rapdu, err := t.Handler.Exchange(ctx, capdu)
	if err != nil {
		return
	}

	if t.Tracer != nil {
		checksum := crc32.ChecksumIEEE(rapdu)
		t.Tracer.Tag(ctx, rapdu)
		if checksum != crc32.ChecksumIEEE(rapdu) {
			err = errors.New("tag tracer munged data")
			return
		}
	}

	return rapdu, nil
}

func (t *Emulator) Reset(ctx context.Context) {
	t.Handler.Reset(ctx)
}
