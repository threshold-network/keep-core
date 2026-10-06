package entry

import (
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/pkg/protocol/group"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
)

func TestSignatureShareMessageRoundTrip(t *testing.T) {
	msg := &SignatureShareMessage{123, make([]byte, 0), "session-1"}
	unmarshaled := &SignatureShareMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if msg.senderID != unmarshaled.senderID {
		t.Errorf(
			"unexpected sender ID\nexpected: [%v]\nactual:   [%v]",
			msg.senderID,
			unmarshaled.senderID,
		)
	}

	testutils.AssertBytesEqual(t, msg.shareBytes, unmarshaled.shareBytes)
}

func TestFuzzSignatureShareMessageRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID   group.MemberIndex
			shareBytes []byte
			sessionID  string
		)

		f := fuzz.New().NilChance(0.1).NumElements(0, 512)

		f.Fuzz(&senderID)
		f.Fuzz(&shareBytes)
		f.Fuzz(&sessionID)

		message := &SignatureShareMessage{
			senderID:   senderID,
			shareBytes: shareBytes,
			sessionID:  sessionID,
		}

		if err := pbutils.RoundTrip(message, &SignatureShareMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzSignatureShareMessageUnmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&SignatureShareMessage{})
}
