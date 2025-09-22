package rfid

import (
	"context"
	"github.com/nvx/go-apdu"
)

// Exchanger is a higher level version of RawSmartCard that takes a context for cancellation
type Exchanger interface {
	Exchange(ctx context.Context, capdu []byte) ([]byte, error)
}

// APDUer is a higher level version of Exchanger that operates on parsed ISO7816 APDUs instead of raw byte slices
type APDUer interface {
	APDU(context.Context, apdu.Capdu) (apdu.Rapdu, error)
}

// ExchangerAPDUer is a composite interface of Exchanger and APDUer
type ExchangerAPDUer interface {
	Exchanger
	APDUer
}

var (
	_ Exchanger = (ExchangerFunc)(nil)
	_ APDUer    = (APDUerFunc)(nil)
	_ Exchanger = (APDUerFunc)(nil)
)

// ExchangerFunc implements the ExchangerAPDUer interface as an Exchanger.Exchange func
type ExchangerFunc func(ctx context.Context, capdu []byte) ([]byte, error)

func (f ExchangerFunc) Exchange(ctx context.Context, capdu []byte) ([]byte, error) {
	return f(ctx, capdu)
}

func (f ExchangerFunc) APDU(ctx context.Context, capdu apdu.Capdu) (_ apdu.Rapdu, err error) {
	defer DeferWrap(ctx, &err)

	b, err := capdu.Bytes()
	if err != nil {
		return
	}

	r, err := f(ctx, b)
	if err != nil {
		return
	}

	return apdu.ParseRapdu(r)
}

// APDUerFunc implements the ExchangerAPDUer interface as an APDUer.APDU func
type APDUerFunc func(context.Context, apdu.Capdu) (apdu.Rapdu, error)

func (f APDUerFunc) APDU(ctx context.Context, capdu apdu.Capdu) (apdu.Rapdu, error) {
	return f(ctx, capdu)
}

func (f APDUerFunc) Exchange(ctx context.Context, capdu []byte) (_ []byte, err error) {
	defer DeferWrap(ctx, &err)

	c, err := apdu.ParseCapdu(capdu)
	if err != nil {
		return
	}

	rapdu, err := f(ctx, c)
	if err != nil {
		return
	}

	return rapdu.Bytes()
}

// RawSmartCard is an interface that is implemented by *github.com/ebfe/go-scard.Card
type RawSmartCard interface {
	Transmit(data []byte) ([]byte, error)
}

// RawSmartCardControl is an interface that is implemented by *github.com/ebfe/go-scard.Card
type RawSmartCardControl interface {
	Control(ioctl uint32, data []byte) ([]byte, error)
}

// RawSmartCardAPDUer transforms RawSmartCard into an ExchangerAPDUer with the ability to abandon waiting for a response
// if the context is cancelled
func RawSmartCardAPDUer(sc RawSmartCard) ExchangerAPDUer {
	return ExchangerFunc(func(ctx context.Context, capdu []byte) ([]byte, error) {
		return Escapable(ctx, func(ctx context.Context) ([]byte, error) {
			return sc.Transmit(capdu)
		})
	})
}

// RawSmartCardControlAPDUer transforms RawSmartCard into an ExchangerAPDUer by performing escape functions with the
// given ioctl. It also provides the ability to abandon waiting for a response if the context is cancelled
func RawSmartCardControlAPDUer(sc RawSmartCardControl, ioctl uint32) ExchangerAPDUer {
	return ExchangerFunc(func(ctx context.Context, capdu []byte) ([]byte, error) {
		return Escapable(ctx, func(ctx context.Context) ([]byte, error) {
			return sc.Control(ioctl, capdu)
		})
	})
}

// SmartCardControl is a higher level version of RawSmartCardControl that takes control codes instead of ioctls for
// platform portability and accepts a context for cancellation
type SmartCardControl interface {
	Control(ctx context.Context, code uint16, data []byte) ([]byte, error)
}

// SmartCardControlFunc implements the SmartCardControl interface as an SmartCardControl.Control func
type SmartCardControlFunc func(ctx context.Context, code uint16, data []byte) ([]byte, error)

func (f SmartCardControlFunc) Control(ctx context.Context, code uint16, data []byte) ([]byte, error) {
	return f(ctx, code, data)
}

// RawSmartCardControlToSmartCardControl transforms RawSmartCardControl into an SmartCardControl with the ability to
// abandon waiting for a response if the context is cancelled
func RawSmartCardControlToSmartCardControl(sc RawSmartCardControl, controlCodeToIoctl func(code uint16) uint32) SmartCardControl {
	return SmartCardControlFunc(func(ctx context.Context, code uint16, data []byte) ([]byte, error) {
		return Escapable(ctx, func(ctx context.Context) ([]byte, error) {
			return sc.Control(controlCodeToIoctl(code), data)
		})
	})
}

// SmartCardControlAPDUer transforms SmartCardControl into an ExchangerAPDUer by performing control functions with the
// given code.
func SmartCardControlAPDUer(sc SmartCardControl, code uint16) ExchangerAPDUer {
	return ExchangerFunc(func(ctx context.Context, capdu []byte) ([]byte, error) {
		return sc.Control(ctx, code, capdu)
	})
}
