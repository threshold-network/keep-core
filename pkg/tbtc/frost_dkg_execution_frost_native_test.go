//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestReserveFrostDKGReadiness_AdmissionFailureIsSynchronous(
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
	reservation, err := reserveFrostDKGReadiness(
		context.Background(),
		controller,
		[]group.MemberIndex{1, 2},
	)
	if err == nil || !strings.Contains(err.Error(), "unreserved") {
		t.Fatalf("unexpected DKG admission result: [%v]", err)
	}
	if reservation != nil {
		t.Fatal("failed DKG admission returned a reservation")
	}
}

func TestReserveFrostDKGReadiness_ReservesEverySelectedLocalSeat(
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

	reservation, err :=
		reserveFrostDKGReadiness(
			context.Background(),
			controller,
			localMemberIndexes,
		)
	if err != nil {
		t.Fatal(err)
	}
	if reservation == nil {
		t.Fatal("successful DKG admission returned no reservation")
	}
	expectedPersistenceCalls := uint64(len(localMemberIndexes) * 2)
	if controller.reserved.Revisions != expectedPersistenceCalls ||
		controller.reserved.Generations != expectedPersistenceCalls {
		t.Fatalf(
			"DKG did not reserve persistence and retirement for every selected local seat: [%+v]",
			controller.reserved,
		)
	}

	reservation.Release()
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

// frostDKGPartialPersistTestEngine drives a complete distributed DKG for two
// co-located local seats with opaque round payloads - the smallest engine that
// can reach the persist stage, which is the only place a DURABLE key package can
// exist. Persist then fails for one seat, reproducing exactly the partial-persist
// failure whose local key groups must survive.
type frostDKGPartialPersistTestEngine struct {
	frostDKGReadinessTestEngine
	keyGroup           string
	failPersistForSeat uint16

	mutex     sync.Mutex
	persisted []uint16
	retired   []string
}

func (engine *frostDKGPartialPersistTestEngine) Part1(
	participantIdentifier string,
	_ uint16,
	_ uint16,
) (*frostsigning.NativeFROSTDKGPart1Result, error) {
	return &frostsigning.NativeFROSTDKGPart1Result{
		SecretPackage: &frostsigning.NativeFROSTDKGRound1SecretPackage{
			Data: []byte("round1-secret-" + participantIdentifier),
		},
		Package: &frostsigning.NativeFROSTDKGRound1Package{
			Identifier: participantIdentifier,
			Data:       []byte("round1-" + participantIdentifier),
		},
	}, nil
}

func (engine *frostDKGPartialPersistTestEngine) Part2(
	_ *frostsigning.NativeFROSTDKGRound1SecretPackage,
	round1Packages []*frostsigning.NativeFROSTDKGRound1Package,
) (*frostsigning.NativeFROSTDKGPart2Result, error) {
	// One round-2 package per OTHER member, addressed by that member's
	// identifier - the shape the runner routes and seals by.
	packages := make(
		[]*frostsigning.NativeFROSTDKGRound2Package,
		0,
		len(round1Packages),
	)
	for _, round1Package := range round1Packages {
		packages = append(
			packages,
			&frostsigning.NativeFROSTDKGRound2Package{
				Identifier: round1Package.Identifier,
				Data:       []byte("round2-for-" + round1Package.Identifier),
			},
		)
	}
	return &frostsigning.NativeFROSTDKGPart2Result{
		SecretPackage: &frostsigning.NativeFROSTDKGRound2SecretPackage{
			Data: []byte("round2-secret"),
		},
		Packages: packages,
	}, nil
}

func (engine *frostDKGPartialPersistTestEngine) Part3(
	_ *frostsigning.NativeFROSTDKGRound2SecretPackage,
	_ []*frostsigning.NativeFROSTDKGRound1Package,
	_ []*frostsigning.NativeFROSTDKGRound2Package,
) (*frostsigning.NativeFROSTDKGResult, error) {
	return &frostsigning.NativeFROSTDKGResult{
		KeyPackage: &frostsigning.NativeFROSTKeyPackage{
			Data: []byte("key-package"),
		},
		PublicKeyPackage: &frostsigning.NativeFROSTPublicKeyPackage{
			VerifyingKey: engine.keyGroup,
		},
	}, nil
}

func (engine *frostDKGPartialPersistTestEngine) PersistDistributedDKGKeyPackage(
	sessionID string,
	participantIdentifier uint16,
	threshold uint16,
	participantCount uint16,
	_ *frostsigning.NativeFROSTKeyPackage,
	_ *frostsigning.NativeFROSTPublicKeyPackage,
) (*frostsigning.NativeTBTCSignerDKGResult, error) {
	if participantIdentifier == engine.failPersistForSeat {
		return nil, errors.New("injected key-package persistence failure")
	}
	engine.mutex.Lock()
	engine.persisted = append(engine.persisted, participantIdentifier)
	engine.mutex.Unlock()
	return &frostsigning.NativeTBTCSignerDKGResult{
		SessionID:        sessionID,
		KeyGroup:         engine.keyGroup,
		ParticipantCount: participantCount,
		Threshold:        threshold,
	}, nil
}

func (engine *frostDKGPartialPersistTestEngine) RetireDistributedDKGKeyPackages(
	keyGroup string,
) error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	engine.retired = append(engine.retired, keyGroup)
	return nil
}

func (engine *frostDKGPartialPersistTestEngine) outcome() ([]uint16, []string) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return append([]uint16{}, engine.persisted...),
		append([]string{}, engine.retired...)
}

// TestExecuteDistributedFrostDKG_PreservesPartiallyPersistedKeyGroups pins the
// invariant that a LOCAL distributed-DKG failure must never destroy durable
// secret shares. A seat only reaches the persist stage after its runner finished
// part 3, so it already broadcast every round package; the other members can
// finish the DKG and register a wallet that still lists this operator. Retiring
// the persisted key group here would leave that live wallet permanently short of
// this node's share, so the node preserves it and leaves retirement to
// reconciliation rooted in finalized retained history.
func TestExecuteDistributedFrostDKG_PreservesPartiallyPersistedKeyGroups(
	t *testing.T,
) {
	const keyGroup = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"

	localChain := Connect()
	_, operatorPublicKey, err := localChain.OperatorKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	operatorAddress := localChain.Signing().PublicKeyBytesToAddress(
		operator.MarshalUncompressed(operatorPublicKey),
	)

	sessionID := fmt.Sprintf(
		"frost-dkg-partial-persist-%d",
		time.Now().UnixNano(),
	)
	channel, err := netlocal.ConnectWithKey(operatorPublicKey).
		BroadcastChannelFor(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	engine := &frostDKGPartialPersistTestEngine{
		keyGroup: keyGroup,
		// Seat 2 fails to persist AFTER seat 1 has durably persisted the very
		// same key group.
		failPersistForSeat: 2,
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCtx()

	executionResult, err := executeDistributedFrostDKG(
		ctx,
		engine,
		&node{
			chain: localChain,
			frostGroupParameters: &GroupParameters{
				GroupSize:       3,
				GroupQuorum:     2,
				HonestThreshold: 2,
			},
		},
		channel,
		[]group.MemberIndex{1, 2},
		[]group.MemberIndex{1, 2},
		[]group.MemberIndex{1, 2},
		&GroupSelectionResult{
			OperatorsAddresses: chain.Addresses{
				operatorAddress,
				operatorAddress,
				operatorAddress,
			},
		},
		2,
		sessionID,
		nil,
	)
	if err == nil || executionResult != nil {
		t.Fatalf(
			"partially persisted FROST DKG reported success: [%+v]",
			executionResult,
		)
	}
	if !strings.Contains(err.Error(), "cannot persist the key package") {
		t.Fatalf("unexpected FROST DKG execution failure: [%v]", err)
	}

	persisted, retired := engine.outcome()
	if len(persisted) != 1 || persisted[0] != 1 {
		t.Fatalf(
			"the run did not reach a partial-persist state: persisted=[%v]",
			persisted,
		)
	}
	if len(retired) != 0 {
		t.Fatalf(
			"a local DKG failure destroyed durably persisted key groups: [%v]",
			retired,
		)
	}
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
