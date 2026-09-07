package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtcpg"

	"github.com/keep-network/keep-common/pkg/persistence"

	"github.com/keep-network/keep-core/build"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/storage"

	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/beacon"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/maintainer/spv"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// StartCommand contains the definition of the start command-line subcommand.
var StartCommand = &cobra.Command{
	Use:   "start",
	Short: "Starts the Keep Client",
	Long:  "Starts the Keep Client in the foreground",
	PreRun: func(cmd *cobra.Command, args []string) {
		if err := clientConfig.ReadConfig(configFilePath, cmd.Flags(), config.StartCmdCategories...); err != nil {
			logger.Fatalf("error reading config: %v", err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := start(cmd); err != nil {
			logger.Fatal(err)
		}
	},
}

func init() {
	initFlags(StartCommand, &configFilePath, clientConfig, config.StartCmdCategories...)

	StartCommand.SetUsageTemplate(
		fmt.Sprintf(`%s
Environment variables:
    %s    Password for Keep operator account keyfile decryption.
    %s                 Space-delimited set of log level directives; set to "help" for help.
`,
			StartCommand.UsageString(),
			config.EthereumPasswordEnvVariable,
			config.LogLevelEnvVariable,
		),
	)
}

// start starts a node
func start(cmd *cobra.Command) error {
	ctx := context.Background()

	beaconChain, tbtcChain, blockCounter, signing, operatorPrivateKey, err :=
		ethereum.Connect(ctx, clientConfig.Ethereum)
	if err != nil {
		return fmt.Errorf("error connecting to Ethereum node: [%v]", err)
	}

	netProvider, err := initializeNetwork(
		ctx,
		[]firewall.Application{beaconChain, tbtcChain},
		operatorPrivateKey,
		blockCounter,
	)
	if err != nil {
		return fmt.Errorf("cannot initialize network: [%v]", err)
	}

	clientInfoRegistry := initializeClientInfo(
		ctx,
		clientConfig,
		netProvider,
		signing,
		blockCounter,
	)

	// Wire performance metrics into network provider if available
	var perfMetrics *clientinfo.PerformanceMetrics
	if clientInfoRegistry != nil {
		perfMetrics = clientinfo.NewPerformanceMetrics(
			ctx,
			clientInfoRegistry,
			clientConfig.Tbtc.Reservations.LeaderDutiesEnabled,
		)
		// Type assert to libp2p provider to set metrics recorder
		// The provider struct is not exported, so we use interface assertion
		if setter, ok := netProvider.(interface {
			SetMetricsRecorder(recorder interface {
				IncrementCounter(name string, value float64)
				SetGauge(name string, value float64)
				RecordDuration(name string, duration time.Duration)
			})
		}); ok {
			setter.SetMetricsRecorder(perfMetrics)
		}

		// Wire performance metrics into firewall validation so live
		// on-chain IsRecognized calls are counted.
		firewall.SetMetricsRecorder(perfMetrics)
	}

	// Initialize beacon and tbtc only for non-bootstrap nodes.
	// Skip initialization for bootstrap nodes as they are only used for network
	// discovery.
	if !isBootstrap() {
		btcChain, err := electrum.Connect(ctx, clientConfig.Bitcoin.Electrum)
		if err != nil {
			return fmt.Errorf("could not connect to Electrum chain: [%v]", err)
		}

		beaconKeyStorePersistence,
			tbtcKeyStorePersistence,
			tbtcDataPersistence,
			err := initializePersistence()
		if err != nil {
			return fmt.Errorf("cannot initialize persistence: [%w]", err)
		}

		scheduler := generator.StartScheduler()

		if clientInfoRegistry != nil {
			clientInfoRegistry.ObserveBtcConnectivity(
				btcChain,
				clientConfig.ClientInfo.BitcoinMetricsTick,
			)

			clientInfoRegistry.RegisterBtcChainInfoSource(btcChain)

			rpcHealthChecker := clientinfo.NewRPCHealthChecker(
				clientInfoRegistry,
				tbtcChain,
				btcChain,
				clientConfig.ClientInfo.RPCHealthCheckInterval,
			)
			// An unavailable RPC must not delay starting the node's
			// initialization (beacon and tbtc).
			go rpcHealthChecker.Start(ctx)
		}

		err = beacon.Initialize(
			ctx,
			beaconChain,
			netProvider,
			beaconKeyStorePersistence,
			scheduler,
		)
		if err != nil {
			return fmt.Errorf("error initializing beacon: [%v]", err)
		}

		proposalGenerator := tbtcpg.NewProposalGenerator(
			tbtcChain,
			btcChain,
			clientConfig.Tbtc.Reservations.LeaderDutiesEnabled,
		)

		err = tbtc.Initialize(
			ctx,
			tbtcChain,
			btcChain,
			netProvider,
			tbtcKeyStorePersistence,
			tbtcDataPersistence,
			scheduler,
			proposalGenerator,
			clientConfig.Tbtc,
			clientInfoRegistry,
			perfMetrics, // Pass the existing performance metrics instance to avoid duplicate registrations
			clientConfig.Ethereum.Network,
		)
		if err != nil {
			return fmt.Errorf("cannot initialize TBTC: [%v]", err)
		}

		// Wire the reservation watchers (stranding, stale-deposit,
		// action-timeout) directly against the same tbtcChain handle:
		// cmd/start.go already imports both tbtc and spv, so there is no
		// import-cycle reason to thread this through tbtc.Initialize via a
		// callback type. Gated on the same flag that gates the reservation
		// proposal generator tasks above. Failing to wire the watchers is
		// fatal: the operator opted into reservations, so a missing
		// watcher would silently strand anchors.
		//
		// The paired-flag check below is a warning, not a hard error,
		// deliberately: config.ReadConfig unmarshals the whole config
		// file into the whole Config struct regardless of which
		// categories a command declares (categories only gate required-
		// field validation and which CLI flags get registered) - so
		// Maintainer.Spv.Reservations.LeaderDutiesEnabled IS populated
		// here whenever the start process's own config file happens to
		// contain a [maintainer.spv.reservations] section. But it CANNOT
		// be set via a start command-line flag at all (StartCmdCategories
		// excludes Maintainer, so no such flag is registered), and a
		// legitimate split deployment's start-process config file has no
		// reason to include a section start never otherwise reads - so
		// this field reading as its zero value here does not reliably
		// mean the maintainer process actually has it disabled. A hard
		// error at this call site would risk rejecting a valid split
		// deployment's config outright; only a shared config file read by
		// both processes can be checked reliably from here.
		tbtcReservationsEnabled := clientConfig.Tbtc.Reservations.LeaderDutiesEnabled
		if tbtcReservationsEnabled {
			if !clientConfig.Maintainer.Spv.Reservations.LeaderDutiesEnabled {
				logger.Warnf("Client reservation proposal generation is enabled; " +
					"ensure the paired Maintainer.Spv.Reservations.LeaderDutiesEnabled flag is also " +
					"enabled in the maintainer config for end-to-end operation")
			}
			if err := spv.WireReservationWatchers(
				ctx,
				tbtcChain,
				tbtcChain,
			); err != nil {
				return fmt.Errorf(
					"failed to wire reservation watchers: [%v]",
					err,
				)
			}
		}
	}

	nodeHeader(
		netProvider.ConnectionManager().AddrStrings(),
		beaconChain.Signing().Address().String(),
		clientConfig.LibP2P.Port,
		clientConfig.Ethereum,
	)

	<-ctx.Done()
	return fmt.Errorf("shutting down the node because its context has ended")
}

func isBootstrap() bool {
	if clientConfig.LibP2P.Bootstrap {
		logger.Warnf("--network.bootstrap is deprecated and will be removed in a future release")
	}
	return clientConfig.LibP2P.Bootstrap
}

func initializeNetwork(
	ctx context.Context,
	applications []firewall.Application,
	operatorPrivateKey *operator.PrivateKey,
	blockCounter chain.BlockCounter,
) (net.Provider, error) {
	firewall := firewall.AnyApplicationPolicy(
		applications,
		firewall.EmptyAllowList(),
	)

	netProvider, err := libp2p.Connect(
		ctx,
		clientConfig.LibP2P,
		operatorPrivateKey,
		firewall,
		retransmission.NewTicker(blockCounter.WatchBlocks(ctx)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed while creating the network provider: [%v]", err)
	}

	return netProvider, nil
}

func initializeClientInfo(
	ctx context.Context,
	config *config.Config,
	netProvider net.Provider,
	signing chain.Signing,
	blockCounter chain.BlockCounter,
) *clientinfo.Registry {
	registry, isConfigured := clientinfo.Initialize(ctx, config.ClientInfo)
	if !isConfigured {
		logger.Infof("client info endpoint not configured")
		return nil
	}

	registry.ObserveConnectedPeersCount(
		netProvider,
		config.ClientInfo.NetworkMetricsTick,
	)

	registry.ObserveConnectedWellknownPeersCount(
		netProvider,
		config.LibP2P.Peers,
		config.ClientInfo.NetworkMetricsTick,
	)

	registry.ObserveEthConnectivity(
		blockCounter,
		config.ClientInfo.EthereumMetricsTick,
	)

	registry.RegisterMetricClientInfo(build.Version)

	registry.RegisterConnectedPeersSource(netProvider, signing)

	registry.RegisterClientInfoSource(
		netProvider,
		signing,
		build.Version,
		build.Revision,
	)

	registry.RegisterEthChainInfoSource(blockCounter)

	logger.Infof(
		"enabled client info endpoint on port [%v]",
		config.ClientInfo.Port,
	)

	return registry
}

func initializePersistence() (
	beaconKeyStorePersistence persistence.ProtectedHandle,
	tbtcKeyStorePersistence persistence.ProtectedHandle,
	tbtcDataPersistence persistence.BasicHandle,
	err error,
) {
	storage, err := storage.Initialize(
		clientConfig.Storage,
		clientConfig.Ethereum.KeyFilePassword,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot initialize storage: [%w]", err)
	}

	beaconKeyStorePersistence, err = storage.InitializeKeyStorePersistence(
		"beacon",
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"cannot initialize beacon keystore persistence: [%w]",
			err,
		)
	}

	tbtcKeyStorePersistence, err = storage.InitializeKeyStorePersistence(
		"tbtc",
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"cannot initialize tbtc keystore persistence: [%w]",
			err,
		)
	}

	tbtcDataPersistence, err = storage.InitializeWorkPersistence("tbtc")
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"cannot initialize tbtc data persistence: [%w]",
			err,
		)
	}

	return
}
