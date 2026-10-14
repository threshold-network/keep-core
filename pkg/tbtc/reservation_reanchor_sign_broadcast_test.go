// Re-anchor execute() sign-step and broadcast-step tests, plus their on-chain setup.
package tbtc

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"go.uber.org/zap"
)

// reanchorActionState builds a pending re-anchor action targeting the given
// public key hash.
func pendingReanchorActionState(targetPKH [20]byte) *ReservationAction {
	return &ReservationAction{
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetPKH,
		TxMaxFee:                  2000,
	}
}

// newReanchorSignFixture builds a re-anchor action wired for sign-step
// observation: a recording signing executor and a recording Bitcoin chain on
// the executor, plus a block recorder standing in for the wait-for-block
// function. All on-chain reads of the re-anchor execute path succeed up to
// the signing step.
func newReanchorSignFixture(
	t *testing.T,
	wantSignFailure bool,
	neverConfirm bool,
	actionState *ReservationAction,
) (
	*reservationReanchorAction,
	*custodySigningExecutor,
	*broadcastRecordingChain,
	*blockRecorder,
) {
	t.Helper()

	custody, _ := newCustodyWallet(t)
	targetPKH := [20]byte{0x02}

	chain, btcChain := setupReanchorOnChain(t, custody, targetPKH)
	chain.setReservationAction(actionState)

	signExecutor := &custodySigningExecutor{custody: custody, fail: wantSignFailure}
	recorder := newBlockRecorder()

	action := newReservationReanchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custody,
		signExecutor,
		&ReservationReanchorProposal{
			ReservationKey:            big.NewInt(777),
			RequestNonce:              1,
			TargetWalletPublicKeyHash: targetPKH,
			ReanchorTxFee:             big.NewInt(1500),
		},
		300000,
		400000,
		recorder.waitForBlock,
		nil,
	)

	recordingChain := &broadcastRecordingChain{
		localBitcoinChain: btcChain,
		neverConfirm:      neverConfirm,
	}
	action.transactionExecutor.btcChain = recordingChain

	return action, signExecutor, recordingChain, recorder
}

func TestReservationReanchorAction_SignError(t *testing.T) {
	targetPKH := [20]byte{0x02}

	action, signExecutor, recordingChain, recorder := newReanchorSignFixture(
		t,
		true,
		false,
		pendingReanchorActionState(targetPKH),
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the sign step to fail, got nil")
	}
	wanted := "cannot sign reservation reanchor transaction: [error while signing transaction's sig hashes: [forced custody signing failure]]"
	if err.Error() != wanted {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%v]",
			wanted,
			err,
		)
	}

	if signExecutor.startCalls() != 1 {
		t.Errorf("expected exactly one sign call, got [%d]", signExecutor.startCalls())
	}
	if len(recordingChain.broadcastCalls()) != 0 {
		t.Errorf(
			"expected no broadcast after a signing failure, got [%d]",
			len(recordingChain.broadcastCalls()),
		)
	}

	// The sign step was called with the deadline expiryBlock - margin.
	if got := recorder.lastDeadline(); got != 400000-reservationActionSigningTimeoutSafetyMarginBlocks {
		t.Errorf(
			"expected the sign deadline to be [%d], got [%d]",
			400000-reservationActionSigningTimeoutSafetyMarginBlocks,
			got,
		)
	}
}

func TestReservationReanchorAction_Success(t *testing.T) {
	targetPKH := [20]byte{0x02}

	action, signExecutor, recordingChain, recorder := newReanchorSignFixture(
		t,
		false,
		false,
		pendingReanchorActionState(targetPKH),
	)

	err := action.execute()
	if err != nil {
		t.Fatalf("expected the re-anchor action to complete, got: [%v]", err)
	}

	if signExecutor.startCalls() != 1 {
		t.Errorf("expected exactly one sign call, got [%d]", signExecutor.startCalls())
	}

	broadcasts := recordingChain.broadcastCalls()
	if len(broadcasts) != 1 {
		t.Fatalf("expected one broadcast call, got [%d]", len(broadcasts))
	}
	// The broadcast step received the signed transaction produced by the
	// sign step.
	broadcastedTx := broadcasts[0]
	if broadcastedTx == nil || broadcastedTx.Hash() == (bitcoin.Hash{}) {
		t.Fatalf("the broadcasted transaction must be a concrete signed transaction")
	}
	if _, lookupErr := action.btcChain.GetTransaction(broadcastedTx.Hash()); lookupErr != nil {
		t.Fatalf(
			"the signed re-anchor transaction is missing from the Bitcoin chain: [%v]",
			lookupErr,
		)
	}

	if got := recorder.lastDeadline(); got != 400000-reservationActionSigningTimeoutSafetyMarginBlocks {
		t.Errorf(
			"expected the sign deadline to be [%d], got [%d]",
			400000-reservationActionSigningTimeoutSafetyMarginBlocks,
			got,
		)
	}
}

func TestReservationReanchorAction_InvalidExpiry(t *testing.T) {
	targetPKH := [20]byte{0x02}

	action, signExecutor, _, _ := newReanchorSignFixture(
		t,
		false,
		false,
		pendingReanchorActionState(targetPKH),
	)
	action.expiryBlock = 100 // below the margin

	err := action.execute()
	if err == nil || err.Error() != "invalid proposal expiry block" {
		t.Fatalf(
			"unexpected error\nexpected: [invalid proposal expiry block]\nactual:   [%v]",
			err,
		)
	}
	if signExecutor.startCalls() != 0 {
		t.Errorf(
			"signing must not run when the expiry block is invalid, got [%d]",
			signExecutor.startCalls(),
		)
	}
}

func TestReservationReanchorAction_PendingRecheck(t *testing.T) {
	targetPKH := [20]byte{0x02}

	cases := map[string]*ReservationAction{
		"settled": {
			ActionType:                ReservationActionTypeReanchor,
			State:                     ReservationActionStateSettled,
			TargetWalletPublicKeyHash: targetPKH,
			TxMaxFee:                  2000,
		},
		"wrong type": {
			ActionType:                ReservationActionTypeAcceptance,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: targetPKH,
			TxMaxFee:                  2000,
		},
	}

	for name, actionState := range cases {
		t.Run(name, func(t *testing.T) {
			action, signExecutor, recordingChain, _ := newReanchorSignFixture(
				t,
				false,
				false,
				actionState,
			)

			err := action.execute()
			if err == nil || err.Error() != "reservation action is not a pending reanchor" {
				t.Fatalf(
					"unexpected error\nexpected: [reservation action is not a pending reanchor]\nactual:   [%v]",
					err,
				)
			}
			if signExecutor.startCalls() != 0 {
				t.Errorf(
					"signing must not run for a non-pending action, got [%d]",
					signExecutor.startCalls(),
				)
			}
			if len(recordingChain.broadcastCalls()) != 0 {
				t.Errorf(
					"broadcast must not run for a non-pending action, got [%d]",
					len(recordingChain.broadcastCalls()),
				)
			}
		})
	}
}

// setupReanchorOnChain builds the on-chain environment for the re-anchor
// execute path against the given custody wallet and target PKH: a reservation
// whose anchor outpoint is spent by the custody wallet, a pending re-anchor
// action, and the target-wallet age-gate reads.
func setupReanchorOnChain(
	t *testing.T,
	custody wallet,
	targetPKH [20]byte,
) (*localChain, *localBitcoinChain) {
	t.Helper()

	walletPKH := bitcoin.PublicKeyHash(custody.publicKey)

	btcChain := newLocalBitcoinChain()
	anchorOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPKH)
	if err != nil {
		t.Fatalf("cannot build the anchor output script: [%v]", err)
	}
	priorAnchorTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 10000},
			{Value: 100000, PublicKeyScript: anchorOutputScript},
		},
	}
	if err := btcChain.BroadcastTransaction(priorAnchorTx); err != nil {
		t.Fatalf("cannot broadcast the prior anchor transaction: [%v]", err)
	}

	headTime := time.Unix(1_700_000_000, 0)

	chain := Connect()
	chain.setReservation(&Reservation{
		WalletPublicKeyHash: walletPKH,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: priorAnchorTx.Hash(),
				OutputIndex:     1,
			},
			Value: 100000,
		},
	})
	chain.setReservationAction(&ReservationAction{
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetPKH,
		TxMaxFee:                  2000,
	})
	// Target wallet is well past the creation period so the age gate passes.
	chain.setWallet(targetPKH, &WalletChainData{
		CreatedAt: headTime.Add(
			-2 * time.Duration(reanchorTestCreationPeriod) * time.Second,
		),
	})
	chain.SetWalletParameters(WalletParameters{
		CreationPeriod: reanchorTestCreationPeriod,
	})
	chain.SetLatestBlockTimestamp(headTime)

	return chain, btcChain
}
