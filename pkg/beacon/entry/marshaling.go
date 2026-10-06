package entry

import (
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/entry/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Type returns a string describing a SignatureShareMessage's type.
func (*SignatureShareMessage) Type() string {
	return "relay/signature/share"
}

// Marshal converts this SignatureShareMessage to a byte array suitable for
// network communication.
func (ssm *SignatureShareMessage) Marshal() ([]byte, error) {
	pbSignatureShare := pb.SignatureShare{
		SenderID:  uint32(ssm.senderID),
		Share:     ssm.shareBytes,
		SessionID: ssm.sessionID,
	}

	return proto.Marshal(&pbSignatureShare)
}

// Unmarshal converts a byte array produced by Marshal to a
// SignatureShareMessage.
func (ssm *SignatureShareMessage) Unmarshal(bytes []byte) error {
	pbSignatureShare := pb.SignatureShare{}
	err := proto.Unmarshal(bytes, &pbSignatureShare)
	if err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbSignatureShare.SenderID)
	if err != nil {
		return err
	}
	ssm.senderID = senderID
	ssm.shareBytes = pbSignatureShare.Share
	ssm.sessionID = pbSignatureShare.SessionID

	return nil
}
