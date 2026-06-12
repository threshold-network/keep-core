package roast

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// encodeSnapshotForTest builds a SignedLocalEvidenceSnapshot envelope from
// arbitrary (possibly invalid) in-memory fields, bypassing the production
// signing/validation path, so tests can exercise Unmarshal's rejection of
// structurally invalid wire bytes.
func encodeSnapshotForTest(t *testing.T, s *LocalEvidenceSnapshot) []byte {
	t.Helper()
	body, err := proto.Marshal(snapshotBodyMessage(s))
	if err != nil {
		t.Fatalf("encode snapshot body: %v", err)
	}
	envelope, err := proto.Marshal(&pb.SignedLocalEvidenceSnapshot{
		Body:              body,
		OperatorSignature: s.OperatorSignature,
	})
	if err != nil {
		t.Fatalf("encode snapshot envelope: %v", err)
	}
	return envelope
}

// encodeTransitionForTest builds a SignedTransitionMessage envelope from
// arbitrary (possibly invalid) in-memory fields; see encodeSnapshotForTest.
func encodeTransitionForTest(t *testing.T, m *TransitionMessage) []byte {
	t.Helper()
	body := &pb.TransitionMessageBody{
		AttemptContextHash: m.AttemptContextHash,
		CoordinatorId:      m.CoordinatorIDValue,
	}
	for i := range m.Bundle {
		body.SignedSnapshots = append(
			body.SignedSnapshots,
			encodeSnapshotForTest(t, &m.Bundle[i]),
		)
	}
	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		t.Fatalf("encode transition body: %v", err)
	}
	envelope, err := proto.Marshal(&pb.SignedTransitionMessage{
		Body:                 bodyBytes,
		CoordinatorSignature: m.CoordinatorSignature,
	})
	if err != nil {
		t.Fatalf("encode transition envelope: %v", err)
	}
	return envelope
}

var pinnedContextHash = [attempt.MessageDigestLength]byte{
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
	0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
	0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
}

func TestLocalEvidenceSnapshot_TypeIsStable(t *testing.T) {
	s := &LocalEvidenceSnapshot{}
	if got := s.Type(); got != LocalEvidenceSnapshotType {
		t.Fatalf("Type() = %q, want %q", got, LocalEvidenceSnapshotType)
	}
	if !strings.HasPrefix(LocalEvidenceSnapshotType, roastMessageTypePrefix) {
		t.Fatalf(
			"Type() must be under the %q prefix; got %q",
			roastMessageTypePrefix, LocalEvidenceSnapshotType,
		)
	}
}

func TestNewLocalEvidenceSnapshot_SortsOverflows(t *testing.T) {
	evidence := attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{
			5: 3,
			1: 2,
			3: 1,
		},
	}
	s := NewLocalEvidenceSnapshot(7, pinnedContextHash, evidence)

	if len(s.Overflows) != 3 {
		t.Fatalf("expected 3 overflow entries, got %d", len(s.Overflows))
	}
	for i := 1; i < len(s.Overflows); i++ {
		if s.Overflows[i].Sender <= s.Overflows[i-1].Sender {
			t.Fatalf(
				"overflows not sorted ascending at index %d: %v",
				i, s.Overflows,
			)
		}
	}
	if s.SenderIDValue != 7 {
		t.Fatalf("SenderIDValue = %d, want 7", s.SenderIDValue)
	}
	if !bytes.Equal(s.AttemptContextHash, pinnedContextHash[:]) {
		t.Fatalf(
			"AttemptContextHash mismatch: got %x want %x",
			s.AttemptContextHash, pinnedContextHash[:],
		)
	}
}

func TestNewLocalEvidenceSnapshot_EmptyEvidenceOmitsOverflows(t *testing.T) {
	s := NewLocalEvidenceSnapshot(1, pinnedContextHash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{},
	})
	if len(s.Overflows) != 0 {
		t.Fatalf("expected empty overflows, got %v", s.Overflows)
	}
	s.OperatorSignature = bytes.Repeat([]byte{0xab}, 64)
	data, err := s.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Overflows) != 0 {
		t.Fatalf(
			"empty overflows must stay empty through the wire; got %v",
			decoded.Overflows,
		)
	}
}

func TestLocalEvidenceSnapshot_RoundTrip(t *testing.T) {
	original := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{
			1: 2,
			3: 1,
			5: 3,
		},
	})
	original.OperatorSignature = bytes.Repeat([]byte{0xab}, 64)

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SenderIDValue != original.SenderIDValue {
		t.Fatalf("sender mismatch")
	}
	if !bytes.Equal(decoded.AttemptContextHash, original.AttemptContextHash) {
		t.Fatalf("attempt context hash mismatch")
	}
	if len(decoded.Overflows) != len(original.Overflows) {
		t.Fatalf(
			"overflow length mismatch: %d vs %d",
			len(decoded.Overflows), len(original.Overflows),
		)
	}
	if !bytes.Equal(decoded.OperatorSignature, original.OperatorSignature) {
		t.Fatalf("signature mismatch")
	}
}

func TestLocalEvidenceSnapshot_RejectsZeroSender(t *testing.T) {
	s := &LocalEvidenceSnapshot{
		SenderIDValue:      0,
		AttemptContextHash: pinnedContextHash[:],
	}
	data := encodeSnapshotForTest(t, s)
	err := (&LocalEvidenceSnapshot{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "senderID is zero") {
		t.Fatalf("expected zero-sender error, got %v", err)
	}
}

func TestLocalEvidenceSnapshot_RejectsWrongHashLength(t *testing.T) {
	bad := encodeSnapshotForTest(t, &LocalEvidenceSnapshot{
		SenderIDValue:      1,
		AttemptContextHash: []byte{0x00, 0x01, 0x02},
	})
	err := (&LocalEvidenceSnapshot{}).Unmarshal(bad)
	if err == nil || !strings.Contains(err.Error(), "attemptContextHash length") {
		t.Fatalf("expected hash-length error, got %v", err)
	}
}

func TestLocalEvidenceSnapshot_RejectsOversizeSignature(t *testing.T) {
	s := NewLocalEvidenceSnapshot(1, pinnedContextHash, attempt.Evidence{})
	s.OperatorSignature = bytes.Repeat([]byte{0xff}, MaxOperatorSignatureBytes+1)
	data := encodeSnapshotForTest(t, s)
	err := (&LocalEvidenceSnapshot{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "exceeds cap") {
		t.Fatalf("expected signature-cap error, got %v", err)
	}
}

func TestLocalEvidenceSnapshot_RejectsUnsortedOverflows(t *testing.T) {
	bad := &LocalEvidenceSnapshot{
		SenderIDValue:      1,
		AttemptContextHash: pinnedContextHash[:],
		Overflows: []OverflowEntry{
			{Sender: 5, Count: 1},
			{Sender: 1, Count: 1},
		},
	}
	data := encodeSnapshotForTest(t, bad)
	err := (&LocalEvidenceSnapshot{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "not sorted") {
		t.Fatalf("expected sort error, got %v", err)
	}
}

func TestLocalEvidenceSnapshot_RejectsDuplicateOverflowSender(t *testing.T) {
	bad := &LocalEvidenceSnapshot{
		SenderIDValue:      1,
		AttemptContextHash: pinnedContextHash[:],
		Overflows: []OverflowEntry{
			{Sender: 3, Count: 1},
			{Sender: 3, Count: 1},
		},
	}
	data := encodeSnapshotForTest(t, bad)
	err := (&LocalEvidenceSnapshot{}).Unmarshal(data)
	if err == nil {
		t.Fatal("expected duplicate-sender error")
	}
}

func TestLocalEvidenceSnapshot_EvidenceReconstructsMap(t *testing.T) {
	original := attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{1: 2, 3: 4},
	}
	s := NewLocalEvidenceSnapshot(7, pinnedContextHash, original)
	got := s.Evidence()
	if len(got.Overflows) != len(original.Overflows) {
		t.Fatalf(
			"map size mismatch: got %d want %d",
			len(got.Overflows), len(original.Overflows),
		)
	}
	for k, v := range original.Overflows {
		if got.Overflows[k] != v {
			t.Fatalf("overflow[%d]: got %d want %d", k, got.Overflows[k], v)
		}
	}
}

func TestLocalEvidenceSnapshot_AttemptContextHashArrayHandlesMalformed(t *testing.T) {
	s := &LocalEvidenceSnapshot{AttemptContextHash: []byte{0x01, 0x02}}
	arr := s.AttemptContextHashArray()
	var zero [attempt.MessageDigestLength]byte
	if arr != zero {
		t.Fatalf("expected zero array for malformed hash, got %x", arr)
	}
}

func TestTransitionMessage_TypeIsStable(t *testing.T) {
	m := &TransitionMessage{}
	if got := m.Type(); got != TransitionMessageType {
		t.Fatalf("Type() = %q, want %q", got, TransitionMessageType)
	}
	if !strings.HasPrefix(TransitionMessageType, roastMessageTypePrefix) {
		t.Fatalf("type prefix mismatch: %q", TransitionMessageType)
	}
}

func TestTransitionMessage_RoundTrip(t *testing.T) {
	m := buildValidTransitionMessage()
	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &TransitionMessage{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.CoordinatorIDValue != m.CoordinatorIDValue {
		t.Fatalf("coordinator id mismatch")
	}
	if len(decoded.Bundle) != len(m.Bundle) {
		t.Fatalf(
			"bundle size mismatch: %d vs %d",
			len(decoded.Bundle), len(m.Bundle),
		)
	}
	for i := range decoded.Bundle {
		if decoded.Bundle[i].SenderIDValue != m.Bundle[i].SenderIDValue {
			t.Fatalf("bundle[%d] sender mismatch", i)
		}
	}
}

func TestTransitionMessage_RejectsBadBundleOrdering(t *testing.T) {
	m := buildValidTransitionMessage()
	// Swap order to make it unsorted.
	m.Bundle[0], m.Bundle[1] = m.Bundle[1], m.Bundle[0]
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "not sorted") {
		t.Fatalf("expected sort error, got %v", err)
	}
}

func TestTransitionMessage_RejectsMismatchedBundleHash(t *testing.T) {
	m := buildValidTransitionMessage()
	// Mutate the first bundled snapshot's hash so it disagrees
	// with the bundle-level hash.
	m.Bundle[0].AttemptContextHash = make([]byte, attempt.MessageDigestLength)
	for i := range m.Bundle[0].AttemptContextHash {
		m.Bundle[0].AttemptContextHash[i] = 0xff
	}
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "does not match bundle hash") {
		t.Fatalf("expected hash-mismatch error, got %v", err)
	}
}

func TestTransitionMessage_RejectsEmptyBundle(t *testing.T) {
	m := buildValidTransitionMessage()
	m.Bundle = nil
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected empty-bundle error, got %v", err)
	}
}

func TestTransitionMessage_RejectsOversizeBundle(t *testing.T) {
	m := buildValidTransitionMessage()
	// Grow bundle beyond the cap by duplicating with monotonically
	// increasing senders.
	m.Bundle = make([]LocalEvidenceSnapshot, MaxSnapshotsPerBundle+1)
	for i := range m.Bundle {
		m.Bundle[i] = LocalEvidenceSnapshot{
			SenderIDValue:      uint32(i + 1),
			AttemptContextHash: append([]byte{}, m.AttemptContextHash...),
		}
	}
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "exceeds cap") {
		t.Fatalf("expected oversize-bundle error, got %v", err)
	}
}

func TestTransitionMessage_RejectsZeroCoordinatorID(t *testing.T) {
	m := buildValidTransitionMessage()
	m.CoordinatorIDValue = 0
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "coordinatorID is zero") {
		t.Fatalf("expected zero-coordinator error, got %v", err)
	}
}

func TestTransitionMessage_RejectsOversizeCoordinatorSignature(t *testing.T) {
	m := buildValidTransitionMessage()
	m.CoordinatorSignature = bytes.Repeat([]byte{0xff}, MaxCoordinatorSignatureBytes+1)
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "exceeds cap") {
		t.Fatalf("expected oversize-signature error, got %v", err)
	}
}

func TestTransitionMessage_RejectsBundleWithInvalidSnapshot(t *testing.T) {
	m := buildValidTransitionMessage()
	m.Bundle[0].SenderIDValue = 0
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil || !strings.Contains(err.Error(), "senderID is zero") {
		t.Fatalf("expected invalid-snapshot error, got %v", err)
	}
}

func TestTransitionMessage_RejectsDuplicateBundleSender(t *testing.T) {
	m := buildValidTransitionMessage()
	m.Bundle[1].SenderIDValue = m.Bundle[0].SenderIDValue
	data := encodeTransitionForTest(t, m)
	err := (&TransitionMessage{}).Unmarshal(data)
	if err == nil {
		t.Fatal("expected duplicate-sender error")
	}
}

func TestTransitionMessage_DeterministicEncodingForIdenticalInputs(t *testing.T) {
	a := buildValidTransitionMessage()
	b := buildValidTransitionMessage()
	dataA, err := a.Marshal()
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	dataB, err := b.Marshal()
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	if !bytes.Equal(dataA, dataB) {
		t.Fatalf(
			"identical inputs produced different wire bytes:\n a=%x\n b=%x",
			dataA, dataB,
		)
	}
}

func buildValidTransitionMessage() *TransitionMessage {
	mkSnap := func(sender group.MemberIndex) LocalEvidenceSnapshot {
		return LocalEvidenceSnapshot{
			SenderIDValue:      uint32(sender),
			AttemptContextHash: append([]byte{}, pinnedContextHash[:]...),
			Overflows: []OverflowEntry{
				{Sender: 99, Count: 1},
			},
			OperatorSignature: bytes.Repeat([]byte{0xab}, 64),
		}
	}
	return &TransitionMessage{
		AttemptContextHash: append([]byte{}, pinnedContextHash[:]...),
		CoordinatorIDValue: 1,
		Bundle: []LocalEvidenceSnapshot{
			mkSnap(1),
			mkSnap(2),
			mkSnap(3),
		},
		CoordinatorSignature: bytes.Repeat([]byte{0xee}, 64),
	}
}
