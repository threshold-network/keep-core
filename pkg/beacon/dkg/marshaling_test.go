package dkg

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"

	"math/big"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"

	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestThresholdSignerRoundtrip(t *testing.T) {
	thresholdSigner := &ThresholdSigner{
		memberIndex:          group.MemberIndex(2),
		groupPublicKey:       new(bn256.G2).ScalarBaseMult(big.NewInt(10)),
		groupPrivateKeyShare: big.NewInt(1),
		groupPublicKeyShares: map[group.MemberIndex]*bn256.G2{
			group.MemberIndex(1): new(bn256.G2).ScalarBaseMult(big.NewInt(10)),
			group.MemberIndex(2): new(bn256.G2).ScalarBaseMult(big.NewInt(11)),
		},
		groupOperators: []chain.Address{"address1", "address2"},
	}

	unmarshaled := &ThresholdSigner{}

	err := pbutils.RoundTrip(thresholdSigner, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(thresholdSigner, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled threshold signer")
	}
}
