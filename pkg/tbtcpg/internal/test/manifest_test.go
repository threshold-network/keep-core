package test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Required fixture files in pkg/tbtcpg/internal/test/testdata.
// These slices explicitly enumerate all fixture files required by the test
// suite. Directory globbing is avoided because a glob that matches nothing
// would pass silently.
var (
	requiredFindDepositsFixtures = []string{
		"find_deposits_scenario_0.json",
		"find_deposits_scenario_1.json",
		"find_deposits_scenario_2.json",
	}

	requiredProposeSweepFixtures = []string{
		"propose_sweep_scenario_0.json",
		"propose_sweep_scenario_1.json",
		"propose_sweep_scenario_2.json",
	}

	requiredFindPendingRedemptionsFixtures = []string{
		"find_pending_redemptions_scenario_0.json",
		"find_pending_redemptions_scenario_1.json",
		"find_pending_redemptions_scenario_2.json",
		"find_pending_redemptions_scenario_3.json",
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
			name:     "FindDeposits",
			required: requiredFindDepositsFixtures,
			load: func() (int, error) {
				scenarios, err := LoadFindDepositsToSweepTestScenario()
				return len(scenarios), err
			},
		},
		{
			name:     "ProposeSweep",
			required: requiredProposeSweepFixtures,
			load: func() (int, error) {
				scenarios, err := LoadProposeSweepTestScenario()
				return len(scenarios), err
			},
		},
		{
			name:     "FindPendingRedemptions",
			required: requiredFindPendingRedemptionsFixtures,
			load: func() (int, error) {
				scenarios, err := LoadFindPendingRedemptionsTestScenario()
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
