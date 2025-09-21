package rfid

import (
	"context"
	"github.com/ansel1/merry/v2"
)

// DeferWrap is called by library functions when returning errors to enrich them with stack trace information etc
// defaults to wrapping with merry but can be overridden if another library is preferred
// where a context is not available context.Background() will be used
var DeferWrap = func(ctx context.Context, err *error) {
	if err != nil {
		*err = merry.WrapSkipping(*err, 1)
	}
}
