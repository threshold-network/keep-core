//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

// This test chains the FULL FROST wallet-creation coordinator<->chain flow into
// ONE in-process run:
//
//	local chain emits FrostDKGStarted
//	  -> initializeFrostDKGCoordinator's subscription fires
//	    -> handleFrostDKGStarted confirms the event and resolves group membership
//	      -> executeFrostDKGIfPossible announces readiness and runs the REAL
//	         cgo tbtc-signer DKG (executeFrostDKG -> RunDKGWithSeed)
//	        -> the assembled result is submitted back through the chain
//	           (FrostDKGChain.SubmitFrostDKGResult)
//	          -> the wallet is verified registered on the local chain.
//
// Until now the coordinator<->chain wiring and the real cgo DKG execution were
// tested SEPARATELY (frost_dkg_coordinator_test.go drives the chain plumbing
// with stub results; frost_dkg_execution_frost_native_test.go drives the real
// DKG in isolation). This test is the first that exercises both in one flow.
//
// WHAT IS REAL vs REDUCED
//
//   - REAL: the DKG output. executeFrostDKG calls the process-global cgo
//     tbtc-signer engine (buildTaggedTBTCSignerEngine, registered via
//     RegisterNativeExecutionFFISigningPrimitiveForBuild). The x-only group key
//     that lands on-chain is the exact key the engine produced - captured by a
//     thin recording wrapper and compared byte-for-byte with the submission, so
//     no fake/injected result can pass.
//   - REAL: the submission path. The result is submitted through the
//     FrostDKGChain interface (SubmitFrostDKGResult), not injected into chain
//     state directly.
//   - REAL: the coordinator wiring. The event is delivered through the
//     OnFrostDKGStarted subscription registered by initializeFrostDKGCoordinator;
//     confirmation, state check, past-event lookup, membership resolution,
//     readiness announcement, result assembly, DKG-result operator-signature
//     collection, and delayed submission all run as in production.
//   - REDUCED (documented): the group is a 3-of-... group whose 3 seats are all
//     held by ONE operator/node. The cgo engine is a process-global
//     OnceLock<Mutex>, so N independent real-custody participants cannot run
//     concurrently in one OS process. The tbtc-signer development dealer DKG by
//     design has a single engine hold every participant's key package, which is
//     exactly this shape: one node drives an n>=2 real DKG (the cgo library
//     rejects n==1: "participants must contain at least 2 entries"). The fully
//     live rehearsal (N node processes vs a real chain with staked operators and
//     sortition) is out of scope here - see the PR "Not covered / follow-up".

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/subscription"
)

// reduced group: 3 seats, all owned by the single participating node.
const (
	frostE2EGroupSize       = 3
	frostE2EGroupQuorum     = 2
	frostE2EHonestThreshold = 2
)

func TestFrostDKGCoordinatorChainEndToEnd_RealCgo(t *testing.T) {
	setupFrostE2ESignerState(t)

	// Register the REAL cgo tbtc-signer engine, then wrap it so we can capture
	// the exact DKG output the coordinator submits on-chain.
	frostsigning.UnregisterNativeTBTCSignerEngine()
	frostsigning.RegisterNativeExecutionFFISigningPrimitiveForBuild()
	t.Cleanup(frostsigning.UnregisterNativeTBTCSignerEngine)

	realEngine := frostsigning.CurrentNativeTBTCSignerEngine()
	seeded, ok := realEngine.(frostsigning.NativeTBTCSignerSeededDKGEngine)
	if realEngine == nil || !ok {
		// The build-tagged engine registers whether or not libfrost_tbtc is
		// actually linked, so this only guards the trivial "no engine at all"
		// case; the real lib-usability probe follows.
		if requireFrostCgo() {
			t.Fatalf(
				"real cgo tbtc-signer seeded DKG engine unavailable "+
					"(last registration error: %v)",
				frostsigning.LastNativeRegistrationError(),
			)
		}
		t.Skip("real cgo tbtc-signer seeded DKG engine unavailable; skipping")
	}

	// P1: probe the REAL linked library UP FRONT. The tbtc-signer engine
	// registers via build tag even when libfrost_tbtc is absent/stale; the
	// missing ABI symbol only surfaces inside a request-taking engine op (which
	// funnels through the once-per-process ABI preflight). Exercise that path
	// here - BEFORE emitting the event - so an unusable lib SKIPS immediately
	// (or FATALs under the require-cgo gate) instead of failing the coordinator
	// goroutine and hanging until the 90s deadline. This runs on the raw seeded
	// engine so it does not pollute the recording wrapper's captured key.
	probeSeed, err := frostDKGSeedHex(randomFrostSeed(t))
	if err != nil {
		t.Fatalf("cannot build probe seed: %v", err)
	}
	// Unique session id per invocation so in-process repeats (-count=N) add a
	// fresh probe DKG session rather than conflicting on a fixed one.
	_, probeErr := seeded.RunDKGWithSeed(
		fmt.Sprintf("frost-e2e-abi-probe-%d-%x", os.Getpid(), randomFrostSeed(t).Bytes()),
		[]frostsigning.NativeTBTCSignerDKGParticipant{
			{Identifier: 1, PublicKeyHex: frostsigning.NativeTBTCSignerDKGPlaceholderPublicKeyHex(1)},
			{Identifier: 2, PublicKeyHex: frostsigning.NativeTBTCSignerDKGPlaceholderPublicKeyHex(2)},
		},
		2,
		probeSeed,
	)
	skipOrFailIfFrostUnavailable(t, "tbtc-signer ABI preflight (RunDKGWithSeed)", probeErr)

	recordingEngine := &recordingSeededDKGEngine{
		NativeTBTCSignerEngine: realEngine,
		seeded:                 seeded,
	}
	if err := frostsigning.RegisterNativeTBTCSignerEngine(recordingEngine); err != nil {
		t.Fatalf("cannot register recording DKG engine: %v", err)
	}

	// Stand up the local chain and a real (in-process) net provider, and build
	// the node exactly as production does via newNode.
	localChain := Connect(time.Millisecond)
	localChain.frostWalletRegistryAvailable = true

	operatorAddress, err := localChain.operatorAddress()
	if err != nil {
		t.Fatalf("cannot resolve operator address: %v", err)
	}

	groupParameters := &GroupParameters{
		GroupSize:       frostE2EGroupSize,
		GroupQuorum:     frostE2EGroupQuorum,
		HonestThreshold: frostE2EHonestThreshold,
	}

	node, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		local.Connect(),
		createMockKeyStorePersistence(t),
		&mockPersistenceHandle{},
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
	)
	if err != nil {
		t.Fatalf("cannot create node: %v", err)
	}

	// The reduced group: 3 seats, every seat mapped to this one operator.
	selectedAddresses := make(chain.Addresses, frostE2EGroupSize)
	selectedIDs := make(chain.OperatorIDs, frostE2EGroupSize)
	for i := 0; i < frostE2EGroupSize; i++ {
		selectedAddresses[i] = operatorAddress
		selectedIDs[i] = chain.OperatorID(i + 1)
	}

	frostChain := &frostE2EChain{
		localChain: localChain,
		logf:       t.Logf,
		state:      Idle,
		group: &GroupSelectionResult{
			OperatorsIDs:       selectedIDs,
			OperatorsAddresses: selectedAddresses,
		},
		startedHandlers:   map[int]func(*FrostDKGStartedEvent){},
		bridgeHandlers:    map[int]func(*BridgeNewWalletRequestedEvent){},
		submittedHandlers: map[int]func(*FrostDKGResultSubmittedEvent){},
		submittedCh:       make(chan *registry.Result, 1),
		recoveryScanned:   make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// (1) wire the coordinator to the chain, exactly as production startup does.
	// This also launches recoverFrostDKGCoordinatorState asynchronously.
	initializeFrostDKGCoordinator(ctx, node, frostChain)

	// P2: wait until recovery has completed its initial state scan (observing
	// IDLE) before flipping the state. Recovery reads GetFrostDKGState exactly
	// once at startup; because the chain is IDLE with no past started events, it
	// exits as a no-op. Gating the emit on that scan removes any reliance on
	// goroutine scheduling: only the OnFrostDKGStarted subscription (which always
	// runs the block-confirmation path) can drive this DKG.
	select {
	case <-frostChain.recoveryScanned:
	case <-ctx.Done():
		t.Fatalf("recovery state scan did not complete: %v", ctx.Err())
	}

	// (2) simulate requestNewWallet -> the chain emits FrostDKGStarted with a
	// unique seed (keeps the derived cgo session id unique across -count=N).
	seed := randomFrostSeed(t)
	t.Logf("STEP 2: emitting FrostDKGStarted seed=0x%x", seed)
	frostChain.startFrostDKG(seed)

	// (3)+(4) wait for the coordinator to run the real DKG and submit the result
	// back through the chain interface.
	var submitted *registry.Result
	select {
	case submitted = <-frostChain.submittedCh:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for FROST DKG result submission: %v", ctx.Err())
	}
	t.Logf("STEP 4: SubmitFrostDKGResult observed on-chain")

	// P2 (regression guard): prove the OnFrostDKGStarted subscription's
	// block-confirmation path actually ran - not the recovery bypass. That path
	// blocks on node.waitForBlockHeight(emitBlock + dkgStartedConfirmationBlocks)
	// before it can announce and submit, so the submission CANNOT land before
	// that block floor. The recovery bypass (handleFrostDKGStarted with
	// waitForConfirmation=false) skips the wait and would submit
	// dkgStartedConfirmationBlocks sooner. This is a deterministic floor
	// (waitForBlockHeight blocks until the height is reached), not a timing race.
	frostChain.mu.Lock()
	emitBlock := frostChain.emitBlock
	submitBlock := frostChain.submitBlock
	frostChain.mu.Unlock()
	if submitBlock < emitBlock+dkgStartedConfirmationBlocks {
		t.Fatalf(
			"submission at block %d did not clear the confirmation floor "+
				"(emit %d + %d); the block-confirmation path was NOT exercised",
			submitBlock,
			emitBlock,
			dkgStartedConfirmationBlocks,
		)
	}
	t.Logf(
		"CONFIRMATION PATH: submit block %d >= emit %d + confirmation %d (block-confirmation path exercised)",
		submitBlock,
		emitBlock,
		dkgStartedConfirmationBlocks,
	)

	// --- LOAD-BEARING ASSERTIONS ---------------------------------------------
	// The submitted x-only group key must be the EXACT output of the real cgo
	// engine (captured independently by the recording wrapper), proving a real
	// DKG output - not an injected/fake value - reached the chain via the
	// coordinator's submission path.
	capturedKey, captured := recordingEngine.capturedOutputKey()
	if !captured {
		t.Fatal("recording engine never observed a real DKG execution")
	}
	if submitted.XOnlyOutputKey != capturedKey {
		t.Fatalf(
			"submitted x-only key does not match the real cgo DKG output\n"+
				"engine:    %x\nsubmitted: %x",
			capturedKey[:],
			submitted.XOnlyOutputKey[:],
		)
	}
	if submitted.XOnlyOutputKey == (frost.OutputKey{}) {
		t.Fatal("submitted x-only key is all-zero")
	}
	// It must be a genuine secp256k1 x-only point (lifts to a valid curve point).
	if _, err := btcec.ParsePubKey(
		append([]byte{0x02}, submitted.XOnlyOutputKey[:]...),
	); err != nil {
		t.Fatalf("submitted x-only key is not a valid curve point: %v", err)
	}
	t.Logf(
		"LOAD-BEARING: real cgo DKG x-only key %x landed on-chain via the coordinator",
		submitted.XOnlyOutputKey[:],
	)

	// (5) the wallet backed by that real key must be registered on the chain.
	pubKey, err := frostOutputKeyToECDSAPublicKey(submitted.XOnlyOutputKey)
	if err != nil {
		t.Fatalf("cannot lift submitted x-only key: %v", err)
	}
	walletID := DeriveLegacyWalletID(bitcoin.PublicKeyHash(pubKey))
	registered, err := frostChain.IsFrostWalletRegistered(walletID)
	if err != nil {
		t.Fatalf("cannot check wallet registration: %v", err)
	}
	if !registered {
		t.Fatal("wallet was not registered on-chain after DKG result submission")
	}
	t.Logf("STEP 5: wallet %x registered on-chain", walletID)

	// Sanity: the result carries the reduced group and no misbehaving members.
	if len(submitted.Members) != frostE2EGroupSize {
		t.Fatalf(
			"unexpected members count: got %d want %d",
			len(submitted.Members),
			frostE2EGroupSize,
		)
	}
	if len(submitted.MisbehavedMembersIndices) != 0 {
		t.Fatalf(
			"unexpected misbehaving members: %v",
			submitted.MisbehavedMembersIndices,
		)
	}
}

// recordingSeededDKGEngine wraps the process-global real cgo tbtc-signer engine
// and captures the x-only output key of the DKG the coordinator runs. Every
// method is delegated to the wrapped engine; only RunDKGWithSeed is intercepted
// to record its (real) output. This lets the test assert the on-chain result is
// the exact engine output, never a fake.
type recordingSeededDKGEngine struct {
	// NativeTBTCSignerEngine is the real cgo engine; embedding it promotes the
	// full base method set (RunDKG, StartSignRound, FinalizeSignRound,
	// BuildTaprootTx, VerifySignatureShare).
	frostsigning.NativeTBTCSignerEngine
	// seeded is the same real engine, used to delegate the intercepted
	// RunDKGWithSeed call.
	seeded frostsigning.NativeTBTCSignerSeededDKGEngine

	mu        sync.Mutex
	outputKey frost.OutputKey
	captured  bool
}

func (e *recordingSeededDKGEngine) RunDKGWithSeed(
	sessionID string,
	participants []frostsigning.NativeTBTCSignerDKGParticipant,
	threshold uint16,
	dkgSeedHex string,
) (*frostsigning.NativeTBTCSignerDKGResult, error) {
	result, err := e.seeded.RunDKGWithSeed(
		sessionID,
		participants,
		threshold,
		dkgSeedHex,
	)
	if err != nil {
		return result, err
	}

	if outputKey, keyErr := outputKeyFromTBTCSignerDKGResult(result); keyErr == nil {
		e.mu.Lock()
		e.outputKey = outputKey
		e.captured = true
		e.mu.Unlock()
	}

	return result, err
}

func (e *recordingSeededDKGEngine) capturedOutputKey() (frost.OutputKey, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.outputKey, e.captured
}

// frostE2EChain is a test-local FrostDKGChain built on top of the package's
// localChain. It emits FrostDKGStarted, exposes GetFrostDKGState / group
// selection, accepts SubmitFrostDKGResult, and records the submitted wallet so
// the test can verify the full lifecycle. It reuses localChain for block
// counting, signing, operator identity, and the wallet registry.
type frostE2EChain struct {
	*localChain

	logf func(string, ...any)

	mu                sync.Mutex
	state             DKGState
	pastStarted       []*FrostDKGStartedEvent
	pastSubmitted     []*FrostDKGResultSubmittedEvent
	group             *GroupSelectionResult
	submitted         *registry.Result
	emitBlock         uint64
	submitBlock       uint64
	startedHandlers   map[int]func(*FrostDKGStartedEvent)
	bridgeHandlers    map[int]func(*BridgeNewWalletRequestedEvent)
	submittedHandlers map[int]func(*FrostDKGResultSubmittedEvent)
	submittedCh       chan *registry.Result

	// recoveryScanned is closed by the FIRST GetFrostDKGState call, which is
	// recoverFrostDKGCoordinatorState's initial scan (nothing else queries the
	// state before the started event is emitted). The test waits on it to
	// guarantee recovery has already observed the IDLE state - making it a
	// genuine no-op - before the state is flipped to AwaitingResult, so the DKG
	// is driven DETERMINISTICALLY through the OnFrostDKGStarted subscription +
	// confirmation path and never through the recovery bypass. See P2.
	recoveryScanned  chan struct{}
	recoveryScanOnce sync.Once
}

// startFrostDKG simulates the on-chain requestNewWallet -> DkgStarted sequence:
// it flips the DKG state to AwaitingResult, records the started event for the
// coordinator's past-event lookup, and fires the subscription handlers.
func (c *frostE2EChain) startFrostDKG(seed *big.Int) {
	block, err := c.blockCounter.CurrentBlock()
	if err != nil {
		block = 0
	}

	event := &FrostDKGStartedEvent{Seed: seed, BlockNumber: block}

	c.mu.Lock()
	c.state = AwaitingResult
	c.emitBlock = block
	c.pastStarted = append(c.pastStarted, event)
	bridgeHandlers := make([]func(*BridgeNewWalletRequestedEvent), 0, len(c.bridgeHandlers))
	for _, handler := range c.bridgeHandlers {
		bridgeHandlers = append(bridgeHandlers, handler)
	}
	startedHandlers := make([]func(*FrostDKGStartedEvent), 0, len(c.startedHandlers))
	for _, handler := range c.startedHandlers {
		startedHandlers = append(startedHandlers, handler)
	}
	c.mu.Unlock()

	for _, handler := range bridgeHandlers {
		handler(&BridgeNewWalletRequestedEvent{BlockNumber: block})
	}
	for _, handler := range startedHandlers {
		handler(event)
	}
}

func (c *frostE2EChain) OnBridgeNewWalletRequested(
	handler func(event *BridgeNewWalletRequestedEvent),
) subscription.EventSubscription {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := generateHandlerID()
	c.bridgeHandlers[id] = handler
	return subscription.NewEventSubscription(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.bridgeHandlers, id)
	})
}

func (c *frostE2EChain) OnFrostDKGStarted(
	handler func(event *FrostDKGStartedEvent),
) subscription.EventSubscription {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := generateHandlerID()
	c.startedHandlers[id] = handler
	return subscription.NewEventSubscription(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.startedHandlers, id)
	})
}

func (c *frostE2EChain) PastFrostDKGStartedEvents(
	_ *FrostDKGStartedEventFilter,
) ([]*FrostDKGStartedEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	events := make([]*FrostDKGStartedEvent, len(c.pastStarted))
	copy(events, c.pastStarted)
	return events, nil
}

func (c *frostE2EChain) OnFrostDKGResultSubmitted(
	handler func(event *FrostDKGResultSubmittedEvent),
) subscription.EventSubscription {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := generateHandlerID()
	c.submittedHandlers[id] = handler
	return subscription.NewEventSubscription(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.submittedHandlers, id)
	})
}

func (c *frostE2EChain) PastFrostDKGResultSubmittedEvents(
	_ *FrostDKGResultSubmittedEventFilter,
) ([]*FrostDKGResultSubmittedEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	events := make([]*FrostDKGResultSubmittedEvent, len(c.pastSubmitted))
	copy(events, c.pastSubmitted)
	return events, nil
}

func (c *frostE2EChain) OnFrostDKGResultChallenged(
	func(event *FrostDKGResultChallengedEvent),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

func (c *frostE2EChain) OnFrostDKGResultApproved(
	func(event *FrostDKGResultApprovedEvent),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

func (c *frostE2EChain) SelectFrostGroup() (*GroupSelectionResult, error) {
	return c.group, nil
}

func (c *frostE2EChain) GetFrostDKGState() (DKGState, error) {
	c.mu.Lock()
	state := c.state
	c.mu.Unlock()

	// The first reader is recoverFrostDKGCoordinatorState's startup scan; signal
	// it has observed the state so the test can safely emit afterwards (P2).
	c.recoveryScanOnce.Do(func() { close(c.recoveryScanned) })

	return state, nil
}

func (c *frostE2EChain) IsFrostDKGResultValid(
	*registry.Result,
) (bool, string, error) {
	return true, "", nil
}

func (c *frostE2EChain) CalculateFrostDKGResultDigest(
	seed *big.Int,
	result *registry.Result,
) ([32]byte, error) {
	hash := sha256.New()
	if seed != nil {
		hash.Write(seed.Bytes())
	}
	hash.Write(result.XOnlyOutputKey[:])
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

// SubmitFrostDKGResult records the coordinator's real DKG result, registers the
// resulting wallet, transitions the DKG state, notifies subscribers, and signals
// the test. This is the single on-chain entry point the coordinator uses to land
// a result - nothing bypasses it.
func (c *frostE2EChain) SubmitFrostDKGResult(result *registry.Result) error {
	pubKey, err := frostOutputKeyToECDSAPublicKey(result.XOnlyOutputKey)
	if err != nil {
		return fmt.Errorf("submitted x-only key does not lift to a curve point: %w", err)
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(pubKey)
	walletID := DeriveLegacyWalletID(walletPublicKeyHash)

	block, err := c.blockCounter.CurrentBlock()
	if err != nil {
		return err
	}

	c.mu.Lock()
	if c.state != AwaitingResult {
		state := c.state
		c.mu.Unlock()
		return fmt.Errorf("not awaiting FROST DKG result, state=%v", state)
	}
	c.state = Challenge
	c.submitted = result
	c.submitBlock = block
	resultHash := DKGChainResultHash(sha256.Sum256(result.XOnlyOutputKey[:]))
	seed := big.NewInt(0)
	if len(c.pastStarted) > 0 {
		seed = c.pastStarted[len(c.pastStarted)-1].Seed
	}
	event := &FrostDKGResultSubmittedEvent{
		Seed:        seed,
		ResultHash:  resultHash,
		Result:      result,
		BlockNumber: block,
	}
	c.pastSubmitted = append(c.pastSubmitted, event)
	submittedHandlers := make([]func(*FrostDKGResultSubmittedEvent), 0, len(c.submittedHandlers))
	for _, handler := range c.submittedHandlers {
		submittedHandlers = append(submittedHandlers, handler)
	}
	c.mu.Unlock()

	// Register the wallet backed by the real DKG output key.
	c.setWallet(walletPublicKeyHash, &WalletChainData{
		WalletID: walletID,
		State:    StateLive,
	})

	if c.logf != nil {
		c.logf(
			"STEP 3+4: chain received SubmitFrostDKGResult x-only=%x wallet=%x",
			result.XOnlyOutputKey[:],
			walletID,
		)
	}

	for _, handler := range submittedHandlers {
		handler(event)
	}

	select {
	case c.submittedCh <- result:
	default:
	}

	return nil
}

func (c *frostE2EChain) ChallengeFrostDKGResult(*registry.Result) error {
	return nil
}

func (c *frostE2EChain) ApproveFrostDKGResult(*registry.Result) error {
	return nil
}

func (c *frostE2EChain) FrostDKGParameters() (*DKGParameters, error) {
	return &DKGParameters{
		// Large submission window so block-time based cancellation never races
		// the DKG; challenge/approve windows are present but not exercised here.
		SubmissionTimeoutBlocks:       100000,
		ChallengePeriodBlocks:         15,
		ApprovePrecedencePeriodBlocks: 5,
	}, nil
}

func setupFrostE2ESignerState(t *testing.T) {
	t.Helper()
	t.Setenv("TBTC_SIGNER_PROFILE", "development")
	t.Setenv("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false")

	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}
	t.Setenv("TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX", hex.EncodeToString(stateKey))

	// Per-PROCESS state path: stable across -count=N (the signer binds its
	// process-global state-file lock to the first path) and unique across
	// processes.
	stateDir := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("keep-frost-coordinator-e2e-state-%d", os.Getpid()),
	)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("cannot create signer state dir: %v", err)
	}
	t.Setenv("TBTC_SIGNER_STATE_PATH", filepath.Join(stateDir, "signer-state"))
}

func randomFrostSeed(t *testing.T) *big.Int {
	t.Helper()
	buffer := make([]byte, 30)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("cannot read random seed: %v", err)
	}
	seed := new(big.Int).SetBytes(buffer)
	if seed.Sign() == 0 {
		seed = big.NewInt(1)
	}
	return seed
}

// requireFrostCgo mirrors the reference harness's FrostRequireCgoEnvVar gate
// (pkg/frost/signing): the "lib unavailable" outcome is a SKIP unless
// KEEP_CORE_FROST_REQUIRE_CGO is truthy, in which case it is FATAL.
func requireFrostCgo() bool {
	return strings.EqualFold(
		strings.TrimSpace(os.Getenv("KEEP_CORE_FROST_REQUIRE_CGO")),
		"true",
	)
}

// skipOrFailIfFrostUnavailable mirrors the reference skipFrostUnavailable
// (pkg/frost/signing/roast_real_cgo_interactive_e2e_frost_native_test.go): a
// missing/stale libfrost_tbtc surfaces as ErrNativeCryptographyUnavailable and
// SKIPS the test (or FATALs under the require-cgo gate); any other error is a
// genuine failure; nil is a no-op.
func skipOrFailIfFrostUnavailable(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		if requireFrostCgo() {
			t.Fatalf(
				"%s: tbtc-signer FFI symbol unavailable but "+
					"KEEP_CORE_FROST_REQUIRE_CGO is set (lib absent, stale, or "+
					"failed to load - the linked libfrost_tbtc must satisfy the "+
					"bridge): %v",
				op,
				err,
			)
		}
		t.Skipf(
			"%s: linked tbtc-signer FFI symbol unavailable (lib absent or "+
				"stale; rebuild libfrost_tbtc): %v",
			op,
			err,
		)
	}
	t.Fatalf("%s: %v", op, err)
}
