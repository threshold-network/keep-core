package tbtcpg_test

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	pkgtest "github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

var reanchorTestLogger = log.Logger("keep-tbtcpg-test")

func TestReservationReanchorTask_Run(t *testing.T) {
	scenarios, err := pkgtest.LoadReservationReanchorTestScenario()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			// findTargetWallet now bounds its wallet-registration scan to
			// ReservationReanchorLookBackBlocks; a small current block keeps
			// the computed StartBlock at 0, matching the filter used below.
			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(1000)
			tbtcChain.SetBlockCounter(blockCounter)

			mainUtxoHash := scenario.SourceWalletMainUtxoHashBytes
			if scenario.SourceWalletMainUtxoTxHash != "" &&
				scenario.SourceWalletMainUtxoTxHash != "0000000000000000000000000000000000000000000000000000000000000000" {
				walletScript, err := bitcoin.PayToWitnessPublicKeyHash(
					scenario.SourceWalletPublicKeyHash,
				)
				if err != nil {
					t.Fatal(err)
				}

				mainUtxoTx := &bitcoin.Transaction{
					Version: 1,
					Outputs: []*bitcoin.TransactionOutput{{
						Value:           scenario.SourceWalletMainUtxoValue,
						PublicKeyScript: walletScript,
					}},
				}
				// DetermineWalletMainUtxo builds the candidate outpoint from
				// transaction.Hash() (the tx's own computed hash), not from
				// whatever key it happens to be stored under - both must
				// agree, so derive the storage key from the same call.
				mainUtxoTxHash := mainUtxoTx.Hash()
				btcChain.SetTransaction(mainUtxoTxHash, mainUtxoTx)
				btcChain.SetTxHashesForPublicKeyHash(
					scenario.SourceWalletPublicKeyHash,
					[]bitcoin.Hash{mainUtxoTxHash},
				)

				mainUtxo := &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: mainUtxoTxHash,
						OutputIndex:     scenario.SourceWalletMainUtxoTxIndex,
					},
					Value: scenario.SourceWalletMainUtxoValue,
				}
				mainUtxoHash = tbtcChain.ComputeMainUtxoHash(mainUtxo)
			}

			tbtcChain.SetWallet(
				scenario.SourceWalletPublicKeyHash,
				&tbtc.WalletChainData{
					State:        scenario.SourceWalletState,
					MainUtxoHash: mainUtxoHash,
				},
			)

			tbtcChain.SetMovingFundsParameters(
				1000000,
				scenario.MovingFundsDustThreshold,
				0,
				0,
				nil,
				0,
				0,
				0,
				0,
				nil,
				0,
			)

			reservationKeys := make([]*big.Int, 0, len(scenario.Reservations))
			for _, r := range scenario.Reservations {
				reservationKeys = append(reservationKeys, r.ReservationKey)

				anchorTxHash, err := bitcoin.NewHashFromString(
					r.AnchorTxHash,
					bitcoin.ReversedByteOrder,
				)
				if err != nil {
					t.Fatal(err)
				}

				anchorWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(
					r.WalletPublicKeyHash,
				)
				if err != nil {
					t.Fatal(err)
				}
				anchorOutputs := make([]*bitcoin.TransactionOutput, r.AnchorTxOutputIndex+1)
				for i := range anchorOutputs {
					anchorOutputs[i] = &bitcoin.TransactionOutput{Value: 1}
				}
				anchorOutputs[r.AnchorTxOutputIndex] = &bitcoin.TransactionOutput{
					Value:           r.AnchorValue,
					PublicKeyScript: anchorWalletScript,
				}
				btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
					Version: 1,
					Outputs: anchorOutputs,
				})

				reservationState := r.State
				if r.HasPendingAction || r.PendingActionState == tbtc.ReservationActionStatePending {
					// On-chain, a reservation with a pending action is in ActionPending state.
					reservationState = tbtc.ReservationStateActionPending
				}

				tbtcChain.SetReservation(r.ReservationKey, &tbtc.Reservation{
					WalletPublicKeyHash: r.WalletPublicKeyHash,
					AnchorUtxo: &bitcoin.UnspentTransactionOutput{
						Outpoint: &bitcoin.TransactionOutpoint{
							TransactionHash: anchorTxHash,
							OutputIndex:     r.AnchorTxOutputIndex,
						},
						Value: r.AnchorValue,
					},
					State:        reservationState,
					RequestNonce: r.RequestNonce,
				})

				// Always install an action record when RequestNonce > 0 so
				// hasPendingAction's real GetReservationAction lookup
				// succeeds and evaluates State directly, matching what a
				// real chain would have: RequestNonce only ever advances
				// alongside a real action record. HasPendingAction=false
				// scenarios use r.PendingActionState (a terminal state, not
				// Pending) to model an already-settled prior generation.
				// HasPendingAction=true scenarios install a Pending Reanchor
				// action authorizing the scenario's TargetWalletPublicKeyHash
				// (the resume path's required target) and carrying the
				// scenario's snapshotted fee bound (so the resumed fee
				// estimate mirrors what the original request captured).
				if r.RequestNonce > 0 {
					action := &tbtc.ReservationAction{
						ActionType: tbtc.ReservationActionTypeReanchor,
						State:      r.PendingActionState,
						// Cancel the signer-time safety margin by default:
						// a zero TimeoutAt makes the production resume
						// path skip the generation (its margin gate sees
						// now+7200 >= 0). Scenarios not exercising the
						// gate default to far-future so they don't trip
						// it; scenarios that exercise it set TimeoutAt
						// explicitly.
						TimeoutAt: r.TimeoutAt,
					}
					if r.TimeoutAt == 0 {
						action.TimeoutAt = uint32(
							time.Now().Add(24 * time.Hour).Unix(),
						)
					}
					if r.HasPendingAction {
						action.TargetWalletPublicKeyHash = scenario.TargetWalletPublicKeyHash
						action.TxMaxFee = scenario.ReservationTxMaxFee
						action.MinAmount = 1000
						action.TermSeconds = 86400
					}
					tbtcChain.SetReservationAction(
						r.ReservationKey,
						r.RequestNonce,
						action,
					)
				}
			}
			tbtcChain.SetWalletReservations(
				scenario.SourceWalletPublicKeyHash,
				reservationKeys,
			)

			// Fall back to a non-zero MaxReservationsPerWallet when the
			// scenario omits it: the headroom pre-check used by
			// findTargetWallet rejects every candidate when the cap is
			// zero (count + 1 > 0 is always true), so a zero default
			// would silently turn every scenario into "no live target".
			maxPerWallet := scenario.MaxReservationsPerWallet
			if maxPerWallet == 0 {
				maxPerWallet = 5
			}
			tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
				ReservationTxMaxFee:      scenario.ReservationTxMaxFee,
				MaxReservationsPerWallet: maxPerWallet,
			})

			btcChain.SetEstimateSatPerVByteFee(1, scenario.EstimateSatPerVByteFee)

			// Unconditionally register the source wallet itself in the
			// same past-registration-events bucket findTargetWallet
			// queries (filter{StartBlock: 0}), even for scenarios with no
			// target wallet. findLiveWalletFromRegistrationEvents always
			// skips a registration matching the source wallet, so this is
			// inert for target selection; its only purpose is to give the
			// mock chain a populated entry so PastNewWalletRegisteredEvents
			// returns an (empty-after-filtering) slice instead of its
			// "nothing ever registered for this filter" sentinel error --
			// matching a real chain's behavior of returning an empty event
			// list, never an error, when nothing matches.
			if err := tbtcChain.AddPastNewWalletRegisteredEvent(
				&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
				&tbtc.NewWalletRegisteredEvent{
					WalletPublicKeyHash: scenario.SourceWalletPublicKeyHash,
				},
			); err != nil {
				t.Fatal(err)
			}

			if scenario.TargetWalletPublicKeyHash != [20]byte{} {
				err := tbtcChain.AddPastNewWalletRegisteredEvent(
					&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
					&tbtc.NewWalletRegisteredEvent{
						WalletPublicKeyHash: scenario.TargetWalletPublicKeyHash,
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				tbtcChain.SetWallet(
					scenario.TargetWalletPublicKeyHash,
					&tbtc.WalletChainData{
						State: tbtc.StateLive,
					},
				)
			}

			tbtcChain.SetLiveWalletsCount(scenario.LiveWalletsCount)

			task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

			if scenario.ExpectedProposal != nil {
				err := tbtcChain.SetReservationReanchorProposalValidationResult(
					scenario.SourceWalletPublicKeyHash,
					scenario.ExpectedProposal,
					true,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			proposal, _, err := task.Run(
				&tbtc.CoordinationProposalRequest{
					WalletPublicKeyHash: scenario.SourceWalletPublicKeyHash,
				},
			)

			expectedErrStr := ""
			if scenario.ExpectedErr != nil {
				expectedErrStr = scenario.ExpectedErr.Error()
			}
			actualErrStr := ""
			if err != nil {
				actualErrStr = err.Error()
			}
			if expectedErrStr != actualErrStr {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v",
					scenario.ExpectedErr,
					err,
				)
			}

			actualProposal, _ := proposal.(*tbtc.ReservationReanchorProposal)

			if !reanchorProposalsEqual(scenario.ExpectedProposal, actualProposal) {
				t.Errorf(
					"invalid reservation re-anchor proposal\n"+
						"expected: %+v\n"+
						"actual:   %+v",
					scenario.ExpectedProposal,
					actualProposal,
				)
			}
		})
	}
}

// reanchorProposalsEqual compares two proposals field-by-field. deep.Equal
// cannot be used here (or anywhere else the package compares a
// *tbtc.ReservationReanchorProposal/*tbtc.ReservationAnchorProposal): by
// default it does not descend into unexported fields, and *big.Int's
// representation is entirely unexported, so deep.Equal silently reports "no
// difference" for any two distinct *big.Int values. ReservationKey and
// ReanchorTxFee are both *big.Int, so they need an explicit .Cmp().
func reanchorProposalsEqual(
	expected, actual *tbtc.ReservationReanchorProposal,
) bool {
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}
	if (expected.ReservationKey == nil) != (actual.ReservationKey == nil) {
		return false
	}
	if expected.ReservationKey != nil &&
		expected.ReservationKey.Cmp(actual.ReservationKey) != 0 {
		return false
	}
	if expected.RequestNonce != actual.RequestNonce {
		return false
	}
	if expected.TargetWalletPublicKeyHash != actual.TargetWalletPublicKeyHash {
		return false
	}
	if (expected.ReanchorTxFee == nil) != (actual.ReanchorTxFee == nil) {
		return false
	}
	if expected.ReanchorTxFee != nil &&
		expected.ReanchorTxFee.Cmp(actual.ReanchorTxFee) != 0 {
		return false
	}
	return true
}

func TestReservationReanchorTask_TargetWalletExclusion_SharedTask(t *testing.T) {
	walletA := hexToByte20("1111111111111111111111111111111111111111")
	walletB := hexToByte20("2222222222222222222222222222222222222222")
	walletC := hexToByte20("3333333333333333333333333333333333333333")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	// Register walletC at block 100, then walletB at block 200 (walletB is newest).
	err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: walletC},
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: walletB},
	)
	if err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(walletA, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(walletB, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetWallet(walletC, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(2)

	tbtcChain.SetMovingFundsParameters(
		1000000,
		1000000,
		0,
		0,
		nil,
		0,
		0,
		0,
		0,
		nil,
		0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	// Setup reservation for Wallet A.
	resAKey := big.NewInt(1001)
	anchorTxHashA, _ := bitcoin.NewHashFromString(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		bitcoin.ReversedByteOrder,
	)
	walletAScript, err := bitcoin.PayToWitnessPublicKeyHash(walletA)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHashA, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 100000, PublicKeyScript: walletAScript},
		},
	})
	tbtcChain.SetReservation(resAKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletA,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHashA,
				OutputIndex:     1,
			},
			Value: 100000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(walletA, []*big.Int{resAKey})

	// Setup reservation for Wallet B.
	resBKey := big.NewInt(2001)
	anchorTxHashB, _ := bitcoin.NewHashFromString(
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		bitcoin.ReversedByteOrder,
	)
	walletBScript, err := bitcoin.PayToWitnessPublicKeyHash(walletB)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHashB, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: walletBScript},
		},
	})
	tbtcChain.SetReservation(resBKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletB,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHashB,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(walletB, []*big.Int{resBKey})

	// Single task instance used for both runs.
	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	// First run for wallet A: should select wallet B as target.
	propA, okA, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletA,
	})
	if err != nil {
		t.Fatalf("unexpected error for wallet A: %v", err)
	}
	if !okA || propA == nil {
		t.Fatalf("expected proposal for wallet A, got ok=%v, prop=%v", okA, propA)
	}
	proposalA, ok := propA.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("unexpected proposal type: %T", propA)
	}
	if proposalA.TargetWalletPublicKeyHash != walletB {
		t.Errorf(
			"wallet A expected target walletB [%x], got [%x]",
			walletB,
			proposalA.TargetWalletPublicKeyHash,
		)
	}

	// Second run with the SAME task instance for wallet B (now in StateMovingFunds):
	// Must NOT select wallet B (itself), even though wallet B was cached in the previous run.
	// Must select wallet C instead.
	tbtcChain.SetWallet(walletB, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	propB, okB, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletB,
	})
	if err != nil {
		t.Fatalf("unexpected error for wallet B: %v", err)
	}
	if !okB || propB == nil {
		t.Fatalf("expected proposal for wallet B, got ok=%v, prop=%v", okB, propB)
	}
	proposalB, ok := propB.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("unexpected proposal type: %T", propB)
	}
	if proposalB.TargetWalletPublicKeyHash == walletB {
		t.Errorf("wallet B selected itself as target wallet: [%x]", walletB)
	}
	if proposalB.TargetWalletPublicKeyHash != walletC {
		t.Errorf(
			"wallet B expected target walletC [%x], got [%x]",
			walletC,
			proposalB.TargetWalletPublicKeyHash,
		)
	}

	// Third run: if wallet C is not live, wallet B must not produce any proposal
	// (must not fall back to selecting itself).
	tbtcChain.SetWallet(walletC, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	propB2, okB2, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletB,
	})
	if err != nil {
		t.Fatalf("unexpected error on third run: %v", err)
	}
	if okB2 || propB2 != nil {
		t.Errorf("expected no proposal when no other live wallet exists, got prop=%v", propB2)
	}
}

func TestReservationReanchorTask_Run_SkipNonActiveReservations(t *testing.T) {
	walletA := hexToByte20("1111111111111111111111111111111111111111")
	walletB := hexToByte20("2222222222222222222222222222222222222222")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: walletB},
	)
	if err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(walletA, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(walletB, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	tbtcChain.SetMovingFundsParameters(
		1000000,
		1000000,
		0,
		0,
		nil,
		0,
		0,
		0,
		0,
		nil,
		0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	// Setup 2 reservations for Wallet A:
	// res1 is in ReservationStateActionPending (should be skipped)
	// res2 is in ReservationStateActive (should be proposed)
	res1Key := big.NewInt(101)
	anchorTxHash1, _ := bitcoin.NewHashFromString(
		"1111111111111111111111111111111111111111111111111111111111111111",
		bitcoin.ReversedByteOrder,
	)
	walletAScript, err := bitcoin.PayToWitnessPublicKeyHash(walletA)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash1, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 100000, PublicKeyScript: walletAScript},
		},
	})
	tbtcChain.SetReservation(res1Key, &tbtc.Reservation{
		WalletPublicKeyHash: walletA,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash1,
				OutputIndex:     1,
			},
			Value: 100000,
		},
		State:        tbtc.ReservationStateActionPending,
		RequestNonce: 1,
	})
	// Seed res1's prior-generation action at nonce 1 in a terminal
	// Settled state, mirroring production: a request that has since
	// settled leaves the action record visible but no longer Pending,
	// and Run's resume path correctly rejects it as "not a resumable
	// re-anchor" rather than as "action not found".
	tbtcChain.SetReservationAction(res1Key, 1, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStateSettled,
	})

	res2Key := big.NewInt(102)
	anchorTxHash2, _ := bitcoin.NewHashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
		bitcoin.ReversedByteOrder,
	)
	btcChain.SetTransaction(anchorTxHash2, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: walletAScript},
		},
	})
	tbtcChain.SetReservation(res2Key, &tbtc.Reservation{
		WalletPublicKeyHash: walletA,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash2,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})

	tbtcChain.SetWalletReservations(walletA, []*big.Int{res1Key, res2Key})

	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	prop, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletA,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || prop == nil {
		t.Fatalf("expected proposal, got ok=%v, prop=%v", ok, prop)
	}
	proposal, ok := prop.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("unexpected proposal type: %T", prop)
	}
	if proposal.ReservationKey.Cmp(res2Key) != 0 {
		t.Errorf("expected proposal for res2 [102], got [%v]", proposal.ReservationKey)
	}
	if proposal.TargetWalletPublicKeyHash != walletB {
		t.Errorf("expected target walletB [%x], got [%x]", walletB, proposal.TargetWalletPublicKeyHash)
	}
}

// reservationReanchorLocalChain is a test-only wrapper around the shared
// LocalChain mock that adds support for injecting past reservation
// acceptance-requested events, filtered by wallet. It exists as a
// separate type so this test file does not need to edit the shared
// chain_test.go fixture (mirrors reservationAcceptanceLocalChain in
// reservation_acceptance_test.go).
type reservationReanchorLocalChain struct {
	*tbtcpg.LocalChain

	acceptanceEvents []*tbtc.ReservationAcceptanceRequestedEvent

	// pastNewWalletRegisteredEventsCalls counts calls to
	// PastNewWalletRegisteredEvents, letting tests assert on how many
	// times findTargetWallet's registration-event scan actually ran
	// (e.g. that a cached target wallet suppressed a repeat scan).
	pastNewWalletRegisteredEventsCalls int

	// forceCapRevertFor, when non-zero, makes RequestReservationReanchor
	// return a "wallet reservations cap exceeded" error for the given
	// target wallet on the first invocation. It models a concurrent
	// consumer that ate the target's capacity between the caller's
	// headroom pre-check and the request submission -- the race Run's
	// cap-revert branch exists to handle. Subsequent invocations fall
	// through to the embedded LocalChain's real implementation so the
	// retry's second candidate can succeed normally.
	forceCapRevertFor [20]byte
	capRevertConsumed bool
}

func newReservationReanchorLocalChain() *reservationReanchorLocalChain {
	return &reservationReanchorLocalChain{LocalChain: tbtcpg.NewLocalChain()}
}

// RequestReservationReanchor simulates a cap-related revert race for the
// wallet set via forceCapRevertFor on its first invocation; the
// production task catches this with isReservationCapRevertError, evicts
// the cached target, and retries with the next candidate, after which the
// wrapper forwards to the embedded LocalChain's real implementation.
func (rrlc *reservationReanchorLocalChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
) ([32]byte, error) {
	if !rrlc.capRevertConsumed &&
		rrlc.forceCapRevertFor == targetWalletPublicKeyHash {
		rrlc.capRevertConsumed = true
		return [32]byte{}, errors.New("wallet reservations cap exceeded")
	}
	return rrlc.LocalChain.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
	)
}

// markReservationTouched records walletPublicKeyHash as having accepted a
// reservation at some point, satisfying
// ReservationReanchorTask.walletHasReservationHistory's gate.
func (rrlc *reservationReanchorLocalChain) markReservationTouched(
	walletPublicKeyHash [20]byte,
) {
	rrlc.acceptanceEvents = append(
		rrlc.acceptanceEvents,
		&tbtc.ReservationAcceptanceRequestedEvent{
			WalletPublicKeyHash: walletPublicKeyHash,
		},
	)
}

func (rrlc *reservationReanchorLocalChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	var results []*tbtc.ReservationAcceptanceRequestedEvent
	for _, event := range rrlc.acceptanceEvents {
		if filter != nil && len(filter.WalletPublicKeyHash) > 0 {
			matched := false
			for _, w := range filter.WalletPublicKeyHash {
				if w == event.WalletPublicKeyHash {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, event)
	}
	return results, nil
}

func (rrlc *reservationReanchorLocalChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	rrlc.pastNewWalletRegisteredEventsCalls++
	return rrlc.LocalChain.PastNewWalletRegisteredEvents(filter)
}

// TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust is a
// regression test for the NotifyMovingFundsBelowDust wiring: once a
// reservation-touched MovingFunds wallet has no reservations left and its
// main UTXO is below the moving funds dust threshold, Run must call
// NotifyMovingFundsBelowDust exactly once with the wallet's resolved main
// UTXO. A wallet above the dust threshold must not trigger any
// notification, nor must a wallet the reservation subsystem never
// touched, nor one that still has reservations remaining.
func TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust(t *testing.T) {
	walletPublicKeyHash := hexToByte20("ffb3f7538bfa98a511495dd96027cfbd57baf2fa")

	newFixture := func(
		mainUtxoValue int64,
		reservationKeys []*big.Int,
		reservationTouched bool,
	) (*reservationReanchorLocalChain, *tbtcpg.LocalBitcoinChain) {
		tbtcChain := newReservationReanchorLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}
		mainUtxoTx := &bitcoin.Transaction{
			Version: 1,
			Outputs: []*bitcoin.TransactionOutput{{
				Value:           mainUtxoValue,
				PublicKeyScript: walletScript,
			}},
		}
		mainUtxoTxHash := mainUtxoTx.Hash()
		btcChain.SetTransaction(mainUtxoTxHash, mainUtxoTx)
		btcChain.SetTxHashesForPublicKeyHash(
			walletPublicKeyHash,
			[]bitcoin.Hash{mainUtxoTxHash},
		)

		mainUtxo := &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: mainUtxoTxHash,
				OutputIndex:     0,
			},
			Value: mainUtxoValue,
		}

		tbtcChain.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{
			State:        tbtc.StateMovingFunds,
			MainUtxoHash: tbtcChain.ComputeMainUtxoHash(mainUtxo),
		})
		tbtcChain.SetMovingFundsParameters(
			1000000, 1000000, 0, 0, nil, 0, 0, 0, 0, nil, 0,
		)
		tbtcChain.SetWalletReservations(walletPublicKeyHash, reservationKeys)
		if reservationTouched {
			tbtcChain.markReservationTouched(walletPublicKeyHash)
		}

		return tbtcChain, btcChain
	}

	t.Run("below dust threshold: notifies exactly once", func(t *testing.T) {
		tbtcChain, btcChain := newFixture(500000, nil, true)
		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

		prop, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || prop != nil {
			t.Fatalf("expected no proposal, got ok=%v, prop=%v", ok, prop)
		}

		notifications := tbtcChain.GetBelowDustNotifications()
		if len(notifications) != 1 {
			t.Fatalf("expected exactly 1 below-dust notification, got %d", len(notifications))
		}
		if notifications[0].WalletPublicKeyHash != walletPublicKeyHash {
			t.Errorf(
				"unexpected notified wallet\nexpected: %x\nactual:   %x",
				walletPublicKeyHash,
				notifications[0].WalletPublicKeyHash,
			)
		}
		if notifications[0].MainUtxo == nil || notifications[0].MainUtxo.Value != 500000 {
			t.Errorf("unexpected notified main UTXO: %+v", notifications[0].MainUtxo)
		}
	})

	t.Run("above dust threshold: no notification", func(t *testing.T) {
		tbtcChain, btcChain := newFixture(2000000, nil, true)
		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

		if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
			t.Fatalf("expected no below-dust notifications, got %d", len(notifications))
		}
	})

	t.Run("wallet never reservation-touched: no notification even below dust", func(t *testing.T) {
		tbtcChain, btcChain := newFixture(500000, nil, false)
		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

		if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
			t.Fatalf(
				"expected no below-dust notifications for a never-touched wallet, got %d",
				len(notifications),
			)
		}
	})

	t.Run("reservations remaining and main UTXO below dust: no notification", func(t *testing.T) {
		reservationKey := big.NewInt(9001)
		tbtcChain, btcChain := newFixture(500000, []*big.Int{reservationKey}, true)
		// No live target wallet is available, so the re-anchor attempt
		// itself resolves to a benign no-op right after the reservations
		// check; the point under test is that a non-empty
		// WalletReservations result must suppress the below-dust
		// notification path entirely, regardless of how the re-anchor
		// attempt for the remaining reservation turns out.
		tbtcChain.SetLiveWalletsCount(0)

		task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

		if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if notifications := tbtcChain.GetBelowDustNotifications(); len(notifications) != 0 {
			t.Fatalf(
				"expected no below-dust notifications while reservations remain, got %d",
				len(notifications),
			)
		}
	})
}

// TestReservationReanchorTask_FindTargetWallet_CachesAcrossRuns was moved
// to reservation_reanchor_metrics_test.go, which is in package tbtcpg and
// can therefore reach the unexported findTargetWallet method directly.
// See that file for the test body.

// TestReservationReanchorTask_CapRevertLeadsToNextCandidate is a
// regression test for the cap-revert branch Run adds on top of
// findTargetWallet: a request-time capacity revert (modeled here as a
// concurrent consumer eating the cached target's headroom between the
// headroom pre-check and the request submission) must not abort the
// coordination window. Run must evict the reverted target, mark it as
// tried for the rest of the pass, and retry the same reservation with
// the next candidate -- so that the proposal surfaces for the second
// candidate, with exactly one RequestReservationReanchor submission
// (the successful one).
func TestReservationReanchorTask_CapRevertLeadsToNextCandidate(t *testing.T) {
	sourceWalletPublicKeyHash := hexToByte20("1111111111111111111111111111111111111111")
	cappedTargetWalletPublicKeyHash := hexToByte20("2222222222222222222222222222222222222222")
	alternateTargetWalletPublicKeyHash := hexToByte20("3333333333333333333333333333333333333333")

	tbtcChain := newReservationReanchorLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	// Register the alternate target (newest), then the capped target.
	if err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: cappedTargetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}
	if err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: alternateTargetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(cappedTargetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetWallet(alternateTargetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(2)

	tbtcChain.SetMovingFundsParameters(
		1000000,
		1000000,
		0,
		0,
		nil,
		0,
		0,
		0,
		0,
		nil,
		0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	resKey := big.NewInt(7001)
	anchorTxHash, _ := bitcoin.NewHashFromString(
		"7777777777777777777777777777777777777777777777777777777777777777",
		bitcoin.ReversedByteOrder,
	)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 100000, PublicKeyScript: sourceWalletScript},
		},
	})
	tbtcChain.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 100000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{resKey})

	// Simulate the race: the first attempt to request a re-anchor on
	// the alternate target reverts with "wallet reservations cap
	// exceeded" -- a concurrent operator consumed the target's room
	// after our headroom pre-check passed. The retry on the second
	// candidate must proceed normally.
	tbtcChain.forceCapRevertFor = alternateTargetWalletPublicKeyHash

	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf("expected a proposal for the second candidate, got ok=%v, prop=%v", ok, proposal)
	}
	reanchorProposal, ok := proposal.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("unexpected proposal type: %T", proposal)
	}
	if reanchorProposal.TargetWalletPublicKeyHash != cappedTargetWalletPublicKeyHash {
		t.Fatalf(
			"expected proposal target = capped target [%x] after the "+
				"alternate target reverted, got [%x]",
			cappedTargetWalletPublicKeyHash,
			reanchorProposal.TargetWalletPublicKeyHash,
		)
	}

	submissions := tbtcChain.GetReservationReanchorRequestSubmissions()
	if len(submissions) != 1 {
		t.Fatalf(
			"expected exactly 1 RequestReservationReanchor submission "+
				"(the successful retry on the capped target), got %d",
			len(submissions),
		)
	}
	if submissions[0].TargetWalletPublicKeyHash != cappedTargetWalletPublicKeyHash {
		t.Fatalf(
			"expected the single submission to target the capped "+
				"candidate [%x], got [%x]",
			cappedTargetWalletPublicKeyHash,
			submissions[0].TargetWalletPublicKeyHash,
		)
	}
}

// TestReservationReanchorTask_ResumesPendingReanchorWithoutNewRequest
// pins the resume path's contract: a reservation already in
// ReservationStateActionPending with a Pending Reanchor generation must
// be picked up at its real nonce and authorized target without a second
// RequestReservationReanchor call. An earlier design would have
// re-requested, which on-chain would always revert because
// requestReservationReanchor requires the Active state.
func TestReservationReanchorTask_ResumesPendingReanchorWithoutNewRequest(t *testing.T) {
	sourceWalletPublicKeyHash := hexToByte20("1111111111111111111111111111111111111111")
	targetWalletPublicKeyHash := hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	if err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: targetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	tbtcChain.SetMovingFundsParameters(
		1000000,
		1000000,
		0,
		0,
		nil,
		0,
		0,
		0,
		0,
		nil,
		0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
		ReservationMinAmount:     1000,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	resKey := big.NewInt(8001)
	anchorTxHash, _ := bitcoin.NewHashFromString(
		"8888888888888888888888888888888888888888888888888888888888888888",
		bitcoin.ReversedByteOrder,
	)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: sourceWalletScript},
		},
	})
	// State ActionPending with RequestNonce = 4 mirrors production:
	// an earlier Run issued the request, the action record was written,
	// and the reservation advanced to ActionPending. The current Run
	// must resume that generation without re-requesting.
	tbtcChain.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActionPending,
		RequestNonce: 4,
	})
	tbtcChain.SetReservationAction(resKey, 4, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TxMaxFee:                  100000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		// Far-future TimeoutAt so the safety-margin gate does not
		// skip the resumed generation.
		TimeoutAt: uint32(time.Now().Add(24 * time.Hour).Unix()),
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{resKey})

	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)

	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf("expected a resumed proposal, got ok=%v, prop=%v", ok, proposal)
	}
	reanchorProposal, ok := proposal.(*tbtc.ReservationReanchorProposal)
	if !ok {
		t.Fatalf("unexpected proposal type: %T", proposal)
	}
	if reanchorProposal.RequestNonce != 4 {
		t.Fatalf(
			"expected the resumed proposal to carry the generation's real "+
				"nonce (4), got [%d]",
			reanchorProposal.RequestNonce,
		)
	}
	if reanchorProposal.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
		t.Fatalf(
			"expected the resumed proposal to target the authorized "+
				"wallet [%x], got [%x]",
			targetWalletPublicKeyHash,
			reanchorProposal.TargetWalletPublicKeyHash,
		)
	}

	if submissions := tbtcChain.GetReservationReanchorRequestSubmissions(); len(submissions) != 0 {
		t.Fatalf(
			"expected zero RequestReservationReanchor submissions on the "+
				"resume path; the resume path must never re-request, "+
				"got %d",
			len(submissions),
		)
	}
}

// TestProposeReservationReanchor_ValidatesOnlyAfterRequestMined pins the
// strict ordering the production task enforces: the action record at the
// proposal's RequestNonce must already exist before validation runs, so
// a strict fake (mirroring WalletProposalValidator.sol's precondition)
// rejects a validate-first attempt. ProposeReservationReanchor succeeds
// only because it issues the request and waits for it to be mined
// before re-reading the action record.
func TestProposeReservationReanchor_ValidatesOnlyAfterRequestMined(t *testing.T) {
	sourceWalletPublicKeyHash := hexToByte20("1111111111111111111111111111111111111111")
	targetWalletPublicKeyHash := hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	if err := tbtcChain.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: targetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	tbtcChain.SetMovingFundsParameters(
		1000000,
		1000000,
		0,
		0,
		nil,
		0,
		0,
		0,
		0,
		nil,
		0,
	)
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	resKey := big.NewInt(9001)
	anchorTxHash, _ := bitcoin.NewHashFromString(
		"9999999999999999999999999999999999999999999999999999999999999999",
		bitcoin.ReversedByteOrder,
	)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 200000, PublicKeyScript: sourceWalletScript},
		},
	})
	// Active reservation with no action record: a validate-first
	// attempt at the (predicted) request nonce must be rejected by the
	// strict fake because the action at that nonce does not exist yet.
	tbtcChain.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 200000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{resKey})

	// Validate-first attempt: the strict fake rejects it because the
	// action record at the (predicted) request nonce does not exist.
	preProposal := &tbtc.ReservationReanchorProposal{
		ReservationKey:            new(big.Int).Set(resKey),
		RequestNonce:              1,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:             big.NewInt(550),
	}
	if err := tbtcChain.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		preProposal,
	); err == nil {
		t.Fatalf(
			"strict fake accepted a proposal before its action record " +
				"existed -- the on-chain validator's action-record " +
				"precondition must reject this",
		)
	}

	// Production's correct order: issue the request first, wait for
	// the mined action record, then build and validate.
	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)
	proposal, err := task.ProposeReservationReanchor(
		reanchorTestLogger,
		sourceWalletPublicKeyHash,
		resKey,
		targetWalletPublicKeyHash,
		0,
	)
	if err != nil {
		t.Fatalf("ProposeReservationReanchor: unexpected error: %v", err)
	}
	if proposal == nil {
		t.Fatalf("expected a non-nil proposal from the request-then-validate flow")
	}
	if proposal.RequestNonce != 1 {
		t.Fatalf("expected proposal RequestNonce = 1, got [%d]", proposal.RequestNonce)
	}

	if submissions := tbtcChain.GetReservationReanchorRequestSubmissions(); len(submissions) != 1 {
		t.Fatalf(
			"expected exactly 1 RequestReservationReanchor submission, "+
				"got %d -- the request-then-validate flow must issue "+
				"exactly one request per Active reservation",
			len(submissions),
		)
	}

	// After the request is mined, a strict validator pass against the
	// same proposal must now succeed.
	if err := tbtcChain.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		proposal,
	); err != nil {
		t.Fatalf(
			"strict fake rejected the proposal after the request was "+
				"mined: %v -- the action-record precondition must be "+
				"satisfied once the request writes the Pending Reanchor "+
				"action",
			err,
		)
	}
}
