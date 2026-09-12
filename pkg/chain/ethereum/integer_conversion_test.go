package ethereum

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestCloserBlockUnsignedDistances(t *testing.T) {
	for _, tc := range []struct {
		timestamp, first, second uint64
		wantSecond               bool
	}{
		{0, math.MaxUint64, 1, true},
		{math.MaxUint64, 0, math.MaxUint64 - 1, true},
		{10, 9, 11, true},
		{math.MaxInt64, math.MaxInt64 - 1, math.MaxUint64, false},
	} {
		first := &types.Header{Number: big.NewInt(1), Time: tc.first}
		second := &types.Header{Number: big.NewInt(2), Time: tc.second}
		actual := closerBlock(tc.timestamp, first, second)
		if (actual == second) != tc.wantSecond {
			t.Fatalf("wrong block for timestamp %d, block times %d/%d", tc.timestamp, tc.first, tc.second)
		}
	}
}

func TestChainMemberIndexBounds(t *testing.T) {
	for _, value := range []*big.Int{nil, big.NewInt(-1), big.NewInt(0), big.NewInt(256), new(big.Int).Lsh(big.NewInt(1), 64)} {
		if _, err := memberIndexFromChain(value); err == nil {
			t.Fatalf("accepted index %v", value)
		}
	}
	for _, value := range []int64{1, 255} {
		index, err := memberIndexFromChain(big.NewInt(value))
		if err != nil || int64(index) != value {
			t.Fatalf("index %d: %d, %v", value, index, err)
		}
	}
}

func TestProofSubmissionsRejectNegativeUtxo(t *testing.T) {
	chain := new(TbtcChain)
	utxo := bitcoin.UnspentTransactionOutput{Value: -1}
	calls := []func() error{
		func() error { return chain.SubmitRedemptionProofWithReimbursement(nil, nil, utxo, [20]byte{}) },
		func() error { return chain.SubmitDepositSweepProofWithReimbursement(nil, nil, utxo, [20]byte{}) },
		func() error { return chain.SubmitMovingFundsProofWithReimbursement(nil, nil, utxo, [20]byte{}) },
		func() error { return chain.SubmitMovedFundsSweepProofWithReimbursement(nil, nil, utxo) },
		func() error { return chain.ValidateMovingFundsProposal([20]byte{}, &utxo, nil) },
	}
	for i, call := range calls {
		if err := call(); err == nil {
			t.Fatalf("submission %d accepted negative UTXO", i)
		}
	}
}
