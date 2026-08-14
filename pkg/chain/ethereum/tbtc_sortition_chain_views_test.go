package ethereum

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"

	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
)

// The pool-monitoring defect these views fix is a WRONG-POOL binding: a single
// hasFrostAuthorization()-switched monitor maintained the FROST pool while
// labeled "legacy ECDSA" (and, post-cutover, nothing). These tests pin each view
// to its intended pool/registry so a regression that crosses the wiring fails.

func newTbtcChainForSortitionViewTest(withFrost bool) (
	tc *TbtcChain,
	ecdsaPool *ecdsacontract.EcdsaSortitionPool,
	frostPool *ecdsacontract.EcdsaSortitionPool,
	operatorAddress common.Address,
) {
	operatorAddress = common.HexToAddress(
		"0x1111111111111111111111111111111111111111",
	)
	ecdsaPool = &ecdsacontract.EcdsaSortitionPool{}
	frostPool = &ecdsacontract.EcdsaSortitionPool{}

	tc = &TbtcChain{
		baseChain: &baseChain{
			key: &keystore.Key{Address: operatorAddress},
		},
		walletRegistry: &ecdsacontract.WalletRegistry{},
		sortitionPool:  ecdsaPool,
	}
	if withFrost {
		tc.frostWalletRegistry = &frostabi.FrostWalletRegistry{}
		tc.frostSortitionPool = frostPool
	}

	return tc, ecdsaPool, frostPool, operatorAddress
}

func TestLegacyECDSASortitionChainBindsToECDSAPool(t *testing.T) {
	// Even with FROST configured, the legacy view must bind to the ECDSA pool --
	// this is the exact crossing the switched monitor got wrong.
	tc, ecdsaPool, frostPool, operatorAddress := newTbtcChainForSortitionViewTest(true)

	view := tc.LegacyECDSASortitionChain()

	ecdsaView, ok := view.(*ecdsaSortitionChain)
	if !ok {
		t.Fatalf("expected *ecdsaSortitionChain, got [%T]", view)
	}
	if ecdsaView.pool != ecdsaPool {
		t.Fatal("legacy view must target the ECDSA sortition pool")
	}
	if ecdsaView.pool == frostPool {
		t.Fatal("legacy view must NOT target the FROST sortition pool")
	}
	if ecdsaView.walletRegistry != tc.walletRegistry {
		t.Fatal("legacy view must target the ECDSA wallet registry")
	}
	if ecdsaView.operatorAddress != operatorAddress {
		t.Fatalf(
			"unexpected operator address\nexpected: [%v]\nactual:   [%v]",
			operatorAddress,
			ecdsaView.operatorAddress,
		)
	}
}

func TestFrostSortitionChainBindsToFrostPoolWhenConfigured(t *testing.T) {
	tc, ecdsaPool, frostPool, operatorAddress := newTbtcChainForSortitionViewTest(true)

	view, configured := tc.FrostSortitionChain()

	if !configured {
		t.Fatal("FROST view must be configured when FROST contracts are set")
	}
	frostView, ok := view.(*frostSortitionChain)
	if !ok {
		t.Fatalf("expected *frostSortitionChain, got [%T]", view)
	}
	if frostView.pool != frostPool {
		t.Fatal("FROST view must target the FROST sortition pool")
	}
	if frostView.pool == ecdsaPool {
		t.Fatal("FROST view must NOT target the ECDSA sortition pool")
	}
	if frostView.tc != tc {
		t.Fatal("FROST view must reference the owning chain for tx submission")
	}
	if frostView.operatorAddress != operatorAddress {
		t.Fatalf(
			"unexpected operator address\nexpected: [%v]\nactual:   [%v]",
			operatorAddress,
			frostView.operatorAddress,
		)
	}
}

func TestFrostSortitionChainUnconfiguredWhenFrostAbsent(t *testing.T) {
	// No FROST contracts -> the FROST monitor loop must not start.
	tc, _, _, _ := newTbtcChainForSortitionViewTest(false)

	view, configured := tc.FrostSortitionChain()

	if configured {
		t.Fatal("FROST view must be unconfigured when FROST contracts are absent")
	}
	if view != nil {
		t.Fatal("FROST view must be nil when FROST is not configured")
	}
}
