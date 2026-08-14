package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/registry"
)

func TestHandleFrostDKGStartedReleasesDeduplicationKeyAfterFailure(t *testing.T) {
	localChain := Connect()
	node := &node{chain: localChain}
	event := &FrostDKGStartedEvent{Seed: big.NewInt(100), BlockNumber: 500}
	frostChain := &transientFrostDKGStartedChain{event: event}
	deduplicator := newDeduplicator()

	// Transient state, past-log, and membership failures must each release the
	// key so the same event can advance on the next replay.
	for i := 0; i < 4; i++ {
		handleFrostDKGStarted(
			context.Background(),
			node,
			frostChain,
			deduplicator,
			event,
			false,
		)
	}

	if frostChain.stateCalls != 4 ||
		frostChain.pastEventsCalls != 3 ||
		frostChain.selectionCalls != 2 {
		t.Fatalf(
			"unexpected retry calls: state [%d], past events [%d], selection [%d]",
			frostChain.stateCalls,
			frostChain.pastEventsCalls,
			frostChain.selectionCalls,
		)
	}

	// The successful replay determined this operator is not a member, which is
	// terminal local handling; further duplicates must stay suppressed.
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		event,
		false,
	)
	if frostChain.stateCalls != 4 {
		t.Fatalf("completed event was processed again")
	}
}

func TestHandleFrostDKGStartedRekeysToLatestSeedAlreadyInProgress(
	t *testing.T,
) {
	localChain := Connect()
	node := &node{chain: localChain}
	staleEvent := &FrostDKGStartedEvent{Seed: big.NewInt(100), BlockNumber: 500}
	latestEvent := &FrostDKGStartedEvent{Seed: big.NewInt(200), BlockNumber: 501}
	frostChain := newRekeyingFrostDKGStartedChain(
		staleEvent,
		latestEvent,
	)
	frostChain.blockFirstState = true
	deduplicator := newDeduplicator()

	latestDone := make(chan struct{})
	go func() {
		defer close(latestDone)
		handleFrostDKGStarted(
			context.Background(),
			node,
			frostChain,
			deduplicator,
			latestEvent,
			false,
		)
	}()

	select {
	case <-frostChain.firstStateStarted:
	case <-time.After(time.Second):
		t.Fatal("latest-seed handler did not acquire its lease")
	}

	// The stale handler resolves the latest event while that event's handler
	// owns its lease. It must complete the stale lease and must not start a
	// second execution for the latest seed.
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		staleEvent,
		false,
	)

	stateCalls, pastEventsCalls, selectionCalls := frostChain.calls()
	if stateCalls != 2 || pastEventsCalls != 1 || selectionCalls != 0 {
		t.Fatalf(
			"stale handler duplicated latest-seed work: state [%d], past [%d], selection [%d]",
			stateCalls,
			pastEventsCalls,
			selectionCalls,
		)
	}

	close(frostChain.releaseFirstState)
	select {
	case <-latestDone:
	case <-time.After(time.Second):
		t.Fatal("latest-seed handler did not finish")
	}

	stateCalls, pastEventsCalls, selectionCalls = frostChain.calls()
	if stateCalls != 2 || pastEventsCalls != 2 || selectionCalls != 1 {
		t.Fatalf(
			"unexpected final calls: state [%d], past [%d], selection [%d]",
			stateCalls,
			pastEventsCalls,
			selectionCalls,
		)
	}
	assertFrostDKGStartedSeedCompleted(t, deduplicator, staleEvent.Seed)
	assertFrostDKGStartedSeedCompleted(t, deduplicator, latestEvent.Seed)
}

func TestHandleFrostDKGStartedRekeysToLatestSeedAlreadyCompleted(
	t *testing.T,
) {
	localChain := Connect()
	node := &node{chain: localChain}
	staleEvent := &FrostDKGStartedEvent{Seed: big.NewInt(100), BlockNumber: 500}
	latestEvent := &FrostDKGStartedEvent{Seed: big.NewInt(200), BlockNumber: 501}
	frostChain := newRekeyingFrostDKGStartedChain(
		staleEvent,
		latestEvent,
	)
	deduplicator := newDeduplicator()

	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		latestEvent,
		false,
	)
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		staleEvent,
		false,
	)

	stateCalls, pastEventsCalls, selectionCalls := frostChain.calls()
	if stateCalls != 2 || pastEventsCalls != 2 || selectionCalls != 1 {
		t.Fatalf(
			"completed latest seed was processed again: state [%d], past [%d], selection [%d]",
			stateCalls,
			pastEventsCalls,
			selectionCalls,
		)
	}
	assertFrostDKGStartedSeedCompleted(t, deduplicator, staleEvent.Seed)
	assertFrostDKGStartedSeedCompleted(t, deduplicator, latestEvent.Seed)
}

func TestHandleFrostDKGStartedRekeyedSeedRetriesAfterTransientFailure(
	t *testing.T,
) {
	localChain := Connect()
	node := &node{chain: localChain}
	staleEvent := &FrostDKGStartedEvent{Seed: big.NewInt(100), BlockNumber: 500}
	latestEvent := &FrostDKGStartedEvent{Seed: big.NewInt(200), BlockNumber: 501}
	frostChain := newRekeyingFrostDKGStartedChain(
		staleEvent,
		latestEvent,
	)
	frostChain.selectionFailures = 1
	deduplicator := newDeduplicator()

	// Handling the stale event transfers to the latest seed, whose first
	// membership lookup fails. The stale seed is terminal, but the latest
	// seed's lease must be released so its own replay can retry.
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		staleEvent,
		false,
	)
	assertFrostDKGStartedSeedCompleted(t, deduplicator, staleEvent.Seed)
	if deduplicator.dkgSeedCache.Has(latestEvent.Seed.Text(16)) {
		t.Fatal("latest seed was completed after transient failure")
	}

	// A stale-event replay remains suppressed and therefore cannot race the
	// latest event. Replaying the latest event retries and completes normally.
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		staleEvent,
		false,
	)
	handleFrostDKGStarted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		latestEvent,
		false,
	)

	stateCalls, pastEventsCalls, selectionCalls := frostChain.calls()
	if stateCalls != 3 || pastEventsCalls != 3 || selectionCalls != 2 {
		t.Fatalf(
			"unexpected retry calls: state [%d], past [%d], selection [%d]",
			stateCalls,
			pastEventsCalls,
			selectionCalls,
		)
	}
	assertFrostDKGStartedSeedCompleted(t, deduplicator, latestEvent.Seed)
}

func TestHandleFrostDKGResultSubmittedReleasesDeduplicationKeyAfterFailure(
	t *testing.T,
) {
	localChain := Connect()
	node := &node{chain: localChain}
	event := &FrostDKGResultSubmittedEvent{
		Seed:        big.NewInt(100),
		ResultHash:  [32]byte{0x01},
		Result:      &registry.Result{},
		BlockNumber: 500,
	}
	frostChain := &transientFrostDKGResultValidationChain{}
	deduplicator := newDeduplicator()

	// A transient validation failure and a later membership failure must both
	// release the key so the result can be replayed.
	for i := 0; i < 3; i++ {
		handleFrostDKGResultSubmitted(
			context.Background(),
			node,
			frostChain,
			deduplicator,
			event,
		)
	}

	if frostChain.validationCalls != 3 || frostChain.selectionCalls != 2 {
		t.Fatalf(
			"unexpected retry calls: validation [%d], selection [%d]",
			frostChain.validationCalls,
			frostChain.selectionCalls,
		)
	}

	// The successful replay determined this operator is not a member, which is
	// terminal local handling; further duplicates must stay suppressed.
	handleFrostDKGResultSubmitted(
		context.Background(),
		node,
		frostChain,
		deduplicator,
		event,
	)
	if frostChain.validationCalls != 3 {
		t.Fatalf("completed result was processed again")
	}
}

func TestScheduleFrostDKGResultApprovalStopsAfterContextCancellation(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	frostChain := &approvalRecordingFrostDKGChain{}
	scheduleFrostDKGResultApproval(
		ctx,
		&node{chain: Connect()},
		frostChain,
		&FrostDKGResultSubmittedEvent{
			ResultHash: [32]byte{0x01},
			Result:     &registry.Result{},
		},
		1,
		1,
	)

	if frostChain.stateCalls != 0 {
		t.Fatalf("queried state after cancellation")
	}
	if frostChain.approvalCalls != 0 {
		t.Fatalf("approved result after cancellation")
	}
}

func TestChallengeInvalidFrostDKGResultRetriesUntilStateLeavesChallenge(t *testing.T) {
	localChain := Connect(time.Millisecond)
	node := &node{chain: localChain}

	frostChain := &retryingFrostDKGChallengeChain{
		state:            Challenge,
		successOnAttempt: 2,
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
	defer cancelCtx()

	challengeInvalidFrostDKGResult(
		ctx,
		node,
		frostChain,
		&FrostDKGResultSubmittedEvent{
			ResultHash: [32]byte{0x01},
			Result:     &registry.Result{},
		},
	)

	if frostChain.challengeCount != 2 {
		t.Fatalf(
			"unexpected challenge count\nexpected: [2]\nactual:   [%d]",
			frostChain.challengeCount,
		)
	}

	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		t.Fatalf("unexpected state error: [%v]", err)
	}
	if state == Challenge {
		t.Fatal("expected challenge loop to leave Challenge state")
	}
}

type retryingFrostDKGChallengeChain struct {
	FrostDKGChain

	mutex            sync.Mutex
	state            DKGState
	challengeCount   int
	successOnAttempt int
}

type transientFrostDKGStartedChain struct {
	FrostDKGChain

	event           *FrostDKGStartedEvent
	stateCalls      int
	pastEventsCalls int
	selectionCalls  int
}

type rekeyingFrostDKGStartedChain struct {
	FrostDKGChain

	mutex sync.Mutex

	events            []*FrostDKGStartedEvent
	stateCalls        int
	pastEventsCalls   int
	selectionCalls    int
	selectionFailures int
	blockFirstState   bool
	firstStateStarted chan struct{}
	releaseFirstState chan struct{}
}

func newRekeyingFrostDKGStartedChain(
	events ...*FrostDKGStartedEvent,
) *rekeyingFrostDKGStartedChain {
	return &rekeyingFrostDKGStartedChain{
		events:            events,
		firstStateStarted: make(chan struct{}),
		releaseFirstState: make(chan struct{}),
	}
}

func (rfdkgsc *rekeyingFrostDKGStartedChain) GetFrostDKGState() (
	DKGState,
	error,
) {
	rfdkgsc.mutex.Lock()
	rfdkgsc.stateCalls++
	call := rfdkgsc.stateCalls
	block := rfdkgsc.blockFirstState && call == 1
	rfdkgsc.mutex.Unlock()

	if block {
		close(rfdkgsc.firstStateStarted)
		<-rfdkgsc.releaseFirstState
	}

	return AwaitingResult, nil
}

func (rfdkgsc *rekeyingFrostDKGStartedChain) PastFrostDKGStartedEvents(
	*FrostDKGStartedEventFilter,
) ([]*FrostDKGStartedEvent, error) {
	rfdkgsc.mutex.Lock()
	defer rfdkgsc.mutex.Unlock()

	rfdkgsc.pastEventsCalls++
	return append([]*FrostDKGStartedEvent{}, rfdkgsc.events...), nil
}

func (rfdkgsc *rekeyingFrostDKGStartedChain) SelectFrostGroup() (
	*GroupSelectionResult,
	error,
) {
	rfdkgsc.mutex.Lock()
	defer rfdkgsc.mutex.Unlock()

	rfdkgsc.selectionCalls++
	if rfdkgsc.selectionFailures > 0 {
		rfdkgsc.selectionFailures--
		return nil, fmt.Errorf("transient selection error")
	}

	return &GroupSelectionResult{
		OperatorsAddresses: chain.Addresses{
			"0x0000000000000000000000000000000000000001",
		},
	}, nil
}

func (rfdkgsc *rekeyingFrostDKGStartedChain) calls() (int, int, int) {
	rfdkgsc.mutex.Lock()
	defer rfdkgsc.mutex.Unlock()

	return rfdkgsc.stateCalls,
		rfdkgsc.pastEventsCalls,
		rfdkgsc.selectionCalls
}

func assertFrostDKGStartedSeedCompleted(
	t *testing.T,
	deduplicator *deduplicator,
	seed *big.Int,
) {
	t.Helper()

	if !deduplicator.dkgSeedCache.Has(seed.Text(16)) {
		t.Fatalf("seed [0x%x] was not completed", seed)
	}
}

func (tfdkgsc *transientFrostDKGStartedChain) GetFrostDKGState() (DKGState, error) {
	tfdkgsc.stateCalls++
	if tfdkgsc.stateCalls == 1 {
		return Idle, fmt.Errorf("transient state error")
	}

	return AwaitingResult, nil
}

func (tfdkgsc *transientFrostDKGStartedChain) PastFrostDKGStartedEvents(
	*FrostDKGStartedEventFilter,
) ([]*FrostDKGStartedEvent, error) {
	tfdkgsc.pastEventsCalls++
	if tfdkgsc.pastEventsCalls == 1 {
		return nil, fmt.Errorf("transient past events error")
	}

	return []*FrostDKGStartedEvent{tfdkgsc.event}, nil
}

func (tfdkgsc *transientFrostDKGStartedChain) SelectFrostGroup() (
	*GroupSelectionResult,
	error,
) {
	tfdkgsc.selectionCalls++
	if tfdkgsc.selectionCalls == 1 {
		return nil, fmt.Errorf("transient selection error")
	}

	return &GroupSelectionResult{
		OperatorsAddresses: chain.Addresses{"0x0000000000000000000000000000000000000001"},
	}, nil
}

type transientFrostDKGResultValidationChain struct {
	FrostDKGChain

	validationCalls int
	selectionCalls  int
}

func (tfdkgvc *transientFrostDKGResultValidationChain) IsFrostDKGResultValid(
	*registry.Result,
) (bool, string, error) {
	tfdkgvc.validationCalls++
	if tfdkgvc.validationCalls == 1 {
		return false, "", fmt.Errorf("transient validation error")
	}

	return true, "", nil
}

func (tfdkgvc *transientFrostDKGResultValidationChain) SelectFrostGroup() (
	*GroupSelectionResult,
	error,
) {
	tfdkgvc.selectionCalls++
	if tfdkgvc.selectionCalls == 1 {
		return nil, fmt.Errorf("transient selection error")
	}

	return &GroupSelectionResult{
		OperatorsAddresses: chain.Addresses{"0x0000000000000000000000000000000000000001"},
	}, nil
}

type approvalRecordingFrostDKGChain struct {
	FrostDKGChain

	stateCalls    int
	approvalCalls int
}

func (arfdkgc *approvalRecordingFrostDKGChain) GetFrostDKGState() (DKGState, error) {
	arfdkgc.stateCalls++
	return Challenge, nil
}

func (arfdkgc *approvalRecordingFrostDKGChain) IsFrostDKGResultValid(
	*registry.Result,
) (bool, string, error) {
	return true, "", nil
}

func (arfdkgc *approvalRecordingFrostDKGChain) ApproveFrostDKGResult(
	*registry.Result,
) error {
	arfdkgc.approvalCalls++
	return nil
}

func (rfdgcc *retryingFrostDKGChallengeChain) GetFrostDKGState() (DKGState, error) {
	rfdgcc.mutex.Lock()
	defer rfdgcc.mutex.Unlock()

	return rfdgcc.state, nil
}

func (rfdgcc *retryingFrostDKGChallengeChain) ChallengeFrostDKGResult(
	*registry.Result,
) error {
	rfdgcc.mutex.Lock()
	defer rfdgcc.mutex.Unlock()

	rfdgcc.challengeCount++
	if rfdgcc.challengeCount >= rfdgcc.successOnAttempt {
		rfdgcc.state = Idle
	}

	return nil
}
