//go:build frost_native

package signing

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestHexBytesRoundTripsWithoutString pins the wire contract: the encoding must
// stay byte-identical to the hex-string form the signer expects, in both
// directions, so switching the secret-bearing fields to []byte cannot change the
// FFI payload.
func TestHexBytesRoundTripsWithoutString(t *testing.T) {
	secret := hexBytes{0x00, 0x01, 0xfe, 0xff, 0x7f, 0x80}

	encoded, err := json.Marshal(struct {
		Secret hexBytes `json:"secret_package_hex"`
	}{secret})
	if err != nil {
		t.Fatal(err)
	}

	const want = `{"secret_package_hex":"0001feff7f80"}`
	if string(encoded) != want {
		t.Fatalf("expected [%s], got [%s]", want, encoded)
	}

	var decoded struct {
		Secret hexBytes `json:"secret_package_hex"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Secret, secret) {
		t.Fatalf("expected [%x], got [%x]", secret, decoded.Secret)
	}
}

// TestHexBytesZeroingIsObservable is the reason the type exists: the decoded
// secret must live in memory the caller can overwrite. A string field could not
// satisfy this.
func TestHexBytesZeroingIsObservable(t *testing.T) {
	var decoded struct {
		Secret hexBytes `json:"secret_package_hex"`
	}
	if err := json.Unmarshal(
		[]byte(`{"secret_package_hex":"deadbeef"}`),
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	backing := decoded.Secret
	for i := range backing {
		backing[i] = 0
	}
	if !bytes.Equal(decoded.Secret, []byte{0, 0, 0, 0}) {
		t.Fatalf("expected the decoded secret to be zeroable in place, got [%x]", decoded.Secret)
	}
}

func TestHexBytesRejectsMalformedInput(t *testing.T) {
	for name, input := range map[string]string{
		"odd length":   `"abc"`,
		"non-hex":      `"zz"`,
		"not a string": `123`,
		"bare object":  `{}`,
		"unterminated": `"ab`,
	} {
		t.Run(name, func(t *testing.T) {
			var h hexBytes
			if err := json.Unmarshal([]byte(input), &h); err == nil {
				t.Fatalf("expected [%s] to be rejected, decoded to [%x]", input, h)
			}
		})
	}

	// JSON null is a legitimate absent value, not a malformed one.
	var h hexBytes
	if err := json.Unmarshal([]byte("null"), &h); err != nil {
		t.Fatalf("expected null to be accepted as absent, got [%v]", err)
	}
	if h != nil {
		t.Fatalf("expected null to decode to nil, got [%x]", h)
	}
}

// TestHexBytesErrorOmitsSecretValue keeps secret material out of logs: these
// fields carry key packages, so a decode failure must not echo the value.
func TestHexBytesErrorOmitsSecretValue(t *testing.T) {
	var h hexBytes
	err := json.Unmarshal([]byte(`"c0ffeeZZ"`), &h)
	if err == nil {
		t.Fatal("expected malformed hex to fail")
	}
	if strings.Contains(err.Error(), "c0ffee") {
		t.Fatalf("error must not echo the field value, got [%v]", err)
	}
}
