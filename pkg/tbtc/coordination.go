package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/keep-network/keep-core/pkg/internal/pb"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"golang.org/x/sync/semaphore"
)

const (
	// coordinationFrequencyBlocks is the number of blocks between two
	// consecutive coordination windows.
	coordinationFrequencyBlocks = 900
	// coordinationActivePhaseDurationBlocks is the number of blocks in the
	// active phase of the coordination window. The active phase is the
	// phase during which the communication between the coordination leader and
	// their followers is allowed.
	coordinationActivePhaseDurationBlocks = 80
	// coordinationPassivePhaseDurationBlocks is the number of blocks in the
	// passive phase of the coordination window. The passive phase is the
	// phase during which communication is not allowed. Participants are
	// expected to validate the result of the coordination and prepare for
	// execution of the proposed wallet action.
	coordinationPassivePhaseDurationBlocks = 20
	// coordinationDurationBlocks is the number of blocks in a single
	// coordination window.
	coordinationDurationBlocks = coordinationActivePhaseDurationBlocks +
		coordinationPassivePhaseDurationBlocks
	// coordinationSafeBlockShift is the number of blocks by which the
	// coordination block is shifted to obtain a safe block whose 32-byte
	// hash can be used as an ingredient for the coordination seed, computed
	// for the given coordination window.
	coordinationSafeBlockShift = 32
	// coordinationHeartbeatProbability is the probability of proposing a
	// heartbeat action during the coordination procedure, assuming no other
	// higher-priority action is proposed.
	coordinationHeartbeatProbability = float64(0.0625)
	// coordinationMessageReceiveBuffer is a buffer for messages received from
	// the broadcast channel needed when the coordination follower is
	// temporarily too slow to handle them. Keep in mind that although we
	// expect only 1 coordination message, it may happen that the follower
	// receives retransmissions of messages from the coordination protocol,
	// and before they are filtered out as not interesting for the follower,
	// they are buffered in the channel.
	coordinationMessageReceiveBuffer = 512
)

// errCoordinationExecutorBusy is an error returned when the coordination
// executor cannot execute the requested coordination due to an ongoing one.
var errCoordinationExecutorBusy = fmt.Errorf("coordination executor is busy")

// coordinationWindow represents a single coordination window. The coordination
// block is the first block of the window.
type coordinationWindow struct {
	// coordinationBlock is the first block of the coordination window.
	coordinationBlock uint64
}

// newCoordinationWindow creates a new coordination window for the given
// coordination block.
func newCoordinationWindow(coordinationBlock uint64) *coordinationWindow {
	return &coordinationWindow{
		coordinationBlock: coordinationBlock,
	}
}

// ActivePhaseEndBlock returns the block number at which the active phase
// of the coordination window ends.
func (cw *coordinationWindow) activePhaseEndBlock() uint64 {
	return cw.coordinationBlock + coordinationActivePhaseDurationBlocks
}

// EndBlock returns the block number at which the coordination window ends.
func (cw *coordinationWindow) endBlock() uint64 {
	return cw.coordinationBlock + coordinationDurationBlocks
}

// isAfter returns true if this coordination window is after the other
// window.
func (cw *coordinationWindow) isAfter(other *coordinationWindow) bool {
	if other == nil {
		return true
	}

	return cw.coordinationBlock > other.coordinationBlock
}

// index returns the index of the coordination window. The index is computed
// by dividing the coordination block number by the coordination frequency.
// A valid index is a positive integer.
//
// For example:
// - window starting at block 900 has index 1
// - window starting at block 1800 has index 2
// - window starting at block 2700 has index 3
//
// If the coordination block number is not a multiple of the coordination
// frequency, the index is 0.
func (cw *coordinationWindow) index() uint64 {
	if cw.coordinationBlock%coordinationFrequencyBlocks == 0 {
		return cw.coordinationBlock / coordinationFrequencyBlocks
	}

	return 0
}

// watchCoordinationWindows watches for new coordination windows and runs
// the given callback when a new window is detected. The callback is run
// in a separate goroutine. It is guaranteed that the callback is not run
// twice for the same window. The context passed as the first parameter
// is used to cancel the watch.
func watchCoordinationWindows(
	ctx context.Context,
	watchBlocksFn func(ctx context.Context) <-chan uint64,
	onWindowFn func(window *coordinationWindow),
) {
	blocksChan := watchBlocksFn(ctx)
	var lastWindow *coordinationWindow

	for {
		select {
		case block := <-blocksChan:
			if window := newCoordinationWindow(block); window.index() > 0 {
				// Make sure the current window is not the same as the last one.
				// There is no guarantee that the block channel will not emit
				// the same block again.
				if window.isAfter(lastWindow) {
					lastWindow = window
					// Run the callback in a separate goroutine to avoid blocking
					// this loop and potentially missing the next block.
					go onWindowFn(window)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// CoordinationFaultType represents a type of the coordination fault.
type CoordinationFaultType uint8

const (
	// FaultUnknown is a fault type used when the fault type is unknown.
	FaultUnknown CoordinationFaultType = iota
	// FaultLeaderIdleness is a fault type used when the leader was idle, i.e.
	// missed their turn to propose a wallet action.
	FaultLeaderIdleness
	// FaultLeaderMistake is a fault type used when the leader's proposal
	// turned out to be invalid.
	FaultLeaderMistake
	// FaultLeaderImpersonation is a fault type used when the leader was
	// impersonated by another operator who raised their own proposal.
	FaultLeaderImpersonation
)

func (cft CoordinationFaultType) String() string {
	switch cft {
	case FaultUnknown:
		return "Unknown"
	case FaultLeaderIdleness:
		return "LeaderIdleness"
	case FaultLeaderMistake:
		return "FaultLeaderMistake"
	case FaultLeaderImpersonation:
		return "LeaderImpersonation"
	default:
		panic("unknown coordination fault type")
	}
}

// coordinationFault represents a single coordination fault.
type coordinationFault struct {
	culprit   chain.Address // address of the operator responsible for the fault
	faultType CoordinationFaultType
}

func (cf *coordinationFault) String() string {
	return fmt.Sprintf(
		"operator [%s], fault [%s]",
		cf.culprit,
		cf.faultType,
	)
}

// CoordinationProposalRequest represents a request for a coordination proposal.
type CoordinationProposalRequest struct {
	WalletPublicKeyHash [20]byte
	WalletOperators     []chain.Address
	ExecutingOperator   chain.Address
	ActionsChecklist    []WalletActionType
}

// CoordinationProposalGenerator is a component responsible for generating
// coordination proposals.
type CoordinationProposalGenerator interface {
	// Generate generates a coordination proposal based on the given checklist
	// of possible wallet actions. The checklist is a list of actions that
	// should be checked for the given coordination window. The generator is
	// expected to return a proposal for the first action from the checklist
	// that is valid for the given wallet's state. If none of the actions are
	// valid, the generator should return a no-op proposal.
	Generate(request *CoordinationProposalRequest) (CoordinationProposal, error)
}

// CoordinationProposal represents a single action proposal for the given wallet.
type CoordinationProposal interface {
	pb.Marshaler
	pb.Unmarshaler

	// ActionType returns the specific type of the walletAction being subject
	// of this proposal.
	ActionType() WalletActionType
	// ValidityBlocks returns the number of blocks for which the proposal is
	// valid. This value SHOULD NOT be marshaled/unmarshaled.
	ValidityBlocks() uint64
}

// NoopProposal is a proposal that does not propose any action.
type NoopProposal struct{}

func (np *NoopProposal) ActionType() WalletActionType {
	return ActionNoop
}

func (np *NoopProposal) ValidityBlocks() uint64 {
	// Panic to make sure that the proposal is not processed by the node.
	panic("noop proposal does not have validity blocks")
}

// coordinationResult represents the result of the coordination procedure
// executed for the given wallet in the given coordination window.
type coordinationResult struct {
	wallet   wallet
	window   *coordinationWindow
	leader   chain.Address
	proposal CoordinationProposal
	faults   []*coordinationFault
}

func (cr *coordinationResult) String() string {
	return fmt.Sprintf(
		"wallet [%s], window [%v], leader [%s], proposal [%s], faults [%s]",
		&cr.wallet,
		cr.window.coordinationBlock,
		cr.leader,
		cr.proposal.ActionType(),
		cr.faults,
	)
}

// coordinationMessage represents a coordination message sent by the leader
// to their followers during the active phase of the coordination window.
type coordinationMessage struct {
	senderID            group.MemberIndex
	coordinationBlock   uint64
	walletPublicKeyHash [20]byte
	proposal            CoordinationProposal
}

func (cm *coordinationMessage) Type() string {
	return "tbtc/coordination_message"
}

// coordinationExecutor is responsible for executing the coordination
// procedure for the given wallet.
type coordinationExecutor struct {
	lock *semaphore.Weighted

	chain Chain

	coordinatedWallet wallet
	membersIndexes    []group.MemberIndex
	operatorAddress   chain.Address

	proposalGenerator CoordinationProposalGenerator

	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	protocolLatch       *generator.ProtocolLatch

	waitForBlockFn waitForBlockFn

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

// newCoordinationExecutor creates a new coordination executor for the
// given wallet.
func newCoordinationExecutor(
	chain Chain,
	coordinatedWallet wallet,
	membersIndexes []group.MemberIndex,
	operatorAddress chain.Address,
	proposalGenerator CoordinationProposalGenerator,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	protocolLatch *generator.ProtocolLatch,
	waitForBlockFn waitForBlockFn,
) *coordinationExecutor {
	return &coordinationExecutor{
		lock:                semaphore.NewWeighted(1),
		chain:               chain,
		coordinatedWallet:   coordinatedWallet,
		membersIndexes:      membersIndexes,
		operatorAddress:     operatorAddress,
		proposalGenerator:   proposalGenerator,
		broadcastChannel:    broadcastChannel,
		membershipValidator: membershipValidator,
		protocolLatch:       protocolLatch,
		waitForBlockFn:      waitForBlockFn,
	}
}

// walletPublicKeyHash returns the 20-byte public key hash of the
// coordinated wallet.
func (ce *coordinationExecutor) walletPublicKeyHash() [20]byte {
	return bitcoin.PublicKeyHash(ce.coordinatedWallet.publicKey)
}

// coordinate executes the coordination procedure for the given coordination
// window.
func (ce *coordinationExecutor) coordinate(
	window *coordinationWindow,
) (*coordinationResult, error) {
	if lockAcquired := ce.lock.TryAcquire(1); !lockAcquired {
		return nil, errCoordinationExecutorBusy
	}
	defer ce.lock.Release(1)

	ce.protocolLatch.Lock()
	defer ce.protocolLatch.Unlock()

	// Just in case, check if the window is valid.
	if window.index() == 0 {
		return nil, fmt.Errorf(
			"invalid coordination block [%v]",
			window.coordinationBlock,
		)
	}

	walletPublicKeyBytes, err := marshalPublicKey(ce.coordinatedWallet.publicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	execLogger := logger.With(
		zap.Uint64("coordinationBlock", window.coordinationBlock),
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	execLogger.Info("starting coordination")

	startTime := time.Now()

	seed, err := ce.getSeed(window.coordinationBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to compute coordination seed: [%v]", err)
	}

	execLogger.Infof("coordination seed is: [0x%x]", seed)

	leader := ce.getLeader(seed)

	execLogger.Infof("coordination leader is: [%s]", leader)

	actionsChecklist := ce.getActionsChecklist(window.index(), seed)

	execLogger.Infof("actions checklist is: [%v]", actionsChecklist)

	// Set up a context that is automatically cancelled when the active phase
	// of the coordination window ends.
	//
	// The coordination leader keeps that context active for the lifetime of the
	// active phase to provide retransmissions of the coordination message thus
	// maximize the chance that all followers receive it on time. The only case
	// when the leader cancels the context prematurely is when the leader's
	// routine fails.
	//
	// The coordination follower cancels the context as soon as it receives
	// the coordination message.
	ctx, cancelCtx := withCancelOnBlock(
		context.Background(),
		window.activePhaseEndBlock(),
		ce.waitForBlockFn,
	)

	var proposal CoordinationProposal
	var faults []*coordinationFault

	if leader == ce.operatorAddress {
		execLogger.Info("executing leader's routine")

		proposal, err = ce.executeLeaderRoutine(
			ctx,
			window.coordinationBlock,
			actionsChecklist,
		)
		if err != nil {
			// Cancel the context upon leader's routine failure. There is
			// no point to keep the context active as retransmissions do not
			// occur anyway.
			cancelCtx()
			if ce.metricsRecorder != nil {
				ce.metricsRecorder.IncrementCounter("coordination_failed_total", 1)
				ce.metricsRecorder.RecordDuration("coordination_duration_seconds", time.Since(startTime))
			}
			return nil, fmt.Errorf(
				"failed to execute leader's routine: [%v]",
				err,
			)
		}

		execLogger.Infof("broadcasted proposal: [%s]", proposal.ActionType())
	} else {
		execLogger.Info("executing follower's routine")

		// Cancel the context upon follower's routine completion.
		defer cancelCtx()

		proposal, faults, err = ce.executeFollowerRoutine(
			ctx,
			leader,
			window.coordinationBlock,
			append(actionsChecklist, ActionNoop),
		)
		if err != nil {
			if ce.metricsRecorder != nil {
				ce.metricsRecorder.IncrementCounter("coordination_failed_total", 1)
				ce.metricsRecorder.RecordDuration("coordination_duration_seconds", time.Since(startTime))
			}
			return nil, fmt.Errorf(
				"failed to execute follower's routine: [%v]",
				err,
			)
		}

		execLogger.Infof(
			"received proposal: [%s]; observed faults: [%v]",
			proposal.ActionType(),
			faults,
		)
	}

	// Just in case, if the proposal is nil, set it to noop.
	if proposal == nil {
		proposal = &NoopProposal{}
	}

	result := &coordinationResult{
		wallet:   ce.coordinatedWallet,
		window:   window,
		leader:   leader,
		proposal: proposal,
		faults:   faults,
	}

	execLogger.Infof("coordination completed with result: [%s]", result)

	// Record successful coordination metrics
	if ce.metricsRecorder != nil {
		ce.metricsRecorder.RecordDuration("coordination_duration_seconds", time.Since(startTime))
	}

	return result, nil
}

// setMetricsRecorder sets the metrics recorder for the coordination executor.
func (ce *coordinationExecutor) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	ce.metricsRecorder = recorder
}

// getSeed computes the coordination seed for the given coordination window.
func (ce *coordinationExecutor) getSeed(
	coordinationBlock uint64,
) ([32]byte, error) {
	walletPublicKeyHash := ce.walletPublicKeyHash()

	safeBlockNumber := coordinationBlock - coordinationSafeBlockShift
	safeBlockHash, err := ce.chain.GetBlockHashByNumber(safeBlockNumber)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"failed to get safe block hash: [%v]",
			err,
		)
	}

	return sha256.Sum256(
		append(
			walletPublicKeyHash[:],
			safeBlockHash[:]...,
		),
	), nil
}

// getLeader returns the address of the coordination leader for the given
// coordination seed.
func (ce *coordinationExecutor) getLeader(seed [32]byte) chain.Address {
	// First, take all operators backing the wallet.
	allOperators := chain.Addresses(ce.coordinatedWallet.signingGroupOperators)

	// Determine a list of unique operators.
	uniqueOperators := make([]chain.Address, 0)
	for operator := range allOperators.Set() {
		uniqueOperators = append(uniqueOperators, operator)
	}

	// Sort the list of unique operators in ascending order.
	sort.Slice(
		uniqueOperators,
		func(i, j int) bool {
			return uniqueOperators[i] < uniqueOperators[j]
		},
	)

	// #nosec G404 (insecure random number source (rand))
	// Shuffling operators does not require secure randomness.
	// Use first 8 bytes of the seed to initialize the RNG.
	rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed[:8]))))

	// Shuffle the list of unique operators.
	rng.Shuffle(
		len(uniqueOperators),
		func(i, j int) {
			uniqueOperators[i], uniqueOperators[j] =
				uniqueOperators[j], uniqueOperators[i]
		},
	)

	// The first operator in the shuffled list is the leader.
	return uniqueOperators[0]
}

// getActionsChecklist returns a list of wallet actions that should be checked
// for the given coordination window. Returns nil for incorrect coordination
// windows whose index is 0.
func (ce *coordinationExecutor) getActionsChecklist(
	windowIndex uint64,
	seed [32]byte,
) []WalletActionType {
	// Return nil checklist for incorrect coordination windows.
	if windowIndex == 0 {
		return nil
	}

	var actions []WalletActionType

	// Redemption action is a priority action and should be checked on every
	// coordination window.
	actions = append(actions, ActionRedemption)

	// Other actions should be checked with a lower frequency. The default
	// frequency is every 4 coordination windows.
	frequencyWindows := uint64(4)

	if windowIndex%frequencyWindows == 0 {
		actions = append(actions, ActionDepositSweep)
	}

	if windowIndex%frequencyWindows == 0 {
		actions = append(actions, ActionMovedFundsSweep)
	}

	if windowIndex%frequencyWindows == 0 {
		actions = append(actions, ActionMovingFunds)
	}

	// #nosec G404 (insecure random number source (rand))
	// Drawing a decision about heartbeat does not require secure randomness.
	// Use first 8 bytes of the seed to initialize the RNG.
	rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed[:8]))))
	if rng.Float64() < coordinationHeartbeatProbability {
		actions = append(actions, ActionHeartbeat)
	}

	return actions
}

// executeLeaderRoutine executes the leader's routine for the given coordination
// window. The routine generates a proposal and broadcasts it to the followers.
// It returns the generated proposal or an error if the routine failed.
func (ce *coordinationExecutor) executeLeaderRoutine(
	ctx context.Context,
	coordinationBlock uint64,
	actionsChecklist []WalletActionType,
) (CoordinationProposal, error) {
	walletPublicKeyHash := ce.walletPublicKeyHash()

	proposal, err := ce.generateProposal(
		&CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
			WalletOperators:     ce.coordinatedWallet.signingGroupOperators,
			ExecutingOperator:   ce.operatorAddress,
			ActionsChecklist:    actionsChecklist,
		},
		2,             // 2 attempts at most
		1*time.Minute, // 1 minute between attempts
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proposal: [%v]", err)
	}

	// Sort members indexes in ascending order, just in case. Choose the first
	// member as the sender of the coordination message.
	membersIndexes := append([]group.MemberIndex{}, ce.membersIndexes...)
	slices.Sort(membersIndexes)
	senderID := membersIndexes[0]

	message := &coordinationMessage{
		senderID:            senderID,
		coordinationBlock:   coordinationBlock,
		walletPublicKeyHash: walletPublicKeyHash,
		proposal:            proposal,
	}

	err = ce.broadcastChannel.Send(
		ctx,
		message,
		net.BackoffRetransmissionStrategy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send coordination message: [%v]", err)
	}

	return proposal, nil
}

// generateProposal generates a proposal for the given coordination request.
// The generator retries the proposal generation if it fails. The number of
// attempts is limited to attemptLimit. The generator waits for retryDelay
// between attempts.
func (ce *coordinationExecutor) generateProposal(
	request *CoordinationProposalRequest,
	attemptLimit uint,
	retryDelay time.Duration,
) (CoordinationProposal, error) {
	var attemptErrs []string

	for attempt := uint(1); attempt <= attemptLimit; attempt++ {
		if attempt > 1 {
			time.Sleep(retryDelay)
		}

		proposal, err := ce.proposalGenerator.Generate(request)
		if err != nil {
			attemptErrs = append(
				attemptErrs,
				fmt.Sprintf("attempt [%v] error: [%v]", attempt, err),
			)
			continue
		}

		return proposal, nil
	}

	return nil, fmt.Errorf(
		"all attempts failed: [%v]",
		strings.Join(attemptErrs, "; "),
	)
}

// executeFollowerRoutine executes the follower's routine for the given coordination
// window. The routine listens for the coordination message from the leader and
// validates it. If the leader's proposal is valid, it returns the received
// proposal. Returns an error if the routine failed.
func (ce *coordinationExecutor) executeFollowerRoutine(
	ctx context.Context,
	leader chain.Address,
	coordinationBlock uint64,
	actionsAllowed []WalletActionType,
) (CoordinationProposal, []*coordinationFault, error) {
	// Cache wallet public key hash to not compute it on every message.
	walletPublicKeyHash := ce.walletPublicKeyHash()
	// Leader ID is the index of the first (index-wise) member controlled by
	// the leader operator. The membersByOperator function returns a list of
	// members controlled by the leader operator in the ascending order.
	// It is enough to take the first member from the list. No need
	// to check for list length as it is guaranteed that the leader operator
	// is one of the operators backing the wallet.
	leaderID := ce.coordinatedWallet.membersByOperator(leader)[0]

	var faults []*coordinationFault

	messagesChan := make(chan net.Message, coordinationMessageReceiveBuffer)

	ce.broadcastChannel.Recv(ctx, func(message net.Message) {
		messagesChan <- message
	})

loop:
	for {
		select {
		case netMessage := <-messagesChan:
			// Filter out messages of wrong type.
			message, ok := netMessage.Payload().(*coordinationMessage)
			if !ok {
				continue
			}

			// Filter out messages from self.
			if slices.Contains(ce.membersIndexes, message.senderID) {
				continue
			}

			// Filter out messages with invalid membership.
			if !ce.membershipValidator.IsValidMembership(
				message.senderID,
				netMessage.SenderPublicKey(),
			) {
				continue
			}

			// Filter out messages with wrong coordination block.
			if coordinationBlock != message.coordinationBlock {
				continue
			}

			// Filter out messages with wrong wallet.
			if walletPublicKeyHash != message.walletPublicKeyHash {
				continue
			}

			// Filter out messages from leader's impersonators.
			if leaderID != message.senderID {
				sender := ce.chain.Signing().PublicKeyBytesToAddress(
					netMessage.SenderPublicKey(),
				)
				faults = append(
					faults, &coordinationFault{
						culprit:   sender,
						faultType: FaultLeaderImpersonation,
					},
				)
				continue
			}

			// Filter out messages that propose an action that is not allowed
			// for the given coordination window.
			if !slices.Contains(actionsAllowed, message.proposal.ActionType()) {
				faults = append(
					faults, &coordinationFault{
						culprit:   leader,
						faultType: FaultLeaderMistake,
					},
				)
				continue
			}

			return message.proposal, faults, nil
		case <-ctx.Done():
			break loop
		}
	}

	faults = append(
		faults, &coordinationFault{
			culprit:   leader,
			faultType: FaultLeaderIdleness,
		},
	)

	return nil, faults, fmt.Errorf("coordination message not received on time")
}
