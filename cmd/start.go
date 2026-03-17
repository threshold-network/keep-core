package cmd

import (
	"context"
	"fmt"
	"time"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/tbtcpg"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/build"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/storage"

	"github.com/spf13/cobra"

	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/beacon"
	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/firewall"
	"github.com/keep-network/keep-core/pkg/generator"
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

type startDeps struct {
	connectEthereum func(
		ctx context.Context,
		config commonEthereum.Config,
	) (
		*ethereum.BeaconChain,
		*ethereum.TbtcChain,
		chain.BlockCounter,
		chain.Signing,
		*operator.PrivateKey,
		error,
	)
	connectElectrum func(
		ctx context.Context,
		config electrum.Config,
	) (bitcoin.Chain, error)
	initializeNetwork func(
		ctx context.Context,
		applications []firewall.Application,
		operatorPrivateKey *operator.PrivateKey,
		blockCounter chain.BlockCounter,
	) (net.Provider, error)
	initializePersistence func() (
		beaconKeyStorePersistence persistence.ProtectedHandle,
		tbtcKeyStorePersistence persistence.ProtectedHandle,
		tbtcDataPersistence persistence.BasicHandle,
		err error,
	)
	initializeBeacon func(
		ctx context.Context,
		chain beaconchain.Interface,
		netProvider net.Provider,
		keyStorePersistence persistence.ProtectedHandle,
		scheduler *generator.Scheduler,
	) error
	initializeTbtc func(
		ctx context.Context,
		chain tbtc.Chain,
		btcChain bitcoin.Chain,
		netProvider net.Provider,
		keyStorePersistance persistence.ProtectedHandle,
		workPersistence persistence.BasicHandle,
		scheduler *generator.Scheduler,
		proposalGenerator tbtc.CoordinationProposalGenerator,
		config tbtc.Config,
		clientInfoRegistry *clientinfo.Registry,
		perfMetrics *clientinfo.PerformanceMetrics,
	) (covenantsigner.Engine, error)
	initializeSigner func(
		ctx context.Context,
		config covenantsigner.Config,
		handle persistence.BasicHandle,
		engine covenantsigner.Engine,
	) (*covenantsigner.Server, bool, error)
	startScheduler func() *generator.Scheduler
}

func defaultStartDeps() startDeps {
	return startDeps{
		connectEthereum:       ethereum.Connect,
		connectElectrum:       electrum.Connect,
		initializeNetwork:     initializeNetwork,
		initializePersistence: initializePersistence,
		initializeBeacon:      beacon.Initialize,
		initializeTbtc:        tbtc.Initialize,
		initializeSigner:      covenantsigner.Initialize,
		startScheduler:        generator.StartScheduler,
	}
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
	return startWithDeps(cmd, defaultStartDeps())
}

func startWithDeps(cmd *cobra.Command, deps startDeps) error {
	ctx := context.Background()

	beaconChain, tbtcChain, blockCounter, signing, operatorPrivateKey, err :=
		deps.connectEthereum(ctx, clientConfig.Ethereum)
	if err != nil {
		return fmt.Errorf("error connecting to Ethereum node: [%v]", err)
	}

	netProvider, err := deps.initializeNetwork(
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
		perfMetrics = clientinfo.NewPerformanceMetrics(ctx, clientInfoRegistry)
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
	}

	// Initialize beacon and tbtc only for non-bootstrap nodes.
	// Skip initialization for bootstrap nodes as they are only used for network
	// discovery.
	if !isBootstrap() {
		btcChain, err := deps.connectElectrum(ctx, clientConfig.Bitcoin.Electrum)
		if err != nil {
			return fmt.Errorf("could not connect to Electrum chain: [%v]", err)
		}

		beaconKeyStorePersistence,
			tbtcKeyStorePersistence,
			tbtcDataPersistence,
			err := deps.initializePersistence()
		if err != nil {
			return fmt.Errorf("cannot initialize persistence: [%w]", err)
		}

		scheduler := deps.startScheduler()

		if clientInfoRegistry != nil {
			clientInfoRegistry.ObserveBtcConnectivity(
				btcChain,
				clientConfig.ClientInfo.BitcoinMetricsTick,
			)

			clientInfoRegistry.RegisterBtcChainInfoSource(btcChain)

			rpcHealthChecker := clientinfo.NewRPCHealthChecker(
				clientInfoRegistry,
				blockCounter,
				btcChain,
				clientConfig.ClientInfo.RPCHealthCheckInterval,
			)
			rpcHealthChecker.Start(ctx)
		}

		err = deps.initializeBeacon(
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
		)

		covenantSignerEngine, err := deps.initializeTbtc(
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
		)
		if err != nil {
			return fmt.Errorf("error initializing TBTC: [%v]", err)
		}

		_, _, err = deps.initializeSigner(
			ctx,
			clientConfig.CovenantSigner,
			tbtcDataPersistence,
			covenantSignerEngine,
		)
		if err != nil {
			return fmt.Errorf("error initializing covenant signer: [%v]", err)
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
	return clientConfig.LibP2P.Bootstrap
}

func initializeNetwork(
	ctx context.Context,
	applications []firewall.Application,
	operatorPrivateKey *operator.PrivateKey,
	blockCounter chain.BlockCounter,
) (net.Provider, error) {
	bootstrapPeersPublicKeys, err := libp2p.ExtractPeersPublicKeys(
		clientConfig.LibP2P.Peers,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error extracting bootstrap peers public keys: [%v]",
			err,
		)
	}

	firewall := firewall.AnyApplicationPolicy(
		applications,
		firewall.NewAllowList(bootstrapPeersPublicKeys),
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
	registry, isConfigured := clientinfo.Initialize(ctx, config.ClientInfo.Port)
	if !isConfigured {
		logger.Infof("client info endpoint not configured")
		return nil
	}

	registry.ObserveConnectedPeersCount(
		netProvider,
		config.ClientInfo.NetworkMetricsTick,
	)

	registry.ObserveConnectedBootstrapCount(
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
