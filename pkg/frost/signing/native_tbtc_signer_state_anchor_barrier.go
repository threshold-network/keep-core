package signing

import (
	"context"
	"errors"
	"fmt"
	stdnet "net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ipfs/go-log/v2"
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

// nativeTBTCSignerStateAnchorEventLogger is the only logging capability this
// file needs. It is narrowed to one method so the poisoning event can be
// observed in tests without standing up a whole logger.
type nativeTBTCSignerStateAnchorEventLogger interface {
	Errorf(format string, args ...interface{})
}

// nativeTBTCSignerStateAnchorLogger names the one event this file emits: the
// transition into the terminal poisoned state. Nothing else here logs, because
// the barrier runs under the process-global signer mutation lock on every
// request-taking call and its refusals are already reported by the caller.
var nativeTBTCSignerStateAnchorLogger nativeTBTCSignerStateAnchorEventLogger = log.Logger(
	"keep-frost-tbtc-signer-state-anchor",
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
	// frozen maximum number of durable Rust generations one request-taking
	// call may advance before the barrier treats the process as terminally
	// poisoned. One generation is one committed state witness, and a call
	// reaches three durable writes: the expiry-sweep prologue's snapshot, the
	// second snapshot the sweep takes for a retirement its own repair
	// unblocked, and the endpoint's own write (or, on Round2/Aggregate, the
	// re-persist of a fail-closed marker in place of that write).
	//
	// This ceiling is deliberately one below what the engine can reach; see
	// NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation.
	NativeTBTCSignerStateAnchorMaximumGenerationAdvancePerOperation uint64 = 3

	// NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation
	// records what the signer engine can actually reach in one request-taking
	// call, which is one more than the frozen ceiling above. Every durable
	// write goes through the engine's replace_state, which commits up to two
	// witnesses: it first reconciles a witness an earlier call prepared and
	// left uncommitted after that call's rename won, and then prepares,
	// renames, and commits its own. Only the first of a call's three writes
	// can find such a carried-in witness, so the reachable worst case is
	// three writes plus one reconciliation.
	//
	// A call that reaches it exceeds the ceiling and poisons the barrier for
	// the life of the process. That is a documented residual, not an assertion
	// that it cannot happen: the interleaving needs an earlier persist that
	// failed after its rename and before its commit, so it is fault-driven,
	// and poisoning is fail-closed - request-taking calls then return
	// ErrNativeTBTCSignerStateAnchorTerminal, no signature share is released,
	// and no replay gate weakens. Raising the ceiling to four would widen the
	// only check that catches a call mutating more state than the pre-sign
	// admission accounting reserved for it, and is a protocol change to this
	// frozen bound rather than a documentation fix.
	NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation uint64 = 4
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

// nativeTBTCSignerStateAnchorPoisonRecord carries one poisoning cause to
// readers that must not take the barrier mutex. It is a struct rather than a
// bare error so a stored value is always non-nil and unambiguous.
type nativeTBTCSignerStateAnchorPoisonRecord struct {
	cause error
}

type nativeTBTCSignerStateAnchorBarrier struct {
	mutex sync.Mutex

	// poisonedSignal mirrors poisoned for callers that must observe the
	// terminal state without taking the barrier mutex. That mutex is held for
	// the whole of a request-taking call - the native call, the remote CAS,
	// and the acknowledgement readback - so a health or attestation path that
	// took it to read poisoned would stall behind a signing operation for the
	// full anchor timeout. Every write happens under the mutex in
	// recordNativeTBTCSignerStateAnchorPoisoning, so this can never report
	// poisoned before the barrier itself is, and it becomes visible in the
	// same critical section that poisons the barrier.
	poisonedSignal atomic.Pointer[nativeTBTCSignerStateAnchorPoisonRecord]

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
		// This is the only remote call the barrier makes BEFORE the native
		// call runs, so failing here cannot have diverged anything: no FFI
		// request-taking symbol has been entered, no Rust generation has
		// advanced, barrier.tip is untouched, and the local readback and trust
		// head were already checked against the installed identity above.
		// Releasing the mutex therefore leaves the barrier byte-for-byte as it
		// was, and the very next call re-reads and re-authenticates everything
		// from scratch.
		//
		// So a failure that only means "the anchor service did not answer this
		// second" - a redeploy, a TLS or DNS hiccup, a reset connection, a
		// momentary load spike - must not be terminal. Poisoning is
		// process-lifetime and only a restart clears it, so spending it on a
		// transport blip permanently disables FROST signing on this node for a
		// fault that healed on its own.
		//
		// Anything this does not positively recognize as a transport failure
		// still poisons. That is deliberate: a stale, rolled-back, forked, or
		// unauthenticated anchor is a comparison against data that was
		// successfully read, produces a plain error carrying none of the causes
		// below, and genuinely means it is unsafe to proceed.
		//
		// Both branches fail closed for THIS operation - no native call runs
		// and no signature share is released either way. The only difference is
		// whether the next call may try again.
		if isNativeTBTCSignerStateAnchorTransportFailure(err) {
			barrier.mutex.Unlock()
			return nil, fmt.Errorf(
				"%w: request-taking operation [%s] is blocked because the state anchor could not be reached before any signer state changed: %v",
				ErrNativeTBTCSignerStateAnchorUnavailable,
				operation,
				err,
			)
		}
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
		// Nothing advanced, so nothing was consumed: no generation, and no
		// revision either, because this path skips the CAS entirely. Counting
		// it would inflate the burn-rate numerator with operations the
		// anchor never had to witness.

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
		// Unlike the pre-operation authentication in
		// beginNativeTBTCSignerStateAnchoredOperation, a transport failure
		// HERE is not declassified, and the distinction is the whole reason
		// this comment exists.
		//
		// By this line the native call has already advanced durable Rust state
		// past lease.expected, so local and remote may disagree in either
		// direction and this node cannot tell which from the error alone. The
		// CAS is not idempotent and the transport cannot say whether the
		// service applied it: the anchor client already exhausts the only safe
		// disambiguation - on an ambiguous CAS it takes a fresh signed read and
		// either recovers the exact acknowledgement or refuses - so by the time
		// an error surfaces here every recoverable outcome has been tried. A
		// bare "the request timed out" is therefore indistinguishable from a
		// genuine CAS conflict, which is exactly the rollback/fork case the
		// anchor exists to catch.
		//
		// Releasing without poisoning would also not buy anything. barrier.tip
		// still holds lease.expected while Rust sits at candidate, so the next
		// call's pre-operation readback would mismatch the committed process
		// tip and poison there instead - one call later, with the original
		// cause lost. And lease.release poisons any lease that leaves
		// incomplete, so a non-poisoning exit would have to claim completion
		// for an operation that did not complete.
		//
		// The designed recovery for a local-ahead-of-remote state is a restart,
		// not a retry: frostNativeSignerAnchorBinding.reconcileStartup
		// re-authenticates the whole service history from the certified floor
		// and, when local is ahead and carries the exact remote anchor
		// reference it advanced from, proves the gap and catches the anchor up.
		// That path only runs at startup, under the offline-certified floor,
		// and after full history authentication - none of which this line can
		// reproduce while holding the signer lock mid-operation.
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

	// Count only after the acknowledgement is validated and durably read back,
	// so the totals describe work the anchor witnessed rather than work
	// attempted. Reaching here means exactly one compare-and-swap succeeded.
	recordNativeTBTCSignerStateAnchorConsumption(
		acknowledged.Generation-lease.expected.Generation,
		1,
	)

	barrier.tip = *acknowledged
	lease.completed = true
	return nil
}

func (lease *nativeTBTCSignerStateAnchorLease) poison(cause error) error {
	if lease == nil || lease.barrier == nil {
		return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
	}
	recordNativeTBTCSignerStateAnchorPoisoning(lease.barrier, cause)
	lease.completed = true
	return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
}

func (lease *nativeTBTCSignerStateAnchorLease) release() {
	if lease == nil || lease.barrier == nil {
		return
	}
	if !lease.completed {
		recordNativeTBTCSignerStateAnchorPoisoning(lease.barrier, fmt.Errorf(
			"native signer operation [%s] escaped without anchor completion",
			lease.operation,
		))
	}
	lease.barrier.mutex.Unlock()
	lease.barrier = nil
}

func poisonAndUnlockNativeTBTCSignerStateAnchor(
	barrier *nativeTBTCSignerStateAnchorBarrier,
	cause error,
) error {
	recordNativeTBTCSignerStateAnchorPoisoning(barrier, cause)
	barrier.mutex.Unlock()
	return fmt.Errorf("%w: %v", ErrNativeTBTCSignerStateAnchorTerminal, cause)
}

// recordNativeTBTCSignerStateAnchorPoisoning is the single place the barrier
// becomes terminally poisoned. It must be called with the barrier mutex held.
//
// It logs at ERROR exactly once per poisoning rather than once per refusal:
// every later request-taking call re-reports the same latched cause, and an
// operator whose node is refusing every signing round would otherwise get one
// ERROR line per attempt for the life of the process. The line names the
// remedy because it is not guessable from the message alone - poisoned lives on
// the package-global barrier and nothing clears it in-process, so only a
// restart recovers, and a restart is safe: startup re-runs anchor
// reconciliation and any durable witness the failed operation carried in is
// self-consuming on reload.
//
// The first cause wins. Poisoning is latched, and the first failure is the one
// that explains what actually happened; a later escape or refusal is only its
// consequence.
func recordNativeTBTCSignerStateAnchorPoisoning(
	barrier *nativeTBTCSignerStateAnchorBarrier,
	cause error,
) {
	if barrier == nil || barrier.poisoned != nil {
		return
	}
	barrier.poisoned = cause
	barrier.poisonedSignal.Store(
		&nativeTBTCSignerStateAnchorPoisonRecord{cause: cause},
	)
	nativeTBTCSignerStateAnchorLogger.Errorf(
		"FROST native tBTC signer state anchor is now terminally poisoned: "+
			"[%v]; every request-taking native signer call on this node is "+
			"refused from now on, and only restarting this process clears it",
		cause,
	)
}

// NativeTBTCSignerStateAnchorPoisoned reports the latched terminal anchor
// failure, or nil while the barrier is healthy. It is the supported way for
// health, admission, and attestation paths to observe that this node has
// stopped being able to sign, and it never blocks on an in-flight signer
// operation: it reads the lock-free mirror rather than taking the barrier
// mutex, which a request-taking call holds across its native call and remote
// commit.
//
// A nil result is not a promise that the next call will be admitted. The
// barrier can still refuse recoverably - it is not installed yet, a certified
// window is exhausted, or the anchor is momentarily unreachable - and those
// refusals are deliberately not terminal.
func NativeTBTCSignerStateAnchorPoisoned() error {
	record := globalNativeTBTCSignerStateAnchorBarrier.poisonedSignal.Load()
	if record == nil {
		return nil
	}
	return fmt.Errorf(
		"%w: %v",
		ErrNativeTBTCSignerStateAnchorTerminal,
		record.cause,
	)
}

// isNativeTBTCSignerStateAnchorTransportFailure reports whether err is a
// failure to REACH the anchor service rather than something the anchor service
// said.
//
// This mirrors isFrostPreSignTransientAuthorizationFailure in pkg/tbtc, cause
// for cause and deliberately no wider. It is duplicated rather than shared
// because pkg/tbtc imports this package, so importing it back would be an
// import cycle, and because the two callers must stay in lockstep: both use it
// to keep a transport blip from latching a permanent refusal.
//
// context.Canceled is excluded for the same reason it is there: it means the
// caller went away, not that the dependency is unreachable.
func isNativeTBTCSignerStateAnchorTransportFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, os.ErrDeadlineExceeded) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ENETDOWN) {
		return true
	}
	var operationError *stdnet.OpError
	if errors.As(err, &operationError) {
		return true
	}
	var resolverError *stdnet.DNSError
	if errors.As(err, &resolverError) {
		return true
	}
	var networkError stdnet.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return true
	}
	return false
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
	barrier.poisonedSignal.Store(nil)
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
