package tbtc

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/persistence"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/sortition"
)

// TODO: Unit tests for `tbtc.go`.

var logger = log.Logger("keep-tbtc")

// ProtocolName denotes the name of the protocol defined by this package.
const ProtocolName = "tbtc"

// GroupParameters is a structure grouping TBTC group parameters.
type GroupParameters struct {
	// GroupSize is the target size of a group in TBTC.
	GroupSize int
	// GroupQuorum is the minimum number of active participants behaving
	// according to the protocol needed to generate a group in TBTC. This value
	// is smaller than the GroupSize and bigger than the HonestThreshold.
	GroupQuorum int
	// HonestThreshold is the minimum number of active participants behaving
	// according to the protocol needed to generate a signature.
	HonestThreshold int
}

// DishonestThreshold is the maximum number of misbehaving participants for
// which it is still possible to generate a signature. Misbehaviour is any
// misconduct to the protocol, including inactivity.
func (gp *GroupParameters) DishonestThreshold() int {
	return gp.GroupSize - gp.HonestThreshold
}

// defaultGroupParameters returns network-based defaults: mainnet-style on
// Ethereum mainnet, and 3/3/2 on Sepolia / developer when no validator override
// is used. When EcdsaDkgValidator is configured in ethereum.ContractAddresses
// (developer.ecdsaDkgValidatorAddress alias), Initialize replaces these with
// values read from that contract on-chain (groupSize/activeThreshold/groupThreshold).
func defaultGroupParameters(n ethereum.Network) *GroupParameters {
	switch n {
	case ethereum.Sepolia, ethereum.Developer:
		logger.Warnf(
			"TBTC group parameters: testnet/small group (size=3, quorum=3, "+
				"honest=2) for %s; quorum equals size so all three operators "+
				"must remain online for DKG to progress. Configure an "+
				"EcdsaDkgValidator contract address to override these defaults "+
				"with on-chain values.",
			n,
		)
		return &GroupParameters{
			GroupSize:       3,
			GroupQuorum:     3,
			HonestThreshold: 2,
		}
	default:
		return &GroupParameters{
			GroupSize:       100,
			GroupQuorum:     90,
			HonestThreshold: 51,
		}
	}
}

const (
	DefaultPreParamsPoolSize              = 1000
	DefaultPreParamsGenerationTimeout     = 2 * time.Minute
	DefaultPreParamsGenerationDelay       = 10 * time.Second
	DefaultPreParamsGenerationConcurrency = 1
)

var DefaultKeyGenerationConcurrency = runtime.GOMAXPROCS(0)

// Config carries the config for tBTC protocol.
type Config struct {
	// The size of the pre-parameters pool for tECDSA.
	PreParamsPoolSize int
	// Timeout for pre-parameters generation for tECDSA.
	PreParamsGenerationTimeout time.Duration
	// The delay between generating new pre-params for tECDSA.
	PreParamsGenerationDelay time.Duration
	// Concurrency level for pre-parameters generation for tECDSA.
	PreParamsGenerationConcurrency int
	// Concurrency level for key-generation for tECDSA.
	KeyGenerationConcurrency int
	// Reservations gates the m1 reservation feature (acceptance, re-anchor,
	// stranding / stale / action-timeout watchers). When disabled the
	// coordination layer constructs without any reservation plumbing, so
	// non-reservation deployments stay side-effect free.
	Reservations ReservationsConfig
}

// ReservationsConfig holds the reservation-related tbtc.Config fields. It is
// a separate type so future reservation knobs (poll intervals, cap overrides)
// can be added without breaking the top-level Config layout.
//
// This flag controls BOTH reservation acceptance / re-anchor proposal
// GENERATION AND watcher wiring in the start process (the `start`
// command's coordination and watcher layers, config category Tbtc). The
// `maintainer` command runs as a separate process reading a
// disjoint config category (see config.MaintainerCategories) and has its
// own independent gate, spv.ReservationsConfig.Enabled, that controls SPV
// PROOF SUBMISSION for those same proposals. Neither command's config
// loading sees the other's category, so this flag cannot be derived from or
// validated against spv.ReservationsConfig.Enabled in code. An operator
// running both `start` and `maintainer` for the reservation feature to work
// end-to-end MUST enable both flags - normally the same [Tbtc.Reservations]
// / [Maintainer.Spv.Reservations] TOML sections in one shared config file.
type ReservationsConfig struct {
	// Enabled toggles reservation acceptance / re-anchor proposal
	// generation and reservation watcher wiring. Defaults to false so
	// existing deployments opt in explicitly.
	Enabled bool
}

// WalletMembersResolver defines the interface for resolving wallet members.
type WalletMembersResolver interface {
	ResolveWalletMembers(walletPublicKeyHash [20]byte) ([]uint32, error)
}

// Initialize kicks off the TBTC by initializing internal state, ensuring
// preconditions like staking are met, and then kicking off the internal TBTC
// implementation. Returns an error if this failed.
//
// Reservation watcher wiring (stranding / stale-deposit / action-timeout,
// see pkg/maintainer/spv.WireReservationWatchers) is not performed here:
// it lives in cmd/start.go, called directly against the same tbtc.Chain
// handle once Initialize returns successfully and gated on the same
// config.Reservations.Enabled flag. Threading it through Initialize via a
// callback type would only exist to dodge a tbtc -> spv import cycle that
// cmd/start.go (which already imports both packages) does not have.

func Initialize(
	ctx context.Context,
	chain Chain,
	btcChain bitcoin.Chain,
	netProvider net.Provider,
	keyStorePersistence persistence.ProtectedHandle,
	workPersistence persistence.BasicHandle,
	scheduler *generator.Scheduler,
	proposalGenerator CoordinationProposalGenerator,
	config Config,
	clientInfo *clientinfo.Registry,
	perfMetrics *clientinfo.PerformanceMetrics,
	ethereumNetwork ethereum.Network,
) (WalletMembersResolver, error) {
	groupParameters := defaultGroupParameters(ethereumNetwork)

	if ethChain, ok := chain.(interface {
		EcdsaWalletGroupParametersFromChain(context.Context) (*GroupParameters, error)
	}); ok {
		gp, err := ethChain.EcdsaWalletGroupParametersFromChain(ctx)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot read TBTC group sizing from ECDSA validator: [%w]",
				err,
			)
		}
		if gp != nil {
			groupParameters = gp
			logger.Infof(
				"TBTC ECDSA group parameters from validator contract (size=[%v] "+
					"groupQuorum/activeThreshold=[%v] "+
					"honestThreshold/groupThreshold=[%v]); overrides network defaults",
				gp.GroupSize,
				gp.GroupQuorum,
				gp.HonestThreshold,
			)
		}
	}

	node, err := newNode(
		groupParameters,
		chain,
		btcChain,
		netProvider,
		keyStorePersistence,
		workPersistence,
		scheduler,
		proposalGenerator,
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot set up TBTC node: [%v]", err)
	}

	err = node.runCoordinationLayer(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot run coordination layer: [%w]", err)
	}

	deduplicator := newDeduplicator()

	if clientInfo != nil {
		// only if client info endpoint is configured
		clientInfo.ObserveApplicationSource(
			"tbtc",
			map[string]clientinfo.Source{
				"pre_params_count": func() float64 {
					return float64(node.dkgExecutor.preParamsCount())
				},
			},
		)

		if perfMetrics == nil {
			perfMetrics = clientinfo.NewPerformanceMetrics(
				ctx,
				clientInfo,
				config.Reservations.Enabled,
			)
		}
		node.setPerformanceMetrics(perfMetrics)

		// Register coordination windows as a diagnostic source
		clientInfo.RegisterApplicationSource(
			"coordination_windows",
			func() clientinfo.ApplicationInfo {
				summary := node.GetCoordinationWindowsSummary()
				if summary == nil {
					return clientinfo.ApplicationInfo{}
				}
				return clientinfo.ApplicationInfo{
					"total_windows":             summary.TotalWindows,
					"total_wallets_coordinated": summary.TotalWalletsCoordinated,
					"total_wallets_successful":  summary.TotalWalletsSuccessful,
					"total_wallets_failed":      summary.TotalWalletsFailed,
					"total_faults":              summary.TotalFaults,
					"windows":                   summary.Windows,
				}
			},
		)
	}

	err = sortition.MonitorPool(
		ctx,
		logger,
		chain,
		sortition.DefaultStatusCheckTick,
		sortition.NewConjunctionPolicy(
			sortition.NewBetaOperatorPolicy(chain, logger),
			&enoughPreParamsInPoolPolicy{
				node:   node,
				config: config,
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not set up sortition pool monitoring: [%v]",
			err,
		)
	}

	_ = chain.OnDKGStarted(func(event *DKGStartedEvent) {
		go func() {
			if ok := deduplicator.notifyDKGStarted(
				event.Seed,
			); !ok {
				logger.Infof(
					"DKG started event with seed [0x%x] has been "+
						"already processed",
					event.Seed,
				)
				return
			}

			confirmationBlock := event.BlockNumber + dkgStartedConfirmationBlocks

			logger.Infof(
				"observed DKG started event with seed [0x%x] and "+
					"starting block [%v]; waiting for block [%v] to confirm",
				event.Seed,
				event.BlockNumber,
				confirmationBlock,
			)

			err := node.waitForBlockHeight(ctx, confirmationBlock)
			if err != nil {
				logger.Errorf("failed to confirm DKG started event: [%v]", err)
				return
			}

			dkgState, err := chain.GetDKGState()
			if err != nil {
				logger.Errorf("failed to check DKG state: [%v]", err)
				return
			}

			if dkgState == AwaitingResult {
				// Fetch all past DKG started events starting from one
				// confirmation period before the original event's block.
				// If there was a chain reorg, the event we received could be
				// moved to a block with a lower number than the one
				// we received.
				pastEvents, err := chain.PastDKGStartedEvents(
					&DKGStartedEventFilter{
						StartBlock: event.BlockNumber - dkgStartedConfirmationBlocks,
					},
				)
				if err != nil {
					logger.Errorf("failed to get past DKG started events: [%v]", err)
					return
				}

				// Should not happen but just in case.
				if len(pastEvents) == 0 {
					logger.Errorf("no past DKG started events")
					return
				}

				lastEvent := pastEvents[len(pastEvents)-1]

				logger.Infof(
					"DKG started with seed [0x%x] at block [%v]",
					lastEvent.Seed,
					lastEvent.BlockNumber,
				)

				// The off-chain protocol should be started as close as possible
				// to the current block or even further. Starting the off-chain
				// protocol with a past block will likely cause a failure of the
				// first attempt as the start block is used to synchronize
				// the announcements and the state machine. Here we ensure
				// a proper start point by delaying the execution by the
				// confirmation period length.
				node.joinDKGIfEligible(
					lastEvent.Seed,
					lastEvent.BlockNumber,
					dkgStartedConfirmationBlocks,
				)
			} else {
				logger.Infof(
					"DKG started event with seed [0x%x] and starting "+
						"block [%v] was not confirmed",
					event.Seed,
					event.BlockNumber,
				)
			}
		}()
	})

	_ = chain.OnDKGResultSubmitted(func(event *DKGResultSubmittedEvent) {
		go func() {
			if ok := deduplicator.notifyDKGResultSubmitted(
				event.Seed,
				event.ResultHash,
				event.BlockNumber,
			); !ok {
				logger.Warnf(
					"Result with hash [0x%x] for DKG with seed [0x%x] "+
						"and starting block [%v] has been already processed",
					event.ResultHash,
					event.Seed,
					event.BlockNumber,
				)
				return
			}

			logger.Infof(
				"Result with hash [0x%x] for DKG with seed [0x%x] "+
					"submitted at block [%v]",
				event.ResultHash,
				event.Seed,
				event.BlockNumber,
			)

			node.validateDKG(
				event.Seed,
				event.BlockNumber,
				event.Result,
				event.ResultHash,
			)
		}()
	})

	_ = chain.OnWalletClosed(func(event *WalletClosedEvent) {
		go func() {
			if ok := deduplicator.notifyWalletClosed(
				event.WalletID,
			); !ok {
				logger.Warnf(
					"Wallet closure for wallet with ID [0x%x] at block [%v] "+
						"has been already processed",
					event.WalletID,
					event.BlockNumber,
				)
				return
			}

			logger.Infof(
				"Wallet with ID [0x%x] has been closed at block [%v]; "+
					"proceeding with handling wallet closure",
				event.WalletID,
				event.BlockNumber,
			)

			err := node.handleWalletClosure(
				event.WalletID,
			)
			if err != nil {
				logger.Errorf(
					"Failure while handling wallet closure with ID [0x%x]: [%v]",
					event.WalletID,
					err,
				)
			}
		}()
	})

	return node, nil
}

// enoughPreParamsInPoolPolicy is a policy that enforces the sufficient size
// of the DKG pre-parameters pool before joining the sortition pool.
type enoughPreParamsInPoolPolicy struct {
	node   *node
	config Config
}

func (eppip *enoughPreParamsInPoolPolicy) ShouldJoin() bool {
	paramsInPool := eppip.node.dkgExecutor.preParamsCount()
	poolSize := eppip.config.PreParamsPoolSize
	return paramsInPool >= poolSize
}
