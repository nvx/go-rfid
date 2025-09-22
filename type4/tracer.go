package type4

import (
	"context"
	"errors"
	"github.com/nvx/go-rfid"
	"hash/crc32"
)

type Tracer interface {
	Reader(ctx context.Context, capdu []byte)
	Tag(ctx context.Context, rapdu []byte)
}

type DynamicTracer struct {
	Tracer
}

func NewDynamicTracer() *DynamicTracer {
	return &DynamicTracer{Tracer: &noopTracer{}}
}

type MultiTracer []Tracer

func (m MultiTracer) Reader(ctx context.Context, b []byte) {
	for _, t := range m {
		t.Reader(ctx, b)
	}
}

func (m MultiTracer) Tag(ctx context.Context, b []byte) {
	for _, t := range m {
		t.Tag(ctx, b)
	}
}

func NewMultiTracer(tracers ...Tracer) Tracer {
	return MultiTracer(tracers)
}

func TracingApduer(tracer Tracer, apduer rfid.Exchanger) rfid.ExchangerAPDUer {
	cb := func(ctx context.Context, capdu []byte) (_ []byte, err error) {
		tracer.Reader(ctx, capdu)

		checksum := crc32.ChecksumIEEE(capdu)
		rapdu, err := apduer.Exchange(ctx, capdu)
		if checksum != crc32.ChecksumIEEE(capdu) {
			err = errors.New("reader tracer munged data")
			return
		}

		if err == nil {
			checksum = crc32.ChecksumIEEE(rapdu)
			tracer.Tag(ctx, rapdu)
			if checksum != crc32.ChecksumIEEE(rapdu) {
				err = errors.New("tag tracer munged data")
				return
			}
		}

		return rapdu, err
	}

	return rfid.ExchangerFunc(cb)
}

type noopTracer struct{}

func (n noopTracer) Reader(context.Context, []byte) {}
func (n noopTracer) Tag(context.Context, []byte)    {}
