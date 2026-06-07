package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestShouldAcceptFrostDKGResultSignatureMessageRejectsLocalSeats(
	t *testing.T,
) {
	includedMembersSet := map[group.MemberIndex]struct{}{
		1: {},
		2: {},
		3: {},
	}
	localMembersSet := map[group.MemberIndex]struct{}{
		1: {},
		3: {},
	}

	if shouldAcceptFrostDKGResultSignatureMessage(
		&frostDKGResultSignatureMessage{
			SenderIDValue: 3,
			SessionID:     "session",
		},
		nil,
		"session",
		localMembersSet,
		includedMembersSet,
		nil,
	) {
		t.Fatal("expected local member signature message to be rejected")
	}

	if !shouldAcceptFrostDKGResultSignatureMessage(
		&frostDKGResultSignatureMessage{
			SenderIDValue: 2,
			SessionID:     "session",
		},
		nil,
		"session",
		localMembersSet,
		includedMembersSet,
		nil,
	) {
		t.Fatal("expected remote included member signature message to be accepted")
	}
}
