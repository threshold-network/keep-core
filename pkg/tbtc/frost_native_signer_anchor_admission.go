package tbtc

import (
	"context"
	"fmt"
	"sync"
)

const (
	// One request-taking interactive signer call can durably advance the Rust
	// generation up to three times: reconcile one previously prepared witness
	// snapshot, persist the sweep/repair snapshot (the expiry sweep and any
	// protected retirement share it), then persist the endpoint's own
	// mutation. Matching pending operations are covered by the sweep;
	// nonmatching operations remain pending without another write. The
	// process output barrier still advances the remote anchor revision only
	// once for the call's final checkpoint.
	frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall uint64 = 3
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

// reserveDKG accounts for one request-taking persistence call per local seat.
// DKG has no interactive sweep prologue, so each call advances at most one
// service revision and one Rust generation.
func (controller *frostNativeSignerAnchorAdmissionController) reserveDKG(
	ctx context.Context,
	localSeatCount uint64,
) (*frostNativeSignerAnchorRevisionReservation, error) {
	if localSeatCount == 0 {
		return nil, fmt.Errorf(
			"FROST native DKG anchor admission controls no local seats",
		)
	}
	cost := frostNativeSignerAnchorCapacity{
		Revisions:   localSeatCount,
		Generations: localSeatCount,
	}

	return controller.reserve(
		"FROST native DKG",
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
// authorized transaction. For every input:
//
//   - BuildTaprootTx is one request-taking call;
//   - every attempt lets each local seat call Open, Round1, Round2, and Abort;
//   - Aggregate is process-memoized to one call per attempt.
//
// Each of those calls can advance one service revision when its sweep prologue
// mutates otherwise-unrelated state. Each interactive call can advance up to
// three Rust generations (prepared-witness reconciliation, one sweep/repair
// write, and its own write), so the proof-window cost is three times the
// anchored-call count. Attempt derivation, package/evidence handling, and
// authorization guards do not persist Rust state.
func frostPreSignMaximumAnchorCapacityCost(
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
	if revisions > maxUint64/
		frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign generation cost overflow",
		)
	}
	generations :=
		revisions * frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall

	if revisions > FrostNativeSignerAnchorMaximumHistoryEvents {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign workflow requires [%d] anchor revisions, exceeding "+
				"the certified history window [%d]",
			revisions,
			FrostNativeSignerAnchorMaximumHistoryEvents,
		)
	}
	if generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
		return frostNativeSignerAnchorCapacity{}, fmt.Errorf(
			"FROST pre-sign workflow requires [%d] signer generations, exceeding "+
				"the certified proof window [%d]",
			generations,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
		)
	}

	return frostNativeSignerAnchorCapacity{
		Revisions:   revisions,
		Generations: generations,
	}, nil
}
