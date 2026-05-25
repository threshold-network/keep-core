//go:build frost_native

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/registry"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestLowestMemberIndex(t *testing.T) {
	actual := lowestMemberIndex([]group.MemberIndex{9, 3, 7})
	if actual != 3 {
		t.Fatalf("unexpected lowest member index\nexpected: [3]\nactual:   [%d]", actual)
	}
}

func TestFrostMisbehavedMemberIndices(t *testing.T) {
	actual := frostMisbehavedMemberIndices(
		7,
		[]group.MemberIndex{1, 3, 4, 7},
	)

	expected := registry.MisbehavedMemberIndices{2, 5, 6}
	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected misbehaved member indices length\nexpected: [%d]\nactual:   [%d]",
			len(expected),
			len(actual),
		)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Fatalf(
				"unexpected misbehaved member index at [%d]\nexpected: [%d]\nactual:   [%d]",
				i,
				expected[i],
				actual[i],
			)
		}
	}
}
