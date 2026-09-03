package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/keep-network/keep-core/pkg/chain"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/internal/testutils"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestComputeOperatorsIDsHash(t *testing.T) {
	operatorIDs := []chain.OperatorID{
		5, 1, 55, 45435534, 33, 345, 23, 235, 3333, 2,
	}

	hash, err := computeOperatorsIDsHash(operatorIDs)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := "8cd41effd4ee91b56d6b2f836efdcac11ab1ef2ae228e348814d0e6c2966d01e"

	testutils.AssertStringsEqual(
		t,
		"hash",
		expectedHash,
		hex.EncodeToString(hash[:]),
	)
}

func TestConvertSignaturesToChainFormat(t *testing.T) {
	signatureSize := 65

	signature1 := common.LeftPadBytes([]byte{1, 2, 3}, signatureSize)
	signature2 := common.LeftPadBytes([]byte{4, 5, 6}, signatureSize)
	signature3 := common.LeftPadBytes([]byte{7}, signatureSize)
	signature4 := common.LeftPadBytes([]byte{8, 9, 10}, signatureSize)
	signature5 := common.LeftPadBytes([]byte{11, 12, 13}, signatureSize)

	invalidSignature := common.LeftPadBytes([]byte("invalid"), signatureSize-1)

	var tests = map[string]struct {
		signaturesMap   map[group.MemberIndex][]byte
		expectedIndices []group.MemberIndex
		expectedError   error
	}{
		"one valid signature": {
			signaturesMap: map[uint8][]byte{
				1: signature1,
			},
			expectedIndices: []group.MemberIndex{1},
		},
		"five valid signatures": {
			signaturesMap: map[group.MemberIndex][]byte{
				3: signature3,
				1: signature1,
				4: signature4,
				5: signature5,
				2: signature2,
			},
			expectedIndices: []group.MemberIndex{1, 2, 3, 4, 5},
		},
		"invalid signature": {
			signaturesMap: map[group.MemberIndex][]byte{
				1: signature1,
				2: invalidSignature,
			},
			expectedError: fmt.Errorf("invalid signature size for member [2] got [64] bytes but [65] bytes required"),
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			indicesSlice, signaturesSlice, err :=
				convertSignaturesToChainFormat(test.signaturesMap)

			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]\n",
					test.expectedError,
					err,
				)
			}

			if test.expectedError == nil {
				if !reflect.DeepEqual(test.expectedIndices, indicesSlice) {
					t.Errorf(
						"unexpected indices\n"+
							"expected: [%v]\n"+
							"actual:   [%v]\n",
						test.expectedIndices,
						indicesSlice,
					)
				}

				testutils.AssertIntsEqual(
					t,
					"signatures slice length",
					signatureSize*len(test.signaturesMap),
					len(signaturesSlice),
				)
			}

			for i, memberIndex := range indicesSlice {
				actualSignature := signaturesSlice[signatureSize*i : signatureSize*(i+1)]
				if !bytes.Equal(
					test.signaturesMap[memberIndex],
					actualSignature,
				) {
					t.Errorf(
						"invalid signatures for member %v\nexpected: %v\nactual:   %v\n",
						memberIndex,
						test.signaturesMap[memberIndex],
						actualSignature,
					)
				}
			}
		})
	}
}

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

func TestValidateMemberIndex(t *testing.T) {
	one := big.NewInt(1)
	maxMemberIndex := big.NewInt(255)

	var tests = map[string]struct {
		chainMemberIndex *big.Int
		expectedError    error
	}{
		"less than max member index": {
			chainMemberIndex: new(big.Int).Sub(maxMemberIndex, one),
			expectedError:    nil,
		},
		"max member index": {
			chainMemberIndex: maxMemberIndex,
			expectedError:    nil,
		},
		"greater than max member index": {
			chainMemberIndex: new(big.Int).Add(maxMemberIndex, one),
			expectedError:    fmt.Errorf("invalid member index value: [256]"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := validateMemberIndex(test.chainMemberIndex)

			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]\n",
					test.expectedError,
					err,
				)
			}
		})
	}
}

func TestCalculateDKGResultSignatureHash(t *testing.T) {
	chainID := big.NewInt(1)

	groupPublicKey, err := hex.DecodeString(
		"989d253b17a6a0f41838b84ff0d20e8898f9d7b1a98f2564da4cc29dcf8581d9d" +
			"218b65e7d91c752f7b22eaceb771a9af3a6f3d3f010a5d471a1aeef7d7713af",
	)
	if err != nil {
		t.Fatal(err)
	}

	misbehavedMembersIndexes := []group.MemberIndex{2, 55}

	startBlock := big.NewInt(2000)

	hash, err := calculateDKGResultSignatureHash(
		chainID,
		groupPublicKey,
		misbehavedMembersIndexes,
		startBlock,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := "25f917154586c2be0b6364f5c4758580e535bc01ed4881211000c9267aef3a3b"

	testutils.AssertStringsEqual(
		t,
		"hash",
		expectedHash,
		hex.EncodeToString(hash[:]),
	)
}

func TestCalculateInactivityClaimHash(t *testing.T) {
	chainID := big.NewInt(31337)
	nonce := big.NewInt(3)

	walletPublicKey, err := hex.DecodeString(
		"9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c473e" +
			"661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
	)
	if err != nil {
		t.Fatal(err)
	}

	inactiveMembersIndexes := []*big.Int{
		big.NewInt(1), big.NewInt(2), big.NewInt(30),
	}

	heartbeatFailed := true

	hash, err := calculateInactivityClaimHash(
		chainID,
		nonce,
		walletPublicKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := "f3210008cba186e90386a1bd0c63b6f29a67666f632350be22ce63ab39fc506e"

	testutils.AssertStringsEqual(
		t,
		"hash",
		expectedHash,
		hex.EncodeToString(hash[:]),
	)
}

func TestCalculateWalletID(t *testing.T) {
	hexToByte32 := func(hexStr string) [32]byte {
		if len(hexStr) != 64 {
			t.Fatal("hex string length incorrect")
		}

		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			t.Fatal(err)
		}

		var result [32]byte
		copy(result[:], decoded)

		return result
	}

	xBytes := hexToByte32(
		"9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c4",
	)

	yBytes := hexToByte32(
		"73e661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
	)

	walletPublicKey := &ecdsa.PublicKey{
		Curve: local_v1.DefaultCurve,
		X:     new(big.Int).SetBytes(xBytes[:]),
		Y:     new(big.Int).SetBytes(yBytes[:]),
	}

	actualWalletID, err := calculateWalletID(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	expectedWalletID := hexToByte32(
		"a6602e554b8cf7c23538fd040e4ff3520ec680e5e5ce9a075259e613a3e5aa79",
	)

	testutils.AssertBytesEqual(t, expectedWalletID[:], actualWalletID[:])
}

func TestParseDkgResultValidationOutcome(t *testing.T) {
	isValid, err := parseDkgResultValidationOutcome(
		&struct {
			bool
			string
		}{
			true,
			"",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBoolsEqual(t, "validation outcome", true, isValid)

	isValid, err = parseDkgResultValidationOutcome(
		&struct {
			bool
			string
		}{
			false,
			"",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBoolsEqual(t, "validation outcome", false, isValid)

	_, err = parseDkgResultValidationOutcome(
		struct {
			bool
			string
		}{
			true,
			"",
		},
	)
	expectedErr := fmt.Errorf("result validation outcome is not a pointer")
	if !reflect.DeepEqual(expectedErr, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedErr,
			err,
		)
	}

	_, err = parseDkgResultValidationOutcome(
		&struct {
			string
			bool
		}{
			"",
			true,
		},
	)
	expectedErr = fmt.Errorf("cannot parse result validation outcome")
	if !reflect.DeepEqual(expectedErr, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedErr,
			err,
		)
	}
}

func TestComputeMainUtxoHash(t *testing.T) {
	transactionHash, err := bitcoin.NewHashFromString(
		"089bd0671a4481c3584919b4b9b6751cb3f8586dab41cb157adec43fd10ccc00",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: transactionHash,
			OutputIndex:     5,
		},
		Value: 143565433,
	}

	mainUtxoHash := computeMainUtxoHash(mainUtxo)

	expectedMainUtxoHash, err := hex.DecodeString(
		"1216f8e993c4c57d3c4c971c0d2651140fc4ab09d41960d9ccd7b41fdcd270d6",
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBytesEqual(t, expectedMainUtxoHash, mainUtxoHash[:])
}

func TestComputeMovingFundsCommitmentHash(t *testing.T) {
	toByte20 := func(s string) [20]byte {
		bytes, err := hex.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}

		if len(bytes) != 20 {
			t.Fatal("incorrect hexstring length")
		}

		var result [20]byte
		copy(result[:], bytes[:])
		return result
	}

	targetWallets := [][20]byte{
		toByte20("4b440cb29c80c3f256212d8fdd4f2125366f3c91"),
		toByte20("888f01315e0268bfa05d5e522f8d63f6824d9a96"),
		toByte20("b2a89e53a4227dbe530a52a1c419040735fa636c"),
	}

	movingFundsCommitmentHash := computeMovingFundsCommitmentHash(
		targetWallets,
	)

	expectedMovingFundsCommitmentHash, err := hex.DecodeString(
		"8ba62d1d754a3429e2ff1fb4f523b5fad2b605c873a2968bb5985a625eb96202",
	)
	if err != nil {
		t.Fatal(err)
	}
	testutils.AssertBytesEqual(
		t,
		expectedMovingFundsCommitmentHash,
		movingFundsCommitmentHash[:],
	)
}

// Test data based on: https://etherscan.io/tx/0x97c7a293127a604da77f7ef8daf4b19da2bf04327dd891b6d717eaef89bd8bca
func TestBuildDepositKey(t *testing.T) {
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputIndex := uint32(1)

	depositKey := buildDepositKey(fundingTxHash, fundingOutputIndex)

	expectedDepositKey := "3e84c1ea6aeaf2f45fb49623a88affe653b798ea6f675805acc0ec3965b6f317"
	testutils.AssertStringsEqual(
		t,
		"deposit key",
		expectedDepositKey,
		depositKey.Text(16),
	)
}

func TestBuildRedemptionKey(t *testing.T) {
	fromHex := func(hexString string) []byte {
		b, err := hex.DecodeString(hexString)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	walletPublicKeyHashBytes := fromHex("8db50eb52063ea9d98b3eac91489a90f738986f6")
	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], walletPublicKeyHashBytes)

	redeemerOutputScript := fromHex("76a9144130879211c54df460e484ddf9aac009cb38ee7488ac")

	redemptionKey, err := buildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
	if err != nil {
		t.Fatal(err)
	}

	expectedRedemptionKey := "cb493004c645792101cfa4cc5da4c16aa3148065034371a6f1478b7df4b92d39"
	testutils.AssertStringsEqual(
		t,
		"redemption key",
		expectedRedemptionKey,
		redemptionKey.Text(16),
	)
}

func TestBuildMovedFundsKey(t *testing.T) {
	fundingTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	fundingOutputIndex := uint32(2)

	movedFundsKey := buildMovedFundsKey(fundingTxHash, fundingOutputIndex)

	expectedMovedFundsKey := "24509b8a853476ebe77af3707bd7ce017d527680e941b6eeaac2d5b712df4f8d"
	testutils.AssertStringsEqual(
		t,
		"moved funds key",
		expectedMovedFundsKey,
		movedFundsKey.Text(16),
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

func TestBuildReservationAnchorProposalAbi(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
	}
	fundingTxHash := bitcoin.Hash{0x21, 0x22, 0x23}

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: 7,
		AnchorTxFee:               big.NewInt(1500),
	}

	fundingTx := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x31},
				OutputIndex:     3,
			},
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           42000,
			PublicKeyScript: []byte{0x00, 0x14},
		}},
		Locktime: 600000,
	}

	deposit := &tbtc.Deposit{
		BlindingFactor:      [8]byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48},
		WalletPublicKeyHash: [20]byte{0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f, 0x60, 0x61, 0x62, 0x63, 0x64},
		RefundPublicKeyHash: [20]byte{0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x7b, 0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84},
		RefundLocktime:      [4]byte{0x91, 0x92, 0x93, 0x94},
	}

	depositExtraInfo := struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	}{Deposit: deposit, FundingTx: fundingTx}

	abiProposal, abiExtraInfo := buildReservationAnchorProposalAbi(
		walletPublicKeyHash,
		proposal,
		depositExtraInfo,
	)

	expectedProposal := tbtcabi.WalletProposalValidatorReservationAnchorProposal{
		WalletPubKeyHash: walletPublicKeyHash,
		DepositKey: tbtcabi.WalletProposalValidatorDepositKey{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: 7,
		},
		AnchorTxFee: big.NewInt(1500),
	}
	if !reflect.DeepEqual(expectedProposal, abiProposal) {
		t.Errorf(
			"unexpected abi proposal\nexpected: [%+v]\nactual:   [%+v]\n",
			expectedProposal,
			abiProposal,
		)
	}

	expectedExtraInfo := tbtcabi.WalletProposalValidatorDepositExtraInfo{
		FundingTx: tbtcabi.BitcoinTxInfo2{
			Version:      fundingTx.SerializeVersion(),
			InputVector:  fundingTx.SerializeInputs(),
			OutputVector: fundingTx.SerializeOutputs(),
			Locktime:     fundingTx.SerializeLocktime(),
		},
		BlindingFactor:   deposit.BlindingFactor,
		WalletPubKeyHash: deposit.WalletPublicKeyHash,
		RefundPubKeyHash: deposit.RefundPublicKeyHash,
		RefundLocktime:   deposit.RefundLocktime,
	}
	if !reflect.DeepEqual(expectedExtraInfo, abiExtraInfo) {
		t.Errorf(
			"unexpected abi extra info\nexpected: [%+v]\nactual:   [%+v]\n",
			expectedExtraInfo,
			abiExtraInfo,
		)
	}
}

func TestBuildReservationReanchorProposalAbi(t *testing.T) {
	sourceWalletPublicKeyHash := [20]byte{
		0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa,
		0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4,
	}
	targetWalletPublicKeyHash := [20]byte{
		0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca,
		0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4,
	}

	proposal := &tbtc.ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:             big.NewInt(1700),
	}

	abiProposal := buildReservationReanchorProposalAbi(
		sourceWalletPublicKeyHash,
		proposal,
	)

	expected := tbtcabi.WalletProposalValidatorReservationReanchorProposal{
		SourceWalletPubKeyHash: sourceWalletPublicKeyHash,
		ReservationKey:         big.NewInt(54321),
		TargetWalletPubKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:          big.NewInt(1700),
	}
	if !reflect.DeepEqual(expected, abiProposal) {
		t.Errorf(
			"unexpected abi proposal\nexpected: [%+v]\nactual:   [%+v]\n",
			expected,
			abiProposal,
		)
	}
}
