//go:build !frost_roast_retry

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestDefaultSigningParticipantSelector_IsLegacyInDefaultBuild(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(legacySigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector in default build must return legacy implementation; got %T",
			sel,
		)
	}
}

// TestLegacySelectorParksSurplusTrim pins the invariant that keeps a single real
// fault from ending a signing session: members dropped only because the included
// set exceeded the honest threshold are unblamed, so they must be reported as
// transiently parked. A ROAST consumer derives permanent exclusions by
// subtracting the parked set from the excluded set, so if the trim is reported as
// a plain exclusion the feasible set starts at exactly the threshold and the next
// exclusion aborts the session (ErrAttemptInfeasible) while honest members idle.
//
// This covers the surplus-trim class of blameless drop, which needs a multi-seat
// operator topology to fire at all. The shuffle-loss class -- the only one that
// occurs when every operator holds a single seat -- is covered by
// TestLegacySigningParticipantSelector_DelegatesToRetryShuffle.
func TestLegacySelectorParksSurplusTrim(t *testing.T) {
	const honestThreshold = 6

	// The package's standard signing-group fixture: ten member indices mapped onto
	// seven distinct operators (address-2 twice, address-8 three times). Because
	// the retry algorithm selects OPERATORS, a selection of honestThreshold
	// operators can carry more than honestThreshold member indices, which is what
	// makes the surplus trim fire.
	operators := chain.Addresses{
		"address-1", "address-2", "address-8", "address-4",
		"address-2", "address-6", "address-7", "address-8",
		"address-9", "address-8",
	}

	// Seven ready members against a threshold of six: the qualified set overshoots
	// by exactly one, so the trim drops a single unblamed member (4).
	readyMembersIndexes := []group.MemberIndex{3, 4, 6, 7, 8, 9, 10}

	selection, err := legacySigningParticipantSelector{}.Select(
		readyMembersIndexes,
		operators,
		1,
		0,
		0,
		honestThreshold,
		"",
		1,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	assertMemberIndexes(
		t,
		"included",
		[]group.MemberIndex{3, 6, 7, 8, 9, 10},
		selection.includedMembersIndexes,
	)

	// The heart of the regression: member 4 lost a coin flip, it did not fail. If
	// it is reported as a plain exclusion instead of parked, a ROAST consumer
	// computing excluded minus parked treats it as permanently excluded, leaving
	// the feasible set at exactly the threshold so the next real fault aborts the
	// session with ErrAttemptInfeasible while honest members sit idle.
	assertMemberIndexes(
		t,
		"transiently parked",
		[]group.MemberIndex{4},
		selection.transientlyParkedMembersIndexes,
	)
}

func assertMemberIndexes(
	t *testing.T,
	what string,
	expected, actual []group.MemberIndex,
) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Fatalf("%s: expected [%v], got [%v]", what, expected, actual)
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("%s: expected [%v], got [%v]", what, expected, actual)
		}
	}
}
