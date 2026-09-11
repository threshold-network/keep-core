package inactivity

import (
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestClaimSignatureMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &claimSignatureMessage{
		senderID:  123,
		claimHash: [32]byte{0: 11, 10: 22, 31: 33},
		signature: []byte("signature"),
		publicKey: []byte("pubkey"),
		sessionID: "session-1",
	}
	unmarshaled := &claimSignatureMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzClaimSignatureMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID  group.MemberIndex
			claimHash ClaimHash
			signature []byte
			publicKey []byte
			sessionID string
		)

		f := fuzz.New().NilChance(0.1).NumElements(0, 512)

		f.Fuzz(&senderID)
		f.Fuzz(&claimHash)
		f.Fuzz(&signature)
		f.Fuzz(&publicKey)
		f.Fuzz(&sessionID)

		message := &claimSignatureMessage{
			senderID:  senderID,
			claimHash: claimHash,
			signature: signature,
			publicKey: publicKey,
			sessionID: sessionID,
		}

		_ = pbutils.RoundTrip(message, &claimSignatureMessage{})
	}
}

func TestFuzzClaimSignatureMessage_Unmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&claimSignatureMessage{})
}
