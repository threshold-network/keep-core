package tbtc

import "fmt"

func frostDKGSignatureThreshold(groupParameters *GroupParameters) (int, error) {
	if groupParameters == nil {
		return 0, fmt.Errorf("group parameters are nil")
	}

	// The on-chain FROST validator names this value groupThreshold. In keep-core
	// group parameters, that is the honest signing threshold, not the ECDSA DKG
	// active-participant quorum.
	threshold := groupParameters.HonestThreshold
	if threshold <= 0 {
		return 0, fmt.Errorf("FROST DKG signature threshold must be positive")
	}
	if threshold > groupParameters.GroupSize {
		return 0, fmt.Errorf(
			"FROST DKG signature threshold [%d] exceeds group size [%d]",
			threshold,
			groupParameters.GroupSize,
		)
	}

	return threshold, nil
}
