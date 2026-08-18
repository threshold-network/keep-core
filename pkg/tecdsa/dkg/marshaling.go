// marshaling.go: protobuf (un)marshalling for the public types in this package.
package dkg

import (
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/crypto/paillier"
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg/gen/pb"
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
	return proto.Marshal(&pb.TSSRoundOneMessage{
		SenderID:         uint32(trom.senderID),
		BroadcastPayload: trom.broadcastPayload,
		SessionID:        trom.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to an tssRoundOneMessage.
func (trom *tssRoundOneMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSRoundOneMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	trom.senderID = group.MemberIndex(pbMsg.SenderID)
	trom.broadcastPayload = pbMsg.BroadcastPayload
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
		SenderID:         uint32(trtm.senderID),
		BroadcastPayload: trtm.broadcastPayload,
		PeersPayload:     peersPayload,
		SessionID:        trtm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to an tssRoundTwoMessage.
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
	trtm.broadcastPayload = pbMsg.BroadcastPayload
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

// Unmarshal converts a byte array produced by Marshal to an tssRoundThreeMessage.
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

// Marshal converts this tssFinalizationMessage to a byte array suitable for
// network communication.
func (tfm *tssFinalizationMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.TSSFinalizationMessage{
		SenderID:  uint32(tfm.senderID),
		SessionID: tfm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to an
// tssFinalizationMessage.
func (tfm *tssFinalizationMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.TSSFinalizationMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}

	tfm.senderID = group.MemberIndex(pbMsg.SenderID)
	tfm.sessionID = pbMsg.SessionID

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

// Marshal converts this resultSignatureMessage to a byte array suitable
// for network communication.
func (rsm *resultSignatureMessage) Marshal() ([]byte, error) {
	return proto.Marshal(&pb.ResultSignatureMessage{
		SenderID:   uint32(rsm.senderID),
		ResultHash: rsm.resultHash[:],
		Signature:  rsm.signature,
		PublicKey:  rsm.publicKey,
		SessionID:  rsm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a
// resultSignatureMessage.
func (rsm *resultSignatureMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.ResultSignatureMessage{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	if err := validateMemberIndex(pbMsg.SenderID); err != nil {
		return err
	}
	rsm.senderID = group.MemberIndex(pbMsg.SenderID)

	resultHash, err := ResultSignatureHashFromBytes(pbMsg.ResultHash)
	if err != nil {
		return err
	}
	rsm.resultHash = resultHash

	rsm.signature = pbMsg.Signature
	rsm.publicKey = pbMsg.PublicKey
	rsm.sessionID = pbMsg.SessionID

	return nil
}

// Marshal converts the PreParams to a byte array.
func (pp *PreParams) Marshal() ([]byte, error) {
	localPreParams := &pb.PreParams_LocalPreParams{
		PaillierSK: &pb.PreParams_PrivateKey{
			PublicKey: &pb.PreParams_PublicKey{
				N: pp.data.PaillierSK.N.Bytes(),
			},
			LambdaN: pp.data.PaillierSK.LambdaN.Bytes(),
			PhiN:    pp.data.PaillierSK.PhiN.Bytes(),
		},
		NTilde: pp.data.NTildei.Bytes(),
		H1I:    pp.data.H1i.Bytes(),
		H2I:    pp.data.H2i.Bytes(),
		Alpha:  pp.data.Alpha.Bytes(),
		Beta:   pp.data.Beta.Bytes(),
		P:      pp.data.P.Bytes(),
		Q:      pp.data.Q.Bytes(),
	}

	return proto.Marshal(&pb.PreParams{
		Data:              localPreParams,
		CreationTimestamp: timestamppb.New(pp.creationTimestamp),
	})
}

// Unmarshal converts a byte array back to the PreParams.
func (pp *PreParams) Unmarshal(bytes []byte) error {
	pbPreParams := pb.PreParams{}
	if err := proto.Unmarshal(bytes, &pbPreParams); err != nil {
		return fmt.Errorf("failed to unmarshal pre params: [%v]", err)
	}

	pp.data = &keygen.LocalPreParams{
		PaillierSK: &paillier.PrivateKey{
			PublicKey: paillier.PublicKey{
				N: new(big.Int).SetBytes(pbPreParams.Data.GetPaillierSK().GetPublicKey().GetN()),
			},
			LambdaN: new(big.Int).SetBytes(pbPreParams.Data.GetPaillierSK().GetLambdaN()),
			PhiN:    new(big.Int).SetBytes(pbPreParams.Data.GetPaillierSK().GetPhiN()),
		},
		NTildei: new(big.Int).SetBytes(pbPreParams.Data.GetNTilde()),
		H1i:     new(big.Int).SetBytes(pbPreParams.Data.GetH1I()),
		H2i:     new(big.Int).SetBytes(pbPreParams.Data.GetH2I()),
		Alpha:   new(big.Int).SetBytes(pbPreParams.Data.GetAlpha()),
		Beta:    new(big.Int).SetBytes(pbPreParams.Data.GetBeta()),
		P:       new(big.Int).SetBytes(pbPreParams.Data.GetP()),
		Q:       new(big.Int).SetBytes(pbPreParams.Data.GetQ()),
	}
	pp.creationTimestamp = pbPreParams.CreationTimestamp.AsTime()

	return nil
}
