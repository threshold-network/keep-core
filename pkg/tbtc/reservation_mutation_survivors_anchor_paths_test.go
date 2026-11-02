// Behavior tests for the reservation anchor and re-anchor execute-path
// error returns that the 2026-10-17 mutation run left unguarded. Each test
// drives the real code path and fails under the specific mutated build
// that dropped the pinned error branch; all pass on the original code.
package tbtc

import (
	"crypto/sha256"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"go.uber.org/zap"
)

// reservationTestDepositor is a 20-byte hex address that satisfies
// Deposit.Script(); "0x12" (one byte) does not.
const (
	reservationTestDepositor = "0x0000000000000000000000000000000000000001"
	malformedDepositor       = "0x12"
)

// nonStandardScript is a one-op locking script that matches neither the
// P2SH/P2WSH inputs nor the P2PKH/P2WPKH inputs the reservation builders
// require.
var nonStandardScript = []byte{0x51}

// newAnchorErrorFixture builds an anchor action over a chain on which
// every read of the execute path succeeds up to the step the test
// targets. The event amount can be made unpackable, the funding output
// script can be made unspendable, and the validator read can be forced
// to fail.
func newAnchorErrorFixture(
	t *testing.T,
	custody wallet,
	walletPKH [20]byte,
	eventAmount uint64,
	fundingOutScript bitcoin.Script,
	validatorErr error,
) (*reservationAnchorAction, *custodySigningExecutor, *broadcastRecordingChain) {
	t.Helper()

	depositForScript := &Deposit{
		Depositor:           reservationTestDepositor,
		WalletPublicKeyHash: walletPKH,
	}
	depositScript, err := depositForScript.Script()
	if err != nil {
		t.Fatalf("cannot build the deposit script: [%v]", err)
	}
	if fundingOutScript == nil {
		fundingOutScript, err = bitcoin.PayToWitnessScriptHash(
			sha256.Sum256(depositScript),
		)
		if err != nil {
			t.Fatalf("cannot build the funding output script: [%v]", err)
		}
	}

	btcChain := newLocalBitcoinChain()
	fundingTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: fundingOutScript,
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
		Amount:              eventAmount,
		Depositor:           reservationTestDepositor,
	})
	chain.setDepositRequest(
		fundingTxHash,
		0,
		&DepositChainRequest{
			Amount:     100000,
			RevealedAt: time.Now(),
		},
	)
	chain.setReservationAction(pendingAnchorActionState(walletPKH))
	if validatorErr != nil {
		chain.setValidateReservationAnchorProposalErr(validatorErr)
	}

	anchorFee := big.NewInt(
		int64(ProtocolMinWalletTxSatPerVByteFee) * anchorVsize(t),
	)
	signExecutor := &custodySigningExecutor{custody: custody, fail: true}
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
			AnchorTxFee:               anchorFee,
		},
		300000,
		400000,
		recorder.waitForBlock,
		nil,
	)

	// Route the executor's broadcast step through a recording chain that
	// shares the same local chain, so the happy steps stay confirmable.
	recordingChain := &broadcastRecordingChain{localBitcoinChain: btcChain}
	action.transactionExecutor.btcChain = recordingChain

	return action, signExecutor, recordingChain
}

// TestReservationAnchorAction_RejectsValidatorError pins the
// ValidateReservationAnchorProposal error return in the anchor execute
// path: a validator rejection must fail execute with the wrap text, and
// signing must never be reached.
func TestReservationAnchorAction_RejectsValidatorError(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, signExecutor, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nil,
		errors.New("validator rejected the proposal"),
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the validator error to fail execute, got nil")
	}
	if !strings.Contains(err.Error(), "cannot validate reservation anchor proposal") {
		t.Fatalf("expected the validator wrap text, got: [%v]", err)
	}
	if got := signExecutor.startCalls(); got != 0 {
		t.Errorf(
			"expected signing to be unreached, got [%d] sign calls",
			got,
		)
	}
}

// anchorTimestampErrorChain is a rangeRevealChain whose
// GetLatestBlockTimestamp read fails on demand.
type anchorTimestampErrorChain struct {
	*rangeRevealChain
	timestampErr error
}

func (c *anchorTimestampErrorChain) GetLatestBlockTimestamp() (time.Time, error) {
	if c.timestampErr != nil {
		return time.Time{}, c.timestampErr
	}
	return c.localChain.GetLatestBlockTimestamp()
}

// TestReservationAnchorAction_RejectsHeadTimestampReadError pins the
// GetLatestBlockTimestamp error return in the anchor execute path: the
// failed read must fail the action with the wrap text, not pass a zero
// time into the reveal lookup.
func TestReservationAnchorAction_RejectsHeadTimestampReadError(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, _, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nil,
		nil,
	)
	inner := action.chain.(*rangeRevealChain)
	action.chain = &anchorTimestampErrorChain{
		rangeRevealChain: inner,
		timestampErr:     errors.New("chain head read failed"),
	}

	err := action.execute()
	if err == nil {
		t.Fatal("expected the chain head read error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot get chain head timestamp") {
		t.Fatalf("expected the chain head wrap text, got: [%v]", err)
	}
}

// TestReservationAnchorAction_RejectsUnpackableDepositEvent pins the
// unpack error return in the anchor execute path: a deposit whose reveal
// amount overflows int64 must fail the action with the wrap text, not
// hand a nil deposit into the validator or the assembly step.
func TestReservationAnchorAction_RejectsUnpackableDepositEvent(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, _, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		math.MaxUint64, // unpack fails: amount exceeds int64 range
		nil,
		nil,
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the unpack error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot unpack deposit revealed event") {
		t.Fatalf("expected the unpack wrap text, got: [%v]", err)
	}
}

// anchorActionReadErrorChain is a rangeRevealChain whose
// GetReservationAction read fails on demand.
type anchorActionReadErrorChain struct {
	*rangeRevealChain
	actionErr error
}

func (c *anchorActionReadErrorChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*ReservationAction, error) {
	if c.actionErr != nil {
		return nil, c.actionErr
	}
	return c.localChain.GetReservationAction(reservationKey, requestNonce)
}

// TestReservationAnchorAction_RejectsActionReadError pins the
// GetReservationAction error return in the anchor execute path: a failed
// read must fail the action with the wrap text, not hand a nil action
// into the type check.
func TestReservationAnchorAction_RejectsActionReadError(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, _, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nil,
		nil,
	)
	inner := action.chain.(*rangeRevealChain)
	action.chain = &anchorActionReadErrorChain{
		rangeRevealChain: inner,
		actionErr:        errors.New("reservation action read failed"),
	}

	err := action.execute()
	if err == nil {
		t.Fatal("expected the reservation action read error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot get reservation action") {
		t.Fatalf("expected the reservation action wrap text, got: [%v]", err)
	}
}

// TestReservationAnchorAction_RejectsUnspendableFundingOutput pins the
// AssembleReservationAnchorTransaction error return in the anchor execute
// path: a funding output whose script is not P2SH/P2WSH must fail the
// assembly step with the wrap text, not hand a nil unsigned transaction
// to signing.
func TestReservationAnchorAction_RejectsUnspendableFundingOutput(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, _, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nonStandardScript,
		nil,
	)

	err := action.execute()
	if err == nil {
		t.Fatal("expected the assembly error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot assemble reservation anchor transaction") {
		t.Fatalf("expected the assembly wrap text, got: [%v]", err)
	}
}
