//go:build frost_native

package tbtc

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestLowestLocalActiveMemberIndex(t *testing.T) {
	testCases := map[string]struct {
		local    []group.MemberIndex
		active   []group.MemberIndex
		expected group.MemberIndex
	}{
		"lowest local slot active": {
			local:    []group.MemberIndex{2, 4, 6},
			active:   []group.MemberIndex{1, 2, 3, 4},
			expected: 2,
		},
		"lowest local slot dropped out": {
			local:    []group.MemberIndex{2, 4, 6},
			active:   []group.MemberIndex{1, 3, 4, 6},
			expected: 4,
		},
		"no local slot active": {
			local:    []group.MemberIndex{2, 4},
			active:   []group.MemberIndex{1, 3, 5},
			expected: 0,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := lowestLocalActiveMemberIndex(test.local, test.active)
			if actual != test.expected {
				t.Fatalf(
					"unexpected lowest local active member index\nexpected: [%d]\nactual:   [%d]",
					test.expected,
					actual,
				)
			}
		})
	}
}

func TestLocalActiveFrostMemberIndexes(t *testing.T) {
	actual := localActiveFrostMemberIndexes(
		[]group.MemberIndex{5, 2, 4, 9},
		[]group.MemberIndex{1, 2, 3, 4, 5},
	)

	expected := []group.MemberIndex{5, 2, 4}
	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected local active member indexes length\nexpected: [%d]\nactual:   [%d]",
			len(expected),
			len(actual),
		)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Fatalf(
				"unexpected local active member index at [%d]\nexpected: [%d]\nactual:   [%d]",
				i,
				expected[i],
				actual[i],
			)
		}
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

func TestOutputKeyFromTBTCSignerDKGResult_AcceptsCompressedKeyGroup(
	t *testing.T,
) {
	const compressedKey = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	const xOnlyKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"

	outputKey, err := outputKeyFromTBTCSignerDKGResult(
		&frostsigning.NativeTBTCSignerDKGResult{
			KeyGroup: compressedKey,
		},
	)
	if err != nil {
		t.Fatalf("output key: %v", err)
	}

	want, _ := hex.DecodeString(xOnlyKey)
	if !bytes.Equal(outputKey[:], want) {
		t.Fatalf(
			"unexpected output key\nexpected: [%x]\nactual:   [%x]",
			want,
			outputKey[:],
		)
	}
}

func TestFinalFrostDKGMemberIndexes_NormalizesToFinalSigningGroupIndexes(
	t *testing.T,
) {
	activeMemberIndexes := []group.MemberIndex{5, 2, 4}

	actual, err := finalFrostDKGMemberIndexes(
		activeMemberIndexes,
		&GroupSelectionResult{
			OperatorsAddresses: chain.Addresses{
				"0xAA",
				"0xBB",
				"0xCC",
				"0xDD",
				"0xEE",
			},
		},
		&GroupParameters{
			GroupSize:       5,
			GroupQuorum:     3,
			HonestThreshold: 2,
		},
	)
	if err != nil {
		t.Fatalf("unexpected final member index error: [%v]", err)
	}

	expected := []group.MemberIndex{1, 2, 3}
	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected final member indexes count\nexpected: [%d]\nactual:   [%d]",
			len(expected),
			len(actual),
		)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Fatalf(
				"unexpected final member index at [%d]\nexpected: [%d]\nactual:   [%d]",
				i,
				expected[i],
				actual[i],
			)
		}
	}

	expectedActive := []group.MemberIndex{5, 2, 4}
	for i := range expectedActive {
		if activeMemberIndexes[i] != expectedActive[i] {
			t.Fatalf(
				"active member indexes should not be mutated\nexpected: [%v]\nactual:   [%v]",
				expectedActive,
				activeMemberIndexes,
			)
		}
	}
}
