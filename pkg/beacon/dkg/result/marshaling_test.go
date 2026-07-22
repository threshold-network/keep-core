package result

import (
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"

	"github.com/keep-network/keep-core/pkg/internal/pbutils"
)

func TestDKGResultHashSignatureMessageRoundtrip(t *testing.T) {
	msg := &DKGResultHashSignatureMessage{
		senderIndex: 10,
		resultHash:  [32]byte{30},
		signature:   []byte("signature"),
		publicKey:   []byte("pubkey"),
		sessionID:   "session-1",
	}

	unmarshaled := &DKGResultHashSignatureMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzDKGResultHashSignatureMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderIndex group.MemberIndex
			resultHash  chain.DKGResultHash
			signature   []byte
			publicKey   []byte
			sessionID   string
		)

		f := fuzz.New().NilChance(0.1).NumElements(0, 512)

		f.Fuzz(&senderIndex)
		f.Fuzz(&resultHash)
		f.Fuzz(&signature)
		f.Fuzz(&publicKey)
		f.Fuzz(&sessionID)

		message := &DKGResultHashSignatureMessage{
			senderIndex: senderIndex,
			resultHash:  resultHash,
			signature:   signature,
			publicKey:   publicKey,
			sessionID:   sessionID,
		}

		_ = pbutils.RoundTrip(message, &DKGResultHashSignatureMessage{})
	}
}

func TestFuzzDKGResultHashSignatureMessageUnmarshaler(t *testing.T) {
	pbutils.FuzzUnmarshaler(&DKGResultHashSignatureMessage{})
}
