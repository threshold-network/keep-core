package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/build"
	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/maintainer"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
)

// MaintainerCommand contains the definition of the maintainer command-line
// subcommand.
var MaintainerCommand = &cobra.Command{
	Use:   "maintainer",
	Short: `(experimental) Starts maintainers`,
	Long:  `(experimental) The maintainer command starts maintainers`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if err := clientConfig.ReadConfig(
			configFilePath,
			cmd.Flags(),
			config.MaintainerCategories...,
		); err != nil {
			logger.Fatalf("error reading config: %v", err)
		}
	},
	RunE: maintainers,
}

func init() {
	initFlags(
		MaintainerCommand,
		&configFilePath,
		clientConfig,
		config.MaintainerCategories...,
	)
}

// validateMaintainerConfig checks the maintainer configuration before any
// chain connection is attempted, so a misconfiguration fails fast and loudly
// at startup instead of degrading into silent runtime behavior. It delegates
// to maintainer.Config.Validate so the command-level check and the
// config-load check (config.ReadConfig) share one rule: SPV settings are
// validated only when the SPV maintainer will actually run (explicitly
// enabled, or neither maintainer enabled).
func validateMaintainerConfig(cfg *config.Config) error {
	if err := cfg.Maintainer.Validate(); err != nil {
		return fmt.Errorf("invalid maintainer configuration: [%v]", err)
	}

	return nil
}

// maintainers initializes maintainer tasks specified by flags passed to the
// maintainer command.
func maintainers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if err := validateMaintainerConfig(clientConfig); err != nil {
		return err
	}

	btcChain, err := electrum.Connect(ctx, clientConfig.Bitcoin.Electrum)
	if err != nil {
		return fmt.Errorf("could not connect to Electrum chain: [%v]", err)
	}

	btcDiffChain, err := ethereum.ConnectBitcoinDifficulty(
		ctx,
		clientConfig.Ethereum,
		clientConfig.Maintainer,
	)
	if err != nil {
		return fmt.Errorf(
			"could not connect to Bitcoin difficulty chain: [%v]",
			err,
		)
	}

	_, tbtcChain, blockCounter, _, _, err := ethereum.Connect(
		ctx,
		clientConfig.Ethereum,
	)
	if err != nil {
		return fmt.Errorf(
			"could not connect to tBTC chain: [%v]",
			err,
		)
	}

	metricsRecorder := initializeMaintainerMetrics(ctx, blockCounter, tbtcChain, btcChain)

	err = maintainer.Initialize(
		ctx,
		maintainerConfig(clientConfig),
		btcChain,
		btcDiffChain,
		tbtcChain,
		metricsRecorder,
	)
	if err != nil {
		return fmt.Errorf("could not initialize maintainer tasks: [%v]", err)
	}

	<-ctx.Done()
	return fmt.Errorf("unexpected context cancellation")
}

// maintainerConfig returns the maintainers' config with the SPV
// maintainer's Ethereum network copied from the [ethereum] section. The
// SPV maintainer looks up the reservation activation block by that
// network; left unset, the network is Unknown and every reservation
// catch-up scan is skipped.
func maintainerConfig(cfg *config.Config) maintainer.Config {
	maintainerConfig := cfg.Maintainer
	maintainerConfig.Spv.EthereumNetwork = cfg.Ethereum.Network
	return maintainerConfig
}

// initializeMaintainerMetrics sets up the client info registry and performance
// metrics for the maintainer command. It returns a metrics recorder wired to
// the SPV maintainer, or nil when the client info endpoint is not configured
// (in which case metrics recording is disabled).
func initializeMaintainerMetrics(
	ctx context.Context,
	blockCounter chain.BlockCounter,
	ethRPC clientinfo.EthereumRPC,
	btcChain bitcoin.Chain,
) spv.MetricsRecorder {
	registry, isConfigured := clientinfo.Initialize(
		ctx,
		clientConfig.ClientInfo,
	)
	if !isConfigured {
		logger.Infof("client info endpoint not configured")
		return nil
	}

	// A maintainer run only for reservation proofs (for example an
	// independent keeper without an operator client) has the client flag
	// off but still hosts the reservation watchers, so either flag turns
	// the reservation gauges on. Without registration SetGauge drops
	// every value.
	reservationGaugesEnabled := clientConfig.Tbtc.ReservationsEnabled ||
		clientConfig.Maintainer.Spv.ReservationProofsEnabled
	perfMetrics := clientinfo.NewPerformanceMetrics(ctx, registry, reservationGaugesEnabled)

	registry.RegisterMetricClientInfo(build.Version)
	registry.ObserveEthConnectivity(
		blockCounter,
		clientConfig.ClientInfo.EthereumMetricsTick,
	)
	registry.RegisterEthChainInfoSource(blockCounter)
	registry.ObserveBtcConnectivity(btcChain, clientConfig.ClientInfo.BitcoinMetricsTick)
	registry.RegisterBtcChainInfoSource(btcChain)
	healthChecker := clientinfo.NewRPCHealthChecker(
		registry, ethRPC, btcChain, clientConfig.ClientInfo.RPCHealthCheckInterval,
	)
	// An unavailable RPC must not delay starting the maintainer's control loop.
	go healthChecker.Start(ctx)

	logger.Infof(
		"enabled client info endpoint on port [%v]",
		clientConfig.ClientInfo.Port,
	)

	return perfMetrics
}
