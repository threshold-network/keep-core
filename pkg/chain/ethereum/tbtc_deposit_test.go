package ethereum

import (
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// Test data based on: https://etherscan.io/tx/0x97c7a293127a604da77f7ef8daf4b19da2bf04327dd891b6d717eaef89bd8bca
func TestBuildDepositKey(t *testing.T) {
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputIndex := uint32(1)

	depositKey := buildDepositKey(fundingTxHash, fundingOutputIndex)

	expectedDepositKey := "3e84c1ea6aeaf2f45fb49623a88affe653b798ea6f675805acc0ec3965b6f317"
	testutils.AssertStringsEqual(
		t,
		"deposit key",
		expectedDepositKey,
		depositKey.Text(16),
	)
}
