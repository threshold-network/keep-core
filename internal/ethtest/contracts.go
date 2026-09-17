package ethtest

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// read answers one decoded contract read. Reads the fixture knows nothing
// about are reported as errors so that a test never mistakes a silently
// defaulted answer for a modelled one.
func (b *Backend) read(
	contractName string,
	method string,
	args []interface{},
) ([]interface{}, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if message, faulted := b.faults[faultKey(contractName, method)]; faulted {
		return nil, &chainFault{message: message}
	}

	switch contractName {
	case BridgeContract:
		switch method {
		case "contractReferences":
			return []interface{}{
				BankAddress,
				RelayAddress,
				WalletRegistryAddress,
				ReimbursementPoolAddress,
			}, nil
		case "getRedemptionWatchtower":
			// Zero keeps the optional watchtower out of the handle, the same
			// way an unset reference does on chain.
			return []interface{}{common.Address{}}, nil
		}
	case WalletRegistryContract:
		switch method {
		case "sortitionPool":
			return []interface{}{EcdsaSortitionPoolAddress}, nil
		case "staking":
			return []interface{}{TokenStakingAddress}, nil
		case "minimumAuthorization":
			return []interface{}{b.state.MinimumAuthorization}, nil
		case "operatorToStakingProvider":
			operator, err := addressArgument(contractName, method, args)
			if err != nil {
				return nil, err
			}

			return []interface{}{b.state.TbtcOperators[operator]}, nil
		case "eligibleStake":
			stakingProvider, err := addressArgument(contractName, method, args)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				amount(b.state.EligibleStakes[stakingProvider]),
			}, nil
		case "pendingAuthorizationDecrease":
			stakingProvider, err := addressArgument(contractName, method, args)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				amount(b.state.PendingDecreases[stakingProvider]),
			}, nil
		}
	case RandomBeaconContract:
		switch method {
		case "sortitionPool":
			return []interface{}{BeaconSortitionPoolAddress}, nil
		case "staking":
			return []interface{}{TokenStakingAddress}, nil
		case "operatorToStakingProvider":
			operator, err := addressArgument(contractName, method, args)
			if err != nil {
				return nil, err
			}

			return []interface{}{b.state.BeaconOperators[operator]}, nil
		case "eligibleStake":
			// Token staking authorizes no stake for the beacon, so the beacon
			// registry reports none for anybody.
			if _, err := addressArgument(contractName, method, args); err != nil {
				return nil, err
			}

			return []interface{}{big.NewInt(0)}, nil
		}
	case TokenStakingContract:
		if method == "rolesOf" {
			stakingProvider, err := addressArgument(contractName, method, args)
			if err != nil {
				return nil, err
			}

			roles := b.state.Roles[stakingProvider]

			return []interface{}{
				roles.Owner,
				roles.Beneficiary,
				roles.Authorizer,
			}, nil
		}
	case AllowlistContract:
		return nil, fmt.Errorf(
			"read [%s] was routed to the allowlist at [%v] instead of token "+
				"staking at [%v]",
			method,
			AllowlistAddress.Hex(),
			TokenStakingAddress.Hex(),
		)
	}

	return nil, fmt.Errorf(
		"the fixture models no read [%s.%s]",
		contractName,
		method,
	)
}

// amount substitutes the zero the registry reports for an address it holds no
// entry for. The generated bindings cannot encode a nil amount.
func amount(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}

	return value
}

func addressArgument(
	contractName string,
	method string,
	args []interface{},
) (common.Address, error) {
	if len(args) != 1 {
		return common.Address{}, fmt.Errorf(
			"read [%s.%s] takes one argument, got %d",
			contractName,
			method,
			len(args),
		)
	}

	address, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf(
			"read [%s.%s] takes an address, got [%T]",
			contractName,
			method,
			args[0],
		)
	}

	return address, nil
}
