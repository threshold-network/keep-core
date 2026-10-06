package maintainer

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
)

func TestConfig_Validate(t *testing.T) {
	validSpv := spv.Config{
		HistoryDepth:       spv.DefaultHistoryDepth,
		TransactionLimit:   spv.DefaultTransactionLimit,
		RestartBackoffTime: spv.DefaultRestartBackoffTime,
		IdleBackoffTime:    spv.DefaultIdleBackOffTime,
		MaxProofHeaders:    spv.DefaultMaxProofHeaders,
	}

	tests := map[string]struct {
		config    Config
		expectErr bool
	}{
		"launch-all with zero SPV maxProofHeaders fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            false,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    0,
				},
			},
			expectErr: true,
		},
		"enabled SPV with zero maxProofHeaders fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    0,
				},
			},
			expectErr: true,
		},
		"difficulty-only with zero SPV maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: true},
				Spv: spv.Config{
					Enabled:         false,
					MaxProofHeaders: 0,
				},
			},
			expectErr: false,
		},
		"launch-all with positive SPV maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv:               validSpv,
			},
			expectErr: false,
		},
		"enabled SPV with positive maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    spv.DefaultMaxProofHeaders,
				},
			},
			expectErr: false,
		},
		"enabled SPV with zero historyDepth fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       0,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    spv.DefaultMaxProofHeaders,
				},
			},
			expectErr: true,
		},
		"enabled SPV with non-positive transactionLimit fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   0,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    spv.DefaultMaxProofHeaders,
				},
			},
			expectErr: true,
		},
		"enabled SPV with zero restartBackoffTime fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: 0,
					IdleBackoffTime:    spv.DefaultIdleBackOffTime,
					MaxProofHeaders:    spv.DefaultMaxProofHeaders,
				},
			},
			expectErr: true,
		},
		"enabled SPV with zero idleBackoffTime fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv: spv.Config{
					Enabled:            true,
					HistoryDepth:       spv.DefaultHistoryDepth,
					TransactionLimit:   spv.DefaultTransactionLimit,
					RestartBackoffTime: spv.DefaultRestartBackoffTime,
					IdleBackoffTime:    0,
					MaxProofHeaders:    spv.DefaultMaxProofHeaders,
				},
			},
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
