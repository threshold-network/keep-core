package bitcoin

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TransactionBuilder is a component that is responsible for the whole
// transaction creation process. It assembles an unsigned transaction,
// prepares it for signing, and applies the given signatures in order to
// produce a full-fledged signed transaction that can be broadcast across
// the Bitcoin network. The builder IS NOT SAFE for concurrent use.
type TransactionBuilder struct {
	chain       Chain
	internal    *internalTransaction
	sigHashArgs []*inputSigHashArgs
	sigHashes   []*big.Int
	// prevOuts holds the locking script and value of the UTXO pointed by
	// each added input. The txscript sighash pre-computation consults it
	// to determine the witness version of the spent outputs.
	//
	// The field is intentionally typed as the concrete
	// *txscript.MultiPrevOutFetcher rather than the txscript.PrevOutputFetcher
	// interface because the completeness check in ComputeSignatureHashes
	// relies on MultiPrevOutFetcher.FetchPrevOutput returning nil for an
	// outpoint that was never registered; the sibling CannedPrevOutputFetcher
	// always returns a zero TxOut instead, which would silently turn the
	// check into a no-op.
	prevOuts *txscript.MultiPrevOutFetcher
}

// NewTransactionBuilder constructs a new TransactionBuilder instance.
func NewTransactionBuilder(chain Chain) *TransactionBuilder {
	return &TransactionBuilder{
		chain:       chain,
		internal:    newInternalTransaction(),
		sigHashArgs: make([]*inputSigHashArgs, 0),
		prevOuts:    txscript.NewMultiPrevOutFetcher(nil),
	}
}

// HasTaprootKeyPathInputs returns true if the builder has at least one P2TR
// input intended to be spent using the Taproot key path.
func (tb *TransactionBuilder) HasTaprootKeyPathInputs() bool {
	for _, sigHashArgs := range tb.sigHashArgs {
		if sigHashArgs.scriptType == P2TRScript {
			return true
		}
	}

	return false
}

// HasOnlyTaprootKeyPathInputs returns true if every input in the builder is a
// P2TR input intended to be spent using the Taproot key path.
func (tb *TransactionBuilder) HasOnlyTaprootKeyPathInputs() bool {
	if len(tb.sigHashArgs) == 0 {
		return false
	}

	for _, sigHashArgs := range tb.sigHashArgs {
		if sigHashArgs.scriptType != P2TRScript {
			return false
		}
	}

	return true
}

// AddPublicKeyHashInput adds an unsigned input pointing to a UTXO locked
// using a P2PKH or P2WPKH script.
//
// For backward compatibility with wallet-action construction that discovers
// the input script type from the chain, this method also accepts P2TR direct
// key-path inputs. New Taproot-specific code should prefer
// AddTaprootKeyPathInput to make that spend policy explicit.
func (tb *TransactionBuilder) AddPublicKeyHashInput(
	utxo *UnspentTransactionOutput,
) error {
	utxoScript, err := tb.getScript(utxo)
	if err != nil {
		return fmt.Errorf(
			"cannot get locking script for UTXO pointed "+
				"by the input: [%v]",
			err,
		)
	}

	scriptType := GetScriptType(utxoScript)
	isDirectKeySpendScript := scriptType == P2PKHScript ||
		scriptType == P2WPKHScript ||
		scriptType == P2TRScript
	if !isDirectKeySpendScript {
		return fmt.Errorf(
			"UTXO pointed by the input is not P2PKH/P2WPKH/P2TR",
		)
	}

	return tb.addDirectKeySpendInput(utxo, utxoScript, scriptType, nil)
}

// AddTaprootKeyPathInput adds an unsigned input pointing to a UTXO locked
// using a P2TR script and intended to be spent using the Taproot key path.
//
// The script's x-only key is treated as the final Taproot output key. The
// builder does not apply a BIP-341/BIP-86 tap tweak during signing; callers
// must ensure the FROST signer can produce signatures for the exact output key
// committed to by the scriptPubKey.
func (tb *TransactionBuilder) AddTaprootKeyPathInput(
	utxo *UnspentTransactionOutput,
) error {
	utxoScript, err := tb.getScript(utxo)
	if err != nil {
		return fmt.Errorf(
			"cannot get locking script for UTXO pointed "+
				"by the input: [%v]",
			err,
		)
	}

	scriptType := GetScriptType(utxoScript)
	if scriptType != P2TRScript {
		return fmt.Errorf(
			"UTXO pointed by the input is not P2TR",
		)
	}

	return tb.addDirectKeySpendInput(utxo, utxoScript, scriptType, nil)
}

// AddTaprootKeyPathInputWithMerkleRoot adds an unsigned input pointing to a
// UTXO locked using a BIP-341 tweaked P2TR output key and intended to be spent
// using the Taproot key path.
//
// The provided internal key and script merkle root must derive the output key
// committed to by the UTXO script. The merkle root is retained as signing
// metadata so the FROST signer can produce a key-path signature under the same
// Taproot tweak.
func (tb *TransactionBuilder) AddTaprootKeyPathInputWithMerkleRoot(
	utxo *UnspentTransactionOutput,
	internalKey [32]byte,
	merkleRoot [32]byte,
) error {
	utxoScript, err := tb.getScript(utxo)
	if err != nil {
		return fmt.Errorf(
			"cannot get locking script for UTXO pointed "+
				"by the input: [%v]",
			err,
		)
	}

	scriptType := GetScriptType(utxoScript)
	if scriptType != P2TRScript {
		return fmt.Errorf(
			"UTXO pointed by the input is not P2TR",
		)
	}

	outputKey, err := ExtractTaprootKey(utxoScript)
	if err != nil {
		return fmt.Errorf("cannot extract taproot output key: [%v]", err)
	}

	expectedOutputKey, err := TaprootOutputKey(internalKey, &merkleRoot)
	if err != nil {
		return fmt.Errorf("cannot derive taproot output key: [%v]", err)
	}

	if !bytes.Equal(outputKey[:], expectedOutputKey[:]) {
		return fmt.Errorf(
			"taproot output key does not match internal key and merkle root",
		)
	}

	return tb.addDirectKeySpendInput(utxo, utxoScript, scriptType, &merkleRoot)
}

// TaprootKeyPathInputMerkleRoots returns per-input Taproot script merkle roots
// retained by the builder. The returned slice is aligned with transaction
// inputs. Non-Taproot inputs and untweaked Taproot inputs have nil entries.
func (tb *TransactionBuilder) TaprootKeyPathInputMerkleRoots() []*[32]byte {
	merkleRoots := make([]*[32]byte, len(tb.sigHashArgs))

	for i, sigHashArgs := range tb.sigHashArgs {
		if sigHashArgs.taprootMerkleRoot == nil {
			continue
		}

		merkleRoots[i] = new([32]byte)
		copy(merkleRoots[i][:], sigHashArgs.taprootMerkleRoot[:])
	}

	return merkleRoots
}

// assertUniformTaprootShape rejects transaction shapes that mix P2TR inputs
// with non-P2TR inputs. Such a mix is not supported by the signing paths:
// AddSignatures refuses builders holding any P2TR input and
// AddTaprootKeyPathSignatures requires all inputs to be P2TR. Catching the mix
// at construction time avoids discovering it only after a completed
// distributed signing round.
func (tb *TransactionBuilder) assertUniformTaprootShape(
	scriptType ScriptType,
) error {
	isTaproot := scriptType == P2TRScript

	for i, existing := range tb.sigHashArgs {
		if (existing.scriptType == P2TRScript) != isTaproot {
			return fmt.Errorf(
				"cannot add [%v] input: mixed P2TR and non-P2TR inputs are "+
					"not supported; existing input [%d] is [%v]",
				scriptType,
				i,
				existing.scriptType,
			)
		}
	}

	return nil
}

func (tb *TransactionBuilder) addDirectKeySpendInput(
	utxo *UnspentTransactionOutput,
	utxoScript Script,
	scriptType ScriptType,
	taprootMerkleRoot *[32]byte,
) error {
	if err := tb.assertUniformTaprootShape(scriptType); err != nil {
		return err
	}

	// The UTXO was locked using a direct key-spend script, so the scriptCode
	// required to build the sighash is equivalent to that script. Worth noting
	// that the P2WPKH script is actually converted to the P2PKH script when
	// used as a scriptCode, according to BIP-0143. For reference see,
	// https://github.com/bitcoin/bips/blob/master/bip-0143.mediawiki#specification.
	// That conversion is handled within the `txscript.CalcWitnessSigHash` call.
	sigHashArgs := &inputSigHashArgs{
		value:             utxo.Value,
		publicKeyScript:   utxoScript,
		scriptCode:        utxoScript,
		scriptType:        scriptType,
		taprootMerkleRoot: taprootMerkleRoot,
		witness:           scriptType == P2WPKHScript || scriptType == P2TRScript,
	}

	hash := chainhash.Hash(utxo.Outpoint.TransactionHash)
	outpoint := wire.NewOutPoint(&hash, utxo.Outpoint.OutputIndex)
	tb.prevOuts.AddPrevOut(*outpoint, wire.NewTxOut(utxo.Value, utxoScript))

	// Deliberately set both `signatureScript` and `witness` arguments to nil
	// because at this point, the input does not contain any signature data.
	tb.internal.AddTxIn(wire.NewTxIn(outpoint, nil, nil))

	tb.sigHashArgs = append(tb.sigHashArgs, sigHashArgs)
	// Adding an input invalidates any previously computed signature hashes.
	tb.sigHashes = nil

	return nil
}

// AddScriptHashInput adds an unsigned input pointing to a UTXO locked
// using a P2SH or P2WSH script. This function also requires the plain-text
// redeemScript whose hash was used to build the P2SH/P2WSH locking script.
func (tb *TransactionBuilder) AddScriptHashInput(
	utxo *UnspentTransactionOutput,
	redeemScript Script,
) error {
	utxoScript, err := tb.getScript(utxo)
	if err != nil {
		return fmt.Errorf(
			"cannot get locking script for UTXO pointed "+
				"by the input: [%v]",
			err,
		)
	}

	scriptType := GetScriptType(utxoScript)
	isScriptHashScript := scriptType == P2SHScript ||
		scriptType == P2WSHScript
	if !isScriptHashScript {
		return fmt.Errorf(
			"UTXO pointed by the input is not P2SH/P2WSH",
		)
	}

	if err := tb.assertUniformTaprootShape(scriptType); err != nil {
		return err
	}

	// The UTXO was locked using a P2SH/P2WSH script so, the scriptCode required
	// to build the sighash is equivalent to the plain-text redeem script whose
	// hash is included in the P2SH/P2WSH script.
	sigHashArgs := &inputSigHashArgs{
		value:           utxo.Value,
		publicKeyScript: utxoScript,
		scriptCode:      redeemScript,
		scriptType:      scriptType,
		witness:         scriptType == P2WSHScript,
	}

	hash := chainhash.Hash(utxo.Outpoint.TransactionHash)
	outpoint := wire.NewOutPoint(&hash, utxo.Outpoint.OutputIndex)
	tb.prevOuts.AddPrevOut(*outpoint, wire.NewTxOut(utxo.Value, utxoScript))

	// Signature data required to unlock a P2SH/P2WSH UTXO needs the plain-text
	// redeem script to be placed as the last item of the `witness` field for
	// P2WSH or the `signatureScript` field for P2SH. Here we prepare to fulfill
	// that requirement by putting the redeem script to the correct field and
	// let the AddSignatures method prepend it with the actual signature
	// and public key.
	if sigHashArgs.witness {
		tb.internal.AddTxIn(wire.NewTxIn(outpoint, nil, [][]byte{redeemScript}))
	} else {
		tb.internal.AddTxIn(wire.NewTxIn(outpoint, redeemScript, nil))
	}

	tb.sigHashArgs = append(tb.sigHashArgs, sigHashArgs)
	// Adding an input invalidates any previously computed signature hashes.
	tb.sigHashes = nil

	return nil
}

// getScript gets the locking script (PublicKeyScript) for the given unspent
// transaction output.
func (tb *TransactionBuilder) getScript(
	utxo *UnspentTransactionOutput,
) (Script, error) {
	hash := utxo.Outpoint.TransactionHash
	transaction, err := tb.chain.GetTransaction(hash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get transaction with hash [%s]: [%v]",
			hash.Hex(InternalByteOrder),
			err,
		)
	}

	outputIndex := utxo.Outpoint.OutputIndex
	if outputIndex >= uint32(len(transaction.Outputs)) {
		return nil, fmt.Errorf(
			"output index [%d] out of range for transaction [%s] "+
				"with [%d] outputs",
			outputIndex,
			hash.Hex(InternalByteOrder),
			len(transaction.Outputs),
		)
	}

	return transaction.Outputs[outputIndex].PublicKeyScript, nil
}

// AddOutput adds a new transaction's output.
//
// Adding an output after ComputeSignatureHashes invalidates the builder's
// cached signature hashes; ComputeSignatureHashes must be called again before
// signing.
func (tb *TransactionBuilder) AddOutput(output *TransactionOutput) {
	tb.internal.AddTxOut(wire.NewTxOut(output.Value, output.PublicKeyScript))
	tb.sigHashes = nil
}

// ComputeSignatureHashes computes the signature hashes for all transaction
// inputs and stores them into the builder's state. Elements of the returned
// slice are ordered in the same way as the transaction inputs they correspond
// to. That is, an element at the given index matches the input with the same
// index.
func (tb *TransactionBuilder) ComputeSignatureHashes() ([]*big.Int, error) {
	sigHashes := make([]*big.Int, len(tb.internal.TxIn))

	// Calculation of sighashes for witness inputs can be faster as common
	// sighash fragments can be pre-computed upfront and reused. The previous
	// outputs of all added inputs must be provided so the pre-computation can
	// determine the witness version of the spent outputs. A missing entry
	// makes the pre-computation panic (except for coinbase-shaped inputs, which
	// the pre-computation skips, and which the two Add* methods never produce),
	// so make sure the builder's state is consistent before handing it over.
	for i, input := range tb.internal.TxIn {
		if tb.prevOuts.FetchPrevOutput(input.PreviousOutPoint) == nil {
			return nil, fmt.Errorf(
				"missing previous output for input [%v]",
				i,
			)
		}
	}

	witnessSigHashFragments := txscript.NewTxSigHashes(
		tb.internal.MsgTx,
		tb.prevOuts,
	)

	for i := range tb.internal.TxIn {
		sigHashArgs := tb.sigHashArgs[i]

		var sigHashBytes []byte
		var err error

		switch sigHashArgs.scriptType {
		case P2TRScript:
			sigHashBytes, err = txscript.CalcTaprootSignatureHash(
				witnessSigHashFragments,
				txscript.SigHashDefault,
				tb.internal.MsgTx,
				i,
				tb.prevOuts,
			)
		case P2WPKHScript, P2WSHScript:
			sigHashBytes, err = txscript.CalcWitnessSigHash(
				sigHashArgs.scriptCode,
				witnessSigHashFragments,
				txscript.SigHashAll,
				tb.internal.MsgTx,
				i,
				sigHashArgs.value,
			)
		default:
			sigHashBytes, err = txscript.CalcSignatureHash(
				sigHashArgs.scriptCode,
				txscript.SigHashAll,
				tb.internal.MsgTx,
				i,
			)
		}

		if err != nil {
			return nil, fmt.Errorf(
				"cannot calculate sighash for input [%v]: [%v]",
				i,
				err,
			)
		}

		sigHashes[i] = new(big.Int).SetBytes(sigHashBytes)
	}

	tb.sigHashes = sigHashes

	return sigHashes, nil
}

// SignatureContainer is a helper type holding signature data.
type SignatureContainer struct {
	R, S      *big.Int
	PublicKey *ecdsa.PublicKey
}

// AddSignatures adds signature data for transaction inputs and returns a
// signed Transaction instance. The signatures slice should have the same
// length as the transaction's input vector. The signature with given index
// should correspond to the input with the same index. Each signature
// should also contain a public key that can be used for verification, i.e.
// this should be the public key that corresponds to the private key used
// to produce the given signature. Each signature is verified and an error
// is produced if any signature is not valid for their input.
func (tb *TransactionBuilder) AddSignatures(
	signatures []*SignatureContainer,
) (*Transaction, error) {
	if len(tb.sigHashes) == 0 {
		return nil, fmt.Errorf("signature hashes must be computed first")
	}

	if len(signatures) != len(tb.internal.TxIn) {
		return nil, fmt.Errorf("wrong signatures count")
	}

	// Hoist the P2TR guard above the per-input loop so that, for a mixed
	// transaction, the loop does not mutate Witness/SignatureScript on
	// preceding inputs before erroring out. The per-input guard below is
	// kept as a defense-in-depth check.
	if tb.HasTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"transaction has P2TR inputs; use AddTaprootKeyPathSignatures",
		)
	}
	for i, input := range tb.internal.TxIn {
		signature := signatures[i]
		sigHashArgs := tb.sigHashArgs[i]

		if sigHashArgs.scriptType == P2TRScript {
			return nil, fmt.Errorf(
				"input [%v] is P2TR; use AddTaprootKeyPathSignatures",
				i,
			)
		}

		// Make a sanity check to avoid producing crap transactions.
		if !ecdsa.Verify(
			signature.PublicKey,
			tb.sigHashes[i].FillBytes(make([]byte, sha256.Size)),
			signature.R,
			signature.S,
		) {
			return nil, fmt.Errorf("invalid signature for input [%v]", i)
		}

		signatureBytes := append(
			(&btcec.Signature{R: signature.R, S: signature.S}).Serialize(),
			byte(txscript.SigHashAll),
		)
		publicKeyBytes := (*btcec.PublicKey)(
			signature.PublicKey,
		).SerializeCompressed()

		if sigHashArgs.witness {
			witness := wire.TxWitness{
				signatureBytes,
				publicKeyBytes,
			}

			// If the Witness field was pre-filled with data, put them at
			// the end of the final witness field. This is the case for
			// P2WSH inputs.
			if len(input.Witness) == 1 {
				witness = append(witness, input.Witness[0])
			}

			input.Witness = witness
		} else {
			builder := txscript.NewScriptBuilder().
				AddData(signatureBytes).
				AddData(publicKeyBytes)

			// If the SignatureScript field was pre-filled with data, put them
			// at the end of the final SignatureScript field. This is the case
			// for P2SH inputs.
			if len(input.SignatureScript) > 0 {
				builder.AddData(input.SignatureScript)
			}

			script, err := builder.Script()
			if err != nil {
				return nil, fmt.Errorf(
					"cannot build signature script for input [%v]: [%v]",
					i,
					err,
				)
			}

			input.SignatureScript = script
		}
	}

	return tb.internal.toTransaction(), nil
}

// SchnorrSignatureContainer is a helper type holding a serialized 64-byte
// BIP-340 Schnorr signature.
type SchnorrSignatureContainer struct {
	Signature [64]byte
}

// AddTaprootKeyPathSignatures adds Schnorr signature data for P2TR key-path
// transaction inputs and returns a signed Transaction instance. Each signature
// is verified against the corresponding input's sighash and an error is
// produced if any signature is invalid.
func (tb *TransactionBuilder) AddTaprootKeyPathSignatures(
	signatures []*SchnorrSignatureContainer,
) (*Transaction, error) {
	if len(tb.sigHashes) == 0 {
		return nil, fmt.Errorf("signature hashes must be computed first")
	}

	if len(signatures) != len(tb.internal.TxIn) {
		return nil, fmt.Errorf("wrong signatures count")
	}

	if !tb.HasOnlyTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"taproot key-path signatures require all inputs to be P2TR",
		)
	}

	for i, input := range tb.internal.TxIn {
		signature := signatures[i]
		if signature == nil {
			return nil, fmt.Errorf("signature for input [%v] is nil", i)
		}

		signatureBytes := make([]byte, len(signature.Signature))
		copy(signatureBytes, signature.Signature[:])

		taprootKey, err := ExtractTaprootKey(tb.sigHashArgs[i].publicKeyScript)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot extract taproot key for input [%v]: [%v]",
				i,
				err,
			)
		}

		taprootPublicKey, err := schnorr.ParsePubKey(taprootKey[:])
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse taproot key for input [%v]: [%v]",
				i,
				err,
			)
		}

		taprootSignature, err := schnorr.ParseSignature(signatureBytes)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse taproot key-path signature for input [%v]: [%v]",
				i,
				err,
			)
		}

		sigHashBytes := tb.sigHashes[i].FillBytes(make([]byte, sha256.Size))
		if !taprootSignature.Verify(sigHashBytes, taprootPublicKey) {
			return nil, fmt.Errorf(
				"invalid taproot key-path signature for input [%v]",
				i,
			)
		}

		input.Witness = wire.TxWitness{signatureBytes}
	}

	return tb.internal.toTransaction(), nil
}

// TotalInputsValue returns the total value of transaction inputs.
func (tb *TransactionBuilder) TotalInputsValue() int64 {
	totalInputsValue := int64(0)

	for _, sigHashArgs := range tb.sigHashArgs {
		totalInputsValue += sigHashArgs.value
	}

	return totalInputsValue
}

// ReplaceUnsignedTransaction replaces the internal unsigned transaction while
// preserving per-input sighash metadata collected during builder input setup.
// It also validates that the replacement's inputs carry no pre-existing signature
// data, that each replacement input's PreviousOutPoint matches the prior input
// at the same index, that the TxOut set matches the prior builder state, and
// restores each input's pre-signing witness or signature-script from the
// previous builder state; replacement inputs whose previous witness had more
// than one element are rejected.
func (tb *TransactionBuilder) ReplaceUnsignedTransaction(
	transaction *Transaction,
) error {
	if transaction == nil {
		return fmt.Errorf("transaction is nil")
	}

	if len(transaction.Inputs) != len(tb.sigHashArgs) {
		return fmt.Errorf(
			"input metadata mismatch: [%d] tx inputs, [%d] sighash args",
			len(transaction.Inputs),
			len(tb.sigHashArgs),
		)
	}

	previousInputs := tb.internal.TxIn
	previousOutputs := append([]*wire.TxOut{}, tb.internal.TxOut...)

	replacedInternal := newInternalTransaction()
	replacedInternal.fromTransaction(transaction)

	// Bind the replacement to the builder's prior state: per-index
	// PreviousOutPoint must match and the TxOut set must match, always.
	// Without these checks, a caller-controlled replacement could redirect
	// funds (different TxOut set) or misalign sigHashArgs[i] with the new-order
	// tx.TxIn[i], producing a self-consistent-but-wrong digest that survives the
	// local Verify gate and broadcasts an unintended transaction.
	for i, prevIn := range previousInputs {
		replIn := replacedInternal.TxIn[i]
		if replIn.PreviousOutPoint != prevIn.PreviousOutPoint {
			return fmt.Errorf(
				"replacement input [%d] PreviousOutPoint differs from builder state",
				i,
			)
		}
	}
	if len(replacedInternal.TxOut) != len(previousOutputs) {
		return fmt.Errorf(
			"replacement TxOut set has [%d] entries; builder state has [%d]",
			len(replacedInternal.TxOut),
			len(previousOutputs),
		)
	}
	for i, prevOut := range previousOutputs {
		replOut := replacedInternal.TxOut[i]
		if replOut.Value != prevOut.Value {
			return fmt.Errorf(
				"replacement TxOut [%d] value [%d] differs from builder state [%d]",
				i,
				replOut.Value,
				prevOut.Value,
			)
		}
		if !bytes.Equal(replOut.PkScript, prevOut.PkScript) {
			return fmt.Errorf(
				"replacement TxOut [%d] PkScript differs from builder state",
				i,
			)
		}
	}

	for i := range replacedInternal.TxIn {
		previousInput := previousInputs[i]
		replacedInput := replacedInternal.TxIn[i]

		if previousInput == nil || replacedInput == nil {
			continue
		}

		if len(replacedInput.SignatureScript) > 0 {
			return fmt.Errorf(
				"replacement transaction input [%d] has unexpected non-empty signature script",
				i,
			)
		}

		if len(replacedInput.Witness) > 0 {
			return fmt.Errorf(
				"replacement transaction input [%d] has unexpected non-empty witness",
				i,
			)
		}

		// The replacement's SignatureScript and Witness are both empty here
		// because of the two refusals above, so the per-input restore below
		// only has to decide what to copy *from* the previous input.
		if tb.sigHashArgs[i].witness {
			// Witness inputs may carry a single-element pre-signing witness
			// that holds a P2WSH-style redeem script. Multi-element witnesses
			// belong to P2TR script-path spends or other workflows. Multi-
			// element witnesses are not supported by this restore path: refuse
			// rather than silently drop pre-signing witness data, which would
			// produce a malformed transaction.

			switch len(previousInput.Witness) {
			case 0:
				// Nothing to restore (typical P2TR key-path or P2WPKH).
			case 1:
				redeemScript := append([]byte{}, previousInput.Witness[0]...)
				replacedInput.Witness = wire.TxWitness{redeemScript}
			default:
				return fmt.Errorf(
					"replacement transaction input [%d] previous witness has "+
						"[%d] elements; only zero- or single-element "+
						"pre-signing witnesses are currently supported for "+
						"restoration",
					i,
					len(previousInput.Witness),
				)
			}
		} else if len(previousInput.SignatureScript) > 0 {
			replacedInput.SignatureScript = append(
				[]byte{},
				previousInput.SignatureScript...,
			)
		}
	}

	tb.internal = replacedInternal
	tb.sigHashes = nil

	return nil
}

// UnsignedTransaction returns the current unsigned transaction builder state.
func (tb *TransactionBuilder) UnsignedTransaction() *Transaction {
	return tb.internal.toTransaction()
}

// UnsignedTransactionInput carries canonical unsigned input metadata extracted
// from the builder state.
type UnsignedTransactionInput struct {
	TxIDHex         string
	Vout            uint32
	ValueSats       uint64
	ScriptPubKeyHex string
}

// UnsignedTransactionOutput carries canonical unsigned output metadata
// extracted from the builder state.
type UnsignedTransactionOutput struct {
	ScriptPubKeyHex string
	ValueSats       uint64
}

// UnsignedTransactionIO returns canonical unsigned transaction input/output
// metadata from the builder state.
func (tb *TransactionBuilder) UnsignedTransactionIO() (
	[]UnsignedTransactionInput,
	[]UnsignedTransactionOutput,
	error,
) {
	if len(tb.internal.TxIn) != len(tb.sigHashArgs) {
		return nil, nil, fmt.Errorf(
			"input metadata mismatch: [%d] tx inputs, [%d] sighash args",
			len(tb.internal.TxIn),
			len(tb.sigHashArgs),
		)
	}

	inputs := make([]UnsignedTransactionInput, 0, len(tb.internal.TxIn))
	for i, input := range tb.internal.TxIn {
		value := tb.sigHashArgs[i].value
		if value < 0 {
			return nil, nil, fmt.Errorf("input [%d] value is negative", i)
		}

		inputs = append(
			inputs,
			UnsignedTransactionInput{
				// chainhash.Hash.String renders txid in standard Bitcoin display
				// (RPC/explorer) byte order, i.e. reversed vs internal bytes.
				TxIDHex:   input.PreviousOutPoint.Hash.String(),
				Vout:      input.PreviousOutPoint.Index,
				ValueSats: uint64(value),
				ScriptPubKeyHex: hex.EncodeToString(
					tb.sigHashArgs[i].publicKeyScript,
				),
			},
		)
	}

	outputs := make([]UnsignedTransactionOutput, 0, len(tb.internal.TxOut))
	for i, output := range tb.internal.TxOut {
		if output.Value < 0 {
			return nil, nil, fmt.Errorf("output [%d] value is negative", i)
		}

		outputs = append(
			outputs,
			UnsignedTransactionOutput{
				ScriptPubKeyHex: hex.EncodeToString(output.PkScript),
				ValueSats:       uint64(output.Value),
			},
		)
	}

	return inputs, outputs, nil
}

// inputSigHashArgs is a helper structure holding some arguments required to
// compute a sighash for the given input.
type inputSigHashArgs struct {
	// value denotes the satoshi value of the UTXO pointed by the given input.
	value int64
	// publicKeyScript is the locking script of the UTXO pointed by the given
	// input.
	publicKeyScript []byte
	// scriptCode is a component of the input's sighash and is the script that
	// is actually executed while unlocking the given UTXO. The scriptCode
	// depends on the script type that was used to lock the given UTXO.
	scriptCode []byte
	// scriptType denotes the locking script type of the UTXO pointed by the
	// given input.
	scriptType ScriptType
	// taprootMerkleRoot denotes the BIP-341 script merkle root used to tweak
	// the P2TR input's output key. It is nil for untweaked P2TR inputs and
	// non-Taproot inputs.
	taprootMerkleRoot *[32]byte
	// witness denotes whether the given input point's to a UTXO locked using
	// a witness script.
	witness bool
}

// internalTransaction is an internal utility representation of the Transaction
// that expose a lot of tools helpful during transaction manipulation.
type internalTransaction struct {
	*wire.MsgTx
}

func newInternalTransaction() *internalTransaction {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.LockTime = 0

	return &internalTransaction{msgTx}
}

func (it *internalTransaction) fromTransaction(transaction *Transaction) {
	it.Version = transaction.Version

	it.TxIn = make([]*wire.TxIn, len(transaction.Inputs))
	for i, input := range transaction.Inputs {
		it.TxIn[i] = &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash(input.Outpoint.TransactionHash),
				Index: input.Outpoint.OutputIndex,
			},
			SignatureScript: input.SignatureScript,
			Witness:         input.Witness,
			Sequence:        input.Sequence,
		}
	}

	it.TxOut = make([]*wire.TxOut, len(transaction.Outputs))
	for i, output := range transaction.Outputs {
		it.TxOut[i] = &wire.TxOut{
			Value:    output.Value,
			PkScript: output.PublicKeyScript,
		}
	}

	it.LockTime = transaction.Locktime
}

func (it *internalTransaction) toTransaction() *Transaction {
	inputs := make([]*TransactionInput, len(it.TxIn))
	for i, input := range it.TxIn {
		inputs[i] = &TransactionInput{
			Outpoint: &TransactionOutpoint{
				TransactionHash: Hash(input.PreviousOutPoint.Hash),
				OutputIndex:     input.PreviousOutPoint.Index,
			},
			SignatureScript: input.SignatureScript,
			Witness:         input.Witness,
			Sequence:        input.Sequence,
		}
	}

	outputs := make([]*TransactionOutput, len(it.TxOut))
	for i, output := range it.TxOut {
		outputs[i] = &TransactionOutput{
			Value:           output.Value,
			PublicKeyScript: output.PkScript,
		}
	}

	return &Transaction{
		Version:  it.Version,
		Inputs:   inputs,
		Outputs:  outputs,
		Locktime: it.LockTime,
	}
}
