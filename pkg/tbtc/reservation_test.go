package tbtc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
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
		ReservationKey:       big.NewInt(12345),
		RequestNonce:         2,
		RedeemerOutputScript: bitcoin.Script{0x00, 0x14, 0x02},
		RedemptionTxFee:      big.NewInt(1600),
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
	marshalJSON := func(t *testing.T, v interface{}) string {
		t.Helper()
		bytes, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(bytes)
	}

	tests := map[string]struct {
		actionType    WalletActionType
		payload       func(t *testing.T) string
		expectedError string
	}{
		"anchor null payload": {
			actionType:    ActionReservationAnchor,
			payload:       func(t *testing.T) string { return "null" },
			expectedError: "cannot unmarshal proposal payload: [deposit funding transaction hash is required]",
		},
		"anchor missing deposit funding tx hash": {
			actionType: ActionReservationAnchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationAnchorProposal{
					RequestNonce: 1,
					AnchorTxFee:  big.NewInt(1500),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [deposit funding transaction hash is required]",
		},
		"anchor missing nonce": {
			actionType: ActionReservationAnchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationAnchorProposal{
					DepositFundingTxHash: bitcoin.Hash{0x01},
					AnchorTxFee:          big.NewInt(1500),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"anchor missing fee": {
			actionType: ActionReservationAnchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationAnchorProposal{
					DepositFundingTxHash: bitcoin.Hash{0x01},
					RequestNonce:         1,
				})
			},
			expectedError: "cannot unmarshal proposal payload: [anchor transaction fee is required]",
		},
		"reserved redemption null payload": {
			actionType:    ActionReservedRedemption,
			payload:       func(t *testing.T) string { return "null" },
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"reserved redemption missing nonce": {
			actionType: ActionReservedRedemption,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservedRedemptionProposal{
					ReservationKey:       big.NewInt(12345),
					RedeemerOutputScript: bitcoin.Script{0x00, 0x14, 0x02},
					RedemptionTxFee:      big.NewInt(1600),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"reserved redemption missing redeemer output script": {
			actionType: ActionReservedRedemption,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservedRedemptionProposal{
					ReservationKey:  big.NewInt(12345),
					RequestNonce:    2,
					RedemptionTxFee: big.NewInt(1600),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [redeemer output script is required]",
		},
		"reserved redemption missing fee": {
			actionType: ActionReservedRedemption,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservedRedemptionProposal{
					ReservationKey:       big.NewInt(12345),
					RequestNonce:         2,
					RedeemerOutputScript: bitcoin.Script{0x00, 0x14, 0x02},
				})
			},
			expectedError: "cannot unmarshal proposal payload: [redemption transaction fee is required]",
		},
		"re-anchor null payload": {
			actionType:    ActionReservationReanchor,
			payload:       func(t *testing.T) string { return "null" },
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"re-anchor missing nonce": {
			actionType: ActionReservationReanchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationReanchorProposal{
					ReservationKey:            big.NewInt(54321),
					TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
					ReanchorTxFee:             big.NewInt(1700),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"re-anchor missing target wallet public key hash": {
			actionType: ActionReservationReanchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationReanchorProposal{
					ReservationKey: big.NewInt(54321),
					RequestNonce:   3,
					ReanchorTxFee:  big.NewInt(1700),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [target wallet public key hash is required]",
		},
		"re-anchor missing fee": {
			actionType: ActionReservationReanchor,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationReanchorProposal{
					ReservationKey:            big.NewInt(54321),
					RequestNonce:              3,
					TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
				})
			},
			expectedError: "cannot unmarshal proposal payload: [re-anchor transaction fee is required]",
		},
		"dissolution null payload": {
			actionType:    ActionReservationDissolution,
			payload:       func(t *testing.T) string { return "null" },
			expectedError: "cannot unmarshal proposal payload: [reservation key is required]",
		},
		"dissolution missing nonce": {
			actionType: ActionReservationDissolution,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationDissolutionProposal{
					ReservationKey:   big.NewInt(99999),
					DissolutionTxFee: big.NewInt(1800),
				})
			},
			expectedError: "cannot unmarshal proposal payload: [request nonce is required]",
		},
		"dissolution missing fee": {
			actionType: ActionReservationDissolution,
			payload: func(t *testing.T) string {
				return marshalJSON(t, &ReservationDissolutionProposal{
					ReservationKey: big.NewInt(99999),
					RequestNonce:   4,
				})
			},
			expectedError: "cannot unmarshal proposal payload: [dissolution transaction fee is required]",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := unmarshalCoordinationProposal(
				uint32(test.actionType),
				[]byte(test.payload(t)),
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
	bridgeChain := Connect()
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

	redeemerOutputScriptHash, err := bridgeChain.ComputeReservationRedeemerOutputScriptHash(
		redeemerScript,
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		action          *ReservationAction
		expectedOutputs []*bitcoin.TransactionOutput
		fee             int64
		expectedError   string
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
			fee: 1500,
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
			fee: 1500,
		},
		"fee exceeds redemption amount": {
			action: &ReservationAction{
				TxMaxFee:                 150000,
				ActionType:               ReservationActionTypeRedemption,
				State:                    ReservationActionStatePending,
				Amount:                   100000,
				RedeemerOutputScriptHash: redeemerOutputScriptHash,
			},
			fee:           100000,
			expectedError: "transaction fee exceeds the redemption amount",
		},
		"partial amount exceeds anchor value": {
			action: &ReservationAction{
				TxMaxFee:                 2000,
				ActionType:               ReservationActionTypeRedemption,
				State:                    ReservationActionStatePending,
				Amount:                   150000,
				RedeemerOutputScriptHash: redeemerOutputScriptHash,
				IsPartial:                true,
			},
			fee:           1500,
			expectedError: "redemption amount exceeds the anchor value",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			builder, err := assembleReservedRedemptionTransaction(
				bitcoinChain,
				bridgeChain,
				anchorUtxo,
				walletPublicKeyHash,
				redeemerScript,
				test.action,
				0,
				test.fee,
			)

			if test.expectedError != "" {
				if err == nil || err.Error() != test.expectedError {
					t.Fatalf("expected error: [%s], got: [%v]", test.expectedError, err)
				}
				return
			}

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
		expectedError       string
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
			action:        &baseAction,
			expectedError: "wallet main UTXO must not be provided when the dissolution action has no expected main UTXO snapshot",
		},
		"mismatched action amount": {
			action: func() *ReservationAction {
				action := baseAction
				action.Amount = 99999
				return &action
			}(),
			expectedError: "dissolution action amount does not match the anchor value",
		},
		"mismatched target wallet": {
			action: func() *ReservationAction {
				action := baseAction
				action.TargetWalletPublicKeyHash = [20]byte{0x02}
				return &action
			}(),
			expectedError: "dissolution action targets a different wallet",
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

			if test.expectedError != "" {
				if err == nil || err.Error() != test.expectedError {
					t.Fatalf("expected error: [%s], got: [%v]", test.expectedError, err)
				}
				return
			}

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

	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x03},
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
	redeemerOutputScriptHash, err := bridgeChain.ComputeReservationRedeemerOutputScriptHash(
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

	anchorAction := &ReservationAction{
		ActionType: ReservationActionTypeAcceptance,
		State:      ReservationActionStatePending,
		TxMaxFee:   2000,
	}
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		anchorAction,
		0,
		1500,
	)
	assertError(err, "deposit is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		nil,
		walletPublicKeyHash,
		redeemerScript,
		redemptionAction,
		0,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		bitcoin.Script{},
		redemptionAction,
		0,
		1500,
	)
	assertError(err, "redeemer output script is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		nil,
		0,
		1500,
	)
	assertError(err, "reservation action is required")

	nonRedemptionAction := *redemptionAction
	nonRedemptionAction.ActionType = ReservationActionTypeReanchor
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&nonRedemptionAction,
		0,
		1500,
	)
	assertError(err, "reservation action is not a redemption")

	nonPendingAction := *redemptionAction
	nonPendingAction.State = ReservationActionStateTimedOut
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&nonPendingAction,
		0,
		1500,
	)
	assertError(err, "reservation action is not pending")

	wrongScriptAction := *redemptionAction
	wrongScriptAction.RedeemerOutputScriptHash = [32]byte{0x01}
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&wrongScriptAction,
		0,
		1500,
	)
	assertError(err, "redeemer output script is not authorized")

	partialWholeAmountAction := *redemptionAction
	partialWholeAmountAction.IsPartial = true
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&partialWholeAmountAction,
		0,
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
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		&partialAmountAction,
		0,
		1500,
	)
	assertError(err, "whole redemption amount must equal the anchor value")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		walletPublicKeyHash,
		redeemerScript,
		redemptionAction,
		0,
		2500,
	)
	assertError(err, "transaction fee exceeds the action fee limit")

	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		anchorAction,
		0,
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

	// (a) ActionType != Dissolution
	invalidDissolutionAction := *dissolutionAction
	invalidDissolutionAction.ActionType = ReservationActionTypeReanchor
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		&invalidDissolutionAction,
		1500,
	)
	assertError(err, "reservation action is not a dissolution")

	// (b) State != Pending
	nonPendingDissolutionAction := *dissolutionAction
	nonPendingDissolutionAction.State = ReservationActionStateTimedOut
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		&nonPendingDissolutionAction,
		1500,
	)
	assertError(err, "reservation action is not pending")

	// (c) fee > TxMaxFee
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		dissolutionAction,
		2500,
	)
	assertError(err, "transaction fee exceeds the action fee limit")

	// (d) totalInputsValue - fee <= 0
	highFeeLimitDissolutionAction := *dissolutionAction
	highFeeLimitDissolutionAction.TxMaxFee = 150000
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		&highFeeLimitDissolutionAction,
		100000,
	)
	assertError(err, "transaction fee exceeds the total inputs value")

	snapshottedMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x04},
			OutputIndex:     1,
		},
		Value: 200000,
	}

	// (e) non-nil walletMainUtxo + no ExpectedMainUtxoHash
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		snapshottedMainUtxo,
		walletPublicKeyHash,
		dissolutionAction,
		1500,
	)
	assertError(
		err,
		"wallet main UTXO must not be provided when the dissolution action has no expected main UTXO snapshot",
	)

	// (f) bridgeChain == nil
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		nil,
		anchorUtxo,
		nil,
		walletPublicKeyHash,
		dissolutionAction,
		1500,
	)
	assertError(err, "bridge chain is required")

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
func TestAssembleReservationAnchorTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{0x01}
	anchorAction := &ReservationAction{
		ActionType: ReservationActionTypeAcceptance,
		State:      ReservationActionStatePending,
		TxMaxFee:   2000,
	}

	deposit := &Deposit{
		Depositor:           chain.Address("0x1111111111111111111111111111111111111111"),
		WalletPublicKeyHash: walletPublicKeyHash,
	}
	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatal(err)
	}
	depositScriptHash := sha256.Sum256(depositScript)
	depositLockingScript, err := bitcoin.PayToWitnessScriptHash(depositScriptHash)
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
				PublicKeyScript: depositLockingScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	// (a) happy path
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		anchorAction,
		0,
		1500,
	)
	if err != nil {
		t.Fatalf("expected no error, got: [%v]", err)
	}

	// (a1) action type not acceptance
	invalidTypeAction := *anchorAction
	invalidTypeAction.ActionType = ReservationActionTypeRedemption
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		&invalidTypeAction,
		0,
		1500,
	)
	if err == nil || err.Error() != "reservation action is not an acceptance" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (a2) action state not pending
	invalidStateAction := *anchorAction
	invalidStateAction.State = ReservationActionStateTimedOut
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		&invalidStateAction,
		0,
		1500,
	)
	if err == nil || err.Error() != "reservation action is not pending" {
		t.Fatalf("expected error, got: [%v]", err)
	}
	// (b) fee > TxMaxFee
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		anchorAction,
		0,
		2500,
	)
	if err == nil || err.Error() != "transaction fee exceeds the action fee limit" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (c) anchorValue < reservationMinAmount
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		anchorAction,
		99500,
		1000,
	)
	if err == nil || err.Error() != "anchor value is below the reservation minimum amount" {
		t.Fatalf("expected error, got: [%v]", err)
	}
}
func TestAssembleReservationReanchorTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()

	privateKeyValue := big.NewInt(100)
	wallet := generateWallet(privateKeyValue)
	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	targetWalletPublicKeyHash := [20]byte{0x02}
	reanchorAction := &ReservationAction{
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
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

	// (a) happy path
	builder, err := assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		reanchorAction,
		0,
		1500,
	)
	if err != nil {
		t.Fatalf("expected no error, got: [%v]", err)
	}
	transaction := signReservationTransaction(
		t,
		builder,
		wallet.publicKey,
		privateKeyValue,
	)
	if transaction.Outputs[0].Value != 98500 {
		t.Errorf("expected output value 98500, got: [%v]", transaction.Outputs[0].Value)
	}

	// (b) action type not reanchor
	invalidAction := *reanchorAction
	invalidAction.ActionType = ReservationActionTypeAcceptance
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		&invalidAction,
		0,
		1500,
	)
	if err == nil || err.Error() != "reservation action is not a re-anchor" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (c) action state not pending
	invalidStateAction := *reanchorAction
	invalidStateAction.State = ReservationActionStateTimedOut
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		&invalidStateAction,
		0,
		1500,
	)
	if err == nil || err.Error() != "reservation action is not pending" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (d) mismatched wallet
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		[20]byte{0x03},
		reanchorAction,
		0,
		1500,
	)
	if err == nil || err.Error() != "reanchor action targets a different wallet" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (e) fee > TxMaxFee
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		reanchorAction,
		0,
		2500,
	)
	if err == nil || err.Error() != "transaction fee exceeds the action fee limit" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (f) reanchorValue < reservationMinAmount
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		reanchorAction,
		99500,
		1000,
	)
	if err == nil || err.Error() != "re-anchor value is below the reservation minimum amount" {
		t.Fatalf("expected error, got: [%v]", err)
	}
}
