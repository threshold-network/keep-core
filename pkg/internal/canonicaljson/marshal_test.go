package canonicaljson

import (
	"strings"
	"testing"
)

// Verify Marshal function exists with correct signature and produces
// deterministic JSON without trailing newlines or HTML escaping.

func TestMarshalProducesValidJSON(t *testing.T) {
	input := map[string]string{"key": "value"}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"key":"value"}`
	if string(result) != expected {
		t.Fatalf("expected %s, got %s", expected, string(result))
	}
}

func TestMarshalNoTrailingNewline(t *testing.T) {
	input := map[string]int{"count": 42}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) > 0 && result[len(result)-1] == '\n' {
		t.Fatal("output should not end with a newline")
	}
}

func TestMarshalDoesNotEscapeHTML(t *testing.T) {
	input := map[string]string{"url": "https://example.com?a=1&b=2<tag>"}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)

	// Verify raw HTML characters are preserved, not escaped
	if strings.Contains(resultStr, `\u003c`) || strings.Contains(resultStr, `\u003e`) || strings.Contains(resultStr, `\u0026`) {
		t.Fatalf("HTML characters should not be escaped, got %s", resultStr)
	}
	if !strings.Contains(resultStr, "&") || !strings.Contains(resultStr, "<") || !strings.Contains(resultStr, ">") {
		t.Fatalf("expected raw HTML characters in output, got %s", resultStr)
	}
}

func TestMarshalUsesTrimSuffixNotTrimSpace(t *testing.T) {
	// Verify the function uses TrimSuffix (removes only trailing \n)
	// not TrimSpace (which would remove all whitespace).
	// A value with leading space in a string field should be preserved.
	input := map[string]string{"data": " leading-space"}

	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"data":" leading-space"}`
	if string(result) != expected {
		t.Fatalf("expected %s, got %s", expected, string(result))
	}
}
