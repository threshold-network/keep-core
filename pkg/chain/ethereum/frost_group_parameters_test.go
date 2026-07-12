package ethereum

import (
	"math/big"
	"testing"
)

func TestWalletGroupParametersFromValidatorValuesMapsFrostConstants(
	t *testing.T,
) {
	parameters, err := walletGroupParametersFromValidatorValues(
		big.NewInt(5), // FrostDkgValidator.groupSize()
		big.NewInt(4), // FrostDkgValidator.activeThreshold()
		big.NewInt(3), // FrostDkgValidator.groupThreshold()
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if parameters.GroupSize != 5 {
		t.Fatalf("unexpected group size: [%d]", parameters.GroupSize)
	}
	if parameters.GroupQuorum != 4 {
		t.Fatalf("unexpected group quorum: [%d]", parameters.GroupQuorum)
	}
	if parameters.HonestThreshold != 3 {
		t.Fatalf("unexpected honest threshold: [%d]", parameters.HonestThreshold)
	}
}
