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
		ReservationTxMaxFee: 100000,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	// Setup reservation for Wallet A.
	resAKey := big.NewInt(1001)
	anchorTxHashA, _ := bitcoin.NewHashFromString(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		bitcoin.ReversedByteOrder,
	)
	btcChain.SetTransaction(anchorTxHashA, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: []byte{},
		}},
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
	btcChain.SetTransaction(anchorTxHashB, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           200000,
			PublicKeyScript: []byte{},
		}},
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
		ReservationTxMaxFee: 100000,
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
	btcChain.SetTransaction(anchorTxHash1, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           100000,
			PublicKeyScript: []byte{},
		}},
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

	res2Key := big.NewInt(102)
	anchorTxHash2, _ := bitcoin.NewHashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
		bitcoin.ReversedByteOrder,
	)
	btcChain.SetTransaction(anchorTxHash2, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           200000,
			PublicKeyScript: []byte{},
		}},
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
