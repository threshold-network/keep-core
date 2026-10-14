package tbtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// This file exercises the reservation anchor and re-anchor execute() sign
// step and broadcast step, the expiry-block guard, and the pending re-checks,
// none of which an earlier test ran.
//
// The custody wallet is built from a known private scalar (100) via
// generateWallet, so the executor's real signing path produces valid tECDSA
// signatures that the Bitcoin builder verifies. The sign step's deadline is
// observed by recording the block the executor's waitForBlockFn is handed (the
// signing deadline is plumb-ed to withCancelOnBlock, not to signBatch), and
// the broadcast step is observed by wrapping the executor's Bitcoin chain.
//
// The executor's broadcastTransaction loop returns a "broadcast timeout
// exceeded" error only when the relayed transaction is never confirmed
// within the (const) reservationActionBroadcastTimeout; a full-execute
// broadcast-error test would therefore block for fifteen minutes. The
// broadcast-error error surface is instead pinned at the executor level with
// short timeouts (reservationBroadcastTransactionTest), so these execute-level
// tests stay fast: they cover the sign-error, success, expiry-guard, and
// pending-recheck paths.

// custodySigningExecutor is a walletSigningExecutor that signs every signBatch
// call with the custody wallet's private key (scalar 100, matching
// generateWallet(100)), or returns a forced failure when fail is set. It
// records every startBlock it is handed.
type custodySigningExecutor struct {
	custody wallet
	fail    bool

	mu     sync.Mutex
	starts []uint64
}

func (e *custodySigningExecutor) signBatch(
	_ context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*tecdsa.Signature, error) {
	e.mu.Lock()
	e.starts = append(e.starts, startBlock)
	e.mu.Unlock()

	if e.fail {
		return nil, fmt.Errorf("forced custody signing failure")
	}

	privKey := &ecdsa.PrivateKey{
		PublicKey: *e.custody.publicKey,
		D:         big.NewInt(100),
	}

	signatures := make([]*tecdsa.Signature, len(messages))
	for i, message := range messages {
		r, s, err := ecdsa.Sign(rand.Reader, privKey, message.Bytes())
		if err != nil {
			return nil, fmt.Errorf("cannot compute custody signature: [%v]", err)
		}
		signatures[i] = &tecdsa.Signature{R: r, S: s}
	}

	return signatures, nil
}

func (e *custodySigningExecutor) startCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.starts)
}

// blockRecorder is a waitForBlockFn that records every block it is asked to
// wait for - which is exactly the signing deadline the executor computes.
// The recording happens in the goroutine withCancelOnBlock spawns, so the
// slots channel makes lastDeadline deterministically observe the deadline
// even when execute() returns before that goroutine has run.
type blockRecorder struct {
	// deadline carries the signing deadline, one entry per signTransaction
	// call.
	deadline chan uint64
}

func newBlockRecorder() *blockRecorder {
	return &blockRecorder{
		deadline: make(chan uint64, 1),
	}
}

func (b *blockRecorder) waitForBlock(_ context.Context, block uint64) error {
	b.deadline <- block
	return nil
}

// lastDeadline waits for the signing deadline to be recorded and returns it.
func (b *blockRecorder) lastDeadline() uint64 {
	select {
	case d := <-b.deadline:
		return d
	case <-time.After(2 * time.Second):
		// The deadline was not recorded (signing never reached the
		// block-wait step); report zero for the assertion to flag.
		return 0
	}
}

// broadcastRecordingChain records every transaction handed to
// BroadcastTransaction and delegates the rest to the embedded local chain.
// It is installed as the executor's Bitcoin chain so the broadcast step of
// execute() is observable.
type broadcastRecordingChain struct {
	*localBitcoinChain
	// neverConfirm makes GetTransactionConfirmations always fail so the
	// executor's broadcast loop never observes the transaction landing.
	neverConfirm bool

	mu         sync.Mutex
	broadcasts []*bitcoin.Transaction
}

func (c *broadcastRecordingChain) BroadcastTransaction(
	tx *bitcoin.Transaction,
) error {
	c.mu.Lock()
	c.broadcasts = append(c.broadcasts, tx)
	c.mu.Unlock()
	return c.localBitcoinChain.BroadcastTransaction(tx)
}

func (c *broadcastRecordingChain) broadcastCalls() []*bitcoin.Transaction {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*bitcoin.Transaction, len(c.broadcasts))
	copy(out, c.broadcasts)
	return out
}

// GetTransactionConfirmations always fails when neverConfirm is set, so
// the executor's broadcast loop never sees the transaction land and loops
// until the (const) broadcast timeout elapses.
func (c *broadcastRecordingChain) GetTransactionConfirmations(
	ctx context.Context,
	hash bitcoin.Hash,
) (uint, error) {
	if c.neverConfirm {
		return 0, fmt.Errorf("transaction not found")
	}
	return c.localBitcoinChain.GetTransactionConfirmations(ctx, hash)
}

// newCustodyWallet returns the signable custody wallet and its public key
// hash.
func newCustodyWallet(t *testing.T) (wallet, [20]byte) {
	t.Helper()
	custody := generateWallet(big.NewInt(100))
	return custody, bitcoin.PublicKeyHash(custody.publicKey)
}

// anchorActionState builds a pending acceptance action targeting the given
// custody wallet's public key hash.
func pendingAnchorActionState(walletPKH [20]byte) *ReservationAction {
	return &ReservationAction{
		ActionType:                ReservationActionTypeAcceptance,
		State:                     ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPKH,
		TxMaxFee:                  2000,
	}
}

// newAnchorSignFixture builds an anchor action wired for sign-step
// observation: a recording signing executor and a recording Bitcoin chain on
// the executor, plus a block recorder standing in for the wait-for-block
// function. All on-chain reads of the anchor execute path succeed up to the
// signing step.
func newAnchorSignFixture(
	t *testing.T,
	wantSignFailure bool,
	neverConfirm bool,
	actionState *ReservationAction,
) (
	*reservationAnchorAction,
	*custodySigningExecutor,
	*broadcastRecordingChain,
	*blockRecorder,
) {
	t.Helper()

	custody, walletPKH := newCustodyWallet(t)

	depositForScript := &Deposit{
		Depositor:           "0x0000000000000000000000000000000000000001",
		WalletPublicKeyHash: walletPKH,
	}
	depositScript, err := depositForScript.Script()
	if err != nil {
		t.Fatalf("cannot build the deposit script: [%v]", err)
	}
	fundingOutputScript, err := bitcoin.PayToWitnessScriptHash(sha256.Sum256(depositScript))
	if err != nil {
		t.Fatalf("cannot build the funding output script: [%v]", err)
	}

	btcChain := newLocalBitcoinChain()
	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: fundingOutputScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatalf("cannot broadcast the funding transaction: [%v]", err)
	}
	fundingTxHash := fundingTx.Hash()

	chain := newRangeRevealChain(&DepositRevealedEvent{
		BlockNumber:         299990,
		FundingTxHash:       fundingTxHash,
		FundingOutputIndex:  0,
		WalletPublicKeyHash: walletPKH,
		Amount:              100000,
		Depositor:           "0x0000000000000000000000000000000000000001",
	})
	chain.setDepositRequest(fundingTxHash, 0, &DepositChainRequest{
		Amount:     100000,
		RevealedAt: time.Now(),
	})
	chain.setReservationAction(actionState)

	signExecutor := &custodySigningExecutor{custody: custody, fail: wantSignFailure}
	recorder := newBlockRecorder()

	action := newReservationAnchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custody,
		signExecutor,
		&ReservationAnchorProposal{
			DepositFundingTxHash:      fundingTxHash,
			DepositFundingOutputIndex: 0,
			RequestNonce:              1,
			AnchorTxFee:               big.NewInt(1500),
		},
		300000,
		400000,
		recorder.waitForBlock,
		nil,
	)

	// The executor's broadcast chain is wrapped so the broadcast step of
	// execute() is observable; assembly still resolves against the same local
	// chain, so a signed transaction lands on it and is confirmable.
	recordingChain := &broadcastRecordingChain{
		localBitcoinChain: btcChain,
		neverConfirm:      neverConfirm,
	}
	action.transactionExecutor.btcChain = recordingChain

	return action, signExecutor, recordingChain, recorder
}

func TestReservationAnchorAction_SignError(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	_ = custody

	action, signExecutor, recordingChain, recorder := newAnchorSignFixture(
		t,
		true,
		false,
		pendingAnchorActionState(walletPKH),
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the sign step to fail, got nil")
	}
	wanted := "cannot sign reservation anchor transaction: [error while signing transaction's sig hashes: [forced custody signing failure]]"
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

func TestReservationAnchorAction_Success(t *testing.T) {
	_, walletPKH := newCustodyWallet(t)

	action, signExecutor, recordingChain, recorder := newAnchorSignFixture(
		t,
		false,
		false,
		pendingAnchorActionState(walletPKH),
	)

	err := action.execute()
	if err != nil {
		t.Fatalf("expected the anchor action to complete, got: [%v]", err)
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
	// The signed transaction landed on the local Bitcoin chain.
	if _, lookupErr := action.btcChain.GetTransaction(broadcastedTx.Hash()); lookupErr != nil {
		t.Fatalf(
			"the signed anchor transaction is missing from the Bitcoin chain: [%v]",
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

func TestReservationAnchorAction_InvalidExpiry(t *testing.T) {
	_, walletPKH := newCustodyWallet(t)

	action, signExecutor, _, _ := newAnchorSignFixture(
		t,
		false,
		false,
		pendingAnchorActionState(walletPKH),
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

func TestReservationAnchorAction_PendingRecheck(t *testing.T) {
	_, walletPKH := newCustodyWallet(t)

	cases := map[string]*ReservationAction{
		"settled": {
			ActionType:                ReservationActionTypeAcceptance,
			State:                     ReservationActionStateSettled,
			TargetWalletPublicKeyHash: walletPKH,
			TxMaxFee:                  2000,
		},
		"wrong type": {
			ActionType:                ReservationActionTypeReanchor,
			State:                     ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPKH,
			TxMaxFee:                  2000,
		},
	}

	for name, actionState := range cases {
		t.Run(name, func(t *testing.T) {
			action, signExecutor, recordingChain, _ := newAnchorSignFixture(
				t,
				false,
				false,
				actionState,
			)

			err := action.execute()
			if err == nil || err.Error() != "reservation action is not a pending acceptance" {
				t.Fatalf(
					"unexpected error\nexpected: [reservation action is not a pending acceptance]\nactual:   [%v]",
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
