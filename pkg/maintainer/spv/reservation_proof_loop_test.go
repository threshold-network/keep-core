package spv

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofBitcoinChain wraps localBitcoinChain (defined in
// bitcoin_chain_test.go, shared spv-package test infrastructure) with a
// working GetTxHashesForPublicKeyHash, so tests that exercise
// walletTransactionsForProof's wallet-history cache through the full
// proveReservationAcceptanceActions/proveReservationReanchorActions
// pipeline don't hit localBitcoinChain's own GetTxHashesForPublicKeyHash,
// which is an unimplemented panic stub unused by any other test in the
// package.
type reservationProofBitcoinChain struct {
	*localBitcoinChain

	// getTransactionCalls counts calls that reach the embedded chain's
	// single-transaction body fetch, letting tests assert
	// walletTransactionsForProof's cache actually skips fetching bodies
	// when nothing changed for a wallet since the previous pass.
	getTransactionCalls int
}

func newReservationProofBitcoinChain() *reservationProofBitcoinChain {
	return &reservationProofBitcoinChain{localBitcoinChain: newLocalBitcoinChain()}
}

// GetTransaction counts each call reaching the embedded chain's
// single-transaction body fetch; see getTransactionCalls.
func (c *reservationProofBitcoinChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	c.getTransactionCalls++
	return c.localBitcoinChain.GetTransaction(transactionHash)
}

// GetTxHashesForPublicKeyHash derives hashes from the same confirmed
// transaction set GetTransactionsForPublicKeyHash serves, so the two stay
// consistent for change detection in walletTransactionsForProof. It calls
// the embedded chain directly, bypassing the counted override above, since
// production code uses this lightweight hash fetch specifically to decide
// whether the full-body fetch is needed at all.
func (c *reservationProofBitcoinChain) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]bitcoin.Hash, error) {
	transactions, err := c.localBitcoinChain.GetTransactionsForPublicKeyHash(publicKeyHash, math.MaxInt)
	if err != nil {
		return nil, err
	}

	hashes := make([]bitcoin.Hash, len(transactions))
	for i, transaction := range transactions {
		hashes[i] = transaction.Hash()
	}

	return hashes, nil
}

// TestFindReservationAcceptanceTransaction verifies the acceptance
// transaction matcher: it must find the 1-input-1-output transaction whose
// sole input spends the deposit UTXO identified by event.ReservationKey (via
// BuildDepositKey), whose sole output is P2WPKH to the custody wallet,
// and whose value is depositAmount - fee (with fee <= TxMaxFee), skip
// transactions with wrong shape, wrong script, or invalid value, and return nil
// when nothing matches.
func TestFindReservationAcceptanceTransaction(t *testing.T) {
	spvChain := newLocalChain()

	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	otherWalletPKH := [20]byte{99, 99, 99}
	otherWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(otherWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	spvChain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})

	matchingTx := &bitcoin.Transaction{
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

	// Wrong script: pays to a different wallet.
	wrongScriptTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: otherWalletScript,
		}},
	}

	// Wrong value: output value >= depositAmount (zero or negative fee).
	wrongValueTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           160000,
			PublicKeyScript: walletScript,
		}},
	}

	// Excess fee: fee 100000 > TxMaxFee 60000.
	excessFeeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           50000,
			PublicKeyScript: walletScript,
		}},
	}

	// Wrong shape: two outputs, must be skipped even though it otherwise
	// spends the right outpoint.
	wrongShapeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 50000, PublicKeyScript: walletScript},
			{Value: 50000, PublicKeyScript: walletScript},
		},
	}

	// Non-matching: correct shape, different outpoint.
	otherTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	nonMatchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: otherTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: walletScript,
		}},
	}

	event := &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		WalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:            60000,
	}
	findMatchingTx := func(candidates []*bitcoin.Transaction) *bitcoin.Transaction {
		candidateTransactions := make(map[string]*bitcoin.Transaction)
		for _, transaction := range candidates {
			if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
				input := transaction.Inputs[0]
				depositKey := spvChain.BuildDepositKey(
					input.Outpoint.TransactionHash,
					input.Outpoint.OutputIndex,
				)
				candidateTransactions[depositKey.String()] = transaction
			}
		}

		if transaction, ok := candidateTransactions[event.ReservationKey.String()]; ok {
			if isMatchingReservationAcceptanceTransaction(spvChain, event, transaction) {
				return transaction
			}
		}

		return nil
	}

	t.Run("finds the matching transaction among candidates", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongShapeTx, nonMatchingTx, matchingTx})
		if found != matchingTx {
			t.Errorf("expected to find the matching transaction, got %v", found)
		}
	})

	t.Run("returns nil when nothing matches", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongShapeTx, nonMatchingTx})
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})

	t.Run("returns nil for an empty candidate list", func(t *testing.T) {
		found := findMatchingTx(nil)
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})

	t.Run("skips transaction with wrong output script", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongScriptTx})
		if found != nil {
			t.Errorf("expected nil for wrong script transaction, got %v", found)
		}
		if isMatchingReservationAcceptanceTransaction(spvChain, event, wrongScriptTx) {
			t.Errorf("expected isMatchingReservationAcceptanceTransaction to be false")
		}
	})

	t.Run("skips transaction with wrong output value", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongValueTx})
		if found != nil {
			t.Errorf("expected nil for wrong value transaction, got %v", found)
		}
		if isMatchingReservationAcceptanceTransaction(spvChain, event, wrongValueTx) {
			t.Errorf("expected isMatchingReservationAcceptanceTransaction to be false")
		}
	})

	t.Run("skips transaction with excess fee", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{excessFeeTx})
		if found != nil {
			t.Errorf("expected nil for excess fee transaction, got %v", found)
		}
		if isMatchingReservationAcceptanceTransaction(spvChain, event, excessFeeTx) {
			t.Errorf("expected isMatchingReservationAcceptanceTransaction to be false")
		}
	})

	t.Run("accepts a P2PKH output paying the authorized wallet", func(t *testing.T) {
		p2pkhScript, err := bitcoin.PayToPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}
		p2pkhTx := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTxHash,
					OutputIndex:     0,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           100000,
				PublicKeyScript: p2pkhScript,
			}},
		}
		if !isMatchingReservationAcceptanceTransaction(spvChain, event, p2pkhTx) {
			t.Error("expected a P2PKH output paying the authorized wallet to match")
		}
	})

	t.Run("skips a P2PKH output paying a different wallet", func(t *testing.T) {
		otherP2pkhScript, err := bitcoin.PayToPublicKeyHash(otherWalletPKH)
		if err != nil {
			t.Fatal(err)
		}
		otherP2pkhTx := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTxHash,
					OutputIndex:     0,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           100000,
				PublicKeyScript: otherP2pkhScript,
			}},
		}
		if isMatchingReservationAcceptanceTransaction(spvChain, event, otherP2pkhTx) {
			t.Error("expected a P2PKH output paying a different wallet to be skipped")
		}
	})
}

// TestFindReservationReanchorTransaction verifies the re-anchor transaction
// matcher: it must find the 1-input-1-output transaction whose sole input
// spends the reservation's current anchor UTXO outpoint exactly, whose sole output
// is P2WPKH to the target wallet, and whose value is anchorUtxo.Value - fee
// (with fee <= TxMaxFee), skip wrong-shape, wrong-script, or invalid-value
// transactions, and return nil when nothing matches.
func TestFindReservationReanchorTransaction(t *testing.T) {
	anchorTxHash, err := bitcoin.NewHashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	targetWalletPKH := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	targetWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPKH)
	if err != nil {
		t.Fatal(err)
	}

	otherPKH := [20]byte{99, 99, 99}
	otherScript, err := bitcoin.PayToWitnessPublicKeyHash(otherPKH)
	if err != nil {
		t.Fatal(err)
	}

	matchingTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	wrongScriptTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: otherScript,
		}},
	}

	wrongValueTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	excessFeeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           500000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	// Same transaction hash, wrong output index: must not match.
	wrongIndexTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetWalletScript,
		}},
	}

	wrongShapeTx := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 300000, PublicKeyScript: targetWalletScript},
			{Value: 290000, PublicKeyScript: targetWalletScript},
		},
	}

	event := &tbtc.ReservationReanchorRequestedEvent{
		TargetWalletPublicKeyHash: targetWalletPKH,
		TxMaxFee:                  20000,
	}

	findMatchingTx := func(candidates []*bitcoin.Transaction) *bitcoin.Transaction {
		candidateTransactions := make(map[bitcoin.TransactionOutpoint]*bitcoin.Transaction)
		for _, transaction := range candidates {
			if len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1 && transaction.Inputs[0].Outpoint != nil {
				candidateTransactions[*transaction.Inputs[0].Outpoint] = transaction
			}
		}

		if transaction, ok := candidateTransactions[*anchorUtxo.Outpoint]; ok {
			if isMatchingReservationReanchorTransaction(event, anchorUtxo, transaction) {
				return transaction
			}
		}

		return nil
	}

	t.Run("finds the matching transaction among candidates", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongShapeTx, wrongIndexTx, matchingTx})
		if found != matchingTx {
			t.Errorf("expected to find the matching transaction, got %v", found)
		}
	})

	t.Run("returns nil when nothing matches", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongShapeTx, wrongIndexTx})
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})

	t.Run("skips transaction with wrong output script", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongScriptTx})
		if found != nil {
			t.Errorf("expected nil for wrong script transaction, got %v", found)
		}
		if isMatchingReservationReanchorTransaction(event, anchorUtxo, wrongScriptTx) {
			t.Errorf("expected isMatchingReservationReanchorTransaction to be false")
		}
	})

	t.Run("skips transaction with wrong output value", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{wrongValueTx})
		if found != nil {
			t.Errorf("expected nil for wrong value transaction, got %v", found)
		}
		if isMatchingReservationReanchorTransaction(event, anchorUtxo, wrongValueTx) {
			t.Errorf("expected isMatchingReservationReanchorTransaction to be false")
		}
	})

	t.Run("skips transaction with excess fee", func(t *testing.T) {
		found := findMatchingTx([]*bitcoin.Transaction{excessFeeTx})
		if found != nil {
			t.Errorf("expected nil for excess fee transaction, got %v", found)
		}
		if isMatchingReservationReanchorTransaction(event, anchorUtxo, excessFeeTx) {
			t.Errorf("expected isMatchingReservationReanchorTransaction to be false")
		}
	})

	t.Run("accepts a P2PKH output paying the target wallet", func(t *testing.T) {
		p2pkhScript, err := bitcoin.PayToPublicKeyHash(targetWalletPKH)
		if err != nil {
			t.Fatal(err)
		}
		p2pkhTx := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: anchorTxHash,
					OutputIndex:     1,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           590000,
				PublicKeyScript: p2pkhScript,
			}},
		}
		if !isMatchingReservationReanchorTransaction(event, anchorUtxo, p2pkhTx) {
			t.Error("expected a P2PKH output paying the target wallet to match")
		}
	})

	t.Run("skips a P2PKH output paying a different wallet", func(t *testing.T) {
		otherP2pkhScript, err := bitcoin.PayToPublicKeyHash(otherPKH)
		if err != nil {
			t.Fatal(err)
		}
		otherP2pkhTx := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: anchorTxHash,
					OutputIndex:     1,
				},
			}},
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           590000,
				PublicKeyScript: otherP2pkhScript,
			}},
		}
		if isMatchingReservationReanchorTransaction(event, anchorUtxo, otherP2pkhTx) {
			t.Error("expected a P2PKH output paying a different wallet to be skipped")
		}
	})
}

// alwaysMatchReservationTransaction is an isMatch predicate that accepts
// every candidate, used by tests that exercise proveReservationTransaction's
// confirmation/skip-reason handling directly and don't care about shape
// matching.
func alwaysMatchReservationTransaction(*bitcoin.Transaction) bool {
	return true
}

// TestProveReservationTransaction covers the submit-vs-skip decision: a
// transaction with enough confirmations and a proof within relay range must
// invoke submit exactly once; a transaction with too few confirmations must
// not invoke submit at all.
func TestProveReservationTransaction(t *testing.T) {
	// Fixture mirrors TestGetProofInfo's "proof entirely within current
	// epoch" case in spv_test.go: factor 6, 20 confirmations, headers
	// spanning the proof window all at the current epoch's difficulty.
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	transaction := &bitcoin.Transaction{}
	transactionHash := transaction.Hash()

	newFixture := func(confirmations uint) (*localChain, *localBitcoinChain) {
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
		btcChain.addTransactionConfirmations(transactionHash, confirmations)

		spvChain.setTxProofDifficultyFactor(big.NewInt(6))
		spvChain.setCurrentEpoch(392)
		spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

		return spvChain, btcChain
	}

	t.Run("submits when confirmations and relay range are sufficient", func(t *testing.T) {
		spvChain, btcChain := newFixture(20)

		submitted := false
		err := proveReservationTransaction(
			[]*bitcoin.Transaction{transaction},
			alwaysMatchReservationTransaction,
			btcChain,
			spvChain,
			spvChain,
			// 0 exercises the zero-fallback that normalizes
			// programmatically-built Config paths to the default bound;
			// without it, getProofInfo would skip with
			// proofSkipExceededMaxHeaders and this submission could never
			// happen.
			0,
			newProofInfoCache(),
			nil,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				submitted = true
				if hash != transaction.Hash() {
					t.Errorf("unexpected submitted hash")
				}
				if requiredConfirmations != 6 {
					t.Errorf(
						"unexpected required confirmations: got %d, want 6",
						requiredConfirmations,
					)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !submitted {
			t.Error("expected submit to be called")
		}
	})

	t.Run("skips without submitting when confirmations are insufficient", func(t *testing.T) {
		spvChain, btcChain := newFixture(2)

		submitted := false
		err := proveReservationTransaction(
			[]*bitcoin.Transaction{transaction},
			alwaysMatchReservationTransaction,
			btcChain,
			spvChain,
			spvChain,
			DefaultMaxProofHeaders,
			newProofInfoCache(),
			nil,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				submitted = true
				return nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if submitted {
			t.Error("expected submit not to be called for insufficient confirmations")
		}
	})

	t.Run("propagates submit errors", func(t *testing.T) {
		spvChain, btcChain := newFixture(20)

		err := proveReservationTransaction(
			[]*bitcoin.Transaction{transaction},
			alwaysMatchReservationTransaction,
			btcChain,
			spvChain,
			spvChain,
			DefaultMaxProofHeaders,
			newProofInfoCache(),
			nil,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				return fmt.Errorf("submission failed")
			},
		)
		if err == nil {
			t.Fatal("expected submit error to propagate")
		}
	})

	t.Run("skips a non-matching candidate to reach a later unconfirmed matching one", func(t *testing.T) {
		// nonMatching is deliberately NOT registered with any confirmations
		// on btcChain: if the loop failed to skip it via isMatch and instead
		// called getProofInfo on it, GetTransactionConfirmations would
		// return "transaction not found" and proveReservationTransaction
		// would return that as an error. A nil error here is therefore
		// direct evidence the non-matching candidate was never examined
		// past the isMatch check, and the walk continued to matching.
		nonMatching := &bitcoin.Transaction{Locktime: 1}
		matching := &bitcoin.Transaction{Locktime: 2}

		spvChain, btcChain := newFixture(2) // insufficient confirmations
		btcChain.addTransactionConfirmations(matching.Hash(), 2)

		isMatch := func(transaction *bitcoin.Transaction) bool {
			return transaction.Hash() == matching.Hash()
		}

		submitted := false
		err := proveReservationTransaction(
			[]*bitcoin.Transaction{nonMatching, matching},
			isMatch,
			btcChain,
			spvChain,
			spvChain,
			DefaultMaxProofHeaders,
			newProofInfoCache(),
			nil,
			func(hash bitcoin.Hash, requiredConfirmations uint) error {
				submitted = true
				return nil
			},
		)
		if err != nil {
			t.Fatalf(
				"unexpected error (a non-nil error here would mean the "+
					"non-matching candidate was processed instead of "+
					"skipped): %v",
				err,
			)
		}
		if submitted {
			t.Error("expected submit not to be called for insufficient confirmations")
		}
	})
}

// TestProveReservationTransaction_RecordsMetrics verifies that the
// reservation path's proof-skip metrics are recorded through a real
// (non-nil) recorder, exercising the same clientinfo.MetricSpvProofSkipped*
// counters as the generic-loop path's TestProveTransactions in spv_test.go.
// Every other reservation-path test in this file runs with
// getMetricsRecorder() == nil, so a wrong metric-name constant on this path
// specifically would otherwise ship silently.
func TestProveReservationTransaction_RecordsMetrics(t *testing.T) {
	const proofStart = 790270

	transaction := &bitcoin.Transaction{}

	tests := map[string]struct {
		headerDifficultyAt       func(uint) *big.Int
		headersTo                uint
		transactionConfirmations uint
		expectedCounter          string
	}{
		// Decisive header (difficulty 8) matches neither epoch -> skipped.
		"outside relay range is skipped and metered": {
			headerDifficultyAt:       func(uint) *big.Int { return big.NewInt(8) },
			headersTo:                proofStart + 19,
			transactionConfirmations: 20,
			expectedCounter:          "spv_proof_skipped_outside_relay_range_total",
		},
		// A run of DIFF1 headers longer than the bound never binds -> skipped.
		"exceeded max headers is skipped and metered": {
			headerDifficultyAt:       func(uint) *big.Int { return big.NewInt(1) },
			headersTo:                proofStart + 149,
			transactionConfirmations: 150,
			expectedCounter:          "spv_proof_skipped_exceeded_max_headers_total",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			spvChain := newLocalChain()
			btcChain := newLocalBitcoinChain()

			if err := populateBlockHeaders(
				btcChain,
				proofStart,
				test.headersTo,
				test.headerDifficultyAt,
			); err != nil {
				t.Fatal(err)
			}
			btcChain.addTransactionConfirmations(
				transaction.Hash(),
				test.transactionConfirmations,
			)

			spvChain.setTxProofDifficultyFactor(big.NewInt(6))
			spvChain.setCurrentEpoch(392)
			spvChain.setCurrentAndPrevEpochDifficulty(big.NewInt(16), big.NewInt(32))

			recorder := &recordingMetricsRecorder{
				counters: make(map[string]float64),
			}

			submitted := false
			if err := proveReservationTransaction(
				[]*bitcoin.Transaction{transaction},
				alwaysMatchReservationTransaction,
				btcChain,
				spvChain,
				spvChain,
				DefaultMaxProofHeaders,
				newProofInfoCache(),
				recorder,
				func(hash bitcoin.Hash, requiredConfirmations uint) error {
					submitted = true
					return nil
				},
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if submitted {
				t.Error("expected no submission on skip")
			}

			if got := recorder.counters[test.expectedCounter]; got != 1 {
				t.Errorf(
					"expected counter [%s] to be 1, got [%v]",
					test.expectedCounter,
					got,
				)
			}
		})
	}
}

// TestProveReservationAcceptanceActions is an end-to-end test of the
// top-level orchestration function wired into production via
// runReservationProofLoop: it seeds a requested event, a matching pending
// action, and a matching wallet transaction, then asserts the submit hook
// fires with the correct (reservationKey, requestNonce) pair on the first pass.
// On a second pass with the on-chain action state mutated to Settled, it
// asserts no second submission occurs and the event is evicted from the pending map.
func TestProveReservationAcceptanceActions(t *testing.T) {
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

	spvChain.setDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Amount: 150000,
	})

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	transaction := &bitcoin.Transaction{
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
	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
	if err := btcChain.addTransactionConfirmations(
		transaction.Hash(),
		20,
	); err != nil {
		t.Fatal(err)
	}
	btcChain.setCoinbaseTxHash(transaction.Hash())

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: walletPublicKeyHash,
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

	var submittedReservationKey *big.Int
	var submittedRequestNonce uint64
	submissions := 0
	spvChain.submitReservationAcceptanceProofHook = func(
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submissions++
		submittedReservationKey = reservationKey
		submittedRequestNonce = requestNonce
		return nil
	}

	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	scanState := newReservationProofScanState()

	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if submissions != 1 {
		t.Fatalf("expected exactly one proof submission, got %d", submissions)
	}
	if submittedReservationKey.Cmp(reservationKey) != 0 {
		t.Errorf(
			"unexpected submitted reservation key\nexpected: %v\nactual:   %v",
			reservationKey,
			submittedReservationKey,
		)
	}
	if submittedRequestNonce != requestNonce {
		t.Errorf(
			"unexpected submitted request nonce\nexpected: %d\nactual:   %d",
			requestNonce,
			submittedRequestNonce,
		)
	}

	// Second pass: action transitions to Settled. Verify it is not resubmitted
	// and is evicted from the pending map.
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStateSettled,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)

	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	if submissions != 1 {
		t.Errorf("expected submissions to remain 1 on second pass, got %d", submissions)
	}
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingAcceptanceEvents[key]; exists {
		t.Errorf("expected settled event to be evicted from pendingAcceptanceEvents")
	}
}

// TestProveReservationReanchorActions is an end-to-end test of the
// top-level orchestration function wired into production via
// runReservationProofLoop: it seeds a requested event, a matching
// reservation with an anchor UTXO, a matching pending action, and a
// matching wallet transaction, then asserts the submit hook fires with the
// correct (reservationKey, requestNonce) pair on the first pass.
// On a second pass with the on-chain action state mutated to Settled, it
// asserts no second submission occurs and the event is evicted from the pending map.
func TestProveReservationReanchorActions(t *testing.T) {
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
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 600000},
		},
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
	if err := btcChain.addTransactionConfirmations(
		transaction.Hash(),
		20,
	); err != nil {
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
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		AnchorUtxo: anchorUtxo,
	})

	var submittedReservationKey *big.Int
	var submittedRequestNonce uint64
	submissions := 0
	spvChain.submitReservationReanchorProofHook = func(
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		submissions++
		submittedReservationKey = reservationKey
		submittedRequestNonce = requestNonce
		return nil
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
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if submissions != 1 {
		t.Fatalf("expected exactly one proof submission, got %d", submissions)
	}
	if submittedReservationKey.Cmp(reservationKey) != 0 {
		t.Errorf(
			"unexpected submitted reservation key\nexpected: %v\nactual:   %v",
			reservationKey,
			submittedReservationKey,
		)
	}
	if submittedRequestNonce != requestNonce {
		t.Errorf(
			"unexpected submitted request nonce\nexpected: %d\nactual:   %d",
			requestNonce,
			submittedRequestNonce,
		)
	}

}

func TestProveReservationAcceptanceActions_LeavesPendingOnChainError(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newLocalBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(12345)
	const requestNonce = 1

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: [20]byte{1},
		BlockNumber:         500,
	})
	spvChain.getReservationActionErr = fmt.Errorf("transient RPC failure")

	scanState := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	key := reservationEventKey(reservationKey, requestNonce)

	// Multiple passes: event must remain pending unconditionally on read error without eviction.
	for i := 1; i <= 5; i++ {
		cache := newProofInfoCache()
		if err := proveReservationAcceptanceActions(
			scanState,
			config,
			spvChain,
			spvChain,
			btcChain,
			cache,
			nil,
		); err != nil {
			t.Fatalf("unexpected error on pass %d: %v", i, err)
		}

		if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
			t.Fatalf("expected event to remain pending on pass %d", i)
		}
	}

	if scanState.acceptanceLastScannedBlock != 1000-reservationEventScanConfirmationBlocks {
		t.Fatalf("expected cursor to advance to confirmed tip %d, got %d", 1000-reservationEventScanConfirmationBlocks, scanState.acceptanceLastScannedBlock)
	}
}

func TestProveReservationReanchorActions_LeavesPendingOnChainError(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newLocalBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(67890)
	const requestNonce = 1

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: [20]byte{2},
		TargetWalletPublicKeyHash: [20]byte{2},
		BlockNumber:               500,
	})
	spvChain.getReservationActionErr = fmt.Errorf("transient RPC failure")

	scanState := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	key := reservationEventKey(reservationKey, requestNonce)

	// Multiple passes: event must remain pending unconditionally on read error without eviction.
	for i := 1; i <= 5; i++ {
		cache := newProofInfoCache()
		if err := proveReservationReanchorActions(
			scanState,
			config,
			spvChain,
			spvChain,
			btcChain,
			cache,
			nil,
		); err != nil {
			t.Fatalf("unexpected error on pass %d: %v", i, err)
		}

		if _, exists := scanState.pendingReanchorEvents[key]; !exists {
			t.Fatalf("expected event to remain pending on pass %d", i)
		}
	}

	if scanState.reanchorLastScannedBlock != 1000-reservationEventScanConfirmationBlocks {
		t.Fatalf("expected cursor to advance to confirmed tip %d, got %d", 1000-reservationEventScanConfirmationBlocks, scanState.reanchorLastScannedBlock)
	}
}

// TestProveReservationTransaction_SelectsConfirmedRBFCandidate is a
// regression test for the RBF (replace-by-fee) candidate-selection bug: a
// still-pending, never-confirming replaced transaction encountered first in
// candidates must not block an already-confirmed later replacement from
// being proved. proveReservationTransaction must walk every candidate and
// prove the first one that is both a shape match and has enough
// confirmations, not assume the first (or only) one considered is correct.
func TestProveReservationTransaction_SelectsConfirmedRBFCandidate(t *testing.T) {
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

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
	spvChain.setTxProofDifficultyFactor(big.NewInt(6))
	spvChain.setCurrentEpoch(392)
	spvChain.setCurrentAndPrevEpochDifficulty(diff(32), diff(16))

	// unconfirmedReplaced is the RBF-replaced transaction; it never
	// accumulates enough confirmations and is seen first in candidate
	// order, mirroring a wallet fetch/map-iteration order that does not
	// match on-chain confirmation order.
	unconfirmedReplaced := &bitcoin.Transaction{Locktime: 1}
	if err := btcChain.addTransactionConfirmations(unconfirmedReplaced.Hash(), 2); err != nil {
		t.Fatal(err)
	}

	// confirmedReplacement is the RBF replacement that actually confirmed.
	confirmedReplacement := &bitcoin.Transaction{Locktime: 2}
	if err := btcChain.addTransactionConfirmations(confirmedReplacement.Hash(), 20); err != nil {
		t.Fatal(err)
	}

	var submittedHash bitcoin.Hash
	submissions := 0
	err := proveReservationTransaction(
		[]*bitcoin.Transaction{unconfirmedReplaced, confirmedReplacement},
		alwaysMatchReservationTransaction,
		btcChain,
		spvChain,
		spvChain,
		DefaultMaxProofHeaders,
		newProofInfoCache(),
		nil,
		func(hash bitcoin.Hash, requiredConfirmations uint) error {
			submissions++
			submittedHash = hash
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if submissions != 1 {
		t.Fatalf("expected exactly one proof submission, got %d", submissions)
	}
	if submittedHash != confirmedReplacement.Hash() {
		t.Errorf(
			"expected the confirmed replacement [%s] to be proved, got [%s]",
			confirmedReplacement.Hash().Hex(bitcoin.ReversedByteOrder),
			submittedHash.Hex(bitcoin.ReversedByteOrder),
		)
	}
}

// TestWalletTransactionsForProof verifies that unchanged transaction hashes
// reuse cached bodies and a changed hash set triggers a refetch: a pass
// whose lightweight GetTxHashesForPublicKeyHash check shows no change for a
// wallet since the previous pass reuses the previous pass's fetched
// transaction bodies instead of fetching them again, and a pass that
// observes a new confirmed transaction for the wallet does refetch.
func TestWalletTransactionsForProof(t *testing.T) {
	btcChain := newReservationProofBitcoinChain()
	state := newReservationProofScanState()

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	firstTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: 1000, PublicKeyScript: walletScript}},
	}
	if err := btcChain.BroadcastTransaction(firstTx); err != nil {
		t.Fatal(err)
	}

	transactions, err := walletTransactionsForProof(state, btcChain, walletPublicKeyHash, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(transactions))
	}
	if btcChain.getTransactionCalls != 1 {
		t.Fatalf(
			"expected 1 transaction body fetch after the first pass, got %d",
			btcChain.getTransactionCalls,
		)
	}

	// Second pass: nothing changed for the wallet. The full-history fetch
	// must be skipped and the previous pass's bodies reused.
	transactions, err = walletTransactionsForProof(state, btcChain, walletPublicKeyHash, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(transactions))
	}
	if btcChain.getTransactionCalls != 1 {
		t.Fatalf(
			"expected transaction body fetches to be skipped when nothing "+
				"changed, but call count is now %d",
			btcChain.getTransactionCalls,
		)
	}

	// Third pass: a new confirmed transaction appears for the wallet. The
	// full-history fetch must run again and observe it.
	secondTx := &bitcoin.Transaction{
		Locktime: 1,
		Outputs:  []*bitcoin.TransactionOutput{{Value: 2000, PublicKeyScript: walletScript}},
	}
	if err := btcChain.BroadcastTransaction(secondTx); err != nil {
		t.Fatal(err)
	}

	transactions, err = walletTransactionsForProof(state, btcChain, walletPublicKeyHash, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transactions) != 2 {
		t.Fatalf("expected 2 transactions after the wallet's history changed, got %d", len(transactions))
	}
	if btcChain.getTransactionCalls != 3 {
		t.Fatalf(
			"expected transaction bodies to be refetched (both hashes) "+
				"after a change, got cumulative call count %d",
			btcChain.getTransactionCalls,
		)
	}
}

// TestEvictStaleWalletTransactionCacheEntries verifies that a
// wallet's walletTransactionCache entry survives as long as either the
// acceptance or the re-anchor pending-event map still tracks a pending
// action for it, and is evicted only once neither map does - mirroring
// runReservationProofLoop's real usage, where
// evictStaleWalletTransactionCacheEntries runs once per pass after both
// proveReservationAcceptanceActions and proveReservationReanchorActions
// have finished updating those two maps for that pass, not as soon as a
// single pending event settles mid-pass.
func TestEvictStaleWalletTransactionCacheEntries(t *testing.T) {
	acceptanceWallet := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	reanchorWallet := [20]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	settledWallet := [20]byte{3, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	newPopulatedState := func() *reservationProofScanState {
		state := newReservationProofScanState()
		for _, wallet := range [][20]byte{acceptanceWallet, reanchorWallet, settledWallet} {
			state.walletTransactionCache[wallet] = &walletTransactionCacheEntry{}
		}
		state.pendingAcceptanceEvents["acceptance"] = &tbtc.ReservationAcceptanceRequestedEvent{
			WalletPublicKeyHash: acceptanceWallet,
		}
		state.pendingReanchorEvents["reanchor"] = &tbtc.ReservationReanchorRequestedEvent{
			SourceWalletPublicKeyHash: reanchorWallet,
		}
		return state
	}

	t.Run("wallet still pending in either map keeps its cache entry", func(t *testing.T) {
		state := newPopulatedState()

		evictStaleWalletTransactionCacheEntries(state)

		if _, exists := state.walletTransactionCache[acceptanceWallet]; !exists {
			t.Error("expected acceptance wallet's cache entry to be kept")
		}
		if _, exists := state.walletTransactionCache[reanchorWallet]; !exists {
			t.Error("expected re-anchor wallet's cache entry to be kept")
		}
	})

	t.Run("wallet with no pending action in either map is evicted", func(t *testing.T) {
		state := newPopulatedState()

		evictStaleWalletTransactionCacheEntries(state)

		if _, exists := state.walletTransactionCache[settledWallet]; exists {
			t.Error("expected settled wallet's cache entry to be evicted")
		}
	})

	t.Run("eviction is deferred to the pass boundary, not triggered by removing a single pending event", func(t *testing.T) {
		state := newPopulatedState()

		// Simulate proveReservationAcceptanceActions settling the
		// acceptance wallet's last pending action mid-pass: it deletes
		// the event from pendingAcceptanceEvents directly, the same way
		// production code does, without itself calling
		// evictStaleWalletTransactionCacheEntries.
		delete(state.pendingAcceptanceEvents, "acceptance")

		if _, exists := state.walletTransactionCache[acceptanceWallet]; !exists {
			t.Fatal("expected acceptance wallet's cache entry to survive until the pass-boundary eviction runs")
		}

		// Only once both passes for the loop iteration have finished
		// updating the pending-event maps does the boundary hook evict it.
		evictStaleWalletTransactionCacheEntries(state)

		if _, exists := state.walletTransactionCache[acceptanceWallet]; exists {
			t.Error("expected acceptance wallet's cache entry to be evicted once its pass-boundary check finds no pending action left")
		}
	})
}

// reservationProofOutputEncoding selects the script encoding a fixture
// transaction's authorized-wallet output uses.
type reservationProofOutputEncoding int

const (
	// encodingP2WPKH encodes the authorized output as a native segwit P2WPKH
	// script.
	encodingP2WPKH reservationProofOutputEncoding = iota
	// encodingP2PKH encodes the authorized output as a legacy P2PKH script.
	encodingP2PKH
)

// authorizedWalletScript builds the output script encoding publicKeyHash in
// the requested encoding.
func authorizedWalletScript(
	publicKeyHash [20]byte,
	encoding reservationProofOutputEncoding,
) (bitcoin.Script, error) {
	if encoding == encodingP2PKH {
		return bitcoin.PayToPublicKeyHash(publicKeyHash)
	}

	return bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
}

// timedOutAcceptanceFixture bundles the chains, scan state, and config needed
// to drive one proveReservationAcceptanceActions pass against a timed-out
// acceptance generation that has a confirmed anchor transaction.
type timedOutAcceptanceFixture struct {
	spvChain       *localChain
	btcChain       *reservationProofBitcoinChain
	state          *reservationProofScanState
	config         Config
	reservationKey *big.Int
	requestNonce   uint64
	walletPKH      [20]byte
	submissions    int
}

// newTimedOutAcceptanceFixture builds the fixture; outputEncoding selects
// whether the anchor transaction's authorized-wallet output is P2WPKH or
// P2PKH encoded. Both encodings are shared by the late-window tests below,
// so a matcher regressed to exact P2WPKH bytes fails the P2PKH variants.
func newTimedOutAcceptanceFixture(
	t *testing.T,
	outputEncoding reservationProofOutputEncoding,
) *timedOutAcceptanceFixture {
	t.Helper()
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	fixture := &timedOutAcceptanceFixture{}

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
	anchorScript, err := authorizedWalletScript(walletPublicKeyHash, outputEncoding)
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
			State:                     tbtc.ReservationActionStateTimedOut,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			// The late acceptance window is TimeoutAt + TermSeconds, where
			// TermSeconds is the per-action snapshot (100), so the window
			// closes at 1100.
			TimeoutAt:   1000,
			TermSeconds: 100,
			MinAmount:   1000,
		},
	)

	spvChain.submitReservationAcceptanceProofHook = func(
		*tbtc.BitcoinTxInfo,
		*tbtc.BitcoinTxProof,
		*big.Int,
		uint64,
	) error {
		fixture.submissions++
		return nil
	}

	fixture.spvChain = spvChain
	fixture.btcChain = btcChain
	fixture.state = newReservationProofScanState()
	fixture.config = Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	fixture.reservationKey = reservationKey
	fixture.requestNonce = requestNonce
	fixture.walletPKH = walletPublicKeyHash

	return fixture
}

// timedOutReanchorFixture is the re-anchor counterpart of
// timedOutAcceptanceFixture.
type timedOutReanchorFixture struct {
	spvChain        *localChain
	btcChain        *reservationProofBitcoinChain
	state           *reservationProofScanState
	config          Config
	reservationKey  *big.Int
	requestNonce    uint64
	sourceWalletPKH [20]byte
	submissions     int
}

// newTimedOutReanchorFixture builds the re-anchor fixture; outputEncoding
// selects whether the re-anchor transaction's authorized-wallet output is
// P2WPKH or P2PKH encoded.
func newTimedOutReanchorFixture(
	t *testing.T,
	outputEncoding reservationProofOutputEncoding,
) *timedOutReanchorFixture {
	t.Helper()
	const proofStart = 790270
	diff := func(d int64) *big.Int { return big.NewInt(d) }

	fixture := &timedOutReanchorFixture{}

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
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 600000},
		},
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
	reanchorScript, err := authorizedWalletScript(sourceWalletPublicKeyHash, outputEncoding)
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
			PublicKeyScript: reanchorScript,
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
		TxMaxFee:                  20000,
		BlockNumber:               500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStateTimedOut,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: sourceWalletPublicKeyHash,
			TimeoutAt:                 1000,
			TermSeconds:               100,
			MinAmount:                 1000,
			// The Bridge snapshotted this generation's source anchor as
			// keccak256(anchorTxHash | big-endian anchorTxOutputIndex) at
			// request time, which for the fixture's anchor outpoint is:
			SourceAnchorUtxoHash: reanchorSourceAnchorHash(
				&tbtc.Reservation{AnchorUtxo: anchorUtxo},
			),
		},
	)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		AnchorUtxo: anchorUtxo,
	})

	spvChain.submitReservationReanchorProofHook = func(
		*tbtc.BitcoinTxInfo,
		*tbtc.BitcoinTxProof,
		*big.Int,
		uint64,
	) error {
		fixture.submissions++
		return nil
	}

	fixture.spvChain = spvChain
	fixture.btcChain = btcChain
	fixture.state = newReservationProofScanState()
	fixture.config = Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	fixture.reservationKey = reservationKey
	fixture.requestNonce = requestNonce
	fixture.sourceWalletPKH = sourceWalletPublicKeyHash

	return fixture
}

// TestProveReservationAcceptanceActions_TimedOutLateWindow verifies that a
// timed-out acceptance generation stays a proof candidate through the
// Bridge's late settlement window: at now <= TimeoutAt + TermSeconds the
// proof is submitted and the generation stays tracked, and strictly past
// the bound it is evicted with zero submissions. Both authorized-wallet
// output encodings (P2WPKH and P2PKH) are driven through the pipeline, and
// time is driven deterministically through
// reservationProofScanState.nowFn.
func TestProveReservationAcceptanceActions_TimedOutLateWindow(t *testing.T) {
	encodings := []struct {
		name     string
		encoding reservationProofOutputEncoding
	}{
		{"p2wpkh", encodingP2WPKH},
		{"p2pkh", encodingP2PKH},
	}

	for _, enc := range encodings {
		t.Run(enc.name, func(t *testing.T) {
			runPass := func(now uint32) (*timedOutAcceptanceFixture, error) {
				fixture := newTimedOutAcceptanceFixture(t, enc.encoding)
				fixture.state.nowFn = func() uint32 { return now }

				err := proveReservationAcceptanceActions(
					fixture.state,
					fixture.config,
					fixture.spvChain,
					fixture.spvChain,
					fixture.btcChain,
					newProofInfoCache(),
					nil,
				)

				return fixture, err
			}

			t.Run("within the late window submits and keeps tracking", func(t *testing.T) {
				fixture, err := runPass(1050)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if fixture.submissions != 1 {
					t.Errorf("expected exactly one proof submission, got %d", fixture.submissions)
				}
				if _, exists := fixture.state.pendingAcceptanceEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; !exists {
					t.Error("expected the timed-out generation to remain tracked inside the late window")
				}
			})

			t.Run("at the exact late-window bound submits", func(t *testing.T) {
				fixture, err := runPass(1100)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if fixture.submissions != 1 {
					t.Errorf("expected exactly one proof submission at the bound, got %d", fixture.submissions)
				}
			})

			t.Run("strictly past the late-window bound evicts with zero submissions", func(t *testing.T) {
				fixture, err := runPass(1101)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if fixture.submissions != 0 {
					t.Errorf("expected zero submissions past the late window, got %d", fixture.submissions)
				}
				if _, exists := fixture.state.pendingAcceptanceEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; exists {
					t.Error("expected the generation past the late window to be evicted")
				}
			})
		})
	}
}

// TestProveReservationReanchorActions_TimedOutUnbounded verifies that a
// timed-out re-anchor generation stays a proof candidate without any
// bound, mirroring the Bridge: even at a now far past TimeoutAt +
// TermSeconds the proof is still submitted and the generation stays
// tracked.
func TestProveReservationReanchorActions_TimedOutUnbounded(t *testing.T) {
	encodings := []struct {
		name     string
		encoding reservationProofOutputEncoding
	}{
		{"p2wpkh", encodingP2WPKH},
		{"p2pkh", encodingP2PKH},
	}

	for _, enc := range encodings {
		t.Run(enc.name, func(t *testing.T) {
			fixture := newTimedOutReanchorFixture(t, enc.encoding)

			// 4000000000 is far past the fixture's TimeoutAt + TermSeconds
			// (1100), so a bounded late window would evict before
			// submitting.
			fixture.state.nowFn = func() uint32 { return 4000000000 }

			if err := proveReservationReanchorActions(
				fixture.state,
				fixture.config,
				fixture.spvChain,
				fixture.spvChain,
				fixture.btcChain,
				newProofInfoCache(),
				nil,
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fixture.submissions != 1 {
				t.Errorf("expected exactly one proof submission, got %d", fixture.submissions)
			}
			if _, exists := fixture.state.pendingReanchorEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; !exists {
				t.Error("expected the timed-out re-anchor generation to remain tracked without bound")
			}
		})
	}
}

// TestProveReservationActions_EvictNonSettleableStates verifies that
// generations whose on-chain state is no longer settleable (settled,
// superseded, vetoed, and unknown) are evicted from the pending-event map
// with no proof submission, in both the acceptance and re-anchor scans.
func TestProveReservationActions_EvictNonSettleableStates(t *testing.T) {
	// absent leaves the generation unseeded, so the fake returns the zero
	// record the Bridge's reservationActions mapping returns for a key it
	// has never written.
	states := []struct {
		name   string
		state  tbtc.ReservationActionState
		absent bool
	}{
		{"settled", tbtc.ReservationActionStateSettled, false},
		{"superseded", tbtc.ReservationActionStateSuperseded, false},
		{"vetoed", tbtc.ReservationActionStateVetoed, false},
		{"unknown", tbtc.ReservationActionStateUnknown, false},
		{"absent", tbtc.ReservationActionStateUnknown, true},
	}

	for _, s := range states {
		t.Run("acceptance "+s.name, func(t *testing.T) {
			fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)
			fixture.spvChain.setReservationAction(
				fixture.reservationKey,
				fixture.requestNonce,
				&tbtc.ReservationAction{
					State:                     s.state,
					ActionType:                tbtc.ReservationActionTypeAcceptance,
					TargetWalletPublicKeyHash: fixture.walletPKH,
					TimeoutAt:                 1000,
					TermSeconds:               100,
					MinAmount:                 1000,
				},
			)
			if s.absent {
				delete(
					fixture.spvChain.reservationActions,
					buildReservationActionKey(fixture.reservationKey, fixture.requestNonce),
				)
			}

			if err := proveReservationAcceptanceActions(
				fixture.state,
				fixture.config,
				fixture.spvChain,
				fixture.spvChain,
				fixture.btcChain,
				newProofInfoCache(),
				nil,
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fixture.submissions != 0 {
				t.Errorf("expected zero submissions for a %s generation, got %d", s.name, fixture.submissions)
			}
			if _, exists := fixture.state.pendingAcceptanceEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; exists {
				t.Errorf("expected the %s generation to be evicted", s.name)
			}
		})

		t.Run("reanchor "+s.name, func(t *testing.T) {
			fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)
			fixture.spvChain.setReservationAction(
				fixture.reservationKey,
				fixture.requestNonce,
				&tbtc.ReservationAction{
					State:                     s.state,
					ActionType:                tbtc.ReservationActionTypeReanchor,
					TargetWalletPublicKeyHash: fixture.sourceWalletPKH,
					TimeoutAt:                 1000,
					TermSeconds:               100,
					MinAmount:                 1000,
				},
			)
			if s.absent {
				delete(
					fixture.spvChain.reservationActions,
					buildReservationActionKey(fixture.reservationKey, fixture.requestNonce),
				)
			}

			if err := proveReservationReanchorActions(
				fixture.state,
				fixture.config,
				fixture.spvChain,
				fixture.spvChain,
				fixture.btcChain,
				newProofInfoCache(),
				nil,
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fixture.submissions != 0 {
				t.Errorf("expected zero submissions for a %s re-anchor generation, got %d", s.name, fixture.submissions)
			}
			if _, exists := fixture.state.pendingReanchorEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; exists {
				t.Errorf("expected the %s re-anchor generation to be evicted", s.name)
			}
		})
	}
}

// TestProveReservationAcceptanceActions_TimedOutUsesActionSnapshot verifies
// that the late acceptance window is governed by the per-action TermSeconds
// snapshot, not by the live ReservationParameters().ReservationTermSeconds:
// a governance change to the live parameter neither shrinks the window for
// a generation snapshotted with a longer term nor extends it for one
// snapshotted with a shorter one.
func TestProveReservationAcceptanceActions_TimedOutUsesActionSnapshot(t *testing.T) {
	runSnapshotPass := func(
		liveTermSeconds uint32,
		now uint32,
	) (*timedOutAcceptanceFixture, error) {
		fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)
		// The fixture's action snapshot carries TermSeconds 100; the live
		// parameter below deliberately differs from it. The production
		// scan path never reads the live parameter, so this only pins
		// the behavior against a regression that starts using it.
		fixture.spvChain.setReservationParameters(&tbtc.ReservationParameters{
			ReservationTermSeconds: liveTermSeconds,
		})
		fixture.state.nowFn = func() uint32 { return now }

		err := proveReservationAcceptanceActions(
			fixture.state,
			fixture.config,
			fixture.spvChain,
			fixture.spvChain,
			fixture.btcChain,
			newProofInfoCache(),
			nil,
		)

		return fixture, err
	}

	t.Run("live parameter lower than the snapshot does not shrink the window", func(t *testing.T) {
		// Live 10 against snapshot 100: TimeoutAt 1000, now 1050. A live
		// bound of 1010 would evict this generation; the snapshot bound
		// of 1100 keeps it.
		fixture, err := runSnapshotPass(10, 1050)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fixture.submissions != 1 {
			t.Errorf("expected exactly one proof submission, got %d", fixture.submissions)
		}
		if _, exists := fixture.state.pendingAcceptanceEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; !exists {
			t.Error("expected the generation to stay tracked while inside the snapshot window")
		}
	})

	t.Run("live parameter higher than the snapshot does not extend the window", func(t *testing.T) {
		// Live 300 against snapshot 100: now 1101 is strictly past the
		// snapshot bound of 1100 even though it is inside the live bound
		// of 1300.
		fixture, err := runSnapshotPass(300, 1101)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fixture.submissions != 0 {
			t.Errorf("expected zero submissions, got %d", fixture.submissions)
		}
		if _, exists := fixture.state.pendingAcceptanceEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; exists {
			t.Error("expected the generation to be evicted at the snapshot bound")
		}
	})

	t.Run("at the exact snapshot bound with a differing live parameter submits", func(t *testing.T) {
		// Live 300 differs from snapshot 100; now 1100 is the exact
		// snapshot bound.
		fixture, err := runSnapshotPass(300, 1100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fixture.submissions != 1 {
			t.Errorf("expected exactly one proof submission at the snapshot bound, got %d", fixture.submissions)
		}
	})
}

// TestProveReservationReanchorActions_TimedOutSourceAnchor verifies that a
// TimedOut re-anchor generation is evicted exactly when it is provably
// un-settleable: the reservation's current anchor outpoint no longer
// matches the source anchor the generation snapshotted on-chain at request
// time (action.SourceAnchorUtxoHash), after which the Bridge's
// requireCurrentSourceAnchor check in ReservationProofs.sol rejects its
// late settlement forever. The comparison is keyed on that on-chain
// snapshot, not on any first-observation record kept by the Go loop, so a
// fresh scan state (process restart) still evicts a generation whose
// source anchor was replaced before it was ever observed. While the source
// anchor is unchanged the generation stays tracked regardless of age, and
// the wallet's cached transaction entry is released only once no tracked
// generation remains.
func TestProveReservationReanchorActions_TimedOutSourceAnchor(t *testing.T) {
	replaceFixtureSourceAnchor := func(fixture *timedOutReanchorFixture) {
		// A later settled generation has re-anchored the reservation:
		// the on-chain reservation record now points at a different
		// anchor outpoint than the one this generation was requested
		// against, while the action's snapshotted
		// SourceAnchorUtxoHash still refers to the original outpoint.
		replacementAnchorTx := &bitcoin.Transaction{
			Outputs: []*bitcoin.TransactionOutput{{Value: 400000}},
		}
		if err := fixture.btcChain.BroadcastTransaction(replacementAnchorTx); err != nil {
			t.Fatal(err)
		}
		fixture.spvChain.setReservation(
			fixture.reservationKey,
			&tbtc.Reservation{
				AnchorUtxo: &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: replacementAnchorTx.Hash(),
						OutputIndex:     0,
					},
					Value: 400000,
				},
			},
		)
	}

	t.Run("replaced source anchor is evicted", func(t *testing.T) {
		fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)

		if err := proveReservationReanchorActions(
			fixture.state,
			fixture.config,
			fixture.spvChain,
			fixture.spvChain,
			fixture.btcChain,
			newProofInfoCache(),
			nil,
		); err != nil {
			t.Fatalf("unexpected error on the first pass: %v", err)
		}
		if fixture.submissions != 1 {
			t.Fatalf("expected exactly one proof submission on the first pass, got %d", fixture.submissions)
		}

		key := reservationEventKey(fixture.reservationKey, fixture.requestNonce)
		if _, tracked := fixture.state.pendingReanchorEvents[key]; !tracked {
			t.Fatal("expected the generation to be tracked before its source anchor moved")
		}

		replaceFixtureSourceAnchor(fixture)

		if err := proveReservationReanchorActions(
			fixture.state,
			fixture.config,
			fixture.spvChain,
			fixture.spvChain,
			fixture.btcChain,
			newProofInfoCache(),
			nil,
		); err != nil {
			t.Fatalf("unexpected error on the second pass: %v", err)
		}

		if fixture.submissions != 1 {
			t.Errorf("expected no further proof submission after eviction, got %d", fixture.submissions)
		}
		if _, tracked := fixture.state.pendingReanchorEvents[key]; tracked {
			t.Error("expected the generation to be evicted once its source anchor was replaced")
		}

		// The wallet's cached entry is released now that no tracked
		// generation remains for it; runReservationProofLoop performs
		// this eviction after each proof round.
		evictStaleWalletTransactionCacheEntries(fixture.state)
		if _, cached := fixture.state.walletTransactionCache[fixture.sourceWalletPKH]; cached {
			t.Error("expected the wallet's cached transaction entry to be released once no tracked generation remains")
		}
	})

	t.Run("unchanged source anchor is kept regardless of age", func(t *testing.T) {
		fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)
		// Far past any bounded late window: re-anchor late settlement is
		// unbounded, so age alone never evicts a tracked generation.
		fixture.state.nowFn = func() uint32 { return 4000000000 }

		key := reservationEventKey(fixture.reservationKey, fixture.requestNonce)

		for pass := 1; pass <= 3; pass++ {
			if err := proveReservationReanchorActions(
				fixture.state,
				fixture.config,
				fixture.spvChain,
				fixture.spvChain,
				fixture.btcChain,
				newProofInfoCache(),
				nil,
			); err != nil {
				t.Fatalf("unexpected error on pass %d: %v", pass, err)
			}
		}

		if fixture.submissions != 3 {
			t.Errorf(
				"expected the still-settleable generation to be proved on each pass, got %d submissions",
				fixture.submissions,
			)
		}
		if _, tracked := fixture.state.pendingReanchorEvents[key]; !tracked {
			t.Error("expected a TimedOut generation with an unchanged source anchor to remain tracked")
		}

		// The wallet's cached entry survives the eviction pass because a
		// tracked generation still references it.
		evictStaleWalletTransactionCacheEntries(fixture.state)
		if _, cached := fixture.state.walletTransactionCache[fixture.sourceWalletPKH]; !cached {
			t.Error("expected the wallet's cached transaction entry to survive while a generation remains tracked")
		}
	})

	t.Run("fresh scan state still evicts a replaced source anchor", func(t *testing.T) {
		// Simulated process restart: the generation's source anchor was
		// already replaced before this scan state ever observed the
		// generation. A Go-side first-observation snapshot would record
		// the replacement anchor and never evict; the on-chain snapshot
		// (action.SourceAnchorUtxoHash) still refers to the original
		// outpoint, so the very first pass evicts the generation.
		fixture := newTimedOutReanchorFixture(t, encodingP2WPKH)
		replaceFixtureSourceAnchor(fixture)

		if err := proveReservationReanchorActions(
			fixture.state,
			fixture.config,
			fixture.spvChain,
			fixture.spvChain,
			fixture.btcChain,
			newProofInfoCache(),
			nil,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fixture.submissions != 0 {
			t.Errorf(
				"expected zero proof submissions against a replaced source anchor, got %d",
				fixture.submissions,
			)
		}
		if _, tracked := fixture.state.pendingReanchorEvents[reservationEventKey(fixture.reservationKey, fixture.requestNonce)]; tracked {
			t.Error("expected a fresh scan state to evict a generation whose source anchor was already replaced")
		}
	})
}

// TestReanchorSourceAnchorHash pins reanchorSourceAnchorHash against a
// hand-computed keccak256 of the Bridge's anchorUtxoHash encoding
// (keccak256(abi.encodePacked(anchorTxHash, uint32 anchorTxOutputIndex)) in
// Reservation.sol) for a fixed outpoint: the transaction hash in its
// Bitcoin internal byte order, the output index big-endian.
func TestReanchorSourceAnchorHash(t *testing.T) {
	txHash := [32]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}

	t.Run("output index 0", func(t *testing.T) {
		reservation := &tbtc.Reservation{
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash(txHash),
					OutputIndex:     0,
				},
			},
		}

		expected := [32]byte{
			0xf3, 0x38, 0x31, 0xec, 0x8a, 0x7f, 0x96, 0x37, 0x9e, 0xf0,
			0xdb, 0x74, 0x5d, 0x66, 0xa8, 0xa8, 0xcb, 0x56, 0x33, 0xc5,
			0x8f, 0xd3, 0xc0, 0xcf, 0x9f, 0xce, 0xa9, 0x78, 0x03, 0x0e,
			0xc8, 0xcd,
		}
		if actual := reanchorSourceAnchorHash(reservation); actual != expected {
			t.Errorf("expected anchor hash [%x], got [%x]", expected, actual)
		}
	})

	t.Run("output index 1", func(t *testing.T) {
		reservation := &tbtc.Reservation{
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash(txHash),
					OutputIndex:     1,
				},
			},
		}

		expected := [32]byte{
			0xb6, 0x48, 0x8c, 0xd5, 0x8d, 0xbe, 0xef, 0x5b, 0x11, 0xde,
			0xe5, 0xf7, 0x8c, 0xc0, 0xe9, 0x21, 0x20, 0x02, 0xae, 0x61,
			0x40, 0x3c, 0x32, 0xfc, 0xf4, 0xdf, 0x32, 0x6d, 0x19, 0xbd,
			0xf4, 0x21,
		}
		if actual := reanchorSourceAnchorHash(reservation); actual != expected {
			t.Errorf("expected anchor hash [%x], got [%x]", expected, actual)
		}
	})

	t.Run("no anchor outpoint yields the zero hash", func(t *testing.T) {
		if actual := reanchorSourceAnchorHash(&tbtc.Reservation{}); actual != ([32]byte{}) {
			t.Errorf("expected the zero hash, got [%x]", actual)
		}
	})
}

// reservationScanCountingChain wraps localChain, counting Past*Events scan
// calls and failing the re-anchor scan on demand, so a test can drive one
// chain-wide read error through a runReservationProofLoop invocation
// followed by a restarted invocation against the same scan state.
type reservationScanCountingChain struct {
	*localChain

	acceptanceScanCalls int
	reanchorScanCalls   int
	failReanchorScan    bool
}

func (c *reservationScanCountingChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	c.acceptanceScanCalls++
	return c.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

func (c *reservationScanCountingChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	c.reanchorScanCalls++
	if c.failReanchorScan {
		return nil, errors.New("simulated provider failure")
	}
	return c.localChain.PastReservationReanchorRequestedEvents(filter)
}

// TestRunReservationProofLoop_ResumesScanAfterRestart verifies that a
// restarted runReservationProofLoop invocation resumes each scan from its
// last completed position instead of replaying the activation-to-tip
// history: the first invocation advances the acceptance scan's cursor to
// the tip and then dies on a chain-wide re-anchor read error; the restart
// against the same state does not refetch the already-scanned acceptance
// range, while the re-anchor scan still completes from its own cursor.
func TestRunReservationProofLoop_ResumesScanAfterRestart(t *testing.T) {
	inner := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	inner.setBlockCounter(blockCounter)

	chain := &reservationScanCountingChain{localChain: inner}

	state := newReservationProofScanState()
	config := Config{
		IdleBackoffTime:  time.Hour,
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	btcChain := newLocalBitcoinChain()

	// First invocation: a chain-wide re-anchor read error aborts the
	// restartable loop, after the acceptance scan has already completed.
	chain.failReanchorScan = true
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()

	if err := runReservationProofLoop(
		firstCtx,
		config,
		chain,
		chain,
		btcChain,
		nil,
		state,
	); err == nil {
		t.Fatal("expected the chain-wide re-anchor read error to abort the loop")
	}
	if chain.acceptanceScanCalls != 1 {
		t.Fatalf("expected exactly one acceptance scan on the first invocation, got %d", chain.acceptanceScanCalls)
	}
	if chain.reanchorScanCalls != 1 {
		t.Fatalf("expected exactly one re-anchor scan on the first invocation, got %d", chain.reanchorScanCalls)
	}

	// Restart: the provider recovers. The restarted loop runs its clean
	// pass against a pre-cancelled context, so it finishes both scans
	// and then exits at the idle-backoff select without sleeping.
	chain.failReanchorScan = false
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	cancelSecond()

	restartErr := runReservationProofLoop(
		secondCtx,
		config,
		chain,
		chain,
		btcChain,
		nil,
		state,
	)
	if restartErr != context.Canceled {
		t.Fatalf(
			"expected the restarted clean pass to end in context cancellation, got: %v",
			restartErr,
		)
	}

	// The acceptance scan already covered activation through the tip on
	// the first invocation; the restart must not refetch that range.
	if chain.acceptanceScanCalls != 1 {
		t.Errorf(
			"expected the restarted invocation to resume the acceptance scan from its last completed position, but it refetched: acceptance scan calls = %d",
			chain.acceptanceScanCalls,
		)
	}
	if chain.reanchorScanCalls != 2 {
		t.Errorf(
			"expected the re-anchor scan to complete on the restart from its own last completed position, calls = %d",
			chain.reanchorScanCalls,
		)
	}
	if state.acceptanceLastScannedBlock != 1000-reservationEventScanConfirmationBlocks {
		t.Errorf("expected the acceptance cursor to survive the restart at %d, got %d", 1000-reservationEventScanConfirmationBlocks, state.acceptanceLastScannedBlock)
	}
	if state.reanchorLastScannedBlock != 1000-reservationEventScanConfirmationBlocks {
		t.Errorf("expected the re-anchor cursor to advance to %d on the restart, got %d", 1000-reservationEventScanConfirmationBlocks, state.reanchorLastScannedBlock)
	}
}

// reservationChunkFailingChain wraps localChain and fails the acceptance
// event fetch for any chunk starting at failChunkStart, so a test can
// drive a scan that breaks part-way through the activation-to-tip walk.
type reservationChunkFailingChain struct {
	*localChain

	failChunkStart *uint64
}

func (c *reservationChunkFailingChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if c.failChunkStart != nil && filter.StartBlock == *c.failChunkStart {
		return nil, errors.New("simulated provider failure")
	}
	return c.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

// TestProveReservationAcceptanceActions_ChunkedScan verifies the proof
// loop's event scan across chunk boundaries: every chunk of the
// activation-to-tip walk is scanned, a failing chunk keeps the progress of
// the chunks before it (the cursor stops at the last good chunk and the
// next pass resumes after it), and the cursor stays
// reservationEventScanConfirmationBlocks behind the tip, so an event that
// only shows up later at a height the first pass could have covered - as
// after a short reorg - is still found by the next pass.
func TestProveReservationAcceptanceActions_ChunkedScan(t *testing.T) {
	const currentBlock = 2*reservationEventScanChunkSize + 500
	confirmedTip := uint64(currentBlock) - reservationEventScanConfirmationBlocks

	newFixture := func() (*localChain, *reservationChunkFailingChain, *reservationProofScanState, Config) {
		inner := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		inner.setBlockCounter(blockCounter)
		return inner,
			&reservationChunkFailingChain{localChain: inner},
			newReservationProofScanState(),
			Config{
				TransactionLimit: 100,
				MaxProofHeaders:  DefaultMaxProofHeaders,
				EthereumNetwork:  ethereum.Developer,
			}
	}

	// addEvent tracks a Pending acceptance generation requested at
	// blockNumber; its wallet has no transactions, so nothing is proved
	// and the generation simply stays tracked.
	addEvent := func(inner *localChain, key int64, blockNumber uint64) string {
		reservationKey := big.NewInt(key)
		inner.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      reservationKey,
			RequestNonce:        1,
			WalletPublicKeyHash: [20]byte{byte(key)},
			BlockNumber:         blockNumber,
		})
		inner.setReservationAction(reservationKey, 1, &tbtc.ReservationAction{
			State:      tbtc.ReservationActionStatePending,
			ActionType: tbtc.ReservationActionTypeAcceptance,
		})
		return reservationEventKey(reservationKey, 1)
	}

	prove := func(chain *reservationChunkFailingChain, state *reservationProofScanState, config Config) error {
		return proveReservationAcceptanceActions(
			state,
			config,
			chain,
			chain,
			newReservationProofBitcoinChain(),
			newProofInfoCache(),
			nil,
		)
	}

	t.Run("every chunk is scanned", func(t *testing.T) {
		inner, chain, state, config := newFixture()
		firstChunkKey := addEvent(inner, 1, 100)
		secondChunkKey := addEvent(inner, 2, reservationEventScanChunkSize+100)

		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, key := range []string{firstChunkKey, secondChunkKey} {
			if _, ok := state.pendingAcceptanceEvents[key]; !ok {
				t.Errorf("expected event [%s] to be tracked", key)
			}
		}
		if state.acceptanceLastScannedBlock != confirmedTip {
			t.Errorf("expected the cursor at %d, got %d", confirmedTip, state.acceptanceLastScannedBlock)
		}
	})

	t.Run("a failing chunk keeps earlier progress", func(t *testing.T) {
		inner, chain, state, config := newFixture()
		firstChunkKey := addEvent(inner, 1, 100)
		secondChunkKey := addEvent(inner, 2, reservationEventScanChunkSize+100)

		failStart := reservationEventScanChunkSize
		chain.failChunkStart = &failStart
		if err := prove(chain, state, config); err == nil {
			t.Fatal("expected the failing chunk to be reported")
		}
		if _, ok := state.pendingAcceptanceEvents[firstChunkKey]; !ok {
			t.Error("expected the first chunk's event to be tracked despite the later failure")
		}
		if _, ok := state.pendingAcceptanceEvents[secondChunkKey]; ok {
			t.Error("expected the failed chunk's event not to be tracked yet")
		}
		if state.acceptanceLastScannedBlock != reservationEventScanChunkSize-1 {
			t.Errorf(
				"expected the cursor at the end of the first chunk %d, got %d",
				reservationEventScanChunkSize-1,
				state.acceptanceLastScannedBlock,
			)
		}

		chain.failChunkStart = nil
		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error on the resumed pass: %v", err)
		}
		if _, ok := state.pendingAcceptanceEvents[secondChunkKey]; !ok {
			t.Error("expected the resumed pass to find the second chunk's event")
		}
		if state.acceptanceLastScannedBlock != confirmedTip {
			t.Errorf("expected the cursor at %d, got %d", confirmedTip, state.acceptanceLastScannedBlock)
		}
	})

	t.Run("the cursor trails the tip", func(t *testing.T) {
		inner, chain, state, config := newFixture()

		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// An event at a height the first pass could have reached if it
		// had scanned to the tip, first visible only now.
		lateKey := addEvent(inner, 3, uint64(currentBlock)-reservationEventScanConfirmationBlocks/2)
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock + reservationEventScanConfirmationBlocks)
		inner.setBlockCounter(blockCounter)

		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := state.pendingAcceptanceEvents[lateKey]; !ok {
			t.Error("expected the event below the old tip to be found by the next pass")
		}
	})
}
