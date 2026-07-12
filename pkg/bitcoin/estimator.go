package bitcoin

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// About half of all signatures generated with a random nonce are 72-byte, about
// half are one byte smaller, and a small percentage are smaller than that.
// For fee estimation purposes, we take the greatest possible value.
var signaturePlaceholder = make([]byte, 72)

// Compressed public keys have always 33 bytes.
var publicKeyPlaceholder = make([]byte, 33)

// Taproot key-path signatures use the 64-byte BIP-340 Schnorr encoding when
// SIGHASH_DEFAULT is used.
var taprootKeyPathSignaturePlaceholder = make([]byte, 64)

// TransactionSizeEstimator is a component allowing to estimate the size
// of a Bitcoin transaction of the provided shape, without constructing it.
type TransactionSizeEstimator struct {
	internal *internalTransaction
	err      error
}

func NewTransactionSizeEstimator() *TransactionSizeEstimator {
	return &TransactionSizeEstimator{
		internal: newInternalTransaction(),
	}
}

// AddPublicKeyHashInputs adds the provided count of P2WPKH (isWitness is true)
// or P2PKH (isWitness is false) inputs to the estimation. If the estimator
// already errored out during previous actions, this method does nothing.
func (tse *TransactionSizeEstimator) AddPublicKeyHashInputs(
	count int,
	isWitness bool,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	var signatureScript []byte
	var witness wire.TxWitness

	if isWitness {
		witness = wire.TxWitness{
			signaturePlaceholder,
			publicKeyPlaceholder,
		}
	} else {
		var err error

		signatureScript, err = txscript.NewScriptBuilder().
			AddData(signaturePlaceholder).
			AddData(publicKeyPlaceholder).
			Script()
		if err != nil {
			tse.err = err
			return tse
		}
	}

	for i := 0; i < count; i++ {
		tse.internal.AddTxIn(
			wire.NewTxIn(
				wire.NewOutPoint((*chainhash.Hash)(&[32]byte{}), 0),
				signatureScript,
				witness,
			),
		)
	}

	return tse
}

// AddPublicKeyScriptInput adds an input spending an output locked by the
// provided direct-key public key script. P2PKH, P2WPKH, and P2TR key-path
// spends are supported. If the estimator already errored out during previous
// actions, this method does nothing.
func (tse *TransactionSizeEstimator) AddPublicKeyScriptInput(
	publicKeyScript Script,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	switch GetScriptType(publicKeyScript) {
	case P2PKHScript:
		return tse.AddPublicKeyHashInputs(1, false)
	case P2WPKHScript:
		return tse.AddPublicKeyHashInputs(1, true)
	case P2TRScript:
		tse.internal.AddTxIn(
			wire.NewTxIn(
				wire.NewOutPoint((*chainhash.Hash)(&[32]byte{}), 0),
				nil,
				wire.TxWitness{taprootKeyPathSignaturePlaceholder},
			),
		)
	default:
		tse.err = fmt.Errorf(
			"unsupported direct-key input script type: [%v]",
			GetScriptType(publicKeyScript),
		)
	}

	return tse
}

// AddScriptHashInputs adds the provided count of P2WSH (isWitness is true)
// or P2SH (isWitness is false) inputs to the estimation. The redeemScriptLength
// argument should be used to pass the byte size of the plain-text redeem
// script used to lock the inputs. If the estimator already errored out during
// previous actions, this method does nothing.
func (tse *TransactionSizeEstimator) AddScriptHashInputs(
	count int,
	redeemScriptLength int,
	isWitness bool,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	var signatureScript []byte
	var witness wire.TxWitness

	redeemScriptPlaceholder := make([]byte, redeemScriptLength)

	if isWitness {
		witness = wire.TxWitness{
			signaturePlaceholder,
			publicKeyPlaceholder,
			redeemScriptPlaceholder,
		}
	} else {
		var err error

		signatureScript, err = txscript.NewScriptBuilder().
			AddData(signaturePlaceholder).
			AddData(publicKeyPlaceholder).
			AddData(redeemScriptPlaceholder).
			Script()
		if err != nil {
			tse.err = err
			return tse
		}
	}

	for i := 0; i < count; i++ {
		tse.internal.AddTxIn(
			wire.NewTxIn(
				wire.NewOutPoint((*chainhash.Hash)(&[32]byte{}), 0),
				signatureScript,
				witness,
			),
		)
	}

	return tse
}

// AddPublicKeyHashOutputs adds the provided count of P2WPKH (isWitness is true)
// or P2PKH (isWitness is false) outputs to the estimation. If the estimator
// already errored out during previous actions, this method does nothing.
func (tse *TransactionSizeEstimator) AddPublicKeyHashOutputs(
	count int,
	isWitness bool,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	var scriptPlaceholder []byte
	var err error

	if isWitness {
		scriptPlaceholder, err = PayToWitnessPublicKeyHash([20]byte{})
		if err != nil {
			tse.err = err
			return tse
		}
	} else {
		scriptPlaceholder, err = PayToPublicKeyHash([20]byte{})
		if err != nil {
			tse.err = err
			return tse
		}
	}

	for i := 0; i < count; i++ {
		tse.internal.AddTxOut(
			wire.NewTxOut(0, scriptPlaceholder),
		)
	}

	return tse
}

// AddScriptHashOutputs adds the provided count of P2WSH (isWitness is true)
// or P2SH (isWitness is false) outputs to the estimation. If the estimator
// already errored out during previous actions, this method does nothing.
func (tse *TransactionSizeEstimator) AddScriptHashOutputs(
	count int,
	isWitness bool,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	var scriptPlaceholder []byte
	var err error

	if isWitness {
		scriptPlaceholder, err = PayToWitnessScriptHash([32]byte{})
		if err != nil {
			tse.err = err
			return tse
		}
	} else {
		scriptPlaceholder, err = PayToScriptHash([20]byte{})
		if err != nil {
			tse.err = err
			return tse
		}
	}

	for i := 0; i < count; i++ {
		tse.internal.AddTxOut(
			wire.NewTxOut(0, scriptPlaceholder),
		)
	}

	return tse
}

// AddOutputScript adds an output locked by the provided public key script. If
// the estimator already errored out during previous actions, this method does
// nothing.
func (tse *TransactionSizeEstimator) AddOutputScript(
	publicKeyScript Script,
) *TransactionSizeEstimator {
	if tse.err != nil {
		return tse
	}

	if len(publicKeyScript) == 0 {
		tse.err = fmt.Errorf("output public key script is empty")
		return tse
	}

	tse.internal.AddTxOut(
		wire.NewTxOut(0, append([]byte(nil), publicKeyScript...)),
	)

	return tse
}

// VirtualSize returns the virtual size of the transaction whose shape was
// provided to the estimator. If any errors occurred while building the
// transaction shape, the first error will be returned.
//
// Worth noting that the estimator can slightly overshoot the virtual size
// estimation as it always assumes the greatest possible byte size of
// signatures (72 bytes, see signaturePlaceholder docs) while actual
// transactions can sometimes have shorter signatures (71 bytes or fewer in
// very rare cases).
func (tse *TransactionSizeEstimator) VirtualSize() (int64, error) {
	if tse.err != nil {
		return 0, tse.err
	}

	return mempool.GetTxVirtualSize(btcutil.NewTx(tse.internal.MsgTx)), nil
}

// TransactionFeeEstimator is a component allowing to estimate the total fee
// for the given transaction virtual size.
type TransactionFeeEstimator struct {
	chain Chain
}

func NewTransactionFeeEstimator(chain Chain) *TransactionFeeEstimator {
	return &TransactionFeeEstimator{chain: chain}
}

// EstimateFee estimates the total fee for the given transaction virtual size,
// assuming the transaction is supposed to be confirmed within the next Bitcoin
// block. The desired confirmation period can be adjusted using the optional
// `blocks` argument.
func (tfe *TransactionFeeEstimator) EstimateFee(
	transactionVirtualSize int64,
	blocks ...uint32,
) (int64, error) {
	resolvedBlocks := uint32(1)

	if len(blocks) == 1 {
		resolvedBlocks = blocks[0]
	}

	satPerVByteFee, err := tfe.chain.EstimateSatPerVByteFee(resolvedBlocks)
	if err != nil {
		return 0, fmt.Errorf("cannot get estimated sat/vbyte fee: [%v]", err)
	}

	fee := satPerVByteFee * transactionVirtualSize

	// Just in case check.
	if fee <= 0 {
		return 0, fmt.Errorf("estimated fee is less than or equal zero")
	}

	return fee, nil
}
