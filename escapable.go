package rfid

import (
	"context"
)

// Escapable runs the given func in a goroutine and waits for the response.
// This is useful to convert non-context aware blocking functions to be context aware with the caveat that cancellation
// will not abort the non-context aware function but does enabling returning early.
// If the context is cancelled it returns immediately with the context.Cause abandoning the goroutine
// If the context is already cancelled  it returns immediately without invoking the func.
func Escapable[T any](ctx context.Context, fn func(context.Context) (T, error)) (_ T, err error) {
	defer DeferWrap(ctx, &err)

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	ch := make(chan struct{})

	var res T
	var rerr error // separate err variable otherwise writing to err is racey if the context is cancelled
	go func() {
		res, rerr = fn(ctx)
		close(ch)
	}()

	select {
	case <-ch:
		return res, rerr
	case <-ctx.Done():
		err = context.Cause(ctx)
		return
	}
}
