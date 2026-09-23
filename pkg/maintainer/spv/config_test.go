package spv

import (
	"context"
	"testing"
)

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
