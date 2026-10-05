package dkg

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"

	"math/big"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/registry/gen/pb"
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

func TestThresholdSigner_Unmarshal_OutOfRangeShare(t *testing.T) {
	groupPublicKey := new(bn256.G2).ScalarBaseMult(big.NewInt(10)).Marshal()

	orderMinusOne := new(big.Int).Sub(bn256.Order, big.NewInt(1))
	orderPlusOne := new(big.Int).Add(bn256.Order, big.NewInt(1))

	tests := map[string]struct {
		share         string
		expectedValid bool
	}{
		"zero":                {share: "0", expectedValid: false},
		"negative":            {share: "-1", expectedValid: false},
		"curve order":         {share: bn256.Order.String(), expectedValid: false},
		"curve order plus 1":  {share: orderPlusOne.String(), expectedValid: false},
		"one":                 {share: "1", expectedValid: true},
		"curve order minus 1": {share: orderMinusOne.String(), expectedValid: true},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			bytes, err := proto.Marshal(&pb.ThresholdSigner{
				MemberIndex:          2,
				GroupPublicKey:       groupPublicKey,
				GroupPrivateKeyShare: test.share,
				GroupPublicKeyShares: map[uint32][]byte{},
				GroupOperators:       []string{"address1"},
			})
			if err != nil {
				t.Fatal(err)
			}

			err = (&ThresholdSigner{}).Unmarshal(bytes)

			if test.expectedValid && err != nil {
				t.Errorf("expected share [%s] to be accepted: [%v]", test.share, err)
			}
			if !test.expectedValid && err == nil {
				t.Errorf("expected share [%s] to be rejected", test.share)
			}
		})
	}
}
