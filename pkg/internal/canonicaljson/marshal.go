// Package canonicaljson provides deterministic JSON marshaling without
// trailing newlines or HTML escaping.
package canonicaljson

import (
	"bytes"
	"encoding/json"
)

// Marshal encodes the given value as JSON without HTML escaping and strips
// the trailing newline that json.Encoder.Encode appends. The result is
// suitable for hashing where byte-level determinism matters.
func Marshal(v any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}
