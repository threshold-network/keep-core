//go:build frost_native

package signing

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// testRoundContributionMessage is a test-only stand-in for the (removed) coarse
// tbtc-signer round-contribution message. The coarse signing path that owned the
// production message type was deleted, but the shared, generic infrastructure it
// exercised - enqueueOrRecordOverflow (evidence_overflow.go), the attempt-context
// hash field helpers (attempt_context_binding.go), and
// verifyMessageAttemptContextHash / setMessageAttemptContextHashIfBound
// (attempt_context_binding_validation_frost_native.go) - is preserved. This type
// is a faithful copy of the deleted message (same fields, same interface methods)
// so those helpers keep their unit coverage without depending on the coarse path.
type testRoundContributionMessage struct {
	SenderIDValue          uint32 `json:"senderID"`
	SessionIDValue         string `json:"sessionID"`
	ContributionIdentifier uint16 `json:"contributionIdentifier"`
	ContributionData       []byte `json:"contributionData"`
	// AttemptContextHash binds this contribution to an RFC-21 attempt
	// context when ROAST retry has registered one for the session.
	AttemptContextHash []byte `json:"attemptContextHash,omitempty"`
}

func (m *testRoundContributionMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(m.SenderIDValue)
}

func (m *testRoundContributionMessage) SessionID() string {
	return m.SessionIDValue
}

func (m *testRoundContributionMessage) Type() string {
	return "frost_signing/native_tbtc_signer/round_contribution"
}

func (m *testRoundContributionMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *testRoundContributionMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, m); err != nil {
		return err
	}

	if m.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}

	if m.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}

	if m.ContributionIdentifier == 0 {
		return fmt.Errorf("contribution identifier is zero")
	}

	if len(m.ContributionData) == 0 {
		return fmt.Errorf("contribution data is empty")
	}

	if err := validateAttemptContextHashField(
		m.AttemptContextHash,
	); err != nil {
		return err
	}

	return nil
}

func (m *testRoundContributionMessage) SetAttemptContextHash(
	hash [AttemptContextHashFieldLength]byte,
) {
	m.AttemptContextHash = attemptContextHashFieldFromArray(hash)
}

func (m *testRoundContributionMessage) GetAttemptContextHash() (
	[AttemptContextHashFieldLength]byte, bool,
) {
	return attemptContextHashFieldToArray(m.AttemptContextHash)
}

// testRoundContributionMessagesEqual is the test-local copy of the deleted
// coarse messagesEqual comparison, preserved so the migrated tests keep their
// exact assertions.
func testRoundContributionMessagesEqual(
	left, right *testRoundContributionMessage,
) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.SenderIDValue == right.SenderIDValue &&
		left.SessionIDValue == right.SessionIDValue &&
		left.ContributionIdentifier == right.ContributionIdentifier &&
		bytes.Equal(left.ContributionData, right.ContributionData) &&
		bytes.Equal(left.AttemptContextHash, right.AttemptContextHash)
}
