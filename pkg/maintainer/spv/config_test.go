package spv

import (
	"testing"
)

// TestConfigValidate_MaxProofHeaders guards against a zero proof-header bound
// reaching the proof assembly loop.
//
// getProofInfo evaluates `headerCount >= maxProofHeaders` at loop entry with
// headerCount starting at zero, so a zero bound returns
// proofSkipExceededMaxHeaders on the first iteration for every transaction.
// That silently disables all SPV proving while logging each transaction as
// "may be permanently unprovable", which reads as a chain condition rather
// than a misconfiguration. The flag default only covers omission, so the zero
// value has to be rejected explicitly.
func TestConfigValidate_MaxProofHeaders(t *testing.T) {
	tests := map[string]struct {
		maxProofHeaders uint
		expectError     bool
	}{
		"zero is rejected": {
			maxProofHeaders: 0,
			expectError:     true,
		},
		"default is accepted": {
			maxProofHeaders: DefaultMaxProofHeaders,
			expectError:     false,
		},
		"one is accepted": {
			maxProofHeaders: 1,
			expectError:     false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			config := Config{MaxProofHeaders: test.maxProofHeaders}

			err := config.Validate()

			if test.expectError && err == nil {
				t.Errorf(
					"expected validation to reject maxProofHeaders [%d], got no error",
					test.maxProofHeaders,
				)
			}
			if !test.expectError && err != nil {
				t.Errorf(
					"expected maxProofHeaders [%d] to be accepted, got error: [%v]",
					test.maxProofHeaders,
					err,
				)
			}
		})
	}
}
