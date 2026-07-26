package signing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrNativeTBTCSignerStateAnchorUnavailable marks a request-taking native
	// signer call attempted before its independent state anchor was installed.
	ErrNativeTBTCSignerStateAnchorUnavailable = errors.New(
		"native tbtc signer state anchor is unavailable",
	)

	// ErrNativeTBTCSignerStateAnchorTerminal marks a process-terminal anchor
	// failure. It intentionally does not wrap ErrNativeCryptographyUnavailable:
	// callers must never route an anchor failure into a legacy implementation.
	ErrNativeTBTCSignerStateAnchorTerminal = errors.New(
		"native tbtc signer state anchor is terminally poisoned",
	)
)

const (
	defaultNativeTBTCSignerStateAnchorTimeout = 15 * time.Second

	// NativeTBTCSignerStateAnchorMaximumRevisionDistance is the frozen maximum
	// number of service revisions that can remain restartable from one
	// certified floor. Callers may choose a smaller conservative window but
	// cannot enlarge this protocol bound.
	NativeTBTCSignerStateAnchorMaximumRevisionDistance uint64 = 4096

	// NativeTBTCSignerStateAnchorMaximumGenerationDistance is the frozen
	// maximum number of Rust state generations that can remain provable from
	// one certified floor.
	NativeTBTCSignerStateAnchorMaximumGenerationDistance uint64 = 4096

	// NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation is the
	// maximum durable Rust state writes one request-taking call may perform:
	// one prepared-witness reconciliation, a sweep/repair snapshot (which
	// also covers protected retirement), and the operation's own write.
	NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation uint64 = 3
)

// NativeTBTCSignerStateAnchorCommitter durably commits a transition to the
// independent linearizable anchor, installs the exact signed acknowledgement
// into Rust, and returns the resulting exact Rust tip readback. It is invoked
// while the process-global signer mutation lock is held.
type NativeTBTCSignerStateAnchorCommitter interface {
	VerifyNativeTBTCSignerStateTip(
		context.Context,
		NativeTBTCSignerStateWitnessTip,
	) error
	CommitNativeTBTCSignerStateTransition(
		context.Context,
		string,
		NativeTBTCSignerStateWitnessTip,
		NativeTBTCSignerStateWitnessTip,
	) (*NativeTBTCSignerStateWitnessTip, error)
}

// NativeTBTCSignerStateAnchorBarrierConfig installs the process-global output
// barrier. InitialTip must have already passed startup reconciliation against
// the independent service.
type NativeTBTCSignerStateAnchorBarrierConfig struct {
	InitialTip                                *NativeTBTCSignerStateWitnessTip
	ExpectedAnchorBindingHash                 [32]byte
	MinimumAnchorServiceEpoch                 uint64
	MaximumAnchorRevisionDistance             uint64
	MaximumStateGenerationDistance            uint64
	MaximumStateGenerationAdvancePerOperation uint64
	ExpectedTrustHead                         *NativeTBTCSignerStateAnchorTrustHead
	ReadTip                                   func() (*NativeTBTCSignerStateWitnessTip, error)
	ReadTrustHead                             func() (*NativeTBTCSignerStateAnchorTrustHead, error)
	Committer                                 NativeTBTCSignerStateAnchorCommitter
	Timeout                                   time.Duration
}

type nativeTBTCSignerStateAnchorBarrier struct {
	mutex sync.Mutex

	installed     bool
	poisoned      error
	tip           NativeTBTCSignerStateWitnessTip
	readTip       func() (*NativeTBTCSignerStateWitnessTip, error)
	readTrustHead func() (*NativeTBTCSignerStateAnchorTrustHead, error)
	committer     NativeTBTCSignerStateAnchorCommitter
	timeout       time.Duration

	expectedAnchorBindingHash                 [32]byte
	minimumAnchorServiceEpoch                 uint64
	maximumAnchorRevisionDistance             uint64
	maximumStateGenerationDistance            uint64
	maximumStateGenerationAdvancePerOperation uint64
	expectedTrustHead                         NativeTBTCSignerStateAnchorTrustHead
}

var globalNativeTBTCSignerStateAnchorBarrier nativeTBTCSignerStateAnchorBarrier

// InstallNativeTBTCSignerStateAnchorBarrier installs one immutable process
// binding. Re-installation is rejected even if identical so tests, reload
// paths, and partial startup cannot silently exchange anchor authorities.
func InstallNativeTBTCSignerStateAnchorBarrier(
	config NativeTBTCSignerStateAnchorBarrierConfig,
) error {
	if config.InitialTip == nil || config.ReadTip == nil ||
		config.ReadTrustHead == nil || config.ExpectedTrustHead == nil ||
		config.Committer == nil ||
		config.ExpectedAnchorBindingHash == [32]byte{} ||
		config.MinimumAnchorServiceEpoch == 0 ||
		config.MaximumAnchorRevisionDistance == 0 ||
		config.MaximumAnchorRevisionDistance >
			NativeTBTCSignerStateAnchorMaximumRevisionDistance ||
		config.MaximumStateGenerationDistance == 0 ||
		config.MaximumStateGenerationDistance >
			NativeTBTCSignerStateAnchorMaximumGenerationDistance ||
		config.MaximumStateGenerationAdvancePerOperation == 0 ||
		config.MaximumStateGenerationAdvancePerOperation >
			NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation {
		return fmt.Errorf("native tbtc signer state anchor dependencies are incomplete")
	}
	initial := *config.InitialTip
	if err := validateNativeTBTCSignerStateWitnessTip(&initial); err != nil {
		return fmt.Errorf("native tbtc signer initial anchor tip is invalid: %w", err)
	}
	if initial.AnchorBindingHash != config.ExpectedAnchorBindingHash ||
		initial.AnchorServiceEpoch < config.MinimumAnchorServiceEpoch ||
		initial.AnchorRevision == 0 ||
		initial.AnchorEventRoot == [32]byte{} ||
		initial.AnchorAcknowledgementDigest == [32]byte{} {
		return fmt.Errorf(
			"native tbtc signer initial tip lacks the pinned signed anchor acknowledgement",
		)
	}
	expectedTrustHead := *config.ExpectedTrustHead
	if expectedTrustHead.Schema != NativeTBTCSignerStateAnchorTrustHeadSchema ||
		expectedTrustHead.CertificateSequence == 0 ||
		expectedTrustHead.CertificateDigest == [32]byte{} ||
		expectedTrustHead.BindingHash != config.ExpectedAnchorBindingHash ||
		expectedTrustHead.ServiceEpoch != initial.AnchorServiceEpoch ||
		expectedTrustHead.ServiceEpoch < config.MinimumAnchorServiceEpoch ||
		expectedTrustHead.CertifiedFloor.ServiceEpoch !=
			expectedTrustHead.ServiceEpoch ||
		expectedTrustHead.CertifiedFloor.Revision > initial.AnchorRevision ||
		initial.AnchorRevision-expectedTrustHead.CertifiedFloor.Revision >
			config.MaximumAnchorRevisionDistance ||
		expectedTrustHead.CertifiedFloor.Checkpoint.StoreFingerprint !=
			initial.StoreFingerprint ||
		expectedTrustHead.CertifiedFloor.Checkpoint.Generation == 0 ||
		expectedTrustHead.CertifiedFloor.Checkpoint.Generation >
			initial.Generation ||
		initial.Generation-
			expectedTrustHead.CertifiedFloor.Checkpoint.Generation >
			config.MaximumStateGenerationDistance {
		return fmt.Errorf(
			"native tbtc signer trust head differs from the initial anchor identity",
		)
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultNativeTBTCSignerStateAnchorTimeout
	}
	if timeout < time.Millisecond || timeout > time.Minute {
		return fmt.Errorf("native tbtc signer state anchor timeout is invalid")
	}

	barrier := &globalNativeTBTCSignerStateAnchorBarrier
	barrier.mutex.Lock()
	defer barrier.mutex.Unlock()
	if barrier.installed {
		return fmt.Errorf("native tbtc signer state anchor is already installed")
	}
	readback, err := config.ReadTip()
	if err != nil {
		return fmt.Errorf("cannot read native tbtc signer initial state tip: %w", err)
	}
	if readback == nil || *readback != initial {
		return fmt.Errorf("native tbtc signer initial state tip changed before installation")
	}
	trustReadback, err := config.ReadTrustHead()
	if err != nil {
		return fmt.Errorf("cannot read native tbtc signer trust head: %w", err)
	}
	if trustReadback == nil || *trustReadback != expectedTrustHead {
		return fmt.Errorf(
			"native tbtc signer trust head changed before barrier installation",
		)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := config.Committer.VerifyNativeTBTCSignerStateTip(
		ctx,
		initial,
	); err != nil {
		return fmt.Errorf(
			"cannot authenticate native tbtc signer initial remote anchor: %w",
			err,
		)
	}

	barrier.installed = true
	barrier.tip = initial
	barrier.readTip = config.ReadTip
	barrier.readTrustHead = config.ReadTrustHead
	barrier.committer = config.Committer
	barrier.timeout = timeout
	barrier.expectedAnchorBindingHash = config.ExpectedAnchorBindingHash
	barrier.minimumAnchorServiceEpoch = config.MinimumAnchorServiceEpoch
	barrier.maximumAnchorRevisionDistance =
		config.MaximumAnchorRevisionDistance
	barrier.maximumStateGenerationDistance =
		config.MaximumStateGenerationDistance
	barrier.maximumStateGenerationAdvancePerOperation =
		config.MaximumStateGenerationAdvancePerOperation
	barrier.expectedTrustHead = expectedTrustHead
	return nil
}

// nativeTBTCSignerStateAnchorLease serializes the native call, its post-call
// readback, remote CAS, Rust acknowledgement install, and final readback.
type nativeTBTCSignerStateAnchorLease struct {
	barrier   *nativeTBTCSignerStateAnchorBarrier
	operation string
	expected  NativeTBTCSignerStateWitnessTip
	completed bool
}

// executeNativeTBTCSignerStateAnchoredOutput is the single source-to-sink
// ordering seam for request-taking FFI calls. invoke may populate an opaque
// Rust-owned result, but releaseOutput (which may copy/parse it) cannot run
// until the remote commit and Rust acknowledgement readback complete. Every
// pre-release failure invokes discard exactly once.
func executeNativeTBTCSignerStateAnchoredOutput(
	operation string,
	invoke func(),
	releaseOutput func() ([]byte, error),
	discard func(),
) ([]byte, error) {
	if invoke == nil || releaseOutput == nil || discard == nil {
		return nil, fmt.Errorf("native tbtc signer output barrier callbacks are incomplete")
	}
	lease, err := beginNativeTBTCSignerStateAnchoredOperation(operation)
	if err != nil {
		return nil, err
	}
	invoked := false
	released := false
	defer func() {
		if invoked && !released {
			discard()
		}
		lease.release()
	}()

	invoked = true
	invoke()
	if err := lease.commit(); err != nil {
		return nil, err
	}
	released = true
	return releaseOutput()
}

func beginNativeTBTCSignerStateAnchoredOperation(
	operation string,
) (*nativeTBTCSignerStateAnchorLease, error) {
	barrier := &globalNativeTBTCSignerStateAnchorBarrier
	barrier.mutex.Lock()
	if !barrier.installed {
		barrier.mutex.Unlock()
		return nil, fmt.Errorf(
			"%w: request-taking operation [%s] is blocked",
			ErrNativeTBTCSignerStateAnchorUnavailable,
			operation,
		)
	}
	if barrier.poisoned != nil {
		err := barrier.poisoned
		barrier.mutex.Unlock()
		return nil, fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, err)
	}

	readback, err := barrier.readTip()
	if err != nil {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf("cannot read pre-operation state tip: %w", err),
		)
	}
	if readback == nil || *readback != barrier.tip {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf("pre-operation state tip differs from the committed process tip"),
		)
	}
	trustHead, err := barrier.readTrustHead()
	if err != nil {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf("cannot read pre-operation trust head: %w", err),
		)
	}
	if trustHead == nil || *trustHead != barrier.expectedTrustHead {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf("pre-operation trust head differs from the installed identity"),
		)
	}
	if readback.AnchorRevision <
		trustHead.CertifiedFloor.Revision ||
		readback.AnchorRevision-trustHead.CertifiedFloor.Revision >=
			barrier.maximumAnchorRevisionDistance {
		barrier.mutex.Unlock()
		return nil, fmt.Errorf(
			"%w: request-taking operation [%s] is blocked because the certified anchor revision window is exhausted; offline anchor rotation is required",
			ErrNativeTBTCSignerStateAnchorUnavailable,
			operation,
		)
	}
	certifiedFloorGeneration :=
		trustHead.CertifiedFloor.Checkpoint.Generation
	if readback.Generation < certifiedFloorGeneration {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf(
				"pre-operation state generation precedes the certified floor",
			),
		)
	}
	generationDistance := readback.Generation - certifiedFloorGeneration
	if generationDistance > barrier.maximumStateGenerationDistance ||
		barrier.maximumStateGenerationDistance-generationDistance <
			barrier.maximumStateGenerationAdvancePerOperation {
		barrier.mutex.Unlock()
		return nil, fmt.Errorf(
			"%w: request-taking operation [%s] is blocked because the certified signer-generation window cannot cover its maximum advance; offline anchor rotation is required",
			ErrNativeTBTCSignerStateAnchorUnavailable,
			operation,
		)
	}
	ctx, cancel := context.WithTimeout(context.Background(), barrier.timeout)
	defer cancel()
	if err := barrier.committer.VerifyNativeTBTCSignerStateTip(
		ctx,
		*readback,
	); err != nil {
		return nil, poisonAndUnlockNativeTBTCSignerStateAnchor(
			barrier,
			fmt.Errorf("cannot authenticate pre-operation remote state anchor: %w", err),
		)
	}

	return &nativeTBTCSignerStateAnchorLease{
		barrier:   barrier,
		operation: operation,
		expected:  *readback,
	}, nil
}

// commit observes the Rust tip after the native call even when that call
// returned an application error. If Rust advanced, it blocks until the exact
// candidate and acknowledgement are durable outside the signer and read back
// from Rust. The caller may parse or copy its native response only after this
// method succeeds.
func (lease *nativeTBTCSignerStateAnchorLease) commit() error {
	if lease == nil || lease.barrier == nil || lease.completed {
		return fmt.Errorf("native tbtc signer state anchor lease is invalid")
	}
	barrier := lease.barrier
	candidate, err := barrier.readTip()
	if err != nil {
		return lease.poison(fmt.Errorf("cannot read post-operation state tip: %w", err))
	}
	if candidate == nil {
		return lease.poison(fmt.Errorf("post-operation state tip is nil"))
	}
	if err := validateNativeTBTCSignerStateTransition(
		&lease.expected,
		candidate,
	); err != nil {
		return lease.poison(err)
	}
	if candidate.Generation > lease.expected.Generation &&
		candidate.Generation-lease.expected.Generation >
			barrier.maximumStateGenerationAdvancePerOperation {
		return lease.poison(fmt.Errorf(
			"native signer operation advanced [%d] generations, exceeding the per-operation bound [%d]",
			candidate.Generation-lease.expected.Generation,
			barrier.maximumStateGenerationAdvancePerOperation,
		))
	}
	certifiedFloorGeneration :=
		barrier.expectedTrustHead.CertifiedFloor.Checkpoint.Generation
	if candidate.Generation < certifiedFloorGeneration ||
		candidate.Generation-certifiedFloorGeneration >
			barrier.maximumStateGenerationDistance {
		return lease.poison(fmt.Errorf(
			"native signer operation exceeded the certified signer-generation window",
		))
	}

	if *candidate == lease.expected {
		barrier.tip = *candidate
		lease.completed = true
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), barrier.timeout)
	defer cancel()
	acknowledged, err := barrier.committer.CommitNativeTBTCSignerStateTransition(
		ctx,
		lease.operation,
		lease.expected,
		*candidate,
	)
	if err != nil {
		return lease.poison(fmt.Errorf(
			"cannot commit native tbtc signer state transition: %w",
			err,
		))
	}
	if acknowledged == nil {
		return lease.poison(fmt.Errorf("state anchor returned a nil acknowledged tip"))
	}
	if !sameNativeTBTCSignerStateCheckpoint(acknowledged, candidate) {
		return lease.poison(fmt.Errorf(
			"state anchor acknowledged a different native signer checkpoint",
		))
	}
	if err := validateNativeTBTCSignerAcknowledgedTip(
		candidate,
		acknowledged,
		barrier.expectedAnchorBindingHash,
		barrier.minimumAnchorServiceEpoch,
	); err != nil {
		return lease.poison(err)
	}

	finalReadback, err := barrier.readTip()
	if err != nil {
		return lease.poison(fmt.Errorf(
			"cannot read acknowledged native tbtc signer state tip: %w",
			err,
		))
	}
	if finalReadback == nil || *finalReadback != *acknowledged {
		return lease.poison(fmt.Errorf(
			"native signer did not durably install the exact anchor acknowledgement",
		))
	}

	barrier.tip = *acknowledged
	lease.completed = true
	return nil
}

func (lease *nativeTBTCSignerStateAnchorLease) poison(cause error) error {
	if lease == nil || lease.barrier == nil {
		return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
	}
	lease.barrier.poisoned = cause
	lease.completed = true
	return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
}

func (lease *nativeTBTCSignerStateAnchorLease) release() {
	if lease == nil || lease.barrier == nil {
		return
	}
	if !lease.completed {
		lease.barrier.poisoned = fmt.Errorf(
			"native signer operation [%s] escaped without anchor completion",
			lease.operation,
		)
	}
	lease.barrier.mutex.Unlock()
	lease.barrier = nil
}

func poisonAndUnlockNativeTBTCSignerStateAnchor(
	barrier *nativeTBTCSignerStateAnchorBarrier,
	cause error,
) error {
	barrier.poisoned = cause
	barrier.mutex.Unlock()
	return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
}

func validateNativeTBTCSignerStateWitnessTip(
	tip *NativeTBTCSignerStateWitnessTip,
) error {
	if tip == nil || tip.Schema != NativeTBTCSignerStateWitnessTipSchema ||
		tip.StoreFingerprint == [32]byte{} || tip.Generation == 0 ||
		tip.PreviousStateCommitment == [32]byte{} ||
		tip.StateCommitment == [32]byte{} ||
		tip.WitnessBaseGeneration == 0 ||
		tip.WitnessBaseGeneration > tip.Generation ||
		tip.WitnessBaseCommitment == [32]byte{} {
		return fmt.Errorf("native signer state-witness tip is incomplete")
	}
	computed := ComputeNativeTBTCSignerStateWitnessCommitment(
		tip.StoreFingerprint,
		tip.Generation,
		tip.PreviousStateCommitment,
		tip.StateImageDigest,
	)
	if computed != tip.StateCommitment {
		return fmt.Errorf("native signer state-witness tip commitment is invalid")
	}
	hasAnchor := tip.AnchorBindingHash != [32]byte{}
	if hasAnchor != (tip.AnchorServiceEpoch != 0) ||
		hasAnchor != (tip.AnchorRevision != 0) ||
		hasAnchor != (tip.AnchorEventRoot != [32]byte{}) ||
		hasAnchor != (tip.AnchorAcknowledgementDigest != [32]byte{}) {
		return fmt.Errorf("native signer anchor acknowledgement metadata is partial")
	}
	return nil
}

func validateNativeTBTCSignerStateTransition(
	expected *NativeTBTCSignerStateWitnessTip,
	candidate *NativeTBTCSignerStateWitnessTip,
) error {
	if err := validateNativeTBTCSignerStateWitnessTip(expected); err != nil {
		return fmt.Errorf("committed native signer state tip is invalid: %w", err)
	}
	if err := validateNativeTBTCSignerStateWitnessTip(candidate); err != nil {
		return fmt.Errorf("candidate native signer state tip is invalid: %w", err)
	}
	if expected.StoreFingerprint != candidate.StoreFingerprint ||
		candidate.Generation < expected.Generation ||
		candidate.WitnessBaseGeneration < expected.WitnessBaseGeneration ||
		candidate.WitnessBaseGeneration > expected.Generation {
		return fmt.Errorf("native signer state transition has invalid store or generation bounds")
	}
	if candidate.WitnessBaseGeneration == expected.WitnessBaseGeneration &&
		candidate.WitnessBaseCommitment == expected.WitnessBaseCommitment {
		// Base is unchanged.
	} else if candidate.WitnessBaseGeneration == expected.Generation &&
		candidate.WitnessBaseCommitment == expected.StateCommitment {
		// Rust may lazily rotate to the exact already-acknowledged checkpoint.
	} else {
		return fmt.Errorf(
			"native signer witness base did not remain fixed or rotate to the committed tip",
		)
	}
	if candidate.Generation == expected.Generation {
		if *candidate != *expected {
			return fmt.Errorf("native signer state changed without advancing its generation")
		}
		return nil
	}
	if candidate.StateCommitment == expected.StateCommitment {
		return fmt.Errorf("native signer state generation advanced without a new commitment")
	}
	// A signer operation may not forge or erase the separately persisted remote
	// acknowledgement. Only the post-CAS acknowledgement call can change it.
	if candidate.AnchorBindingHash != expected.AnchorBindingHash ||
		candidate.AnchorServiceEpoch != expected.AnchorServiceEpoch ||
		candidate.AnchorRevision != expected.AnchorRevision ||
		candidate.AnchorEventRoot != expected.AnchorEventRoot ||
		candidate.AnchorAcknowledgementDigest != expected.AnchorAcknowledgementDigest {
		return fmt.Errorf("native signer operation changed anchor acknowledgement metadata")
	}
	return nil
}

func validateNativeTBTCSignerAcknowledgedTip(
	candidate *NativeTBTCSignerStateWitnessTip,
	acknowledged *NativeTBTCSignerStateWitnessTip,
	expectedBindingHash [32]byte,
	minimumServiceEpoch uint64,
) error {
	if err := validateNativeTBTCSignerStateWitnessTip(acknowledged); err != nil {
		return fmt.Errorf("acknowledged native signer state tip is invalid: %w", err)
	}
	if acknowledged.AnchorBindingHash != expectedBindingHash ||
		acknowledged.AnchorServiceEpoch < minimumServiceEpoch ||
		acknowledged.AnchorRevision == 0 ||
		acknowledged.AnchorEventRoot == [32]byte{} ||
		acknowledged.AnchorAcknowledgementDigest == [32]byte{} {
		return fmt.Errorf("native signer state acknowledgement metadata is absent")
	}
	if acknowledged.WitnessBaseGeneration == candidate.WitnessBaseGeneration &&
		acknowledged.WitnessBaseCommitment == candidate.WitnessBaseCommitment {
		// Base is unchanged.
	} else if acknowledged.WitnessBaseGeneration == candidate.Generation &&
		acknowledged.WitnessBaseCommitment == candidate.StateCommitment {
		// Rust rotated exactly to the newly acknowledged checkpoint.
	} else {
		return fmt.Errorf(
			"native signer acknowledgement rotated to an unacknowledged witness base",
		)
	}
	if candidate.AnchorBindingHash != [32]byte{} &&
		acknowledged.AnchorBindingHash != candidate.AnchorBindingHash {
		return fmt.Errorf("native signer state acknowledgement binding changed")
	}
	if candidate.AnchorServiceEpoch != 0 {
		if candidate.AnchorRevision == ^uint64(0) ||
			acknowledged.AnchorServiceEpoch != candidate.AnchorServiceEpoch ||
			acknowledged.AnchorRevision != candidate.AnchorRevision+1 {
			return fmt.Errorf(
				"native signer state acknowledgement did not advance by one revision in the pinned service epoch",
			)
		}
	}
	return nil
}

func sameNativeTBTCSignerStateCheckpoint(
	left *NativeTBTCSignerStateWitnessTip,
	right *NativeTBTCSignerStateWitnessTip,
) bool {
	return left != nil && right != nil &&
		left.StoreFingerprint == right.StoreFingerprint &&
		left.Generation == right.Generation &&
		left.PreviousStateCommitment == right.PreviousStateCommitment &&
		left.StateImageDigest == right.StateImageDigest &&
		left.StateCommitment == right.StateCommitment
}

func resetNativeTBTCSignerStateAnchorBarrierForTest() {
	barrier := &globalNativeTBTCSignerStateAnchorBarrier
	barrier.mutex.Lock()
	defer barrier.mutex.Unlock()
	barrier.installed = false
	barrier.poisoned = nil
	barrier.tip = NativeTBTCSignerStateWitnessTip{}
	barrier.readTip = nil
	barrier.readTrustHead = nil
	barrier.committer = nil
	barrier.timeout = 0
	barrier.expectedAnchorBindingHash = [32]byte{}
	barrier.minimumAnchorServiceEpoch = 0
	barrier.maximumAnchorRevisionDistance = 0
	barrier.maximumStateGenerationDistance = 0
	barrier.maximumStateGenerationAdvancePerOperation = 0
	barrier.expectedTrustHead = NativeTBTCSignerStateAnchorTrustHead{}
}
