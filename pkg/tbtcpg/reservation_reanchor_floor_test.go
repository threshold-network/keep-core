package tbtcpg_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestProposeReservationReanchor_RejectsAnchorBelowFloor verifies that the
// proposal path applies the same request-time floor the run path applies:
// the anchor must stay strictly above txMaxFee + minAmount, or the on-chain
// request reverts with "Reanchor would fall below the minimum reservation
// amount" (see Reservation.sol).
//
// Without the check the proposal path burns a round and the gas of a request
// the contract rejects.
func TestProposeReservationReanchor_RejectsAnchorBelowFloor(t *testing.T) {
	sourceWalletPublicKeyHash := hexToByte20("1111111111111111111111111111111111111111")
	targetWalletPublicKeyHash := hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	// The floor is txMaxFee + minAmount = 10000 + 5000 = 15000. The
	// reservation's anchor is 15000, so it sits exactly on the floor and
	// the contract would reject the request.
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      10000,
		ReservationMinAmount:     5000,
		MaxReservationsPerWallet: 5,
		ReservationActionTimeout: 86400,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	resKey := big.NewInt(9002)
	anchorTxHash, _ := bitcoin.NewHashFromString(
		"8888888888888888888888888888888888888888888888888888888888888888",
		bitcoin.ReversedByteOrder,
	)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 15000, PublicKeyScript: sourceWalletScript},
		},
	})
	tbtcChain.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			Value: 15000,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{resKey})

	seedAddr := chain.Address("0x00000000000000000000000000000000000000ab")
	if err := tbtcChain.SetOperatorID(seedAddr, 7); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)
	proposal, err := task.ProposeReservationReanchor(
		reanchorTestLogger,
		sourceWalletPublicKeyHash,
		resKey,
		targetWalletPublicKeyHash,
		0,
		[]uint32{7},
		1,
	)
	if err == nil {
		t.Fatalf(
			"expected the proposal path to reject an anchor at the floor, "+
				"got proposal [%v]",
			proposal,
		)
	}

	// The error must be the client's own floor check, not the chain's
	// rejection: the client's check runs before the request, so no
	// request reaches the chain at all. A chain-side rejection would
	// mean the client still spent a round and the gas.
	if !strings.Contains(err.Error(), "is not above the re-anchor floor") {
		t.Errorf(
			"expected the client's own floor check to reject the proposal, "+
				"got [%v]",
			err,
		)
	}

	// No request must have reached the chain: the client rejects the
	// proposal before the request.
	if submissions := tbtcChain.GetReservationReanchorRequestSubmissions(); len(submissions) != 0 {
		t.Errorf(
			"expected no re-anchor request to be submitted, got [%d]",
			len(submissions),
		)
	}
}

// TestProposeReservationReanchor_RejectsNegativeAnchorValue pins that the
// proposal path rejects a reservation whose anchor value is negative,
// before the int64 to uint64 conversion that reads it.
//
// The conversion is guarded because a negative amount would wrap into a
// huge positive one and pass the floor check, which would let the client
// issue a request the contract rejects.
func TestProposeReservationReanchor_RejectsNegativeAnchorValue(t *testing.T) {
	sourceWalletPublicKeyHash := hexToByte20("1111111111111111111111111111111111111111")
	targetWalletPublicKeyHash := hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c")

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	tbtcChain.SetBlockCounter(blockCounter)

	tbtcChain.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	tbtcChain.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	tbtcChain.SetLiveWalletsCount(1)

	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      10000,
		ReservationMinAmount:     5000,
		MaxReservationsPerWallet: 5,
		ReservationActionTimeout: 86400,
	})
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	resKey := big.NewInt(9003)
	anchorTxHash, _ := bitcoin.NewHashFromString(
		"9999999999999999999999999999999999999999999999999999999999999999",
		bitcoin.ReversedByteOrder,
	)
	sourceWalletScript, err := bitcoin.PayToWitnessPublicKeyHash(sourceWalletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	btcChain.SetTransaction(anchorTxHash, &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1},
			{Value: 20000, PublicKeyScript: sourceWalletScript},
		},
	})
	tbtcChain.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		AnchorUtxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: anchorTxHash,
				OutputIndex:     1,
			},
			// A negative anchor value. It is above the floor once wrapped
			// into a uint64, so only an explicit guard rejects it.
			Value: -1,
		},
		State:        tbtc.ReservationStateActive,
		RequestNonce: 0,
	})
	tbtcChain.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{resKey})

	seedAddr := chain.Address("0x00000000000000000000000000000000000000ab")
	if err := tbtcChain.SetOperatorID(seedAddr, 7); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationReanchorTask(tbtcChain, btcChain)
	proposal, err := task.ProposeReservationReanchor(
		reanchorTestLogger,
		sourceWalletPublicKeyHash,
		resKey,
		targetWalletPublicKeyHash,
		0,
		[]uint32{7},
		1,
	)
	if err == nil {
		t.Fatalf(
			"expected the proposal path to reject a negative anchor value, "+
				"got proposal [%v]",
			proposal,
		)
	}

	if !strings.Contains(err.Error(), "negative anchor value") {
		t.Errorf(
			"expected the negative anchor value guard to reject the "+
				"proposal, got [%v]",
			err,
		)
	}

	if submissions := tbtcChain.GetReservationReanchorRequestSubmissions(); len(submissions) != 0 {
		t.Errorf(
			"expected no re-anchor request to be submitted, got [%d]",
			len(submissions),
		)
	}
}
