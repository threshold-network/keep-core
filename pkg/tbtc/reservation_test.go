package tbtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
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

func fundUtxo(
	t *testing.T,
	bitcoinChain bitcoin.Chain,
	script bitcoin.Script,
	values ...int64,
) []*bitcoin.UnspentTransactionOutput {
	t.Helper()

	outputs := make([]*bitcoin.TransactionOutput, len(values))
	for i, value := range values {
		outputs[i] = &bitcoin.TransactionOutput{
			Value:           value,
			PublicKeyScript: script,
		}
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
		Outputs: outputs,
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatal(err)
	}

	utxos := make([]*bitcoin.UnspentTransactionOutput, len(values))
	for i, value := range values {
		utxos[i] = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTransaction.Hash(),
				OutputIndex:     uint32(i),
			},
			Value: value,
		}
	}
	return utxos
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

	anchorUtxo := fundUtxo(t, bitcoinChain, walletScript, 100000)[0]

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
		"fee must be positive": {
			action: &ReservationAction{
				TxMaxFee:                 2000,
				ActionType:               ReservationActionTypeRedemption,
				State:                    ReservationActionStatePending,
				Amount:                   100000,
				RedeemerOutputScriptHash: redeemerOutputScriptHash,
			},
			fee:           0,
			expectedError: "transaction fee must be positive",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			builder, err := assembleReservedRedemptionTransaction(
				bitcoinChain,
				bridgeChain,
				anchorUtxo,
				redeemerScript,
				test.action,
				test.fee,
				0,
				anchorUtxo.Outpoint,
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

	utxos := fundUtxo(t, bitcoinChain, walletScript, 100000, 200000)
	anchorUtxo := utxos[0]
	walletMainUtxo := utxos[1]

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
				wallet.publicKey,
				test.action,
				1500,
				0,
				anchorUtxo.Outpoint,
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
	walletPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	walletPublicKey := &walletPrivateKey.PublicKey
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
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
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		ActionType:                ReservationActionTypeAcceptance,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
	}
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKey,
		anchorAction,
		0,
		1500,
		0,
	)
	assertError(err, "deposit is required")

	mismatchedWalletAnchorAction := *anchorAction
	mismatchedWalletAnchorAction.TargetWalletPublicKeyHash = [20]byte{0x02}
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKey,
		&mismatchedWalletAnchorAction,
		0,
		1500,
		0,
	)
	assertError(err, "acceptance action targets a different wallet")

	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKey,
		anchorAction,
		0,
		0,
		0,
	)
	assertError(err, "transaction fee must be positive")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		nil,
		redeemerScript,
		redemptionAction,
		1500,
		0,
		&bitcoin.TransactionOutpoint{},
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		bitcoin.Script{},
		redemptionAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "redeemer output script is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		nil,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "reservation action is required")

	nonRedemptionAction := *redemptionAction
	nonRedemptionAction.ActionType = ReservationActionTypeReanchor
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		&nonRedemptionAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "reservation action is not a redemption")

	nonPendingAction := *redemptionAction
	nonPendingAction.State = ReservationActionStateTimedOut
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		&nonPendingAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "reservation action has already been settled")

	wrongScriptAction := *redemptionAction
	wrongScriptAction.RedeemerOutputScriptHash = [32]byte{0x01}
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		&wrongScriptAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "redeemer output script is not authorized")

	zeroValueAnchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: anchorUtxo.Outpoint,
		Value:    0,
	}
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		zeroValueAnchorUtxo,
		redeemerScript,
		redemptionAction,
		1500,
		0,
		zeroValueAnchorUtxo.Outpoint,
	)
	assertError(err, "anchor UTXO value must be positive")

	zeroAmountAction := *redemptionAction
	zeroAmountAction.Amount = 0
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		&zeroAmountAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "redemption amount must be positive")

	wrongAmountAction := *redemptionAction
	wrongAmountAction.Amount = 40000
	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		&wrongAmountAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "whole redemption amount must equal the anchor value")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		redemptionAction,
		2500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "transaction fee exceeds the action fee limit")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		redeemerScript,
		redemptionAction,
		0,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "transaction fee must be positive")

	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		anchorAction,
		0,
		1500,
		0,
		&bitcoin.TransactionOutpoint{},
	)
	assertError(err, "anchor UTXO is required")

	zeroFeeReanchorAction := &ReservationAction{
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
	}
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		walletPublicKeyHash,
		zeroFeeReanchorAction,
		0,
		0,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "transaction fee must be positive")

	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		nil,
		nil,
		walletPublicKey,
		dissolutionAction,
		1500,
		0,
		&bitcoin.TransactionOutpoint{},
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKey,
		nil,
		1500,
		0,
		anchorUtxo.Outpoint,
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
		walletPublicKey,
		&invalidDissolutionAction,
		1500,
		0,
		anchorUtxo.Outpoint,
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
		walletPublicKey,
		&nonPendingDissolutionAction,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "reservation action has already been settled")

	// (c) fee > TxMaxFee
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKey,
		dissolutionAction,
		2500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "transaction fee exceeds the action fee limit")

	// (c1) fee must be positive
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKey,
		dissolutionAction,
		0,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(err, "transaction fee must be positive")

	// (d) totalInputsValue - fee <= 0
	highFeeLimitDissolutionAction := *dissolutionAction
	highFeeLimitDissolutionAction.TxMaxFee = 150000
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		anchorUtxo,
		nil,
		walletPublicKey,
		&highFeeLimitDissolutionAction,
		150000,
		0,
		anchorUtxo.Outpoint,
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
		walletPublicKey,
		dissolutionAction,
		1500,
		0,
		anchorUtxo.Outpoint,
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
		walletPublicKey,
		dissolutionAction,
		1500,
		0,
		&bitcoin.TransactionOutpoint{},
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
		walletPublicKey,
		&actionWithMainUtxo,
		1500,
		0,
		anchorUtxo.Outpoint,
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
		walletPublicKey,
		&actionWithMainUtxo,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	assertError(
		err,
		"wallet main UTXO does not match the dissolution action snapshot",
	)
}
func TestAssembleReservationAnchorTransaction(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	walletPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	walletPublicKey := &walletPrivateKey.PublicKey
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	anchorAction := &ReservationAction{
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		ActionType:                ReservationActionTypeAcceptance,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
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
	builder, err := assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKey,
		anchorAction,
		0,
		1500,
		0,
	)
	if err != nil {
		t.Fatalf("expected no error, got: [%v]", err)
	}

	transaction := signReservationTransaction(
		t,
		builder,
		walletPublicKey,
		walletPrivateKey.D,
	)

	if len(transaction.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %v", len(transaction.Inputs))
	}
	if !reflect.DeepEqual(transaction.Inputs[0].Outpoint, deposit.Utxo.Outpoint) {
		t.Errorf(
			"unexpected input outpoint\nexpected: [%+v]\nactual:   [%+v]",
			deposit.Utxo.Outpoint,
			transaction.Inputs[0].Outpoint,
		)
	}

	expectedAnchorValue := int64(100000 - 1500)
	expectedScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	expectedOutputs := []*bitcoin.TransactionOutput{
		{
			Value:           expectedAnchorValue,
			PublicKeyScript: expectedScript,
		},
	}
	if !reflect.DeepEqual(expectedOutputs, transaction.Outputs) {
		t.Errorf(
			"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
			expectedOutputs,
			transaction.Outputs,
		)
	}
	// (a1) action type not acceptance
	invalidTypeAction := *anchorAction
	invalidTypeAction.ActionType = ReservationActionTypeRedemption
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKey,
		&invalidTypeAction,
		0,
		1500,
		0,
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
		walletPublicKey,
		&invalidStateAction,
		0,
		1500,
		0,
	)
	if err == nil || err.Error() != "reservation action has already been settled" {
		t.Fatalf("expected error, got: [%v]", err)
	}
	// (b) fee > TxMaxFee
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKey,
		anchorAction,
		0,
		2500,
		0,
	)
	if err == nil || err.Error() != "transaction fee exceeds the action fee limit" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (c) anchorValue < reservationMinAmount
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKey,
		anchorAction,
		99500,
		1000,
		0,
	)
	if err == nil || err.Error() != "anchor value is below the reservation minimum amount" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (d) nil action
	_, err = assembleReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKey,
		nil,
		0,
		1500,
		0,
	)
	if err == nil || err.Error() != "reservation action is required" {
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
	anchorUtxo := fundUtxo(t, bitcoinChain, walletScript, 100000)[0]

	reanchorAction := &ReservationAction{
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		Amount:                    uint64(anchorUtxo.Value),
	}

	// (a) happy path
	builder, err := assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		reanchorAction,
		0,
		1500,
		0,
		anchorUtxo.Outpoint,
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
		0,
		anchorUtxo.Outpoint,
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
		0,
		anchorUtxo.Outpoint,
	)
	if err == nil || err.Error() != "reservation action has already been settled" {
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
		0,
		anchorUtxo.Outpoint,
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
		0,
		anchorUtxo.Outpoint,
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
		0,
		anchorUtxo.Outpoint,
	)
	if err == nil || err.Error() != "re-anchor value is below the reservation minimum amount" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (g) nil action
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		nil,
		0,
		1500,
		0,
		anchorUtxo.Outpoint,
	)
	if err == nil || err.Error() != "reservation action is required" {
		t.Fatalf("expected error, got: [%v]", err)
	}

	// (h) expected anchor outpoint mismatch
	wrongOutpoint := &bitcoin.TransactionOutpoint{
		TransactionHash: anchorUtxo.Outpoint.TransactionHash,
		OutputIndex:     anchorUtxo.Outpoint.OutputIndex + 1,
	}
	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		reanchorAction,
		0,
		1500,
		0,
		wrongOutpoint,
	)
	if err == nil || err.Error() != "anchor UTXO outpoint does not match the action snapshot" {
		t.Fatalf("expected error, got: [%v]", err)
	}
}

func TestAssembleReservationReanchorTransaction_AmountMismatch(t *testing.T) {
	// Setup
	bitcoinChain := newLocalBitcoinChain()
	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x01},
			OutputIndex:     0,
		},
		Value: 100000,
	}
	action := &ReservationAction{
		TargetWalletPublicKeyHash: [20]byte{0x01},
		ActionType:                ReservationActionTypeReanchor,
		State:                     ReservationActionStatePending,
		TxMaxFee:                  2000,
		Amount:                    50000, // Mismatch
	}

	// Execute
	_, err := assembleReservationReanchorTransaction(
		bitcoinChain,
		anchorUtxo,
		[20]byte{0x01},
		action,
		0,
		1000,
		0,
		anchorUtxo.Outpoint,
	)

	// Assert
	if err == nil || err.Error() != "reanchor action amount does not match the anchor value" {
		t.Errorf("expected error [reanchor action amount does not match the anchor value], got [%v]", err)
	}
}

func TestAssembleReservationTransactions_BoundaryErrors(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	bridgeChain := Connect()
	walletPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	walletPublicKey := &walletPrivateKey.PublicKey

	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	anchorUtxo := fundUtxo(t, bitcoinChain, walletScript, 100000)[0]

	// 1. Anchor's deposit-value-exceeded
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
	deposit.Utxo = fundUtxo(t, bitcoinChain, depositLockingScript, 1000)[0]
	_, err = assembleReservationAnchorTransaction(bitcoinChain, deposit, walletPublicKey, &ReservationAction{TargetWalletPublicKeyHash: walletPublicKeyHash, ActionType: ReservationActionTypeAcceptance, State: ReservationActionStatePending, TxMaxFee: 2000}, 0, 2000, 0)
	if err == nil || err.Error() != "transaction fee exceeds the deposit value" {
		t.Errorf("expected error [transaction fee exceeds the deposit value], got [%v]", err)
	}

	// 2. Redemption's bridge-chain-nil
	_, err = assembleReservedRedemptionTransaction(bitcoinChain, nil, anchorUtxo, bitcoin.Script{0x00}, &ReservationAction{}, 100, 0, anchorUtxo.Outpoint)
	if err == nil || err.Error() != "bridge chain is required" {
		t.Errorf("expected error [bridge chain is required], got [%v]", err)
	}

	// 3. Redemption's amount-exceeds-anchor
	_, err = assembleReservedRedemptionTransaction(bitcoinChain, bridgeChain, anchorUtxo, bitcoin.Script{0x00}, &ReservationAction{ActionType: ReservationActionTypeRedemption, State: ReservationActionStatePending, Amount: 200000}, 100, 0, anchorUtxo.Outpoint)
	if err == nil || err.Error() != "redemption amount exceeds the anchor value" {
		t.Errorf("expected error [redemption amount exceeds the anchor value], got [%v]", err)
	}

	// 4. Reanchor's fee-exceeds-anchor-value
	_, err = assembleReservationReanchorTransaction(bitcoinChain, anchorUtxo, [20]byte{0x01}, &ReservationAction{TargetWalletPublicKeyHash: [20]byte{0x01}, ActionType: ReservationActionTypeReanchor, State: ReservationActionStatePending, TxMaxFee: 200000, Amount: 100000}, 0, 200000, 0, anchorUtxo.Outpoint)
	if err == nil || err.Error() != "transaction fee exceeds the anchor value" {
		t.Errorf("expected error [transaction fee exceeds the anchor value], got [%v]", err)
	}

	// 5. Dissolution's zero-value-anchor
	zeroValueAnchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x09},
			OutputIndex:     0,
		},
		Value: 0,
	}
	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		bridgeChain,
		zeroValueAnchorUtxo,
		nil,
		walletPublicKey,
		&ReservationAction{
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			ActionType:                ReservationActionTypeDissolution,
			State:                     ReservationActionStatePending,
			TxMaxFee:                  200,
		},
		100,
		0,
		zeroValueAnchorUtxo.Outpoint,
	)
	if err == nil || err.Error() != "anchor UTXO value must be positive" {
		t.Errorf("expected error [anchor UTXO value must be positive], got [%v]", err)
	}
}
