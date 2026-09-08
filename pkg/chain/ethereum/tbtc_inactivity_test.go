package ethereum

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

func TestCalculateInactivityClaimHash(t *testing.T) {
	chainID := big.NewInt(31337)
	nonce := big.NewInt(3)

	walletPublicKey, err := hex.DecodeString(
		"9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c473e" +
			"661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
	)
	if err != nil {
		t.Fatal(err)
	}

	inactiveMembersIndexes := []*big.Int{
		big.NewInt(1), big.NewInt(2), big.NewInt(30),
	}

	heartbeatFailed := true

	hash, err := calculateInactivityClaimHash(
		chainID,
		nonce,
		walletPublicKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := "f3210008cba186e90386a1bd0c63b6f29a67666f632350be22ce63ab39fc506e"

	testutils.AssertStringsEqual(
		t,
		"hash",
		expectedHash,
		hex.EncodeToString(hash[:]),
	)
}
