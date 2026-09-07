package ethereum

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
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
