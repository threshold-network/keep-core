package gjkr

import (
	"fmt"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/beacon/gjkr/gen/pb"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Type returns a string describing an EphemeralPublicKeyMessage type for
// marshaling purposes.
func (epkm *EphemeralPublicKeyMessage) Type() string {
	return "gjkr/ephemeral_public_key"
}

// Marshal converts this EphemeralPublicKeyMessage to a byte array suitable for
// network communication.
func (epkm *EphemeralPublicKeyMessage) Marshal() ([]byte, error) {
	ephemeralPublicKeys, err := marshalPublicKeyMap(epkm.ephemeralPublicKeys)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.EphemeralPublicKey{
		SenderID:            uint32(epkm.senderID),
		EphemeralPublicKeys: ephemeralPublicKeys,
		SessionID:           epkm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// an EphemeralPublicKeyMessage
func (epkm *EphemeralPublicKeyMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.EphemeralPublicKey{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	epkm.senderID = senderID

	ephemeralPublicKeys, err := unmarshalPublicKeyMap(pbMsg.EphemeralPublicKeys)
	if err != nil {
		return err
	}

	epkm.ephemeralPublicKeys = ephemeralPublicKeys
	epkm.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing a MemberCommitmentsMessage type
// for marshaling purposes.
func (mcm *MemberCommitmentsMessage) Type() string {
	return "gjkr/member_commitments"
}

// Marshal converts this MemberCommitmentsMessage to a byte array suitable for
// network communication.
func (mcm *MemberCommitmentsMessage) Marshal() ([]byte, error) {
	commitmentBytes := make([][]byte, 0, len(mcm.commitments))
	for _, commitment := range mcm.commitments {
		commitmentBytes = append(commitmentBytes, commitment.Marshal())
	}

	return proto.Marshal(&pb.MemberCommitments{
		SenderID:    uint32(mcm.senderID),
		Commitments: commitmentBytes,
		SessionID:   mcm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// a MemberCommitmentsMessage
func (mcm *MemberCommitmentsMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.MemberCommitments{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	mcm.senderID = senderID

	var commitments []*bn256.G1
	for _, commitmentBytes := range pbMsg.Commitments {
		commitment := new(bn256.G1)
		_, err := commitment.Unmarshal(commitmentBytes)
		if err != nil {
			return fmt.Errorf(
				"could not unmarshal member's commitment [%v]",
				err,
			)
		}
		commitments = append(commitments, commitment)
	}
	mcm.commitments = commitments
	mcm.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing a PeerSharesMessage type for marshaling
// purposes
func (psm *PeerSharesMessage) Type() string {
	return "gjkr/peer_shares"
}

// Marshal converts this PeerSharesMessage to a byte array suitable for
// network communication.
func (psm *PeerSharesMessage) Marshal() ([]byte, error) {
	pbShares := make(map[uint32]*pb.PeerShares_Shares)
	for memberID, shares := range psm.shares {
		if shares == nil {
			return nil, fmt.Errorf("nil shares for member [%v]", memberID)
		}

		pbShares[uint32(memberID)] = &pb.PeerShares_Shares{
			EncryptedShareS: shares.encryptedShareS,
			EncryptedShareT: shares.encryptedShareT,
		}
	}

	return proto.Marshal(&pb.PeerShares{
		SenderID:  uint32(psm.senderID),
		Shares:    pbShares,
		SessionID: psm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to a PeerSharesMessage.
func (psm *PeerSharesMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.PeerShares{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	psm.senderID = senderID

	shares := make(map[group.MemberIndex]*peerShares)
	for memberID, pbShares := range pbMsg.Shares {
		memberIndex, err := group.MemberIndexFromUint32(memberID)
		if err != nil {
			return err
		}

		if pbShares == nil {
			return fmt.Errorf("nil shares from member [%v]", memberID)
		}

		shares[memberIndex] = &peerShares{
			encryptedShareS: pbShares.EncryptedShareS,
			encryptedShareT: pbShares.EncryptedShareT,
		}
	}

	psm.shares = shares
	psm.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing a SecretSharesAccusationsMessage type
// for marshalling purposes.
func (ssam *SecretSharesAccusationsMessage) Type() string {
	return "gjkr/secret_shares_accusations"
}

// Marshal converts this SecretSharesAccusationsMessage to a byte array
// suitable for network communication.
func (ssam *SecretSharesAccusationsMessage) Marshal() ([]byte, error) {
	accusedMembersKeys, err := marshalPrivateKeyMap(ssam.accusedMembersKeys)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.SecretSharesAccusations{
		SenderID:           uint32(ssam.senderID),
		AccusedMembersKeys: accusedMembersKeys,
		SessionID:          ssam.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// a SecretSharesAccusationsMessage.
func (ssam *SecretSharesAccusationsMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.SecretSharesAccusations{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	ssam.senderID = senderID

	accusedMembersKeys, err := unmarshalPrivateKeyMap(pbMsg.AccusedMembersKeys)
	if err != nil {
		return err
	}

	ssam.accusedMembersKeys = accusedMembersKeys
	ssam.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing MemberPublicKeySharePointsMessage type for
// marshaling purposes
func (mpspm *MemberPublicKeySharePointsMessage) Type() string {
	return "gjkr/member_public_key_share_points"
}

// Marshal converts this MemberPublicKeySharePointsMessage to a byte array
// suitable for network communication.
func (mpspm *MemberPublicKeySharePointsMessage) Marshal() ([]byte, error) {
	keySharePoints := make([][]byte, 0, len(mpspm.publicKeySharePoints))
	for i, keySharePoint := range mpspm.publicKeySharePoints {
		if keySharePoint == nil {
			return nil, fmt.Errorf("nil key share point at index [%v]", i)
		}

		keySharePoints = append(keySharePoints, keySharePoint.Marshal())
	}

	return proto.Marshal(&pb.MemberPublicKeySharePoints{
		SenderID:             uint32(mpspm.senderID),
		PublicKeySharePoints: keySharePoints,
		SessionID:            mpspm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// a MemberPublicKeySharePointsMessage.
func (mpspm *MemberPublicKeySharePointsMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.MemberPublicKeySharePoints{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	mpspm.senderID = senderID

	var keySharePoints []*bn256.G2
	for _, keySharePointBytes := range pbMsg.PublicKeySharePoints {
		keySharePoint := new(bn256.G2)
		_, err := keySharePoint.Unmarshal(keySharePointBytes)
		if err != nil {
			return fmt.Errorf(
				"could not unmarshal member's key share point [%v]",
				err,
			)
		}
		keySharePoints = append(keySharePoints, keySharePoint)
	}
	mpspm.publicKeySharePoints = keySharePoints
	mpspm.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing PointsAccusationsMessage type for
// marshaling purposes.
func (pam *PointsAccusationsMessage) Type() string {
	return "gjkr/points_accusations_message"
}

// Marshal converts this PointsAccusationsMessage to a byte array suitable
// for network communication.
func (pam *PointsAccusationsMessage) Marshal() ([]byte, error) {
	accusedMembersKeys, err := marshalPrivateKeyMap(pam.accusedMembersKeys)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.PointsAccusations{
		SenderID:           uint32(pam.senderID),
		AccusedMembersKeys: accusedMembersKeys,
		SessionID:          pam.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// a PointsAccusationsMessage.
func (pam *PointsAccusationsMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.PointsAccusations{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	pam.senderID = senderID

	accusedMembersKeys, err := unmarshalPrivateKeyMap(pbMsg.AccusedMembersKeys)
	if err != nil {
		return err
	}

	pam.accusedMembersKeys = accusedMembersKeys
	pam.sessionID = pbMsg.SessionID

	return nil
}

// Type returns a string describing MisbehavedEphemeralKeysMessage type for
// marshalling purposes.
func (mekm *MisbehavedEphemeralKeysMessage) Type() string {
	return "gjkr/misbehaved_ephemeral_keys_message"
}

// Marshal converts this MisbehavedEphemeralKeysMessage to a byte array
// suitable for network communication.
func (mekm *MisbehavedEphemeralKeysMessage) Marshal() ([]byte, error) {
	privateKeys, err := marshalPrivateKeyMap(mekm.privateKeys)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&pb.MisbehavedEphemeralKeys{
		SenderID:    uint32(mekm.senderID),
		PrivateKeys: privateKeys,
		SessionID:   mekm.sessionID,
	})
}

// Unmarshal converts a byte array produced by Marshal to
// a MisbehavedEphemeralKeysMessage.
func (mekm *MisbehavedEphemeralKeysMessage) Unmarshal(bytes []byte) error {
	pbMsg := pb.MisbehavedEphemeralKeys{}
	if err := proto.Unmarshal(bytes, &pbMsg); err != nil {
		return err
	}

	senderID, err := group.MemberIndexFromUint32(pbMsg.SenderID)
	if err != nil {
		return err
	}
	mekm.senderID = senderID

	privateKeys, err := unmarshalPrivateKeyMap(pbMsg.PrivateKeys)
	if err != nil {
		return err
	}

	mekm.privateKeys = privateKeys
	mekm.sessionID = pbMsg.SessionID

	return nil
}

func marshalPublicKeyMap(
	publicKeys map[group.MemberIndex]*ephemeral.PublicKey,
) (map[uint32][]byte, error) {
	marshalled := make(map[uint32][]byte, len(publicKeys))
	for id, publicKey := range publicKeys {
		if publicKey == nil {
			return nil, fmt.Errorf("nil public key for member [%v]", id)
		}

		marshalled[uint32(id)] = publicKey.Marshal()
	}
	return marshalled, nil
}

func unmarshalPublicKeyMap(
	publicKeys map[uint32][]byte,
) (map[group.MemberIndex]*ephemeral.PublicKey, error) {
	var unmarshalled = make(map[group.MemberIndex]*ephemeral.PublicKey, len(publicKeys))
	for memberID, publicKeyBytes := range publicKeys {
		memberIndex, err := group.MemberIndexFromUint32(memberID)
		if err != nil {
			return nil, err
		}

		publicKey, err := ephemeral.UnmarshalPublicKey(publicKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal public key [%v]", err)
		}

		unmarshalled[memberIndex] = publicKey

	}

	return unmarshalled, nil
}

func marshalPrivateKeyMap(
	privateKeys map[group.MemberIndex]*ephemeral.PrivateKey,
) (map[uint32][]byte, error) {
	marshalled := make(map[uint32][]byte, len(privateKeys))
	for id, privateKey := range privateKeys {
		if privateKey == nil {
			return nil, fmt.Errorf("nil private key for member [%v]", id)
		}

		marshalled[uint32(id)] = privateKey.Marshal()
	}
	return marshalled, nil
}

func unmarshalPrivateKeyMap(
	privateKeys map[uint32][]byte,
) (map[group.MemberIndex]*ephemeral.PrivateKey, error) {
	var unmarshalled = make(map[group.MemberIndex]*ephemeral.PrivateKey, len(privateKeys))
	for memberID, privateKeyBytes := range privateKeys {
		memberIndex, err := group.MemberIndexFromUint32(memberID)
		if err != nil {
			return nil, err
		}

		if len(privateKeyBytes) == 0 {
			return nil, fmt.Errorf(
				"private key bytes slice is empty for member [%v]", memberID,
			)
		}

		unmarshalled[memberIndex] = ephemeral.UnmarshalPrivateKey(privateKeyBytes)
	}

	return unmarshalled, nil
}
