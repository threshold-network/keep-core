package entry

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/beacon/entry/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestNewSignatureShareMessage(t *testing.T) {
	senderID := group.MemberIndex(7)
	shareBytes := []byte{0x01, 0x02, 0x03}
	sessionID := "session-1"

	message := NewSignatureShareMessage(senderID, shareBytes, sessionID)

	if message.SenderID() != senderID {
		t.Errorf(
			"unexpected sender ID\nexpected: [%v]\nactual:   [%v]",
			senderID,
			message.SenderID(),
		)
	}
	testutils.AssertBytesEqual(t, shareBytes, message.shareBytes)
	if message.sessionID != sessionID {
		t.Errorf(
			"unexpected session ID\nexpected: [%v]\nactual:   [%v]",
			sessionID,
			message.sessionID,
		)
	}
}

// The Type string is the wire identifier used to route this message on the
// broadcast channel. Changing it silently would break network compatibility
// with other clients, so it must stay stable.
func TestSignatureShareMessageType(t *testing.T) {
	message := &SignatureShareMessage{}

	if got := message.Type(); got != "relay/signature/share" {
		t.Errorf(
			"unexpected message type\nexpected: [relay/signature/share]\nactual:   [%v]",
			got,
		)
	}
}

// MemberIndex is a uint8 in the protocol but uint32 on the wire; Unmarshal must
// reject values that would overflow the uint8 rather than silently truncating
// them into a valid-looking but wrong member index.
func TestUnmarshalRejectsOverflowingMemberIndex(t *testing.T) {
	overflowing, err := proto.Marshal(&pb.SignatureShare{
		SenderID:  256, // maxMemberIndex is 255
		Share:     []byte{0x01},
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("failed to marshal test fixture: [%v]", err)
	}

	if err := (&SignatureShareMessage{}).Unmarshal(overflowing); err == nil {
		t.Fatal("expected an error for a member index exceeding the uint8 range")
	}
}
