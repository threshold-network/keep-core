package tbtc

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"testing"
)

// FuzzDepositSweepProposalRoundTrip tests the round-trip marshaling and unmarshaling
// of DepositSweepProposal structs with clamped, valid fields.
//
// This target would have caught the tbtc DepositSweepProposal generator bug where
// DepositsRevealBlocks contained unbounded *big.Int values (up to 512 big.Words)
// produced by the default fuzzBigInt generator. DepositSweepProposal.Marshal
// strictly requires each reveal block to satisfy block.IsUint64() (fitting into
// [0, 2^64)), causing unmarshaled proposals to fail with "invalid deposit reveal block".
func FuzzDepositSweepProposalRoundTrip(f *testing.F) {
	// Seed 1: Empty and all-zero inputs (zero fee, zero keys, zero reveal blocks).
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 48))

	// Seed 2: Small inputs producing small fees, indices, and reveal blocks.
	f.Add([]byte{0x01})
	f.Add([]byte{0x00, 0x01, 0x00, 0x00, 0x03, 0xe8})

	smallSeed := make([]byte, 0, 48)
	smallSeed = append(smallSeed, 0x08)                                                      // fee len
	smallSeed = append(smallSeed, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}...) // fee = 256
	var smallHash [32]byte
	smallHash[0] = 0x01
	smallSeed = append(smallSeed, smallHash[:]...)
	smallSeed = binary.BigEndian.AppendUint32(smallSeed, 1)
	smallSeed = binary.BigEndian.AppendUint64(smallSeed, 500)
	f.Add(smallSeed)

	// Seed 3: Large in-range values near the top of the accepted domain [0, 2^64)
	// including reveal blocks equal to MaxUint64 (2^64 - 1), max uint32 output index,
	// and maximum 256-bit fee.
	maxSeed := make([]byte, 0, 80)
	maxSeed = append(maxSeed, 32) // fee len: 32 bytes of 0xFF
	maxSeed = append(maxSeed, bytes.Repeat([]byte{0xFF}, 32)...)
	maxSeed = append(maxSeed, bytes.Repeat([]byte{0xFF}, 32)...) // 32-byte hash
	maxSeed = binary.BigEndian.AppendUint32(maxSeed, math.MaxUint32)
	maxSeed = binary.BigEndian.AppendUint64(maxSeed, math.MaxUint64) // 2^64 - 1
	f.Add(maxSeed)

	f.Add(bytes.Repeat([]byte{0xFF}, 128))

	f.Fuzz(func(t *testing.T, data []byte) {
		proposal := buildFuzzedDepositSweepProposal(data)

		marshaled, err := proposal.Marshal()
		if err != nil {
			t.Fatalf("failed to marshal valid DepositSweepProposal: %v", err)
		}

		unmarshaled := &DepositSweepProposal{}
		if err := unmarshaled.Unmarshal(marshaled); err != nil {
			t.Fatalf("failed to unmarshal DepositSweepProposal: %v", err)
		}

		// Explicit field-by-field equality comparison to ensure exact mathematical
		// value checks for *big.Int pointers and detailed diagnostic output on mismatch.
		assertDepositSweepProposalEqual(t, proposal, unmarshaled)
	})
}

func buildFuzzedDepositSweepProposal(data []byte) *DepositSweepProposal {
	mod := new(big.Int).Lsh(big.NewInt(1), 64)

	if len(data) == 0 {
		return &DepositSweepProposal{
			DepositsKeys:         []DepositKey{},
			SweepTxFee:           big.NewInt(0),
			DepositsRevealBlocks: []*big.Int{},
		}
	}

	r := bytes.NewReader(data)

	// 1. Fee: read 0..32 bytes and convert to non-negative *big.Int
	feeLenByte, err := r.ReadByte()
	if err != nil {
		feeLenByte = 0
	}
	feeLen := int(feeLenByte % 33)
	feeBuf := make([]byte, feeLen)
	_, _ = r.Read(feeBuf)
	sweepTxFee := new(big.Int).SetBytes(feeBuf)

	// 2. DepositsKeys: read chunks of 36 bytes (32 bytes hash + 4 bytes index)
	keys := make([]DepositKey, 0)
	for r.Len() >= 36 {
		var hash [32]byte
		_, _ = r.Read(hash[:])
		var indexBuf [4]byte
		_, _ = r.Read(indexBuf[:])
		index := binary.BigEndian.Uint32(indexBuf[:])
		keys = append(keys, DepositKey{
			FundingTxHash:      hash,
			FundingOutputIndex: index,
		})
	}

	// 3. DepositsRevealBlocks: chunk remaining bytes into 8-byte uint64s / big.Ints
	// clamped to [0, 2^64)
	revealBlocks := make([]*big.Int, 0)
	for r.Len() >= 8 {
		var blockBuf [8]byte
		_, _ = r.Read(blockBuf[:])
		val := binary.BigEndian.Uint64(blockBuf[:])
		revealBlocks = append(revealBlocks, new(big.Int).SetUint64(val))
	}

	// Any remaining bytes are turned into a big.Int mod 2^64
	if r.Len() > 0 {
		remBuf := make([]byte, r.Len())
		_, _ = r.Read(remBuf)
		block := new(big.Int).SetBytes(remBuf)
		block.Mod(block, mod)
		revealBlocks = append(revealBlocks, block)
	}

	return &DepositSweepProposal{
		DepositsKeys:         keys,
		SweepTxFee:           sweepTxFee,
		DepositsRevealBlocks: revealBlocks,
	}
}

func assertDepositSweepProposalEqual(t *testing.T, expected, actual *DepositSweepProposal) {
	t.Helper()

	if (expected == nil) != (actual == nil) {
		t.Fatalf("proposal nilness mismatch: expected %v, got %v", expected, actual)
	}
	if expected == nil {
		return
	}

	// DepositsKeys comparison
	if len(expected.DepositsKeys) != len(actual.DepositsKeys) {
		t.Fatalf(
			"DepositsKeys length mismatch: expected %d, got %d",
			len(expected.DepositsKeys),
			len(actual.DepositsKeys),
		)
	}
	for i := range expected.DepositsKeys {
		if expected.DepositsKeys[i] != actual.DepositsKeys[i] {
			t.Fatalf(
				"DepositsKeys[%d] mismatch:\nexpected: %+v\nactual:   %+v",
				i,
				expected.DepositsKeys[i],
				actual.DepositsKeys[i],
			)
		}
	}

	// SweepTxFee comparison (*big.Int.Cmp)
	if (expected.SweepTxFee == nil) != (actual.SweepTxFee == nil) {
		t.Fatalf(
			"SweepTxFee nilness mismatch: expected %v, got %v",
			expected.SweepTxFee,
			actual.SweepTxFee,
		)
	}
	if expected.SweepTxFee != nil && expected.SweepTxFee.Cmp(actual.SweepTxFee) != 0 {
		t.Fatalf(
			"SweepTxFee mismatch:\nexpected: %s\nactual:   %s",
			expected.SweepTxFee.String(),
			actual.SweepTxFee.String(),
		)
	}

	// DepositsRevealBlocks comparison (*big.Int.Cmp)
	if len(expected.DepositsRevealBlocks) != len(actual.DepositsRevealBlocks) {
		t.Fatalf(
			"DepositsRevealBlocks length mismatch: expected %d, got %d",
			len(expected.DepositsRevealBlocks),
			len(actual.DepositsRevealBlocks),
		)
	}
	for i := range expected.DepositsRevealBlocks {
		expBlock := expected.DepositsRevealBlocks[i]
		actBlock := actual.DepositsRevealBlocks[i]
		if (expBlock == nil) != (actBlock == nil) {
			t.Fatalf(
				"DepositsRevealBlocks[%d] nilness mismatch: expected %v, got %v",
				i,
				expBlock,
				actBlock,
			)
		}
		if expBlock != nil && expBlock.Cmp(actBlock) != 0 {
			t.Fatalf(
				"DepositsRevealBlocks[%d] mismatch:\nexpected: %s\nactual:   %s",
				i,
				expBlock.String(),
				actBlock.String(),
			)
		}
	}
}
