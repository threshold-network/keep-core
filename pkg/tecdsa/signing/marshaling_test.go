package signing

import (
	fuzz "github.com/google/gofuzz"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"reflect"
	"testing"
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

	publicKeys := make(map[group.MemberIndex]*ephemeral.PublicKey)
	publicKeys[group.MemberIndex(211)] = keyPair1.PublicKey
	publicKeys[group.MemberIndex(19)] = keyPair2.PublicKey

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
			ephemeralPublicKeys map[group.MemberIndex]*ephemeral.PublicKey
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

		_ = pbutils.RoundTrip(message, &ephemeralPublicKeyMessage{})
	}
}

func TestFuzzEphemeralPublicKeyMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&ephemeralPublicKeyMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundOneMessage{})
	}
}

func TestFuzzTssRoundOneMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundOneMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundTwoMessage{})
	}
}

func TestFuzzTssRoundTwoMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundTwoMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundThreeMessage{})
	}
}

func TestFuzzTssRoundThreeMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundThreeMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundFourMessage{})
	}
}

func TestFuzzTssRoundFourMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundFourMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundFiveMessage{})
	}
}

func TestFuzzTssRoundFiveMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundFiveMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundSixMessage{})
	}
}

func TestFuzzTssRoundSixMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundSixMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundSevenMessage{})
	}
}

func TestFuzzTssRoundSevenMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundSevenMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundEightMessage{})
	}
}

func TestFuzzTssRoundEightMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundEightMessage{})
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

		_ = pbutils.RoundTrip(message, &tssRoundNineMessage{})
	}
}

func TestFuzzTssRoundNineMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&tssRoundNineMessage{})
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
		ephemeralPublicKeys: map[group.MemberIndex]*ephemeral.PublicKey{
			group.MemberIndex(211): kp1.PublicKey,
			group.MemberIndex(19):  kp2.PublicKey,
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
		ephemeralPublicKeys: map[group.MemberIndex]*ephemeral.PublicKey{
			group.MemberIndex(211): kp1.PublicKey,
			group.MemberIndex(19):  kp2.PublicKey,
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
		ephemeralPublicKeys: map[group.MemberIndex]*ephemeral.PublicKey{
			group.MemberIndex(211): kp1.PublicKey,
			group.MemberIndex(19):  kp2.PublicKey,
		},
		sessionID: "session-1",
	}
	b.ResetTimer()
	for range b.N {
		data, _ := msg.Marshal()
		_ = new(ephemeralPublicKeyMessage).Unmarshal(data)
	}
}
