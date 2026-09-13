package dkg

import (
	"math"
	"math/big"
	"testing"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/registry/gen/pb"
)

func TestThresholdSignerMemberIndexBounds(t *testing.T) {
	publicKey := new(bn256.G2).ScalarBaseMult(big.NewInt(10)).Marshal()
	for _, index := range []uint32{0, 1, 255, 256, math.MaxUint32} {
		for _, inMap := range []bool{false, true} {
			message := &pb.ThresholdSigner{MemberIndex: index, GroupPublicKey: publicKey, GroupPrivateKeyShare: "1"}
			if inMap {
				message.MemberIndex = 1
				message.GroupPublicKeyShares = map[uint32][]byte{index: publicKey}
			}
			data, err := proto.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			decoded := new(ThresholdSigner)
			err = decoded.Unmarshal(data)
			rejected := index > 255 || (!inMap && index == 0)
			if rejected {
				if err == nil {
					t.Fatalf("accepted invalid index %d (map: %v)", index, inMap)
				}
			} else if err != nil {
				t.Fatalf("rejected valid index %d (map: %v): %v", index, inMap, err)
			}
		}
	}
}
