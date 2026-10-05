package ethereum

import "testing"

func TestGasEstimateWithMargin(t *testing.T) {
	tests := map[string]struct {
		gasEstimate uint64
		expected    uint64
	}{
		"zero":           {gasEstimate: 0, expected: 0},
		"one unit":       {gasEstimate: 1, expected: 2},
		"two units":      {gasEstimate: 2, expected: 3},
		"three units":    {gasEstimate: 3, expected: 4},
		"four units":     {gasEstimate: 4, expected: 5},
		"five units":     {gasEstimate: 5, expected: 6},
		"six units":      {gasEstimate: 6, expected: 8},
		"thirteen units": {gasEstimate: 13, expected: 16},
		"retarget gas":   {gasEstimate: 54000, expected: 64800},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actual := gasEstimateWithMargin(test.gasEstimate)
			if actual != test.expected {
				t.Errorf(
					"unexpected result for estimate [%v]\nexpected: [%v]\nactual:   [%v]",
					test.gasEstimate,
					test.expected,
					actual,
				)
			}
		})
	}
}
