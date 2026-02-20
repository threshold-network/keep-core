package tbtc

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestAttemptIncludedMembersIndexes(t *testing.T) {
	included := attemptIncludedMembersIndexes(
		6,
		[]group.MemberIndex{6, 2, 4, 2},
	)

	expected := []group.MemberIndex{1, 3, 5}
	if !reflect.DeepEqual(expected, included) {
		t.Fatalf("unexpected included members\nexpected: [%v]\nactual:   [%v]", expected, included)
	}
}

func TestSigningAttemptSeed(t *testing.T) {
	first := signingAttemptSeed(big.NewInt(100))
	again := signingAttemptSeed(big.NewInt(100))
	if first != again {
		t.Fatalf("seed should be stable\nfirst: [%v]\nagain: [%v]", first, again)
	}

	second := signingAttemptSeed(big.NewInt(101))
	if first == second {
		t.Fatal("different messages should produce different attempt seeds")
	}
}
