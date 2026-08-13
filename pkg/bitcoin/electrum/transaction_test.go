package electrum

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// TestDecodeTransactionDeclaredNotDelivered mirrors
// bitcoin.TestTransaction_DeserializeDeclaredNotDelivered: it proves
// decodeTransaction cannot be crashed by a transaction whose declared (but
// undelivered) script length exceeds the underlying btcd decoder's shared
// script buffer's actual remaining capacity, even though the transaction's
// total wire length is a small fraction of bitcoin.MaxTransactionByteLength.
// See the doc comment on that constant for the full explanation of the
// underlying btcd behavior this guards against.
func TestDecodeTransactionDeclaredNotDelivered(t *testing.T) {
	var data []byte

	appendVarInt := func(value uint64) {
		switch {
		case value < 0xfd:
			data = append(data, byte(value))
		case value <= 0xffff:
			data = append(data, 0xfd)
			data = binary.LittleEndian.AppendUint16(data, uint16(value))
		case value <= 0xffffffff:
			data = append(data, 0xfe)
			data = binary.LittleEndian.AppendUint32(data, uint32(value))
		default:
			data = append(data, 0xff)
			data = binary.LittleEndian.AppendUint64(data, value)
		}
	}

	const (
		// scriptSlabSize is btcd's shared, fixed-size script buffer
		// (1 << 22 bytes) that every script and witness item of a single
		// transaction is sliced out of.
		scriptSlabSize = 4_194_304

		// maxDeclaredScriptSize is btcd's flat per-item cap
		// (maxWitnessItemSize), checked against the declared length
		// instead of the slab's actual remaining capacity.
		maxDeclaredScriptSize = 4_000_000

		// realScriptSize is the minimum number of actually delivered
		// script bytes needed to shrink the slab's remaining capacity to
		// one byte below maxDeclaredScriptSize, so that later declaring
		// (without delivering) a maxDeclaredScriptSize item panics.
		realScriptSize = scriptSlabSize - maxDeclaredScriptSize + 1
	)

	data = binary.LittleEndian.AppendUint32(data, 1) // version
	appendVarInt(1)                                  // input count
	data = append(data, make([]byte, 32)...)         // previous transaction hash
	data = binary.LittleEndian.AppendUint32(data, 0) // previous output index
	appendVarInt(0)                                  // empty signature script
	data = binary.LittleEndian.AppendUint32(data, 0xffffffff)

	appendVarInt(2) // output count

	// Output 1: really deliver enough script bytes to shrink the shared
	// buffer's remaining capacity below maxDeclaredScriptSize.
	data = binary.LittleEndian.AppendUint64(data, 1000) // value
	appendVarInt(realScriptSize)
	data = append(data, make([]byte, realScriptSize)...)

	// Output 2: declare a script of the maximum allowed size, but do not
	// deliver any of its bytes - the slice expression that would panic is
	// evaluated as an argument to io.ReadFull before any read is attempted,
	// so no witness/script bytes need to actually follow.
	data = binary.LittleEndian.AppendUint64(data, 1000) // value
	appendVarInt(maxDeclaredScriptSize)
	rawTx := hex.EncodeToString(data)

	// rawTx is two hex characters per byte, so the decoded length is at
	// most len(rawTx)/2. The sibling test guards the same invariant
	// before encoding; doing it after the encode here matches what
	// decodeTransaction actually sees and protects against a future cap
	// change silently misdirecting this test into the wrong code path.
	if len(rawTx)/2 >= bitcoin.MaxTransactionByteLength {
		t.Fatalf(
			"test transaction of [%v] bytes does not stay under the "+
				"maximum of [%v] bytes",
			len(rawTx)/2,
			bitcoin.MaxTransactionByteLength,
		)
	}

	_, err := decodeTransaction(rawTx)
	if err == nil {
		t.Fatal("expected a non-nil error; decoding should not have succeeded")
	}

	// Invariant: the process survives and an error is returned. The
	// specific "recovered from a panic" message is an implementation
	// detail that depends on today's btcd readScriptBuf bound; a
	// future btcd version that returns a plain EOF here would still
	// satisfy the invariant.
}
