package maintainer

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
)

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config    Config
		expectErr bool
	}{
		"launch-all with zero SPV maxProofHeaders fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv:               spv.Config{Enabled: false, MaxProofHeaders: 0},
			},
			expectErr: true,
		},
		"enabled SPV with zero maxProofHeaders fails": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv:               spv.Config{Enabled: true, MaxProofHeaders: 0},
			},
			expectErr: true,
		},
		"difficulty-only with zero SPV maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: true},
				Spv:               spv.Config{Enabled: false, MaxProofHeaders: 0},
			},
			expectErr: false,
		},
		"launch-all with positive SPV maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv:               spv.Config{Enabled: false, MaxProofHeaders: spv.DefaultMaxProofHeaders},
			},
			expectErr: false,
		},
		"enabled SPV with positive maxProofHeaders passes": {
			config: Config{
				BitcoinDifficulty: btcdiff.Config{Enabled: false},
				Spv:               spv.Config{Enabled: true, MaxProofHeaders: spv.DefaultMaxProofHeaders},
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
