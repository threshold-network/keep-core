package tbtc

import (
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

func TestReservationProposals_MarshalingRoundtrip(t *testing.T) {
	anchorProposal := &ReservationAnchorProposal{
		DepositFundingTxHash:      bitcoin.Hash{0x01, 0x02},
		DepositFundingOutputIndex: 3,
		AnchorTxFee:               big.NewInt(1500),
	}

	redemptionProposal := &ReservedRedemptionProposal{
		ReservationKey:  big.NewInt(12345),
		RedemptionTxFee: big.NewInt(1600),
	}

	reanchorProposal := &ReservationReanchorProposal{
		ReservationKey:            big.NewInt(54321),
		TargetWalletPublicKeyHash: [20]byte{0xaa, 0xbb},
		ReanchorTxFee:             big.NewInt(1700),
	}

	dissolutionProposal := &ReservationDissolutionProposal{
		ReservationKey:   big.NewInt(99999),
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

func TestAssembleReservationTransactions_InputValidation(t *testing.T) {
	bitcoinChain := newLocalBitcoinChain()
	walletPublicKeyHash := [20]byte{0x01}
	redeemerScript := bitcoin.Script{0x00, 0x14, 0x02}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x03},
			OutputIndex:     0,
		},
		Value: 100000,
	}

	assertError := func(err error, expected string) {
		if err == nil || err.Error() != expected {
			t.Errorf("expected error [%v], got [%v]", expected, err)
		}
	}

	_, err := assembleReservationAnchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		1500,
	)
	assertError(err, "deposit is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		nil,
		redeemerScript,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservedRedemptionTransaction(
		bitcoinChain,
		anchorUtxo,
		bitcoin.Script{},
		1500,
	)
	assertError(err, "redeemer output script is required")

	_, err = assembleReservationReanchorTransaction(
		bitcoinChain,
		nil,
		walletPublicKeyHash,
		1500,
	)
	assertError(err, "anchor UTXO is required")

	_, err = assembleReservationDissolutionTransaction(
		bitcoinChain,
		nil,
		nil,
		walletPublicKeyHash,
		1500,
	)
	assertError(err, "anchor UTXO is required")
}
