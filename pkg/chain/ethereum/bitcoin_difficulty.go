package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/maintainer"
)

// Definitions of contract names.
const (
	LightRelayContractName                = "LightRelay"
	LightRelayMaintainerProxyContractName = "LightRelayMaintainerProxy"
)

// waitDeployBackendTransactionMinedTimeout bounds the synchronous post-submit
// wait for retarget transactions. The wait covers both receipt polling and the
// follow-up confirmation-depth wait. Without a bound, a stalled RPC or a chain
// that stops producing blocks would hang the maintainer indefinitely.
const waitDeployBackendTransactionMinedTimeout = 10 * time.Minute

// BitcoinDifficultyChain represents a Bitcoin difficulty-specific chain handle.
type BitcoinDifficultyChain struct {
	*baseChain

	lightRelay                *contract.LightRelay
	lightRelayMaintainerProxy *contract.LightRelayMaintainerProxy
}

// NewBitcoinDifficultyChain construct a new instance of the Bitcoin difficulty
// - specific Ethereum chain handle.
func NewBitcoinDifficultyChain(
	ethereumConfig ethereum.Config,
	maintainerConfig maintainer.Config,
	baseChain *baseChain,
) (*BitcoinDifficultyChain, error) {
	lightRelayAddress, err := ethereumConfig.ContractAddress(
		LightRelayContractName,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to LightRelay contract: [%w]",
			err,
		)
	}

	lightRelay, err :=
		contract.NewLightRelay(
			lightRelayAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to LightRelay contract: [%w]",
			err,
		)
	}

	// If the Bitcoin difficulty should be updated directly via LightRelay,
	// quit early without creating a handle to LightRelayMaintainerProxy.
	if maintainerConfig.BitcoinDifficulty.DisableProxy {
		return &BitcoinDifficultyChain{
			baseChain:                 baseChain,
			lightRelay:                lightRelay,
			lightRelayMaintainerProxy: nil,
		}, nil
	}

	// The Bitcoin difficulty should be updated via LightRelayMaintainerProxy.
	lightRelayMaintainerProxyAddress, err := ethereumConfig.ContractAddress(
		LightRelayMaintainerProxyContractName,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to LightRelayMaintainerProxy contract: [%w]",
			err,
		)
	}

	lightRelayMaintainerProxy, err :=
		contract.NewLightRelayMaintainerProxy(
			lightRelayMaintainerProxyAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to LightRelayMaintainerProxy contract: [%w]",
			err,
		)
	}

	retrievedLightRelayAddress, err := lightRelayMaintainerProxy.LightRelay()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve the relay address from LightRelayMaintainerProxy "+
				"contract: [%w]",
			err,
		)
	}

	// Verify the LightRelay set in LightRelayMaintainerProxy is the same
	// instance as the one set in the client via configuration.
	if lightRelayAddress != retrievedLightRelayAddress {
		return nil, fmt.Errorf("mismatch between LightRelay addresses")
	}

	return &BitcoinDifficultyChain{
		baseChain:                 baseChain,
		lightRelay:                lightRelay,
		lightRelayMaintainerProxy: lightRelayMaintainerProxy,
	}, nil
}

// waitDeployBackendTransactionMined blocks until tx is mined. Generated contract
// bindings submit via asynchronous mining waiter; without this wait, callers can
// read stale LightRelay.currentEpoch via eth_call and fail the next
// RetargetGasEstimate with "Invalid target in pre-retarget headers".
func (bdc *BitcoinDifficultyChain) waitDeployBackendTransactionMined(
	tx *types.Transaction,
	method string,
) error {
	if tx == nil {
		return fmt.Errorf("nil transaction waiting for [%s]", method)
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		waitDeployBackendTransactionMinedTimeout,
	)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, bdc.client, tx)
	if err != nil {
		return fmt.Errorf("waiting for transaction [%s] [%s]: [%w]", method, tx.Hash().Hex(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf(
			"transaction [%s] [%s] failed with status [%d]",
			method,
			tx.Hash().Hex(),
			receipt.Status,
		)
	}
	// Some RPC load balancers can lag reads after receipt; wait a few L1
	// confirmations before the next RetargetGasEstimate/eth_call.
	if receipt.BlockNumber != nil && bdc.blockCounter != nil {
		includedAt := receipt.BlockNumber.Uint64()
		const confirmDepth = uint64(3)
		if err := waitForBlockHeightCtx(
			ctx, bdc.blockCounter, includedAt+confirmDepth,
		); err != nil {
			return fmt.Errorf(
				"waiting confirmation depth after [%s] [%s]: [%w]",
				method,
				tx.Hash().Hex(),
				err,
			)
		}
	}
	return nil
}

// waitForBlockHeightCtxPollInterval is how often waitForBlockHeightCtx polls the
// chain height. Block times on the supported networks are far larger, so this is
// responsive without busy-looping.
const waitForBlockHeightCtxPollInterval = 1 * time.Second

// waitForBlockHeightCtx waits until the chain reaches blockNumber or ctx is
// cancelled, whichever happens first. It polls the context-less
// chain.BlockCounter via the non-blocking CurrentBlock rather than spawning a
// goroutine parked in WaitForBlockHeight: callers retry this under a deadline,
// so a goroutine that cannot be cancelled would accumulate on a stalled chain.
// Polling leaves nothing behind once the function returns.
func waitForBlockHeightCtx(
	ctx context.Context,
	bc chain.BlockCounter,
	blockNumber uint64,
) error {
	ticker := time.NewTicker(waitForBlockHeightCtxPollInterval)
	defer ticker.Stop()

	for {
		current, err := bc.CurrentBlock()
		if err != nil {
			return fmt.Errorf("failed to read current block height: [%w]", err)
		}
		if current >= blockNumber {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Ready checks whether the relay is active (i.e. genesis has been performed).
// Note that if the relay is used by querying the current and previous epoch
// difficulty, at least one retarget needs to be provided after genesis;
// otherwise the prevEpochDifficulty will be uninitialised and zero.
func (bdc *BitcoinDifficultyChain) Ready() (bool, error) {
	return bdc.lightRelay.Ready()
}

// IsAuthorized checks whether the given address has been authorized to submit
// a retarget directly to LightRelay. This function should be used when
// retargetting via LightRelayMaintainerProxy is disabled.
func (bdc *BitcoinDifficultyChain) IsAuthorized(address chain.Address) (bool, error) {
	authorizationRequired, err := bdc.lightRelay.AuthorizationRequired()
	if err != nil {
		return false, fmt.Errorf(
			"cannot check whether authorization is required to submit "+
				"block headers: [%w]",
			err,
		)
	}

	if !authorizationRequired {
		return true, nil
	}

	return bdc.lightRelay.IsAuthorized(
		common.HexToAddress(address.String()),
	)
}

// IsAuthorizedForRefund checks whether the given address has been authorized to
// submit a retarget via LightRelayMaintainerProxy. This function should be used
// when retargetting via LightRelayMaintainerProxy is not disabled.
func (bdc *BitcoinDifficultyChain) IsAuthorizedForRefund(address chain.Address) (bool, error) {
	return bdc.lightRelayMaintainerProxy.IsAuthorized(
		common.HexToAddress(address.String()),
	)
}

// Retarget adds a new epoch to the relay by providing a proof of the difficulty
// before and after the retarget. The cost of calling this function is not
// refunded to the caller.
func (bdc *BitcoinDifficultyChain) Retarget(headers []*bitcoin.BlockHeader) error {
	var serializedHeaders []byte
	for _, header := range headers {
		serializedHeader := header.Serialize()
		serializedHeaders = append(serializedHeaders, serializedHeader[:]...)
	}

	// Update Bitcoin difficulty directly via LightRelay.
	tx, err := bdc.lightRelay.Retarget(serializedHeaders)
	if err != nil {
		return err
	}
	return bdc.waitDeployBackendTransactionMined(tx, "LightRelay.Retarget")
}

// RetargetWithRefund adds a new epoch to the relay by providing a proof of the
// difficulty before and after the retarget. The cost of calling this function
// is refunded to the caller.
func (bdc *BitcoinDifficultyChain) RetargetWithRefund(headers []*bitcoin.BlockHeader) error {
	var serializedHeaders []byte
	for _, header := range headers {
		serializedHeader := header.Serialize()
		serializedHeaders = append(serializedHeaders, serializedHeader[:]...)
	}

	gasEstimate, err := bdc.lightRelayMaintainerProxy.RetargetGasEstimate(
		serializedHeaders,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to estimate gas for retarget with refund: [%w]",
			err,
		)
	}

	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

	// Update Bitcoin difficulty via LightRelayMaintainerProxy.
	tx, err := bdc.lightRelayMaintainerProxy.Retarget(
		serializedHeaders,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)
	if err != nil {
		return err
	}
	return bdc.waitDeployBackendTransactionMined(
		tx,
		"LightRelayMaintainerProxy.Retarget",
	)
}

// CurrentEpoch returns the number of the latest difficulty epoch which is
// proven to the relay. If the genesis epoch's number is set correctly, and
// retargets along the way have been legitimate, this equals the height of
// the block starting the most recent epoch, divided by 2016.
func (bdc *BitcoinDifficultyChain) CurrentEpoch() (uint64, error) {
	return bdc.lightRelay.CurrentEpoch()
}

// ProofLength returns the number of blocks required for each side of a retarget
// proof.
func (bdc *BitcoinDifficultyChain) ProofLength() (uint64, error) {
	return bdc.lightRelay.ProofLength()
}

// GetCurrentAndPrevEpochDifficulty returns the difficulties of the current
// and previous Bitcoin epochs.
func (bdc *BitcoinDifficultyChain) GetCurrentAndPrevEpochDifficulty() (
	*big.Int, *big.Int, error,
) {
	difficulties, err := bdc.lightRelay.GetCurrentAndPrevEpochDifficulty()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get epoch difficulties: [%w]",
			err,
		)
	}

	return difficulties.Current, difficulties.Previous, nil
}
