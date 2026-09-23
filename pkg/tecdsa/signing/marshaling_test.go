package signing

import (
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestEphemeralPublicKeyMessage_MarshalingRoundtrip(t *testing.T) {
	keyPair1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyPair2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	publicKeys := map[group.MemberIndex][]byte{
		group.MemberIndex(211): keyPair1.PublicKey.Marshal(),
		group.MemberIndex(19):  keyPair2.PublicKey.Marshal(),
	}

	msg := &ephemeralPublicKeyMessage{
		senderID:            group.MemberIndex(38),
		ephemeralPublicKeys: publicKeys,
		sessionID:           "session-1",
	}
	unmarshaled := &ephemeralPublicKeyMessage{}

	err = pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzEphemeralPublicKeyMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID            group.MemberIndex
			ephemeralPublicKeys map[group.MemberIndex][]byte
			sessionID           string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&ephemeralPublicKeys)
		f.Fuzz(&sessionID)

		message := &ephemeralPublicKeyMessage{
			senderID:            senderID,
			ephemeralPublicKeys: ephemeralPublicKeys,
			sessionID:           sessionID,
		}

		if err := pbutils.RoundTrip(message, &ephemeralPublicKeyMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzEphemeralPublicKeyMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&ephemeralPublicKeyMessage{})
}

func TestTssRoundOneMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundOneMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		peersPayload: map[group.MemberIndex][]byte{
			1: {6, 7, 8, 9, 10},
			2: {11, 12, 13, 14, 15},
		},
		sessionID: "session-1",
	}
	unmarshaled := &tssRoundOneMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundOneMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID         group.MemberIndex
			broadcastPayload []byte
			peersPayload     map[group.MemberIndex][]byte
			sessionID        string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&broadcastPayload)
		f.Fuzz(&peersPayload)
		f.Fuzz(&sessionID)

		message := &tssRoundOneMessage{
			senderID:         senderID,
			broadcastPayload: broadcastPayload,
			peersPayload:     peersPayload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundOneMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundOneMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundOneMessage{})
}

func TestTssRoundTwoMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundTwoMessage{
		senderID: group.MemberIndex(50),
		peersPayload: map[group.MemberIndex][]byte{
			1: {6, 7, 8, 9, 10},
			2: {11, 12, 13, 14, 15},
		},
		sessionID: "session-1",
	}
	unmarshaled := &tssRoundTwoMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundTwoMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID     group.MemberIndex
			peersPayload map[group.MemberIndex][]byte
			sessionID    string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&peersPayload)
		f.Fuzz(&sessionID)

		message := &tssRoundTwoMessage{
			senderID:     senderID,
			peersPayload: peersPayload,
			sessionID:    sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundTwoMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundTwoMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundTwoMessage{})
}

func TestTssRoundThreeMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundThreeMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundThreeMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundThreeMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundThreeMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundThreeMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundThreeMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundThreeMessage{})
}

func TestTssRoundFourMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundFourMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundFourMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundFourMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundFourMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundFourMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundFourMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundFourMessage{})
}

func TestTssRoundFiveMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundFiveMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundFiveMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundFiveMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundFiveMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundFiveMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundFiveMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundFiveMessage{})
}

func TestTssRoundSixMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundSixMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundSixMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundSixMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundSixMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundSixMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundSixMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundSixMessage{})
}

func TestTssRoundSevenMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundSevenMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundSevenMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundSevenMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundSevenMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundSevenMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundSevenMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundSevenMessage{})
}

func TestTssRoundEightMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundEightMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundEightMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundEightMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundEightMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundEightMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundEightMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundEightMessage{})
}

func TestTssRoundNineMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &tssRoundNineMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		sessionID:        "session-1",
	}
	unmarshaled := &tssRoundNineMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzTssRoundNineMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			payload   []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&payload)
		f.Fuzz(&sessionID)

		message := &tssRoundNineMessage{
			senderID:         senderID,
			broadcastPayload: payload,
			sessionID:        sessionID,
		}

		if err := pbutils.RoundTrip(message, &tssRoundNineMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzTssRoundNineMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&tssRoundNineMessage{})
}

// --- Benchmarks ---

func BenchmarkMarshalEphemeralPublicKeyMessage(b *testing.B) {
	kp1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	kp2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	msg := &ephemeralPublicKeyMessage{
		senderID: group.MemberIndex(38),
		ephemeralPublicKeys: map[group.MemberIndex][]byte{
			group.MemberIndex(211): kp1.PublicKey.Marshal(),
			group.MemberIndex(19):  kp2.PublicKey.Marshal(),
		},
		sessionID: "session-1",
	}
	b.ResetTimer()
	for range b.N {
		_, _ = msg.Marshal()
	}
}

func BenchmarkUnmarshalEphemeralPublicKeyMessage(b *testing.B) {
	kp1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	kp2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	msg := &ephemeralPublicKeyMessage{
		senderID: group.MemberIndex(38),
		ephemeralPublicKeys: map[group.MemberIndex][]byte{
			group.MemberIndex(211): kp1.PublicKey.Marshal(),
			group.MemberIndex(19):  kp2.PublicKey.Marshal(),
		},
		sessionID: "session-1",
	}
	data, err := msg.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		_ = new(ephemeralPublicKeyMessage).Unmarshal(data)
	}
}

// buildEphemeralKeyMap generates n key pairs and returns the serialized public
// key map as it would appear in a real EphemeralPublicKeyMessage (one entry per peer).
func buildEphemeralKeyMap(b *testing.B, n int) map[group.MemberIndex][]byte {
	b.Helper()
	m := make(map[group.MemberIndex][]byte, n)
	for i := 0; i < n; i++ {
		kp, err := ephemeral.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}
		m[group.MemberIndex(i+1)] = kp.PublicKey.Marshal()
	}
	return m
}

// BenchmarkMarshalEphemeralPublicKeyMessage_100Keys benchmarks marshaling with
// a realistic group size (100 members = 99 peer keys per message).
func BenchmarkMarshalEphemeralPublicKeyMessage_100Keys(b *testing.B) {
	msg := &ephemeralPublicKeyMessage{
		senderID:            group.MemberIndex(1),
		ephemeralPublicKeys: buildEphemeralKeyMap(b, 99),
		sessionID:           "session-1",
	}
	b.ResetTimer()
	for range b.N {
		_, _ = msg.Marshal()
	}
}

// Benchmarks unmarshaling the wire-format bytes. EC point parsing is
// deferred to use-time in generateSymmetricKeys (protocol.go).
func BenchmarkUnmarshalEphemeralPublicKeyMessage_100Keys(b *testing.B) {
	msg := &ephemeralPublicKeyMessage{
		senderID:            group.MemberIndex(1),
		ephemeralPublicKeys: buildEphemeralKeyMap(b, 99),
		sessionID:           "session-1",
	}
	data, err := msg.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		_ = new(ephemeralPublicKeyMessage).Unmarshal(data)
	}
}

// BenchmarkMarshalSigningShareMessage benchmarks the heaviest per-member
// message in a signing round: round-one carries both broadcast and peer
// payloads.
func BenchmarkMarshalSigningShareMessage(b *testing.B) {
	msg := &tssRoundOneMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		peersPayload: map[group.MemberIndex][]byte{
			1: {6, 7, 8, 9, 10},
			2: {11, 12, 13, 14, 15},
		},
		sessionID: "session-1",
	}
	b.ResetTimer()
	for range b.N {
		_, _ = msg.Marshal()
	}
}

func BenchmarkUnmarshalSigningShareMessage(b *testing.B) {
	msg := &tssRoundOneMessage{
		senderID:         group.MemberIndex(50),
		broadcastPayload: []byte{1, 2, 3, 4, 5},
		peersPayload: map[group.MemberIndex][]byte{
			1: {6, 7, 8, 9, 10},
			2: {11, 12, 13, 14, 15},
		},
		sessionID: "session-1",
	}
	data, err := msg.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		_ = new(tssRoundOneMessage).Unmarshal(data)
	}
}

func BenchmarkRoundTripEphemeralKey(b *testing.B) {
	kp1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	kp2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	msg := &ephemeralPublicKeyMessage{
		senderID: group.MemberIndex(38),
		ephemeralPublicKeys: map[group.MemberIndex][]byte{
			group.MemberIndex(211): kp1.PublicKey.Marshal(),
			group.MemberIndex(19):  kp2.PublicKey.Marshal(),
		},
		sessionID: "session-1",
	}
	b.ResetTimer()
	for range b.N {
		data, _ := msg.Marshal()
		_ = new(ephemeralPublicKeyMessage).Unmarshal(data)
	}
}
