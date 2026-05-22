//go:build frost_native

package signing

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var pinnedAttemptContextHash = [AttemptContextHashFieldLength]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

func TestValidateAttemptContextHashField_AcceptsAbsentOrCorrectLength(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "nil is absent", input: nil},
		{name: "empty slice is absent", input: []byte{}},
		{
			name:  "exact length is accepted",
			input: pinnedAttemptContextHash[:],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAttemptContextHashField(tt.input); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAttemptContextHashField_RejectsWrongLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{name: "too short", length: 31},
		{name: "too long", length: 33},
		{name: "one byte", length: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAttemptContextHashField(
				bytes.Repeat([]byte{0xff}, tt.length),
			)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "wrong length") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAttemptContextHashField_ArrayRoundTrip(t *testing.T) {
	field := attemptContextHashFieldFromArray(pinnedAttemptContextHash)
	if len(field) != AttemptContextHashFieldLength {
		t.Fatalf(
			"expected length %d, got %d",
			AttemptContextHashFieldLength, len(field),
		)
	}
	got, present := attemptContextHashFieldToArray(field)
	if !present {
		t.Fatal("expected presence=true")
	}
	if got != pinnedAttemptContextHash {
		t.Fatalf("array round-trip mismatch: got %x want %x", got, pinnedAttemptContextHash)
	}
}

func TestAttemptContextHashField_ArrayToArrayAbsent(t *testing.T) {
	got, present := attemptContextHashFieldToArray(nil)
	if present {
		t.Fatal("expected presence=false for nil")
	}
	var zero [AttemptContextHashFieldLength]byte
	if got != zero {
		t.Fatalf("expected zero array, got %x", got)
	}
}

func TestAttemptContextHashField_FromArrayDoesNotAliasCaller(t *testing.T) {
	arr := pinnedAttemptContextHash
	field := attemptContextHashFieldFromArray(arr)
	field[0] = 0xff
	if arr[0] == 0xff {
		t.Fatal("mutation through returned slice modified caller's array")
	}
}

func TestRoundOneCommitmentMessage_OptionalFieldRoundTrip(t *testing.T) {
	original := &nativeFROSTRoundOneCommitmentMessage{
		SenderIDValue:         1,
		SessionIDValue:        "session-1",
		ParticipantIdentifier: "p1",
		CommitmentData:        []byte{0xaa, 0xbb},
	}

	t.Run("absent field round-trips as absent", func(t *testing.T) {
		data, err := original.Marshal()
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if strings.Contains(string(data), "attemptContextHash") {
			t.Fatalf(
				"absent field should be omitted by omitempty, got JSON: %s",
				string(data),
			)
		}
		decoded := &nativeFROSTRoundOneCommitmentMessage{}
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if _, present := decoded.GetAttemptContextHash(); present {
			t.Fatal("expected attempt context hash to be absent after round-trip")
		}
	})

	t.Run("present field round-trips with same value", func(t *testing.T) {
		withHash := *original
		withHash.SetAttemptContextHash(pinnedAttemptContextHash)
		data, err := withHash.Marshal()
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if !strings.Contains(string(data), "attemptContextHash") {
			t.Fatalf(
				"present field should appear in JSON, got: %s",
				string(data),
			)
		}
		decoded := &nativeFROSTRoundOneCommitmentMessage{}
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		got, present := decoded.GetAttemptContextHash()
		if !present {
			t.Fatal("expected attempt context hash to be present")
		}
		if got != pinnedAttemptContextHash {
			t.Fatalf("round-trip altered hash: got %x want %x", got, pinnedAttemptContextHash)
		}
	})
}

func TestRoundOneCommitmentMessage_BackwardCompatWithOldJSON(t *testing.T) {
	// JSON emitted by a pre-Phase-1B peer: no attemptContextHash field
	// at all. The new struct must accept it without error and report
	// the hash as absent.
	oldJSON := []byte(`{
		"senderID":1,
		"sessionID":"session-1",
		"participantIdentifier":"p1",
		"commitmentData":"qrs="
	}`)

	decoded := &nativeFROSTRoundOneCommitmentMessage{}
	if err := decoded.Unmarshal(oldJSON); err != nil {
		t.Fatalf("unmarshal of old-format JSON failed: %v", err)
	}
	if _, present := decoded.GetAttemptContextHash(); present {
		t.Fatal("expected absent hash for old-format JSON")
	}
}

func TestRoundOneCommitmentMessage_RejectsWrongLengthHashField(t *testing.T) {
	badJSON := []byte(`{
		"senderID":1,
		"sessionID":"session-1",
		"participantIdentifier":"p1",
		"commitmentData":"qrs=",
		"attemptContextHash":"AAEC"
	}`)

	decoded := &nativeFROSTRoundOneCommitmentMessage{}
	err := decoded.Unmarshal(badJSON)
	if err == nil {
		t.Fatal("expected wrong-length validation error")
	}
	if !strings.Contains(err.Error(), "wrong length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoundTwoSignatureShareMessage_OptionalFieldRoundTrip(t *testing.T) {
	withHash := &nativeFROSTRoundTwoSignatureShareMessage{
		SenderIDValue:         2,
		SessionIDValue:        "session-2",
		ParticipantIdentifier: "p2",
		SignatureShareData:    []byte{0xcc, 0xdd},
	}
	withHash.SetAttemptContextHash(pinnedAttemptContextHash)
	data, err := withHash.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded := &nativeFROSTRoundTwoSignatureShareMessage{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	got, present := decoded.GetAttemptContextHash()
	if !present || got != pinnedAttemptContextHash {
		t.Fatalf("round-trip lost hash: present=%v got=%x", present, got)
	}
}

func TestRoundTwoSignatureShareMessage_BackwardCompatWithOldJSON(t *testing.T) {
	oldJSON := []byte(`{
		"senderID":2,
		"sessionID":"session-2",
		"participantIdentifier":"p2",
		"signatureShareData":"qrs="
	}`)

	decoded := &nativeFROSTRoundTwoSignatureShareMessage{}
	if err := decoded.Unmarshal(oldJSON); err != nil {
		t.Fatalf("unmarshal of old-format JSON failed: %v", err)
	}
	if _, present := decoded.GetAttemptContextHash(); present {
		t.Fatal("expected absent hash for old-format JSON")
	}
}

func TestRoundTwoSignatureShareMessage_RejectsWrongLengthHashField(t *testing.T) {
	badJSON := []byte(`{
		"senderID":2,
		"sessionID":"session-2",
		"participantIdentifier":"p2",
		"signatureShareData":"qrs=",
		"attemptContextHash":"AAEC"
	}`)

	decoded := &nativeFROSTRoundTwoSignatureShareMessage{}
	err := decoded.Unmarshal(badJSON)
	if err == nil {
		t.Fatal("expected wrong-length validation error")
	}
}

func TestBuildTaggedTBTCSignerRoundContributionMessage_OptionalFieldRoundTrip(t *testing.T) {
	withHash := &buildTaggedTBTCSignerRoundContributionMessage{
		SenderIDValue:          3,
		SessionIDValue:         "session-3",
		ContributionIdentifier: 1,
		ContributionData:       []byte{0xee, 0xff},
	}
	withHash.SetAttemptContextHash(pinnedAttemptContextHash)
	data, err := withHash.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded := &buildTaggedTBTCSignerRoundContributionMessage{}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	got, present := decoded.GetAttemptContextHash()
	if !present || got != pinnedAttemptContextHash {
		t.Fatalf("round-trip lost hash: present=%v got=%x", present, got)
	}
}

func TestBuildTaggedTBTCSignerRoundContributionMessage_BackwardCompatWithOldJSON(t *testing.T) {
	oldJSON := []byte(`{
		"senderID":3,
		"sessionID":"session-3",
		"contributionIdentifier":1,
		"contributionData":"qrs="
	}`)

	decoded := &buildTaggedTBTCSignerRoundContributionMessage{}
	if err := decoded.Unmarshal(oldJSON); err != nil {
		t.Fatalf("unmarshal of old-format JSON failed: %v", err)
	}
	if _, present := decoded.GetAttemptContextHash(); present {
		t.Fatal("expected absent hash for old-format JSON")
	}
}

func TestBuildTaggedTBTCSignerRoundContributionMessage_RejectsWrongLengthHashField(t *testing.T) {
	badJSON := []byte(`{
		"senderID":3,
		"sessionID":"session-3",
		"contributionIdentifier":1,
		"contributionData":"qrs=",
		"attemptContextHash":"AAEC"
	}`)

	decoded := &buildTaggedTBTCSignerRoundContributionMessage{}
	err := decoded.Unmarshal(badJSON)
	if err == nil {
		t.Fatal("expected wrong-length validation error")
	}
}

func TestBuildTaggedTBTCSignerRoundContributionMessagesEqual_HashFieldDifferentiates(t *testing.T) {
	base := &buildTaggedTBTCSignerRoundContributionMessage{
		SenderIDValue:          1,
		SessionIDValue:         "session-1",
		ContributionIdentifier: 1,
		ContributionData:       []byte{0xaa},
	}
	withHashA := *base
	withHashA.SetAttemptContextHash(pinnedAttemptContextHash)

	otherHash := pinnedAttemptContextHash
	otherHash[0] ^= 0xff
	withHashB := *base
	withHashB.SetAttemptContextHash(otherHash)

	if buildTaggedTBTCSignerRoundContributionMessagesEqual(base, &withHashA) {
		t.Fatal("base (no hash) vs with-hash must compare unequal")
	}
	if buildTaggedTBTCSignerRoundContributionMessagesEqual(&withHashA, &withHashB) {
		t.Fatal("messages with different hashes must compare unequal")
	}
	withHashAClone := *base
	withHashAClone.SetAttemptContextHash(pinnedAttemptContextHash)
	if !buildTaggedTBTCSignerRoundContributionMessagesEqual(&withHashA, &withHashAClone) {
		t.Fatal("messages with the same hash must compare equal")
	}
	if !buildTaggedTBTCSignerRoundContributionMessagesEqual(base, base) {
		t.Fatal("identical-pointer comparison must be equal")
	}
}

func TestRoundOneCommitmentMessage_JSONEncoderOmitsAbsentField(t *testing.T) {
	original := &nativeFROSTRoundOneCommitmentMessage{
		SenderIDValue:         1,
		SessionIDValue:        "s",
		ParticipantIdentifier: "p",
		CommitmentData:        []byte{0xaa},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}
	if _, ok := raw["attemptContextHash"]; ok {
		t.Fatalf(
			"omitempty did not suppress absent attemptContextHash; raw=%v",
			raw,
		)
	}
}
