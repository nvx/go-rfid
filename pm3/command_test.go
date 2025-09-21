package pm3

import (
	"bytes"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestCommand_WriteTo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	_, err := CommandEnterStandalone.WriteTo(&buf)
	require.NoError(t, err)
	out := strings.ToUpper(hex.EncodeToString(buf.Bytes()))
	assert.Equal(t, "504D336101801501016133", out)
}
