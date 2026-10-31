package spv

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestRecordReservationProofRetry verifies that the retry counter is
// incremented for a failed proof round and that a nil recorder is safe.
func TestRecordReservationProofRetry(t *testing.T) {
	recorder := &mockMetricsRecorder{counts: make(map[string]float64)}

	recordReservationProofRetry(recorder)

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsRetriedTotal]; count != 1 {
		t.Errorf(
			"retry counter = %v, want 1",
			count,
		)
	}

	// A nil recorder must not panic: the proof loop runs with metrics
	// disabled when the client info pipeline is off.
	recordReservationProofRetry(nil)
}

// TestRecordReservationProofSuccess verifies that the liveness gauge is
// set to the current unix time and that a nil recorder is safe.
func TestRecordReservationProofSuccess(t *testing.T) {
	recorder := &mockMetricsRecorder{counts: make(map[string]float64)}

	recordReservationProofSuccess(recorder)

	value := recorder.counts[clientinfo.MetricSpvReservationProofLastSuccessTimestamp]
	if value <= 0 {
		t.Errorf(
			"liveness gauge = %v, want a positive unix timestamp",
			value,
		)
	}

	recordReservationProofSuccess(nil)
}

// TestSubmitReservationActionProof_CountsClientinfoReservationProofs
// verifies that a successful reservation proof submission increments the
// clientinfo-level submitted and succeeded counters, and that a failed
// submission increments the failed counter.
//
// These counters are the client-level view of the reservation proof round:
// without them an operator cannot tell a working proof loop from one that
// submits nothing.
func TestSubmitReservationActionProof_CountsClientinfoReservationProofs(t *testing.T) {
	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	recorder := &mockMetricsRecorder{counts: make(map[string]float64)}

	// A submission with a zero confirmation count fails before any chain
	// interaction: the failed counter must still be incremented.
	err := submitReservationAcceptanceProof(
		bitcoin.Hash{},
		0,
		big.NewInt(1),
		1,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
		recorder,
	)
	if err == nil {
		t.Fatal("expected an error for zero required confirmations")
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsSubmittedTotal]; count != 1 {
		t.Errorf("submitted counter = %v, want 1", count)
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsFailedTotal]; count != 1 {
		t.Errorf("failed counter = %v, want 1", count)
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsSucceededTotal]; count != 0 {
		t.Errorf("succeeded counter = %v, want 0", count)
	}
}

// TestSubmitReservationAcceptanceProof_CountsSucceededCounter verifies that
// a proof the Bridge accepts increments the clientinfo-level succeeded
// counter, so an operator can see the proof loop actually settling
// reservations rather than only attempting to.
func TestSubmitReservationAcceptanceProof_CountsSucceededCounter(t *testing.T) {
	requiredConfirmations := uint(6)

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           600000,
			PublicKeyScript: []byte{},
		}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	walletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPKH)
	if err != nil {
		t.Fatal(err)
	}

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: walletScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}

	proof := &bitcoin.SpvProof{
		MerkleProof:    []byte{0x01},
		TxIndexInBlock: 2,
		BitcoinHeaders: []byte{0x03},
	}

	mockSpvProofAssembler := func(
		hash bitcoin.Hash,
		confirmations uint,
		btcChain bitcoin.Chain,
	) (*bitcoin.Transaction, *bitcoin.SpvProof, error) {
		return anchorTx, proof, nil
	}

	reservationKey := big.NewInt(43)
	requestNonce := uint64(1)

	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPKH,
		TermSeconds:               86400,
		MinAmount:                 100000,
	})

	spvChain.submitReservationAcceptanceProofHook = func(
		txInfo *tbtc.BitcoinTxInfo,
		txProof *tbtc.BitcoinTxProof,
		rk *big.Int,
		rn uint64,
	) error {
		return nil
	}

	recorder := &mockMetricsRecorder{counts: make(map[string]float64)}

	if err := submitReservationAcceptanceProof(
		anchorTx.Hash(),
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		mockSpvProofAssembler,
		recorder,
	); err != nil {
		t.Fatal(err)
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsSubmittedTotal]; count != 1 {
		t.Errorf("submitted counter = %v, want 1", count)
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsSucceededTotal]; count != 1 {
		t.Errorf("succeeded counter = %v, want 1", count)
	}

	if count := recorder.counts[clientinfo.MetricSpvReservationProofsFailedTotal]; count != 0 {
		t.Errorf("failed counter = %v, want 0", count)
	}
}
