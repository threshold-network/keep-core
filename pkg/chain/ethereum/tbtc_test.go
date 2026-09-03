package ethereum

import (
	"crypto/ecdsa"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestConvertPubKeyToChainFormat(t *testing.T) {
	bytes30 := []byte{229, 19, 136, 216, 125, 157, 135, 142, 67, 130,
		136, 13, 76, 188, 32, 218, 243, 134, 95, 73, 155, 24, 38, 73, 117, 90,
		215, 95, 216, 19}
	bytes31 := []byte{182, 142, 176, 51, 131, 130, 111, 197, 191, 103, 180, 137,
		171, 101, 34, 78, 251, 234, 118, 184, 16, 116, 238, 82, 131, 153, 134,
		17, 46, 158, 94}

	expectedResult := [64]byte{
		// padding
		00, 00,
		// bytes30
		229, 19, 136, 216, 125, 157, 135, 142, 67, 130, 136, 13, 76, 188, 32,
		218, 243, 134, 95, 73, 155, 24, 38, 73, 117, 90, 215, 95, 216, 19,
		// padding
		00,
		// bytes31
		182, 142, 176, 51, 131, 130, 111, 197, 191, 103, 180, 137, 171, 101, 34,
		78, 251, 234, 118, 184, 16, 116, 238, 82, 131, 153, 134, 17, 46, 158, 94,
	}

	actualResult, err := convertPubKeyToChainFormat(
		&ecdsa.PublicKey{
			X: new(big.Int).SetBytes(bytes30),
			Y: new(big.Int).SetBytes(bytes31),
		},
	)

	if err != nil {
		t.Errorf("unexpected error [%v]", err)
	}

	testutils.AssertBytesEqual(
		t,
		expectedResult[:],
		actualResult[:],
	)
}

func TestConvertReservationFromAbiType(t *testing.T) {
	ownerAddress := common.HexToAddress(
		"0x1234567890AbcdEF1234567890aBcdef12345678",
	)
	anchorTxHash := [32]byte{0x01, 0x02, 0x03, 0x04}

	validAbiReservation := tbtcabi.ReservationReservationRequest{
		Owner:                 ownerAddress,
		MintedAmount:          100000,
		AcceptedAt:            1700000000,
		WalletPubKeyHash:      [20]byte{0xaa, 0xbb, 0xcc},
		AnchorAmount:          99000,
		ExpiresAt:             1700100000,
		AnchorTxHash:          anchorTxHash,
		AnchorTxOutputIndex:   1,
		State:                 1, // Active
		RequestNonce:          7,
		RetryCredit:           true,
		DissolutionEligibleAt: 1700200000,
		// CumulativeReanchorFee is intentionally dropped on the Go
		// boundary (see the Field omissions note on
		// convertReservationFromAbiType).
		CumulativeReanchorFee: 12345,
	}

	t.Run("valid states", func(t *testing.T) {
		var tests = map[string]struct {
			abiState      uint8
			expectedState tbtc.ReservationState
		}{
			"unknown":  {0, tbtc.ReservationStateUnknown},
			"active":   {1, tbtc.ReservationStateActive},
			"pending":  {2, tbtc.ReservationStateActionPending},
			"closed":   {3, tbtc.ReservationStateClosed},
			"stranded": {4, tbtc.ReservationStateStranded},
		}

		for testName, test := range tests {
			t.Run(testName, func(t *testing.T) {
				abiReservation := validAbiReservation
				abiReservation.State = test.abiState

				reservation, err := convertReservationFromAbiType(abiReservation)
				if err != nil {
					t.Fatalf("unexpected error: [%v]", err)
				}

				if reservation.State != test.expectedState {
					t.Errorf("expected state [%v], got [%v]", test.expectedState, reservation.State)
				}
			})
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		invalidAbiReservation := validAbiReservation
		invalidAbiReservation.State = 255

		reservation, err := convertReservationFromAbiType(invalidAbiReservation)
		if reservation != nil {
			t.Errorf("expected nil reservation, got [%+v]", reservation)
		}
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// t.Run below documents the intentional CumulativeReanchorFee drop
	// performed by convertReservationFromAbiType: the field is written
	// on-chain by every re-anchor hop but is not exposed on
	// tbtc.Reservation (see the Field omissions note on
	// convertReservationFromAbiType). It also pins that every other
	// field maps correctly - each field below is a distinct value so a
	// future accidental restoration of CumulativeReanchorFee, or a
	// swapped adjacent field, does not go unnoticed.
	t.Run("drops cumulative reanchor fee and maps every other field", func(t *testing.T) {
		abiReservation := tbtcabi.ReservationReservationRequest{
			Owner:                 common.HexToAddress("0x111111111111111111111111111111111111111B"),
			MintedAmount:          111,
			AcceptedAt:            222,
			WalletPubKeyHash:      [20]byte{0x01, 0x02, 0x03},
			AnchorAmount:          333,
			ExpiresAt:             444,
			AnchorTxHash:          [32]byte{0x04, 0x05, 0x06},
			AnchorTxOutputIndex:   555,
			State:                 1, // ReservationStateActive
			RequestNonce:          666,
			RetryCredit:           true,
			DissolutionEligibleAt: 777,
			CumulativeReanchorFee: 888, // must not appear anywhere in the output
		}

		expected := &tbtc.Reservation{
			Owner:        chain.Address("0x111111111111111111111111111111111111111B"),
			MintedAmount: 111,
			AcceptedAt:   222,
			WalletPublicKeyHash: [20]byte{
				0x01, 0x02, 0x03,
			},
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x04, 0x05, 0x06},
					OutputIndex:     555,
				},
				Value: 333,
			},
			ExpiresAt:             444,
			State:                 tbtc.ReservationStateActive,
			RequestNonce:          666,
			RetryCredit:           true,
			DissolutionEligibleAt: 777,
		}

		actual, err := convertReservationFromAbiType(abiReservation)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"unexpected reservation\nexpected: [%+v]\nactual:   [%+v]",
				expected,
				actual,
			)
		}
	})
}

func TestConvertReservationActionFromAbiType(t *testing.T) {
	targetWalletPKH := [20]byte{0x11, 0x22, 0x33}
	redeemerAddress := common.HexToAddress(
		"0xAbCdEf1234567890abcDef1234567890AbCdEf12",
	)
	actionDataHash := [32]byte{0xde, 0xad, 0xbe, 0xef}

	baseAbiAction := tbtcabi.ReservationReservationAction{
		TargetWalletPubKeyHash: targetWalletPKH,
		RequestedAt:            1700000000,
		TimeoutAt:              1700003600,
		TxMaxFee:               5000,
		State:                  1, // Pending
		FeePaid:                true,
		Redeemer:               redeemerAddress,
		Amount:                 50000,
		ActionDataHash:         actionDataHash,
		IsPartial:              true,
	}

	// The action-type-to-hash-field routing (redemption -> redeemer output
	// script hash, dissolution -> expected main UTXO hash, everything else
	// -> neither) is the one non-trivial branch in this converter; exercise
	// all three shapes.
	var tests = map[string]struct {
		abiActionType                    uint8
		expectedActionType               tbtc.ReservationActionType
		expectedRedeemerOutputScriptHash [32]byte
		expectedExpectedMainUtxoHash     [32]byte
	}{
		"redemption routes hash to redeemer output script": {
			abiActionType:                    2,
			expectedActionType:               tbtc.ReservationActionTypeRedemption,
			expectedRedeemerOutputScriptHash: actionDataHash,
			expectedExpectedMainUtxoHash:     [32]byte{},
		},
		"dissolution routes hash to expected main utxo": {
			abiActionType:                    4,
			expectedActionType:               tbtc.ReservationActionTypeDissolution,
			expectedRedeemerOutputScriptHash: [32]byte{},
			expectedExpectedMainUtxoHash:     actionDataHash,
		},
		"acceptance leaves both hash fields zero": {
			abiActionType:                    1,
			expectedActionType:               tbtc.ReservationActionTypeAcceptance,
			expectedRedeemerOutputScriptHash: [32]byte{},
			expectedExpectedMainUtxoHash:     [32]byte{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			abiAction := baseAbiAction
			abiAction.ActionType = test.abiActionType

			action, err := convertReservationActionFromAbiType(abiAction)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			expected := &tbtc.ReservationAction{
				TargetWalletPublicKeyHash: targetWalletPKH,
				RequestedAt:               1700000000,
				TimeoutAt:                 1700003600,
				TxMaxFee:                  5000,
				ActionType:                test.expectedActionType,
				State:                     tbtc.ReservationActionStatePending,
				FeePaid:                   true,
				Redeemer:                  chain.Address(redeemerAddress.String()),
				Amount:                    50000,
				RedeemerOutputScriptHash:  test.expectedRedeemerOutputScriptHash,
				ExpectedMainUtxoHash:      test.expectedExpectedMainUtxoHash,
				IsPartial:                 true,
			}

			if !reflect.DeepEqual(expected, action) {
				t.Errorf(
					"unexpected action\nexpected: [%+v]\nactual:   [%+v]\n",
					expected,
					action,
				)
			}
		})
	}

	t.Run("invalid action type", func(t *testing.T) {
		abiAction := baseAbiAction
		abiAction.ActionType = 255

		action, err := convertReservationActionFromAbiType(abiAction)
		if action != nil {
			t.Errorf("expected nil action, got [%+v]", action)
		}
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid action state", func(t *testing.T) {
		abiAction := baseAbiAction
		abiAction.ActionType = 1
		abiAction.State = 255

		action, err := convertReservationActionFromAbiType(abiAction)
		if action != nil {
			t.Errorf("expected nil action, got [%+v]", action)
		}
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("valid action states", func(t *testing.T) {
		var tests = map[string]struct {
			abiState      uint8
			expectedState tbtc.ReservationActionState
		}{
			"unknown":    {0, tbtc.ReservationActionStateUnknown},
			"pending":    {1, tbtc.ReservationActionStatePending},
			"settled":    {2, tbtc.ReservationActionStateSettled},
			"timed out":  {3, tbtc.ReservationActionStateTimedOut},
			"vetoed":     {4, tbtc.ReservationActionStateVetoed},
			"superseded": {5, tbtc.ReservationActionStateSuperseded},
		}

		for testName, test := range tests {
			t.Run(testName, func(t *testing.T) {
				abiAction := baseAbiAction
				abiAction.ActionType = 1
				abiAction.State = test.abiState

				action, err := convertReservationActionFromAbiType(abiAction)
				if err != nil {
					t.Fatalf("unexpected error: [%v]", err)
				}

				if action.State != test.expectedState {
					t.Errorf("expected state [%v], got [%v]", test.expectedState, action.State)
				}
			})
		}
	})
}

// TestConvertReservationParametersFromAbiType verifies the full 10-tuple
// field mapping performed by convertReservationParametersFromAbiType.
// Field count/order had not previously been cross-checked against the
// live Solidity struct; every field below is set to a distinct non-zero
// value so a swapped or dropped field is caught, not masked by a shared
// zero-value default.
func TestConvertReservationParametersFromAbiType(t *testing.T) {
	vaultAddress := common.HexToAddress(
		"0x111111111111111111111111111111111111111A",
	)

	abiParameters := struct {
		ReservationVault                common.Address
		ReservationMinAmount            uint64
		ReservationTxMaxFee             uint64
		ReservationTermSeconds          uint32
		ReservationDissolutionDelay     uint32
		ReservationMaxTotalAmount       uint64
		ReservationTotalAmount          uint64
		MaxReservationsPerWallet        uint32
		ReservationActionTimeout        uint32
		ReservationRenewalWindowSeconds uint32
	}{
		ReservationVault:                vaultAddress,
		ReservationMinAmount:            1000,
		ReservationTxMaxFee:             5000,
		ReservationTermSeconds:          1209600,
		ReservationDissolutionDelay:     3600,
		ReservationMaxTotalAmount:       10000000,
		ReservationTotalAmount:          2500000,
		MaxReservationsPerWallet:        5,
		ReservationActionTimeout:        86400,
		ReservationRenewalWindowSeconds: 604800,
	}

	expected := &tbtc.ReservationParameters{
		ReservationVault:                chain.Address("0x111111111111111111111111111111111111111A"),
		ReservationMinAmount:            1000,
		ReservationTxMaxFee:             5000,
		ReservationTermSeconds:          1209600,
		ReservationDissolutionDelay:     3600,
		ReservationMaxTotalAmount:       10000000,
		ReservationTotalAmount:          2500000,
		MaxReservationsPerWallet:        5,
		ReservationActionTimeout:        86400,
		ReservationRenewalWindowSeconds: 604800,
	}

	actual := convertReservationParametersFromAbiType(abiParameters)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf(
			"unexpected reservation parameters\nexpected: [%+v]\nactual:   [%+v]",
			expected,
			actual,
		)
	}
}
