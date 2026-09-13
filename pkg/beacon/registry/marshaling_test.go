package registry

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"

	"math/big"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/keep-network/keep-core/pkg/beacon/dkg"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestMembershipRoundtrip(t *testing.T) {
	signer := dkg.NewThresholdSigner(
		group.MemberIndex(2),
		new(bn256.G2).ScalarBaseMult(big.NewInt(10)),
		big.NewInt(1),
		map[group.MemberIndex]*bn256.G2{
			group.MemberIndex(1): new(bn256.G2).ScalarBaseMult(big.NewInt(10)),
			group.MemberIndex(2): new(bn256.G2).ScalarBaseMult(big.NewInt(11)),
		},
		[]chain.Address{"address1", "address2"},
	)

	membership := &Membership{
		Signer:      signer,
		ChannelName: "channel_test_name",
	}

	unmarshaled := &Membership{}

	err := pbutils.RoundTrip(membership, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(membership, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled membership")
	}
}
