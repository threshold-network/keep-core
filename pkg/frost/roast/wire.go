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
}

// SignableBytes returns the exact byte stream the OperatorSignature covers:
// the serialized LocalEvidenceSnapshotBody. For a self-authored snapshot
// the body is marshaled once and cached - sign exactly what will be
// transmitted. For a snapshot parsed off the wire this returns the
// received body bytes verbatim - verify exactly what was received. The
// snapshot's evidence fields must not be mutated afterwards.
func (s *LocalEvidenceSnapshot) SignableBytes() ([]byte, error) {
	if s == nil {
		return nil, errors.New("roast: cannot encode a nil snapshot")
	}
	if s.signedBody != nil {
		return s.signedBody, nil
	}
	body, err := proto.Marshal(snapshotBodyMessage(s))
	if err != nil {
		return nil, fmt.Errorf("roast: marshal snapshot body: %w", err)
	}
	s.signedBody = body
	return body, nil
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
	body, err := s.SignableBytes()
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

// SignableBytes returns the exact byte stream the CoordinatorSignature
// covers: the serialized TransitionMessageBody, which embeds every
// snapshot's signed envelope verbatim. The coordinator's signature
// attests that these specific signed snapshots were assembled in this
// specific order. For a message parsed off the wire this returns the
// received body bytes verbatim.
func (m *TransitionMessage) SignableBytes() ([]byte, error) {
	if m == nil {
		return nil, errors.New("roast: cannot encode a nil transition message")
	}
	if m.signedBody != nil {
		return m.signedBody, nil
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
	m.signedBody = bodyBytes
	return bodyBytes, nil
}
