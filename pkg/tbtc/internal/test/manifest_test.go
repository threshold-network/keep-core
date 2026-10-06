package test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Required fixture files in pkg/tbtc/internal/test/testdata.
// These slices explicitly enumerate all fixture files required by the test
// suite. Directory globbing is avoided because a glob that matches nothing
// would pass silently.
var (
	requiredDepositSweepFixtures = []string{
		"deposit_sweep_scenario_0.json",
		"deposit_sweep_scenario_1.json",
		"deposit_sweep_scenario_2.json",
		"deposit_sweep_scenario_3.json",
		"deposit_sweep_scenario_4.json",
	}

	requiredRedemptionFixtures = []string{
		"redemption_scenario_0.json",
		"redemption_scenario_1.json",
		"redemption_scenario_2.json",
		"redemption_scenario_3.json",
		"redemption_scenario_4.json",
		"redemption_scenario_5.json",
	}

	requiredMovingFundsFixtures = []string{
		"moving_funds_scenario_0.json",
		"moving_funds_scenario_1.json",
		"moving_funds_scenario_2.json",
	}

	requiredMovedFundsSweepFixtures = []string{
		"moved_funds_sweep_scenario_0.json",
		"moved_funds_sweep_scenario_1.json",
	}
)

// TestRequiredFixturesPresent guards the test fixtures in testdata against
// accidental deletion or omission.
func TestRequiredFixturesPresent(t *testing.T) {
	_, callerFileName, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(callerFileName), "testdata")

	tests := []struct {
		name     string
		required []string
		load     func() (int, error)
	}{
		{
			name:     "DepositSweep",
			required: requiredDepositSweepFixtures,
			load: func() (int, error) {
				scenarios, err := LoadDepositSweepTestScenarios()
				return len(scenarios), err
			},
		},
		{
			name:     "Redemption",
			required: requiredRedemptionFixtures,
			load: func() (int, error) {
				scenarios, err := LoadRedemptionTestScenarios()
				return len(scenarios), err
			},
		},
		{
			name:     "MovingFunds",
			required: requiredMovingFundsFixtures,
			load: func() (int, error) {
				scenarios, err := LoadMovingFundsTestScenarios()
				return len(scenarios), err
			},
		},
		{
			name:     "MovedFundsSweep",
			required: requiredMovedFundsSweepFixtures,
			load: func() (int, error) {
				scenarios, err := LoadMovedFundsSweepTestScenarios()
				return len(scenarios), err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.required) == 0 {
				t.Fatal("manifest is empty")
			}

			for _, fileName := range tc.required {
				filePath := filepath.Join(testDataDir, fileName)
				if _, err := os.Stat(filePath); err != nil {
					t.Errorf("missing required fixture file [%s]: %v", fileName, err)
				}
			}

			count, err := tc.load()
			if err != nil {
				t.Fatalf("loader failed: %v", err)
			}

			if count == 0 {
				t.Fatal("loader returned 0 scenarios")
			}

			if count != len(tc.required) {
				t.Errorf(
					"unexpected scenario count: got %d, want %d",
					count,
					len(tc.required),
				)
			}
		})
	}
}
