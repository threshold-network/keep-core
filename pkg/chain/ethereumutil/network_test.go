package ethereum

import (
	"testing"
)

func TestNetworkString(t *testing.T) {
	var tests = map[string]struct {
		network        Network
		expectedString string
	}{
		"Unknown": {
			network:        Unknown,
			expectedString: "unknown",
		},
		"Mainnet": {
			network:        Mainnet,
			expectedString: "mainnet",
		},
		"Sepolia": {
			network:        Sepolia,
			expectedString: "sepolia",
		},
		"Developer": {
			network:        Developer,
			expectedString: "developer",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			result := test.network.String()

			if result != test.expectedString {
				t.Errorf(
					"\nexpected: %s\nactual:   %s",
					test.expectedString,
					result,
				)
			}
		})
	}
}
