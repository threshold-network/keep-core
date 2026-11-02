package spv

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// proofLoopBlockCounter counts the chain tip reads of a
// maintainReservationProofs run. Every pass of runReservationProofLoop reads
// the tip once per proof round, so two reads complete one pass.
type proofLoopBlockCounter struct {
	*mockBlockCounter

	mutex sync.Mutex
	reads int64
}

// CurrentBlock counts the read before answering with the configured block.
func (c *proofLoopBlockCounter) CurrentBlock() (uint64, error) {
	c.mutex.Lock()
	c.reads++
	c.mutex.Unlock()

	return c.mockBlockCounter.CurrentBlock()
}

// passReads returns the number of chain tip reads performed so far.
func (c *proofLoopBlockCounter) passReads() int64 {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.reads
}

// proofLoopTestChain wraps localChain so a test can observe how many times a
// maintainReservationProofs run read a generation's on-chain action: an
// evicted generation is read once and never again, a still-tracked one is
// re-read on every pass.
type proofLoopTestChain struct {
	*localChain

	mutex       sync.Mutex
	actionReads map[string]int
}

// GetReservationAction counts the read before delegating to the local chain.
func (c *proofLoopTestChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	c.mutex.Lock()
	c.actionReads[buildReservationActionKey(reservationKey, requestNonce)]++
	c.mutex.Unlock()

	return c.localChain.GetReservationAction(reservationKey, requestNonce)
}

// actionReadCount returns how many times the given generation's action was
// read.
func (c *proofLoopTestChain) actionReadCount(
	reservationKey *big.Int,
	requestNonce uint64,
) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.actionReads[buildReservationActionKey(reservationKey, requestNonce)]
}

// proofLoopSubmissions records the reservation keys a maintainer submitted
// proofs for, in submission order. The maintainer runs in its own goroutine,
// so every access is guarded.
type proofLoopSubmissions struct {
	mutex sync.Mutex
	keys  []string
}

func (s *proofLoopSubmissions) record(reservationKey *big.Int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.keys = append(s.keys, reservationKey.String())
}

// count returns how many proofs were submitted for the reservation.
func (s *proofLoopSubmissions) count(reservationKey *big.Int) int {
	key := reservationKey.String()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	count := 0
	for _, submitted := range s.keys {
		if submitted == key {
			count++
		}
	}

	return count
}

// total returns how many proofs were submitted overall.
func (s *proofLoopSubmissions) total() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return len(s.keys)
}

// reservationProofLoopFixture builds the local chain and Bitcoin chain a
// maintainReservationProofs test drives: block headers and the difficulty
// epochs a proof needs, a chain tip the event scans read, and a counting
// action reader.
type reservationProofLoopFixture struct {
	chain        *proofLoopTestChain
	btcChain     *reservationProofBitcoinChain
	blockCounter *proofLoopBlockCounter
	submissions  *proofLoopSubmissions
}

func newReservationProofLoopFixture(t *testing.T) *reservationProofLoopFixture {
	t.Helper()

	const proofStart = 790270
	difficulty := func(d int64) *big.Int { return big.NewInt(d) }

	btcChain := newReservationProofBitcoinChain()
	if err := populateBlockHeaders(
		btcChain.localBitcoinChain,
		proofStart,
		proofStart+19,
		func(uint) *big.Int { return difficulty(32) },
	); err != nil {
		t.Fatal(err)
	}

	blockCounter := &proofLoopBlockCounter{mockBlockCounter: newMockBlockCounter()}
	blockCounter.SetCurrentBlock(1000)

	local := newLocalChain()
	local.setBlockCounter(blockCounter)
	local.setTxProofDifficultyFactor(big.NewInt(6))
	local.setCurrentEpoch(392)
	local.setCurrentAndPrevEpochDifficulty(difficulty(32), difficulty(16))

	return &reservationProofLoopFixture{
		chain:        &proofLoopTestChain{localChain: local, actionReads: make(map[string]int)},
		btcChain:     btcChain,
		blockCounter: blockCounter,
		submissions:  &proofLoopSubmissions{},
	}
}

// addAcceptanceGeneration installs one provable acceptance action
// generation: a funding transaction, an anchor transaction spending it in a
// single input and paying walletPublicKeyHash in its single output with
// enough confirmations to prove, the matching
// ReservationAcceptanceRequested event, and the action in actionState. It
// returns the reservation key.
func (f *reservationProofLoopFixture) addAcceptanceGeneration(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	fundingValue int64,
	actionState tbtc.ReservationActionState,
) *big.Int {
	t.Helper()

	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: fundingValue}},
	}
	if err := f.btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}
	fundingTxHash := fundingTx.Hash()

	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	anchorTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := f.btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}
	if err := f.btcChain.addTransactionConfirmations(anchorTx.Hash(), 20); err != nil {
		t.Fatal(err)
	}
	f.btcChain.setCoinbaseTxHash(anchorTx.Hash())

	reservationKey := f.chain.BuildDepositKey(fundingTxHash, 0)
	f.chain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})
	f.chain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        reservationAcceptanceTestNonce,
		WalletPublicKeyHash: walletPublicKeyHash,
		DepositAmount:       150000,
		BlockNumber:         500,
	})
	f.chain.setReservationAction(
		reservationKey,
		reservationAcceptanceTestNonce,
		&tbtc.ReservationAction{
			State:                     actionState,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)

	return reservationKey
}

// reservationAcceptanceTestNonce is the request nonce of every generation the
// fixture installs; the reservation keys keep them apart.
const reservationAcceptanceTestNonce = 1

// recordAcceptanceSubmissions has the chain record every submitted acceptance
// proof and, when onSubmit is set, lets the test fail the submission.
func (f *reservationProofLoopFixture) recordAcceptanceSubmissions(
	onSubmit func(reservationKey *big.Int) error,
) {
	f.chain.submitReservationAcceptanceProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		_ uint64,
	) error {
		f.submissions.record(reservationKey)

		if onSubmit != nil {
			return onSubmit(reservationKey)
		}

		return nil
	}
}

// maintainerConfig returns a Config for a maintainer run. The restart backoff
// is deliberately long in every caller: a pass that ends in a restart instead
// of the idle backoff must stall long enough for the test to notice.
func maintainerConfig(restartBackoff, idleBackoff time.Duration) Config {
	return Config{
		TransactionLimit:   100,
		MaxProofHeaders:    DefaultMaxProofHeaders,
		EthereumNetwork:    ethereum.Developer,
		RestartBackoffTime: restartBackoff,
		IdleBackoffTime:    idleBackoff,
	}
}

// runMaintainer starts maintainReservationProofs in a goroutine and returns a
// channel closed when it returns.
func (f *reservationProofLoopFixture) runMaintainer(
	ctx context.Context,
	config Config,
) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		maintainReservationProofs(
			ctx,
			config,
			f.chain,
			f.chain,
			f.btcChain,
			nil,
		)
	}()

	return done
}

// proofLoopDeadline bounds how long a test waits for the maintainer to reach a
// state or to return. It is generous: the bound exists to fail under a broken
// loop, not to measure speed.
const proofLoopDeadline = 10 * time.Second

// waitForProofLoopCondition polls condition until it holds or the deadline
// passes.
func waitForProofLoopCondition(
	t *testing.T,
	condition func() bool,
	what string,
) {
	t.Helper()

	deadline := time.Now().Add(proofLoopDeadline)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

// waitForMaintainerReturn asserts the maintainer returned before the
// deadline.
func waitForMaintainerReturn(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(proofLoopDeadline):
		t.Fatalf(
			"maintainReservationProofs did not return within %v of its context "+
				"being cancelled",
			proofLoopDeadline,
		)
	}
}

// walletPublicKeyHashFor returns a distinct test wallet public key hash per
// index.
func walletPublicKeyHashFor(index byte) [20]byte {
	hash := [20]byte{}
	for i := range hash {
		hash[i] = index
	}

	return hash
}

// TestMaintainReservationProofs_ProcessesEveryPendingGeneration proves the
// maintainer's single pass proves every tracked acceptance generation, not
// just the first one the event scan delivers. The context is cancelled before
// the run, so the maintainer performs exactly one pass and then stops.
func TestMaintainReservationProofs_ProcessesEveryPendingGeneration(t *testing.T) {
	fixture := newReservationProofLoopFixture(t)
	fixture.recordAcceptanceSubmissions(nil)

	firstKey := fixture.addAcceptanceGeneration(
		t,
		walletPublicKeyHashFor(1),
		150000,
		tbtc.ReservationActionStatePending,
	)
	secondKey := fixture.addAcceptanceGeneration(
		t,
		walletPublicKeyHashFor(2),
		160000,
		tbtc.ReservationActionStatePending,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Hour))
	waitForMaintainerReturn(t, done)

	for _, reservationKey := range []*big.Int{firstKey, secondKey} {
		if count := fixture.submissions.count(reservationKey); count != 1 {
			t.Fatalf(
				"expected exactly one proof submitted for reservation [%v], "+
					"got [%d]: a single pass must prove every pending generation",
				reservationKey,
				count,
			)
		}
	}
	if total := fixture.submissions.total(); total != 2 {
		t.Fatalf("expected two submissions in total, got [%d]", total)
	}
}

// TestMaintainReservationProofs_OneFailedGenerationKeepsTheOthers proves two
// things about a failing generation: it does not starve the generation
// processed after it in the same pass, and its failure is retried after the
// idle backoff instead of forcing a restart. The restart backoff is an hour,
// so a loop that treated the per-item error as chain-wide would never reach
// the retry within the test's deadline.
func TestMaintainReservationProofs_OneFailedGenerationKeepsTheOthers(t *testing.T) {
	fixture := newReservationProofLoopFixture(t)

	firstKey := fixture.addAcceptanceGeneration(
		t,
		walletPublicKeyHashFor(3),
		150000,
		tbtc.ReservationActionStatePending,
	)
	secondKey := fixture.addAcceptanceGeneration(
		t,
		walletPublicKeyHashFor(4),
		160000,
		tbtc.ReservationActionStatePending,
	)

	// Generations are proved in ascending reservation key order: fail the one
	// the pass reaches first.
	failingKey, provedKey := firstKey, secondKey
	if secondKey.Cmp(firstKey) < 0 {
		failingKey, provedKey = secondKey, firstKey
	}

	fixture.recordAcceptanceSubmissions(func(reservationKey *big.Int) error {
		if reservationKey.Cmp(failingKey) == 0 {
			return errors.New("simulated submission failure")
		}

		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Millisecond))

	waitForProofLoopCondition(
		t,
		func() bool {
			return fixture.submissions.count(provedKey) >= 1 &&
				fixture.submissions.count(failingKey) >= 2
		},
		"the failing generation to be retried after the idle backoff while "+
			"the other generation is proved",
	)

	cancel()
	waitForMaintainerReturn(t, done)

	if count := fixture.submissions.count(failingKey); count < 2 {
		t.Fatalf(
			"expected the failed generation to be retried after the idle "+
				"backoff, submissions = [%d]",
			count,
		)
	}
	if count := fixture.submissions.count(provedKey); count < 1 {
		t.Fatalf(
			"expected the healthy generation to be proved despite the "+
				"failure of the generation processed before it, submissions = [%d]",
			count,
		)
	}
}

// TestMaintainReservationProofs_StopsOnContextCancel proves the maintainer
// returns on context cancellation both when the context is already cancelled
// before the run and when it is cancelled from inside a pass.
func TestMaintainReservationProofs_StopsOnContextCancel(t *testing.T) {
	t.Run("cancelled before the run", func(t *testing.T) {
		fixture := newReservationProofLoopFixture(t)
		fixture.recordAcceptanceSubmissions(nil)

		reservationKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(5),
			150000,
			tbtc.ReservationActionStatePending,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Hour))
		waitForMaintainerReturn(t, done)

		// The pass runs before the loop checks the context; a cancelled
		// context must end the maintainer after that one pass.
		if count := fixture.submissions.count(reservationKey); count != 1 {
			t.Fatalf(
				"expected one pass to prove the pending generation, got [%d] "+
					"submissions",
				count,
			)
		}
		if reads := fixture.blockCounter.passReads(); reads != 2 {
			t.Fatalf(
				"expected exactly one pass (two chain tip reads) before the "+
					"cancel is honoured, got [%d] reads",
				reads,
			)
		}
	})

	t.Run("cancelled during a pass", func(t *testing.T) {
		fixture := newReservationProofLoopFixture(t)

		reservationKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(6),
			150000,
			tbtc.ReservationActionStatePending,
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		fixture.recordAcceptanceSubmissions(func(*big.Int) error {
			// Someone cancels the maintainer while its pass is running, as a
			// shutdown does.
			cancel()
			return nil
		})

		// The idle backoff is long so that only the cancelled context can end
		// the loop: a short backoff would race with the cancellation in the
		// select and could let another pass start.
		done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Minute))
		waitForMaintainerReturn(t, done)

		if count := fixture.submissions.count(reservationKey); count != 1 {
			t.Fatalf(
				"expected the cancellation to end the loop after the running "+
					"pass instead of allowing another one, got [%d] submissions",
				count,
			)
		}
		if reads := fixture.blockCounter.passReads(); reads != 2 {
			t.Fatalf(
				"expected the loop to stop after the pass it was cancelled "+
					"during (two chain tip reads), got [%d] reads",
				reads,
			)
		}
	})
}

// TestMaintainReservationProofs_DoesNotResubmitSettledGenerations proves the
// loop drops settled generations instead of proving them again: one already
// settled before the first pass is never submitted and never re-read, and one
// that settles after its proof lands is submitted exactly once.
func TestMaintainReservationProofs_DoesNotResubmitSettledGenerations(t *testing.T) {
	t.Run("settled before the first pass", func(t *testing.T) {
		fixture := newReservationProofLoopFixture(t)
		fixture.recordAcceptanceSubmissions(nil)

		pendingKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(7),
			150000,
			tbtc.ReservationActionStatePending,
		)
		settledKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(8),
			160000,
			tbtc.ReservationActionStateSettled,
		)
		supersededKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(9),
			170000,
			tbtc.ReservationActionStateSuperseded,
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Millisecond))

		// Four chain tip reads complete two passes: the settled generations
		// had their second chance to be submitted by then.
		waitForProofLoopCondition(
			t,
			func() bool { return fixture.blockCounter.passReads() >= 4 },
			"the proof loop to complete two passes",
		)

		cancel()
		waitForMaintainerReturn(t, done)

		if count := fixture.submissions.count(pendingKey); count < 1 {
			t.Fatalf(
				"expected the pending generation to be proved, got [%d] "+
					"submissions",
				count,
			)
		}
		for _, settled := range []*big.Int{settledKey, supersededKey} {
			if count := fixture.submissions.count(settled); count != 0 {
				t.Fatalf(
					"expected no proof for the non-settleable generation [%v], "+
						"got [%d] submissions",
					settled,
					count,
				)
			}
			if reads := fixture.chain.actionReadCount(
				settled,
				reservationAcceptanceTestNonce,
			); reads != 1 {
				t.Fatalf(
					"expected the non-settleable generation [%v] to be read "+
						"once and then dropped from the loop's state, got [%d] reads",
					settled,
					reads,
				)
			}
		}
	})

	t.Run("settled after its proof is submitted", func(t *testing.T) {
		fixture := newReservationProofLoopFixture(t)

		reservationKey := fixture.addAcceptanceGeneration(
			t,
			walletPublicKeyHashFor(10),
			150000,
			tbtc.ReservationActionStatePending,
		)

		// The Bridge settles the generation as soon as the proof lands.
		fixture.recordAcceptanceSubmissions(func(reservationKey *big.Int) error {
			fixture.chain.setReservationAction(
				reservationKey,
				reservationAcceptanceTestNonce,
				&tbtc.ReservationAction{
					State:                     tbtc.ReservationActionStateSettled,
					ActionType:                tbtc.ReservationActionTypeAcceptance,
					TargetWalletPublicKeyHash: walletPublicKeyHashFor(10),
					TermSeconds:               100,
					MinAmount:                 1000,
				},
			)

			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := fixture.runMaintainer(ctx, maintainerConfig(time.Hour, time.Millisecond))

		waitForProofLoopCondition(
			t,
			func() bool { return fixture.blockCounter.passReads() >= 4 },
			"the proof loop to complete two passes after the submission",
		)

		cancel()
		waitForMaintainerReturn(t, done)

		if count := fixture.submissions.count(reservationKey); count != 1 {
			t.Fatalf(
				"expected the settled generation to be proved exactly once, "+
					"got [%d] submissions",
				count,
			)
		}

		// The action is read once to classify the generation, once to submit
		// its proof and once on the next pass to notice the settle; after that
		// the generation is dropped. A loop that kept a settled generation
		// would read it again on every pass, and would keep retrying a
		// submission the Bridge can no longer settle.
		if reads := fixture.chain.actionReadCount(
			reservationKey,
			reservationAcceptanceTestNonce,
		); reads != 3 {
			t.Fatalf(
				"expected the settled generation to be read while it was "+
					"proved and once more to notice the settle (three times), "+
					"then dropped from the loop's state, got [%d] reads",
				reads,
			)
		}
	})
}

// proofLoopBlockCounter must remain installable as the chain's block counter,
// and proofLoopTestChain must remain usable wherever the maintainer expects a
// Chain; both assertions are compile-time.
var _ chain.BlockCounter = (*proofLoopBlockCounter)(nil)

var _ Chain = (*proofLoopTestChain)(nil)
