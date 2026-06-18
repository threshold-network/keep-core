//go:build frost_roast_retry

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func selectorTestMembers() []chain.Address {
	return []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
}

func TestDefaultSigningParticipantSelector_IsROASTInTaggedBuild(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(roastSigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector in frost_roast_retry build must return ROAST impl; got %T",
			sel,
		)
	}
}

// In PR2a the ROAST selector delegates to legacy: the transition record is
// produced + stored (the data foundation) but NOT consumed (consumption +
// distribution land in PR2b, where consuming a purely-local record would no
// longer diverge across peers). This test asserts both halves: a legacy-shaped
// result, and that an existing record is left UNTOUCHED (not consumed).
func TestROASTSelector_DelegatesToLegacyAndDoesNotConsumeRecord(t *testing.T) {
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)

	signing.RecordRoastTransition("session", 1, signing.RoastTransitionRecord{
		Bundle:            &roast.TransitionMessage{CoordinatorIDValue: 1},
		DkgGroupPublicKey: []byte{0x01, 0x02, 0x03},
	})

	sel := roastSigningParticipantSelector{}
	got, err := sel.Select(
		[]group.MemberIndex{1, 2, 3, 4, 5},
		selectorTestMembers(),
		42, 0, 3, "session", 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected a legacy-shaped included set; got %d", len(got))
	}

	// The record must be untouched: PR2a's selector does not consume it.
	if _, ok := signing.RoastTransitionForSession("session", 1); !ok {
		t.Fatal("PR2a selector must not consume the transition record")
	}
}
