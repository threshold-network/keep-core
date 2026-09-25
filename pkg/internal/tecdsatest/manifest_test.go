package tecdsatest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Required fixture files in pkg/internal/tecdsatest/testdata.
// These slices explicitly enumerate all fixture files required by the test
// suite. Directory globbing is avoided because a glob that matches nothing
// would pass silently.
var (
	requiredPrivateKeyShareFixtures = []string{
		"private_key_share_data_0.json",
		"private_key_share_data_1.json",
		"private_key_share_data_2.json",
		"private_key_share_data_3.json",
		"private_key_share_data_4.json",
	}
)

// TestRequiredFixturesPresent guards the test fixtures in testdata against
// accidental deletion or corruption.
func TestRequiredFixturesPresent(t *testing.T) {
	_, callerFileName, _, _ := runtime.Caller(0)
	testDataDir := filepath.Join(filepath.Dir(callerFileName), "testdata")

	tests := []struct {
		name     string
		required []string
		load     func() (int, error)
	}{
		{
			name:     "PrivateKeyShare",
			required: requiredPrivateKeyShareFixtures,
			load: func() (int, error) {
				shares, err := LoadPrivateKeyShareTestFixtures(
					len(requiredPrivateKeyShareFixtures),
				)
				return len(shares), err
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
				t.Fatal("loader returned 0 fixtures")
			}

			if count != len(tc.required) {
				t.Errorf(
					"unexpected fixture count: got %d, want %d",
					count,
					len(tc.required),
				)
			}
		})
	}
}
