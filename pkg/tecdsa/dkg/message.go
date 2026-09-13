package dkg

import (
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const messageTypePrefix = "tecdsa_dkg/"

// message holds common traits of all signing protocol messages.
type message interface {
	// SenderID returns protocol-level identifier of the message sender.
	SenderID() group.MemberIndex
	// SessionID returns the session identifier of the message.
	SessionID() string
	// Type returns the exact type of the message.
	Type() string
}

// ephemeralPublicKeyMessage is a message payload that carries the sender's
// ephemeral public keys generated for all other group members.
//
// The receiver performs ECDH on a sender's ephemeral public key intended for
// the receiver and on the receiver's private ephemeral key, creating a symmetric
// key used for encrypting a conversation between the sender and the receiver.
type ephemeralPublicKeyMessage struct {
	senderID group.MemberIndex

	ephemeralPublicKeys map[group.MemberIndex][]byte
	sessionID           string
}

// SenderID returns protocol-level identifier of the message sender.
func (epkm *ephemeralPublicKeyMessage) SenderID() group.MemberIndex {
	return epkm.senderID
}

// SessionID returns the session identifier of the message.
func (epkm *ephemeralPublicKeyMessage) SessionID() string {
	return epkm.sessionID
}

// Type returns a string describing an ephemeralPublicKeyMessage type for
// marshaling purposes.
func (epkm *ephemeralPublicKeyMessage) Type() string {
	return messageTypePrefix + "ephemeral_public_key_message"
}

// tssRoundOneMessage is a message payload that carries the sender's TSS
// commitments and the Paillier public key.
type tssRoundOneMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trom *tssRoundOneMessage) SenderID() group.MemberIndex {
	return trom.senderID
}

// SessionID returns the session identifier of the message.
func (trom *tssRoundOneMessage) SessionID() string {
	return trom.sessionID
}

// Type returns a string describing an tssRoundOneMessage type for
// marshaling purposes.
func (trom *tssRoundOneMessage) Type() string {
	return messageTypePrefix + "tss_round_one_message"
}

// tssRoundTwoMessage is a message payload that carries the sender's TSS
// shares and de-commitments.
type tssRoundTwoMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	peersPayload     map[group.MemberIndex][]byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trtm *tssRoundTwoMessage) SenderID() group.MemberIndex {
	return trtm.senderID
}

// SessionID returns the session identifier of the message.
func (trtm *tssRoundTwoMessage) SessionID() string {
	return trtm.sessionID
}

// Type returns a string describing an tssRoundTwoMessage type for
// marshaling purposes.
func (trtm *tssRoundTwoMessage) Type() string {
	return messageTypePrefix + "tss_round_two_message"
}

// tssRoundThreeMessage is a message payload that carries the sender's TSS
// Paillier proof.
type tssRoundThreeMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trtm *tssRoundThreeMessage) SenderID() group.MemberIndex {
	return trtm.senderID
}

// SessionID returns the session identifier of the message.
func (trtm *tssRoundThreeMessage) SessionID() string {
	return trtm.sessionID
}

// Type returns a string describing an tssRoundThreeMessage type for
// marshaling purposes.
func (trtm *tssRoundThreeMessage) Type() string {
	return messageTypePrefix + "tss_round_three_message"
}

// tssFinalizationMessage is a message payload that carries the sender's TSS
// finalization confirmation.
type tssFinalizationMessage struct {
	senderID group.MemberIndex

	sessionID string
}

// SenderID returns protocol-level identifier of the message sender.
func (tfm *tssFinalizationMessage) SenderID() group.MemberIndex {
	return tfm.senderID
}

// SessionID returns the session identifier of the message.
func (tfm *tssFinalizationMessage) SessionID() string {
	return tfm.sessionID
}

// Type returns a string describing an tssFinalizationMessage type for
// marshaling purposes.
func (tfm *tssFinalizationMessage) Type() string {
	return messageTypePrefix + "tss_finalization_message"
}

// resultSignatureMessage is a message payload that carries a hash of
// the DKG result and a signature over this hash for a DKG result.
//
// It is expected to be broadcast within the group.
type resultSignatureMessage struct {
	senderID group.MemberIndex

	resultHash ResultSignatureHash
	signature  []byte
	publicKey  []byte
	sessionID  string
}

// SenderID returns protocol-level identifier of the message sender.
func (rsm *resultSignatureMessage) SenderID() group.MemberIndex {
	return rsm.senderID
}

// SessionID returns the session identifier of the message.
func (rsm *resultSignatureMessage) SessionID() string {
	return rsm.sessionID
}

// Type returns a string describing an resultSignatureMessage type for
// marshaling purposes.
func (rsm *resultSignatureMessage) Type() string {
	return messageTypePrefix + "result_signature_message"
}
