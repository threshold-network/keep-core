package bitcoin

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/wire"
)

// TransactionSerializationFormat represents the Bitcoin transaction
// serialization format.
type TransactionSerializationFormat int

const (
	// Standard is the traditional transaction serialization format
	// [version][inputs][outputs][locktime].
	Standard TransactionSerializationFormat = iota

	// Witness is the witness transaction serialization format
	// [version][marker][flag][inputs][outputs][witness][locktime]
	// introduced by BIP-0141. For reference, see:
	// https://github.com/bitcoin/bips/blob/master/bip-0141.mediawiki#specification
	Witness
)

// MaxTransactionByteLength is the maximum byte length of a serialized
// transaction accepted by the deserialization routines. A consensus-valid
// transaction can never reach it: a transaction's weight is
// `3*base_size + total_size` and cannot exceed the maximum block weight of
// 4,000,000 weight units, so its serialized length is always strictly below
// that number.
// Rejecting longer input keeps deserialization defensive against grossly
// oversized, untrusted input, but this cap alone does NOT close the
// underlying btcd decoder's panic. btcd slices every script and witness
// item of a single transaction out of one shared, fixed-size 4,194,304-byte
// buffer, and readScriptBuf only checks a declared item's length varint
// against the flat 4,000,000-byte maxWitnessItemSize constant, never
// against that buffer's actual remaining capacity. Because a declared
// length does not need to be backed by that many delivered bytes for the
// panicking slice expression to be evaluated, a transaction whose real
// wire length is far below this cap can still reach the panic once earlier
// scripts have consumed enough of the shared buffer. The recover() in
// Deserialize below turns that slice-bounds panic into an error; it does
// NOT cover fatal runtime errors such as out-of-memory from attacker-
// declared varint counts that btcd's wire decoder honors before
// reading backing bytes.
const MaxTransactionByteLength = 4_000_000

// Transaction represents a Bitcoin transaction. For reference, see:
// https://developer.bitcoin.org/reference/transactions.html#raw-transaction-format
type Transaction struct {
	// Version is the transaction version number. Usually, version 1 or 2.
	Version int32
	// Inputs is a slice holding all inputs of the transaction.
	Inputs []*TransactionInput
	// Outputs is a slice holding all outputs of the transaction.
	Outputs []*TransactionOutput
	// Locktime is the transaction locktime being a Unix epoch time or
	// block number, interpreted according to the locktime parsing rules:
	// https://developer.bitcoin.org/devguide/transactions.html#locktime-and-sequence-number
	Locktime uint32
}

// Serialize serializes the transaction to a byte array using the specified
// serialization format. The actual result depends on the transaction type
// as described below.
//
// If the transaction CONTAINS witness inputs and Serialize is called with:
//   - Standard serialization format, the result is actually in the Standard
//     format and does not include witness data referring to the witness inputs
//   - Witness serialization format, the result is actually in the Witness
//     format and includes witness data referring to the witness inputs
//
// If the transaction DOES NOT CONTAIN witness inputs and Serialize is
// called with:
//   - Standard serialization format, the result is actually in the Standard
//     format
//   - Witness serialization format, the result is actually in the Standard
//     format because there are no witness inputs whose data can be included
//
// By default, the Witness format is used and that can be changed using the
// optional format argument. The Witness format is used by default as it
// fits more use cases.
func (t *Transaction) Serialize(
	format ...TransactionSerializationFormat,
) []byte {
	internal := newInternalTransaction()
	internal.fromTransaction(t)

	resolvedFormat := Witness

	if len(format) == 1 {
		resolvedFormat = format[0]
	}

	switch resolvedFormat {
	case Standard:
		buffer := bytes.NewBuffer(
			make([]byte, 0, internal.SerializeSizeStripped()),
		)
		err := internal.SerializeNoWitness(buffer)
		if err != nil {
			return nil
		}
		return buffer.Bytes()
	case Witness:
		buffer := bytes.NewBuffer(
			make([]byte, 0, internal.SerializeSize()),
		)
		err := internal.Serialize(buffer)
		if err != nil {
			return nil
		}
		return buffer.Bytes()
	default:
		panic("unknown transaction serialization format")
	}
}

// SerializeVersion serializes the transaction version to a little-endian
// 4-byte array.
func (t *Transaction) SerializeVersion() [4]byte {
	result := [4]byte{}
	binary.LittleEndian.PutUint32(result[:], uint32(t.Version))
	return result
}

// SerializeInputs serializes the transaction inputs to a byte array prepended
// with a CompactSizeUint denoting the total number of inputs.
func (t *Transaction) SerializeInputs() []byte {
	internal := newInternalTransaction()
	internal.fromTransaction(t)

	inputsByteSize := wire.VarIntSerializeSize(uint64(len(internal.TxIn)))
	for _, txIn := range internal.TxIn {
		inputsByteSize += txIn.SerializeSize()
	}

	// The first 4 bytes are version. The input vector starts at the 5th byte.
	startingByte := 4
	endingByte := startingByte + inputsByteSize

	return t.Serialize(Standard)[startingByte:endingByte]
}

// SerializeOutputs serializes the transaction outputs to a byte array prepended
// with a CompactSizeUint denoting the total number of outputs.
func (t *Transaction) SerializeOutputs() []byte {
	internal := newInternalTransaction()
	internal.fromTransaction(t)

	outputsByteSize := wire.VarIntSerializeSize(uint64(len(internal.TxOut)))
	for _, txOut := range internal.TxOut {
		outputsByteSize += txOut.SerializeSize()
	}

	serializedTx := t.Serialize(Standard)

	// The last 4 bytes are locktime. The output vector ends just before it.
	endingByte := len(serializedTx) - 4
	startingByte := endingByte - outputsByteSize

	return serializedTx[startingByte:endingByte]
}

// SerializeLocktime serializes the transaction locktime to a little-endian
// 4-byte array.
func (t *Transaction) SerializeLocktime() [4]byte {
	result := [4]byte{}
	binary.LittleEndian.PutUint32(result[:], t.Locktime)
	return result
}

// Deserialize deserializes the given byte array to a Transaction.
func (t *Transaction) Deserialize(data []byte) (err error) {
	if len(data) > MaxTransactionByteLength {
		return fmt.Errorf(
			"transaction byte length [%v] exceeds the maximum of [%v]",
			len(data),
			MaxTransactionByteLength,
		)
	}

	// data ultimately originates from an untrusted source (e.g. an Electrum
	// server) and the underlying btcd decoder can panic on carefully
	// crafted input that stays well under MaxTransactionByteLength (see the
	// doc comment on that constant). The recover() below turns that
	// slice-bounds panic into an error; it does NOT cover fatal runtime
	// errors such as out-of-memory from attacker-declared varint counts
	// that btcd's wire decoder honors before reading backing bytes.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf(
				"recovered from a panic while deserializing a transaction: [%v]",
				r,
			)
		}
	}()

	internal := newInternalTransaction()
	err = internal.Deserialize(bytes.NewReader(data))
	if err != nil {
		return err
	}

	transaction := internal.toTransaction()

	t.Version = transaction.Version
	t.Inputs = transaction.Inputs
	t.Outputs = transaction.Outputs
	t.Locktime = transaction.Locktime

	return nil
}

// Hash calculates the transaction's hash as the double SHA-256 of the
// Standard serialization format. The outcome is equivalent to the txid field
// defined in the Bitcoin specification and is used as transaction identifier.
func (t *Transaction) Hash() Hash {
	return ComputeHash(t.Serialize(Standard))
}

// WitnessHash calculates the transaction's witness hash as the double SHA-256
// of the Witness serialization format. The outcome is equivalent to the
// wtxid field defined by BIP-0141. The outcome of WitnessHash is equivalent
// to the result of Hash for non-witness transactions, i.e. transaction which
// does not have witness inputs. For reference, see:
// https://github.com/bitcoin/bips/blob/master/bip-0141.mediawiki#transaction-id
func (t *Transaction) WitnessHash() Hash {
	return ComputeHash(t.Serialize(Witness))
}

// TransactionOutpoint represents a Bitcoin transaction outpoint.
// For reference, see:
// https://developer.bitcoin.org/reference/transactions.html#outpoint-the-specific-part-of-a-specific-output
type TransactionOutpoint struct {
	// TransactionHash is the hash of the transaction holding the output
	// to spend.
	TransactionHash Hash
	// OutputIndex is the zero-based index of the output to spend from the
	// specified transaction.
	OutputIndex uint32
}

// TransactionInput represents a Bitcoin transaction input. For reference, see:
// https://developer.bitcoin.org/reference/transactions.html#txin-a-transaction-input-non-coinbase
type TransactionInput struct {
	// Outpoint is the previous transaction outpoint being spent by this input.
	Outpoint *TransactionOutpoint
	// SignatureScript is a script-language script that satisfies the conditions
	// placed in the outpoint's public key script (see TransactionOutput.PublicKeyScript).
	// This slice MUST NOT start with the byte-length of the script encoded as
	// a CompactSizeUint as this is done during transaction serialization.
	// This field is not set (nil or empty) for SegWit transaction inputs.
	// That means it is mutually exclusive with the below Witness field.
	SignatureScript []byte
	// Witness holds the witness data for the given input. It should be interpreted
	// as a stack with one or many elements. Individual elements MUST NOT start
	// with the byte-length of the script encoded as a CompactSizeUint as this
	// is done during transaction serialization. This field is not set
	// (nil or empty) for non-SegWit transaction inputs. That means it is
	// mutually exclusive with the above SignatureScript field.
	Witness [][]byte
	// Sequence is the sequence number for this input. Default value
	// is 0xffffffff. For reference, see:
	// https://developer.bitcoin.org/devguide/transactions.html#locktime-and-sequence-number
	Sequence uint32
}

// TransactionOutput represents a Bitcoin transaction output. For reference, see:
// https://developer.bitcoin.org/reference/transactions.html#txout-a-transaction-output
type TransactionOutput struct {
	// Value denotes the number of satoshis to spend. Zero is a valid value.
	Value int64
	// PublicKeyScript defines the conditions that must be satisfied to spend
	// this output. As stated in the Script docstring, this field MUST NOT
	// start with the byte-length of the script encoded as CompactSizeUint as
	// this is done during transaction serialization.
	PublicKeyScript Script
}

// UnspentTransactionOutput represents an unspent output (UTXO) of a Bitcoin
// transaction.
type UnspentTransactionOutput struct {
	// Outpoint is the transaction outpoint this UTXO points to.
	Outpoint *TransactionOutpoint
	// Value denotes the number of unspent satoshis.
	Value int64
}

// TransactionMerkleProof holds information about the merkle branch to a
// confirmed transaction.
type TransactionMerkleProof struct {
	// BlockHeight is the height of the block the transaction was confirmed in.
	BlockHeight uint

	// MerkleNodes is a list of transaction hashes the current hash is paired
	// with, recursively, in order to trace up to obtain the merkle root of the
	// including block, deepest pairing first. Each hash is an unprefixed hex
	// string.
	MerkleNodes []string

	// Position is the 0-based index of the transaction's position in the block.
	Position uint
}
