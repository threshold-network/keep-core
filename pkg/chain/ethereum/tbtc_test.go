package ethereum

import (
	"crypto/ecdsa"
	"fmt"
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

func TestBuildReservationAnchorProposalAbi(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
	}
	fundingTxHash := bitcoin.Hash{0x21, 0x22, 0x23}

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: 7,
		// Non-zero RequestNonce exercises the P0 #3 fix's new mapping:
		// a zero value on both sides would silently pass even if the
		// builder omitted the field.
		RequestNonce: 17,
		AnchorTxFee:  big.NewInt(1500),
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
		RequestNonce: 17,
		AnchorTxFee:  big.NewInt(1500),
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
		ReservationKey: big.NewInt(54321),
		// Non-zero RequestNonce exercises the P0 #3 fix's new mapping:
		// a zero value on both sides would silently pass even if the
		// builder omitted the field.
		RequestNonce:              23,
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
		RequestNonce:           23,
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

// TestResolveCustodiedReservationKeys exercises the pure-logic core of
// TbtcChain.WalletReservations against fake event slices and a fake
// reservation lookup - the surrounding TbtcChain method depends on real
// go-ethereum simulated-backend infrastructure that does not exist anywhere
// in pkg/chain/ethereum today (a package-wide gap, explicitly deferred).
//
// The four subtests cover the actual failure modes the original P0 fix
// turns on: a key present in both event lists appears exactly once
// (dedup), a reservation whose current custody is a different wallet is
// excluded (reanchored-away), a key only reachable via the reanchor
// event list is included (reanchored-in), and the output is sorted
// deterministically regardless of event order.
func TestResolveCustodiedReservationKeys(t *testing.T) {
	walletPKH := [20]byte{0x42}
	otherPKH := [20]byte{0x99}

	acceptance := func(key int64, pkh [20]byte) *tbtc.ReservationAcceptanceRequestedEvent {
		return &tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      big.NewInt(key),
			WalletPublicKeyHash: pkh,
		}
	}
	reanchor := func(key int64, targetPKH [20]byte) *tbtc.ReservationReanchorRequestedEvent {
		return &tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            big.NewInt(key),
			TargetWalletPublicKeyHash: targetPKH,
		}
	}
	reservation := func(pkh [20]byte) *tbtc.Reservation {
		return &tbtc.Reservation{
			WalletPublicKeyHash: pkh,
		}
	}

	// Single dispatch: every reservation key gets a deterministic outcome
	// from one lookup table. Keys 1 and 3 are currently custodied by
	// walletPKH; key 2 has reanchored away to otherPKH; key 4 only
	// appears via a reanchor event (reanchored-into this wallet).
	lookup := func(key *big.Int) (*tbtc.Reservation, error) {
		switch key.Int64() {
		case 1:
			return reservation(walletPKH), nil
		case 2:
			return reservation(otherPKH), nil
		case 3:
			return reservation(walletPKH), nil
		case 4:
			return reservation(walletPKH), nil
		}
		return nil, fmt.Errorf("unexpected lookup call")
	}

	t.Run("dedup - key in both event lists appears once", func(t *testing.T) {
		keys, err := resolveCustodiedReservationKeys(
			walletPKH,
			[]*tbtc.ReservationAcceptanceRequestedEvent{acceptance(1, walletPKH)},
			[]*tbtc.ReservationReanchorRequestedEvent{reanchor(1, walletPKH)},
			lookup,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedKeys := []*big.Int{big.NewInt(1)}
		if !reflect.DeepEqual(expectedKeys, keys) {
			t.Errorf("expected [%+v], actual [%+v]", expectedKeys, keys)
		}
	})

	t.Run("custody filter - reanchored-away excluded", func(t *testing.T) {
		keys, err := resolveCustodiedReservationKeys(
			walletPKH,
			[]*tbtc.ReservationAcceptanceRequestedEvent{
				acceptance(1, walletPKH),
				acceptance(2, walletPKH),
				acceptance(3, walletPKH),
			},
			nil,
			lookup,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedKeys := []*big.Int{big.NewInt(1), big.NewInt(3)}
		if !reflect.DeepEqual(expectedKeys, keys) {
			t.Errorf("expected [%+v], actual [%+v]", expectedKeys, keys)
		}
	})

	t.Run("reanchored-in - key only in reanchor event list", func(t *testing.T) {
		keys, err := resolveCustodiedReservationKeys(
			walletPKH,
			nil,
			[]*tbtc.ReservationReanchorRequestedEvent{reanchor(4, walletPKH)},
			lookup,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedKeys := []*big.Int{big.NewInt(4)}
		if !reflect.DeepEqual(expectedKeys, keys) {
			t.Errorf("expected [%+v], actual [%+v]", expectedKeys, keys)
		}
	})

	t.Run("sorted output - reverse event order produces ascending keys", func(t *testing.T) {
		// Reverse the natural order to prove the sort isn't incidental
		// to event iteration order.
		keys, err := resolveCustodiedReservationKeys(
			walletPKH,
			[]*tbtc.ReservationAcceptanceRequestedEvent{
				acceptance(3, walletPKH),
				acceptance(1, walletPKH),
			},
			[]*tbtc.ReservationReanchorRequestedEvent{
				reanchor(2, otherPKH),
				reanchor(1, walletPKH),
			},
			lookup,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedKeys := []*big.Int{big.NewInt(1), big.NewInt(3)}
		if !reflect.DeepEqual(expectedKeys, keys) {
			t.Errorf("expected [%+v], actual [%+v]", expectedKeys, keys)
		}
	})

	t.Run("lookup error short-circuits the whole call", func(t *testing.T) {
		failingLookup := func(key *big.Int) (*tbtc.Reservation, error) {
			return nil, fmt.Errorf("simulated lookup failure")
		}
		_, err := resolveCustodiedReservationKeys(
			walletPKH,
			[]*tbtc.ReservationAcceptanceRequestedEvent{acceptance(1, walletPKH)},
			nil,
			failingLookup,
		)
		if err == nil {
			t.Fatal("expected lookup error to propagate out of resolveCustodiedReservationKeys")
		}
	})
}

// TestEarliestRegistrationEventBlock exercises the pure-logic core of
// TbtcChain.earliestWalletRegistrationBlock: given a set of
// NewWalletRegistered events already scoped to one wallet, resolve the
// block number of the earliest one. The surrounding TbtcChain method
// depends on real go-ethereum simulated-backend infrastructure that does
// not exist anywhere in pkg/chain/ethereum today (a package-wide gap,
// explicitly deferred), so this pure-logic core is tested directly
// instead, mirroring the existing pattern in this file (e.g.
// resolveCustodiedReservationKeys).
func TestEarliestRegistrationEventBlock(t *testing.T) {
	event := func(blockNumber uint64) *tbtc.NewWalletRegisteredEvent {
		return &tbtc.NewWalletRegisteredEvent{BlockNumber: blockNumber}
	}

	tests := map[string]struct {
		events        []*tbtc.NewWalletRegisteredEvent
		expectedBlock uint64
	}{
		"zero registration events found": {
			events:        nil,
			expectedBlock: 0,
		},
		"exactly one registration event": {
			events:        []*tbtc.NewWalletRegisteredEvent{event(500)},
			expectedBlock: 500,
		},
		"multiple registration events - earliest wins regardless of order": {
			events: []*tbtc.NewWalletRegisteredEvent{
				event(900),
				event(300),
				event(700),
			},
			expectedBlock: 300,
		},
		"nil entry and zero-block entry are ignored": {
			events: []*tbtc.NewWalletRegisteredEvent{
				event(0),
				nil,
				event(600),
			},
			expectedBlock: 600,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actual := earliestRegistrationEventBlock(test.events)
			if actual != test.expectedBlock {
				t.Errorf(
					"expected earliest block [%d], actual [%d]",
					test.expectedBlock,
					actual,
				)
			}
		})
	}
}

// TestResolveWalletTerminationCause exercises the pure-logic core of
// TbtcChain.WalletTerminationCause: the fixed MovingFundsTimedOut ->
// MovedFundsSweepTimedOut -> FraudChallengeDefeatTimedOut priority order
// documented on WalletTerminationCause. The two "both present" subtests
// pin the case the doc comment now calls out explicitly - more than one
// pre-termination timeout event found for the same wallet, which is not
// expected in normal Bridge operation but is not verified impossible by
// the surrounding method - confirming the higher-priority cause always
// wins rather than being left as an untested assumption.
func TestResolveWalletTerminationCause(t *testing.T) {
	tests := map[string]struct {
		movingFundsEventCount     int
		movedFundsSweepEventCount int
		fraudChallengeEventCount  int
		expectedCause             tbtc.WalletTerminationCause
	}{
		"no events found": {
			expectedCause: tbtc.WalletTerminationCauseUnknown,
		},
		"only moving funds timed out": {
			movingFundsEventCount: 1,
			expectedCause:         tbtc.WalletTerminationCauseMovingFundsTimeout,
		},
		"only moved funds sweep timed out": {
			movedFundsSweepEventCount: 1,
			expectedCause:             tbtc.WalletTerminationCauseMovedFundsSweepTimeout,
		},
		"only fraud challenge defeat timed out": {
			fraudChallengeEventCount: 1,
			expectedCause:            tbtc.WalletTerminationCauseFraudChallengeDefeat,
		},
		"moving funds and fraud challenge both present - moving funds wins": {
			movingFundsEventCount:    1,
			fraudChallengeEventCount: 1,
			expectedCause:            tbtc.WalletTerminationCauseMovingFundsTimeout,
		},
		"moved funds sweep and fraud challenge both present - moved funds sweep wins": {
			movedFundsSweepEventCount: 1,
			fraudChallengeEventCount:  1,
			expectedCause:             tbtc.WalletTerminationCauseMovedFundsSweepTimeout,
		},
		"all three present - moving funds still wins": {
			movingFundsEventCount:     1,
			movedFundsSweepEventCount: 1,
			fraudChallengeEventCount:  1,
			expectedCause:             tbtc.WalletTerminationCauseMovingFundsTimeout,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actual := resolveWalletTerminationCause(
				test.movingFundsEventCount,
				test.movedFundsSweepEventCount,
				test.fraudChallengeEventCount,
			)
			if actual != test.expectedCause {
				t.Errorf(
					"expected termination cause [%v], actual [%v]",
					test.expectedCause,
					actual,
				)
			}
		})
	}
}
