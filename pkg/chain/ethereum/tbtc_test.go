package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtcpkg "github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/keep-network/keep-core/pkg/chain"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
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

type pastNewWalletRegisteredV2EventsBridgeMock struct {
	pastEvents func(
		startBlock uint64,
		endBlock *uint64,
		walletID [][32]byte,
		ecdsaWalletID [][32]byte,
		walletPublicKeyHash [][20]byte,
	) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error)
}

func (m *pastNewWalletRegisteredV2EventsBridgeMock) PastNewWalletRegisteredV2Events(
	startBlock uint64,
	endBlock *uint64,
	walletID [][32]byte,
	ecdsaWalletID [][32]byte,
	walletPublicKeyHash [][20]byte,
) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error) {
	return m.pastEvents(
		startBlock,
		endBlock,
		walletID,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
}

type pastNewWalletRegisteredV2EventsAltFieldBridgeMock struct {
	pastEvents func(
		startBlock uint64,
		endBlock *uint64,
		walletID [][32]byte,
		ecdsaWalletID [][32]byte,
		walletPublicKeyHash [][20]byte,
	) ([]*pastNewWalletRegisteredV2EventsAltFieldEvent, error)
}

type pastNewWalletRegisteredV2EventsAltFieldEvent struct {
	WalletID            [32]byte
	EcdsaWalletID       [32]byte
	WalletPublicKeyHash [20]byte
	Raw                 types.Log
}

func (m *pastNewWalletRegisteredV2EventsAltFieldBridgeMock) PastNewWalletRegisteredV2Events(
	startBlock uint64,
	endBlock *uint64,
	walletID [][32]byte,
	ecdsaWalletID [][32]byte,
	walletPublicKeyHash [][20]byte,
) ([]*pastNewWalletRegisteredV2EventsAltFieldEvent, error) {
	return m.pastEvents(
		startBlock,
		endBlock,
		walletID,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
}

type pastNewWalletRegisteredV2EventsMissingRawBridgeMock struct {
	pastEvents func(
		startBlock uint64,
		endBlock *uint64,
		walletID [][32]byte,
		ecdsaWalletID [][32]byte,
		walletPublicKeyHash [][20]byte,
	) ([]*pastNewWalletRegisteredV2EventsMissingRawEvent, error)
}

type pastNewWalletRegisteredV2EventsMissingRawEvent struct {
	WalletID         [32]byte
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
}

func (m *pastNewWalletRegisteredV2EventsMissingRawBridgeMock) PastNewWalletRegisteredV2Events(
	startBlock uint64,
	endBlock *uint64,
	walletID [][32]byte,
	ecdsaWalletID [][32]byte,
	walletPublicKeyHash [][20]byte,
) ([]*pastNewWalletRegisteredV2EventsMissingRawEvent, error) {
	return m.pastEvents(
		startBlock,
		endBlock,
		walletID,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
}

type pastNewWalletRegisteredV2EventsWrongSignatureBridgeMock struct{}

func (m *pastNewWalletRegisteredV2EventsWrongSignatureBridgeMock) PastNewWalletRegisteredV2Events(
	startBlock uint64,
) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error) {
	return nil, nil
}

func TestPastNewWalletRegisteredEvents_UsesV2EventsWhenAvailable(t *testing.T) {
	startBlock := uint64(500)
	endBlock := uint64(700)

	expectedWalletIDA := [32]byte{0xaa}
	expectedWalletIDB := [32]byte{0xbb}

	expectedECDSAWalletIDA := [32]byte{0xa1}
	expectedECDSAWalletIDB := [32]byte{0xb1}

	expectedWalletPublicKeyHashA := [20]byte{0x11}
	expectedWalletPublicKeyHashB := [20]byte{0x22}

	legacyFallbackCalled := false

	actualEvents, err := pastNewWalletRegisteredEvents(
		startBlock,
		&endBlock,
		nil,
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsBridgeMock{
			pastEvents: func(
				actualStartBlock uint64,
				actualEndBlock *uint64,
				_ [][32]byte,
				_ [][32]byte,
				_ [][20]byte,
			) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error) {
				if actualStartBlock != startBlock {
					t.Fatalf("unexpected start block: [%v]", actualStartBlock)
				}

				if actualEndBlock == nil || *actualEndBlock != endBlock {
					t.Fatalf("unexpected end block: [%v]", actualEndBlock)
				}

				// Provide events out of order to verify post-conversion sort.
				return []*tbtcabi.BridgeNewWalletRegisteredV2{
					{
						WalletID:         expectedWalletIDB,
						EcdsaWalletID:    expectedECDSAWalletIDB,
						WalletPubKeyHash: expectedWalletPublicKeyHashB,
						Raw:              types.Log{BlockNumber: 650},
					},
					{
						WalletID:         expectedWalletIDA,
						EcdsaWalletID:    expectedECDSAWalletIDA,
						WalletPubKeyHash: expectedWalletPublicKeyHashA,
						Raw:              types.Log{BlockNumber: 600},
					},
				}, nil
			},
		},
		func(uint64, *uint64, [][32]byte, [][20]byte) ([]*tbtcabi.BridgeNewWalletRegistered, error) {
			legacyFallbackCalled = true
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if legacyFallbackCalled {
		t.Fatal("legacy fallback should not be called when v2 events are present")
	}

	if len(actualEvents) != 2 {
		t.Fatalf("unexpected events count: [%v]", len(actualEvents))
	}

	// Expect ascending block order after conversion.
	if actualEvents[0].BlockNumber != 600 || actualEvents[1].BlockNumber != 650 {
		t.Fatalf(
			"unexpected event ordering by block: [%v], [%v]",
			actualEvents[0].BlockNumber,
			actualEvents[1].BlockNumber,
		)
	}

	if actualEvents[0].WalletID != expectedWalletIDA ||
		actualEvents[1].WalletID != expectedWalletIDB {
		t.Fatal("unexpected wallet IDs in converted events")
	}
}

func TestPastNewWalletRegisteredEvents_FallsBackToLegacyWhenV2Empty(t *testing.T) {
	expectedECDSAWalletID := [32]byte{0xdd}
	expectedWalletPublicKeyHash := [20]byte{0xee}

	legacyFallbackCalled := false

	actualEvents, err := pastNewWalletRegisteredEvents(
		1,
		nil,
		nil, // no canonical wallet-ID filter -> fallback path enabled
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsBridgeMock{
			pastEvents: func(
				uint64,
				*uint64,
				[][32]byte,
				[][32]byte,
				[][20]byte,
			) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error) {
				return []*tbtcabi.BridgeNewWalletRegisteredV2{}, nil
			},
		},
		func(uint64, *uint64, [][32]byte, [][20]byte) ([]*tbtcabi.BridgeNewWalletRegistered, error) {
			legacyFallbackCalled = true
			return []*tbtcabi.BridgeNewWalletRegistered{
				{
					EcdsaWalletID:    expectedECDSAWalletID,
					WalletPubKeyHash: expectedWalletPublicKeyHash,
					Raw:              types.Log{BlockNumber: 1000},
				},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if !legacyFallbackCalled {
		t.Fatal("legacy fallback should be called when v2 events are empty")
	}

	if len(actualEvents) != 1 {
		t.Fatalf("unexpected events count: [%v]", len(actualEvents))
	}

	expectedWalletID := tbtcpkg.DeriveLegacyWalletID(expectedWalletPublicKeyHash)
	if actualEvents[0].WalletID != expectedWalletID {
		t.Fatalf(
			"unexpected derived legacy wallet ID\nexpected: [%x]\nactual:   [%x]",
			expectedWalletID,
			actualEvents[0].WalletID,
		)
	}
}

func TestPastNewWalletRegisteredEvents_DoesNotFallbackWithWalletIDFilter(t *testing.T) {
	legacyFallbackCalled := false

	walletIDFilter := [][32]byte{
		{0x1},
	}

	actualEvents, err := pastNewWalletRegisteredEvents(
		1,
		nil,
		walletIDFilter,
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsBridgeMock{
			pastEvents: func(
				uint64,
				*uint64,
				[][32]byte,
				[][32]byte,
				[][20]byte,
			) ([]*tbtcabi.BridgeNewWalletRegisteredV2, error) {
				return []*tbtcabi.BridgeNewWalletRegisteredV2{}, nil
			},
		},
		func(uint64, *uint64, [][32]byte, [][20]byte) ([]*tbtcabi.BridgeNewWalletRegistered, error) {
			legacyFallbackCalled = true
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if legacyFallbackCalled {
		t.Fatal("legacy fallback should be skipped when walletID filter is provided")
	}

	if len(actualEvents) != 0 {
		t.Fatalf("unexpected events count: [%v]", len(actualEvents))
	}
}

func TestPastNewWalletRegisteredV2Events_ReturnsEmptyWhenMethodUnavailable(t *testing.T) {
	actualEvents, err := pastNewWalletRegisteredV2Events(
		1,
		nil,
		nil,
		nil,
		nil,
		struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if len(actualEvents) != 0 {
		t.Fatalf("unexpected events count: [%v]", len(actualEvents))
	}
}

func TestPastNewWalletRegisteredV2Events_UsesWalletPublicKeyHashFallbackField(t *testing.T) {
	expectedWalletID := [32]byte{0x01}
	expectedECDSAWalletID := [32]byte{0x02}
	expectedWalletPublicKeyHash := [20]byte{0x03}

	actualEvents, err := pastNewWalletRegisteredV2Events(
		11,
		nil,
		nil,
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsAltFieldBridgeMock{
			pastEvents: func(
				uint64,
				*uint64,
				[][32]byte,
				[][32]byte,
				[][20]byte,
			) ([]*pastNewWalletRegisteredV2EventsAltFieldEvent, error) {
				return []*pastNewWalletRegisteredV2EventsAltFieldEvent{
					{
						WalletID:            expectedWalletID,
						EcdsaWalletID:       expectedECDSAWalletID,
						WalletPublicKeyHash: expectedWalletPublicKeyHash,
						Raw:                 types.Log{BlockNumber: 121},
					},
				}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if len(actualEvents) != 1 {
		t.Fatalf("unexpected events count: [%v]", len(actualEvents))
	}

	if actualEvents[0].WalletPublicKeyHash != expectedWalletPublicKeyHash {
		t.Fatalf(
			"unexpected wallet public key hash\nexpected: [%x]\nactual:   [%x]",
			expectedWalletPublicKeyHash,
			actualEvents[0].WalletPublicKeyHash,
		)
	}
}

func TestPastNewWalletRegisteredV2Events_ReturnsErrorOnCallPanic(t *testing.T) {
	_, err := pastNewWalletRegisteredV2Events(
		1,
		nil,
		nil,
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsWrongSignatureBridgeMock{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "panic calling PastNewWalletRegisteredV2Events") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestPastNewWalletRegisteredV2Events_ReturnsErrorWhenRawMissing(t *testing.T) {
	_, err := pastNewWalletRegisteredV2Events(
		1,
		nil,
		nil,
		nil,
		nil,
		&pastNewWalletRegisteredV2EventsMissingRawBridgeMock{
			pastEvents: func(
				uint64,
				*uint64,
				[][32]byte,
				[][32]byte,
				[][20]byte,
			) ([]*pastNewWalletRegisteredV2EventsMissingRawEvent, error) {
				return []*pastNewWalletRegisteredV2EventsMissingRawEvent{
					{
						WalletID:         [32]byte{0x05},
						EcdsaWalletID:    [32]byte{0x06},
						WalletPubKeyHash: [20]byte{0x07},
					},
				}, nil
			},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "raw event payload") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

type walletPublicKeyHashForWalletIDBridgeMock struct {
	resolve func(walletID [32]byte) ([20]byte, error)
}

func (m *walletPublicKeyHashForWalletIDBridgeMock) WalletPubKeyHashForWalletID(
	walletID [32]byte,
) ([20]byte, error) {
	return m.resolve(walletID)
}

func TestResolveWalletPublicKeyHashForWalletID(t *testing.T) {
	t.Run("returns canonical mapping when non-zero", func(t *testing.T) {
		walletID := [32]byte{0x01}
		expectedWalletPublicKeyHash := [20]byte{0xaa}

		actualWalletPublicKeyHash, err := resolveWalletPublicKeyHashForWalletID(
			walletID,
			&walletPublicKeyHashForWalletIDBridgeMock{
				resolve: func(actualWalletID [32]byte) ([20]byte, error) {
					if actualWalletID != walletID {
						t.Fatalf("unexpected wallet ID: [%x]", actualWalletID)
					}

					return expectedWalletPublicKeyHash, nil
				},
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}

		if actualWalletPublicKeyHash != expectedWalletPublicKeyHash {
			t.Fatalf(
				"unexpected wallet public key hash\nexpected: [%x]\nactual:   [%x]",
				expectedWalletPublicKeyHash,
				actualWalletPublicKeyHash,
			)
		}
	})

	t.Run("falls back to legacy extraction when canonical lookup errors", func(t *testing.T) {
		expectedWalletPublicKeyHash := [20]byte{0xbb}
		legacyWalletID := tbtcpkg.DeriveLegacyWalletID(expectedWalletPublicKeyHash)

		actualWalletPublicKeyHash, err := resolveWalletPublicKeyHashForWalletID(
			legacyWalletID,
			&walletPublicKeyHashForWalletIDBridgeMock{
				resolve: func([32]byte) ([20]byte, error) {
					return [20]byte{}, errors.New("canonical lookup unavailable")
				},
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}

		if actualWalletPublicKeyHash != expectedWalletPublicKeyHash {
			t.Fatalf(
				"unexpected wallet public key hash\nexpected: [%x]\nactual:   [%x]",
				expectedWalletPublicKeyHash,
				actualWalletPublicKeyHash,
			)
		}
	})

	t.Run("falls back to legacy extraction when canonical lookup returns zero", func(t *testing.T) {
		expectedWalletPublicKeyHash := [20]byte{0xbc}
		legacyWalletID := tbtcpkg.DeriveLegacyWalletID(expectedWalletPublicKeyHash)

		actualWalletPublicKeyHash, err := resolveWalletPublicKeyHashForWalletID(
			legacyWalletID,
			&walletPublicKeyHashForWalletIDBridgeMock{
				resolve: func([32]byte) ([20]byte, error) {
					return [20]byte{}, nil
				},
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}

		if actualWalletPublicKeyHash != expectedWalletPublicKeyHash {
			t.Fatalf(
				"unexpected wallet public key hash\nexpected: [%x]\nactual:   [%x]",
				expectedWalletPublicKeyHash,
				actualWalletPublicKeyHash,
			)
		}
	})

	t.Run("returns wrapped canonical error for non-legacy IDs", func(t *testing.T) {
		walletID := [32]byte{0xff}
		canonicalErr := errors.New("rpc failure")

		_, err := resolveWalletPublicKeyHashForWalletID(
			walletID,
			&walletPublicKeyHashForWalletIDBridgeMock{
				resolve: func([32]byte) ([20]byte, error) {
					return [20]byte{}, canonicalErr
				},
			},
		)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "cannot resolve wallet public key hash") {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if !strings.Contains(err.Error(), canonicalErr.Error()) {
			t.Fatalf("expected canonical error to be wrapped: [%v]", err)
		}
	})

	t.Run("returns not found for non-legacy IDs when canonical lookup returns zero", func(t *testing.T) {
		walletID := [32]byte{0xfe}

		_, err := resolveWalletPublicKeyHashForWalletID(
			walletID,
			&walletPublicKeyHashForWalletIDBridgeMock{
				resolve: func([32]byte) ([20]byte, error) {
					return [20]byte{}, nil
				},
			},
		)
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "wallet public key hash not found") {
			t.Fatalf("unexpected error: [%v]", err)
		}
	})
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
