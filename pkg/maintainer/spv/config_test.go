package spv

import (
	"context"
	"testing"
)

// TestConfig_Validate guards against a zero proof-header bound reaching the
// proof assembly loop.
//
// getProofInfo evaluates `headerCount >= maxProofHeaders` at loop entry with
// headerCount starting at zero, so a zero bound returns
// proofSkipExceededMaxHeaders on the first iteration for every transaction.
// That silently disables all SPV proving while logging each transaction as
// "may be permanently unprovable", which reads as a chain condition rather
// than a misconfiguration. The flag default only covers omission, so the
// zero value has to be rejected explicitly.
func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config    Config
		expectErr bool
	}{
		"zero maxProofHeaders is rejected": {
			config: Config{
				MaxProofHeaders: 0,
			},
			expectErr: true,
		},
		"default positive maxProofHeaders is accepted": {
			config: Config{
				MaxProofHeaders: DefaultMaxProofHeaders,
			},
			expectErr: false,
		},
		"custom positive maxProofHeaders is accepted": {
			config: Config{
				MaxProofHeaders: 288,
			},
			expectErr: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := test.config.Validate()

			if test.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestInitialize_InvalidConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := Config{
		MaxProofHeaders: 0,
	}

	err := Initialize(
		ctx,
		config,
		nil,
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected Initialize to fail on invalid config, got nil")
	}
}
