package canonicaljson

import (
	"bytes"
	"strings"
	"testing"
)

// testPayload mirrors the signerApprovalCertificateSignerSetPayload struct
// pattern used in production code, with string and int fields using camelCase
// JSON tags.
type testPayload struct {
	WalletID  string `json:"walletId"`
	Threshold int    `json:"threshold"`
}

func TestMarshal_MapInput(t *testing.T) {
	input := map[string]any{
		"name":   "test",
		"active": true,
		"count":  3,
	}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Keys must be alphabetically sorted by json.Encoder.
	expected := `{"active":true,"count":3,"name":"test"}`
	if string(result) != expected {
		t.Fatalf(
			"unexpected result\nexpected: %s\nactual:   %s",
			expected,
			string(result),
		)
	}
}

func TestMarshal_StructInput(t *testing.T) {
	input := testPayload{
		WalletID:  "0xabc",
		Threshold: 51,
	}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// JSON tags determine field names; struct declaration order is preserved.
	expected := `{"walletId":"0xabc","threshold":51}`
	if string(result) != expected {
		t.Fatalf(
			"unexpected result\nexpected: %s\nactual:   %s",
			expected,
			string(result),
		)
	}
}

func TestMarshal_NoTrailingNewline(t *testing.T) {
	input := map[string]int{"count": 42}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.HasSuffix(result, []byte("\n")) {
		t.Fatalf("output ends with trailing newline: %q", result)
	}
}

func TestMarshal_HTMLNotEscaped(t *testing.T) {
	input := map[string]string{"html": "<b>bold</b> & fun > safe"}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(result)

	// Verify Unicode escape sequences are absent.
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(output, escaped) {
			t.Fatalf("found unwanted escape %s in output: %s", escaped, output)
		}
	}

	// Verify raw HTML characters are present.
	for _, raw := range []string{"<", ">", "&"} {
		if !strings.Contains(output, raw) {
			t.Fatalf("missing raw character %q in output: %s", raw, output)
		}
	}
}

func TestMarshal_DeterministicMapOrder(t *testing.T) {
	input := map[string]string{
		"zebra":  "last",
		"alpha":  "first",
		"middle": "center",
	}

	first, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify alphabetical key ordering.
	expected := `{"alpha":"first","middle":"center","zebra":"last"}`
	if string(first) != expected {
		t.Fatalf(
			"unexpected key order\nexpected: %s\nactual:   %s",
			expected,
			string(first),
		)
	}

	// Verify repeated calls produce byte-identical output.
	for i := 0; i < 10; i++ {
		result, err := Marshal(input)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if !bytes.Equal(first, result) {
			t.Fatalf(
				"iteration %d: output differs\nexpected: %s\nactual:   %s",
				i, first, result,
			)
		}
	}
}
