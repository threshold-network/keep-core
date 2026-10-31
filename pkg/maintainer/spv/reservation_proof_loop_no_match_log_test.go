package spv

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"

	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
)

// noAnchorSpendLogFragment is the fragment every "the wallet has not
// broadcast the transaction yet" line carries, whatever the proof path it
// comes from.
const noAnchorSpendLogFragment = "has no Bitcoin transaction spending"

// noAnchorSpendLogEntries returns the captured log messages that carry the
// "no Bitcoin transaction spending" fragment.
func noAnchorSpendLogEntries(entries []infeasibleErrorLogEntry) []string {
	var matching []string
	for _, entry := range entries {
		if strings.Contains(entry.Message, noAnchorSpendLogFragment) {
			matching = append(matching, entry.Message)
		}
	}
	return matching
}

// noAnchorSpendTestWallet is a wallet whose Bitcoin history holds a
// transaction, so the proof loop fetches the wallet's history and indexes
// its transactions, but none of them spends the outpoint the tracked
// generation is waiting for.
type noAnchorSpendTestWallet struct {
	publicKeyHash [20]byte
	script        bitcoin.Script
}

// newNoAnchorSpendTestWallet builds a wallet and the single transaction
// that pays it, spending an outpoint unrelated to any tracked generation.
func newNoAnchorSpendTestWallet(
	t *testing.T,
	publicKeyHash [20]byte,
) *noAnchorSpendTestWallet {
	t.Helper()

	script, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	return &noAnchorSpendTestWallet{
		publicKeyHash: publicKeyHash,
		script:        script,
	}
}

// broadcastUnrelatedTransaction broadcasts a one-input, one-output
// transaction that pays the wallet and spends an outpoint no tracked
// generation is waiting for.
func (w *noAnchorSpendTestWallet) broadcastUnrelatedTransaction(
	t *testing.T,
	btcChain *reservationProofBitcoinChain,
) {
	t.Helper()

	unrelatedTxHash := bitcoin.Hash{0xAB, 0xCD, 0xEF}
	transaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: unrelatedTxHash,
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           1000,
			PublicKeyScript: w.script,
		}},
	}

	if err := btcChain.BroadcastTransaction(transaction); err != nil {
		t.Fatal(err)
	}
}

// TestProveReservationReanchorActions_LogsWhenNoAnchorSpendExists checks
// that a tracked re-anchor generation whose anchor outpoint has no
// spending transaction on the Bitcoin chain is reported in the log. The
// generation is discovered and followed by the loop, so its silence would
// leave an operator unable to tell "the wallet has not broadcast the
// re-anchor yet" from "the proof loop is dead".
func TestProveReservationReanchorActions_LogsWhenNoAnchorSpendExists(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(424242)
	const requestNonce = 2

	sourceWallet := newNoAnchorSpendTestWallet(
		t,
		[20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40},
	)
	targetWallet := [20]byte{41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60}

	// The anchor UTXO the re-anchor must spend. The wallet's history
	// holds no transaction spending it.
	anchorTxHash := bitcoin.Hash{0x11, 0x22, 0x33}
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	sourceWallet.broadcastUnrelatedTransaction(t, btcChain)

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: sourceWallet.publicKeyHash,
		TargetWalletPublicKeyHash: targetWallet,
		BlockNumber:               500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: targetWallet,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		AnchorUtxo: anchorUtxo,
	})

	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	scanState := newReservationProofScanState()

	entries := captureSpvLogs(t, func() {
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
	})

	matching := noAnchorSpendLogEntries(entries)
	if len(matching) != 1 {
		t.Fatalf(
			"expected exactly one log line reporting the missing anchor spend, got %d: %v",
			len(matching),
			matching,
		)
	}

	line := matching[0]
	if !strings.Contains(line, reservationKey.String()) {
		t.Errorf(
			"expected the log line to name the reservation key [%s], got: %s",
			reservationKey.String(),
			line,
		)
	}
	expectedOutpoint := fmt.Sprintf("%s:%d", anchorTxHash.String(), 1)
	if !strings.Contains(line, expectedOutpoint) {
		t.Errorf(
			"expected the log line to name the anchor outpoint [%s], got: %s",
			expectedOutpoint,
			line,
		)
	}

	// The generation stays tracked: the loop is waiting for the wallet's
	// transaction, not giving up on the generation.
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingReanchorEvents[key]; !exists {
		t.Errorf("expected the generation to stay tracked in pendingReanchorEvents")
	}
}

// TestProveReservationReanchorActions_NoLogWhenAnchorSpendExists checks the
// other side of the branch: a generation whose anchor outpoint is spent by
// a wallet transaction must not produce the "no Bitcoin transaction
// spending" line.
func TestProveReservationReanchorActions_NoLogWhenAnchorSpendExists(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(515151)
	const requestNonce = 3

	sourceWallet := newNoAnchorSpendTestWallet(
		t,
		[20]byte{61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80},
	)

	anchorTxHash := bitcoin.Hash{0x44, 0x55, 0x66}
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: anchorTxHash,
			OutputIndex:     1,
		},
		Value: 600000,
	}

	// The wallet broadcasts a transaction spending the anchor outpoint.
	reanchorTransaction := &bitcoin.Transaction{
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: sourceWallet.script,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchorTransaction); err != nil {
		t.Fatal(err)
	}

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              requestNonce,
		SourceWalletPublicKeyHash: sourceWallet.publicKeyHash,
		TargetWalletPublicKeyHash: sourceWallet.publicKeyHash,
		BlockNumber:               500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeReanchor,
			TargetWalletPublicKeyHash: sourceWallet.publicKeyHash,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)
	spvChain.setReservation(reservationKey, &tbtc.Reservation{
		AnchorUtxo: anchorUtxo,
	})

	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	scanState := newReservationProofScanState()

	entries := captureSpvLogs(t, func() {
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
	})

	if matching := noAnchorSpendLogEntries(entries); len(matching) != 0 {
		t.Errorf(
			"expected no missing-anchor-spend log line, got %d: %v",
			len(matching),
			matching,
		)
	}
}

// TestProveReservationAcceptanceActions_LogsWhenNoAcceptanceSpendExists
// checks the acceptance proof path, which has the same silent branch: a
// tracked acceptance generation whose deposit has not been swept by the
// wallet yet is skipped without a word.
func TestProveReservationAcceptanceActions_LogsWhenNoAcceptanceSpendExists(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet := newNoAnchorSpendTestWallet(
		t,
		[20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
	)

	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{Value: 150000}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}
	fundingTxHash := fundingTx.Hash()
	reservationKey := spvChain.BuildDepositKey(fundingTxHash, 0)
	const requestNonce = 1

	// The wallet holds a transaction, but none of them spends the
	// deposit the acceptance generation waits for.
	wallet.broadcastUnrelatedTransaction(t, btcChain)

	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: wallet.publicKeyHash,
		BlockNumber:         500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: wallet.publicKeyHash,
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

	entries := captureSpvLogs(t, func() {
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
	})

	matching := noAnchorSpendLogEntries(entries)
	if len(matching) != 1 {
		t.Fatalf(
			"expected exactly one log line reporting the missing acceptance spend, got %d: %v",
			len(matching),
			matching,
		)
	}

	if !strings.Contains(matching[0], reservationKey.String()) {
		t.Errorf(
			"expected the log line to name the reservation key [%s], got: %s",
			reservationKey.String(),
			matching[0],
		)
	}

	// The generation stays tracked: the loop is waiting for the wallet's
	// transaction, not giving up on the generation.
	key := reservationEventKey(reservationKey, requestNonce)
	if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
		t.Errorf("expected the generation to stay tracked in pendingAcceptanceEvents")
	}
}
