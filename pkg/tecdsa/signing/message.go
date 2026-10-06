package signing

import (
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const messageTypePrefix = "tecdsa_signing/"

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

// tssRoundOneMessage is a message payload that carries the sender's
// TSS round one components.
type tssRoundOneMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	peersPayload     map[group.MemberIndex][]byte
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

// Type returns a string describing a tssRoundOneMessage type for
// marshaling purposes.
func (trom *tssRoundOneMessage) Type() string {
	return messageTypePrefix + "tss_round_one_message"
}

// tssRoundTwoMessage is a message payload that carries the sender's
// TSS round two components.
type tssRoundTwoMessage struct {
	senderID group.MemberIndex

	peersPayload map[group.MemberIndex][]byte
	sessionID    string
}

// SenderID returns protocol-level identifier of the message sender.
func (trtm *tssRoundTwoMessage) SenderID() group.MemberIndex {
	return trtm.senderID
}

// SessionID returns the session identifier of the message.
func (trtm *tssRoundTwoMessage) SessionID() string {
	return trtm.sessionID
}

// Type returns a string describing a tssRoundTwoMessage type for
// marshaling purposes.
func (trtm *tssRoundTwoMessage) Type() string {
	return messageTypePrefix + "tss_round_two_message"
}

// tssRoundThreeMessage is a message payload that carries the sender's
// TSS round three components.
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

// Type returns a string describing a tssRoundThreeMessage type for
// marshaling purposes.
func (trtm *tssRoundThreeMessage) Type() string {
	return messageTypePrefix + "tss_round_three_message"
}

// tssRoundFourMessage is a message payload that carries the sender's
// TSS round four components.
type tssRoundFourMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trfm *tssRoundFourMessage) SenderID() group.MemberIndex {
	return trfm.senderID
}

// SessionID returns the session identifier of the message.
func (trfm *tssRoundFourMessage) SessionID() string {
	return trfm.sessionID
}

// Type returns a string describing a tssRoundFourMessage type for
// marshaling purposes.
func (trfm *tssRoundFourMessage) Type() string {
	return messageTypePrefix + "tss_round_four_message"
}

// tssRoundFiveMessage is a message payload that carries the sender's
// TSS round five components.
type tssRoundFiveMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trfm *tssRoundFiveMessage) SenderID() group.MemberIndex {
	return trfm.senderID
}

// SessionID returns the session identifier of the message.
func (trfm *tssRoundFiveMessage) SessionID() string {
	return trfm.sessionID
}

// Type returns a string describing a tssRoundFiveMessage type for
// marshaling purposes.
func (trfm *tssRoundFiveMessage) Type() string {
	return messageTypePrefix + "tss_round_five_message"
}

// tssRoundSixMessage is a message payload that carries the sender's
// TSS round six components.
type tssRoundSixMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trsm *tssRoundSixMessage) SenderID() group.MemberIndex {
	return trsm.senderID
}

// SessionID returns the session identifier of the message.
func (trsm *tssRoundSixMessage) SessionID() string {
	return trsm.sessionID
}

// Type returns a string describing a tssRoundSixMessage type for
// marshaling purposes.
func (trfm *tssRoundSixMessage) Type() string {
	return messageTypePrefix + "tss_round_six_message"
}

// tssRoundSevenMessage is a message payload that carries the sender's
// TSS round seven components.
type tssRoundSevenMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trsm *tssRoundSevenMessage) SenderID() group.MemberIndex {
	return trsm.senderID
}

// SessionID returns the session identifier of the message.
func (trfm *tssRoundSevenMessage) SessionID() string {
	return trfm.sessionID
}

// Type returns a string describing a tssRoundSevenMessage type for
// marshaling purposes.
func (trfm *tssRoundSevenMessage) Type() string {
	return messageTypePrefix + "tss_round_seven_message"
}

// tssRoundEightMessage is a message payload that carries the sender's
// TSS round eight components.
type tssRoundEightMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trem *tssRoundEightMessage) SenderID() group.MemberIndex {
	return trem.senderID
}

// SessionID returns the session identifier of the message.
func (trem *tssRoundEightMessage) SessionID() string {
	return trem.sessionID
}

// Type returns a string describing a tssRoundEightMessage type for
// marshaling purposes.
func (trem *tssRoundEightMessage) Type() string {
	return messageTypePrefix + "tss_round_eight_message"
}

// tssRoundNineMessage is a message payload that carries the sender's
// TSS round nine components.
type tssRoundNineMessage struct {
	senderID group.MemberIndex

	broadcastPayload []byte
	sessionID        string
}

// SenderID returns protocol-level identifier of the message sender.
func (trnm *tssRoundNineMessage) SenderID() group.MemberIndex {
	return trnm.senderID
}

// SessionID returns the session identifier of the message.
func (trnm *tssRoundNineMessage) SessionID() string {
	return trnm.sessionID
}

// Type returns a string describing a tssRoundNineMessage type for
// marshaling purposes.
func (trnm *tssRoundNineMessage) Type() string {
	return messageTypePrefix + "tss_round_nine_message"
}
