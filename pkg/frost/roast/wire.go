package roast

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Evidence wire format: signed-body protobuf envelopes.
//
// Operator and coordinator signatures cover exact serialized body bytes,
// and those bytes travel verbatim - a verifier checks the signature over
// the bytes it received and only then parses them. Nothing in the
// evidence chain is ever re-encoded, so signature validity never depends
// on any serializer's canonical form, across protobuf library versions or
// across languages. Producers marshal a body exactly once (at signing
// time) and cache it; parsed messages cache the received bytes.
//
// Domain separation. Several signed bodies share the node's operator key and
// are structurally similar - each carries an attempt-context hash and a member
// index. The TransitionMessageBody and the signing-package body are outright
// wire-compatible (both have attempt_context_hash as a length-delimited field
// 1). The LocalEvidenceSnapshotBody is only INCIDENTALLY distinguished - its
// field 1 is sender_id (a varint), not a length-delimited field - a difference
// a later proto change could erase. So a signature over one body must not be
// acceptable as a signature over another. Each signed-body type therefore
// prepends a UNIQUE domain tag to the bytes it signs and verifies
// (SignableBytes), making the separation intentional rather than incidental,
// while the body that travels on the wire stays the bare serialized body
// (bodyBytes).
//
// Each tag BEGINS with byte 0x00 - an illegal protobuf tag (field number 0) -
// so the signed payload is undecodable as any protobuf message. That separates
// the domains in both directions without relying on field layout: a signature
// over one body cannot be replayed onto another envelope (whose decoder
// proto.Unmarshals and rejects the 0x00-leading body), and a genuine
// serialized protobuf body always begins with a valid tag (>= 0x08), so its
// signature can never verify against domain || body. The tags are NOT carried
// on the wire; signer and verifier prepend the same constant. (The signed
// signing-package envelope follows the same scheme with its own tag.)
var (
	localEvidenceSnapshotSignatureDomain = []byte("\x00roast/signed-evidence-snapshot/v1\x00")
	transitionMessageSignatureDomain     = []byte("\x00roast/signed-transition-message/v1\x00")
)

func snapshotBodyMessage(s *LocalEvidenceSnapshot) *pb.LocalEvidenceSnapshotBody {
	body := &pb.LocalEvidenceSnapshotBody{
		SenderId:           s.SenderIDValue,
		AttemptContextHash: s.AttemptContextHash,
	}
	for _, e := range s.Overflows {
		body.Overflows = append(body.Overflows, &pb.OverflowEntry{
			Sender: uint32(e.Sender),
			Count:  uint64(e.Count),
		})
	}
	for _, e := range s.Rejects {
		body.Rejects = append(body.Rejects, &pb.RejectEntry{
			Sender: uint32(e.Sender),
			Reason: e.Reason,
			Count:  uint64(e.Count),
		})
	}
	for _, e := range s.Conflicts {
		body.Conflicts = append(body.Conflicts, &pb.ConflictEntry{
			Sender: uint32(e.Sender),
			Count:  uint64(e.Count),
		})
	}
	// Carried verbatim; canonical order + bounds are enforced by Validate. Nil
	// when none, so a proof-free snapshot encodes exactly as before.
	body.CoordinatorPackageProofs = s.CoordinatorPackageProofs
	return body
}

func snapshotFieldsFromBody(s *LocalEvidenceSnapshot, body *pb.LocalEvidenceSnapshotBody) {
	s.SenderIDValue = body.SenderId
	s.AttemptContextHash = append([]byte(nil), body.AttemptContextHash...)
	s.Overflows = nil
	for _, e := range body.Overflows {
		s.Overflows = append(s.Overflows, OverflowEntry{
			Sender: group.MemberIndex(e.Sender),
			Count:  uint(e.Count),
		})
	}
	s.Rejects = nil
	for _, e := range body.Rejects {
		s.Rejects = append(s.Rejects, RejectEntry{
			Sender: group.MemberIndex(e.Sender),
			Reason: e.Reason,
			Count:  uint(e.Count),
		})
	}
	s.Conflicts = nil
	for _, e := range body.Conflicts {
		s.Conflicts = append(s.Conflicts, ConflictEntry{
			Sender: group.MemberIndex(e.Sender),
			Count:  uint(e.Count),
		})
	}
	s.CoordinatorPackageProofs = nil
	for _, proof := range body.CoordinatorPackageProofs {
		s.CoordinatorPackageProofs = append(
			s.CoordinatorPackageProofs,
			append([]byte(nil), proof...),
		)
	}
}

// bodyBytes returns the exact serialized LocalEvidenceSnapshotBody - the body
// carried verbatim in the SignedLocalEvidenceSnapshot envelope. Marshaled once
// and cached for a self-authored snapshot; the received bytes verbatim for a
// parsed one. The returned slice is the internal cache - callers must not
// mutate it.
func (s *LocalEvidenceSnapshot) bodyBytes() ([]byte, error) {
	if s == nil {
		return nil, errors.New("roast: cannot encode a nil snapshot")
	}
	if s.bodyCache != nil {
		return s.bodyCache, nil
	}
	body, err := proto.Marshal(snapshotBodyMessage(s))
	if err != nil {
		return nil, fmt.Errorf("roast: marshal snapshot body: %w", err)
	}
	s.bodyCache = body
	return body, nil
}

// SignableBytes returns the exact byte stream the OperatorSignature covers:
// the snapshot domain tag (localEvidenceSnapshotSignatureDomain) followed by
// the serialized LocalEvidenceSnapshotBody. The domain tag is a fixed constant
// prepended by both signer and verifier and is NOT carried on the wire - it
// domain-separates this signature from the node's other signed bodies (see the
// package comment). The body half is the bytes that travel: marshaled once for
// a self-authored snapshot, or the received body verbatim for a parsed one
// (verify exactly what was received). Evidence fields must not be mutated
// afterwards, and the returned slice is the internal cache - callers must not
// mutate it.
func (s *LocalEvidenceSnapshot) SignableBytes() ([]byte, error) {
	if s == nil {
		return nil, errors.New("roast: cannot encode a nil snapshot")
	}
	if s.signaturePayloadCache != nil {
		return s.signaturePayloadCache, nil
	}
	body, err := s.bodyBytes()
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 0, len(localEvidenceSnapshotSignatureDomain)+len(body))
	payload = append(payload, localEvidenceSnapshotSignatureDomain...)
	payload = append(payload, body...)
	s.signaturePayloadCache = payload
	return payload, nil
}

// wireEnvelopeBytes returns the exact on-wire SignedLocalEvidenceSnapshot
// envelope. For parsed snapshots this is the received envelope verbatim;
// for self-authored snapshots it is built once (after signing) and cached,
// so the broadcast bytes and the bytes embedded into a coordinator bundle
// are identical.
func (s *LocalEvidenceSnapshot) wireEnvelopeBytes() ([]byte, error) {
	if s.wireEnvelope != nil {
		return s.wireEnvelope, nil
	}
	if len(s.OperatorSignature) == 0 {
		return nil, errors.New(
			"roast: snapshot must be signed before wire encoding",
		)
	}
	body, err := s.bodyBytes()
	if err != nil {
		return nil, err
	}
	envelope, err := proto.Marshal(&pb.SignedLocalEvidenceSnapshot{
		Body:              body,
		OperatorSignature: s.OperatorSignature,
	})
	if err != nil {
		return nil, fmt.Errorf("roast: marshal snapshot envelope: %w", err)
	}
	s.wireEnvelope = envelope
	return envelope, nil
}

// bodyBytes returns the exact serialized TransitionMessageBody, which embeds
// every snapshot's signed envelope verbatim - the body carried in the
// SignedTransitionMessage envelope. Built and cached once for a self-authored
// message; the received bytes verbatim for a parsed one. The returned slice is
// the internal cache - callers must not mutate it.
func (m *TransitionMessage) bodyBytes() ([]byte, error) {
	if m == nil {
		return nil, errors.New("roast: cannot encode a nil transition message")
	}
	if m.bodyCache != nil {
		return m.bodyCache, nil
	}
	body := &pb.TransitionMessageBody{
		AttemptContextHash: m.AttemptContextHash,
		CoordinatorId:      m.CoordinatorIDValue,
	}
	for i := range m.Bundle {
		envelope, err := m.Bundle[i].wireEnvelopeBytes()
		if err != nil {
			return nil, fmt.Errorf("roast: bundle[%d]: %w", i, err)
		}
		body.SignedSnapshots = append(body.SignedSnapshots, envelope)
	}
	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("roast: marshal transition body: %w", err)
	}
	m.bodyCache = bodyBytes
	return bodyBytes, nil
}

// SignableBytes returns the exact byte stream the CoordinatorSignature covers:
// the transition domain tag (transitionMessageSignatureDomain) followed by the
// serialized TransitionMessageBody. The domain tag is a fixed constant
// prepended by both signer and verifier and is NOT carried on the wire - it
// domain-separates the coordinator's bundle signature from its other signed
// bodies (see the package comment). The coordinator's signature attests that
// these specific signed snapshots were assembled in this specific order; for a
// message parsed off the wire the body half is the received bytes verbatim.
// The returned slice is the internal cache - callers must not mutate it.
func (m *TransitionMessage) SignableBytes() ([]byte, error) {
	if m == nil {
		return nil, errors.New("roast: cannot encode a nil transition message")
	}
	if m.signaturePayloadCache != nil {
		return m.signaturePayloadCache, nil
	}
	body, err := m.bodyBytes()
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 0, len(transitionMessageSignatureDomain)+len(body))
	payload = append(payload, transitionMessageSignatureDomain...)
	payload = append(payload, body...)
	m.signaturePayloadCache = payload
	return payload, nil
}
