//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestReserveAndAnnounceFrostDKGReadiness_AdmissionFailureDoesNotAnnounce(
	t *testing.T,
) {
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return frostNativeSignerAnchorCapacity{
				Revisions: FrostNativeSignerAnchorRotationWarningHeadroom + 1,
				Generations: FrostNativeSignerAnchorRotationWarningHeadroom +
					1,
			}, nil
		},
		reserved: frostNativeSignerAnchorCapacity{
			Revisions:   FrostNativeSignerAnchorRotationWarningHeadroom,
			Generations: FrostNativeSignerAnchorRotationWarningHeadroom,
		},
	}
	announced := false

	admission, err := reserveAndAnnounceFrostDKGReadiness(
		context.Background(),
		controller,
		[]group.MemberIndex{1, 2},
		func() (
			[]group.MemberIndex,
			registry.MisbehavedMemberIndices,
			error,
		) {
			announced = true
			return nil, nil, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unreserved") {
		t.Fatalf("unexpected DKG admission result: [%v]", err)
	}
	if admission != nil {
		t.Fatal("failed DKG admission returned a reservation")
	}
	if announced {
		t.Fatal("DKG readiness was announced after admission failed")
	}
}

func TestReserveAndAnnounceFrostDKGReadiness_ReservesEverySelectedLocalSeat(
	t *testing.T,
) {
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return frostNativeSignerAnchorCapacity{
				Revisions: FrostNativeSignerAnchorRotationWarningHeadroom + 10,
				Generations: FrostNativeSignerAnchorRotationWarningHeadroom +
					10,
			}, nil
		},
	}
	localMemberIndexes := []group.MemberIndex{2, 4, 6}

	admission, err :=
		reserveAndAnnounceFrostDKGReadiness(
			context.Background(),
			controller,
			localMemberIndexes,
			func() (
				[]group.MemberIndex,
				registry.MisbehavedMemberIndices,
				error,
			) {
				return []group.MemberIndex{1, 2}, nil, nil
			},
		)
	if err != nil {
		t.Fatal(err)
	}
	if admission == nil || admission.anchorReservation == nil {
		t.Fatal("successful DKG admission returned no reservation")
	}
	if len(admission.activeMemberIndexes) != 2 {
		t.Fatalf(
			"unexpected active member indexes: [%v]",
			admission.activeMemberIndexes,
		)
	}
	expectedPersistenceCalls := uint64(len(localMemberIndexes) + 1)
	if controller.reserved.Revisions != expectedPersistenceCalls ||
		controller.reserved.Generations != expectedPersistenceCalls {
		t.Fatalf(
			"DKG did not reserve every selected local seat and retirement: [%+v]",
			controller.reserved,
		)
	}

	admission.anchorReservation.Release()
	if controller.reserved != (frostNativeSignerAnchorCapacity{}) {
		t.Fatalf(
			"released DKG reservation remained charged: [%+v]",
			controller.reserved,
		)
	}
}

func TestExecuteFrostDKGIfPossible_RequiresRoastRetryReadiness(t *testing.T) {
	t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "true")
	t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "")
	registerFrostDKGReadinessTestEngine(t)

	executed := executeFrostDKGIfPossible(
		context.Background(),
		nil,
		nil,
		&FrostDKGStartedEvent{Seed: big.NewInt(100)},
		[]group.MemberIndex{1},
		nil,
	)

	if executed {
		t.Fatal("DKG must not execute without ROAST retry readiness")
	}
}

type currentFrostDKGResultTestChain struct {
	FrostDKGChain
	state  DKGState
	events []*FrostDKGResultSubmittedEvent
	valid  bool
}

func (chain *currentFrostDKGResultTestChain) GetFrostDKGState() (
	DKGState,
	error,
) {
	return chain.state, nil
}

func (chain *currentFrostDKGResultTestChain) PastFrostDKGResultSubmittedEvents(
	*FrostDKGResultSubmittedEventFilter,
) ([]*FrostDKGResultSubmittedEvent, error) {
	return chain.events, nil
}

func (chain *currentFrostDKGResultTestChain) IsFrostDKGResultValid(
	*registry.Result,
) (bool, string, error) {
	return chain.valid, "", nil
}

func TestCurrentFrostDKGResultMatchesExactPendingWallet(t *testing.T) {
	expected := &registry.Result{
		XOnlyOutputKey:           [32]byte{1},
		MembersHash:              [32]byte{2},
		Members:                  registry.FullMembers{10, 20, 30},
		MisbehavedMembersIndices: registry.MisbehavedMemberIndices{2},
		Signatures:               []byte{1},
	}
	peerSubmission := *expected
	peerSubmission.SubmitterMemberIndex = 3
	peerSubmission.Signatures = []byte{2, 3}
	peerSubmission.SigningMembersIndices = []uint64{1, 3}
	chain := &currentFrostDKGResultTestChain{
		state: Challenge,
		events: []*FrostDKGResultSubmittedEvent{{
			BlockNumber: 10,
			Result:      &peerSubmission,
		}},
		valid: true,
	}

	matches, err := currentFrostDKGResultMatches(chain, expected)
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("the same valid pending wallet result was not recognized")
	}

	other := *expected
	other.XOnlyOutputKey[0]++
	matches, err = currentFrostDKGResultMatches(chain, &other)
	if err != nil {
		t.Fatal(err)
	}
	if matches {
		t.Fatal("a different pending wallet result was accepted")
	}
}

type frostDKGReadinessTestEngine struct{}

func (*frostDKGReadinessTestEngine) RetireDistributedDKGKeyPackages(
	string,
) error {
	return nil
}

func (*frostDKGReadinessTestEngine) BuildTaprootTx(
	string,
	[]frostsigning.NativeTBTCSignerTxInput,
	[]frostsigning.NativeTBTCSignerTxOutput,
	*string,
) (*frostsigning.NativeTBTCSignerTxResult, error) {
	return nil, nil
}

func (*frostDKGReadinessTestEngine) VerifySignatureShare(
	string,
	[]byte,
	[]byte,
	uint16,
	*[32]byte,
) (frostsigning.NativeShareVerificationVerdict, error) {
	return frostsigning.NativeShareVerdictValid, nil
}

func registerFrostDKGReadinessTestEngine(t *testing.T) {
	t.Helper()

	previous := frostsigning.CurrentNativeTBTCSignerEngine()
	frostsigning.UnregisterNativeTBTCSignerEngine()
	if err := frostsigning.RegisterNativeTBTCSignerEngine(
		&frostDKGReadinessTestEngine{},
	); err != nil {
		t.Fatalf("failed to register native engine: [%v]", err)
	}

	t.Cleanup(func() {
		frostsigning.UnregisterNativeTBTCSignerEngine()
		if previous != nil {
			if err := frostsigning.RegisterNativeTBTCSignerEngine(previous); err != nil {
				t.Errorf("failed to restore native engine: [%v]", err)
			}
		}
	})
}

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
