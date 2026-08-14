package ethereum

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/keep-network/keep-core/pkg/chain"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	"github.com/keep-network/keep-core/pkg/sortition"
)

// The seven sortition.Chain methods that TbtcChain routes through
// hasFrostAuthorization() are inherently a process-global switch: a single
// MonitorPool loop reads them and therefore maintains exactly one pool. During
// the ECDSA->FROST migration an operator participates in BOTH the legacy ECDSA
// and the FROST sortition pools at once (existing ECDSA wallets keep draining
// while FROST is live), and each pool needs its own maintenance loop. These two
// views bind the sortition.Chain surface to one specific pool/registry with NO
// hasFrostAuthorization() switch, so the caller can run one MonitorPool per pool.
//
// The pool-monitoring loops use these views directly. Wallet heartbeat actions
// also select the view matching the executing wallet, so a process-wide FROST
// configuration cannot redirect a draining legacy wallet's authorization check.
// GetOperatorID stays bound to whichever pool the view targets, matching the
// per-pool member IDs (the legacy view's GetOperatorID equals TbtcChain's
// deliberately-ECDSA-bound GetOperatorID).

// sortitionPoolView implements the pool-bound subset of sortition.Chain shared by
// both views. The FROST and ECDSA sortition pools are the same contract type
// (ecdsacontract.EcdsaSortitionPool), distinct instances, so these seven methods
// are identical apart from which pool instance they target.
type sortitionPoolView struct {
	pool            *ecdsacontract.EcdsaSortitionPool
	operatorAddress common.Address
}

func (v *sortitionPoolView) IsPoolLocked() (bool, error) {
	return v.pool.IsLocked()
}

func (v *sortitionPoolView) IsEligibleForRewards() (bool, error) {
	return v.pool.IsEligibleForRewards(v.operatorAddress)
}

func (v *sortitionPoolView) CanRestoreRewardEligibility() (bool, error) {
	return v.pool.CanRestoreRewardEligibility(v.operatorAddress)
}

func (v *sortitionPoolView) RestoreRewardEligibility() error {
	_, err := v.pool.RestoreRewardEligibility(v.operatorAddress)
	return err
}

func (v *sortitionPoolView) IsChaosnetActive() (bool, error) {
	return v.pool.IsChaosnetActive()
}

func (v *sortitionPoolView) IsBetaOperator() (bool, error) {
	return v.pool.IsBetaOperator(v.operatorAddress)
}

func (v *sortitionPoolView) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	return getOperatorID(v.pool, common.HexToAddress(operatorAddress.String()))
}

// ecdsaSortitionChain is a sortition.Chain bound explicitly to the legacy ECDSA
// WalletRegistry and sortition pool. Its registry methods mirror the non-FROST
// branch of the corresponding TbtcChain methods.
type ecdsaSortitionChain struct {
	sortitionPoolView
	walletRegistry *ecdsacontract.WalletRegistry
}

func (c *ecdsaSortitionChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	stakingProvider, err := c.walletRegistry.OperatorToStakingProvider(c.operatorAddress)
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			c.operatorAddress,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return "", false, nil
	}

	return chain.Address(stakingProvider.Hex()), true, nil
}

func (c *ecdsaSortitionChain) EligibleStake(
	stakingProvider chain.Address,
) (*big.Int, error) {
	eligibleStake, err := c.walletRegistry.EligibleStake(
		common.HexToAddress(stakingProvider.String()),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get eligible stake for staking provider %s: [%w]",
			stakingProvider,
			err,
		)
	}

	return eligibleStake, nil
}

func (c *ecdsaSortitionChain) IsOperatorInPool() (bool, error) {
	return c.walletRegistry.IsOperatorInPool(c.operatorAddress)
}

func (c *ecdsaSortitionChain) IsOperatorUpToDate() (bool, error) {
	return c.walletRegistry.IsOperatorUpToDate(c.operatorAddress)
}

func (c *ecdsaSortitionChain) JoinSortitionPool() error {
	_, err := c.walletRegistry.JoinSortitionPool()
	return err
}

func (c *ecdsaSortitionChain) UpdateOperatorStatus() error {
	_, err := c.walletRegistry.UpdateOperatorStatus(c.operatorAddress)
	return err
}

// frostSortitionChain is a sortition.Chain bound explicitly to the FROST
// WalletRegistry and sortition pool. Its registry methods mirror the FROST
// branch of the corresponding TbtcChain methods, including the explicit CallOpts
// and the transaction-mutex/nonce-serialized submission helper (shared with the
// ECDSA registry binding, so the two monitor loops never race on the operator
// account nonce).
type frostSortitionChain struct {
	sortitionPoolView
	tc *TbtcChain
}

func (c *frostSortitionChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	stakingProvider, err := c.tc.frostWalletRegistry.OperatorToStakingProvider(
		&bind.CallOpts{From: c.operatorAddress},
		c.operatorAddress,
	)
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			c.operatorAddress,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return "", false, nil
	}

	return chain.Address(stakingProvider.Hex()), true, nil
}

func (c *frostSortitionChain) EligibleStake(
	stakingProvider chain.Address,
) (*big.Int, error) {
	eligibleStake, err := c.tc.frostWalletRegistry.EligibleStake(
		&bind.CallOpts{From: c.operatorAddress},
		common.HexToAddress(stakingProvider.String()),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get eligible stake for staking provider %s: [%w]",
			stakingProvider,
			err,
		)
	}

	return eligibleStake, nil
}

func (c *frostSortitionChain) IsOperatorInPool() (bool, error) {
	return c.tc.frostWalletRegistry.IsOperatorInPool(
		&bind.CallOpts{From: c.operatorAddress},
		c.operatorAddress,
	)
}

func (c *frostSortitionChain) IsOperatorUpToDate() (bool, error) {
	return c.tc.frostWalletRegistry.IsOperatorUpToDate(
		&bind.CallOpts{From: c.operatorAddress},
		c.operatorAddress,
	)
}

func (c *frostSortitionChain) JoinSortitionPool() error {
	return c.tc.submitFrostWalletRegistryTransaction(
		"joinSortitionPool",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return c.tc.frostWalletRegistry.JoinSortitionPool(opts)
		},
	)
}

func (c *frostSortitionChain) UpdateOperatorStatus() error {
	return c.tc.submitFrostWalletRegistryTransaction(
		"updateOperatorStatus",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return c.tc.frostWalletRegistry.UpdateOperatorStatus(
				opts,
				c.operatorAddress,
			)
		},
	)
}

// LegacyECDSASortitionChain returns a sortition.Chain bound explicitly to the
// legacy ECDSA sortition pool, for pool monitoring and legacy wallet actions
// during the FROST migration drain (independent of whether FROST authorization
// is configured).
func (tc *TbtcChain) LegacyECDSASortitionChain() sortition.Chain {
	return &ecdsaSortitionChain{
		sortitionPoolView: sortitionPoolView{
			pool:            tc.sortitionPool,
			operatorAddress: tc.key.Address,
		},
		walletRegistry: tc.walletRegistry,
	}
}

// FrostSortitionChain returns a sortition.Chain bound explicitly to the FROST
// sortition pool, and a flag reporting whether FROST authorization is configured
// for this node. When the flag is false, the returned chain is nil and FROST
// pool monitoring or wallet actions must not be started.
func (tc *TbtcChain) FrostSortitionChain() (sortition.Chain, bool) {
	if !tc.hasFrostAuthorization() {
		return nil, false
	}

	return &frostSortitionChain{
		sortitionPoolView: sortitionPoolView{
			pool:            tc.frostSortitionPool,
			operatorAddress: tc.key.Address,
		},
		tc: tc,
	}, true
}
