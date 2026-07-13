package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	chainEthereum "github.com/keep-network/keep-core/pkg/chain/ethereum"

	ethereumBeacon "github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen"
	ethereumEcdsa "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen"
	ethereumTbtc "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen"
	ethereumThreshold "github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen"
)

// GetDeveloperContractAddressKey returns a key for developer contract address configuration.
func GetDeveloperContractAddressKey(contractName string) string {
	return fmt.Sprintf(
		"developer.%sAddress",
		strings.ToLower(contractName[:1])+contractName[1:],
	)
}

// Configuration properties for contract addresses are expected to be defined with
// name `developer.<contractName>Address`. Although the `Config` struct stores them
// in the `Ethereum.ContractAddresses` map. This function binds these two with
// an alias.
func initializeContractAddressesAliases() {
	aliasEthereumContract := func(contractName string) {
		configStructProperty := fmt.Sprintf("Ethereum.ContractAddresses.%s", contractName)
		configKey := GetDeveloperContractAddressKey(contractName)

		viper.RegisterAlias(configStructProperty, configKey)
	}

	aliasEthereumContract(chainEthereum.RandomBeaconContractName)
	aliasEthereumContract(chainEthereum.TokenStakingContractName)
	aliasEthereumContract(chainEthereum.WalletRegistryContractName)
	aliasEthereumContract(chainEthereum.BridgeContractName)
	aliasEthereumContract(chainEthereum.MaintainerProxyContractName)
	aliasEthereumContract(chainEthereum.LightRelayContractName)
	aliasEthereumContract(chainEthereum.LightRelayMaintainerProxyContractName)
	aliasEthereumContract(chainEthereum.WalletProposalValidatorContractName)
	aliasEthereumContract(chainEthereum.EcdsaDkgValidatorContractName)
}

// resolveContractsAddresses verifies if contracts addresses are configured, if not
// it will set contracts addresses to the defaults, obtained from published NPM packages.
// This function should be used to complete developer configuration that allows
// users to explicitly configure contracts addresses.
// TODO: Resolve the addresses based on a `--network` flag once multi-network
// is supported.
func (c *Config) resolveContractsAddresses() {
	if c.Ethereum.ContractAddresses == nil {
		c.Ethereum.ContractAddresses = make(map[string]string)
	}

	resolveContractAddress := func(contractName string, defaultAddress string) {
		_, err := c.Ethereum.ContractAddress(contractName)
		if errors.Is(err, commonEthereum.ErrAddressNotConfigured) {
			c.Ethereum.SetContractAddress(contractName, defaultAddress)
		}
	}

	resolveContractAddress(
		chainEthereum.RandomBeaconContractName,
		ethereumBeacon.RandomBeaconAddress,
	)
	resolveContractAddress(
		chainEthereum.WalletRegistryContractName,
		ethereumEcdsa.WalletRegistryAddress,
	)
	resolveContractAddress(
		chainEthereum.BridgeContractName,
		ethereumTbtc.BridgeAddress,
	)
	resolveContractAddress(
		chainEthereum.MaintainerProxyContractName,
		ethereumTbtc.MaintainerProxyAddress,
	)
	resolveContractAddress(
		chainEthereum.LightRelayContractName,
		ethereumTbtc.LightRelayAddress,
	)
	resolveContractAddress(
		chainEthereum.LightRelayMaintainerProxyContractName,
		ethereumTbtc.LightRelayMaintainerProxyAddress,
	)
	resolveContractAddress(
		chainEthereum.TokenStakingContractName,
		ethereumThreshold.TokenStakingAddress,
	)
	resolveContractAddress(
		chainEthereum.WalletProposalValidatorContractName,
		ethereumTbtc.WalletProposalValidatorAddress,
	)
}
