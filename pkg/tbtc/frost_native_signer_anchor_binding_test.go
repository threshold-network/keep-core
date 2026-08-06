package tbtc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type testAnchorBindingStore struct {
	history       *FrostNativeSignerStateWitnessAnchorHistory
	record        *FrostNativeSignerStateWitnessAnchorRecord
	proofEntries  map[uint64]frostsigning.NativeTBTCSignerStateWitnessProofEntry
	identity      FrostNativeSignerAnchorIdentity
	historyCalls  int
	readCalls     int
	casCalls      int
	lastExpected  FrostNativeSignerStateWitnessCheckpoint
	lastCandidate FrostNativeSignerStateWitnessCheckpoint
}

func (store *testAnchorBindingStore) ReadFrostNativeSignerStateWitnessAnchor(
	context.Context,
) (*FrostNativeSignerStateWitnessAnchorRecord, error) {
	store.readCalls++
	if store.record == nil {
		return nil, fmt.Errorf("anchor record is absent")
	}
	result := *store.record
	return &result, nil
}

func (store *testAnchorBindingStore) ReadFrostNativeSignerStateWitnessAnchorHistory(
	_ context.Context,
	floor FrostNativeSignerStateWitnessAnchorReference,
) (*FrostNativeSignerStateWitnessAnchorHistory, error) {
	store.historyCalls++
	if store.history == nil || store.history.Floor != floor {
		return nil, fmt.Errorf("unexpected anchor history floor")
	}
	return store.history, nil
}

func (store *testAnchorBindingStore) CompareAndSwapFrostNativeSignerStateWitnessAnchor(
	_ context.Context,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) (*FrostNativeSignerStateWitnessAnchorCASResult, error) {
	store.casCalls++
	store.lastExpected = expected
	store.lastCandidate = candidate
	if store.record == nil || store.record.Checkpoint != expected {
		return nil, fmt.Errorf("test anchor CAS expected checkpoint mismatch")
	}
	current := frostNativeSignerAnchorReferenceFromRecord(store.record)
	acknowledgement := testAnchorBindingAcknowledgement(
		store.identity,
		current,
		candidate,
		proof,
		time.Now().Add(20*time.Second),
	)
	store.record = frostNativeSignerAnchorRecord(&acknowledgement)
	return &FrostNativeSignerStateWitnessAnchorCASResult{
		Acknowledgement: acknowledgement,
	}, nil
}

type testAnchorBindingFixture struct {
	binding *frostNativeSignerAnchorBinding
	store   *testAnchorBindingStore
	tip     frostsigning.NativeTBTCSignerStateWitnessTip
	floor   FrostNativeSignerStateWitnessAnchorReference
	target  FrostNativeSignerStateWitnessAnchorReference
	now     time.Time

	recoverCalls     int
	acknowledgeCalls int
}

func newTestAnchorBindingFixture(
	t *testing.T,
	descendant bool,
) *testAnchorBindingFixture {
	t.Helper()
	now := time.Now().Truncate(time.Millisecond)
	storeFingerprint := [32]byte{0x11}
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                      [32]byte{0x12},
		ActivationManifestHash:          [32]byte{0x13},
		ActivationManifestSequence:      1,
		TrustDomainID:                   "test-native-anchor",
		EndpointLeafSPKIHash:            [32]byte{0x14},
		OnlineKeyHash:                   [32]byte{0x15},
		OperatorFingerprint:             [32]byte{0x16},
		HistoryStoreID:                  "test-anchor-history",
		HistoryStoreFingerprint:         [32]byte{0x17},
		HistoryClusterFingerprint:       [32]byte{0x18},
		OfflineAuthorityHash:            [32]byte{0x19},
		ClientSPKIHash:                  [32]byte{0x1a},
		SignerStoreFingerprint:          storeFingerprint,
		TransportBinding:                [32]byte{0x1b},
		WitnessMaximumRecords:           100,
		WitnessRotationThresholdRecords: 8,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)

	floorCheckpoint := testAnchorBindingCheckpoint(
		storeFingerprint,
		1,
		[32]byte{0x21},
		[32]byte{0x22},
	)
	floor := FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          7,
		Revision:              1,
		EventRoot:             [32]byte{0x23},
		AcknowledgementDigest: [32]byte{0x24},
		Checkpoint:            floorCheckpoint,
	}
	target := floor
	proofEntries := make(
		map[uint64]frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	)
	events := []FrostNativeSignerStateWitnessAnchorHistoryEvent{}
	var targetAcknowledgement *FrostNativeSignerCheckpointAcknowledgement
	if descendant {
		targetCheckpoint := testAnchorBindingCheckpoint(
			storeFingerprint,
			2,
			floorCheckpoint.StateCommitment,
			[32]byte{0x25},
		)
		proof := []frostsigning.NativeTBTCSignerStateWitnessProofEntry{{
			Generation:              targetCheckpoint.Generation,
			PreviousStateCommitment: targetCheckpoint.PreviousStateCommitment,
			StateImageDigest:        targetCheckpoint.StateImageDigest,
			StateCommitment:         targetCheckpoint.StateCommitment,
		}}
		proofEntries[2] = proof[0]
		acknowledgement := testAnchorBindingAcknowledgement(
			identity,
			floor,
			targetCheckpoint,
			proof,
			now.Add(-time.Minute),
		)
		acknowledgement.ExactReadRecovery = []byte(`{"fresh":"read"}`)
		acknowledgement.ReadRecoveryExpiresAt =
			uint64(now.Add(20 * time.Second).UnixMilli())
		targetAcknowledgement = &acknowledgement
		target = frostNativeSignerAnchorReferenceFromAcknowledgement(
			&acknowledgement,
		)
		events = append(events, FrostNativeSignerStateWitnessAnchorHistoryEvent{
			Acknowledgement: acknowledgement,
			WitnessProof:    proof,
		})
	}

	var record *FrostNativeSignerStateWitnessAnchorRecord
	if targetAcknowledgement != nil {
		record = frostNativeSignerAnchorRecord(targetAcknowledgement)
	} else {
		record = &FrostNativeSignerStateWitnessAnchorRecord{
			Checkpoint:             floor.Checkpoint,
			BindingHash:            ComputeFrostNativeSignerAnchorBindingHash(identity),
			AcknowledgementDigest:  floor.AcknowledgementDigest,
			OperationID:            [32]byte{0x26},
			TransitionDigest:       [32]byte{0x27},
			ServiceEpoch:           floor.ServiceEpoch,
			Revision:               floor.Revision,
			EventRoot:              floor.EventRoot,
			AcknowledgementJSON:    []byte(`{"floor":"ack"}`),
			AcknowledgementExpires: uint64(now.Add(-time.Minute).UnixMilli()),
			ReadRecoveryJSON:       []byte(`{"fresh":"floor-read"}`),
			ReadRecoveryExpires:    uint64(now.Add(20 * time.Second).UnixMilli()),
		}
	}
	store := &testAnchorBindingStore{
		record:       record,
		proofEntries: proofEntries,
		identity:     identity,
	}
	store.history = &FrostNativeSignerStateWitnessAnchorHistory{
		Floor:     floor,
		Target:    target,
		Events:    events,
		FinalRead: record,
	}
	fixture := &testAnchorBindingFixture{
		store:  store,
		floor:  floor,
		target: target,
		now:    now,
	}
	fixture.tip = testAnchorBindingTip(
		target.Checkpoint,
		floor.Checkpoint,
		ComputeFrostNativeSignerAnchorBindingHash(identity),
		target,
	)
	manifest := FrostNativeSignerAnchorManifest{
		Identity:                        identity,
		WitnessMaximumRecords:           identity.WitnessMaximumRecords,
		WitnessRotationThresholdRecords: identity.WitnessRotationThresholdRecords,
	}
	binding, err := newFrostNativeSignerAnchorBinding(
		store,
		manifest,
		floor,
		[32]byte{},
		func() (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
			result := fixture.tip
			return &result, nil
		},
		func(
			request *frostsigning.NativeTBTCSignerStateWitnessProofRequest,
		) (*frostsigning.NativeTBTCSignerStateWitnessProof, error) {
			return store.proof(request)
		},
		func(
			[]byte,
		) (*frostsigning.NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult, error) {
			fixture.acknowledgeCalls++
			return fixture.installRecordAsAcknowledgement(false), nil
		},
		func(
			[]byte,
		) (*frostsigning.NativeTBTCSignerStateWitnessCheckpointRecoveryResult, error) {
			fixture.recoverCalls++
			return fixture.installRecordAsRecovery(), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	binding.now = func() time.Time { return fixture.now }
	fixture.binding = binding
	return fixture
}

func (store *testAnchorBindingStore) proof(
	request *frostsigning.NativeTBTCSignerStateWitnessProofRequest,
) (*frostsigning.NativeTBTCSignerStateWitnessProof, error) {
	if request == nil {
		return nil, fmt.Errorf("nil proof request")
	}
	entries := make(
		[]frostsigning.NativeTBTCSignerStateWitnessProofEntry,
		0,
		request.TargetGeneration-request.AncestorGeneration,
	)
	previousCommitment := request.AncestorCommitment
	for generation := request.AncestorGeneration + 1; ; generation++ {
		entry, ok := store.proofEntries[generation]
		if !ok || entry.PreviousStateCommitment != previousCommitment {
			return nil, fmt.Errorf("missing test proof generation [%d]", generation)
		}
		entries = append(entries, entry)
		previousCommitment = entry.StateCommitment
		if generation == request.TargetGeneration {
			break
		}
	}
	return &frostsigning.NativeTBTCSignerStateWitnessProof{
		Schema:             frostsigning.NativeTBTCSignerStateWitnessProofSchema,
		StoreFingerprint:   request.StoreFingerprint,
		AncestorGeneration: request.AncestorGeneration,
		AncestorCommitment: request.AncestorCommitment,
		TargetGeneration:   request.TargetGeneration,
		TargetCommitment:   request.TargetCommitment,
		Complete:           true,
		Entries:            entries,
	}, nil
}

func (fixture *testAnchorBindingFixture) installRecordAsAcknowledgement(
	idempotent bool,
) *frostsigning.NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult {
	record := fixture.store.record
	fixture.tip.AnchorBindingHash = record.BindingHash
	fixture.tip.AnchorServiceEpoch = record.ServiceEpoch
	fixture.tip.AnchorRevision = record.Revision
	fixture.tip.AnchorEventRoot = record.EventRoot
	fixture.tip.AnchorAcknowledgementDigest = record.AcknowledgementDigest
	return &frostsigning.NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult{
		Schema:                      frostsigning.NativeTBTCSignerStateWitnessCheckpointAcknowledgementResultSchema,
		Acknowledged:                true,
		Idempotent:                  idempotent,
		StoreFingerprint:            fixture.tip.StoreFingerprint,
		Generation:                  fixture.tip.Generation,
		StateCommitment:             fixture.tip.StateCommitment,
		WitnessBaseGeneration:       fixture.tip.WitnessBaseGeneration,
		WitnessBaseCommitment:       fixture.tip.WitnessBaseCommitment,
		AnchorServiceEpoch:          record.ServiceEpoch,
		AnchorServiceRevision:       record.Revision,
		AnchorEventRoot:             record.EventRoot,
		AnchorAcknowledgementDigest: record.AcknowledgementDigest,
	}
}

func (fixture *testAnchorBindingFixture) installRecordAsRecovery() *frostsigning.NativeTBTCSignerStateWitnessCheckpointRecoveryResult {
	acknowledgement := fixture.installRecordAsAcknowledgement(true)
	return &frostsigning.NativeTBTCSignerStateWitnessCheckpointRecoveryResult{
		Schema:                      frostsigning.NativeTBTCSignerStateWitnessCheckpointRecoveryResultSchema,
		Recovered:                   true,
		Idempotent:                  acknowledgement.Idempotent,
		Rotated:                     acknowledgement.Rotated,
		StoreFingerprint:            acknowledgement.StoreFingerprint,
		Generation:                  acknowledgement.Generation,
		StateCommitment:             acknowledgement.StateCommitment,
		WitnessBaseGeneration:       acknowledgement.WitnessBaseGeneration,
		WitnessBaseCommitment:       acknowledgement.WitnessBaseCommitment,
		AnchorServiceEpoch:          acknowledgement.AnchorServiceEpoch,
		AnchorServiceRevision:       acknowledgement.AnchorServiceRevision,
		AnchorEventRoot:             acknowledgement.AnchorEventRoot,
		AnchorAcknowledgementDigest: acknowledgement.AnchorAcknowledgementDigest,
	}
}

func TestFrostNativeSignerAnchorBindingReconcilesAuthenticatedDescendant(
	t *testing.T,
) {
	fixture := newTestAnchorBindingFixture(t, true)
	result, err := fixture.binding.reconcileStartup(context.Background())
	if err != nil {
		t.Fatalf("authenticated descendant startup was rejected: %v", err)
	}
	if *result != fixture.tip || fixture.store.historyCalls != 1 ||
		fixture.recoverCalls != 0 || fixture.store.casCalls != 0 {
		t.Fatal("authenticated descendant did not reconcile exactly")
	}
}

func TestFrostNativeSignerAnchorBindingRecoversMissingOrPreviousAnchor(
	t *testing.T,
) {
	for _, descendant := range []bool{false, true} {
		name := "floor"
		if descendant {
			name = "descendant"
		}
		t.Run(name+" missing anchor", func(t *testing.T) {
			fixture := newTestAnchorBindingFixture(t, descendant)
			clearTestAnchorBindingTipAnchor(&fixture.tip)
			result, err := fixture.binding.reconcileStartup(context.Background())
			if err != nil {
				t.Fatalf("missing exact-checkpoint anchor was not recovered: %v", err)
			}
			if result.AnchorRevision != fixture.target.Revision ||
				fixture.recoverCalls != 1 {
				t.Fatal("missing anchor recovery did not install exact target")
			}
		})
	}

	t.Run("immediately preceding anchor", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		setTestAnchorBindingTipAnchor(
			&fixture.tip,
			ComputeFrostNativeSignerAnchorBindingHash(
				fixture.binding.identity,
			),
			fixture.floor,
		)
		result, err := fixture.binding.reconcileStartup(context.Background())
		if err != nil {
			t.Fatalf("immediately preceding anchor was not recovered: %v", err)
		}
		if result.AnchorRevision != fixture.target.Revision ||
			fixture.recoverCalls != 1 {
			t.Fatal("preceding anchor recovery did not install exact target")
		}
	})
}

func TestFrostNativeSignerAnchorBindingCatchesUpLocalAheadState(t *testing.T) {
	fixture := newTestAnchorBindingFixture(t, true)
	targetCheckpoint := fixture.target.Checkpoint
	localCheckpoint := testAnchorBindingCheckpoint(
		targetCheckpoint.StoreFingerprint,
		3,
		targetCheckpoint.StateCommitment,
		[32]byte{0x31},
	)
	fixture.store.proofEntries[3] =
		frostsigning.NativeTBTCSignerStateWitnessProofEntry{
			Generation:              3,
			PreviousStateCommitment: localCheckpoint.PreviousStateCommitment,
			StateImageDigest:        localCheckpoint.StateImageDigest,
			StateCommitment:         localCheckpoint.StateCommitment,
		}
	fixture.tip = testAnchorBindingTip(
		localCheckpoint,
		fixture.floor.Checkpoint,
		ComputeFrostNativeSignerAnchorBindingHash(fixture.binding.identity),
		fixture.target,
	)
	result, err := fixture.binding.reconcileStartup(context.Background())
	if err != nil {
		t.Fatalf("local-ahead crash state was not caught up: %v", err)
	}
	if result.Generation != 3 || fixture.store.casCalls != 1 ||
		fixture.acknowledgeCalls != 1 ||
		fixture.store.lastExpected != targetCheckpoint ||
		fixture.store.lastCandidate != localCheckpoint {
		t.Fatal("local-ahead startup did not CAS and install the exact checkpoint")
	}
}

func TestFrostNativeSignerAnchorBindingExactHeadRestartRecoversCrashWindowsInOnePass(
	t *testing.T,
) {
	t.Run("durable state before remote CAS", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		remoteCheckpoint := fixture.target.Checkpoint
		localCheckpoint := testAnchorBindingCheckpoint(
			remoteCheckpoint.StoreFingerprint,
			remoteCheckpoint.Generation+1,
			remoteCheckpoint.StateCommitment,
			[32]byte{0x91},
		)
		fixture.store.proofEntries[localCheckpoint.Generation] =
			frostsigning.NativeTBTCSignerStateWitnessProofEntry{
				Generation:              localCheckpoint.Generation,
				PreviousStateCommitment: localCheckpoint.PreviousStateCommitment,
				StateImageDigest:        localCheckpoint.StateImageDigest,
				StateCommitment:         localCheckpoint.StateCommitment,
			}
		fixture.tip = testAnchorBindingTip(
			localCheckpoint,
			fixture.floor.Checkpoint,
			fixture.binding.bindingHash,
			fixture.target,
		)

		result, err := fixture.binding.reconcileStartup(
			context.Background(),
		)
		if err != nil {
			t.Fatalf("single restart did not repair pre-CAS crash: %v", err)
		}
		if fixture.store.casCalls != 1 ||
			fixture.acknowledgeCalls != 1 ||
			!fixture.binding.localTipMatchesRemoteRecord(
				*result,
				fixture.store.record,
			) {
			t.Fatal("pre-CAS crash required more than one restart to converge")
		}
	})

	t.Run("remote CAS before Rust acknowledgement", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		setTestAnchorBindingTipAnchor(
			&fixture.tip,
			fixture.binding.bindingHash,
			fixture.floor,
		)

		result, err := fixture.binding.reconcileStartup(
			context.Background(),
		)
		if err != nil {
			t.Fatalf("single restart did not repair post-CAS crash: %v", err)
		}
		if fixture.store.casCalls != 0 ||
			fixture.recoverCalls != 1 ||
			!fixture.binding.localTipMatchesRemoteRecord(
				*result,
				fixture.store.record,
			) {
			t.Fatal("post-CAS crash required more than one restart to converge")
		}
	})
}

func TestFrostNativeSignerAnchorBindingEnforcesRestartableRevisionHeadroom(
	t *testing.T,
) {
	fixture := newTestAnchorBindingFixture(t, false)
	floorRevision := fixture.floor.Revision
	tests := []struct {
		distance         uint64
		expectedHeadroom uint64
		expectError      bool
	}{
		{distance: 4095, expectedHeadroom: 1},
		{distance: 4096, expectedHeadroom: 0},
		{distance: 4097, expectError: true},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.distance), func(t *testing.T) {
			headroom, err := fixture.binding.restartableRevisionHeadroom(
				fixture.floor.ServiceEpoch,
				floorRevision+test.distance,
			)
			if test.expectError {
				if err == nil {
					t.Fatal("out-of-window revision was accepted")
				}
				return
			}
			if err != nil || headroom != test.expectedHeadroom {
				t.Fatalf(
					"unexpected revision headroom [%d, %v]",
					headroom,
					err,
				)
			}
		})
	}

	fixture.store.record.Revision = floorRevision + 4097
	fixture.store.record.PreviousEventRoot = fixture.floor.EventRoot
	if err := fixture.binding.validateRemoteRecord(
		fixture.store.record,
	); err == nil {
		t.Fatal("startup remote target beyond the restartable bound was accepted")
	}
}

func TestFrostNativeSignerAnchorBindingEnforcesRestartableGenerationHeadroom(
	t *testing.T,
) {
	fixture := newTestAnchorBindingFixture(t, false)
	floorGeneration := fixture.floor.Checkpoint.Generation
	tests := []struct {
		distance         uint64
		expectedHeadroom uint64
		expectError      bool
	}{
		{distance: 4095, expectedHeadroom: 1},
		{distance: 4096, expectedHeadroom: 0},
		{distance: 4097, expectError: true},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.distance), func(t *testing.T) {
			headroom, err := fixture.binding.restartableGenerationHeadroom(
				floorGeneration + test.distance,
			)
			if test.expectError {
				if err == nil {
					t.Fatal("out-of-window generation was accepted")
				}
				return
			}
			if err != nil || headroom != test.expectedHeadroom {
				t.Fatalf(
					"unexpected generation headroom [%d, %v]",
					headroom,
					err,
				)
			}
		})
	}

	fixture.store.record.Checkpoint = testAnchorBindingCheckpoint(
		fixture.floor.Checkpoint.StoreFingerprint,
		floorGeneration+4097,
		fixture.floor.Checkpoint.StateCommitment,
		[32]byte{0xfd},
	)
	if err := fixture.binding.validateRemoteRecord(
		fixture.store.record,
	); err == nil {
		t.Fatal("remote target beyond the restartable generation bound was accepted")
	}
}

func TestFrostNativeSignerAnchorBindingRejectsRollbackForkAndPartialAnchor(
	t *testing.T,
) {
	t.Run("remote ahead", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		fixture.tip = testAnchorBindingTip(
			fixture.floor.Checkpoint,
			fixture.floor.Checkpoint,
			ComputeFrostNativeSignerAnchorBindingHash(fixture.binding.identity),
			fixture.floor,
		)
		if _, err := fixture.binding.reconcileStartup(
			context.Background(),
		); err == nil || !strings.Contains(err.Error(), "behind") {
			t.Fatalf("remote-ahead rollback was accepted: %v", err)
		}
	})

	t.Run("equal generation fork", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		fork := testAnchorBindingCheckpoint(
			fixture.target.Checkpoint.StoreFingerprint,
			fixture.target.Checkpoint.Generation,
			fixture.floor.Checkpoint.StateCommitment,
			[32]byte{0xee},
		)
		fixture.tip = testAnchorBindingTip(
			fork,
			fixture.floor.Checkpoint,
			ComputeFrostNativeSignerAnchorBindingHash(fixture.binding.identity),
			fixture.target,
		)
		if _, err := fixture.binding.reconcileStartup(
			context.Background(),
		); err == nil || !strings.Contains(err.Error(), "forks") {
			t.Fatalf("equal-generation fork was accepted: %v", err)
		}
	})

	t.Run("partial anchor", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		clearTestAnchorBindingTipAnchor(&fixture.tip)
		fixture.tip.AnchorBindingHash =
			ComputeFrostNativeSignerAnchorBindingHash(fixture.binding.identity)
		if _, err := fixture.binding.reconcileStartup(
			context.Background(),
		); err == nil || !strings.Contains(err.Error(), "partial") {
			t.Fatalf("partial local anchor was accepted: %v", err)
		}
	})

	t.Run("expired fresh Read", func(t *testing.T) {
		fixture := newTestAnchorBindingFixture(t, true)
		clearTestAnchorBindingTipAnchor(&fixture.tip)
		fixture.store.record.ReadRecoveryExpires =
			uint64(fixture.now.UnixMilli())
		if _, err := fixture.binding.reconcileStartup(
			context.Background(),
		); err == nil || !strings.Contains(err.Error(), "expired") {
			t.Fatalf("expired recovery wrapper was accepted: %v", err)
		}
	})
}

func TestFrostNativeSignerAnchorBindingRejectsCertifiedFloorHistoryGapsAndForks(
	t *testing.T,
) {
	tests := map[string]func(*testAnchorBindingFixture){
		"skipped event": func(fixture *testAnchorBindingFixture) {
			fixture.store.history.Events = nil
		},
		"gapped revision": func(fixture *testAnchorBindingFixture) {
			fixture.store.history.Events[0].Acknowledgement.Revision++
		},
		"forked parent": func(fixture *testAnchorBindingFixture) {
			fixture.store.history.Events[0].Acknowledgement.
				PreviousEventRoot[0] ^= 1
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newTestAnchorBindingFixture(t, true)
			mutate(fixture)
			if _, err := fixture.binding.reconcileStartup(
				context.Background(),
			); err == nil {
				t.Fatal("discontinuous certified-floor history was accepted")
			}
			if fixture.store.casCalls != 0 ||
				fixture.acknowledgeCalls != 0 ||
				fixture.recoverCalls != 0 {
				t.Fatal("invalid history triggered a signer or service mutation")
			}
		})
	}
}

func testAnchorBindingCheckpoint(
	storeFingerprint [32]byte,
	generation uint64,
	previous [32]byte,
	image [32]byte,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              generation,
		PreviousStateCommitment: previous,
		StateImageDigest:        image,
		StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			generation,
			previous,
			image,
		),
	}
}

func testAnchorBindingAcknowledgement(
	identity FrostNativeSignerAnchorIdentity,
	previous FrostNativeSignerStateWitnessAnchorReference,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	expires time.Time,
) FrostNativeSignerCheckpointAcknowledgement {
	operationID := [32]byte{byte(candidate.Generation + 0x40)}
	acknowledgement := FrostNativeSignerCheckpointAcknowledgement{
		BindingHash:       ComputeFrostNativeSignerAnchorBindingHash(identity),
		RequestDigest:     [32]byte{0x41},
		Nonce:             [32]byte{0x42},
		Status:            "applied",
		ServiceEpoch:      previous.ServiceEpoch,
		Revision:          previous.Revision + 1,
		PreviousEventRoot: previous.EventRoot,
		Checkpoint:        candidate,
		OperationID:       operationID,
		TransitionDigest: computeFrostNativeSignerAnchorTransitionDigest(
			identity,
			operationID,
			previous.Checkpoint,
			candidate,
			proof,
		),
		CommittedAtUnixMs: uint64(expires.Add(-20 * time.Second).UnixMilli()),
		ExpiresAtUnixMs:   uint64(expires.UnixMilli()),
		SigningDigest:     [32]byte{0x43},
		Signature:         [64]byte{0x44},
		ExactAcknowledgement: []byte(fmt.Sprintf(
			`{"testAcknowledgementRevision":"%d"}`,
			previous.Revision+1,
		)),
	}
	acknowledgement.EventRoot =
		computeFrostNativeSignerAnchorEventRoot(acknowledgement)
	acknowledgement.AcknowledgementDigest =
		computeFrostNativeSignerCheckpointAcknowledgementDigest(
			acknowledgement.SigningDigest,
			acknowledgement.Signature,
			identity.OnlineKeyHash,
		)
	return acknowledgement
}

func testAnchorBindingTip(
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
	base FrostNativeSignerStateWitnessCheckpoint,
	bindingHash [32]byte,
	reference FrostNativeSignerStateWitnessAnchorReference,
) frostsigning.NativeTBTCSignerStateWitnessTip {
	return frostsigning.NativeTBTCSignerStateWitnessTip{
		Schema:                      frostsigning.NativeTBTCSignerStateWitnessTipSchema,
		StoreFingerprint:            checkpoint.StoreFingerprint,
		Generation:                  checkpoint.Generation,
		PreviousStateCommitment:     checkpoint.PreviousStateCommitment,
		StateImageDigest:            checkpoint.StateImageDigest,
		StateCommitment:             checkpoint.StateCommitment,
		WitnessBaseGeneration:       base.Generation,
		WitnessBaseCommitment:       base.StateCommitment,
		AnchorBindingHash:           bindingHash,
		AnchorServiceEpoch:          reference.ServiceEpoch,
		AnchorRevision:              reference.Revision,
		AnchorEventRoot:             reference.EventRoot,
		AnchorAcknowledgementDigest: reference.AcknowledgementDigest,
	}
}

func clearTestAnchorBindingTipAnchor(
	tip *frostsigning.NativeTBTCSignerStateWitnessTip,
) {
	tip.AnchorBindingHash = [32]byte{}
	tip.AnchorServiceEpoch = 0
	tip.AnchorRevision = 0
	tip.AnchorEventRoot = [32]byte{}
	tip.AnchorAcknowledgementDigest = [32]byte{}
}

func setTestAnchorBindingTipAnchor(
	tip *frostsigning.NativeTBTCSignerStateWitnessTip,
	bindingHash [32]byte,
	reference FrostNativeSignerStateWitnessAnchorReference,
) {
	tip.AnchorBindingHash = bindingHash
	tip.AnchorServiceEpoch = reference.ServiceEpoch
	tip.AnchorRevision = reference.Revision
	tip.AnchorEventRoot = reference.EventRoot
	tip.AnchorAcknowledgementDigest = reference.AcknowledgementDigest
}

// Headroom must refresh when the anchor acknowledges a tip, not only when a
// full readiness reconciliation succeeds. Without this a node signing steadily
// and a node that has stalled are indistinguishable at the scrape, and pre-sign
// authorization caches readiness per finality window, so an entire authorized
// batch can burn window invisibly inside one window.
func TestFrostNativeSignerAnchorBindingPublishesHeadroomOnAcknowledgement(
	t *testing.T,
) {
	fixture := newTestAnchorBindingFixture(t, false)

	// The mirror is process-global and its reset helper is unexported, so this
	// test establishes its own baseline rather than clearing it. Every
	// assertion below is relative to this publication.
	//
	// It also supplies the seat count the commit path reuses: that path has
	// authenticated headroom but no inventory of its own.
	frostsigning.RecordNativeTBTCSignerStateAnchorRestartableHeadroom(
		4096,
		4096,
		false,
		4,
	)

	tip := fixture.tip
	tip.AnchorServiceEpoch = fixture.floor.ServiceEpoch
	tip.AnchorRevision = fixture.floor.Revision + 96
	tip.Generation = fixture.floor.Checkpoint.Generation + 96
	fixture.binding.publishRestartableHeadroomLocked(&tip)

	revisions, generations, observed :=
		frostsigning.NativeTBTCSignerStateAnchorRestartableHeadroom()
	if !observed ||
		revisions != FrostNativeSignerAnchorMaximumHistoryEvents-96 ||
		generations != FrostNativeSignerAnchorMaximumHistoryProofEntries-96 {
		t.Fatalf(
			"acknowledged tip did not refresh the headroom mirror: (%d, %d, %v)",
			revisions,
			generations,
			observed,
		)
	}
	if seats, _ :=
		frostsigning.NativeTBTCSignerStateAnchorLargestLocalSeatCount(); seats != 4 {
		t.Fatalf("seat count was not carried forward: got %d want 4", seats)
	}

	// A tip outside the certified floor leaves the previous reading standing
	// rather than publishing a partial or misleading pair.
	before, _, _ := frostsigning.NativeTBTCSignerStateAnchorRestartableHeadroom()
	outside := tip
	outside.AnchorServiceEpoch = fixture.floor.ServiceEpoch + 1
	fixture.binding.publishRestartableHeadroomLocked(&outside)
	if after, _, _ :=
		frostsigning.NativeTBTCSignerStateAnchorRestartableHeadroom(); after != before {
		t.Fatalf(
			"unauthenticated tip overwrote the mirror: got %d want %d",
			after,
			before,
		)
	}
}
