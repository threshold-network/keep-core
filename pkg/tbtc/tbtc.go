package tbtc

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"

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
// implementation. Returns the covenant signer engine bound to the initialized
// node together with an error if initialization failed.
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
	minActiveOutpointConfirmations uint,
	bridgeCovenantFraudDefenseConfirmed bool,
	eip712ChainID uint64,
	eip712Salt [32]byte,
) (covenantsigner.Engine, error) {
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
			perfMetrics = clientinfo.NewPerformanceMetrics(ctx, clientInfo)
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
			// handleDKGStartedEvent records the deduplication entry as completed
			// only once the local DKG join has been dispatched (or the event was
			// authoritatively found unconfirmed), so a transient early return in
			// the handler below leaves the event retryable on redelivery.
			handleDKGStartedEvent(
				deduplicator,
				event,
				func(event *DKGStartedEvent) error {
					confirmationBlock := event.BlockNumber + dkgStartedConfirmationBlocks

					logger.Infof(
						"observed DKG started event with seed [0x%x] and "+
							"starting block [%v]; waiting for block [%v] to confirm",
						event.Seed,
						event.BlockNumber,
						confirmationBlock,
					)

					if err := node.waitForBlockHeight(ctx, confirmationBlock); err != nil {
						return fmt.Errorf(
							"failed to confirm DKG started event: [%w]",
							err,
						)
					}

					dkgState, err := chain.GetDKGState()
					if err != nil {
						return fmt.Errorf("failed to check DKG state: [%w]", err)
					}

					if dkgState != AwaitingResult {
						logger.Infof(
							"DKG started event with seed [0x%x] and starting "+
								"block [%v] was not confirmed",
							event.Seed,
							event.BlockNumber,
						)

						// The event was authoritatively determined to be
						// unconfirmed; there is nothing to retry, so treat it as
						// terminally handled.
						return nil
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
						return fmt.Errorf(
							"failed to get past DKG started events: [%w]",
							err,
						)
					}

					// Should not happen but just in case.
					if len(pastEvents) == 0 {
						return fmt.Errorf("no past DKG started events")
					}

					lastEvent := pastEvents[len(pastEvents)-1]

					logger.Infof(
						"DKG started with seed [0x%x] at block [%v]",
						lastEvent.Seed,
						lastEvent.BlockNumber,
					)

					// The off-chain protocol should be started as close as
					// possible to the current block or even further. Starting the
					// off-chain protocol with a past block will likely cause a
					// failure of the first attempt as the start block is used to
					// synchronize the announcements and the state machine. Here we
					// ensure a proper start point by delaying the execution by the
					// confirmation period length.
					node.joinDKGIfEligible(
						lastEvent.Seed,
						lastEvent.BlockNumber,
						dkgStartedConfirmationBlocks,
					)

					// The local DKG join has been dispatched; the event is
					// terminally handled.
					return nil
				},
			)
		}()
	})

	_ = chain.OnDKGResultSubmitted(func(event *DKGResultSubmittedEvent) {
		go func() {
			// handleDKGResultSubmittedEvent records the deduplication entry as
			// completed only after validation reaches a terminal state, so a
			// transient failure below leaves the event retryable on redelivery.
			handleDKGResultSubmittedEvent(
				deduplicator,
				event,
				func(event *DKGResultSubmittedEvent) error {
					return node.validateDKG(
						event.Seed,
						event.BlockNumber,
						event.Result,
						event.ResultHash,
					)
				},
			)
		}()
	})

	_ = chain.OnWalletClosed(func(event *WalletClosedEvent) {
		go func() {
			// handleWalletClosedEvent records the deduplication entry as
			// completed only after the wallet has actually been archived, so a
			// transient archival failure below leaves the event retryable on
			// redelivery.
			handleWalletClosedEvent(
				deduplicator,
				event,
				func(event *WalletClosedEvent) error {
					return node.handleWalletClosure(event.WalletID)
				},
			)
		}()
	})

	return newCovenantSignerEngine(
		node,
		minActiveOutpointConfirmations,
		bridgeCovenantFraudDefenseConfirmed,
		eip712ChainID,
		eip712Salt,
	), nil
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
