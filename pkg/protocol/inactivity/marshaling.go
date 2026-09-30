// marshaling.go: protobuf (un)marshaling for the public types in this package.
package inactivity

import (
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity/gen/pb"
)

// Marshal converts this claimSignatureMessage to a byte array suitable
// for network communication.
func (csm *claimSignatureMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.ClaimSignatureMessage{
		SenderID:  uint32(csm.senderID),
		ClaimHash: csm.claimHash[:],
		Signature: csm.signature,
		PublicKey: csm.publicKey,
		SessionID: csm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a
// claimSignatureMessage.
func (csm *claimSignatureMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.ClaimSignatureMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	csm.senderID = senderID

	claimHash, err := ClaimHashFromBytes(pbMsg.ClaimHash)
	if err != nil {
		return err
	}
	csm.claimHash = claimHash

	csm.signature = pbMsg.Signature
	csm.publicKey = pbMsg.PublicKey
	csm.sessionID = pbMsg.SessionID

	return nil
}
