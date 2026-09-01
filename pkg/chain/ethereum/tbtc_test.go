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

// TestConvertReservationParametersFromAbiType verifies the full 10-tuple
// field mapping performed by convertReservationParametersFromAbiType.
// Gap-analysis Minor row: field count/order was not yet cross-checked
// against the live Solidity struct; every field below is set to a distinct
// non-zero value so a swapped or dropped field is caught, not masked by a
// shared zero-value default.
func TestConvertReservationParametersFromAbiType(t *testing.T) {
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
		ReservationVault:                common.HexToAddress("0x1111111111111111111111111111111111111a"),
		ReservationMinAmount:            10000,
		ReservationTxMaxFee:             20000,
		ReservationTermSeconds:          30000,
		ReservationDissolutionDelay:     40000,
		ReservationMaxTotalAmount:       50000,
		ReservationTotalAmount:          60000,
		MaxReservationsPerWallet:        70000,
		ReservationActionTimeout:        80000,
		ReservationRenewalWindowSeconds: 90000,
	}

	expected := &tbtc.ReservationParameters{
		ReservationVault:                chain.Address(common.HexToAddress("0x1111111111111111111111111111111111111a").String()),
		ReservationMinAmount:            10000,
		ReservationTxMaxFee:             20000,
		ReservationTermSeconds:          30000,
		ReservationDissolutionDelay:     40000,
		ReservationMaxTotalAmount:       50000,
		ReservationTotalAmount:          60000,
		MaxReservationsPerWallet:        70000,
		ReservationActionTimeout:        80000,
		ReservationRenewalWindowSeconds: 90000,
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

// TestConvertReservationFromAbiType_DropsCumulativeReanchorFee documents
// the intentional CumulativeReanchorFee drop performed by
// convertReservationFromAbiType: the field is written on-chain by every
// re-anchor hop but is not exposed on tbtc.Reservation because m1 has no
// fee-ceiling enforcement (own comment, tbtc.go:2672-2676). This test both
// pins that intentional omission and verifies every other field maps
// correctly - each field below is a distinct value so a future accidental
// restoration of CumulativeReanchorFee, or a swapped adjacent field, does
// not go unnoticed.
func TestConvertReservationFromAbiType_DropsCumulativeReanchorFee(t *testing.T) {
	abiReservation := tbtcabi.ReservationReservationRequest{
		Owner:                 common.HexToAddress("0x1111111111111111111111111111111111111b"),
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
		Owner:        chain.Address(common.HexToAddress("0x1111111111111111111111111111111111111b").String()),
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
}
