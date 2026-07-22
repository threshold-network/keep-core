package inactivity

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity/gen/pb"
)

func validateMemberIndex(protoIndex uint32) error {
	// Protobuf does not have uint8 type, so we are using uint32. When
	// unmarshalling message, we need to make sure we do not overflow.
	if protoIndex > group.MaxMemberIndex {
		return fmt.Errorf("invalid member index value: [%v]", protoIndex)
	}
	return nil
}

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

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}
	csm.senderID = group.MemberIndex(pbMsg.SenderID)

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
