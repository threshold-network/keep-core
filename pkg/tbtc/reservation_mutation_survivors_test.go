package tbtc

import (
	"context"
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"go.uber.org/zap"
)

// These tests pin the boundaries and error paths in reservation.go that the
// 2026-10-17 mutation run found unguarded. Each test fails on the mutant it
// names and passes on the original code.

// failingSigningExecutor fails the signing step so a test can tell that
// execute reached it, without signing anything.
type failingSigningExecutor struct{}

func (e *failingSigningExecutor) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*tecdsa.Signature, error) {
	return nil, errors.New("signing reached")
}

func noBlockWait(context.Context, uint64) error { return nil }

// TestAssembleReservationTransactions_AcceptBoundaryValues pins the inclusive
// side of the fee and anchor-value checks: a fee of 1, a fee equal to the
// action's maximum, and a leftover anchor value of 1 must all be accepted.
func TestAssembleReservationTransactions_AcceptBoundaryValues(t *testing.T) {
	walletPublicKeyHash := [20]byte{0x01}

	scenarios := map[string]struct {
		value  int64
		maxFee uint64
		fee    int64
		anchor bool
	}{
		"anchor fee of 1": {
			value: 100000, maxFee: 2000, fee: 1, anchor: true,
		},
		"anchor fee equal to the maximum": {
			value: 100000, maxFee: 2000, fee: 2000, anchor: true,
		},
		"anchor value of 1": {
			value: 2001, maxFee: 2000, fee: 2000, anchor: true,
		},
		"reanchor fee of 1": {
			value: 100000, maxFee: 2000, fee: 1,
		},
		"reanchor fee equal to the maximum": {
			value: 100000, maxFee: 2000, fee: 2000,
		},
		"reanchor value of 1": {
			value: 2001, maxFee: 2000, fee: 2000,
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			deposit := &Deposit{
				Depositor:           "0x934b98637ca318a4d6e7ca6ffd1690b8e77df637",
				WalletPublicKeyHash: [20]byte{0xaa},
				RefundPublicKeyHash: [20]byte{0xbb},
				RefundLocktime:      [4]byte{0x60, 0xbc, 0xea, 0x61},
			}
			depositScript, err := deposit.Script()
			if err != nil {
				t.Fatal(err)
			}
			var inputScript []byte
			var err2 error
			if scenario.anchor {
				inputScript, err2 = bitcoin.PayToWitnessScriptHash(
					sha256.Sum256(depositScript),
				)
			} else {
				inputScript, err2 = bitcoin.PayToWitnessPublicKeyHash(
					walletPublicKeyHash,
				)
			}
			if err2 != nil {
				t.Fatal(err2)
			}
			fundingTx := &bitcoin.Transaction{
				Outputs: []*bitcoin.TransactionOutput{{
					Value:           scenario.value,
					PublicKeyScript: inputScript,
				}},
			}
			if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
				t.Fatal(err)
			}
			utxo := &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTx.Hash(),
					OutputIndex:     0,
				},
				Value: scenario.value,
			}
			deposit.Utxo = utxo
			action := &ReservationAction{TxMaxFee: scenario.maxFee}

			if scenario.anchor {
				_, err = AssembleReservationAnchorTransaction(
					bitcoinChain,
					deposit,
					walletPublicKeyHash,
					action,
					scenario.fee,
				)
			} else {
				_, err = AssembleReservationReanchorTransaction(
					bitcoinChain,
					utxo,
					walletPublicKeyHash,
					action,
					scenario.fee,
				)
			}
			if err != nil {
				t.Errorf("expected boundary value to be accepted, got: [%v]", err)
			}
		})
	}
}

// anchorScenario is the anchor execute fixture with the fields the mutation
// tests vary.
type anchorScenario struct {
	action     *ReservationAction
	eventIndex uint32
	proposal   *ReservationAnchorProposal
	fee        int64
	expiry     uint64
	signing    walletSigningExecutor
	monitor    *transactionMonitor
	// skipFundingTx leaves the funding transaction out of the chain, so the
	// fetch fails.
	skipFundingTx bool
}

func newAnchorScenario(t *testing.T, s anchorScenario) *reservationAnchorAction {
	t.Helper()

	const fundingOutputIndex = uint32(0)

	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value: 100000,
		}},
	}
	var fundingTxHash bitcoin.Hash
	if !s.skipFundingTx {
		depositForScript := &Deposit{
			Depositor:           "0x0000000000000000000000000000000000000001",
			WalletPublicKeyHash: walletPublicKeyHash,
		}
		depositScript, err := depositForScript.Script()
		if err != nil {
			t.Fatal(err)
		}
		fundingOutputScript, err := bitcoin.PayToWitnessScriptHash(
			sha256.Sum256(depositScript),
		)
		if err != nil {
			t.Fatal(err)
		}
		fundingTx.Outputs[0].PublicKeyScript = fundingOutputScript
		fundingTxHash = fundingTx.Hash()
	} else {
		fundingTxHash = bitcoin.Hash{0xff}
	}

	btcChain := newLocalBitcoinChain()
	if !s.skipFundingTx {
		if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
			t.Fatal(err)
		}
	}

	eventIndex := s.eventIndex
	if eventIndex == 0 && !s.skipFundingTx {
		eventIndex = fundingOutputIndex
	}
	chain := newRangeRevealChain(&DepositRevealedEvent{
		BlockNumber:         299990,
		FundingTxHash:       fundingTxHash,
		FundingOutputIndex:  eventIndex,
		WalletPublicKeyHash: walletPublicKeyHash,
		Amount:              100000,
		Depositor:           "0x0000000000000000000000000000000000000001",
	})
	chain.setDepositRequest(fundingTxHash, fundingOutputIndex, &DepositChainRequest{
		Amount:     100000,
		RevealedAt: time.Now(),
	})
	action := &ReservationAction{
		ActionType:                ReservationActionTypeAcceptance,
		State:                     ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  100000,
	}
	if s.action != nil {
		action = s.action
	}
	chain.setReservationAction(action)

	proposal := &ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: fundingOutputIndex,
		RequestNonce:              1,
		AnchorTxFee:               big.NewInt(s.fee),
	}
	if s.proposal != nil {
		proposal = s.proposal
	}
	if proposal.AnchorTxFee == nil {
		proposal.AnchorTxFee = big.NewInt(s.fee)
	}

	return newReservationAnchorAction(
		zap.NewNop().Sugar(),
		chain,
		btcChain,
		custodyWallet,
		s.signing,
		proposal,
		300000,
		s.expiry,
		noBlockWait,
		s.monitor,
	)
}

// TestReservationAnchorAction_AcceptsExpiryAtSigningMargin pins the expiry
// check as strict: an expiry exactly at the signing safety margin is valid,
// so execute must reach signing.
func TestReservationAnchorAction_AcceptsExpiryAtSigningMargin(t *testing.T) {
	fee := int64(ProtocolMinWalletTxSatPerVByteFee) * anchorVsize(t)

	err := newAnchorScenario(t, anchorScenario{
		fee:     fee,
		expiry:  reservationActionSigningTimeoutSafetyMarginBlocks,
		signing: &failingSigningExecutor{},
	}).execute()
	if err == nil || !strings.Contains(err.Error(), "error while signing") {
		t.Errorf(
			"expected execute to reach signing at the margin, got: [%v]",
			err,
		)
	}
}

// TestReservationReanchorAction_AcceptsExpiryAtSigningMargin is the
// re-anchor counterpart.
func TestReservationReanchorAction_AcceptsExpiryAtSigningMargin(t *testing.T) {
	fixture := newAgedReanchorFixture(t)
	fee := big.NewInt(int64(ProtocolMinWalletTxSatPerVByteFee) * reanchorVsize(t))

	action := fixture.action(
		fixture.chain,
		fee,
		reservationActionSigningTimeoutSafetyMarginBlocks,
	)
	action.transactionExecutor.signingExecutor = &failingSigningExecutor{}
	action.transactionExecutor.waitForBlockFn = noBlockWait

	err := action.execute()
	if err == nil || !strings.Contains(err.Error(), "error while signing") {
		t.Errorf(
			"expected execute to reach signing at the margin, got: [%v]",
			err,
		)
	}
}

// TestReservationAnchorAction_RejectsNonPendingAction pins the early return
// when the on-chain action is not a pending acceptance.
func TestReservationAnchorAction_RejectsNonPendingAction(t *testing.T) {
	fee := int64(ProtocolMinWalletTxSatPerVByteFee) * anchorVsize(t)

	custodyWallet := reservationTestWallet(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(custodyWallet.publicKey)

	err := newAnchorScenario(t, anchorScenario{
		fee:    fee,
		expiry: lowExpiryBlock,
		action: &ReservationAction{
			ActionType:                ReservationActionTypeAcceptance,
			State:                     ReservationActionStateSettled,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  100000,
		},
	}).execute()
	if err == nil || err.Error() != "reservation action is not a pending acceptance" {
		t.Errorf("expected a non-pending action rejection, got: [%v]", err)
	}
}

// TestReservationReanchorAction_RejectsNonPendingAction is the re-anchor
// counterpart.
func TestReservationReanchorAction_RejectsNonPendingAction(t *testing.T) {
	fixture := newAgedReanchorFixture(t)
	chain := fixture.chain
	chain.setReservationAction(&ReservationAction{
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStateSettled,
		TargetWalletPublicKeyHash: fixture.targetWalletPKH,
		TxMaxFee:                  100000,
	})

	fee := big.NewInt(int64(ProtocolMinWalletTxSatPerVByteFee) * reanchorVsize(t))
	err := fixture.action(chain, fee, lowExpiryBlock).execute()
	if err == nil || err.Error() != "reservation action is not a pending reanchor" {
		t.Errorf("expected a non-pending action rejection, got: [%v]", err)
	}
}

// TestReservationAnchorAction_RejectsWrongFundingOutputIndex pins the event
// filter: a reveal with the right funding hash but a different output index
// must not match.
func TestReservationAnchorAction_RejectsWrongFundingOutputIndex(t *testing.T) {
	fee := int64(ProtocolMinWalletTxSatPerVByteFee) * anchorVsize(t)

	err := newAnchorScenario(t, anchorScenario{
		fee:        fee,
		expiry:     lowExpiryBlock,
		eventIndex: 3,
	}).execute()
	if err == nil || err.Error() != "no matching DepositRevealed event for deposit" {
		t.Errorf("expected no event match, got: [%v]", err)
	}
}

// TestReservationAnchorAction_RejectsFundingFetchError pins the early return
// when the funding transaction cannot be fetched.
func TestReservationAnchorAction_RejectsFundingFetchError(t *testing.T) {
	fee := int64(ProtocolMinWalletTxSatPerVByteFee) * anchorVsize(t)

	err := newAnchorScenario(t, anchorScenario{
		fee:           fee,
		expiry:        lowExpiryBlock,
		skipFundingTx: true,
	}).execute()
	if err == nil || !strings.HasPrefix(err.Error(), "cannot fetch funding transaction:") {
		t.Errorf("expected a funding fetch error, got: [%v]", err)
	}
}

// TestReservationActionConstructors_WireTransactionMonitor pins that both
// constructors pass the monitor to the transaction executor.
func TestReservationActionConstructors_WireTransactionMonitor(t *testing.T) {
	monitor := &transactionMonitor{}
	custodyWallet := reservationTestWallet(t)
	chain := Connect()

	anchorAction := newReservationAnchorAction(
		zap.NewNop().Sugar(),
		chain,
		newLocalBitcoinChain(),
		custodyWallet,
		nil,
		&ReservationAnchorProposal{AnchorTxFee: big.NewInt(1)},
		300000,
		400000,
		noBlockWait,
		monitor,
	)
	if anchorAction.transactionExecutor.transactionMonitor != monitor {
		t.Error("anchor action did not wire the transaction monitor")
	}

	reanchorAction := newReservationReanchorAction(
		zap.NewNop().Sugar(),
		chain,
		newLocalBitcoinChain(),
		custodyWallet,
		nil,
		&ReservationReanchorProposal{ReanchorTxFee: big.NewInt(1)},
		300000,
		400000,
		noBlockWait,
		monitor,
	)
	if reanchorAction.transactionExecutor.transactionMonitor != monitor {
		t.Error("re-anchor action did not wire the transaction monitor")
	}
}

// TestReservationRPCCallWithTimeoutErr_ReleasesSlotAfterTimeout pins that a timed-out
// call does not hold its RPC slot forever: the result channel must be
// buffered so the worker can finish after execute has moved on.
func TestReservationRPCCallWithTimeoutErr_ReleasesSlotAfterTimeout(t *testing.T) {
	oldTimeout := reservationRPCRequestTimeout
	reservationRPCRequestTimeout = 20 * time.Millisecond
	defer func() { reservationRPCRequestTimeout = oldTimeout }()

	release := make(chan struct{})
	callReturned := make(chan struct{})

	err := rpcCallWithTimeoutErr(
		func() error {
			<-release
			close(callReturned)
			return nil
		},
		"probe",
	)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got: [%v]", err)
	}

	close(release)
	select {
	case <-callReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("the timed-out call never finished")
	}

	// All slots must be free again. A worker blocked on an unbuffered result
	// channel would still hold one, so cap-1 slots would be all we can take.
	acquired := 0
	defer func() {
		for i := 0; i < acquired; i++ {
			<-reservationRPCSlots
		}
	}()
	deadline := time.After(2 * time.Second)
	for acquired < cap(reservationRPCSlots) {
		select {
		case reservationRPCSlots <- struct{}{}:
			acquired++
		case <-deadline:
			t.Fatalf(
				"only %d of %d RPC slots are free: a timed-out call kept its slot, so its worker is blocked",
				acquired,
				cap(reservationRPCSlots),
			)
		}
	}
}
