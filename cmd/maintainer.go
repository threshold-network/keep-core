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

// maintainers initializes maintainer tasks specified by flags passed to the
// maintainer command.
func maintainers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

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

	maintainer.Initialize(
		ctx,
		clientConfig.Maintainer,
		btcChain,
		btcDiffChain,
		tbtcChain,
		metricsRecorder,
	)

	<-ctx.Done()
	return fmt.Errorf("unexpected context cancellation")
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

	perfMetrics := clientinfo.NewPerformanceMetrics(ctx, registry)

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
