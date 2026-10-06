package spv

import (
	"context"
	"testing"
)

// TestConfig_Validate guards against silently-degrading zero settings
// reaching the maintainer's runtime. Each field is checked for the
// misconfiguration that looks like a valid value but renders the maintainer
// inert: a zero MaxProofHeaders makes getProofInfo skip every transaction on
// the first loop iteration; a zero HistoryDepth anchors the event search at
// the current chain tip; a non-positive TransactionLimit returns no
// transactions to prove; zero backoff times remove the pauses between
// control-loop iterations. The flag/file defaults only cover omission, so an
// operator's explicit zero has to be rejected at startup.
func TestConfig_Validate(t *testing.T) {
	validConfig := Config{
		HistoryDepth:       DefaultHistoryDepth,
		TransactionLimit:   DefaultTransactionLimit,
		RestartBackoffTime: DefaultRestartBackoffTime,
		IdleBackoffTime:    DefaultIdleBackOffTime,
		MaxProofHeaders:    DefaultMaxProofHeaders,
	}

	tests := map[string]struct {
		config    Config
		expectErr bool
	}{
		"valid config is accepted": {
			config:    validConfig,
			expectErr: false,
		},
		"zero maxProofHeaders is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.MaxProofHeaders = 0 }),
			expectErr: true,
		},
		"custom positive maxProofHeaders is accepted": {
			config:    withCustomMaxProofHeaders(validConfig, 288),
			expectErr: false,
		},
		"zero historyDepth is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.HistoryDepth = 0 }),
			expectErr: true,
		},
		"zero transactionLimit is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.TransactionLimit = 0 }),
			expectErr: true,
		},
		"negative transactionLimit is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.TransactionLimit = -1 }),
			expectErr: true,
		},
		"zero restartBackoffTime is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.RestartBackoffTime = 0 }),
			expectErr: true,
		},
		"zero idleBackoffTime is rejected": {
			config:    withZeroField(validConfig, func(c *Config) { c.IdleBackoffTime = 0 }),
			expectErr: true,
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

// withZeroField returns a copy of base with one field mutated to its invalid
// value; used to build single-field-invalid configs for the table test.
func withZeroField(base Config, mutate func(*Config)) Config {
	c := base
	mutate(&c)
	return c
}

func withCustomMaxProofHeaders(base Config, value uint) Config {
	c := base
	c.MaxProofHeaders = value
	return c
}

// TestInitialize_InvalidConfig guards the public entry point: Initialize must
// reject a misconfigured config before touching any chain interface.
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
