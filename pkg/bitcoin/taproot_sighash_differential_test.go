package bitcoin

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/btcsuite/btcd/txscript"
)

// fillRandom fills b with pseudo-random bytes from rng. math/rand/v2 drops the
// Read method that math/rand exposed on *Rand, and these bytes only need to vary
// across iterations, never to be unpredictable.
func fillRandom(rng *rand.Rand, b []byte) {
	var block [8]byte
	for offset := 0; offset < len(b); offset += len(block) {
		binary.LittleEndian.PutUint64(block[:], rng.Uint64())
		copy(b[offset:], block[:])
	}
}

// TestTaprootKeyPathSigHashMatchesTxscript is a differential test of the
// hand-rolled BIP-341 key-path signature hash against btcd's
// txscript.CalcTaprootSignatureHash over randomized multi-input transactions.
//
// calcTaprootKeyPathSignatureHash assembles the SigMsg preimage by hand -- epoch,
// hash type, version, locktime, the five midstate hashes, spend type and input
// index -- and a single mismatched byte or a misordered field produces a digest
// that signs a transaction the wallet did not intend. That is the fund-loss class,
// and a hardcoded vector cannot catch it because the vector fixes exactly one
// input count, one ordering and one output shape.
//
// The randomization deliberately varies what the midstate commits to: input count
// and ordering, per-input values, sequences, script types (so P2TR inputs sit at
// arbitrary indices in a mixed transaction), output count and values, version and
// locktime. Every P2TR input of every generated transaction must agree with
// txscript byte for byte.
//
// The seed is fixed so failures reproduce exactly.
func TestTaprootKeyPathSigHashMatchesTxscript(t *testing.T) {
	const (
		iterations   = 200
		maxInputs    = 5
		maxOutputs   = 4
		fundingValue = 1_000_000
	)

	rng := rand.New(rand.NewPCG(20260813, 0x9E3779B97F4A7C15))

	for iteration := 0; iteration < iterations; iteration++ {
		localChain := newLocalChain()
		builder := NewTransactionBuilder(localChain)

		inputCount := 1 + rng.IntN(maxInputs)
		taprootInputs := make([]int, 0, inputCount)

		for i := 0; i < inputCount; i++ {
			// Mixed script types put P2TR inputs at arbitrary indices. The midstate
			// commits to every input's amount and script regardless of type, so a
			// non-Taproot input at a lower index must still be folded in correctly.
			isTaproot := rng.IntN(2) == 0 || (i == inputCount-1 && len(taprootInputs) == 0)

			var (
				lockingScript Script
				err           error
			)
			if isTaproot {
				var outputKey [32]byte
				fillRandom(rng, outputKey[:])
				lockingScript, err = PayToTaproot(outputKey)
			} else {
				var publicKeyHash [20]byte
				fillRandom(rng, publicKeyHash[:])
				lockingScript, err = PayToWitnessPublicKeyHash(publicKeyHash)
			}
			if err != nil {
				t.Fatalf("iteration %d input %d: %v", iteration, i, err)
			}

			// Vary the value per input: the midstate's hashInputAmounts commits to
			// all of them in input order.
			value := int64(1 + rng.IntN(fundingValue))

			var previousHash Hash
			previousHash[0] = byte(iteration)
			previousHash[1] = byte(i)

			fundingTransaction := &Transaction{
				Version: 1,
				Inputs: []*TransactionInput{
					{
						Outpoint: &TransactionOutpoint{
							TransactionHash: previousHash,
							OutputIndex:     0,
						},
						SignatureScript: []byte{0x51},
						Sequence:        0xffffffff,
					},
				},
				Outputs: []*TransactionOutput{
					{
						Value:           value,
						PublicKeyScript: lockingScript,
					},
				},
				Locktime: 0,
			}
			if err := localChain.addTransaction(fundingTransaction); err != nil {
				t.Fatalf("iteration %d input %d: %v", iteration, i, err)
			}

			utxo := &UnspentTransactionOutput{
				Outpoint: &TransactionOutpoint{
					TransactionHash: fundingTransaction.Hash(),
					OutputIndex:     0,
				},
				Value: value,
			}

			if isTaproot {
				if err := builder.AddTaprootKeyPathInput(utxo); err != nil {
					t.Fatalf("iteration %d input %d: %v", iteration, i, err)
				}
				taprootInputs = append(taprootInputs, i)
			} else {
				if err := builder.AddPublicKeyHashInput(utxo); err != nil {
					t.Fatalf("iteration %d input %d: %v", iteration, i, err)
				}
			}
		}

		// Vary the outputs: hashOutputs commits to every value and script.
		outputCount := 1 + rng.IntN(maxOutputs)
		for i := 0; i < outputCount; i++ {
			var publicKeyHash [20]byte
			fillRandom(rng, publicKeyHash[:])
			outputScript, err := PayToWitnessPublicKeyHash(publicKeyHash)
			if err != nil {
				t.Fatalf("iteration %d output %d: %v", iteration, i, err)
			}
			builder.AddOutput(&TransactionOutput{
				Value:           int64(1 + rng.IntN(fundingValue/2)),
				PublicKeyScript: outputScript,
			})
		}

		// Version and locktime are both in the preimage, ahead of the midstate
		// hashes, so a field-order error shows up as soon as they are not their
		// default values.
		builder.internal.Version = int32(1 + rng.IntN(2))
		builder.internal.LockTime = uint32(rng.IntN(500_000))

		sigHashes, err := builder.ComputeSignatureHashes()
		if err != nil {
			t.Fatalf("iteration %d: compute sighashes: %v", iteration, err)
		}

		// The reference implementation, given the identical transaction and the
		// identical previous outputs.
		reference := txscript.NewTxSigHashes(builder.internal.MsgTx, builder.prevOuts)

		for _, inputIndex := range taprootInputs {
			expected, err := txscript.CalcTaprootSignatureHash(
				reference,
				txscript.SigHashDefault,
				builder.internal.MsgTx,
				inputIndex,
				builder.prevOuts,
			)
			if err != nil {
				t.Fatalf(
					"iteration %d input %d: txscript reference: %v",
					iteration,
					inputIndex,
					err,
				)
			}

			// ComputeSignatureHashes returns big.Int, which drops leading zero
			// bytes; FillBytes restores the fixed 32-byte digest.
			actual := sigHashes[inputIndex].FillBytes(make([]byte, 32))

			if !bytes.Equal(expected, actual) {
				t.Fatalf(
					"iteration %d input %d of %d (taproot inputs %v, outputs %d, "+
						"version %d, locktime %d):\n"+
						"txscript: %x\n"+
						"builder:  %x",
					iteration,
					inputIndex,
					inputCount,
					taprootInputs,
					outputCount,
					builder.internal.Version,
					builder.internal.LockTime,
					expected,
					actual,
				)
			}
		}
	}
}
