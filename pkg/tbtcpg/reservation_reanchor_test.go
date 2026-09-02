package tbtcpg_test

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	pkgtest "github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

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

				btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
					Version: 1,
					Outputs: []*bitcoin.TransactionOutput{{
						Value:           r.AnchorValue,
						PublicKeyScript: []byte{},
					}},
				})

				tbtcChain.SetReservation(r.ReservationKey, &tbtc.Reservation{
					WalletPublicKeyHash: r.WalletPublicKeyHash,
					AnchorUtxo: &bitcoin.UnspentTransactionOutput{
						Outpoint: &bitcoin.TransactionOutpoint{
							TransactionHash: anchorTxHash,
							OutputIndex:     r.AnchorTxOutputIndex,
						},
						Value: r.AnchorValue,
					},
					State:        r.State,
					RequestNonce: r.RequestNonce,
				})

				// Always install an action record when RequestNonce > 0 so
				// hasPendingAction's real GetReservationAction lookup
				// succeeds and evaluates State directly, matching what a
				// real chain would have: RequestNonce only ever advances
				// alongside a real action record. HasPendingAction=false
				// scenarios use r.PendingActionState (a terminal state, not
				// Pending) to model an already-settled prior generation.
				if r.RequestNonce > 0 {
					tbtcChain.SetReservationAction(
						r.ReservationKey,
						r.RequestNonce,
						&tbtc.ReservationAction{
							ActionType: tbtc.ReservationActionTypeReanchor,
							State:      r.PendingActionState,
						},
					)
				}
			}
			tbtcChain.SetWalletReservations(
				scenario.SourceWalletPublicKeyHash,
				reservationKeys,
			)

			tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
				ReservationTxMaxFee: scenario.ReservationTxMaxFee,
			})

			btcChain.SetEstimateSatPerVByteFee(1, scenario.EstimateSatPerVByteFee)

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
