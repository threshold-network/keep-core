package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func walletMainUtxoScriptType(
	bitcoinChain bitcoin.Chain,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
) (bitcoin.ScriptType, error) {
	if walletMainUtxo == nil {
		return bitcoin.NonStandardScript, fmt.Errorf("wallet main UTXO is required")
	}

	if walletMainUtxo.Outpoint == nil {
		return bitcoin.NonStandardScript, fmt.Errorf(
			"wallet main UTXO outpoint is required",
		)
	}

	transaction, err := bitcoinChain.GetTransaction(
		walletMainUtxo.Outpoint.TransactionHash,
	)
	if err != nil {
		return bitcoin.NonStandardScript, fmt.Errorf(
			"cannot get transaction with hash [%s]: [%v]",
			walletMainUtxo.Outpoint.TransactionHash.Hex(bitcoin.InternalByteOrder),
			err,
		)
	}

	outputIndex := walletMainUtxo.Outpoint.OutputIndex
	if outputIndex >= uint32(len(transaction.Outputs)) {
		return bitcoin.NonStandardScript, fmt.Errorf(
			"output index [%d] out of range for transaction [%s] "+
				"with [%d] outputs",
			outputIndex,
			walletMainUtxo.Outpoint.TransactionHash.Hex(bitcoin.InternalByteOrder),
			len(transaction.Outputs),
		)
	}

	return bitcoin.GetScriptType(
		transaction.Outputs[outputIndex].PublicKeyScript,
	), nil
}
