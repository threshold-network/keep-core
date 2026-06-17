//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"testing"
)

func TestRunnerTransportMessage_RoundTrip(t *testing.T) {
	original := &runnerTransportMessage{
		messageType: RunnerMsgSigningPackage,
		sender:      3,
		attempt:     [attemptContextHashLength]byte{0x42, 0x99, 0xff},
		payload:     []byte("signed-signing-package-envelope"),
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The registered unmarshaler presets messageType (it is not on the wire).
	decoded := &runnerTransportMessage{messageType: RunnerMsgSigningPackage}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != original.sender {
		t.Fatalf("sender: got [%d], want [%d]", decoded.sender, original.sender)
	}
	if decoded.attempt != original.attempt {
		t.Fatalf("attempt mismatch: got [%x], want [%x]", decoded.attempt, original.attempt)
	}
	if !bytes.Equal(decoded.payload, original.payload) {
		t.Fatalf("payload mismatch: got [%q], want [%q]", decoded.payload, original.payload)
	}
	if decoded.Type() != "frost/roast_runner/signing_package" {
		t.Fatalf("unexpected wire type: [%s]", decoded.Type())
	}
}

func TestRunnerTransportMessage_EmptyPayloadRoundTrips(t *testing.T) {
	original := &runnerTransportMessage{messageType: RunnerMsgCommitments, sender: 1}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &runnerTransportMessage{messageType: RunnerMsgCommitments}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != 1 || len(decoded.payload) != 0 {
		t.Fatalf("unexpected decode: sender [%d], payload len [%d]", decoded.sender, len(decoded.payload))
	}
}

func TestRunnerTransportMessage_RejectsInvalid(t *testing.T) {
	// Zero sender on marshal.
	if _, err := (&runnerTransportMessage{messageType: RunnerMsgCommitments, sender: 0}).Marshal(); err == nil {
		t.Fatal("expected marshal to reject a zero sender")
	}
	// Shorter than the fixed header.
	if err := (&runnerTransportMessage{}).Unmarshal([]byte{0x00, 0x01}); err == nil {
		t.Fatal("expected unmarshal to reject a short message")
	}
	// Exactly the header but a zero sender (all-zero prefix).
	if err := (&runnerTransportMessage{}).Unmarshal(make([]byte, 4+attemptContextHashLength)); err == nil {
		t.Fatal("expected unmarshal to reject a zero sender")
	}
}

func TestRunnerTransportType_CoversEveryStream(t *testing.T) {
	// Every type the subscriber demuxes must have a distinct wire type string,
	// else BroadcastChannel cannot dispatch it.
	types := []RunnerMessageType{
		RunnerMsgCommitments,
		RunnerMsgSigningPackage,
		RunnerMsgShareSubmission,
		RunnerMsgEvidenceSnapshot,
		RunnerMsgTransitionBundle,
	}
	seen := map[string]struct{}{}
	for _, mt := range types {
		s := runnerTransportType[mt]
		if s == "" {
			t.Fatalf("runner message type [%v] has no wire type string", mt)
		}
		if _, dup := seen[s]; dup {
			t.Fatalf("wire type string [%s] is not distinct", s)
		}
		seen[s] = struct{}{}
	}
}

func TestNewBroadcastChannelRunnerBus_RejectsNilDependencies(t *testing.T) {
	ctx := context.Background()
	// nil context
	if _, err := NewBroadcastChannelRunnerBus(nil, nil, nil, nil); err == nil { //nolint:staticcheck
		t.Fatal("expected nil context to be rejected")
	}
	// nil channel (other deps non-nil-checked after)
	if _, err := NewBroadcastChannelRunnerBus(ctx, nil, nil, nil); err == nil {
		t.Fatal("expected nil channel to be rejected")
	}
}
