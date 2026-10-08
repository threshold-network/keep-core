package spv

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofItemErrorFixture builds a local chain + Bitcoin chain with
// a single confirmed, provable transaction, and wires the submit hooks so a
// test can force the proof pass to fail per-item (a failed submit) or succeed.
func reservationProofItemErrorFixture(
	t *testing.T,
) (
	*localChain,
	*reservationProofBitcoinChain,
	Config,
	*reservationProofScanState,
	*big.Int,
	uint64,
) {
	t.Helper()
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	if err := populateBlockHeaders(
		btcChain.localBitcoinChain,
		proofStart,
		proofStart+19,
		func(uint) *big.Int { return diff(32) },
	); err != nil {
		t.Fatal(err)
	}
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: 150000}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}
	fundingTxHash := fundingTx.Hash()
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)
	const requestNonce = 1

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	anchorScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
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
			PublicKeyScript: anchorScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(anchorTx.Hash(), 20); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(anchorTx.Hash())

	spvChain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: walletPublicKeyHash,
		DepositAmount:       150000,
		BlockNumber:         500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)

	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	scanState := newReservationProofScanState()

	return spvChain, btcChain, config, scanState, reservationKey, requestNonce
}

// TestProveReservationAcceptanceActions_AggregatesItemErrors proves that a
// single failed acceptance generation must not block the pass from
// reporting the skip: when the submitter fails, the pass returns the
// per-item error (not nil, not a scan error) so the caller can idle-back
// off and retry it on the next pass. Before the fix this returned nil,
// silently dropping the failed item.
func TestProveReservationAcceptanceActions_AggregatesItemErrors(t *testing.T) {
	spvChain, btcChain, config, scanState, reservationKey, requestNonce :=
		reservationProofItemErrorFixture(t)

	submissions := 0
	spvChain.submitReservationAcceptanceProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		_ *big.Int,
		_ uint64,
	) error {
		submissions++
		return fmt.Errorf("simulated gas provider drop on first submit")
	}

	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		newProofInfoCache(),
		nil,
	); err == nil {
		t.Fatal("expected the failed submission to surface as a per-item error, got nil")
	} else {
		// The failed item must still be tracked for the next pass; the
		// aggregate error is not a scan error (chunkScanError), it is the
		// spvItemErrors aggregate.
		if _, isItemErrs := err.(spvItemErrors); !isItemErrs {
			t.Fatalf(
				"expected the per-item aggregate (spvItemErrors), got %T: %v",
				err,
				err,
			)
		}
		if !containsSubError(err.Error(), "simulated gas provider drop") {
			t.Fatalf("error should name the failed submission: %s", err.Error())
		}
	}

	if submissions != 1 {
		t.Fatalf("expected exactly one submission attempt, got [%d]", submissions)
	}

	// The event must remain tracked for the next pass - the pass saw the
	// item fail and left it to be retried, rather than evicting it.
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
		t.Fatal("the failed event must remain pending so the next pass retries it")
	}
}

// TestProveReservationAcceptanceActions_KeepsProvingOtherItems drives the
// pass with two tracked generations where the first one's proof fails and
// the second succeeds. The fix must not let the first failure skip the
// second: both items are still visited in the same pass, and only the
// failing item is reported.
func TestProveReservationAcceptanceActions_KeepsProvingOtherItems(t *testing.T) {
	spvChain, btcChain, config, scanState, _, _ :=
		reservationProofItemErrorFixture(t)

	// A second tracked generation for a distinct reservation key, whose
	// proof succeeds.
	secondFunding := &bitcoin.Transaction{
		// A distinct output value so the second funding transaction's
		// hash differs from the fixture's; otherwise the local chain
		// rejects the duplicate.
		Outputs: []*bitcoin.TransactionOutput{{Value: 250000}},
	}
	if err := btcChain.BroadcastTransaction(secondFunding); err != nil {
		t.Fatal(err)
	}
	secondKey := spvChain.BuildDepositKey(secondFunding.Hash(), 0)
	secondWallet := [20]byte{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}
	secondScript, err := bitcoin.PayToWitnessPublicKeyHash(secondWallet)
	if err != nil {
		t.Fatal(err)
	}
	secondAnchor := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: secondFunding.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: secondScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(secondAnchor); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(secondAnchor.Hash(), 20); err != nil {
		t.Fatal(err)
	}
	spvChain.setDepositRequest(secondFunding.Hash(), 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      secondKey,
		RequestNonce:        1,
		WalletPublicKeyHash: secondWallet,
		DepositAmount:       150000,
		BlockNumber:         500,
	})
	spvChain.setReservationAction(
		secondKey,
		1,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: secondWallet,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)

	// The first reservation's submit fails; the second's succeeds.
	var firstKey *big.Int
	submissions := 0
	spvChain.submitReservationAcceptanceProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		_ uint64,
	) error {
		submissions++
		if firstKey == nil {
			firstKey = reservationKey
		}
		// Fail only the first distinct reservation, prove the rest.
		if reservationKey.Cmp(firstKey) == 0 && submissions == 1 {
			return fmt.Errorf("first reservation submit failed")
		}
		return nil
	}

	passErr := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		newProofInfoCache(),
		nil,
	)
	if passErr == nil {
		t.Fatal("expected the failing reservation to surface as a per-item error")
	}
	// Both reservations must have been visited: the failing one is
	// reported, but the pass must still have attempted the second.
	if submissions < 2 {
		t.Fatalf(
			"expected the pass to visit both generations (>=2 submissions), got [%d]; "+
				"a failing item must not skip the others",
			submissions,
		)
	}
}

// TestProveReservationReanchorActions_AggregatesItemErrors proves the
// same rule for the re-anchor pass: a failed re-anchor submission is
// returned as the per-item aggregate so the next pass retries it, instead
// of being silently dropped.
func TestProveReservationReanchorActions_AggregatesItemErrors(t *testing.T) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()
	if err := populateBlockHeaders(
		btcChain.localBitcoinChain,
		proofStart,
		proofStart+19,
		func(uint) *big.Int { return diff(32) },
	); err != nil {
		t.Fatal(err)
	}
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(424242)
	const requestNonce = 2

	priorAnchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: 10000}, {Value: 600000}},
	}
	if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
		t.Fatal(err)
	}
	anchorTxHash := priorAnchorTx.Hash()
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	sourceWalletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	transaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(transaction.Hash(), 20); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(transaction.Hash())

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
		TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
		BlockNumber:               500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{AnchorUtxo: anchorUtxo})

	submissions := 0
	spvChain.submitReservationReanchorProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		_ *big.Int,
		_ uint64,
	) error {
		submissions++
		return fmt.Errorf("simulated re-anchor submit drop")
	}

	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	scanState := newReservationProofScanState()

	if err := proveReservationReanchorActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		newProofInfoCache(),
		nil,
	); err == nil {
		t.Fatal("expected the failed re-anchor submission to surface as a per-item error")
	} else if _, isItemErrs := err.(spvItemErrors); !isItemErrs {
		t.Fatalf(
			"expected the per-item aggregate (spvItemErrors), got %T: %v",
			err,
			err,
		)
	}
	if submissions != 1 {
		t.Fatalf("expected exactly one re-anchor submission attempt, got [%d]", submissions)
	}
}

// TestIsRetryableReservationRoundError pins the caller-side rule that a
// per-item aggregate (or a chunk scan error) is retryable after the idle
// backoff, while a plain chain-wide error is not.
func TestIsRetryableReservationRoundError(t *testing.T) {
	if isRetryableReservationRoundError(nil) {
		t.Fatal("a nil error is not retryable")
	}
	if !isRetryableReservationRoundError(spvItemErrors{fmt.Errorf("one item")}) {
		t.Fatal("a per-item aggregate must be retryable")
	}
	if !isRetryableReservationRoundError(&chunkScanError{err: fmt.Errorf("scan chunk")}) {
		t.Fatal("a chunk scan error must be retryable")
	}
	if isRetryableReservationRoundError(fmt.Errorf("chain-wide failure")) {
		t.Fatal("a plain chain-wide error must not be retryable")
	}
}

// containsSubError is a small helper so the tests can assert on the
// flattened item-error string.
func containsSubError(haystack string, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
