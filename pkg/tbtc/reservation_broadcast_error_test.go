// Execute-level reservation broadcast-error tests: executor-level error surface plus the two env-gated full-execute tests.
package tbtc

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"go.uber.org/zap"
)

// TestReservationBroadcastTransaction_BroadcastError pins the error the
// executor's broadcastTransaction returns when a broadcast transaction is
// never confirmed: the loop keeps re-checking until the timeout elapses and
// reports the timeout. The reservation execute() methods wrap exactly this
// error ("cannot broadcast reservation ... transaction: [broadcast timeout
// exceeded]"), so this is the error surface their broadcast-error paths
// report.
func TestReservationBroadcastTransaction_BroadcastError(t *testing.T) {
	custody, _ := newCustodyWallet(t)

	// A Bitcoin chain whose confirmation lookup always fails, so
	// broadcastTransaction never observes the transaction landing.
	neverConfirming := &reservationBroadcastErrorChain{
		localBitcoinChain: newLocalBitcoinChain(),
	}

	tx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x01},
				OutputIndex:     0,
			},
			Sequence: 0xffffffff,
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           1000,
			PublicKeyScript: []byte{0x51},
		}},
	}

	executor := newWalletTransactionExecutor(
		neverConfirming,
		custody,
		&custodySigningExecutor{custody: custody},
		func(_ context.Context, _ uint64) error { return nil },
	)

	err := executor.broadcastTransaction(
		zap.NewNop().Sugar(),
		tx,
		300*time.Millisecond,
		50*time.Millisecond,
	)
	if err == nil || err.Error() != "broadcast timeout exceeded" {
		t.Fatalf("expected the broadcast timeout error, got: [%v]", err)
	}
}

// reservationBroadcastErrorChain refuses to relay and never confirms a
// transaction, forcing broadcastTransaction to time out on its confirmation
// check instead of succeeding on the first relay attempt.
type reservationBroadcastErrorChain struct {
	*localBitcoinChain
}

func (c *reservationBroadcastErrorChain) BroadcastTransaction(
	_ *bitcoin.Transaction,
) error {
	return fmt.Errorf("relay refused")
}

func (c *reservationBroadcastErrorChain) GetTransactionConfirmations(
	_ context.Context,
	_ bitcoin.Hash,
) (uint, error) {
	return 0, fmt.Errorf("transaction not found")
}

// reservationExecuteBroadcastSlowTest reports whether the execute-level
// reservation broadcast-error tests should run. They are gated because they
// drive the production broadcast loop to its const
// reservationActionBroadcastTimeout (fifteen minutes), which would
// otherwise block the default reservation suite.
func reservationExecuteBroadcastSlowTest(t *testing.T) bool {
	t.Helper()
	if os.Getenv("RESERVATION_BROADCAST_ERROR_TEST") == "" {
		t.Skip(
			"set RESERVATION_BROADCAST_ERROR_TEST=1 to exercise the " +
				"execute-level reservation broadcast error path (15-minute loop)",
		)
	}
	return true
}

// TestReservationAnchorAction_BroadcastError drives the anchor execute()
// broadcast step through a never-confirming Bitcoin chain so the executor's
// broadcast loop times out, killing the swallowed-broadcast-error path.
func TestReservationAnchorAction_BroadcastError(t *testing.T) {
	if !reservationExecuteBroadcastSlowTest(t) {
		return
	}
	_, walletPKH := newCustodyWallet(t)

	action, signExecutor, recordingChain, _ := newAnchorSignFixture(
		t,
		false,
		true, // never confirm, so the broadcast step times out
		pendingAnchorActionState(walletPKH),
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the broadcast step to fail, got nil")
	}
	if !strings.Contains(
		err.Error(),
		"cannot broadcast reservation anchor transaction",
	) {
		t.Fatalf(
			"expected the anchor broadcast error, got: [%v]",
			err,
		)
	}
	if signExecutor.startCalls() != 1 {
		t.Errorf(
			"expected exactly one sign call before the broadcast failure, got [%d]",
			signExecutor.startCalls(),
		)
	}
	if len(recordingChain.broadcastCalls()) == 0 {
		t.Fatal("expected the broadcast step to be attempted")
	}
}

// TestReservationReanchorAction_BroadcastError drives the re-anchor execute()
// broadcast step through a never-confirming Bitcoin chain so the executor's
// broadcast loop times out, killing the swallowed-broadcast-error path.
func TestReservationReanchorAction_BroadcastError(t *testing.T) {
	if !reservationExecuteBroadcastSlowTest(t) {
		return
	}
	targetPKH := [20]byte{0x02}

	action, signExecutor, recordingChain, _ := newReanchorSignFixture(
		t,
		false,
		true, // never confirm, so the broadcast step times out
		pendingReanchorActionState(targetPKH),
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the broadcast step to fail, got nil")
	}
	if !strings.Contains(
		err.Error(),
		"cannot broadcast reservation reanchor transaction",
	) {
		t.Fatalf(
			"expected the re-anchor broadcast error, got: [%v]",
			err,
		)
	}
	if signExecutor.startCalls() != 1 {
		t.Errorf(
			"expected exactly one sign call before the broadcast failure, got [%d]",
			signExecutor.startCalls(),
		)
	}
	if len(recordingChain.broadcastCalls()) == 0 {
		t.Fatal("expected the broadcast step to be attempted")
	}
}
