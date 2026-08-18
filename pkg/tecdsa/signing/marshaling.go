// marshaling.go: protobuf (un)marshalling for the public types in this package.
package signing

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa/signing/gen/pb"
)

// Marshal converts this ephemeralPublicKeyMessage to a byte array suitable for
// network communication.
func (epkm *ephemeralPublicKeyMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.EphemeralPublicKeyMessage{
		SenderID:            uint32(epkm.senderID),
		EphemeralPublicKeys: marshalPublicKeyMap(epkm.ephemeralPublicKeys),
		SessionID:           epkm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// an ephemeralPublicKeyMessage
func (epkm *ephemeralPublicKeyMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.EphemeralPublicKeyMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}
	epkm.senderID = group.MemberIndex(pbMsg.SenderID)

	ephemeralPublicKeys, err := unmarshalPublicKeyMap(pbMsg.EphemeralPublicKeys)
	if err != nil {
		return err
	}

	epkm.ephemeralPublicKeys = ephemeralPublicKeys
	epkm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundOneMessage to a byte array suitable for
// network communication.
func (trom *tssRoundOneMessage) Marshal() ([]byte, error) {
	peersPayload := make(map[uint32][]byte, len(trom.peersPayload))
	for receiverID, payload := range trom.peersPayload {
		peersPayload[uint32(receiverID)] = payload
	}

	return proto.Marshal(&pb.TSSRoundOneMessage{
		SenderID:         uint32(trom.senderID),
		BroadcastPayload: trom.broadcastPayload,
		PeersPayload:     peersPayload,
		SessionID:        trom.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundOneMessage.
func (trom *tssRoundOneMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundOneMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	peersPayload := make(map[group.MemberIndex][]byte, len(pbMsg.PeersPayload))
	for receiverID, payload := range pbMsg.PeersPayload {
		if err := validateMemberIndex(receiverID); err != nil {
			return err
		}

		peersPayload[group.MemberIndex(receiverID)] = payload
	}

	trom.senderID = group.MemberIndex(pbMsg.SenderID)
	trom.broadcastPayload = pbMsg.BroadcastPayload
	trom.peersPayload = peersPayload
	trom.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundTwoMessage to a byte array suitable for
// network communication.
func (trtm *tssRoundTwoMessage) Marshal() ([]byte, error) {
	peersPayload := make(map[uint32][]byte, len(trtm.peersPayload))
	for receiverID, payload := range trtm.peersPayload {
		peersPayload[uint32(receiverID)] = payload
	}

	return proto.Marshal(&pb.TSSRoundTwoMessage{
		SenderID:     uint32(trtm.senderID),
		PeersPayload: peersPayload,
		SessionID:    trtm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundTwoMessage.
func (trtm *tssRoundTwoMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundTwoMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	peersPayload := make(map[group.MemberIndex][]byte, len(pbMsg.PeersPayload))
	for receiverID, payload := range pbMsg.PeersPayload {
		if err := validateMemberIndex(receiverID); err != nil {
			return err
		}

		peersPayload[group.MemberIndex(receiverID)] = payload
	}

	trtm.senderID = group.MemberIndex(pbMsg.SenderID)
	trtm.peersPayload = peersPayload
	trtm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundThreeMessage to a byte array suitable for
// network communication.
func (trtm *tssRoundThreeMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundThreeMessage{
		SenderID:         uint32(trtm.senderID),
		BroadcastPayload: trtm.broadcastPayload,
		SessionID:        trtm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundThreeMessage.
func (trtm *tssRoundThreeMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundThreeMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trtm.senderID = group.MemberIndex(pbMsg.SenderID)
	trtm.broadcastPayload = pbMsg.BroadcastPayload
	trtm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundFourMessage to a byte array suitable for
// network communication.
func (trfm *tssRoundFourMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundFourMessage{
		SenderID:         uint32(trfm.senderID),
		BroadcastPayload: trfm.broadcastPayload,
		SessionID:        trfm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundFourMessage.
func (trfm *tssRoundFourMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundFourMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trfm.senderID = group.MemberIndex(pbMsg.SenderID)
	trfm.broadcastPayload = pbMsg.BroadcastPayload
	trfm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundFiveMessage to a byte array suitable for
// network communication.
func (trfm *tssRoundFiveMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundFiveMessage{
		SenderID:         uint32(trfm.senderID),
		BroadcastPayload: trfm.broadcastPayload,
		SessionID:        trfm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundFiveMessage.
func (trfm *tssRoundFiveMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundFiveMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trfm.senderID = group.MemberIndex(pbMsg.SenderID)
	trfm.broadcastPayload = pbMsg.BroadcastPayload
	trfm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundSixMessage to a byte array suitable for
// network communication.
func (trsm *tssRoundSixMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundSixMessage{
		SenderID:         uint32(trsm.senderID),
		BroadcastPayload: trsm.broadcastPayload,
		SessionID:        trsm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundSixMessage.
func (trsm *tssRoundSixMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundSixMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trsm.senderID = group.MemberIndex(pbMsg.SenderID)
	trsm.broadcastPayload = pbMsg.BroadcastPayload
	trsm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundSevenMessage to a byte array suitable for
// network communication.
func (trsm *tssRoundSevenMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundSevenMessage{
		SenderID:         uint32(trsm.senderID),
		BroadcastPayload: trsm.broadcastPayload,
		SessionID:        trsm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundSevenMessage.
func (trsm *tssRoundSevenMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundSevenMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trsm.senderID = group.MemberIndex(pbMsg.SenderID)
	trsm.broadcastPayload = pbMsg.BroadcastPayload
	trsm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundEightMessage to a byte array suitable for
// network communication.
func (trem *tssRoundEightMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundEightMessage{
		SenderID:         uint32(trem.senderID),
		BroadcastPayload: trem.broadcastPayload,
		SessionID:        trem.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundEightMessage.
func (trem *tssRoundEightMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundEightMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trem.senderID = group.MemberIndex(pbMsg.SenderID)
	trem.broadcastPayload = pbMsg.BroadcastPayload
	trem.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts this tssRoundNineMessage to a byte array suitable for
// network communication.
func (trnm *tssRoundNineMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSRoundNineMessage{
		SenderID:         uint32(trnm.senderID),
		BroadcastPayload: trnm.broadcastPayload,
		SessionID:        trnm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a tssRoundNineMessage.
func (trnm *tssRoundNineMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundNineMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trnm.senderID = group.MemberIndex(pbMsg.SenderID)
	trnm.broadcastPayload = pbMsg.BroadcastPayload
	trnm.sessionID = pbMsg.SessionID

	return nil
}

func validateMemberIndex(protoIndex uint32) error {
	// Protobuf does not have uint8 type, so we are using uint32. When
	// unmarshalling message, we need to make sure we do not overflow.
	if protoIndex > group.MaxMemberIndex {
		return fmt.Errorf("invalid member index value: [%v]", protoIndex)
	}
	return nil
}

func marshalPublicKeyMap(
	publicKeys map[group.MemberIndex][]byte,
) map[uint32][]byte {
	marshalled := make(map[uint32][]byte, len(publicKeys))
	for id, keyBytes := range publicKeys {
		marshalled[uint32(id)] = keyBytes
	}
	return marshalled
}

// unmarshalPublicKeyMap converts the wire-format map to an internal byte map,
// validating member indices but deferring EC point parsing to use-time so that
// only the one key per message actually needed for ECDH is ever parsed.
func unmarshalPublicKeyMap(
	publicKeys map[uint32][]byte,
) (map[group.MemberIndex][]byte, error) {
	unmarshalled := make(map[group.MemberIndex][]byte, len(publicKeys))
	for memberID, publicKeyBytes := range publicKeys {
		if err := validateMemberIndex(memberID); err != nil {
			return nil, err
		}
		unmarshalled[group.MemberIndex(memberID)] = publicKeyBytes
	}
	return unmarshalled, nil
}
