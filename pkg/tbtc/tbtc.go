package tbtc

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/ipfs/go-log"

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
}

// Initialize kicks off the TBTC by initializing internal state, ensuring
// preconditions like staking are met, and then kicking off the internal TBTC
// implementation. Returns an error if this failed.
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
) error {
	groupParameters := &GroupParameters{
		GroupSize:       100,
		GroupQuorum:     90,
		HonestThreshold: 51,
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
		return fmt.Errorf("cannot set up TBTC node: [%v]", err)
	}

	err = node.runCoordinationLayer(ctx)
	if err != nil {
		return fmt.Errorf("cannot run coordination layer: [%w]", err)
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

		// Initialize performance metrics
		perfMetrics := clientinfo.NewPerformanceMetrics(clientInfo)
		node.setPerformanceMetrics(perfMetrics)
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
		return fmt.Errorf(
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
				// Increment DKG requested metric
				if node.performanceMetrics != nil {
					node.performanceMetrics.IncrementCounter(clientinfo.MetricDKGRequestedTotal, 1)
				}
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

	return nil
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
