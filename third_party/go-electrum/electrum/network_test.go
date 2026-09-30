package electrum

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApiErrUnmarshalNonNumericCode regression-tests a server response whose
// error.code is not a JSON number (e.g. electrs/esplora sending a string
// error code instead of the JSON-RPC 2.0 integer). This previously panicked
// on an unchecked type assertion in apiErr.UnmarshalJSON.
func TestApiErrUnmarshalNonNumericCode(t *testing.T) {
	raw := []byte(`{"error":{"code":"RPC_ERROR"}}`)

	var msg response
	require.NotPanics(t, func() {
		require.NoError(t, json.Unmarshal(raw, &msg))
	})

	require.NotNil(t, msg.Error)
	require.Zero(t, msg.Error.Code, "code must stay zero when the server sends a non-numeric value")
	require.NotEmpty(t, msg.Error.Error(), "the resulting error must still be usable")
}
