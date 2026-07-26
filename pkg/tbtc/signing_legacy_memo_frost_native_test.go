//go:build frost_native

package tbtc

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/signing"
)

// Two legacy wallets heartbeat-signing the same proposal at the same start
// block derive IDENTICAL stable ROAST session IDs: the wallet-disambiguating
// key-group fold is empty for legacy material. The interactive aggregate memo
// registry rejects a duplicate live owner, so if legacy wallets claimed memo
// ownership, the second wallet's signing would fail outright in frost-native
// builds. The executor therefore gates memo ownership on
// usesSchnorrSignatures(); this test pins both halves of that reasoning.
func TestLegacyHeartbeatWallets_DoNotContendForAggregateMemoOwnership(
	t *testing.T,
) {
	message := new(big.Int).SetBytes([]byte{0xff, 0xff, 0x01})
	firstWalletSID := roastSessionIDWithAuthorization(message, nil, 0, "", nil)
	secondWalletSID := roastSessionIDWithAuthorization(message, nil, 0, "", nil)
	if firstWalletSID != secondWalletSID {
		t.Fatal("legacy wallets signing the same message at the same start " +
			"block must derive one roast session ID for this scenario")
	}

	// The registry refuses a duplicate live owner: this is exactly the
	// failure two concurrently heartbeating legacy wallets would hit if
	// they began memo ownership.
	first, err := signing.BeginInteractiveAggregateMemoSession(firstWalletSID)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := signing.BeginInteractiveAggregateMemoSession(
		secondWalletSID,
	); err == nil {
		t.Fatal("duplicate live memo ownership was accepted")
	}

	// The executor's gate keeps legacy wallets away from that contention:
	// scaffold-era tECDSA signing material must never read as Schnorr, so
	// the signing executor never begins interactive aggregate memo
	// ownership for it.
	executor := setupSigningExecutor(t)
	if executor.usesSchnorrSignatures() {
		t.Fatal("legacy tECDSA executor unexpectedly reports Schnorr signatures")
	}
}
