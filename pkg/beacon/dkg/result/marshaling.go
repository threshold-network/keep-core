package result

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/beacon/dkg/result/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// MemberIndex is represented as uint8 in gjkr. Protobuf does not have uint8
// type so we are using uint32. When unmarshalling message, we need to make
// sure we do not overflow.
const maxMemberIndex = 255

func validateMemberIndex(protoIndex uint32) error {
	if protoIndex > maxMemberIndex {
		return fmt.Errorf("invalid member index value: [%v]", protoIndex)
	}
	return nil
}

// Type returns a string describing a DKGResultHashSignatureMessage type for
// marshalling purposes.
func (d *DKGResultHashSignatureMessage) Type() string {
	return "result/dkg_result_hash_signature_message"
}

// Marshal converts this DKGResultHashSignatureMessage to a byte array suitable
// for network communication.
func (d *DKGResultHashSignatureMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.DKGResultHashSignature{
		SenderIndex: uint32(d.senderIndex),
		ResultHash:  d.resultHash[:],
		Signature:   d.signature,
		PublicKey:   d.publicKey,
		SessionID:   d.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a
// DKGResultHashSignatureMessage.
func (d *DKGResultHashSignatureMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.DKGResultHashSignature{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderIndex); err != nil {
		return err
	}
	d.senderIndex = group.MemberIndex(pbMsg.SenderIndex)

	resultHash, err := chain.DKGResultHashFromBytes(pbMsg.ResultHash)
	if err != nil {
		return err
	}
	d.resultHash = resultHash

	d.signature = pbMsg.Signature
	d.publicKey = pbMsg.PublicKey
	d.sessionID = pbMsg.SessionID

	return nil
}
