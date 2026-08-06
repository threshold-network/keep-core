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

	// A reservation cannot multiply the per-operation bound by its call count:
	// at production parameters that reserves more of the certified proof window
	// than exists for large local seat counts - charging three per call puts one
	// input's worth at 60*seats+21, which passes the 4096-entry window at 68
	// local seats. What actually bounds the consumption is the witness journal -
	// generations advanced equals witness COMMITs, and every COMMIT belongs
	// either to a persist that reached its rename or to the single witness that
	// may have been carried into the admitted work already prepared. The cost is
	// therefore the number of persists that reach a rename, not the call count
	// times the per-call ceiling.
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
	// the per-input allowance below is what absorbs them.
	frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall uint64 = 2

	// The allowance added on top of the amortized per-call cost, once per
	// admitted input. It covers one witness prepared before that input's
	// admission and reconciled inside it - prepare_witness refuses to prepare a
	// second, so at most one is ever outstanding - plus roughly two further
	// post-rename persist failures, each of which overruns the amortized bound
	// by about one generation.
	//
	// It is charged per input rather than once per batch because one input is
	// the unit admission reserves: a batch signs its inputs strictly
	// sequentially and each input takes and releases its own reservation, so
	// each of them can meet a carried-in witness at its own boundary. A
	// 21-input sweep therefore carries 21 of these allowances over its life
	// rather than one, which is more slack than the batch-wide charge gave, not
	// less.
	//
	// It is an allowance, not a proof, and the reservation it completes is not
	// exact. Nothing in the signer bounds how many times a failing store can
	// fail a persist after its rename, so an input that suffers more of them
	// than this covers consumes more of the proof window than it reserved. The
	// allowance is kept small deliberately: the slack that really absorbs
	// faults is that most calls sweep nothing and spend one advance rather
	// than two, and a node failing this many persists after rename has a
	// failing store, where blocking is the correct outcome.
	//
	// The residual is fail-closed availability, never authority. The
	// reservation only promises that the certified proof window can cover the
	// admitted input; overrunning it means that window can run out part way
	// through one. What an operator then sees is request-taking calls blocked
	// with "the certified signer-generation window cannot cover its maximum
	// advance; offline anchor rotation is required" and new pre-sign work
	// rejected for want of unreserved headroom - on a node that was already
	// failing every one of those persists. No share is released, no replay
	// gate weakens, and the recovery is the offline anchor rotation that
	// window exhaustion always requires.
	frostNativeSignerTerminalGenerationAdvancesPerAdmittedInput uint64 = 3
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
// Reservations retain their full worst-case cost until the admitted unit of
// work exits. Because current headroom already reflects consumed work, this
// intentionally double-counts in-flight mutations for later admissions and is
// conservative.
//
// The admitted unit for pre-sign signing is ONE transaction input, not a whole
// batch. A batch signs its inputs strictly sequentially, so no batch ever needs
// more than one input's worth of unconsumed window at any instant; charging the
// whole batch up front reserved 21 times what the peak demand is and put a hard
// four-local-seat ceiling on an operator's ability to sign a full deposit sweep
// at all. Nothing about that reservation is given up by charging per input: the
// unit still covers every signing attempt for its input, so an admitted input
// keeps its full retry budget, and the reservation is still taken before any
// anchored call for that input runs.
//
// What is given up is the whole-batch promise. Under a per-input reservation a
// batch can be admitted for input 0 and refused at input 7 because the window
// ran out under it. That is a clean, fail-closed refusal - the input's
// reservation is released, no share is produced, and the wallet action fails
// with the rotation remedy named - and it replaces a guarantee that was
// unpurchasable anyway: the whole-batch charge did not stop the window running
// out mid batch, it only stopped most operators from starting one.
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

// reservePreSign reserves one input's complete upper bound. A pre-sign workflow
// calls it once per input plus once up front, and every call charges the same
// one-input cost:
//
//   - once inside authorize(), before the authorization is relayed on chain, so
//     the relay is still gated by a node that has the capacity to act on it. It
//     is released before the loop below starts, because the two are the same
//     size and holding them together would charge two inputs for one input's
//     work; and
//   - once for each input of the sequential signing loop, released as soon as
//     that input is signed or has failed.
//
// inputCount is the size of the batch this admission belongs to. It bounds
// nothing in the cost - one input costs the same whether it is the only input
// or one of twenty-one - and is taken so a batch outside the protocol's legal
// range is refused here as well as at proposal validation.
//
// The readiness snapshot was independently reconciled twice, but may be stale
// by the time this mutex is acquired. The authenticated current tip is
// therefore read under the admission lock and the smaller headroom in each
// dimension is authoritative.
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
	// Raw-window exhaustion and reservation contention need different remedies.
	// If cost exceeds raw headroom, no in-flight workflow release can make it
	// fit: neither window refills on its own, so offline rotation is required.
	// If raw headroom can cover cost but the conservative in-flight reservations
	// leave too little unreserved, the refusal is temporary and must not tell an
	// operator to rotate. The reservation is released when its workflow exits
	// and commonly spends less than its worst-case charge.
	//
	// Since admission reserves one input at a time, this refusal can also land
	// part way through a batch whose authorization is already relayed and
	// finalized on chain. That is a deliberate trade - the batch-wide charge
	// that would have caught it earlier is what excluded most operators from
	// signing at all - and it is made visible by its own counter rather than
	// hidden behind this message.
	unreservedRevisions := headroom.Revisions
	if controller.reserved.Revisions >= headroom.Revisions {
		unreservedRevisions = 0
	} else {
		unreservedRevisions -= controller.reserved.Revisions
	}
	unreservedGenerations := headroom.Generations
	if controller.reserved.Generations >= headroom.Generations {
		unreservedGenerations = 0
	} else {
		unreservedGenerations -= controller.reserved.Generations
	}
	// Exhaustion in either raw window takes precedence over temporary
	// contention in the other: releasing reservations cannot make a workflow
	// fit a dimension whose live headroom is already too small.
	if cost.Revisions > headroom.Revisions {
		recordFrostNativeSignerAnchorUnreservedHeadroomRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] anchor revisions but only [%d] are unreserved; "+
				"the certified history window does not refill on its own, so "+
				"offline anchor rotation is required before this workflow can "+
				"be admitted",
			workflow,
			cost.Revisions,
			unreservedRevisions,
		)
	}
	if cost.Generations > headroom.Generations {
		recordFrostNativeSignerAnchorUnreservedHeadroomRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] signer generations but only [%d] are unreserved; "+
				"the certified proof window does not refill on its own, so "+
				"offline anchor rotation is required before this workflow can "+
				"be admitted",
			workflow,
			cost.Generations,
			unreservedGenerations,
		)
	}
	if cost.Revisions > unreservedRevisions {
		recordFrostNativeSignerAnchorReservationContentionRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] anchor revisions but only [%d] are currently "+
				"unreserved because in-flight workflows hold temporary "+
				"reservations; retry after those workflows finish",
			workflow,
			cost.Revisions,
			unreservedRevisions,
		)
	}
	if cost.Generations > unreservedGenerations {
		recordFrostNativeSignerAnchorReservationContentionRejection()
		return nil, fmt.Errorf(
			"%s requires [%d] signer generations but only [%d] are currently "+
				"unreserved because in-flight workflows hold temporary "+
				"reservations; retry after those workflows finish",
			workflow,
			cost.Generations,
			unreservedGenerations,
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

// frostPreSignMaximumAnchorCapacityCost freezes the upper bound for ONE
// admitted transaction input and rejects work no certified restart window can
// cover. For that one input:
//
//   - BuildTaprootTx is one request-taking call;
//   - every attempt lets each local seat call Open, Round1, Round2, and Abort;
//   - Aggregate is process-memoized to one call per attempt.
//
// Each of those calls can advance one service revision when its sweep prologue
// mutates otherwise-unrelated state. The proof-window cost is the amortized
// per-call generation bound times the anchored-call count plus the per-input
// terminal allowance, not the hard per-operation bound times that count: no
// input can pay a repair advance on every one of its calls, because every
// repair advance consumes a pending operation that only an earlier failed
// persist creates. That makes the generation figure an amortized reservation
// with a bounded allowance rather than an exact upper bound - the constants
// above state precisely what it does not cover. Attempt derivation,
// package/evidence handling, and authorization guards do not persist Rust
// state.
//
// inputCount is validated but does not scale the cost. A batch's inputs are
// signed strictly sequentially and each one holds its own reservation for its
// own lifetime, so a batch never needs more than one input's worth of
// unconsumed window at any instant. Charging the whole batch up front was the
// arithmetic error this replaces: at production parameters (21 inputs, 5
// signing attempts) it admitted at most four local seats. Signers are
// equal-weighted, so a hundred-seat wallet shared by around twenty operators
// gives every operator about five - meaning that ceiling excluded every
// operator, not merely the larger ones, and left formed wallets unable to
// sweep the deposits they had already received.
//
// A ceiling still exists in principle, because both windows are finite. One
// input costs 20*seats+6 revisions and 40*seats+15 generations, so the
// 4096-entry proof window is the binding one and the last seat count it covers
// is 102. A wallet has only frostPreSignAuthorizationMaximumSeats (100) seats to
// give in total, so no protocol-legal seat count is excluded and the ceiling is
// unreachable rather than merely large. The rejections below are kept, and
// still name the ceiling, because they are the guard that makes a future change
// to the certified windows, the signing-attempt limit or the seat cap visible
// instead of silent.
func frostPreSignMaximumAnchorCapacityCost(
	inputCount uint64,
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (frostNativeSignerAnchorCapacity, error) {
	if err := validateFrostPreSignAdmissionInputCount(inputCount); err != nil {
		return frostNativeSignerAnchorCapacity{}, err
	}
	cost, err := frostPreSignAnchoredInputCost(
		localSeatCount,
		maximumSigningAttempts,
	)
	if err != nil {
		return frostNativeSignerAnchorCapacity{}, err
	}
	if cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents {
		recordFrostNativeSignerAnchorSeatCeilingRejection()
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"one FROST pre-sign input with [%d] local seats requires [%d] "+
				"anchor revisions, exceeding the certified history window [%d]; %s",
			localSeatCount,
			cost.Revisions,
			FrostNativeSignerAnchorMaximumHistoryEvents,
			frostPreSignAdmissibleLocalSeatAdvice(
				localSeatCount,
				maximumSigningAttempts,
			),
		)
	}
	if cost.Generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
		recordFrostNativeSignerAnchorSeatCeilingRejection()
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"one FROST pre-sign input with [%d] local seats requires [%d] "+
				"signer generations, exceeding the certified proof window [%d]; %s",
			localSeatCount,
			cost.Generations,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
			frostPreSignAdmissibleLocalSeatAdvice(
				localSeatCount,
				maximumSigningAttempts,
			),
		)
	}
	return cost, nil
}

// validateFrostPreSignAdmissionInputCount refuses a batch outside the
// protocol's legal size. Batch size no longer scales what admission reserves,
// so this is a legality check rather than a cost input; it is kept here as well
// as at proposal validation so a batch that never passed a proposal check
// cannot reach the native signer through this path.
func validateFrostPreSignAdmissionInputCount(inputCount uint64) error {
	if inputCount == 0 ||
		inputCount > uint64(frostPreSignAuthorizationMaximumInputs) {
		return fmt.Errorf(
			"FROST pre-sign anchor admission input count [%d] is outside [1,%d]",
			inputCount,
			frostPreSignAuthorizationMaximumInputs,
		)
	}
	return nil
}

// frostPreSignAdmissibleLocalSeatAdvice states the seat ceiling that produced a
// window rejection in operator-actionable terms. Every local seat above the
// ceiling is excluded from every batch, whatever its size, and the wallet loses
// those seats toward its signing threshold, so this belongs in the error
// itself: the exclusion is otherwise visible only as a group that stops
// assembling a threshold, with no node reporting a cause.
//
// Only one lever is named now. Signing a smaller batch was the second lever
// while the reservation scaled with the batch; it no longer helps at all,
// because one input costs the same in a one-input batch as in a full sweep.
// Offering it would send an operator to a remedy that cannot work.
func frostPreSignAdmissibleLocalSeatAdvice(
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) string {
	admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		maximumSigningAttempts,
	)
	if admissibleSeats == 0 {
		return "no local seat count can sign a single pre-sign input within " +
			"the certified restart windows; this wallet can serve no batch of " +
			"any size until those windows or the signing-attempt limit change"
	}
	return fmt.Sprintf(
		"at most [%d] local seats can sign one pre-sign input within the "+
			"certified restart windows and this node holds [%d], so it is "+
			"excluded from every batch of every size and all of its seats are "+
			"lost to the wallet's signing threshold; batch size is not a lever "+
			"here - one input costs the same in a one-input batch as in a full "+
			"sweep - so the only remedy is to shed seats down to [%d]",
		admissibleSeats,
		localSeatCount,
		admissibleSeats,
	)
}

// frostPreSignMaximumAdmissibleLocalSeatCount reports the largest local seat
// count whose complete worst-case single-input reservation still fits both
// certified restart windows, or zero when no seat count does. Cost rises
// monotonically with the seat count, so the first rejection ends the scan.
//
// It takes no input count: batch size does not enter the reservation any more,
// so the answer is the same for a one-input batch and for a full sweep. The
// scan stops at frostPreSignAuthorizationMaximumSeats because that is the most
// seats a wallet has to give, and under the current constants every one of them
// fits - a hundred-seat holder reserves 4015 of the 4096-entry proof window for
// one input - so this returns the cap itself and the ceiling excludes nobody.
func frostPreSignMaximumAdmissibleLocalSeatCount(
	maximumSigningAttempts uint64,
) uint64 {
	admissibleSeats := uint64(0)
	for seats := uint64(1); seats <=
		uint64(frostPreSignAuthorizationMaximumSeats); seats++ {
		cost, err := frostPreSignAnchoredInputCost(
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

// frostPreSignAnchoredInputCost is the window-independent arithmetic behind
// frostPreSignMaximumAnchorCapacityCost, for exactly one transaction input. It
// is kept separate so the seat ceiling can be scanned without re-entering the
// window diagnostics that report it.
func frostPreSignAnchoredInputCost(
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (frostNativeSignerAnchorCapacity, error) {
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
	revisions := uint64(1) + maximumSigningAttempts*perAttemptCalls
	if revisions > (maxUint64-
		frostNativeSignerTerminalGenerationAdvancesPerAdmittedInput)/
		frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign generation cost overflow",
		)
	}
	generations := revisions*
		frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall +
		frostNativeSignerTerminalGenerationAdvancesPerAdmittedInput

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
// one wallet, because a pre-sign reservation is per wallet and reserves against
// that wallet's local member indexes. It is a parameter rather than something
// derived here because this file has no view of the node's sortition result;
// guessing it would be worse than requiring the caller that does know it to
// say so.
//
// The flat floor (FrostNativeSignerAnchorRotationWarningHeadroom, 256) is kept
// as a hard lower bound, and this condition is added to it rather than
// substituted for it, because the two guard different things. The flat floor
// guards the rotation-blocked refusal in reserve(): at or below it ALL new work
// is refused regardless of size. This term guards the earlier moment at which
// this particular node stops being able to admit an input of its own.
//
// Which of the two fires first now depends on the seat count, and that is the
// honest answer rather than a weakness. One input costs 40*seats+15
// generations, so a node holding six or fewer local seats needs less than the
// flat floor's 256 and the flat floor is genuinely the binding warning for it;
// a node holding fifty reserves 2015 and is warned about 1759 generations
// earlier than the flat floor would. While the reservation covered a whole
// batch this term was doing far heavier lifting - a four-seat node then
// reserved 3615 generations at once and stopped signing 3359 generations before
// the flat floor - and that gap is exactly what reserving per input removed.
//
// Each window is compared against its own dimension's cost rather than
// comparing the smaller headroom against the generation cost. The generation
// cost is about twice the revision cost, so collapsing the two would raise the
// warning on the revision window for a shortfall that only the generation
// window has.
//
// Native DKG and DKG retirement share the same admission controller but reserve
// two capacity units per local seat - well below one pre-sign input - so the
// pre-sign cost bounds them too and they need no separate term.
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
// capacity one pre-sign admission can reserve on a node holding this many local
// seats in a single wallet, together with whether anything is admissible at
// all. That is one input's cost: admission's unit is an input, and a batch
// never holds more than one input's reservation at a time.
//
// It reports false, rather than a cost, for a seat count no single input can be
// admitted for and for a seat count of zero. Both mean the same thing for this
// purpose: pre-sign admission is not what will consume the windows, so only the
// flat rotation floor is a meaningful warning threshold, and inventing a cost
// here would warn permanently on a node whose seats a rotation cannot restore.
func frostNativeSignerAnchorLargestAdmissibleWorkflowCost(
	localSeatCount uint64,
) (frostNativeSignerAnchorCapacity, bool) {
	if localSeatCount == 0 {
		return frostNativeSignerAnchorCapacity{}, false
	}
	cost, err := frostPreSignAnchoredInputCost(
		localSeatCount,
		signingAttemptsLimit,
	)
	if err != nil ||
		cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
		cost.Generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
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
// countable on its own, because the six causes need six different responses:
//
//   - seatCeiling is a CONFIGURATION signal. It never clears on its own and no
//     rotation fixes it: this node holds more local seats than the certified
//     windows can serve for a single transaction input, and those seats are lost
//     to the wallet's signing threshold until the operator sheds them. Any
//     non-zero value should alert. Under the current constants it is
//     unreachable - a wallet's whole hundred-seat set costs 4015 of the
//     4096-entry proof window for one input - so a non-zero value means a
//     protocol constant moved, which is exactly why the counter is kept.
//   - poisoned is a TERMINAL signal. The state anchor has latched a permanent
//     failure and only a process restart clears it. Any non-zero value should
//     alert.
//   - reservationContention is a TEMPORARY signal. Raw certified headroom can
//     cover the workflow, but other in-flight workflows currently hold enough
//     conservative worst-case capacity to prevent another admission. Their
//     reservations release on exit, so this is a retry signal, not a rotation
//     signal.
//   - unreservedHeadroom is the ROTATION-DUE signal. It is the first refusal a
//     healthy, correctly configured node produces once raw headroom cannot
//     cover the workflow, and it can arrive while most of one window remains
//     unspent.
//   - rotationFloor is the ROTATION-OVERDUE signal. All new work is already
//     being refused regardless of size.
//   - preSignInput is the WORK-LOST signal, and it is the one that costs money.
//     A batch is admitted one input at a time, so a batch whose authorization
//     has already been relayed and finalized on chain can still be refused at
//     input 7 of 21 when the window runs out under it. The gas is spent, the
//     wallet action fails, and the underlying cause is counted by
//     reservationContention, unreservedHeadroom or rotationFloor alongside it -
//     this counter is what separates "refused before we spent anything" from
//     "refused after we did". It counts refusals of the per-input reservation,
//     including the first input's, because at that point the relay has already
//     happened either way.
//
// They follow the roast_interactive_signing_metrics.go pattern, are emitted in
// every build, and stay at zero until an admission is actually refused - so
// they are inert by default and registering them activates no behavior.
var (
	frostNativeSignerAnchorSeatCeilingRejections           atomic.Uint64
	frostNativeSignerAnchorReservationContentionRejections atomic.Uint64
	frostNativeSignerAnchorUnreservedHeadroomRejections    atomic.Uint64
	frostNativeSignerAnchorRotationFloorRejections         atomic.Uint64
	frostNativeSignerAnchorPoisonedRejections              atomic.Uint64
	frostNativeSignerAnchorPreSignInputRejections          atomic.Uint64
)

// frostNativeSignerAnchorAdmissionMetricsApplication is the clientinfo
// application-label prefix; the registry concatenates it with each per-source
// name, so the final labels look like
// "frost_native_signer_anchor_admission_poisoned_rejected_total".
const frostNativeSignerAnchorAdmissionMetricsApplication = "frost_native_signer_anchor_admission"

const (
	frostNativeSignerAnchorSeatCeilingMetricName           = "seat_ceiling_rejected_total"
	frostNativeSignerAnchorReservationContentionMetricName = "reservation_contention_rejected_total"
	frostNativeSignerAnchorUnreservedHeadroomMetricName    = "unreserved_headroom_rejected_total"
	frostNativeSignerAnchorRotationFloorMetricName         = "rotation_floor_rejected_total"
	frostNativeSignerAnchorPoisonedMetricName              = "poisoned_rejected_total"
	frostNativeSignerAnchorPreSignInputMetricName          = "pre_sign_input_rejected_total"
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
			frostNativeSignerAnchorReservationContentionMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorReservationContentionRejections.Load(),
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
			frostNativeSignerAnchorPreSignInputMetricName: func() float64 {
				return float64(
					frostNativeSignerAnchorPreSignInputRejections.Load(),
				)
			},
		},
	)
}

// recordFrostNativeSignerAnchorSeatCeilingRejection marks one admission refused
// because one input's worst case cannot fit a certified restart window at this
// node's local seat count.
func recordFrostNativeSignerAnchorSeatCeilingRejection() {
	frostNativeSignerAnchorSeatCeilingRejections.Add(1)
}

// recordFrostNativeSignerAnchorReservationContentionRejection marks one
// workflow refused only because in-flight workflows temporarily hold enough
// worst-case capacity to prevent another admission.
func recordFrostNativeSignerAnchorReservationContentionRejection() {
	frostNativeSignerAnchorReservationContentionRejections.Add(1)
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

// recordFrostNativeSignerAnchorPreSignInputRejection marks one input of an
// already-relayed pre-sign batch refused for want of anchor capacity. The cause
// is counted separately by whichever reserve() refusal produced it.
func recordFrostNativeSignerAnchorPreSignInputRejection() {
	frostNativeSignerAnchorPreSignInputRejections.Add(1)
}

// resetFrostNativeSignerAnchorAdmissionMetricsForTest clears the cumulative
// counters. Exposed only for the package's own tests; not a production helper.
func resetFrostNativeSignerAnchorAdmissionMetricsForTest() {
	frostNativeSignerAnchorSeatCeilingRejections.Store(0)
	frostNativeSignerAnchorReservationContentionRejections.Store(0)
	frostNativeSignerAnchorUnreservedHeadroomRejections.Store(0)
	frostNativeSignerAnchorRotationFloorRejections.Store(0)
	frostNativeSignerAnchorPoisonedRejections.Store(0)
	frostNativeSignerAnchorPreSignInputRejections.Store(0)
}
