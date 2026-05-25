package tbtc

import "testing"

func TestFrostDKGSignatureThresholdUsesHonestThreshold(t *testing.T) {
	params := &GroupParameters{
		GroupSize:       100,
		GroupQuorum:     90,
		HonestThreshold: 51,
	}

	threshold, err := frostDKGSignatureThreshold(params)
	if err != nil {
		t.Fatalf("unexpected threshold error: [%v]", err)
	}
	if threshold != 51 {
		t.Fatalf("unexpected threshold\nexpected: [51]\nactual:   [%d]", threshold)
	}
}

func TestFrostDKGSignatureThresholdRejectsInvalidParameters(t *testing.T) {
	testCases := map[string]*GroupParameters{
		"nil":              nil,
		"zero threshold":   {GroupSize: 100, GroupQuorum: 90, HonestThreshold: 0},
		"above group size": {GroupSize: 3, GroupQuorum: 3, HonestThreshold: 4},
	}

	for name, params := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := frostDKGSignatureThreshold(params)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBoundedFrostDKGRecoveryStartBlock(t *testing.T) {
	lookBackBlocks := uint64(13560)

	testCases := map[string]struct {
		currentBlock uint64
		expected     uint64
	}{
		"below lookback": {
			currentBlock: 100,
			expected:     0,
		},
		"equal lookback": {
			currentBlock: lookBackBlocks,
			expected:     0,
		},
		"one block above lookback": {
			currentBlock: lookBackBlocks + 1,
			expected:     1,
		},
		"above lookback": {
			currentBlock: lookBackBlocks + 123,
			expected:     123,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := boundedFrostDKGRecoveryStartBlock(
				test.currentBlock,
				lookBackBlocks,
			)
			if actual != test.expected {
				t.Fatalf(
					"unexpected start block\nexpected: [%d]\nactual:   [%d]",
					test.expected,
					actual,
				)
			}
		})
	}
}

func TestFrostDKGRecoveryLookBackBlocksCoversFullLifecycle(t *testing.T) {
	params := &DKGParameters{
		SubmissionTimeoutBlocks:       500,
		ChallengePeriodBlocks:         11520,
		ApprovePrecedencePeriodBlocks: 20,
	}
	groupParameters := &GroupParameters{
		GroupSize:       100,
		GroupQuorum:     90,
		HonestThreshold: 51,
	}

	actual, err := frostDKGRecoveryLookBackBlocks(params, groupParameters)
	if err != nil {
		t.Fatalf("unexpected lookback error: [%v]", err)
	}

	expected := uint64(500 + 11520 + 20 + 100*dkgResultApprovalDelayStepBlocks + dkgStartedConfirmationBlocks)
	if actual != expected {
		t.Fatalf(
			"unexpected lookback\nexpected: [%d]\nactual:   [%d]",
			expected,
			actual,
		)
	}
}
