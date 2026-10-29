package spv

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reanchorProofDecisionConfirmations is the confirmation count the decision
// tests submit with; the assembler is mocked, so only zero versus non-zero
// matters.
const reanchorProofDecisionConfirmations = uint(6)

// reanchorProofDecisionFixture builds a re-anchor action generation whose
// signed transaction is acceptable: a single input spending the reservation's
// anchor output, a single output paying the action's authorized target wallet,
// and the action generation in the Pending state. Every decision test breaks
// exactly one of those properties, so the positive control below keeps the
// fixture honest.
type reanchorProofDecisionFixture struct {
	btcChain  *localBitcoinChain
	spvChain  *localChain
	anchorTx  *bitcoin.Transaction
	reanchor  *bitcoin.Transaction
	proof     *bitcoin.SpvProof
	target    [20]byte
	key       *big.Int
	nonce     uint64
	metrics   *mockMetricsRecorder
	submitted []string
}

func newReanchorProofDecisionFixture(t *testing.T) *reanchorProofDecisionFixture {
	t.Helper()

	btcChain := newLocalBitcoinChain()
	spvChain := newLocalChain()

	anchorTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{Value: 600000}},
	}
	if err := btcChain.BroadcastTransaction(anchorTx); err != nil {
		t.Fatal(err)
	}

	target := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e, 0x63, 0x9e, 0xde, 0xde, 0x4c, 0x75, 0xe1, 0x84, 0x30, 0x7c}
	targetScript, err := bitcoin.PayToWitnessPublicKeyHash(target)
	if err != nil {
		t.Fatal(err)
	}

	reanchor := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTx.Hash(),
				OutputIndex:     0,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           590000,
			PublicKeyScript: targetScript,
		}},
	}
	if err := btcChain.BroadcastTransaction(reanchor); err != nil {
		t.Fatal(err)
	}

	fixture := &reanchorProofDecisionFixture{
		btcChain: btcChain,
		spvChain: spvChain,
		anchorTx: anchorTx,
		reanchor: reanchor,
		proof: &bitcoin.SpvProof{
			MerkleProof:    []byte{0x01},
			TxIndexInBlock: 2,
			BitcoinHeaders: []byte{0x03},
		},
		target:  target,
		key:     big.NewInt(4242),
		nonce:   3,
		metrics: &mockMetricsRecorder{counts: make(map[string]float64)},
	}
	fixture.setAction(tbtc.ReservationActionTypeReanchor, tbtc.ReservationActionStatePending, target)
	spvChain.submitReservationReanchorProofHook = func(
		_ *tbtc.BitcoinTxInfo,
		_ *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error {
		fixture.submitted = append(
			fixture.submitted,
			reservationKey.String(),
		)
		if requestNonce != fixture.nonce {
			t.Errorf("unexpected request nonce proof for: %d", requestNonce)
		}

		return nil
	}

	return fixture
}

// setAction installs the generation the submitter reads back from the Bridge.
func (f *reanchorProofDecisionFixture) setAction(
	actionType tbtc.ReservationActionType,
	state tbtc.ReservationActionState,
	targetWalletPublicKeyHash [20]byte,
) {
	f.spvChain.setReservationAction(f.key, f.nonce, &tbtc.ReservationAction{
		ActionType:                actionType,
		State:                     state,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TermSeconds:               86400,
		MinAmount:                 600000,
	})
}

// assembler returns a proof assembler that yields the given transaction and
// error, as the real one would for a transaction it cannot or can prove.
func (f *reanchorProofDecisionFixture) assembler(
	transaction *bitcoin.Transaction,
	err error,
) spvProofAssembler {
	return func(
		_ bitcoin.Hash,
		_ uint,
		_ bitcoin.Chain,
	) (*bitcoin.Transaction, *bitcoin.SpvProof, error) {
		if err != nil {
			return nil, nil, err
		}

		return transaction, f.proof, nil
	}
}

// submit drives the re-anchor proof submitter against the fixture's chains.
func (f *reanchorProofDecisionFixture) submit(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	assembler spvProofAssembler,
) error {
	return submitReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		f.btcChain,
		f.spvChain,
		assembler,
		f.metrics,
	)
}

// metricCount returns how often the recorder counted the metric.
func (f *reanchorProofDecisionFixture) metricCount(metric string) float64 {
	return f.metrics.counts["reservation_reanchor_proof_"+metric]
}

// assertRefused checks the submitter decided not to submit: it returned an
// error naming the reason, no proof reached the Bridge, and the attempt was
// counted as a failed submission rather than a successful one.
func (f *reanchorProofDecisionFixture) assertRefused(
	t *testing.T,
	err error,
	wantErr string,
) {
	t.Helper()

	if err == nil {
		t.Fatalf(
			"expected the submitter to refuse the generation with [%s], got "+
				"no error",
			wantErr,
		)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("expected an error containing [%s], got [%v]", wantErr, err)
	}
	if len(f.submitted) != 0 {
		t.Fatalf(
			"expected no proof to reach the Bridge, got [%d] submissions",
			len(f.submitted),
		)
	}
	if count := f.metricCount("submissions_failed_total"); count != 1 {
		t.Fatalf("expected one counted failed submission, got [%v]", count)
	}
	if count := f.metricCount("submissions_succeeded_total"); count != 0 {
		t.Fatalf("expected no counted successful submission, got [%v]", count)
	}
	if count := f.metricCount("submissions_total"); count != 1 {
		t.Fatalf("expected one counted submission attempt, got [%v]", count)
	}
}

// TestSubmitReservationReanchorProof_SubmitsAuthorizedGeneration is the
// positive control for the decision tests: an authorized, provable re-anchor
// generation is submitted, so a refusal test cannot pass merely because the
// fixture never works.
func TestSubmitReservationReanchorProof_SubmitsAuthorizedGeneration(t *testing.T) {
	f := newReanchorProofDecisionFixture(t)

	if err := f.submit(
		f.reanchor.Hash(),
		reanchorProofDecisionConfirmations,
		f.key,
		f.nonce,
		f.assembler(f.reanchor, nil),
	); err != nil {
		t.Fatalf("expected the authorized generation to be submitted: %v", err)
	}

	if len(f.submitted) != 1 || f.submitted[0] != f.key.String() {
		t.Fatalf(
			"expected one submission for reservation [%v], got [%v]",
			f.key,
			f.submitted,
		)
	}
	if count := f.metricCount("submissions_succeeded_total"); count != 1 {
		t.Fatalf("expected one counted successful submission, got [%v]", count)
	}
	if count := f.metricCount("submissions_failed_total"); count != 0 {
		t.Fatalf("expected no counted failed submission, got [%v]", count)
	}
}

// TestSubmitReservationReanchorProof_RefusesToSubmit covers every branch of
// the re-anchor proof path that decides whether a proof may be submitted: an
// unprovable transaction, a transaction that does not spend the anchor and pay
// the authorized target wallet, an unreadable or unauthorized action
// generation, malformed arguments, and a rejected submission.
func TestSubmitReservationReanchorProof_RefusesToSubmit(t *testing.T) {
	t.Run("proof assembly fails", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(nil, errors.New("simulated assembly failure")),
		)

		f.assertRefused(t, err, "failed to assemble transaction spv proof")
	})

	t.Run("transaction has two inputs", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		twoInputs := &bitcoin.Transaction{
			Inputs:  append(f.reanchor.Inputs, f.reanchor.Inputs[0]),
			Outputs: f.reanchor.Outputs,
		}

		err := f.submit(
			twoInputs.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(twoInputs, nil),
		)

		f.assertRefused(
			t,
			err,
			"reservation transaction must have exactly one input",
		)
	})

	t.Run("previous transaction cannot be fetched", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		unknownParent := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0xde, 0xad},
					OutputIndex:     0,
				},
			}},
			Outputs: f.reanchor.Outputs,
		}

		err := f.submit(
			unknownParent.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(unknownParent, nil),
		)

		f.assertRefused(t, err, "cannot fetch previous transaction")
	})

	t.Run("spent output index is beyond the previous transaction", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		outOfBounds := &bitcoin.Transaction{
			Inputs: []*bitcoin.TransactionInput{{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: f.anchorTx.Hash(),
					OutputIndex:     uint32(len(f.anchorTx.Outputs)),
				},
			}},
			Outputs: f.reanchor.Outputs,
		}

		err := f.submit(
			outOfBounds.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(outOfBounds, nil),
		)

		f.assertRefused(t, err, "out of bounds for previous transaction")
	})

	t.Run("transaction has two outputs", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		twoOutputs := &bitcoin.Transaction{
			Inputs: f.reanchor.Inputs,
			Outputs: append(
				f.reanchor.Outputs,
				f.reanchor.Outputs[0],
			),
		}

		err := f.submit(
			twoOutputs.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(twoOutputs, nil),
		)

		f.assertRefused(
			t,
			err,
			"reservation re-anchor transaction must have exactly one output",
		)
	})

	t.Run("output does not pay a wallet", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		notAWallet := &bitcoin.Transaction{
			Inputs: f.reanchor.Inputs,
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           590000,
				PublicKeyScript: []byte{0x6a},
			}},
		}

		err := f.submit(
			notAWallet.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(notAWallet, nil),
		)

		f.assertRefused(t, err, "cannot extract re-anchor public key hash")
	})

	t.Run("output pays a wallet other than the authorized target", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)
		f.setAction(
			tbtc.ReservationActionTypeReanchor,
			tbtc.ReservationActionStatePending,
			[20]byte{0xff},
		)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "target wallet public key hash mismatch")
	})

	t.Run("action generation is not a re-anchor", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)
		f.setAction(
			tbtc.ReservationActionTypeAcceptance,
			tbtc.ReservationActionStatePending,
			f.target,
		)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "not expected type")
	})

	t.Run("action generation is no longer settleable", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)
		f.setAction(
			tbtc.ReservationActionTypeReanchor,
			tbtc.ReservationActionStateSettled,
			f.target,
		)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "not settleable")
	})

	t.Run("action generation cannot be read", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)
		f.spvChain.getReservationActionErr = errors.New("simulated chain failure")

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "cannot fetch reservation action generation")
	})

	t.Run("submission is rejected", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)
		f.spvChain.submitReservationReanchorProofHook = func(
			_ *tbtc.BitcoinTxInfo,
			_ *tbtc.BitcoinTxProof,
			_ *big.Int,
			_ uint64,
		) error {
			return errors.New("simulated Bridge rejection")
		}

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		if err == nil {
			t.Fatal("expected the rejected submission to surface as an error")
		}
		if !strings.Contains(err.Error(), "failed to submit reservation proof") {
			t.Fatalf("expected a submission failure, got [%v]", err)
		}
		if count := f.metricCount("submissions_failed_total"); count != 1 {
			t.Fatalf("expected one counted failed submission, got [%v]", count)
		}
		if count := f.metricCount("submissions_succeeded_total"); count != 0 {
			t.Fatalf("expected no counted successful submission, got [%v]", count)
		}
	})

	t.Run("zero required confirmations", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		err := f.submit(
			f.reanchor.Hash(),
			0,
			f.key,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(
			t,
			err,
			"provided required confirmations count must be greater than 0",
		)
	})

	t.Run("nil reservation key", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			nil,
			f.nonce,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "reservation key is required")
	})

	t.Run("zero request nonce", func(t *testing.T) {
		f := newReanchorProofDecisionFixture(t)

		err := f.submit(
			f.reanchor.Hash(),
			reanchorProofDecisionConfirmations,
			f.key,
			0,
			f.assembler(f.reanchor, nil),
		)

		f.assertRefused(t, err, "request nonce must be > 0")
	})
}
