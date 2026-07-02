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
	"fmt"
	"math/big"
	"os"
	"path/filepath"
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
		if requireFrostCgo() {
			t.Fatalf(
				"real cgo tbtc-signer seeded DKG engine unavailable "+
					"(last registration error: %v)",
				frostsigning.LastNativeRegistrationError(),
			)
		}
		t.Skip("real cgo tbtc-signer seeded DKG engine unavailable; skipping")
	}

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
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// (1) wire the coordinator to the chain, exactly as production startup does.
	initializeFrostDKGCoordinator(ctx, node, frostChain)

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
	startedHandlers   map[int]func(*FrostDKGStartedEvent)
	bridgeHandlers    map[int]func(*BridgeNewWalletRequestedEvent)
	submittedHandlers map[int]func(*FrostDKGResultSubmittedEvent)
	submittedCh       chan *registry.Result
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
	defer c.mu.Unlock()
	return c.state, nil
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

func requireFrostCgo() bool {
	value := os.Getenv("KEEP_CORE_FROST_REQUIRE_CGO")
	switch value {
	case "", "0", "false", "FALSE", "False":
		return false
	default:
		return true
	}
}
