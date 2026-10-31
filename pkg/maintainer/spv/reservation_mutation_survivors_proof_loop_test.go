package spv

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestWalletTransactionsForProof_MutationSurvivors_CacheSlice pins the
// cache-hit slice offsets: a cached entry holds the transaction bodies from
// its own firstIndex onward, so the returned window starts at
// firstIndex - cached.firstIndex.
func TestWalletTransactionsForProof_MutationSurvivors_CacheSlice(t *testing.T) {
	btcChain := newReservationProofBitcoinChain()
	state := newReservationProofScanState()

	walletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	for i := uint32(0); i < 6; i++ {
		transaction := &bitcoin.Transaction{
			Locktime: i,
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           1000,
				PublicKeyScript: walletScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
	}

	txHashes, err := btcChain.GetTxHashesForPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(txHashes) != 6 {
		t.Fatalf("expected 6 transaction hashes, got %d", len(txHashes))
	}

	// The cache holds the bodies from index 2 onward.
	cachedTransactions := make([]*bitcoin.Transaction, 0, 4)
	for i := uint32(2); i < 6; i++ {
		transaction, err := btcChain.GetTransaction(txHashes[i])
		if err != nil {
			t.Fatal(err)
		}
		cachedTransactions = append(cachedTransactions, transaction)
	}
	state.walletTransactionCache[walletPublicKeyHash] = &walletTransactionCacheEntry{
		txHashes:     txHashes,
		firstIndex:   2,
		transactions: cachedTransactions,
	}

	// limit 4 puts firstIndex at 2, exactly the cached firstIndex, so the
	// whole cached body slice is the answer.
	transactions, err := walletTransactionsForProof(
		state,
		btcChain,
		walletPublicKeyHash,
		4,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transactions) != len(cachedTransactions) {
		t.Fatalf(
			"expected the %d cached transactions from the cache's firstIndex, got %d",
			len(cachedTransactions),
			len(transactions),
		)
	}
	for i, transaction := range transactions {
		if transaction.Hash() != cachedTransactions[i].Hash() {
			t.Fatalf(
				"unexpected transaction at position %d: got %s, want %s",
				i,
				transaction.Hash(),
				cachedTransactions[i].Hash(),
			)
		}
	}
}

// TestWalletTransactionsForProof_MutationSurvivors_SinceCursor pins the
// since-transaction cursor: the fetch window is widened back to the
// earliest named transaction, matching the hash exactly.
func TestWalletTransactionsForProof_MutationSurvivors_SinceCursor(t *testing.T) {
	btcChain := newReservationProofBitcoinChain()
	state := newReservationProofScanState()

	walletPublicKeyHash := [20]byte{41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	for i := uint32(0); i < 6; i++ {
		transaction := &bitcoin.Transaction{
			Locktime: i,
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           1000,
				PublicKeyScript: walletScript,
			}},
		}
		if err := btcChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
	}

	txHashes, err := btcChain.GetTxHashesForPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	// limit 4 selects the newest 4 transactions (indices 2..5); naming
	// the transaction at index 1 as "since" widens the window back to it,
	// so 5 transactions (indices 1..5) are returned.
	transactions, err := walletTransactionsForProof(
		state,
		btcChain,
		walletPublicKeyHash,
		4,
		[]bitcoin.Hash{txHashes[1]},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transactions) != 5 {
		t.Fatalf(
			"expected the window to widen back to the since transaction "+
				"(5 transactions), got %d",
			len(transactions),
		)
	}
	if transactions[0].Hash() != txHashes[1] {
		t.Fatalf(
			"expected the window to start at the since transaction, got %s",
			transactions[0].Hash(),
		)
	}
}

// TestRecordReservationProofMetrics_TypedNilPerformanceMetrics guards the
// proof-loop metric helpers against a typed-nil *clientinfo.PerformanceMetrics
// recorder boxed into the MetricsRecorder interface: the interface value is
// not nil, so without the guard the recorder call dereferences a nil pointer
// and panics the proof loop.
func TestRecordReservationProofMetrics_TypedNilPerformanceMetrics(t *testing.T) {
	var typedNilMetrics *clientinfo.PerformanceMetrics
	var recorder MetricsRecorder = typedNilMetrics

	recordReservationProofRetry(recorder)   // must not panic
	recordReservationProofSuccess(recorder) // must not panic

	t.Run("a live recorder records the metrics", func(t *testing.T) {
		live := &recordingMetricsRecorder{counters: make(map[string]float64)}
		recordReservationProofRetry(live)
		if got := live.Counter(clientinfo.MetricSpvReservationProofsRetriedTotal); got != 1 {
			t.Fatalf("expected one proof retry increment, got %v", got)
		}

		recordReservationProofSuccess(live)
		if got := live.Gauge(clientinfo.MetricSpvReservationProofLastSuccessTimestamp); got <= 0 {
			t.Fatalf("expected the last-success timestamp gauge to be set, got %v", got)
		}
	})
}

// acceptanceTransactionFixture builds one acceptance-request event and a
// matching base transaction for the acceptance matcher tests.
type acceptanceTransactionFixture struct {
	spvChain  *localChain
	event     *tbtc.ReservationAcceptanceRequestedEvent
	fundingTx bitcoin.Hash
	script    []byte
}

func newAcceptanceTransactionFixture(t *testing.T) *acceptanceTransactionFixture {
	t.Helper()

	spvChain := newLocalChain()

	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyHash := [20]byte{71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	return &acceptanceTransactionFixture{
		spvChain:  spvChain,
		fundingTx: fundingTxHash,
		script:    walletScript,
		event: &tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      spvChain.BuildDepositKey(fundingTxHash, 0),
			WalletPublicKeyHash: walletPublicKeyHash,
		},
	}
}

func (f *acceptanceTransactionFixture) transaction(outputValue int64) *bitcoin.Transaction {
	return &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: f.fundingTx,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           outputValue,
			PublicKeyScript: f.script,
		}},
	}
}

// TestIsMatchingReservationAcceptanceTransaction_MutationSurvivors_FeeAndAmount
// pins the acceptance matcher's value checks: the int64 casts are bounded
// inclusively at math.MaxInt64 (an amount exactly at the bound is
// representable), the fee must be strictly positive, and the fee bound is
// checked only when it is set, inclusively at the bound.
func TestIsMatchingReservationAcceptanceTransaction_MutationSurvivors_FeeAndAmount(
	t *testing.T,
) {
	t.Run("deposit request amount exactly MaxInt64 is representable", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.spvChain.setDepositRequest(
			fixture.fundingTx,
			0,
			&tbtc.DepositChainRequest{Amount: math.MaxInt64},
		)
		fixture.event.TxMaxFee = 0

		if !isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(1000),
		) {
			t.Fatal("expected an amount exactly at MaxInt64 to be accepted")
		}
	})

	t.Run("event deposit amount exactly MaxInt64 is representable", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = math.MaxInt64
		fixture.event.TxMaxFee = 0

		if !isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(1000),
		) {
			t.Fatal("expected an amount exactly at MaxInt64 to be accepted")
		}
	})

	t.Run("a positive fee with no bound is accepted", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 150_000
		fixture.event.TxMaxFee = 0

		if !isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a positive fee with no fee bound to be accepted")
		}
	})

	t.Run("a zero fee is rejected", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 100_000
		fixture.event.TxMaxFee = 0

		if isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a zero fee to be rejected")
		}
	})

	t.Run("a fee exactly at the bound is accepted", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 160_000
		fixture.event.TxMaxFee = 60_000

		if !isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a fee exactly at the fee bound to be accepted")
		}
	})

	t.Run("a fee above the bound is rejected", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 170_000
		fixture.event.TxMaxFee = 60_000

		if isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a fee above the fee bound to be rejected")
		}
	})

	t.Run("an output above the deposit amount is rejected without a bound", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 100_000
		fixture.event.TxMaxFee = 0

		if isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(150_000),
		) {
			t.Fatal("expected a negative fee to be rejected")
		}
	})

	t.Run("an output above the deposit amount is rejected with a bound", func(t *testing.T) {
		fixture := newAcceptanceTransactionFixture(t)
		fixture.event.DepositAmount = 300_000
		fixture.event.TxMaxFee = 60_000

		if isMatchingReservationAcceptanceTransaction(
			fixture.spvChain,
			fixture.event,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a fee above the fee bound to be rejected")
		}
	})
}

// reanchorTransactionFixture builds one re-anchor event, the anchor UTXO it
// spends and a matching re-anchor transaction.
type reanchorTransactionFixture struct {
	event     *tbtc.ReservationReanchorRequestedEvent
	anchor    *bitcoin.UnspentTransactionOutput
	script    []byte
	anchorTx  bitcoin.Hash
	targetPKH [20]byte
}

func newReanchorTransactionFixture(
	t *testing.T,
	anchorValue int64,
) *reanchorTransactionFixture {
	t.Helper()

	anchorTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	targetPKH := [20]byte{91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110}
	script, err := bitcoin.PayToWitnessPublicKeyHash(targetPKH)
	if err != nil {
		t.Fatal(err)
	}

	return &reanchorTransactionFixture{
		event: &tbtc.ReservationReanchorRequestedEvent{
			TargetWalletPublicKeyHash: targetPKH,
		},
		anchor: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
			Value: anchorValue,
		},
		script:    script,
		anchorTx:  anchorTxHash,
		targetPKH: targetPKH,
	}
}

func (f *reanchorTransactionFixture) transaction(
	outputValue int64,
) *bitcoin.Transaction {
	return &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: f.anchorTx,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           outputValue,
			PublicKeyScript: f.script,
		}},
	}
}

// TestIsMatchingReservationReanchorTransaction_MutationSurvivors_FeeAndDustFloor
// pins the re-anchor matcher's value checks: a fee exactly at the fee bound
// is accepted, and the settlement dust floor is skipped when no fee bound is
// snapshotted.
func TestIsMatchingReservationReanchorTransaction_MutationSurvivors_FeeAndDustFloor(
	t *testing.T,
) {
	t.Run("a fee exactly at the bound is accepted", func(t *testing.T) {
		fixture := newReanchorTransactionFixture(t, 160_000)
		fixture.event.TxMaxFee = 60_000

		if !isMatchingReservationReanchorTransaction(
			fixture.event,
			fixture.anchor,
			fixture.transaction(100_000),
		) {
			t.Fatal("expected a fee exactly at the fee bound to be accepted")
		}
	})

	t.Run("no fee bound skips the dust floor", func(t *testing.T) {
		fixture := newReanchorTransactionFixture(t, 100_000)
		fixture.event.TxMaxFee = 0

		if !isMatchingReservationReanchorTransaction(
			fixture.event,
			fixture.anchor,
			fixture.transaction(0),
		) {
			t.Fatal(
				"expected the dust floor to be skipped when the event carries " +
					"no fee bound",
			)
		}
	})
}

// TestReservationActionAbandoned_MutationSurvivors_EqualTimestamps pins the
// abandonment guard: a generation whose requestedAt equals its timeoutAt has
// a zero scrutinee window and is abandoned once the chain head passes the
// timeout, so the guard must only reject requestedAt strictly after
// timeoutAt.
func TestReservationActionAbandoned_MutationSurvivors_EqualTimestamps(t *testing.T) {
	action := &tbtc.ReservationAction{
		RequestedAt: 100,
		TimeoutAt:   100,
	}

	if !reservationActionAbandoned(action, time.Unix(200, 0)) {
		t.Fatal(
			"expected a generation with requestedAt == timeoutAt to be " +
				"abandoned once the head passed the timeout",
		)
	}
}

// TestSortReservationProofGenerations_MutationSurvivors_PendingFirst pins the
// sort order: within one reservation key the Pending generation is proved
// before any TimedOut generation, regardless of nonce order.
func TestSortReservationProofGenerations_MutationSurvivors_PendingFirst(t *testing.T) {
	key := big.NewInt(0xC200)
	type generation = reservationProofGeneration[*tbtc.ReservationAcceptanceRequestedEvent]

	generations := []generation{
		{key: "timed-out", reservationKey: key, requestNonce: 2, pending: false},
		{key: "pending", reservationKey: key, requestNonce: 1, pending: true},
	}

	sortReservationProofGenerations(generations)

	if generations[0].key != "pending" {
		t.Fatalf(
			"expected the Pending generation to sort first, got %q",
			generations[0].key,
		)
	}

	t.Run("distinct reservation keys sort ascending", func(t *testing.T) {
		lowKey := big.NewInt(0xC201)
		highKey := big.NewInt(0xC202)

		generations := []generation{
			{key: "high", reservationKey: highKey, requestNonce: 1},
			{key: "low", reservationKey: lowKey, requestNonce: 1},
		}

		sortReservationProofGenerations(generations)

		if generations[0].key != "low" {
			t.Fatalf(
				"expected the lower reservation key to sort first, got %q",
				generations[0].key,
			)
		}
	})
}

// TestProveReservationTransaction_MutationSurvivors_AtRequiredConfirmations
// pins the confirmation comparison as inclusive: a transaction with exactly
// the required number of confirmations is provable.
func TestProveReservationTransaction_MutationSurvivors_AtRequiredConfirmations(
	t *testing.T,
) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	transaction := &bitcoin.Transaction{}
	transactionHash := transaction.Hash()

	spvChain := newLocalChain()
	btcChain := newLocalBitcoinChain()

	if err := populateBlockHeaders(
		btcChain,
		proofStart,
		proofStart+19,
		func(uint) *big.Int { return diff(32) },
	); err != nil {
		t.Fatal(err)
	}
	// The required confirmation count for this fixture is 6 (see
	// TestProveReservationTransaction); 6 accumulated confirmations sit
	// exactly on the boundary.
	const requiredConfirmations = 6
	btcChain.addTransactionConfirmations(transactionHash, requiredConfirmations)

	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	submitted := false
	outcome, err := proveReservationTransaction(
		[]*bitcoin.Transaction{transaction},
		alwaysMatchReservationTransaction,
		btcChain,
		spvChain,
		spvChain,
		DefaultMaxProofHeaders,
		newProofInfoCache(),
		nil,
		func(hash bitcoin.Hash, required uint) error {
			submitted = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !submitted {
		t.Fatal("expected a transaction exactly at the required confirmations to be submitted")
	}
	if outcome != reservationProofSubmitted {
		t.Errorf("expected outcome %v, got %v", reservationProofSubmitted, outcome)
	}
}
