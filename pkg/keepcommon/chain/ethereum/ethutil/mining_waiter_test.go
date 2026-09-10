package ethutil

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
)

var config = ethereum.Config{
	MiningCheckInterval: 1,
	MaxGasFeeCap:        *ethereum.WrapWei(big.NewInt(45000000000)), // 45 Gwei
}

var originalTransactorOptions = &bind.TransactOpts{
	Nonce: big.NewInt(100),
}

func TestForceMining_Legacy_NoResubmission(t *testing.T) {
	originalTransaction := createLegacyTransaction(big.NewInt(20000000000)) // 20 Gwei

	chain := &mockAdaptedEthereumClientWithReceipt{}

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		return createLegacyTransaction(newTransactorOptions.GasPrice), nil
	}

	// receipt is already there
	chain.receipt = &types.Receipt{}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != 0 {
		t.Fatalf("expected no resubmissions; has: [%v]", resubmissionCount)
	}
}

func TestForceMining_Legacy_OneResubmission(t *testing.T) {
	originalTransaction := createLegacyTransaction(big.NewInt(20000000000)) // 20 Gwei

	chain := &mockAdaptedEthereumClientWithReceipt{}

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// first resubmission succeeded
		chain.receipt = &types.Receipt{}
		return createLegacyTransaction(newTransactorOptions.GasPrice), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != 1 {
		t.Fatalf("expected one resubmission; has: [%v]", resubmissionCount)
	}

	resubmission := resubmissions[0]

	assertNonceUnchanged(t, resubmission)

	if resubmission.GasFeeCap != nil || resubmission.GasTipCap != nil {
		t.Fatalf("gas fee and tip cap should be nil")
	}

	expectedGasPrice := big.NewInt(24000000000)
	if resubmission.GasPrice.Cmp(expectedGasPrice) != 0 {
		t.Fatalf(
			"unexpected gas price value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedGasPrice,
			resubmission.GasPrice,
		)
	}
}

func TestForceMining_Legacy_MultipleAttempts(t *testing.T) {
	originalTransaction := createLegacyTransaction(big.NewInt(20000000000)) // 20 Gwei

	chain := &mockAdaptedEthereumClientWithReceipt{}

	var resubmissions []*bind.TransactOpts

	expectedAttempts := 3
	expectedResubmissionGasPrices := []*big.Int{
		big.NewInt(24000000000), // + 20%
		big.NewInt(28800000000), // + 20%
		big.NewInt(34560000000), // + 20%
	}

	attemptsSoFar := 1
	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		if attemptsSoFar == expectedAttempts {
			chain.receipt = &types.Receipt{}
		} else {
			attemptsSoFar++
		}
		return createLegacyTransaction(newTransactorOptions.GasPrice), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != expectedAttempts {
		t.Fatalf(
			"expected [%v] resubmission; has: [%v]",
			expectedAttempts,
			resubmissionCount,
		)
	}

	for index, resubmission := range resubmissions {
		assertNonceUnchanged(t, resubmission)

		if resubmission.GasFeeCap != nil || resubmission.GasTipCap != nil {
			t.Fatalf("resubmission [%v] gas fee and tip cap should be nil", index)
		}

		price := resubmission.GasPrice
		if price.Cmp(expectedResubmissionGasPrices[index]) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v] gas price\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedResubmissionGasPrices[index],
				price,
			)
		}
	}
}

func TestForceMining_Legacy_MaxAllowedPriceReached(t *testing.T) {
	originalTransaction := createLegacyTransaction(big.NewInt(20000000000)) // 20 Gwei

	chain := &mockAdaptedEthereumClientWithReceipt{}

	var resubmissions []*bind.TransactOpts

	expectedAttempts := 5
	expectedResubmissionGasPrices := []*big.Int{
		big.NewInt(24000000000), // + 20%
		big.NewInt(28800000000), // + 20%
		big.NewInt(34560000000), // + 20%
		big.NewInt(41472000000), // + 20%
		big.NewInt(45000000000), // max allowed
	}

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// not setting mockBackend.receipt, mining takes a very long time
		return createLegacyTransaction(newTransactorOptions.GasPrice), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != expectedAttempts {
		t.Fatalf(
			"expected [%v] resubmission; has: [%v]",
			expectedAttempts,
			resubmissionCount,
		)
	}

	for index, resubmission := range resubmissions {
		assertNonceUnchanged(t, resubmission)

		if resubmission.GasFeeCap != nil || resubmission.GasTipCap != nil {
			t.Fatalf("resubmission [%v] gas fee and tip cap should be nil", index)
		}

		price := resubmission.GasPrice
		if price.Cmp(expectedResubmissionGasPrices[index]) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v] gas price\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedResubmissionGasPrices[index],
				price,
			)
		}
	}
}

func TestForceMining_Legacy_OriginalPriceHigherThanMaxAllowed(t *testing.T) {
	// original transaction was priced at 46 Gwei, the maximum allowed gas price
	// is 45 Gwei
	originalTransaction := createLegacyTransaction(big.NewInt(46000000000))

	chain := &mockAdaptedEthereumClientWithReceipt{}

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// not setting mockBackend.receipt, mining takes a very long time
		return createLegacyTransaction(newTransactorOptions.GasPrice), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != 0 {
		t.Fatalf("expected no resubmissions; has: [%v]", resubmissionCount)
	}
}

func TestForceMining_DynamicFee_NoResubmission(t *testing.T) {
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(24000000000) // 24 Gwei (2 * baseFee + gasTipCap)

	originalTransaction := createDynamicFeeTransaction(
		originalGasFeeCap,
		originalGasTipCap,
	)

	chain := &mockAdaptedEthereumClientWithReceipt{
		mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
	}

	// Base fee remains unchanged.
	chain.blocks = append(chain.blocks, big.NewInt(1))
	chain.blocksBaseFee = append(chain.blocksBaseFee, originalBaseFee)

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		return createDynamicFeeTransaction(
			newTransactorOptions.GasFeeCap,
			newTransactorOptions.GasTipCap,
		), nil
	}

	// Receipt is already there.
	chain.receipt = &types.Receipt{}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != 0 {
		t.Fatalf("expected no resubmissions; has: [%v]", resubmissionCount)
	}
}

func TestForceMining_DynamicFee_OneResubmission(t *testing.T) {
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(24000000000) // 24 Gwei (2 * baseFee + gasTipCap)

	var tests = map[string]struct {
		nextBaseFee       *big.Int
		expectedGasFeeCap *big.Int
		expectedGasTipCap *big.Int
	}{
		"base fee decreased": {
			// Base fee decreased to 5 Gwei.
			nextBaseFee: big.NewInt(5000000000),
			// Gas fee cap should be computed as: 2 * 5 Gwei + 4.8 Gwei = 14.8 Gwei.
			// However, this value doesn't fulfill the required base fee bump threshold.
			// In result, the new gas fee value should be bumped up by 10% to
			// satisfy the condition: 24 Gwei * 1.1 = 26.4 Gwei
			expectedGasFeeCap: big.NewInt(26400000000),
			// Gas tip cap should be bumped up by 20%: 4 Gwei * 1.2 = 4.8 Gwei
			expectedGasTipCap: big.NewInt(4800000000),
		},
		"base fee unchanged": {
			// Base fee remains 10 Gwei.
			nextBaseFee: originalBaseFee,
			// Gas fee cap should be computed as: 2 * 10 Gwei + 4.8 Gwei = 24.8 Gwei.
			// However, this value doesn't fulfill the required base fee bump threshold.
			// In result, the new gas fee value should be bumped up by 10% to
			// satisfy the condition: 24 Gwei * 1.1 = 26.4 Gwei
			expectedGasFeeCap: big.NewInt(26400000000),
			// Gas tip cap should be bumped up by 20%: 4 Gwei * 1.2 = 4.8 Gwei
			expectedGasTipCap: big.NewInt(4800000000),
		},
		"base fee increased": {
			// Base fee increased to 20 Gwei.
			nextBaseFee: big.NewInt(20000000000),
			// Gas fee cap should be computed as: 2 * 20 Gwei + 4.8 Gwei = 44.8 Gwei.
			expectedGasFeeCap: big.NewInt(44800000000),
			// Gas tip cap should be bumped up by 20%: 4 Gwei * 1.2 = 4.8 Gwei
			expectedGasTipCap: big.NewInt(4800000000),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			originalTransaction := createDynamicFeeTransaction(
				originalGasFeeCap,
				originalGasTipCap,
			)

			chain := &mockAdaptedEthereumClientWithReceipt{
				mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
			}

			chain.blocks = append(chain.blocks, big.NewInt(1))
			chain.blocksBaseFee = append(chain.blocksBaseFee, test.nextBaseFee)

			var resubmissions []*bind.TransactOpts

			resubmitFn := func(
				newTransactorOptions *bind.TransactOpts,
			) (*types.Transaction, error) {
				resubmissions = append(resubmissions, newTransactorOptions)
				// First resubmission succeeded.
				chain.receipt = &types.Receipt{}
				return createDynamicFeeTransaction(
					newTransactorOptions.GasFeeCap,
					newTransactorOptions.GasTipCap,
				), nil
			}

			waiter := NewMiningWaiter(chain, config)
			waiter.ForceMining(
				originalTransaction,
				originalTransactorOptions,
				resubmitFn,
			)

			resubmissionCount := len(resubmissions)
			if resubmissionCount != 1 {
				t.Fatalf(
					"expected one resubmission; has: [%v]",
					resubmissionCount,
				)
			}

			resubmission := resubmissions[0]

			assertNonceUnchanged(t, resubmission)

			if resubmission.GasPrice != nil {
				t.Fatalf("gas price should be nil")
			}

			if resubmission.GasFeeCap.Cmp(test.expectedGasFeeCap) != 0 {
				t.Fatalf(
					"unexpected gas fee cap value\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedGasFeeCap,
					resubmission.GasFeeCap,
				)
			}

			if resubmission.GasTipCap.Cmp(test.expectedGasTipCap) != 0 {
				t.Fatalf(
					"unexpected gas tip cap value\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedGasTipCap,
					resubmission.GasTipCap,
				)
			}
		})
	}
}

func TestForceMining_DynamicFee_MultipleAttemps(t *testing.T) {
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(24000000000) // 24 Gwei (2 * baseFee + gasTipCap)

	originalTransaction := createDynamicFeeTransaction(
		originalGasFeeCap,
		originalGasTipCap,
	)

	chain := &mockAdaptedEthereumClientWithReceipt{
		mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
	}

	// Base fee remains unchanged.
	chain.blocks = append(chain.blocks, big.NewInt(1))
	chain.blocksBaseFee = append(chain.blocksBaseFee, originalBaseFee)

	var resubmissions []*bind.TransactOpts

	type gasPriceTuple struct {
		gasFeeCap *big.Int
		gasTipCap *big.Int
	}

	expectedAttempts := 3
	expectedResubmissionParams := []*gasPriceTuple{
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(26400000000), big.NewInt(4800000000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(29040000000), big.NewInt(5760000000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(31944000000), big.NewInt(6912000000)},
	}

	attemptsSoFar := 1
	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		if attemptsSoFar == expectedAttempts {
			chain.receipt = &types.Receipt{}
		} else {
			attemptsSoFar++
		}
		return createDynamicFeeTransaction(
			newTransactorOptions.GasFeeCap,
			newTransactorOptions.GasTipCap,
		), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != expectedAttempts {
		t.Fatalf(
			"expected [%v] resubmission; has: [%v]",
			expectedAttempts,
			resubmissionCount,
		)
	}

	for index, resubmission := range resubmissions {
		assertNonceUnchanged(t, resubmission)

		if resubmission.GasPrice != nil {
			t.Fatalf("resubmission [%v] gas price should be nil", index)
		}

		expectedGasFeeCap := expectedResubmissionParams[index].gasFeeCap
		if resubmission.GasFeeCap.Cmp(expectedGasFeeCap) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v] gas fee cap value\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedGasFeeCap,
				resubmission.GasFeeCap,
			)
		}

		expectedGasTipCap := expectedResubmissionParams[index].gasTipCap
		if resubmission.GasTipCap.Cmp(expectedGasTipCap) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v]  gas tip cap value\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedGasTipCap,
				resubmission.GasTipCap,
			)
		}
	}
}

func TestForceMining_DynamicFee_MaxAllowedPriceReached(t *testing.T) {
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(24000000000) // 24 Gwei (2 * baseFee + gasTipCap)

	originalTransaction := createDynamicFeeTransaction(
		originalGasFeeCap,
		originalGasTipCap,
	)

	chain := &mockAdaptedEthereumClientWithReceipt{
		mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
	}

	// Massive increase of base fee to 30 Gwei. This is needed
	// to exceed the maximum gas fee cap value.
	chain.blocks = append(chain.blocks, big.NewInt(1))
	chain.blocksBaseFee = append(
		chain.blocksBaseFee,
		new(big.Int).Mul(originalBaseFee, big.NewInt(3)),
	)

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// Not setting mockBackend.receipt, mining takes a very long time.
		return createDynamicFeeTransaction(
			newTransactorOptions.GasFeeCap,
			newTransactorOptions.GasTipCap,
		), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	// The new gas fee value should be computed as usual:
	// 2 * 30 Gwei + 4.8 gwei = 64.8 Gwei. However, this value exceeds the
	// max gas fee cap value of 45 Gwei. In effect, one resubmission with the
	// maximum value should be made.
	resubmissionCount := len(resubmissions)
	if resubmissionCount != 1 {
		t.Fatalf(
			"expected one resubmission; has: [%v]",
			resubmissionCount,
		)
	}

	resubmission := resubmissions[0]

	assertNonceUnchanged(t, resubmission)

	if resubmission.GasPrice != nil {
		t.Fatalf("gas price should be nil")
	}

	// The new gas fee value should be computed as usual:
	// 2 * 30 Gwei + 4.8 gwei = 64.8 Gwei. However, max gas fee cap value
	// is 45 Gwei so this value should be used instead.
	expectedGasFeeCap := big.NewInt(45000000000)
	if resubmission.GasFeeCap.Cmp(expectedGasFeeCap) != 0 {
		t.Fatalf(
			"unexpected gas fee cap value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedGasFeeCap,
			resubmission.GasFeeCap,
		)
	}

	// Gas tip cap should be bumped up by 20%: 4 Gwei * 1.2 = 4.8 Gwei
	expectedGasTipCap := big.NewInt(4800000000)
	if resubmission.GasTipCap.Cmp(expectedGasTipCap) != 0 {
		t.Fatalf(
			"unexpected gas tip cap value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedGasTipCap,
			resubmission.GasTipCap,
		)
	}
}

func TestForceMining_DynamicFee_MaxAllowedPriceReachedButBelowThreshold(t *testing.T) {
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(24000000000) // 24 Gwei (2 * baseFee + gasTipCap)

	originalTransaction := createDynamicFeeTransaction(
		originalGasFeeCap,
		originalGasTipCap,
	)

	chain := &mockAdaptedEthereumClientWithReceipt{
		mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
	}

	// Base fee remains unchanged.
	chain.blocks = append(chain.blocks, big.NewInt(1))
	chain.blocksBaseFee = append(chain.blocksBaseFee, originalBaseFee)

	var resubmissions []*bind.TransactOpts

	type gasPriceTuple struct {
		gasFeeCap *big.Int
		gasTipCap *big.Int
	}

	expectedAttempts := 6
	expectedResubmissionParams := []*gasPriceTuple{
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(26400000000), big.NewInt(4800000000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(29040000000), big.NewInt(5760000000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(31944000000), big.NewInt(6912000000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(35138400000), big.NewInt(8294400000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(38652240000), big.NewInt(9953280000)},
		// gasFeeCap +10%, gasTipCap +20%
		{big.NewInt(42517464000), big.NewInt(11943936000)},
		// The last attempt would bump the gas fee cap up to 46769210400.
		// However, this value exceeds the max gas fee cap which is 45000000000.
		// On the other hand, the max gas fee cap is under the required fee bump
		// threshold for this iteration so resubmission should not be performed
		// at all.
	}

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// Not setting mockBackend.receipt, mining takes a very long time.
		return createDynamicFeeTransaction(
			newTransactorOptions.GasFeeCap,
			newTransactorOptions.GasTipCap,
		), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != expectedAttempts {
		t.Fatalf(
			"expected [%v] resubmission; has: [%v]",
			expectedAttempts,
			resubmissionCount,
		)
	}

	for index, resubmission := range resubmissions {
		assertNonceUnchanged(t, resubmission)

		if resubmission.GasPrice != nil {
			t.Fatalf("resubmission [%v] gas price should be nil", index)
		}

		expectedGasFeeCap := expectedResubmissionParams[index].gasFeeCap
		if resubmission.GasFeeCap.Cmp(expectedGasFeeCap) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v] gas fee cap value\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedGasFeeCap,
				resubmission.GasFeeCap,
			)
		}

		expectedGasTipCap := expectedResubmissionParams[index].gasTipCap
		if resubmission.GasTipCap.Cmp(expectedGasTipCap) != 0 {
			t.Fatalf(
				"unexpected resubmission [%v]  gas tip cap value\n"+
					"expected: [%v]\n"+
					"actual:   [%v]",
				index,
				expectedGasTipCap,
				resubmission.GasTipCap,
			)
		}
	}
}

func TestForceMining_DynamicFee_OriginalPriceHigherThanMaxAllowed(t *testing.T) {
	// Original transaction has gas fee cap set at 46 Gwei, the maximum allowed
	// gas fee cap is 45 Gwei.
	originalBaseFee := big.NewInt(10000000000)   // 10 Gwei
	originalGasTipCap := big.NewInt(4000000000)  // 4 Gwei
	originalGasFeeCap := big.NewInt(46000000000) // 46 Gwei

	originalTransaction := createDynamicFeeTransaction(
		originalGasFeeCap,
		originalGasTipCap,
	)

	chain := &mockAdaptedEthereumClientWithReceipt{
		mockAdaptedEthereumClient: &mockAdaptedEthereumClient{},
	}

	// Base fee remains unchanged.
	chain.blocks = append(chain.blocks, big.NewInt(1))
	chain.blocksBaseFee = append(chain.blocksBaseFee, originalBaseFee)

	var resubmissions []*bind.TransactOpts

	resubmitFn := func(
		newTransactorOptions *bind.TransactOpts,
	) (*types.Transaction, error) {
		resubmissions = append(resubmissions, newTransactorOptions)
		// Not setting mockBackend.receipt, mining takes a very long time.
		return createDynamicFeeTransaction(
			newTransactorOptions.GasFeeCap,
			newTransactorOptions.GasTipCap,
		), nil
	}

	waiter := NewMiningWaiter(chain, config)
	waiter.ForceMining(
		originalTransaction,
		originalTransactorOptions,
		resubmitFn,
	)

	resubmissionCount := len(resubmissions)
	if resubmissionCount != 0 {
		t.Fatalf("expected no resubmissions; has: [%v]", resubmissionCount)
	}
}

func assertNonceUnchanged(
	t *testing.T,
	newTransactionOptions *bind.TransactOpts,
) {
	if newTransactionOptions.Nonce.Cmp(originalTransactorOptions.Nonce) != 0 {
		t.Fatalf("nonce should remain unchanged")
	}
}

func createLegacyTransaction(gasPrice *big.Int) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		GasPrice: gasPrice,
		Gas:      25000,
	})
}

func createDynamicFeeTransaction(gasFeeCap, gasTipCap *big.Int) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       35000,
	})
}

type mockAdaptedEthereumClientWithReceipt struct {
	*mockAdaptedEthereumClient

	receipt *types.Receipt
}

func (maecwr *mockAdaptedEthereumClientWithReceipt) TransactionReceipt(
	ctx context.Context,
	txHash common.Hash,
) (*types.Receipt, error) {
	return maecwr.receipt, nil
}
