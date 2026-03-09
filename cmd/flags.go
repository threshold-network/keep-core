package cmd

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/cmd/flag"
	"github.com/keep-network/keep-common/pkg/rate"
	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/config/network"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	chainEthereum "github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func initGlobalFlags(
	cmd *cobra.Command,
	configFilePath *string,
) {
	initGlobalConfigFlags(cmd, configFilePath)
	initGlobalNetworkFlags(cmd)
}

func initFlags(
	cmd *cobra.Command,
	configFilePath *string,
	cfg *config.Config,
	categories ...config.Category,
) {
	for _, category := range categories {
		switch category {
		case config.Ethereum:
			initEthereumFlags(cmd, cfg)
		case config.BitcoinElectrum:
			initBitcoinElectrumFlags(cmd, cfg)
		case config.Network:
			initNetworkFlags(cmd, cfg)
		case config.Storage:
			initStorageFlags(cmd, cfg)
		case config.ClientInfo:
			initClientInfoFlags(cmd, cfg)
		case config.CovenantSigner:
			initCovenantSignerFlags(cmd, cfg)
		case config.Tbtc:
			initTbtcFlags(cmd, cfg)
		case config.Maintainer:
			initMaintainerFlags(cmd, cfg)
		case config.Developer:
			initDeveloperFlags(cmd)
		}
	}

	// Display flags in help in the same order they are defined. By default the
	// flags are ordered alphabetically which reduces readability.
	cmd.Flags().SortFlags = false
}

// Initialize flag for configuration file path.
func initGlobalConfigFlags(cmd *cobra.Command, configFilePath *string) {
	cmd.PersistentFlags().StringVarP(
		configFilePath,
		"config",
		"c",
		"", // Don't define default value as it would fail configuration reading.
		"Path to the configuration file. Supported formats: TOML, YAML, JSON.",
	)
}

// Initializes boolean flags for network configuration. The flags can be used
// to run a client for a specific network, e.g. add `--testnet` to the client
// start command to run the client against test networks. Only one flag
// from this set is allowed.
func initGlobalNetworkFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(
		network.Mainnet.String(),
		false,
		"Mainnet network",
	)

	cmd.PersistentFlags().Bool(
		network.Testnet.String(),
		false,
		"Testnet network",
	)

	cmd.PersistentFlags().Bool(
		network.Developer.String(),
		false,
		"Developer network",
	)

	cmd.MarkFlagsMutuallyExclusive(
		network.Mainnet.String(),
		network.Testnet.String(),
		network.Developer.String(),
	)
}

// Initialize flags for Ethereum configuration.
func initEthereumFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(
		&cfg.Ethereum.URL,
		"ethereum.url",
		"",
		"WS connection URL for Ethereum client.",
	)

	cmd.Flags().StringVar(
		&cfg.Ethereum.Account.KeyFile,
		"ethereum.keyFile",
		"",
		"The local filesystem path to Keep operator account keyfile.",
	)

	cmd.Flags().DurationVar(
		&cfg.Ethereum.MiningCheckInterval,
		"ethereum.miningCheckInterval",
		ethutil.DefaultMiningCheckInterval,
		"The time interval in seconds in which transaction mining status is checked. If the transaction is not mined within this time, the gas price is increased and transaction is resubmitted.",
	)

	flag.WeiVarFlag(
		cmd.Flags(),
		&cfg.Ethereum.MaxGasFeeCap,
		"ethereum.maxGasFeeCap",
		ethutil.DefaultMaxGasFeeCap,
		"The maximum gas fee the client is willing to pay for the transaction to be mined. If reached, no resubmission attempts are performed.",
	)

	cmd.Flags().IntVar(
		&cfg.Ethereum.RequestsPerSecondLimit,
		"ethereum.requestPerSecondLimit",
		rate.DefaultRequestsPerSecondLimit,
		"Request per second limit for all types of Ethereum client requests.",
	)

	cmd.Flags().IntVar(
		&cfg.Ethereum.ConcurrencyLimit,
		"ethereum.concurrencyLimit",
		rate.DefaultConcurrencyLimit,
		"The maximum number of concurrent requests which can be executed against Ethereum client.",
	)

	flag.WeiVarFlag(
		cmd.Flags(),
		&cfg.Ethereum.BalanceAlertThreshold,
		"ethereum.balanceAlertThreshold",
		*commonEthereum.WrapWei(big.NewInt(500000000000000000)), // 0.5 ether
		"The minimum balance of operator account below which client starts reporting errors in logs.",
	)
}

// Initialize flags for Bitcoin electrum configuration.
func initBitcoinElectrumFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(
		&cfg.Bitcoin.Electrum.URL,
		"bitcoin.electrum.url",
		"",
		"URL to the Electrum server in format: `scheme://hostname:port`.",
	)

	cmd.Flags().DurationVar(
		&cfg.Bitcoin.Electrum.ConnectTimeout,
		"bitcoin.electrum.connectTimeout",
		electrum.DefaultConnectTimeout,
		"Timeout for a single attempt of Electrum connection establishment.",
	)

	cmd.Flags().DurationVar(
		&cfg.Bitcoin.Electrum.ConnectRetryTimeout,
		"bitcoin.electrum.connectRetryTimeout",
		electrum.DefaultConnectRetryTimeout,
		"Timeout for Electrum connection establishment retries.",
	)

	cmd.Flags().DurationVar(
		&cfg.Bitcoin.Electrum.RequestTimeout,
		"bitcoin.electrum.requestTimeout",
		electrum.DefaultRequestTimeout,
		"Timeout for a single attempt of Electrum protocol request.",
	)

	cmd.Flags().DurationVar(
		&cfg.Bitcoin.Electrum.RequestRetryTimeout,
		"bitcoin.electrum.requestRetryTimeout",
		electrum.DefaultRequestRetryTimeout,
		"Timeout for Electrum protocol request retries.",
	)

	cmd.Flags().DurationVar(
		&cfg.Bitcoin.Electrum.KeepAliveInterval,
		"bitcoin.electrum.keepAliveInterval",
		electrum.DefaultKeepAliveInterval,
		"Interval for connection keep alive requests.",
	)
}

// Initialize flags for Network configuration.
func initNetworkFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().BoolVar(
		&cfg.LibP2P.Bootstrap,
		"network.bootstrap",
		false,
		"Run the client in bootstrap mode.",
	)

	cmd.Flags().StringSliceVar(
		&cfg.LibP2P.Peers,
		"network.peers",
		[]string{},
		"Addresses of the network bootstrap nodes.",
	)

	cmd.Flags().IntVarP(
		&cfg.LibP2P.Port,
		"network.port",
		"p",
		libp2p.DefaultPort,
		"Keep client listening port.",
	)

	cmd.Flags().StringSliceVar(
		&cfg.LibP2P.AnnouncedAddresses,
		"network.announcedAddresses",
		[]string{},
		"Overwrites the default Keep client address announced in the network. Should be used for NAT or when more advanced firewall rules are applied.",
	)

	cmd.Flags().IntVar(
		&cfg.LibP2P.DisseminationTime,
		"network.disseminationTime",
		0,
		"Specifies courtesy message dissemination time in seconds for topics the node is not subscribed to. Should be used only on selected bootstrap nodes. (0 = none)",
	)
}

// Initialize flags for Storage configuration.
func initStorageFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(
		&cfg.Storage.Dir,
		"storage.dir",
		"",
		"Location to store the Keep client key shares and other sensitive data.",
	)
}

// Initialize flags for ClientInfo configuration.
func initClientInfoFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().IntVar(
		&cfg.ClientInfo.Port,
		"clientInfo.port",
		9601,
		"Client Info HTTP server listening port.",
	)

	cmd.Flags().DurationVar(
		&cfg.ClientInfo.NetworkMetricsTick,
		"clientInfo.networkMetricsTick",
		clientinfo.DefaultNetworkMetricsTick,
		"Client Info network metrics check tick in seconds.",
	)

	cmd.Flags().DurationVar(
		&cfg.ClientInfo.EthereumMetricsTick,
		"clientInfo.ethereumMetricsTick",
		clientinfo.DefaultEthereumMetricsTick,
		"Client info Ethereum metrics check tick in seconds.",
	)
}

func initTbtcFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().IntVar(
		&cfg.Tbtc.PreParamsPoolSize,
		"tbtc.preParamsPoolSize",
		tbtc.DefaultPreParamsPoolSize,
		"tECDSA pre-parameters pool size.",
	)

	cmd.Flags().DurationVar(
		&cfg.Tbtc.PreParamsGenerationTimeout,
		"tbtc.preParamsGenerationTimeout",
		tbtc.DefaultPreParamsGenerationTimeout,
		"tECDSA pre-parameters generation timeout.",
	)

	cmd.Flags().DurationVar(
		&cfg.Tbtc.PreParamsGenerationDelay,
		"tbtc.preParamsGenerationDelay",
		tbtc.DefaultPreParamsGenerationDelay,
		"tECDSA pre-parameters generation delay.",
	)

	cmd.Flags().IntVar(
		&cfg.Tbtc.PreParamsGenerationConcurrency,
		"tbtc.preParamsGenerationConcurrency",
		tbtc.DefaultPreParamsGenerationConcurrency,
		"tECDSA pre-parameters generation concurrency.",
	)

	cmd.Flags().IntVar(
		&cfg.Tbtc.KeyGenerationConcurrency,
		"tbtc.keyGenerationConcurrency",
		tbtc.DefaultKeyGenerationConcurrency,
		"tECDSA key generation concurrency.",
	)
}

func initCovenantSignerFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().IntVar(
		&cfg.CovenantSigner.Port,
		"covenantSigner.port",
		covenantsigner.Config{}.Port,
		"Covenant signer provider HTTP server listening port. Zero disables the service.",
	)
}

// Initialize flags for Maintainer configuration.
func initMaintainerFlags(command *cobra.Command, cfg *config.Config) {
	command.Flags().BoolVar(
		&cfg.Maintainer.BitcoinDifficulty.Enabled,
		"bitcoinDifficulty",
		false,
		"Start Bitcoin difficulty maintainer.",
	)

	command.Flags().BoolVar(
		&cfg.Maintainer.BitcoinDifficulty.DisableProxy,
		"bitcoinDifficulty.disableProxy",
		false,
		"Disable Bitcoin difficulty proxy.",
	)

	command.Flags().BoolVar(
		&cfg.Maintainer.Spv.Enabled,
		"spv",
		false,
		"Start SPV maintainer.",
	)

	command.Flags().Uint64Var(
		&cfg.Maintainer.Spv.HistoryDepth,
		"spv.historyDepth",
		spv.DefaultHistoryDepth,
		"Number of blocks to look back for past wallet-related events.",
	)

	command.Flags().IntVar(
		&cfg.Maintainer.Spv.TransactionLimit,
		"spv.transactionLimit",
		spv.DefaultTransactionLimit,
		"The maximum number of confirmed transactions returned when getting "+
			"transactions for a public key hash.",
	)

	command.Flags().DurationVar(
		&cfg.Maintainer.Spv.RestartBackoffTime,
		"spv.restartBackoffTime",
		spv.DefaultRestartBackoffTime,
		"The restart backoff which should be applied when the SPV maintainer "+
			"is restarted.",
	)

	command.Flags().DurationVar(
		&cfg.Maintainer.Spv.IdleBackoffTime,
		"spv.idleBackoffTime",
		spv.DefaultIdleBackOffTime,
		"The wait time which should be applied when there are no more "+
			"transaction proofs to submit.",
	)
}

// Initialize flags for Developer configuration.
func initDeveloperFlags(command *cobra.Command) {
	initContractAddressFlag := func(contractName string) {
		command.Flags().String(
			config.GetDeveloperContractAddressKey(contractName),
			"",
			fmt.Sprintf(
				"Address of the %s smart contract",
				contractName,
			),
		)
	}

	initContractAddressFlag(chainEthereum.BridgeContractName)
	initContractAddressFlag(chainEthereum.MaintainerProxyContractName)
	initContractAddressFlag(chainEthereum.LightRelayContractName)
	initContractAddressFlag(chainEthereum.LightRelayMaintainerProxyContractName)
	initContractAddressFlag(chainEthereum.RandomBeaconContractName)
	initContractAddressFlag(chainEthereum.TokenStakingContractName)
	initContractAddressFlag(chainEthereum.WalletRegistryContractName)
	initContractAddressFlag(chainEthereum.WalletProposalValidatorContractName)
}
