package ethutil

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// TransactionOptions represents custom transaction options which can be used
// while invoking contracts methods.
type TransactionOptions struct {
	// GasLimit specifies a gas limit to set for a transaction; estimated if
	// set to 0.
	GasLimit uint64

	// GasFeeCap specifies gas fee cap for EIP-1559 transaction; estimated if
	// set to nil.
	GasFeeCap *big.Int

	// GasTipCap specifies gas priority fee cap to use for EIP-1559 transaction;
	// estimated if set to nil.
	GasTipCap *big.Int
}

// Apply takes a bind.TransactOpts pointer and applies the options described in
// TransactionOptions to it. Note that GasTipCap, GasFeeCap set to nil and
// GasLimit set to 0 are not applied to the passed bind.TransactOpts; these
// values indicate that the original values in bind.TransactOpts should remain
// unchanged.
func (to TransactionOptions) Apply(transactorOptions *bind.TransactOpts) {
	if customGasLimit := to.GasLimit; customGasLimit != 0 {
		transactorOptions.GasLimit = customGasLimit
	}
	if customGasFeeCap := to.GasFeeCap; customGasFeeCap != nil {
		transactorOptions.GasFeeCap = customGasFeeCap
	}
	if customGasTipCap := to.GasTipCap; customGasTipCap != nil {
		transactorOptions.GasTipCap = customGasTipCap
	}
}
