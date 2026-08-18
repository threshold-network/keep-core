package gjkr

import (
	"math/big"
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestEphemeralPublicKeyMessageRoundtrip(t *testing.T) {
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

	msg := &EphemeralPublicKeyMessage{
		senderID:            group.MemberIndex(38),
		ephemeralPublicKeys: publicKeys,
		sessionID:           "session-1",
	}
	unmarshaled := &EphemeralPublicKeyMessage{}

	err = pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzEphemeralPublicKeyMessageRoundtrip(t *testing.T) {
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

		message := &EphemeralPublicKeyMessage{
			senderID:            senderID,
			ephemeralPublicKeys: ephemeralPublicKeys,
			sessionID:           sessionID,
		}

		_ = pbutils.RoundTrip(message, &EphemeralPublicKeyMessage{})
	}
}

func TestFuzzEphemeralPublicKeyMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&EphemeralPublicKeyMessage{})
}

func TestMemberCommitmentsMessageRoundtrip(t *testing.T) {
	msg := &MemberCommitmentsMessage{
		senderID: group.MemberIndex(141),
		commitments: []*bn256.G1{
			new(bn256.G1).ScalarBaseMult(big.NewInt(966)),
			new(bn256.G1).ScalarBaseMult(big.NewInt(1385)),
			new(bn256.G1).ScalarBaseMult(big.NewInt(1569)),
		},
		sessionID: "session-1",
	}
	unmarshaled := &MemberCommitmentsMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzMemberCommitmentsMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID    group.MemberIndex
			commitments []*bn256.G1
			sessionID   string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&commitments)
		f.Fuzz(&sessionID)

		message := &MemberCommitmentsMessage{
			senderID:    senderID,
			commitments: commitments,
			sessionID:   sessionID,
		}

		_ = pbutils.RoundTrip(message, &MemberCommitmentsMessage{})
	}
}

func TestFuzzMemberCommitmentsMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&MemberCommitmentsMessage{})
}

func TestPeerSharesMessageRoundtrip(t *testing.T) {
	shares := make(map[group.MemberIndex]*peerShares)
	shares[group.MemberIndex(112)] = &peerShares{
		encryptedShareS: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		encryptedShareT: []byte{0x0F, 0x0E, 0x0D, 0x0C, 0x0B},
	}
	shares[group.MemberIndex(223)] = &peerShares{
		encryptedShareS: []byte{0x0A, 0x0E, 0x0F, 0x0F, 0x0F},
		encryptedShareT: []byte{0x01, 0x0F, 0x0E, 0x0E, 0x0D},
	}

	msg := &PeerSharesMessage{
		senderID:  group.MemberIndex(97),
		shares:    shares,
		sessionID: "session-1",
	}

	unmarshaled := &PeerSharesMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzPeerSharesMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			shares    map[group.MemberIndex]*peerShares
			sessionID string
		)

		fuzzPeerShares := func(shares *peerShares, c fuzz.Continue) {
			var encryptedShareS, encryptedShareT []byte

			c.Fuzz(&encryptedShareS)
			c.Fuzz(&encryptedShareT)

			shares.encryptedShareS = encryptedShareS
			shares.encryptedShareT = encryptedShareT
		}

		fuzzFuncs := []interface{}{fuzzPeerShares}
		fuzzFuncs = append(fuzzFuncs, pbutils.FuzzFuncs()...)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(fuzzFuncs...)

		f.Fuzz(&senderID)
		f.Fuzz(&shares)
		f.Fuzz(&sessionID)

		message := &PeerSharesMessage{
			senderID:  senderID,
			shares:    shares,
			sessionID: sessionID,
		}

		_ = pbutils.RoundTrip(message, &PeerSharesMessage{})
	}
}

func TestFuzzPeerSharesMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&PeerSharesMessage{})
}

func TestSecretSharesAccusationsMessageRoundtrip(t *testing.T) {
	keyPair1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyPair2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	msg := &SecretSharesAccusationsMessage{
		senderID: group.MemberIndex(121),
		accusedMembersKeys: map[group.MemberIndex]*ephemeral.PrivateKey{
			group.MemberIndex(12): keyPair1.PrivateKey,
			group.MemberIndex(92): keyPair2.PrivateKey,
		},
		sessionID: "session-1",
	}
	unmarshaled := &SecretSharesAccusationsMessage{}

	err = pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzSecretSharesAccusationsMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID           group.MemberIndex
			accusedMembersKeys map[group.MemberIndex]*ephemeral.PrivateKey
			sessionID          string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&accusedMembersKeys)
		f.Fuzz(&sessionID)

		message := &SecretSharesAccusationsMessage{
			senderID:           senderID,
			accusedMembersKeys: accusedMembersKeys,
			sessionID:          sessionID,
		}

		_ = pbutils.RoundTrip(message, &SecretSharesAccusationsMessage{})
	}
}

func TestFuzzSecretSharesAccusationsMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&SecretSharesAccusationsMessage{})
}

func TestMemberPublicKeySharePointsMessageRoundtrip(t *testing.T) {
	msg := &MemberPublicKeySharePointsMessage{
		senderID: group.MemberIndex(98),
		publicKeySharePoints: []*bn256.G2{
			new(bn256.G2).ScalarBaseMult(big.NewInt(18211)),
			new(bn256.G2).ScalarBaseMult(big.NewInt(12311)),
			new(bn256.G2).ScalarBaseMult(big.NewInt(18828)),
			new(bn256.G2).ScalarBaseMult(big.NewInt(88711)),
		},
		sessionID: "session-1",
	}
	unmarshaled := &MemberPublicKeySharePointsMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzMemberPublicKeySharePointsMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID             group.MemberIndex
			publicKeySharePoints []*bn256.G2
			sessionID            string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&publicKeySharePoints)
		f.Fuzz(&sessionID)

		message := &MemberPublicKeySharePointsMessage{
			senderID:             senderID,
			publicKeySharePoints: publicKeySharePoints,
			sessionID:            sessionID,
		}

		_ = pbutils.RoundTrip(message, &MemberPublicKeySharePointsMessage{})
	}
}

func TestFuzzMemberPublicKeySharePointsMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&MemberPublicKeySharePointsMessage{})
}

func TestPointsAccusationsMessageRoundtrip(t *testing.T) {
	keyPair1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyPair2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	msg := &PointsAccusationsMessage{
		senderID: group.MemberIndex(141),
		accusedMembersKeys: map[group.MemberIndex]*ephemeral.PrivateKey{
			group.MemberIndex(41): keyPair1.PrivateKey,
			group.MemberIndex(11): keyPair2.PrivateKey,
		},
		sessionID: "session-1",
	}
	unmarshaled := &PointsAccusationsMessage{}

	err = pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzPointsAccusationsMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID           group.MemberIndex
			accusedMembersKeys map[group.MemberIndex]*ephemeral.PrivateKey
			sessionID          string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&accusedMembersKeys)
		f.Fuzz(&sessionID)

		message := &PointsAccusationsMessage{
			senderID:           senderID,
			accusedMembersKeys: accusedMembersKeys,
			sessionID:          sessionID,
		}

		_ = pbutils.RoundTrip(message, &PointsAccusationsMessage{})
	}
}

func TestFuzzPointsAccusationsMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&PointsAccusationsMessage{})
}

func TestMisbehavedEphemeralKeysMessageRoundtrip(t *testing.T) {
	keyPair1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyPair2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	msg := &MisbehavedEphemeralKeysMessage{
		senderID: group.MemberIndex(18),
		privateKeys: map[group.MemberIndex]*ephemeral.PrivateKey{
			group.MemberIndex(181): keyPair1.PrivateKey,
			group.MemberIndex(88):  keyPair2.PrivateKey,
		},
		sessionID: "session-1",
	}
	unmarshaled := &MisbehavedEphemeralKeysMessage{}

	err = pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzMisbehavedEphemeralKeysMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID    group.MemberIndex
			privateKeys map[group.MemberIndex]*ephemeral.PrivateKey
			sessionID   string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&privateKeys)
		f.Fuzz(&sessionID)

		message := &MisbehavedEphemeralKeysMessage{
			senderID:    senderID,
			privateKeys: privateKeys,
			sessionID:   sessionID,
		}

		_ = pbutils.RoundTrip(message, &MisbehavedEphemeralKeysMessage{})
	}
}

func TestFuzzMisbehavedEphemeralKeysMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&MisbehavedEphemeralKeysMessage{})
}

// --- Benchmarks ---

// buildEphemeralKeyMap generates n key pairs and returns the public key map as
// it would appear in a real EphemeralPublicKeyMessage (one entry per peer).
func buildEphemeralKeyMap(b *testing.B, n int) map[group.MemberIndex]*ephemeral.PublicKey {
	b.Helper()
	m := make(map[group.MemberIndex]*ephemeral.PublicKey, n)
	for i := 0; i < n; i++ {
		kp, err := ephemeral.GenerateKeyPair()
		if err != nil {
			b.Fatal(err)
		}
		m[group.MemberIndex(i+1)] = kp.PublicKey
	}
	return m
}

func BenchmarkMarshalEphemeralPublicKeyMessage(b *testing.B) {
	kp1, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	kp2, err := ephemeral.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	msg := &EphemeralPublicKeyMessage{
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
	msg := &EphemeralPublicKeyMessage{
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
		_ = new(EphemeralPublicKeyMessage).Unmarshal(data)
	}
}

// BenchmarkMarshalEphemeralPublicKeyMessage_64Keys benchmarks marshaling with
// the beacon group size (64 members = 63 peer keys per message).
func BenchmarkMarshalEphemeralPublicKeyMessage_64Keys(b *testing.B) {
	msg := &EphemeralPublicKeyMessage{
		senderID:            group.MemberIndex(1),
		ephemeralPublicKeys: buildEphemeralKeyMap(b, 63),
		sessionID:           "session-1",
	}
	b.ResetTimer()
	for range b.N {
		_, _ = msg.Marshal()
	}
}

// BenchmarkUnmarshalEphemeralPublicKeyMessage_64Keys benchmarks unmarshaling
// with the beacon group size. Each btcec.ParsePubKey call dominates; with 63
// peers this represents the real per-participant beacon DKG cost.
func BenchmarkUnmarshalEphemeralPublicKeyMessage_64Keys(b *testing.B) {
	msg := &EphemeralPublicKeyMessage{
		senderID:            group.MemberIndex(1),
		ephemeralPublicKeys: buildEphemeralKeyMap(b, 63),
		sessionID:           "session-1",
	}
	data, err := msg.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		_ = new(EphemeralPublicKeyMessage).Unmarshal(data)
	}
}
