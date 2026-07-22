package registry

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/dkg"
	"github.com/keep-network/keep-core/pkg/beacon/registry/gen/pb"
)

// Marshal converts Membership to a byte array.
func (m *Membership) Marshal() ([]byte, error) {
	signer, err := m.Signer.Marshal()
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.Membership{
		Signer:  signer,
		Channel: m.ChannelName,
	})
}

// Unmarshal converts a byte array produced by Marshal to Membership.
func (m *Membership) Unmarshal(bytes []byte) error {
	pbMembership := pb.Membership{}
	if err := proto.Unmarshal(bytes, &pbMembership); err != nil {
		return err
	}

	signer := &dkg.ThresholdSigner{}

	err := signer.Unmarshal(pbMembership.Signer)
	if err != nil {
		return fmt.Errorf("unexpected error occured [%v]", err)
	}

	m.Signer = signer
	m.ChannelName = pbMembership.Channel

	return nil
}
