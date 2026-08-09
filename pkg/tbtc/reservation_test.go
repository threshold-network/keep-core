package tbtc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestReservationActionTypes(t *testing.T) {
	for value, expected := range map[uint8]WalletActionType{
		6: ActionReservationAnchor,
		7: ActionReservedRedemption,
		8: ActionReservationReanchor,
		9: ActionReservationDissolution,
	} {
		parsed, err := ParseWalletActionType(value)
		if err != nil {
			t.Fatal(err)
		}
		if parsed != expected {
			t.Errorf(
				"unexpected action type for [%v]: expected [%v] got [%v]",
				value,
				expected,
				parsed,
			)
		}
	}
}

func TestReservationStateValues(t *testing.T) {
	tests := map[ReservationState]uint8{
		ReservationStateUnknown:       0,
		ReservationStateActive:        1,
		ReservationStateActionPending: 2,
		ReservationStateClosed:        3,
		ReservationStateStranded:      4,
	}

	for state, expected := range tests {
		if actual := uint8(state); actual != expected {
			t.Errorf(
				"unexpected reservation state value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationActionTypeValues(t *testing.T) {
	tests := map[ReservationActionType]uint8{
		ReservationActionTypeNone:        0,
		ReservationActionTypeAcceptance:  1,
		ReservationActionTypeRedemption:  2,
		ReservationActionTypeReanchor:    3,
		ReservationActionTypeDissolution: 4,
	}

	for actionType, expected := range tests {
		if actual := uint8(actionType); actual != expected {
			t.Errorf(
				"unexpected reservation action type value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationActionStateValues(t *testing.T) {
	tests := map[ReservationActionState]uint8{
		ReservationActionStateUnknown:    0,
		ReservationActionStatePending:    1,
		ReservationActionStateSettled:    2,
		ReservationActionStateTimedOut:   3,
		ReservationActionStateVetoed:     4,
		ReservationActionStateSuperseded: 5,
	}

	for state, expected := range tests {
		if actual := uint8(state); actual != expected {
			t.Errorf(
				"unexpected reservation action state value\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	}
}

func TestReservationProposals_MarshalingRoundtrip(t *testing.T) {
	anchorProposal := &ReservationAnchorProposal{
		DepositFundingTxHash:      bitcoin.Hash{0x01, 0x02},
		DepositFundingOutputIndex: 3,
		RequestNonce:              1,
		AnchorTxFee:               big.NewInt(1500),
	}

	redemptionProposal := &ReservedRedemptionProposal{
		ReservationKey:  big.NewInt(12345),
		RequestNonce:    2,
		RedemptionTxFee: big.NewInt(1600),
	}

	reanchorProposal := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		RequestNonce:              3,
		TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
		ReanchorTxFee:             big.NewInt(1700),
	}

	dissolutionProposal := &ReservationDissolutionProposal{
		ReservationKey:   big.NewInt(99999),
		RequestNonce:     4,
		DissolutionTxFee: big.NewInt(1800),
	}

	roundtrip := func(
		proposal CoordinationProposal,
		fresh CoordinationProposal,
	) {
		marshaled, err := proposal.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		if err := fresh.Unmarshal(marshaled); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(proposal, fresh) {
			t.Errorf(
				"unexpected unmarshaled proposal: expected [%+v] got [%+v]",
				proposal,
				fresh,
			)
		}
	}

	roundtrip(anchorProposal, &ReservationAnchorProposal{})
	roundtrip(redemptionProposal, &ReservedRedemptionProposal{})
	roundtrip(reanchorProposal, &ReservationReanchorProposal{})
	roundtrip(dissolutionProposal, &ReservationDissolutionProposal{})
}

func TestReservationProposals_UnmarshalRejectsMissingIntegers(t *testing.T) {
	tests := map[string]struct {
		actionType    WalletActionType
		payload       string
		expectedError string
	}{
		"anchor empty object": {
			actionType:    ActionReservationAnchor,
			payload:       `{}`,
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor null payload": {
			actionType:    ActionReservationAnchor,
			payload:       `null`,
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"anchor missing nonce": {
			actionType:    ActionReservationAnchor,
			payload:       `{"AnchorTxFee":1500}`,
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"reserved redemption null payload": {
			actionType:    ActionReservedRedemption,
			payload:       `null`,
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"reserved redemption missing nonce": {
			actionType:    ActionReservedRedemption,
			payload:       `{"ReservationKey":12345,"RedemptionTxFee":1600}`,
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"reserved redemption missing fee": {
			actionType:    ActionReservedRedemption,
			payload:       `{"ReservationKey":12345,"RequestNonce":2}`,
			expectedError: "cannot unmarshal proposal payload: [redemption transaction fee is required]",
		},
		"re-anchor null payload": {
			actionType:    ActionReservationReanchor,
			payload:       `null`,
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"re-anchor missing nonce": {
			actionType:    ActionReservationReanchor,
			payload:       `{"ReservationKey":54321,"ReanchorTxFee":1700}`,
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"re-anchor missing fee": {
			actionType:    ActionReservationReanchor,
			payload:       `{"ReservationKey":54321,"RequestNonce":3}`,
			expectedError: "cannot unmarshal proposal payload: [re-anchor transaction fee is required]",
		},
		"dissolution null payload": {
			actionType:    ActionReservationDissolution,
			payload:       `null`,
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"dissolution missing nonce": {
			actionType:    ActionReservationDissolution,
			payload:       `{"ReservationKey":99999,"DissolutionTxFee":1800}`,
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"dissolution missing fee": {
			actionType:    ActionReservationDissolution,
			payload:       `{"ReservationKey":99999,"RequestNonce":4}`,
			expectedError: "cannot unmarshal proposal payload: [dissolution transaction fee is required]",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := unmarshalCoordinationProposal(
				uint32(test.actionType),
				[]byte(test.payload),
			)
			if err == nil || err.Error() != test.expectedError {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					test.expectedError,
					err,
				)
			}
		})
	}
}

func TestAssembleReservedRedemptionTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()

	privateKeyValue := big.NewInt(100)
	wallet := generateWallet(privateKeyValue)
	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	redeemerScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: walletScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	redeemerOutputScriptHash, err := computeReservationRedeemerOutputScriptHash(
		redeemerScript,
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		action          *ReservationAction
		expectedOutputs []*bitcoin.TransactionOutput
	}{
		"whole redemption": {
			action: &ReservationAction{
				TxMaxFee:                 2000,
				ActionType:               ReservationActionTypeRedemption,
				State:                    ReservationActionStatePending,
				Amount:                   100000,
				RedeemerOutputScriptHash: redeemerOutputScriptHash,
			},
			expectedOutputs: []*bitcoin.TransactionOutput{
				{
					Value:           98500,
					PublicKeyScript: redeemerScript,
				},
			},
		},
		"partial redemption": {
			action: &ReservationAction{
				TxMaxFee:                 2000,
				ActionType:               ReservationActionTypeRedemption,
				State:                    ReservationActionStatePending,
				Amount:                   40000,
				RedeemerOutputScriptHash: redeemerOutputScriptHash,
				IsPartial:                true,
			},
			expectedOutputs: []*bitcoin.TransactionOutput{
				{
					Value:           38500,
					PublicKeyScript: redeemerScript,
				},
				{
					Value:           60000,
					PublicKeyScript: walletScript,
				},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			builder, err := assembleReservedRedemptionTransaction(
				bitcoinChain,
				anchorUtxo,
				walletPublicKeyHash,
				redeemerScript,
				test.action,
				1500,
			)
			if err != nil {
				t.Fatal(err)
			}

			transaction := signReservationTransaction(
				t,
				builder,
				wallet.publicKey,
				privateKeyValue,
			)

			if !reflect.DeepEqual(test.expectedOutputs, transaction.Outputs) {
				t.Errorf(
					"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
					test.expectedOutputs,
					transaction.Outputs,
				)
			}
		})
	}
}

func TestAssembleReservationDissolutionTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	bridgeChain := Connect()

	privateKeyValue := big.NewInt(100)
	wallet := generateWallet(privateKeyValue)
	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: walletScript,
			},
			{
				Value:           200000,
				PublicKeyScript: walletScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}
	walletMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     1,
		},
		Value: 200000,
	}

	baseAction := ReservationAction{
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  2000,
		ActionType:                ReservationActionTypeDissolution,
		State:                     ReservationActionStatePending,
		Amount:                    100000,
	}

	tests := map[string]struct {
		action              *ReservationAction
		expectedInputUtxos  []*bitcoin.UnspentTransactionOutput
		expectedOutputValue int64
	}{
		"snapshotted main UTXO": {
			action: func() *ReservationAction {
				action := baseAction
				action.ExpectedMainUtxoHash = bridgeChain.ComputeMainUtxoHash(
					walletMainUtxo,
				)
				return &action
			}(),
			expectedInputUtxos: []*bitcoin.UnspentTransactionOutput{
				anchorUtxo,
				walletMainUtxo,
			},
			expectedOutputValue: 298500,
		},
		"no-main-UTXO snapshot with newly current main UTXO": {
			action: &baseAction,
			expectedInputUtxos: []*bitcoin.UnspentTransactionOutput{
				anchorUtxo,
			},
			expectedOutputValue: 98500,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			builder, err := assembleReservationDissolutionTransaction(
				bitcoinChain,
				bridgeChain,
				anchorUtxo,
				walletMainUtxo,
				walletPublicKeyHash,
				test.action,
				1500,
			)
			if err != nil {
				t.Fatal(err)
			}

			transaction := signReservationTransaction(
				t,
				builder,
				wallet.publicKey,
				privateKeyValue,
			)

			if len(transaction.Inputs) != len(test.expectedInputUtxos) {
				t.Fatalf(
					"unexpected input count\nexpected: [%v]\nactual:   [%v]",
					len(test.expectedInputUtxos),
					len(transaction.Inputs),
				)
			}
			for i, expectedInputUtxo := range test.expectedInputUtxos {
				if !reflect.DeepEqual(
					expectedInputUtxo.Outpoint,
					transaction.Inputs[i].Outpoint,
				) {
					t.Errorf(
						"unexpected input at index [%v]\nexpected: [%+v]\nactual:   [%+v]",
						i,
						expectedInputUtxo.Outpoint,
						transaction.Inputs[i].Outpoint,
					)
				}
			}

			expectedOutputs := []*bitcoin.TransactionOutput{
				{
					Value:           test.expectedOutputValue,
					PublicKeyScript: walletScript,
				},
			}
			if !reflect.DeepEqual(expectedOutputs, transaction.Outputs) {
				t.Errorf(
					"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
					expectedOutputs,
					transaction.Outputs,
				)
			}
		})
	}
}

func signReservationTransaction(
	t *testing.T,
	builder *bitcoin.TransactionBuilder,
	publicKey *ecdsa.PublicKey,
	privateKeyValue *big.Int,
) *bitcoin.Transaction {
	t.Helper()

	sigHashes, err := builder.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}

	privateKey := &ecdsa.PrivateKey{
		PublicKey: *publicKey,
		D:         privateKeyValue,
	}
	signatures := make([]*bitcoin.SignatureContainer, len(sigHashes))
	for i, sigHash := range sigHashes {
		r, s, err := ecdsa.Sign(rand.Reader, privateKey, sigHash.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures[i] = &bitcoin.SignatureContainer{
			R:         r,
			S:         s,
			PublicKey: publicKey,
		}
	}

	transaction, err := builder.AddSignatures(signatures)
	if err != nil {
		t.Fatal(err)
	}

	return transaction
}

func TestAssembleReservationTransactions_InputValidation(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	bridgeChain := Connect()
	walletPublicKeyHash := [20]byte{0x01}
	redeemerScript := bitcoin.Script{0x00, 0x14, 0x02}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x03},
			OutputIndex:     0,
		},
		Value: 100000,
	}
	redeemerOutputScriptHash, err := computeReservationRedeemerOutputScriptHash(
		redeemerScript,
	)
	if err != nil {
		t.Fatal(err)
	}
	redemptionAction := &ReservationAction{
		TxMaxFee:                 2000,
		ActionType:               ReservationActionTypeRedemption,
		State:                    ReservationActionStatePending,
		Amount:                   100000,
		RedeemerOutputScriptHash: redeemerOutputScriptHash,
	}
	dissolutionAction := &ReservationAction{
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  2000,
		ActionType:                ReservationActionTypeDissolution,
		State:                     ReservationActionStatePending,
		Amount:                    100000,
	}

	assertError := func(err error, expected string) {
		if err == nil || err.Error() != expected {
			t.Errorf("expected error [%v], got [%v]", expected, err)
		}
	}

	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		1500,
	)
	assertError(err, "deposit is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		redeemerScript,
		redemptionAction,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		bitcoin.Script{},
		redemptionAction,
		1500,
	)
	assertError(err, "redeemer output script is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		nil,
		1500,
	)
	assertError(err, "reservation action is required")

	nonRedemptionAction := *redemptionAction
	nonRedemptionAction.ActionType = ReservationActionTypeReanchor
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&nonRedemptionAction,
		1500,
	)
	assertError(err, "reservation action is not a redemption")

	nonPendingAction := *redemptionAction
	nonPendingAction.State = ReservationActionStateTimedOut
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&nonPendingAction,
		1500,
	)
	assertError(err, "reservation action is not pending")

	wrongScriptAction := *redemptionAction
	wrongScriptAction.RedeemerOutputScriptHash = [32]byte{0x01}
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&wrongScriptAction,
		1500,
	)
	assertError(err, "redeemer output script is not authorized")

	partialWholeAmountAction := *redemptionAction
	partialWholeAmountAction.IsPartial = true
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&partialWholeAmountAction,
		1500,
	)
	assertError(
		err,
		"partial redemption amount must be less than the anchor value",
	)

	partialAmountAction := *redemptionAction
	partialAmountAction.Amount = 40000
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&partialAmountAction,
		1500,
	)
	assertError(err, "whole redemption amount must equal the anchor value")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		redemptionAction,
		2500,
	)
	assertError(err, "transaction fee exceeds the action fee limit")

	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		nil,
		nil,
		walletPublicKeyHash,
		dissolutionAction,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		nil,
		1500,
	)
	assertError(err, "reservation action is required")

	snapshottedMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x04},
			OutputIndex:     1,
		},
		Value: 200000,
	}
	actionWithMainUtxo := *dissolutionAction
	actionWithMainUtxo.ExpectedMainUtxoHash = bridgeChain.ComputeMainUtxoHash(
		snapshottedMainUtxo,
	)
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		&actionWithMainUtxo,
		1500,
	)
	assertError(err, "wallet main UTXO is required by the dissolution action")

	currentMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x05},
			OutputIndex:     2,
		},
		Value: 300000,
	}
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		currentMainUtxo,
		walletPublicKeyHash,
		&actionWithMainUtxo,
		1500,
	)
	assertError(
		err,
		"wallet main UTXO does not match the dissolution action snapshot",
	)
}
