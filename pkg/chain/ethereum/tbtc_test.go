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

// TestConvertReservationParametersFromAbiType verifies the field-by-field
// mapping performed by convertReservationParametersFromAbiType, including
// the address-to-chain.Address conversion and that no fields are dropped or
// shifted.
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
