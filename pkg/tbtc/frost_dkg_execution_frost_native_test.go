//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

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

func TestExecuteFrostDKG_PrefersTBTCSignerMaterial(t *testing.T) {
	tbtcSignerEngine := &testNativeTBTCSignerSeededDKGEngine{}
	uniffiEngine := &testNativeFROSTDKGEngine{}

	result, err := executeFrostDKG(
		context.Background(),
		nil,
		uniffiEngine,
		tbtcSignerEngine,
		&FrostDKGStartedEvent{Seed: big.NewInt(0x1234)},
		1,
		[]group.MemberIndex{1, 2, 3},
		&GroupSelectionResult{},
		2,
		"test-session",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected DKG error: [%v]", err)
	}

	if !tbtcSignerEngine.runDKGWithSeedCalled {
		t.Fatal("expected tbtc-signer DKG engine to be used")
	}
	if uniffiEngine.called {
		t.Fatal("did not expect UniFFI native FROST DKG engine to be used")
	}
	if result.signerMaterial == nil {
		t.Fatal("expected signer material")
	}
	if result.signerMaterial.Format !=
		frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			result.signerMaterial.Format,
		)
	}
}

type testNativeTBTCSignerSeededDKGEngine struct {
	runDKGWithSeedCalled bool
}

func (tntsde *testNativeTBTCSignerSeededDKGEngine) RunDKG(
	string,
	[]frostsigning.NativeTBTCSignerDKGParticipant,
	uint16,
) (*frostsigning.NativeTBTCSignerDKGResult, error) {
	return nil, fmt.Errorf("unseeded RunDKG should not be used")
}

func (tntsde *testNativeTBTCSignerSeededDKGEngine) RunDKGWithSeed(
	sessionID string,
	participants []frostsigning.NativeTBTCSignerDKGParticipant,
	threshold uint16,
	dkgSeedHex string,
) (*frostsigning.NativeTBTCSignerDKGResult, error) {
	tntsde.runDKGWithSeedCalled = true

	if sessionID != "test-session" {
		return nil, fmt.Errorf("unexpected session ID: [%s]", sessionID)
	}
	if len(participants) != 3 {
		return nil, fmt.Errorf("unexpected participant count: [%d]", len(participants))
	}
	if threshold != 2 {
		return nil, fmt.Errorf("unexpected threshold: [%d]", threshold)
	}
	if dkgSeedHex == "" {
		return nil, fmt.Errorf("expected DKG seed")
	}

	return &frostsigning.NativeTBTCSignerDKGResult{
		SessionID:        sessionID,
		KeyGroup:         "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
		ParticipantCount: uint16(len(participants)),
		Threshold:        threshold,
		CreatedAtUnix:    1,
	}, nil
}

func (tntsde *testNativeTBTCSignerSeededDKGEngine) StartSignRound(
	string,
	uint16,
	[]byte,
	string,
	[]uint16,
	*[32]byte,
) (*frostsigning.NativeTBTCSignerRoundState, error) {
	return nil, fmt.Errorf("StartSignRound should not be used")
}

func (tntsde *testNativeTBTCSignerSeededDKGEngine) FinalizeSignRound(
	string,
	[]frostsigning.NativeTBTCSignerRoundContribution,
	*[32]byte,
) ([]byte, error) {
	return nil, fmt.Errorf("FinalizeSignRound should not be used")
}

func (tntsde *testNativeTBTCSignerSeededDKGEngine) BuildTaprootTx(
	string,
	[]frostsigning.NativeTBTCSignerTxInput,
	[]frostsigning.NativeTBTCSignerTxOutput,
	*string,
) (*frostsigning.NativeTBTCSignerTxResult, error) {
	return nil, fmt.Errorf("BuildTaprootTx should not be used")
}

type testNativeFROSTDKGEngine struct {
	called bool
}

func (tnfdkg *testNativeFROSTDKGEngine) Part1(
	string,
	uint16,
	uint16,
) (*frostsigning.NativeFROSTDKGPart1Result, error) {
	tnfdkg.called = true
	return nil, fmt.Errorf("UniFFI DKG Part1 should not be used")
}

func (tnfdkg *testNativeFROSTDKGEngine) Part2(
	*frostsigning.NativeFROSTDKGRound1SecretPackage,
	[]*frostsigning.NativeFROSTDKGRound1Package,
) (*frostsigning.NativeFROSTDKGPart2Result, error) {
	tnfdkg.called = true
	return nil, fmt.Errorf("UniFFI DKG Part2 should not be used")
}

func (tnfdkg *testNativeFROSTDKGEngine) Part3(
	*frostsigning.NativeFROSTDKGRound2SecretPackage,
	[]*frostsigning.NativeFROSTDKGRound1Package,
	[]*frostsigning.NativeFROSTDKGRound2Package,
) (*frostsigning.NativeFROSTDKGResult, error) {
	tnfdkg.called = true
	return nil, fmt.Errorf("UniFFI DKG Part3 should not be used")
}
