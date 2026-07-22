package dkg

import (
	"fmt"
	"math/big"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/chain"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"

	"github.com/keep-network/keep-core/pkg/beacon/registry/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Marshal converts ThresholdSigner to byte array.
func (ts *ThresholdSigner) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.ThresholdSigner{
		MemberIndex:          uint32(ts.memberIndex),
		GroupPublicKey:       ts.groupPublicKey.Marshal(),
		GroupPrivateKeyShare: ts.groupPrivateKeyShare.String(),
		GroupPublicKeyShares: marshalGroupPublicKeyShares(ts.groupPublicKeyShares),
		GroupOperators:       marshalGroupOperators(ts.groupOperators),
	})
}

func marshalGroupPublicKeyShares(
	shares map[group.MemberIndex]*bn256.G2,
) map[uint32][]byte {
	marshalled := make(map[uint32][]byte, len(shares))

	for id, share := range shares {
		marshalled[uint32(id)] = share.Marshal()
	}

	return marshalled
}

func marshalGroupOperators(groupOperators []chain.Address) []string {
	marshalled := make([]string, len(groupOperators))

	for index := range marshalled {
		marshalled[index] = groupOperators[index].String()
	}

	return marshalled
}

// Unmarshal converts a byte array back to ThresholdSigner.
func (ts *ThresholdSigner) Unmarshal(bytes []byte) error {
	pbThresholdSigner := pb.ThresholdSigner{}
	if err := proto.Unmarshal(bytes, &pbThresholdSigner); err != nil {
		return err
	}

	groupPublicKey := new(bn256.G2)
	_, err := groupPublicKey.Unmarshal(pbThresholdSigner.GroupPublicKey)
	if err != nil {
		return err
	}

	privateKeyShare := new(big.Int)
	privateKeyShare, ok := privateKeyShare.SetString(pbThresholdSigner.GroupPrivateKeyShare, 10)
	if !ok {
		return fmt.Errorf("error occurred while converting a private key share to string")
	}

	groupPublicKeyShares, err := unmarshalGroupPublicKeyShares(
		pbThresholdSigner.GroupPublicKeyShares,
	)
	if err != nil {
		return err
	}

	ts.memberIndex = group.MemberIndex(pbThresholdSigner.MemberIndex)
	ts.groupPublicKey = groupPublicKey
	ts.groupPrivateKeyShare = privateKeyShare
	ts.groupPublicKeyShares = groupPublicKeyShares
	ts.groupOperators = unmarshalGroupOperators(pbThresholdSigner.GetGroupOperators())

	return nil
}

func unmarshalGroupPublicKeyShares(
	shares map[uint32][]byte,
) (map[group.MemberIndex]*bn256.G2, error) {
	var unmarshalled = make(map[group.MemberIndex]*bn256.G2, len(shares))

	for memberID, shareBytes := range shares {
		share := new(bn256.G2)
		_, err := share.Unmarshal(shareBytes)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal share [%v]", err)
		}

		unmarshalled[group.MemberIndex(memberID)] = share
	}

	return unmarshalled, nil
}

func unmarshalGroupOperators(groupOperators []string) []chain.Address {
	unmarshalled := make([]chain.Address, len(groupOperators))

	for index := range unmarshalled {
		unmarshalled[index] = chain.Address(groupOperators[index])
	}

	return unmarshalled
}
