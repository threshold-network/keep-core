package tbtc

import (
	"context"
	"fmt"
	"sync"
)

const (
	// One request-taking interactive signer call can durably advance the Rust
	// generation up to three times: the expiry-sweep prologue takes one
	// snapshot and, when a prior write left a repair pending, a second for the
	// retirement that repair unblocked, then the endpoint persists its own
	// mutation. Matching pending operations are covered by the sweep;
	// nonmatching operations remain pending without another write. The
	// process output barrier still advances the remote anchor revision only
	// once for the call's final checkpoint.
	//
	// This is a hard per-operation bound, not an accounting convenience: node
	// startup installs it as the output barrier's MaximumStateGenerationAdvance
	// PerOperation, and one call advancing further poisons the process. It must
	// stay at the signer's own worst case for one in-flight request, which
	// TBTC_SIGNER_STATE_WITNESS_ROTATION_TERMINAL_RECORD_RESERVATION states as
	// three snapshots (six journal records, two per snapshot).
	frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall uint64 = 3

	// A workflow reservation cannot simply multiply the per-operation bound by
	// its call count, because the third advance of any single call is always a
	// repair and no workflow can pay it on every call. Both extra writes - the
	// sweep's second snapshot and the Round2/Aggregate marker re-persist - are
	// reached only when an earlier snapshot failed and left a pending
	// persistence operation, and the marker re-persist excludes that call's own
	// mutation because flushing the marker makes the replay gate reject the
	// retry. A snapshot that failed before replacing the state image, or that
	// replaced it and left its witness prepared and uncommitted, advanced no
	// generation at all, so its call spent one of the two advances reserved
	// here and the later repair lands the one it did not. The steady-state cost
	// is therefore the shared sweep prologue plus the endpoint's own mutation.
	frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall uint64 = 2

	// Two interleavings outrun the amortized bound, and neither recurs. A
	// snapshot that committed its witness and then failed its post-commit
	// revalidation both spends an advance and leaves a repair pending; it
	// cannot happen twice in a workflow because replace_state revalidates the
	// store before replacing anything, so every later anchored call fails
	// closed without advancing a generation. A witness prepared before
	// admission and reconciled inside the workflow lands one further advance;
	// prepare_witness refuses to prepare a second, so at most one is ever
	// outstanding. This workflow-wide allowance covers both.
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
		return nil, fmt.Errorf(
			"%s is blocked with revision/generation headroom [%d/%d]; "+
				"offline anchor rotation is required before admitting new work",
			workflow,
			headroom.Revisions,
			headroom.Generations,
		)
	}
	if controller.reserved.Revisions > headroom.Revisions ||
		cost.Revisions >
			headroom.Revisions-controller.reserved.Revisions {
		return nil, fmt.Errorf(
			"%s requires [%d] anchor revisions but only [%d] are unreserved",
			workflow,
			cost.Revisions,
			headroom.Revisions-controller.reserved.Revisions,
		)
	}
	if controller.reserved.Generations > headroom.Generations ||
		cost.Generations >
			headroom.Generations-controller.reserved.Generations {
		return nil, fmt.Errorf(
			"%s requires [%d] signer generations but only [%d] are unreserved",
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
// that count: no workflow can pay a repair advance on every one of its calls.
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
				maximumSigningAttempts,
			),
		)
	}
	if cost.Generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
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
				maximumSigningAttempts,
			),
		)
	}
	return cost, nil
}

// frostPreSignAdmissibleLocalSeatAdvice states the seat ceiling that produced a
// window rejection in operator-actionable terms. Every local seat above the
// ceiling is permanently excluded from batches of this size, and the wallet
// loses those seats toward its signing threshold, so this belongs in the error
// itself: the exclusion is otherwise visible only as a group that stops
// assembling a threshold on full-size batches, with no node reporting a cause.
func frostPreSignAdmissibleLocalSeatAdvice(
	inputCount uint64,
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
	return fmt.Sprintf(
		"at most [%d] local seats can sign [%d] inputs within the certified "+
			"restart windows, so every local seat above that ceiling is excluded "+
			"from every batch of this size and is lost to the wallet's signing "+
			"threshold",
		admissibleSeats,
		inputCount,
	)
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
