package tbtc

import (
	"math"
	"math/big"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
)

func TestSignerUnmarshalMemberIndexBounds(t *testing.T) {
	original, err := createMockSigner(t).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	message := new(pb.Signer)
	if err := proto.Unmarshal(original, message); err != nil {
		t.Fatal(err)
	}
	for _, index := range []uint32{0, 1, 255, 256, math.MaxUint32} {
		message.SigningGroupMemberIndex = index
		data, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		decoded := new(signer)
		err = decoded.Unmarshal(data)
		if index > 255 {
			if err == nil {
				t.Fatalf("accepted out-of-range index %d", index)
			}
		} else if err != nil || uint32(decoded.signingGroupMemberIndex) != index {
			t.Fatalf("index %d: got %d, %v", index, decoded.signingGroupMemberIndex, err)
		}
	}
}

func TestDepositRevealBlockUnsignedRoundtrip(t *testing.T) {
	blocks := []uint64{0, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64}
	original := &DepositSweepProposal{SweepTxFee: big.NewInt(1)}
	for _, block := range blocks {
		original.DepositsRevealBlocks = append(original.DepositsRevealBlocks, new(big.Int).SetUint64(block))
	}
	data, err := original.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(DepositSweepProposal)
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatal(err)
	}
	for i, want := range original.DepositsRevealBlocks {
		if decoded.DepositsRevealBlocks[i].Cmp(want) != 0 {
			t.Fatalf("block %d: got %v, want %v", i, decoded.DepositsRevealBlocks[i], want)
		}
	}
}

func TestDepositRevealBlockMarshalBounds(t *testing.T) {
	for _, block := range []*big.Int{nil, big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 64)} {
		proposal := &DepositSweepProposal{SweepTxFee: big.NewInt(1), DepositsRevealBlocks: []*big.Int{block}}
		if _, err := proposal.Marshal(); err == nil {
			t.Fatalf("accepted block %v", block)
		}
	}
}

func TestDepositRevealedEventAmountBounds(t *testing.T) {
	for _, value := range []uint64{0, 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64} {
		deposit, err := (&DepositRevealedEvent{Amount: value}).unpack(nil)
		if value > math.MaxInt64 {
			if err == nil {
				t.Fatalf("accepted amount %d", value)
			}
		} else if err != nil || uint64(deposit.Utxo.Value) != value {
			t.Fatalf("amount %d: got %v, %v", value, deposit, err)
		}
	}
}
