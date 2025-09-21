package rfid

import (
	"encoding/hex"
	"log/slog"
	"strings"
)

func LogHex(key string, value []byte) slog.Attr {
	return slog.String(key, strings.ToUpper(hex.EncodeToString(value)))
}

var ErrorAttrs = func(err error) slog.Attr {
	return slog.String("error", err.Error())
}
