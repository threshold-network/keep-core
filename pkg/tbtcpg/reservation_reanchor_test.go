package tbtcpg_test

import (
	"math/big"
	"testing"

	"github.com/go-test/deep"

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

			tbtcChain.SetWallet(
				scenario.SourceWalletPublicKeyHash,
				&tbtc.WalletChainData{
					State:        scenario.SourceWalletState,
					MainUtxoHash: scenario.SourceWalletMainUtxoHashBytes,
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

				if r.HasPendingAction {
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
					nil,
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

			var actualProposals []*tbtc.ReservationReanchorProposal
			if p, ok := proposal.(*tbtc.ReservationReanchorProposal); ok && p != nil {
				actualProposals = append(actualProposals, p)
			}

			var expectedProposals []*tbtc.ReservationReanchorProposal
			if p := scenario.ExpectedProposal; p != nil {
				expectedProposals = append(expectedProposals, p)
			}

			if diff := deep.Equal(actualProposals, expectedProposals); diff != nil {
				t.Errorf("invalid reservation re-anchor proposal: %v", diff)
			}
		})
	}
}
