package ethereum

import (
	"testing"

	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
)

func TestTbtcChainFrostWalletRegistryAvailable(t *testing.T) {
	chainWithoutFrostRegistry := &TbtcChain{}
	if chainWithoutFrostRegistry.FrostWalletRegistryAvailable() {
		t.Fatal("expected FROST wallet registry to be unavailable")
	}

	chainWithFrostRegistry := &TbtcChain{
		frostWalletRegistry: &frostabi.FrostWalletRegistry{},
	}
	if !chainWithFrostRegistry.FrostWalletRegistryAvailable() {
		t.Fatal("expected FROST wallet registry to be available")
	}
}
