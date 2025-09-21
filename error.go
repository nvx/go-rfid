package rfid

import (
	"context"
)

// DeferWrap is called by library functions when returning errors to enrich them with stack trace information.
// By default, this is a no-op but exists so consumers of the library can BYO their own library
// If context is not available in a given function context.Background() will be used
var DeferWrap = func(ctx context.Context, err *error) {}
