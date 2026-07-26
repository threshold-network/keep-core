package signing

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testNativeTBTCSignerStateAnchorCommitter struct {
	mutex       sync.Mutex
	err         error
	verifyErr   error
	calls       int
	verifyCalls int
	operation   string
	expected    NativeTBTCSignerStateWitnessTip
	candidate   NativeTBTCSignerStateWitnessTip
	acknowledge func(NativeTBTCSignerStateWitnessTip) NativeTBTCSignerStateWitnessTip
	current     *NativeTBTCSignerStateWitnessTip
	commitStart chan struct{}
	allowCommit chan struct{}
	startOnce   sync.Once
}

func testNativeTBTCSignerStateAnchorTrustHead() *NativeTBTCSignerStateAnchorTrustHead {
	floor := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	return &NativeTBTCSignerStateAnchorTrustHead{
		Schema:                      NativeTBTCSignerStateAnchorTrustHeadSchema,
		CertificateSequence:         1,
		CertificateDigest:           [32]byte{0x21},
		ActivationManifestSequence:  1,
		ActivationManifestHash:      [32]byte{0x22},
		BindingHash:                 [32]byte{10},
		ResponsePublicKeySPKISHA256: [32]byte{0x23},
		OfflineAuthoritySPKISHA256:  [32]byte{0x24},
		ServiceEpoch:                1,
		CertifiedFloor: NativeTBTCSignerStateAnchorTrustReference{
			ServiceEpoch:          1,
			Revision:              1,
			EventRoot:             [32]byte{0x25},
			AcknowledgementDigest: [32]byte{0x26},
			Checkpoint: NativeTBTCSignerStateAnchorCheckpoint{
				StoreFingerprint:        floor.StoreFingerprint,
				Generation:              floor.Generation,
				PreviousStateCommitment: floor.PreviousStateCommitment,
				StateImageDigest:        floor.StateImageDigest,
				StateCommitment:         floor.StateCommitment,
			},
		},
		WitnessMaximumRecords:           4096,
		WitnessRotationThresholdRecords: 1024,
	}
}

func readTestNativeTBTCSignerStateAnchorTrustHead() (
	*NativeTBTCSignerStateAnchorTrustHead,
	error,
) {
	return testNativeTBTCSignerStateAnchorTrustHead(), nil
}

func (committer *testNativeTBTCSignerStateAnchorCommitter) VerifyNativeTBTCSignerStateTip(
	ctx context.Context,
	local NativeTBTCSignerStateWitnessTip,
) error {
	committer.mutex.Lock()
	defer committer.mutex.Unlock()
	committer.verifyCalls++
	return committer.verifyErr
}

func (committer *testNativeTBTCSignerStateAnchorCommitter) CommitNativeTBTCSignerStateTransition(
	ctx context.Context,
	operation string,
	expected NativeTBTCSignerStateWitnessTip,
	candidate NativeTBTCSignerStateWitnessTip,
) (*NativeTBTCSignerStateWitnessTip, error) {
	if committer.commitStart != nil {
		committer.startOnce.Do(func() { close(committer.commitStart) })
	}
	if committer.allowCommit != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-committer.allowCommit:
		}
	}
	committer.mutex.Lock()
	defer committer.mutex.Unlock()
	committer.calls++
	committer.operation = operation
	committer.expected = expected
	committer.candidate = candidate
	if committer.err != nil {
		return nil, committer.err
	}
	acknowledged := committer.acknowledge(candidate)
	*committer.current = acknowledged
	return &acknowledged, nil
}

func TestNativeTBTCSignerOutputBarrierDoesNotReleaseBeforeAcknowledgement(
	t *testing.T,
) {
	for _, nativeErr := range []bool{false, true} {
		name := "native-success"
		if nativeErr {
			name = "native-error"
		}
		t.Run(name, func(t *testing.T) {
			resetNativeTBTCSignerStateAnchorBarrierForTest()
			t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

			initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
			candidate := testNativeTBTCSignerStateWitnessTip(
				2,
				initial.StateCommitment,
			)
			current := initial
			commitStart := make(chan struct{})
			allowCommit := make(chan struct{})
			committer := &testNativeTBTCSignerStateAnchorCommitter{
				current:     &current,
				commitStart: commitStart,
				allowCommit: allowCommit,
				acknowledge: func(candidate NativeTBTCSignerStateWitnessTip) NativeTBTCSignerStateWitnessTip {
					candidate.AnchorServiceEpoch = 1
					candidate.AnchorRevision = 2
					candidate.AnchorEventRoot = [32]byte{13}
					candidate.AnchorAcknowledgementDigest = [32]byte{14}
					return candidate
				},
			}
			if err := InstallNativeTBTCSignerStateAnchorBarrier(
				NativeTBTCSignerStateAnchorBarrierConfig{
					InitialTip:                                &initial,
					ExpectedAnchorBindingHash:                 [32]byte{10},
					MinimumAnchorServiceEpoch:                 1,
					MaximumAnchorRevisionDistance:             4096,
					MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
					MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
					ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
					ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
						copy := current
						return &copy, nil
					},
					ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
					Committer:     committer,
				},
			); err != nil {
				t.Fatal(err)
			}

			releaseCalled := make(chan struct{})
			returned := make(chan error, 1)
			var discardCount atomic.Int32
			go func() {
				payload, err := executeNativeTBTCSignerStateAnchoredOutput(
					"InteractiveRound1",
					func() {
						current = candidate
					},
					func() ([]byte, error) {
						close(releaseCalled)
						if nativeErr {
							return nil, errors.New("native operation failed")
						}
						return []byte("sentinel-output"), nil
					},
					func() {
						discardCount.Add(1)
					},
				)
				if !nativeErr && string(payload) != "sentinel-output" {
					err = errors.New("sentinel output was not released")
				}
				returned <- err
			}()

			select {
			case <-commitStart:
			case <-time.After(time.Second):
				t.Fatal("remote commit did not start")
			}
			select {
			case <-releaseCalled:
				t.Fatal("native output was parsed before anchor acknowledgement")
			default:
			}
			select {
			case <-returned:
				t.Fatal("native call returned before anchor acknowledgement")
			default:
			}

			close(allowCommit)
			select {
			case err := <-returned:
				if nativeErr && (err == nil ||
					err.Error() != "native operation failed") {
					t.Fatalf("native error was not returned after acknowledgement: %v", err)
				}
				if !nativeErr && err != nil {
					t.Fatalf("anchored output was not released: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("native call did not return after acknowledgement")
			}
			if discardCount.Load() != 0 {
				t.Fatal("successfully anchored native result was discarded")
			}
		})
	}
}

func TestNativeTBTCSignerOutputBarrierDiscardsExactlyOnceOnAnchorFailure(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	candidate := testNativeTBTCSignerStateWitnessTip(2, initial.StateCommitment)
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		err:     errors.New("CAS outcome cannot be authenticated"),
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}
	var releaseCount atomic.Int32
	var discardCount atomic.Int32
	_, err := executeNativeTBTCSignerStateAnchoredOutput(
		"InteractiveRound2",
		func() {
			current = candidate
		},
		func() ([]byte, error) {
			releaseCount.Add(1)
			return []byte("must-not-escape"), nil
		},
		func() {
			discardCount.Add(1)
		},
	)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf("anchor failure did not poison output barrier: %v", err)
	}
	if releaseCount.Load() != 0 || discardCount.Load() != 1 {
		t.Fatalf(
			"anchor failure release/discard counts are [%d/%d], want [0/1]",
			releaseCount.Load(),
			discardCount.Load(),
		)
	}
}

func TestNativeTBTCSignerOutputBarrierDiscardsAndPoisonsOnInvokePanic(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	var discardCount atomic.Int32
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("native invoke panic did not propagate")
			}
		}()
		_, _ = executeNativeTBTCSignerStateAnchoredOutput(
			"InteractiveRound1",
			func() { panic("native call panic") },
			func() ([]byte, error) {
				t.Fatal("panicking native call released output")
				return nil, nil
			},
			func() { discardCount.Add(1) },
		)
	}()
	if discardCount.Load() != 1 {
		t.Fatalf(
			"panicking native call discard count is [%d], want [1]",
			discardCount.Load(),
		)
	}
	if _, err := beginNativeTBTCSignerStateAnchoredOperation(
		"InteractiveRound2",
	); !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf("panicking native call did not poison the barrier: %v", err)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierCommitsBeforeCompletion(t *testing.T) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		acknowledge: func(candidate NativeTBTCSignerStateWitnessTip) NativeTBTCSignerStateWitnessTip {
			candidate.AnchorBindingHash = [32]byte{10}
			candidate.AnchorServiceEpoch = 1
			candidate.AnchorRevision = 2
			candidate.AnchorEventRoot = [32]byte{11}
			candidate.AnchorAcknowledgementDigest = [32]byte{12}
			return candidate
		},
	}
	readTip := func() (*NativeTBTCSignerStateWitnessTip, error) {
		copy := current
		return &copy, nil
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip:                                   readTip,
			ReadTrustHead:                             readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:                                 committer,
		},
	); err != nil {
		t.Fatalf("cannot install state anchor barrier: %v", err)
	}

	lease, err := beginNativeTBTCSignerStateAnchoredOperation("InteractiveRound2")
	if err != nil {
		t.Fatalf("cannot begin anchored operation: %v", err)
	}
	candidate := testNativeTBTCSignerStateWitnessTip(2, initial.StateCommitment)
	current = candidate
	if err := lease.commit(); err != nil {
		lease.release()
		t.Fatalf("cannot commit anchored operation: %v", err)
	}
	lease.release()

	if committer.calls != 1 || committer.operation != "InteractiveRound2" ||
		committer.expected != initial || committer.candidate != candidate {
		t.Fatal("state transition committer did not receive the exact operation and tips")
	}
	if current.AnchorAcknowledgementDigest == [32]byte{} {
		t.Fatal("signed acknowledgement was not installed before completion")
	}
	if committer.verifyCalls != 2 {
		t.Fatal("startup and pre-operation authenticated remote reads were not required")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierChecksTipWithoutMutation(t *testing.T) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		acknowledge: func(candidate NativeTBTCSignerStateWitnessTip) NativeTBTCSignerStateWitnessTip {
			return candidate
		},
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}
	lease, err := beginNativeTBTCSignerStateAnchoredOperation("VerifySignatureShare")
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.commit(); err != nil {
		lease.release()
		t.Fatal(err)
	}
	lease.release()
	if committer.calls != 0 {
		t.Fatal("unchanged Rust tip unexpectedly issued a remote CAS")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierPoisonsAfterCommitFailure(t *testing.T) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		err:     errors.New("unknown CAS outcome"),
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	lease, err := beginNativeTBTCSignerStateAnchoredOperation("InteractiveRound1")
	if err != nil {
		t.Fatal(err)
	}
	current = testNativeTBTCSignerStateWitnessTip(2, initial.StateCommitment)
	err = lease.commit()
	lease.release()
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf("commit failure did not terminally poison the barrier: %v", err)
	}
	if _, err := beginNativeTBTCSignerStateAnchoredOperation(
		"InteractiveRound2",
	); !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) {
		t.Fatalf("poisoned barrier allowed another operation: %v", err)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierRejectsPreMigrationInitialTip(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	preMigration := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	postMigration := testNativeTBTCSignerStateWitnessTip(
		2,
		preMigration.StateCommitment,
	)
	committer := &testNativeTBTCSignerStateAnchorCommitter{}
	err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &preMigration,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := postMigration
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	)
	if err == nil {
		t.Fatal("tip captured before migration-induced advancement was accepted")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierRejectsUnacknowledgedInitialTip(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	initial.AnchorBindingHash = [32]byte{}
	initial.AnchorServiceEpoch = 0
	initial.AnchorRevision = 0
	initial.AnchorEventRoot = [32]byte{}
	initial.AnchorAcknowledgementDigest = [32]byte{}
	err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             4096,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := initial
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     &testNativeTBTCSignerStateAnchorCommitter{},
		},
	)
	if err == nil {
		t.Fatal("unacknowledged initial state tip was accepted")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierRejectsUnboundedRevisionWindow(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                &initial,
			ExpectedAnchorBindingHash: [32]byte{10},
			MinimumAnchorServiceEpoch: 1,
			MaximumAnchorRevisionDistance: NativeTBTCSignerStateAnchorMaximumRevisionDistance +
				1,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := initial
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     &testNativeTBTCSignerStateAnchorCommitter{},
		},
	)
	if err == nil {
		t.Fatal("caller-controlled revision window exceeded the frozen bound")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierRejectsUnboundedGenerationConfig(
	t *testing.T,
) {
	for _, test := range []struct {
		name     string
		distance uint64
		advance  uint64
	}{
		{
			"distance",
			NativeTBTCSignerStateAnchorMaximumGenerationDistance + 1,
			NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
		},
		{
			"per-operation advance",
			NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation + 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetNativeTBTCSignerStateAnchorBarrierForTest()
			t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

			initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
			err := InstallNativeTBTCSignerStateAnchorBarrier(
				NativeTBTCSignerStateAnchorBarrierConfig{
					InitialTip:                                &initial,
					ExpectedAnchorBindingHash:                 [32]byte{10},
					MinimumAnchorServiceEpoch:                 1,
					MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
					MaximumStateGenerationDistance:            test.distance,
					MaximumStateGenerationAdvancePerOperation: test.advance,
					ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
					ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
						copy := initial
						return &copy, nil
					},
					ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
					Committer:     &testNativeTBTCSignerStateAnchorCommitter{},
				},
			)
			if err == nil {
				t.Fatal("caller-controlled generation bound exceeded the frozen maximum")
			}
		})
	}
}

func TestNativeTBTCSignerStateAnchorBarrierRejectsInitialGenerationBeyondFloorWindow(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(
		1+NativeTBTCSignerStateAnchorMaximumGenerationDistance+1,
		[32]byte{2},
	)
	err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := initial
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     &testNativeTBTCSignerStateAnchorCommitter{},
		},
	)
	if err == nil {
		t.Fatal("initial generation beyond the certified floor window was accepted")
	}
}

func TestNativeTBTCSignerStateAnchorBarrierBlocksBeforeMutationAtRevisionBound(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	initial.AnchorRevision =
		testNativeTBTCSignerStateAnchorTrustHead().
			CertifiedFloor.Revision +
			NativeTBTCSignerStateAnchorMaximumRevisionDistance
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	invoked := false
	released := false
	discarded := false
	_, err := executeNativeTBTCSignerStateAnchoredOutput(
		"InteractiveRound1",
		func() {
			invoked = true
			current.Generation++
		},
		func() ([]byte, error) {
			released = true
			return nil, nil
		},
		func() {
			discarded = true
		},
	)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorUnavailable) {
		t.Fatalf("exhausted revision window was not blocked: %v", err)
	}
	if invoked || released || discarded || current != initial ||
		committer.calls != 0 || committer.verifyCalls != 1 {
		t.Fatal(
			"revision-bound admission mutated signer state or contacted the commit path",
		)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierBlocksBeforeMutationWithoutGenerationCapacity(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	floorGeneration :=
		testNativeTBTCSignerStateAnchorTrustHead().
			CertifiedFloor.Checkpoint.Generation
	initial := testNativeTBTCSignerStateWitnessTip(
		floorGeneration+
			NativeTBTCSignerStateAnchorMaximumGenerationDistance-2,
		[32]byte{2},
	)
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{current: &current}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	invoked := false
	released := false
	discarded := false
	_, err := executeNativeTBTCSignerStateAnchoredOutput(
		"InteractiveRound1",
		func() {
			invoked = true
		},
		func() ([]byte, error) {
			released = true
			return nil, nil
		},
		func() {
			discarded = true
		},
	)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorUnavailable) {
		t.Fatalf("insufficient generation capacity was not blocked: %v", err)
	}
	if invoked || released || discarded || current != initial ||
		committer.calls != 0 || committer.verifyCalls != 1 {
		t.Fatal(
			"generation-bound admission mutated signer state or contacted the commit path",
		)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierPoisonsOversizedGenerationAdvance(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{current: &current}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	var discarded atomic.Int32
	_, err := executeNativeTBTCSignerStateAnchoredOutput(
		"InteractiveRound2",
		func() {
			current = testNativeTBTCSignerStateWitnessTip(
				initial.Generation+
					NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation+1,
				initial.StateCommitment,
			)
		},
		func() ([]byte, error) {
			t.Fatal("oversized generation advance released native output")
			return nil, nil
		},
		func() {
			discarded.Add(1)
		},
	)
	if !errors.Is(err, ErrNativeTBTCSignerStateAnchorTerminal) ||
		discarded.Load() != 1 || committer.calls != 0 {
		t.Fatalf(
			"oversized generation advance was not terminally rejected: [%v] discarded [%d] commits [%d]",
			err,
			discarded.Load(),
			committer.calls,
		)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierAcceptsMaximumGenerationAdvance(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		acknowledge: func(
			candidate NativeTBTCSignerStateWitnessTip,
		) NativeTBTCSignerStateWitnessTip {
			candidate.AnchorRevision++
			candidate.AnchorEventRoot = [32]byte{0xa1}
			candidate.AnchorAcknowledgementDigest = [32]byte{0xa2}
			return candidate
		},
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            NativeTBTCSignerStateAnchorMaximumGenerationDistance,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	payload, err := executeNativeTBTCSignerStateAnchoredOutput(
		"InteractiveRound2",
		func() {
			current = testNativeTBTCSignerStateWitnessTip(
				initial.Generation+
					NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
				initial.StateCommitment,
			)
		},
		func() ([]byte, error) {
			return []byte("accepted"), nil
		},
		func() {
			t.Fatal("maximum valid generation advance was discarded")
		},
	)
	if err != nil || string(payload) != "accepted" ||
		current.Generation !=
			initial.Generation+
				NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation ||
		committer.calls != 1 {
		t.Fatalf(
			"maximum generation advance was rejected [payload %q generation %d commits %d err %v]",
			payload,
			current.Generation,
			committer.calls,
			err,
		)
	}
}

func TestNativeTBTCSignerStateAnchorBarrierGenerationCapacityAdvancesFasterThanRevision(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	initial := testNativeTBTCSignerStateWitnessTip(1, [32]byte{2})
	current := initial
	committer := &testNativeTBTCSignerStateAnchorCommitter{
		current: &current,
		acknowledge: func(
			candidate NativeTBTCSignerStateWitnessTip,
		) NativeTBTCSignerStateWitnessTip {
			candidate.AnchorRevision++
			candidate.AnchorEventRoot =
				[32]byte{byte(candidate.AnchorRevision + 20)}
			candidate.AnchorAcknowledgementDigest =
				[32]byte{byte(candidate.AnchorRevision + 30)}
			return candidate
		},
	}
	if err := InstallNativeTBTCSignerStateAnchorBarrier(
		NativeTBTCSignerStateAnchorBarrierConfig{
			InitialTip:                                &initial,
			ExpectedAnchorBindingHash:                 [32]byte{10},
			MinimumAnchorServiceEpoch:                 1,
			MaximumAnchorRevisionDistance:             NativeTBTCSignerStateAnchorMaximumRevisionDistance,
			MaximumStateGenerationDistance:            6,
			MaximumStateGenerationAdvancePerOperation: NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation,
			ExpectedTrustHead:                         testNativeTBTCSignerStateAnchorTrustHead(),
			ReadTip: func() (*NativeTBTCSignerStateWitnessTip, error) {
				copy := current
				return &copy, nil
			},
			ReadTrustHead: readTestNativeTBTCSignerStateAnchorTrustHead,
			Committer:     committer,
		},
	); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		lease, err := beginNativeTBTCSignerStateAnchoredOperation(
			"InteractiveRound1",
		)
		if err != nil {
			t.Fatal(err)
		}
		candidate := testNativeTBTCSignerStateWitnessTip(
			current.Generation+2,
			current.StateCommitment,
		)
		candidate.AnchorBindingHash = current.AnchorBindingHash
		candidate.AnchorServiceEpoch = current.AnchorServiceEpoch
		candidate.AnchorRevision = current.AnchorRevision
		candidate.AnchorEventRoot = current.AnchorEventRoot
		candidate.AnchorAcknowledgementDigest =
			current.AnchorAcknowledgementDigest
		current = candidate
		if err := lease.commit(); err != nil {
			lease.release()
			t.Fatal(err)
		}
		lease.release()
	}

	if current.Generation != initial.Generation+4 ||
		current.AnchorRevision != initial.AnchorRevision+2 ||
		committer.calls != 2 {
		t.Fatalf(
			"unexpected dual-dimension advance [generation %d revision %d commits %d]",
			current.Generation,
			current.AnchorRevision,
			committer.calls,
		)
	}
	if _, err := beginNativeTBTCSignerStateAnchoredOperation(
		"InteractiveRound1",
	); !errors.Is(err, ErrNativeTBTCSignerStateAnchorUnavailable) {
		t.Fatalf(
			"generation capacity did not block before the revision window: %v",
			err,
		)
	}
}

func TestValidateNativeTBTCSignerAcknowledgedTipRequiresNextRevisionInPinnedEpoch(
	t *testing.T,
) {
	candidate := testNativeTBTCSignerStateWitnessTip(2, [32]byte{3})
	valid := candidate
	valid.AnchorRevision++
	valid.AnchorEventRoot = [32]byte{0x41}
	valid.AnchorAcknowledgementDigest = [32]byte{0x42}
	if err := validateNativeTBTCSignerAcknowledgedTip(
		&candidate,
		&valid,
		candidate.AnchorBindingHash,
		candidate.AnchorServiceEpoch,
	); err != nil {
		t.Fatalf("next acknowledgement revision was rejected: %v", err)
	}

	for name, mutate := range map[string]func(*NativeTBTCSignerStateWitnessTip){
		"service epoch changed": func(tip *NativeTBTCSignerStateWitnessTip) {
			tip.AnchorServiceEpoch++
		},
		"revision skipped": func(tip *NativeTBTCSignerStateWitnessTip) {
			tip.AnchorRevision++
		},
		"revision did not advance": func(tip *NativeTBTCSignerStateWitnessTip) {
			tip.AnchorRevision = candidate.AnchorRevision
		},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := valid
			mutate(&invalid)
			if err := validateNativeTBTCSignerAcknowledgedTip(
				&candidate,
				&invalid,
				candidate.AnchorBindingHash,
				candidate.AnchorServiceEpoch,
			); err == nil {
				t.Fatal("invalid acknowledgement epoch/revision was accepted")
			}
		})
	}
}

func testNativeTBTCSignerStateWitnessTip(
	generation uint64,
	previousCommitment [32]byte,
) NativeTBTCSignerStateWitnessTip {
	storeFingerprint := [32]byte{1}
	stateImageDigest := [32]byte{byte(generation + 10)}
	tip := NativeTBTCSignerStateWitnessTip{
		Schema:                      NativeTBTCSignerStateWitnessTipSchema,
		StoreFingerprint:            storeFingerprint,
		Generation:                  generation,
		PreviousStateCommitment:     previousCommitment,
		StateImageDigest:            stateImageDigest,
		WitnessBaseGeneration:       1,
		AnchorBindingHash:           [32]byte{10},
		AnchorServiceEpoch:          1,
		AnchorRevision:              1,
		AnchorEventRoot:             [32]byte{11},
		AnchorAcknowledgementDigest: [32]byte{12},
	}
	tip.StateCommitment = ComputeNativeTBTCSignerStateWitnessCommitment(
		tip.StoreFingerprint,
		tip.Generation,
		tip.PreviousStateCommitment,
		tip.StateImageDigest,
	)
	if generation == 1 {
		tip.WitnessBaseCommitment = tip.StateCommitment
	} else {
		tip.WitnessBaseCommitment =
			testNativeTBTCSignerStateWitnessTip(1, [32]byte{2}).StateCommitment
	}
	return tip
}
