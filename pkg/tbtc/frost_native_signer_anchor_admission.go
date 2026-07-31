package tbtc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	// One request-taking interactive signer call performs up to three durable
	// Rust writes: the expiry-sweep prologue takes one snapshot and, when a
	// prior write left a repair pending, a second for the retirement that
	// repair unblocked, then the endpoint persists its own mutation (or, on
	// Round2/Aggregate, re-persists a fail-closed marker instead of it).
	// Matching pending operations are covered by the sweep; nonmatching
	// operations remain pending without another write. The process output
	// barrier still advances the remote anchor revision only once for the
	// call's final checkpoint.
	//
	// Those three writes are not three generations. One generation is one
	// committed state witness, and the signer's replace_state commits up to
	// two per write: it first reconciles a witness an earlier call prepared
	// and left uncommitted after that call's rename won, then prepares,
	// renames, and commits its own. Only a call's first write can find such a
	// carried-in witness, so a single call can reach FOUR advances - see
	// frostsigning.NativeTBTCSignerStateAnchorEngineReachableGeneration
	// AdvancePerOperation, which pins that reachable worst case.
	//
	// This constant stays at three anyway, and the gap is a documented
	// residual rather than a claim that four cannot happen. Node startup
	// installs it as the output barrier's MaximumStateGenerationAdvance
	// PerOperation, whose own frozen protocol ceiling is three, and a call
	// that advances four poisons the process terminally. The fourth advance
	// needs an earlier persist that failed after its rename and before its
	// commit, so it is fault-driven, and the poison is fail-closed: no share
	// is released and no replay gate weakens. Raising it would widen the only
	// check that catches an anchored call mutating more than this accounting
	// reserved, and would have to raise the frozen barrier ceiling with it.
	frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall uint64 = 3

	// A workflow reservation cannot multiply the per-operation bound by its
	// call count: at production parameters that reserves more of the certified
	// proof window than exists, and excludes every operator holding three or
	// more local seats from full-size batches. What actually bounds the
	// consumption is the witness journal - generations advanced equals witness
	// COMMITs, and every COMMIT belongs either to a persist that reached its
	// rename or to the single witness that may have been carried into the
	// workflow already prepared. The cost is therefore the number of persists
	// that reach a rename, not the call count times the per-call ceiling.
	//
	// Two per call is the fault-free steady state, not a proven upper bound:
	// the sweep prologue's snapshot plus the endpoint's own mutation. The
	// sweep's second snapshot and the Round2/Aggregate marker re-persist both
	// require a pending persistence operation that only a failed persist
	// creates, and the marker re-persist excludes that call's own mutation
	// because flushing the marker makes the replay gate reject the retry. A
	// persist that fails BEFORE its rename advances nothing, so its call
	// leaves an advance unspent and the later repair lands the one it did not.
	// A persist that fails AFTER its rename does not leave that slack: it has
	// either already committed its own witness (a failed post-commit
	// revalidation) or left it for the next call's reconciliation to commit,
	// while the repair call that follows still pays its own sweep snapshot,
	// its repair snapshot, and its own mutation. Each post-rename persist
	// failure therefore costs roughly one generation more than this reserves;
	// the workflow-wide allowance below is what absorbs them.
	frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall uint64 = 2

	// The workflow-wide allowance on top of the amortized per-call cost. It
	// covers one witness prepared before admission and reconciled inside the
	// workflow - prepare_witness refuses to prepare a second, so at most one
	// is ever outstanding - plus roughly two further post-rename persist
	// failures, each of which overruns the amortized bound by about one
	// generation.
	//
	// It is an allowance, not a proof, and the reservation it completes is not
	// exact. Nothing in the signer bounds how many times a failing store can
	// fail a persist after its rename, so a workflow that suffers more of them
	// than this covers consumes more of the proof window than it reserved. The
	// allowance is kept small deliberately: the slack that really absorbs
	// faults is that most calls sweep nothing and spend one advance rather
	// than two, and a node failing this many persists after rename has a
	// failing store, where blocking is the correct outcome.
	//
	// The residual is fail-closed availability, never authority. The
	// reservation only promises that the certified proof window can cover the
	// whole workflow; overrunning it means that window can run out mid
	// workflow. What an operator then sees is request-taking calls blocked
	// with "the certified signer-generation window cannot cover its maximum
	// advance; offline anchor rotation is required" and new pre-sign work
	// rejected for want of unreserved headroom - on a node that was already
	// failing every one of those persists. No share is released, no replay
	// gate weakens, and the recovery is the offline anchor rotation that
	// window exhaustion always requires.
	frostNativeSignerTerminalGenerationAdvancesPerWorkflow uint64 = 3
)

type frostNativeSignerAnchorCapacity struct {
	Revisions   uint64
	Generations uint64
}

// frostNativeSignerAnchorAdmissionController accounts for both independent
// restart bounds of every production workflow that may mutate native signer
// state after startup:
//
//   - service revisions retained by the external anchor; and
//   - Rust state generations retained by the witness proof window.
//
// Pre-sign authorization and native DKG share one controller, so concurrent
// workflows cannot consume either dimension already promised to admitted work.
// Reservations retain their full worst-case cost until workflow exit. Because
// current headroom already reflects consumed work, this intentionally
// double-counts in-flight mutations for later admissions and is conservative.
type frostNativeSignerAnchorAdmissionController struct {
	mutex sync.Mutex

	anchorBinding *frostNativeSignerAnchorBinding
	// readHeadroom is test-only injection. Production always authenticates the
	// current tip through anchorBinding while holding mutex.
	readHeadroom func(context.Context) (frostNativeSignerAnchorCapacity, error)
	// anchorPoisoned is test-only injection. Production always reads the
	// process-global state-anchor barrier through
	// frostsigning.NativeTBTCSignerStateAnchorPoisoned. The seam exists because
	// that barrier is a package-global in another package with no exported way
	// to poison it, and poisoning it for real inside a test would latch a
	// terminal failure into every later test in this process.
	anchorPoisoned func() error

	reserved frostNativeSignerAnchorCapacity
}

type frostNativeSignerAnchorRevisionReservation struct {
	controller *frostNativeSignerAnchorAdmissionController
	cost       frostNativeSignerAnchorCapacity
	release    sync.Once
}

func newFrostNativeSignerAnchorAdmissionController(
	anchorBinding *frostNativeSignerAnchorBinding,
) (*frostNativeSignerAnchorAdmissionController, error) {
	if anchorBinding == nil {
		return nil, fmt.Errorf(
			"FROST native signer anchor admission binding is nil",
		)
	}

	return &frostNativeSignerAnchorAdmissionController{
		anchorBinding: anchorBinding,
	}, nil
}

// reservePreSign reserves the entire upper bound before backend authorization
// or native signing begins. The readiness snapshot was independently
// reconciled twice, but may be stale by the time this mutex is acquired. The
// authenticated current tip is therefore read under the admission lock and the
// smaller headroom in each dimension is authoritative.
func (controller *frostNativeSignerAnchorAdmissionController) reservePreSign(
	ctx context.Context,
	snapshot *frostProductionSignerReadinessSnapshot,
	inputCount uint64,
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (*frostNativeSignerAnchorRevisionReservation, error) {
	cost, err := frostPreSignMaximumAnchorCapacityCost(
		inputCount,
		localSeatCount,
		maximumSigningAttempts,
	)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.Inventory == nil {
		return nil, fmt.Errorf(
			"FROST pre-sign anchor admission has no authenticated inventory snapshot",
		)
	}
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		snapshot.Inventory,
	); err != nil {
		return nil, err
	}
	snapshotHeadroom := frostNativeSignerAnchorCapacity{
		Revisions:   snapshot.Inventory.RestartableRevisionHeadroom,
		Generations: snapshot.Inventory.RestartableGenerationHeadroom,
	}

	return controller.reserve(
		"FROST pre-sign authorization",
		cost,
		func() (frostNativeSignerAnchorCapacity, error) {
			current, err := controller.currentRestartableHeadroom(ctx)
			if err != nil {
				return frostNativeSignerAnchorCapacity{}, err
			}
			return frostNativeSignerAnchorCapacity{
				Revisions: minFrostNativeSignerAnchorHeadroom(
					snapshotHeadroom.Revisions,
					current.Revisions,
				),
				Generations: minFrostNativeSignerAnchorHeadroom(
					snapshotHeadroom.Generations,
					current.Generations,
				),
			}, nil
		},
	)
}

// reserveDKG accounts for one request-taking persistence call and one
// worst-case retirement call per local seat. Successful seats normally share
// one key group and need one retirement, but a disagreement can produce one
// distinct durable group per seat. DKG has no interactive sweep prologue, so
// each call advances at most one service revision and one Rust generation.
func (controller *frostNativeSignerAnchorAdmissionController) reserveDKG(
	ctx context.Context,
	localSeatCount uint64,
) (*frostNativeSignerAnchorRevisionReservation, error) {
	if localSeatCount == 0 {
		return nil, fmt.Errorf(
			"FROST native DKG anchor admission controls no local seats",
		)
	}
	if localSeatCount > ^uint64(0)/2 {
		return nil, fmt.Errorf(
			"FROST native DKG anchor admission seat count overflows",
		)
	}
	maximumPersistenceCalls := localSeatCount * 2
	cost := frostNativeSignerAnchorCapacity{
		Revisions:   maximumPersistenceCalls,
		Generations: maximumPersistenceCalls,
	}

	return controller.reserve(
		"FROST native DKG",
		cost,
		func() (frostNativeSignerAnchorCapacity, error) {
			return controller.currentRestartableHeadroom(ctx)
		},
	)
}

func (controller *frostNativeSignerAnchorAdmissionController) reserveDKGRetirement(
	ctx context.Context,
	keyGroupCount uint64,
) (*frostNativeSignerAnchorRevisionReservation, error) {
	if keyGroupCount == 0 {
		return nil, fmt.Errorf(
			"FROST native DKG retirement controls no key groups",
		)
	}
	cost := frostNativeSignerAnchorCapacity{
		Revisions:   keyGroupCount,
		Generations: keyGroupCount,
	}
	return controller.reserve(
		"FROST native DKG retirement",
		cost,
		func() (frostNativeSignerAnchorCapacity, error) {
			return controller.currentRestartableHeadroom(ctx)
		},
	)
}

func (controller *frostNativeSignerAnchorAdmissionController) reserve(
	workflow string,
	cost frostNativeSignerAnchorCapacity,
	readHeadroom func() (frostNativeSignerAnchorCapacity, error),
) (*frostNativeSignerAnchorRevisionReservation, error) {
	if controller == nil || readHeadroom == nil ||
		cost.Revisions == 0 || cost.Generations == 0 {
		return nil, fmt.Errorf(
			"%s anchor admission dependencies are incomplete",
			workflow,
		)
	}

	// A terminally poisoned state anchor is the one refusal reason this
	// controller cannot otherwise see. currentRestartableHeadroom reads the
	// witness tip straight through the inventory bridge and authenticates it
	// against the anchor service; neither path takes the request-taking
	// barrier, so a barrier that has already latched a terminal failure still
	// reports perfectly healthy headroom here. Without this check the node goes
	// on admitting pre-sign, DKG and DKG-retirement workflows whose every
	// request-taking signer call will be refused with
	// ErrNativeTBTCSignerStateAnchorTerminal - it accepts work it cannot
	// finish, and the wallet loses this member's seats toward its signing
	// threshold with no node reporting a cause.
	//
	// This is checked before the headroom read, and outside the admission
	// mutex, because it is both cheaper (one atomic load against an anchor
	// round trip) and strictly more terminal than anything the headroom can
	// say: nothing clears poisoned in-process, so no later headroom value can
	// make this workflow admissible. Racing a poisoning that lands immediately
	// after this load is harmless - the reservation is then made and the
	// workflow's own calls are refused by the barrier, which is the behavior
	// that already existed.
	anchorPoisoned := frostsigning.NativeTBTCSignerStateAnchorPoisoned
	if controller.anchorPoisoned != nil {
		anchorPoisoned = controller.anchorPoisoned
	}
	if poisoned := anchorPoisoned(); poisoned != nil {
		recordFrostNativeSignerAnchorPoisonedRejection()
		return nil, fmt.Errorf(
			"%s is blocked because this node's native signer state anchor is "+
				"terminally poisoned: [%w]; every request-taking native signer "+
				"call is refused until this process is restarted, and only a "+
				"process restart clears it",
			workflow,
			poisoned,
		)
	}

	controller.mutex.Lock()
	defer controller.mutex.Unlock()

	headroom, err := readHeadroom()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot determine %s anchor headroom: [%w]",
			workflow,
			err,
		)
	}
	minimumHeadroom := minFrostNativeSignerAnchorHeadroom(
		headroom.Revisions,
		headroom.Generations,
	)
	if minimumHeadroom <= FrostNativeSignerAnchorRotationWarningHeadroom {
		recordFrostNativeSignerAnchorRotationFloorRejection()
		return nil, fmt.Errorf(
			"%s is blocked with revision/generation headroom [%d/%d]; "+
				"offline anchor rotation is required before admitting new work",
			workflow,
			headroom.Revisions,
			headroom.Generations,
		)
	}
	// Both unreserved-headroom refusals name the same remedy as the
	// rotation-floor refusal above. They are not a smaller version of it: a
	// node refused here still has most of both windows unspent - a four-seat
	// node is refused its next maximum-size sweep with 3508 of 4096 generations
	// left - so the number in the message reads healthy and the refusal looks
	// transient. It is not. Neither window refills on its own; only an
	// offline-authorized rotation moves the certified floor forward, so this is
	// the first message an operator of a correctly configured node sees, and it
	// has to say what to do about it rather than leave rotation to be inferred
	// from the far rarer floor refusal.
	if controller.reserved.Revisions > headroom.Revisions ||
		cost.Revisions >
			headroom.Revisions-controller.reserved.Revisions {
		recordFrostNativeSignerAnchorUnreservedHeadroomRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] anchor revisions but only [%d] are unreserved; "+
				"the certified history window does not refill on its own, so "+
				"offline anchor rotation is required before this workflow can "+
				"be admitted",
			workflow,
			cost.Revisions,
			headroom.Revisions-controller.reserved.Revisions,
		)
	}
	if controller.reserved.Generations > headroom.Generations ||
		cost.Generations >
			headroom.Generations-controller.reserved.Generations {
		recordFrostNativeSignerAnchorUnreservedHeadroomRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] signer generations but only [%d] are unreserved; "+
				"the certified proof window does not refill on its own, so "+
				"offline anchor rotation is required before this workflow can "+
				"be admitted",
			workflow,
			cost.Generations,
			headroom.Generations-controller.reserved.Generations,
		)
	}

	controller.reserved.Revisions += cost.Revisions
	controller.reserved.Generations += cost.Generations
	return &frostNativeSignerAnchorRevisionReservation{
		controller: controller,
		cost:       cost,
	}, nil
}

func (controller *frostNativeSignerAnchorAdmissionController) currentRestartableHeadroom(
	ctx context.Context,
) (frostNativeSignerAnchorCapacity, error) {
	if controller == nil || controller.anchorBinding == nil {
		if controller != nil && controller.readHeadroom != nil {
			return controller.readHeadroom(ctx)
		}
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST native signer anchor admission binding is unavailable",
		)
	}
	if ctx == nil {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST native signer anchor admission context is nil",
		)
	}

	tip, err := controller.anchorBinding.readTip()
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"cannot read native signer state tip: [%w]",
			err,
		)
	}
	if tip == nil {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"native signer state tip is nil",
		)
	}
	if err := controller.anchorBinding.VerifyNativeTBTCSignerStateTip(
		ctx,
		*tip,
	); err != nil {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"cannot authenticate native signer state tip: [%w]",
			err,
		)
	}

	revisionHeadroom, err :=
		controller.anchorBinding.restartableRevisionHeadroom(
			tip.AnchorServiceEpoch,
			tip.AnchorRevision,
		)
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, err
	}
	generationHeadroom, err :=
		controller.anchorBinding.restartableGenerationHeadroom(
			tip.Generation,
		)
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, err
	}
	return frostNativeSignerAnchorCapacity{
		Revisions:   revisionHeadroom,
		Generations: generationHeadroom,
	}, nil
}

func (reservation *frostNativeSignerAnchorRevisionReservation) Release() {
	if reservation == nil {
		return
	}

	reservation.release.Do(func() {
		controller := reservation.controller
		if controller == nil {
			return
		}

		controller.mutex.Lock()
		defer controller.mutex.Unlock()

		if reservation.cost.Revisions > controller.reserved.Revisions ||
			reservation.cost.Generations > controller.reserved.Generations {
			panic("FROST native signer anchor reservation accounting underflow")
		}
		controller.reserved.Revisions -= reservation.cost.Revisions
		controller.reserved.Generations -= reservation.cost.Generations
	})
}

// frostPreSignMaximumAnchorCapacityCost freezes the upper bound for one
// authorized transaction and rejects a workflow no certified restart window
// can cover. For every input:
//
//   - BuildTaprootTx is one request-taking call;
//   - every attempt lets each local seat call Open, Round1, Round2, and Abort;
//   - Aggregate is process-memoized to one call per attempt.
//
// Each of those calls can advance one service revision when its sweep prologue
// mutates otherwise-unrelated state. The proof-window cost is the amortized
// per-call generation bound times the anchored-call count plus the
// workflow-wide terminal allowance, not the hard per-operation bound times
// that count: no workflow can pay a repair advance on every one of its calls,
// because every repair advance consumes a pending operation that only an
// earlier failed persist creates. That makes the generation figure an
// amortized reservation with a bounded allowance rather than an exact upper
// bound - the constants above state precisely what it does not cover.
// Attempt derivation, package/evidence handling, and authorization guards do
// not persist Rust state.
//
// Both windows are finite, so the local seat count an operator can serve at a
// given batch size is finite too. With production parameters (21 inputs, the
// standard 20-deposit sweep plus its main UTXO, and 5 signing attempts) the
// ceiling is four local seats. This is a real operational limit rather than an
// accounting artifact: the windows bound how far a restarting node can still
// prove its own history back to the last offline-authorized floor, and a
// workflow admitted beyond them could not be recovered after a crash. A seat
// above the ceiling is excluded from every batch of that size on every one of
// its attempts, so the errors below name the ceiling explicitly - an operator
// that crosses it would otherwise have to infer it from a wallet that quietly
// stops reaching its signing threshold on full-size batches.
//
// The ceiling of four is an accepted exclusion, not a solved problem. Five
// seats in a hundred-seat wallet is an ordinary sortition result for a large
// staker, one seat over the ceiling, and such a node is refused every standard
// 20-deposit sweep: it tops out at 19 inputs, and a six-seat node at 16. Those
// seats are lost to the wallet's signing threshold on full-size batches for as
// long as the node holds them. Moving the ceiling requires enlarging the
// certified restart windows or lowering the signing-attempt limit, both of
// which are protocol-level changes to constants this function only reads, so
// this accounting cannot make the exclusion smaller on its own.
func frostPreSignMaximumAnchorCapacityCost(
	inputCount uint64,
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (frostNativeSignerAnchorCapacity, error) {
	cost, err := frostPreSignAnchoredWorkflowCost(
		inputCount,
		localSeatCount,
		maximumSigningAttempts,
	)
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, err
	}
	if cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents {
		recordFrostNativeSignerAnchorSeatCeilingRejection()
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign workflow over [%d] inputs with [%d] local seats "+
				"requires [%d] anchor revisions, exceeding the certified history "+
				"window [%d]; %s",
			inputCount,
			localSeatCount,
			cost.Revisions,
			FrostNativeSignerAnchorMaximumHistoryEvents,
			frostPreSignAdmissibleLocalSeatAdvice(
				inputCount,
				localSeatCount,
				maximumSigningAttempts,
			),
		)
	}
	if cost.Generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
		recordFrostNativeSignerAnchorSeatCeilingRejection()
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign workflow over [%d] inputs with [%d] local seats "+
				"requires [%d] signer generations, exceeding the certified proof "+
				"window [%d]; %s",
			inputCount,
			localSeatCount,
			cost.Generations,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
			frostPreSignAdmissibleLocalSeatAdvice(
				inputCount,
				localSeatCount,
				maximumSigningAttempts,
			),
		)
	}
	return cost, nil
}

// frostPreSignAdmissibleLocalSeatAdvice states the seat ceiling that produced a
// window rejection in operator-actionable terms, together with the largest
// batch this node's own seat count could still have signed. Every local seat
// above the ceiling is permanently excluded from batches of this size, and the
// wallet loses those seats toward its signing threshold, so this belongs in the
// error itself: the exclusion is otherwise visible only as a group that stops
// assembling a threshold on full-size batches, with no node reporting a cause.
// Both numbers are reported because both levers are the operator's - shed
// seats, or sign smaller batches - and neither is inferable from the other.
func frostPreSignAdmissibleLocalSeatAdvice(
	inputCount uint64,
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) string {
	admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		inputCount,
		maximumSigningAttempts,
	)
	if admissibleSeats == 0 {
		return fmt.Sprintf(
			"no local seat count can sign [%d] inputs within the certified "+
				"restart windows; the wallet cannot serve batches of this size "+
				"until those windows or the signing-attempt limit change",
			inputCount,
		)
	}
	admissibleInputs := frostPreSignMaximumAdmissibleInputCount(
		localSeatCount,
		maximumSigningAttempts,
	)
	if admissibleInputs == 0 {
		return fmt.Sprintf(
			"at most [%d] local seats can sign [%d] inputs within the certified "+
				"restart windows, and [%d] local seats can sign no batch size at "+
				"all, so every local seat above that ceiling is excluded from "+
				"every batch of this size and is lost to the wallet's signing "+
				"threshold",
			admissibleSeats,
			inputCount,
			localSeatCount,
		)
	}
	return fmt.Sprintf(
		"at most [%d] local seats can sign [%d] inputs within the certified "+
			"restart windows, and [%d] local seats can sign at most [%d] inputs, "+
			"so every local seat above that ceiling is excluded from every batch "+
			"of this size and is lost to the wallet's signing threshold",
		admissibleSeats,
		inputCount,
		localSeatCount,
		admissibleInputs,
	)
}

// frostPreSignMaximumAdmissibleInputCount reports the largest batch size this
// local seat count can still sign within both certified restart windows, or
// zero when no batch size can. Cost rises monotonically with the input count,
// so the first rejection ends the scan.
func frostPreSignMaximumAdmissibleInputCount(
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) uint64 {
	admissibleInputs := uint64(0)
	for inputs := uint64(1); inputs <=
		uint64(frostPreSignAuthorizationMaximumInputs); inputs++ {
		cost, err := frostPreSignAnchoredWorkflowCost(
			inputs,
			localSeatCount,
			maximumSigningAttempts,
		)
		if err != nil ||
			cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
			cost.Generations >
				FrostNativeSignerAnchorMaximumHistoryProofEntries {
			break
		}
		admissibleInputs = inputs
	}
	return admissibleInputs
}

// frostPreSignMaximumAdmissibleLocalSeatCount reports the largest local seat
// count whose complete worst-case workflow still fits both certified restart
// windows at this input count, or zero when no seat count does. Cost rises
// monotonically with the seat count, so the first rejection ends the scan.
func frostPreSignMaximumAdmissibleLocalSeatCount(
	inputCount uint64,
	maximumSigningAttempts uint64,
) uint64 {
	admissibleSeats := uint64(0)
	for seats := uint64(1); seats <=
		uint64(frostPreSignAuthorizationMaximumSeats); seats++ {
		cost, err := frostPreSignAnchoredWorkflowCost(
			inputCount,
			seats,
			maximumSigningAttempts,
		)
		if err != nil ||
			cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
			cost.Generations >
				FrostNativeSignerAnchorMaximumHistoryProofEntries {
			break
		}
		admissibleSeats = seats
	}
	return admissibleSeats
}

// frostPreSignAnchoredWorkflowCost is the window-independent arithmetic behind
// frostPreSignMaximumAnchorCapacityCost. It is kept separate so the seat
// ceiling can be scanned without re-entering the window diagnostics that report
// it.
func frostPreSignAnchoredWorkflowCost(
	inputCount uint64,
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (frostNativeSignerAnchorCapacity, error) {
	if inputCount == 0 ||
		inputCount > uint64(frostPreSignAuthorizationMaximumInputs) {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchor admission input count [%d] is outside [1,%d]",
			inputCount,
			frostPreSignAuthorizationMaximumInputs,
		)
	}
	if localSeatCount == 0 ||
		localSeatCount > uint64(frostPreSignAuthorizationMaximumSeats) {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchor admission local seat count [%d] is outside [1,%d]",
			localSeatCount,
			frostPreSignAuthorizationMaximumSeats,
		)
	}
	if maximumSigningAttempts == 0 {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchor admission signing-attempt limit is zero",
		)
	}

	maxUint64 := ^uint64(0)
	if localSeatCount > (maxUint64-1)/4 {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchored-call cost overflow",
		)
	}
	perAttemptCalls := 4*localSeatCount + 1
	if maximumSigningAttempts > (maxUint64-1)/perAttemptCalls {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchored-call cost overflow",
		)
	}
	perInputCalls := uint64(1) + maximumSigningAttempts*perAttemptCalls
	if inputCount > maxUint64/perInputCalls {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign anchored-call cost overflow",
		)
	}
	revisions := inputCount * perInputCalls
	if revisions > (maxUint64-
		frostNativeSignerTerminalGenerationAdvancesPerWorkflow)/
		frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign generation cost overflow",
		)
	}
	generations := revisions*
		frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall +
		frostNativeSignerTerminalGenerationAdvancesPerWorkflow

	return frostNativeSignerAnchorCapacity{
		Revisions:   revisions,
		Generations: generations,
	}, nil
}

// frostNativeSignerAnchorWorkloadRotationWarning reports whether this node
// should already be arranging an offline anchor rotation, measured against the
// work it can actually be asked to admit rather than against a flat number.
//
// localSeatCount is the largest number of local seats this node holds in any
// one wallet, because a pre-sign workflow is per wallet and reserves against
// that wallet's local member indexes. It is a parameter rather than something
// derived here because this file has no view of the node's sortition result;
// guessing it would be worse than requiring the caller that does know it to
// say so.
//
// The flat floor (FrostNativeSignerAnchorRotationWarningHeadroom, 256) is kept
// as a hard lower bound, and this condition is added to it rather than
// substituted for it, because the two guard different things. The flat floor
// guards the rotation-blocked refusal in reserve(): at or below it ALL new work
// is refused regardless of size, so a node that reaches it has already stopped
// signing outright. That makes it a useless warning threshold, because
// admission stops long before it is reached. A node holding four local seats -
// an ordinary sortition result, and the ceiling for the standard 21-input
// sweep - reserves 3615 generations for one maximum-size workflow, so its next
// sweep is refused with 3508 of the 4096 generations still unspent, roughly
// 3359 generations before the flat warning would fire. An operator alerting on
// the flat flag alone learns that the node stopped signing long after it
// stopped signing.
//
// Each window is compared against its own dimension's cost rather than
// comparing the smaller headroom against the generation cost. The generation
// cost is about twice the revision cost, so collapsing the two would raise the
// warning on the revision window for a shortfall that only the generation
// window has.
//
// The measuring stick is deliberately the largest ADMISSIBLE workflow, not
// unconditionally a 21-input batch. A node above the seat ceiling can never be
// admitted for 21 inputs at all, and its 21-input cost exceeds the whole
// 4096-entry window, so measuring it that way would latch this warning on for
// the life of the process; because the warning also forces the activation
// handshake's top-level health to false, that would take an otherwise healthy
// five-seat node out of service for a seat-ceiling exclusion no rotation can
// fix. frostPreSignMaximumAdmissibleInputCount reports what this seat count can
// actually be admitted for, so a five-seat node is measured against its own
// 19-input maximum.
//
// Native DKG and DKG retirement share the same admission controller but reserve
// two capacity units per local seat - orders of magnitude below a pre-sign
// workflow - so the pre-sign cost bounds them too and they need no separate
// term.
func frostNativeSignerAnchorWorkloadRotationWarning(
	revisionHeadroom uint64,
	generationHeadroom uint64,
	localSeatCount uint64,
) bool {
	if frostNativeSignerAnchorRotationWarning(
		minFrostNativeSignerAnchorHeadroom(
			revisionHeadroom,
			generationHeadroom,
		),
	) {
		return true
	}
	cost, admissible :=
		frostNativeSignerAnchorLargestAdmissibleWorkflowCost(localSeatCount)
	if !admissible {
		return false
	}
	return revisionHeadroom <= cost.Revisions ||
		generationHeadroom <= cost.Generations
}

// frostNativeSignerAnchorLargestAdmissibleWorkflowCost reports the worst-case
// capacity one pre-sign workflow can reserve on a node holding this many local
// seats in a single wallet, together with whether any workflow is admissible at
// all.
//
// It reports false, rather than a cost, for a seat count that no batch size can
// serve and for a seat count of zero. Both mean the same thing for this
// purpose: pre-sign admission is not what will consume the windows, so only the
// flat rotation floor is a meaningful warning threshold, and inventing a cost
// here would warn permanently on a node whose seats a rotation cannot restore.
func frostNativeSignerAnchorLargestAdmissibleWorkflowCost(
	localSeatCount uint64,
) (frostNativeSignerAnchorCapacity, bool) {
	admissibleInputs := frostPreSignMaximumAdmissibleInputCount(
		localSeatCount,
		signingAttemptsLimit,
	)
	if admissibleInputs == 0 {
		return frostNativeSignerAnchorCapacity{}, false
	}
	cost, err := frostPreSignAnchoredWorkflowCost(
		admissibleInputs,
		localSeatCount,
		signingAttemptsLimit,
	)
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, false
	}
	return cost, true
}

// FROST native signer anchor admission rejection counters.
//
// A node that stops admitting FROST work does so silently: the refusals reach
// the log wrapped inside a generic wallet-action failure, and nothing anchored
// is registered with clientinfo at all, so a monitoring system cannot tell a
// node that is refusing every signing request from one that simply has not been
// asked. These process-wide cumulative counters make each refusal cause
// countable on its own, because the four causes need four different responses:
//
//   - seatCeiling is a CONFIGURATION signal. It never clears on its own and no
//     rotation fixes it: this node holds more local seats than the certified
//     windows can serve at the requested batch size, and those seats are lost to
//     the wallet's signing threshold until the operator sheds them. Any non-zero
//     value should alert.
//   - poisoned is a TERMINAL signal. The state anchor has latched a permanent
//     failure and only a process restart clears it. Any non-zero value should
//     alert.
//   - unreservedHeadroom is the ROTATION-DUE signal. It is the first refusal a
//     healthy, correctly configured node produces, and it arrives while most of
//     both windows are still unspent.
//   - rotationFloor is the ROTATION-OVERDUE signal. All new work is already
//     being refused regardless of size.
//
// They follow the roast_interactive_signing_metrics.go pattern, are emitted in
// every build, and stay at zero until an admission is actually refused - so
// they are inert by default and registering them activates no behavior.
var (
	frostNativeSignerAnchorSeatCeilingRejections        atomic.Uint64
	frostNativeSignerAnchorUnreservedHeadroomRejections atomic.Uint64
	frostNativeSignerAnchorRotationFloorRejections      atomic.Uint64
	frostNativeSignerAnchorPoisonedRejections           atomic.Uint64
)

// frostNativeSignerAnchorAdmissionMetricsApplication is the clientinfo
// application-label prefix; the registry concatenates it with each per-source
// name, so the final labels look like
// "frost_native_signer_anchor_admission_poisoned_rejected_total".
const frostNativeSignerAnchorAdmissionMetricsApplication = "frost_native_signer_anchor_admission"

const (
	frostNativeSignerAnchorSeatCeilingMetricName        = "seat_ceiling_rejected_total"
	frostNativeSignerAnchorUnreservedHeadroomMetricName = "unreserved_headroom_rejected_total"
	frostNativeSignerAnchorRotationFloorMetricName      = "rotation_floor_rejected_total"
	frostNativeSignerAnchorPoisonedMetricName           = "poisoned_rejected_total"
)

// RegisterFrostNativeSignerAnchorAdmissionMetrics registers the cumulative
// anchor-admission rejection counters with the supplied clientinfo registry.
// The node's startup sequence calls it alongside
// frostsigning.RegisterInteractiveSigningMetrics so the counters appear in the
// Prometheus scrape; without that call they increment internally and never
// reach an operator. A nil registry is a no-op.
func RegisterFrostNativeSignerAnchorAdmissionMetrics(
	registry *clientinfo.Registry,
) {
	if registry == nil {
		return
	}
	registry.ObserveApplicationSource(
		frostNativeSignerAnchorAdmissionMetricsApplication,
		map[string]clientinfo.Source{
			frostNativeSignerAnchorSeatCeilingMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorSeatCeilingRejections.Load(),
				)
			},
			frostNativeSignerAnchorUnreservedHeadroomMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorUnreservedHeadroomRejections.Load(),
				)
			},
			frostNativeSignerAnchorRotationFloorMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorRotationFloorRejections.Load(),
				)
			},
			frostNativeSignerAnchorPoisonedMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorPoisonedRejections.Load(),
				)
			},
		},
	)
}

// recordFrostNativeSignerAnchorSeatCeilingRejection marks one workflow refused
// because its worst case cannot fit a certified restart window at this node's
// local seat count.
func recordFrostNativeSignerAnchorSeatCeilingRejection() {
	frostNativeSignerAnchorSeatCeilingRejections.Add(1)
}

// recordFrostNativeSignerAnchorUnreservedHeadroomRejection marks one workflow
// refused because the unreserved part of a certified window cannot cover it.
func recordFrostNativeSignerAnchorUnreservedHeadroomRejection() {
	frostNativeSignerAnchorUnreservedHeadroomRejections.Add(1)
}

// recordFrostNativeSignerAnchorRotationFloorRejection marks one workflow
// refused because a certified window has fallen to the rotation floor, where
// all new work is refused regardless of size.
func recordFrostNativeSignerAnchorRotationFloorRejection() {
	frostNativeSignerAnchorRotationFloorRejections.Add(1)
}

// recordFrostNativeSignerAnchorPoisonedRejection marks one workflow refused
// because the native signer state anchor is terminally poisoned.
func recordFrostNativeSignerAnchorPoisonedRejection() {
	frostNativeSignerAnchorPoisonedRejections.Add(1)
}

// resetFrostNativeSignerAnchorAdmissionMetricsForTest clears the cumulative
// counters. Exposed only for the package's own tests; not a production helper.
func resetFrostNativeSignerAnchorAdmissionMetricsForTest() {
	frostNativeSignerAnchorSeatCeilingRejections.Store(0)
	frostNativeSignerAnchorUnreservedHeadroomRejections.Store(0)
	frostNativeSignerAnchorRotationFloorRejections.Store(0)
	frostNativeSignerAnchorPoisonedRejections.Store(0)
}
