package ethereum

import (
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestComputeMovingFundsCommitmentHash(t *testing.T) {
	toByte20 := func(s string) [20]byte {
		bytes, err := hex.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}

		if len(bytes) != 20 {
			t.Fatal("incorrect hexstring length")
		}

		var result [20]byte
		copy(result[:], bytes[:])
		return result
	}

	targetWallets := [][20]byte{
		toByte20("4b440cb29c80c3f256212d8fdd4f2125366f3c91"),
		toByte20("888f01315e0268bfa05d5e522f8d63f6824d9a96"),
		toByte20("b2a89e53a4227dbe530a52a1c419040735fa636c"),
	}

	movingFundsCommitmentHash := computeMovingFundsCommitmentHash(
		targetWallets,
	)

	expectedMovingFundsCommitmentHash, err := hex.DecodeString(
		"8ba62d1d754a3429e2ff1fb4f523b5fad2b605c873a2968bb5985a625eb96202",
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBytesEqual(
		t,
		expectedMovingFundsCommitmentHash,
		movingFundsCommitmentHash[:],
	)
}

func TestBuildMovedFundsKey(t *testing.T) {
	fundingTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputIndex := uint32(2)

	movedFundsKey := buildMovedFundsKey(fundingTxHash, fundingOutputIndex)

	expectedMovedFundsKey := "24509b8a853476ebe77af3707bd7ce017d527680e941b6eeaac2d5b712df4f8d"
	testutils.AssertStringsEqual(
		t,
		"moved funds key",
		expectedMovedFundsKey,
		movedFundsKey.Text(16),
	)
}
