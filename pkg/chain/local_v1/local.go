package local_v1

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand"
	"sync"

	"github.com/ipfs/go-log"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/beacon/event"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/subscription"
	"golang.org/x/crypto/sha3"
)

var logger = log.Logger("keep-chainlocal")

var seedGroupPublicKey = []byte("seed to group public key")
var groupActiveTime = uint64(10)
var relayRequestTimeout = uint64(8)

type localGroup struct {
	groupPublicKey          []byte
	registrationBlockHeight uint64
}

type localChain struct {
	relayConfig *beaconchain.Config

	groups []localGroup

	lastSubmittedDKGResult           *beaconchain.DKGResult
	lastSubmittedDKGResultSignatures map[beaconchain.GroupMemberIndex][]byte
	lastSubmittedRelayEntry          []byte

	handlerMutex             sync.Mutex
	relayEntryHandlers       map[int]func(entry *event.RelayEntrySubmitted)
	relayRequestHandlers     map[int]func(request *event.RelayEntryRequested)
	groupRegisteredHandlers  map[int]func(groupRegistration *event.GroupRegistration)
	dkgStartedHandlers       map[int]func(submission *event.DKGStarted)
	resultSubmissionHandlers map[int]func(submission *event.DKGResultSubmission)

	simulatedHeight uint64
	blockCounter    chain.BlockCounter

	relayEntryTimeoutReportsMutex sync.Mutex
	relayEntryTimeoutReports      []uint64

	operatorPrivateKey *operator.PrivateKey
}

func (c *localChain) BlockCounter() (chain.BlockCounter, error) {
	return c.blockCounter, nil
}

func (c *localChain) Signing() chain.Signing {
	return NewSigner(c.operatorPrivateKey)
}

func (c *localChain) OperatorKeyPair() (*operator.PrivateKey, *operator.PublicKey, error) {
	return c.operatorPrivateKey, &c.operatorPrivateKey.PublicKey, nil
}

func (c *localChain) GetConfig() *beaconchain.Config {
	return c.relayConfig
}

func (c *localChain) SubmitRelayEntry(newEntry []byte) error {
	currentBlock, err := c.blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot read current block: [%v]", err)
	}

	entry := &event.RelayEntrySubmitted{
		BlockNumber: currentBlock,
	}

	c.handlerMutex.Lock()
	for _, handler := range c.relayEntryHandlers {
		go func(handler func(entry *event.RelayEntrySubmitted), entry *event.RelayEntrySubmitted) {
			handler(entry)
		}(handler, entry)
	}
	c.handlerMutex.Unlock()

	c.lastSubmittedRelayEntry = newEntry

	return nil
}

func (c *localChain) OnRelayEntrySubmitted(
	handler func(entry *event.RelayEntrySubmitted),
) subscription.EventSubscription {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	handlerID := GenerateHandlerID()
	c.relayEntryHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()

		delete(c.relayEntryHandlers, handlerID)
	})
}

func (c *localChain) GetLastRelayEntry() []byte {
	return c.lastSubmittedRelayEntry
}

func (c *localChain) OnRelayEntryRequested(
	handler func(request *event.RelayEntryRequested),
) subscription.EventSubscription {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	handlerID := GenerateHandlerID()
	c.relayRequestHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()

		delete(c.relayRequestHandlers, handlerID)
	})
}

func (c *localChain) SelectGroup(seed *big.Int) (chain.Addresses, error) {
	panic("not implemented")
}

func (c *localChain) OnGroupRegistered(
	handler func(groupRegistration *event.GroupRegistration),
) subscription.EventSubscription {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	handlerID := GenerateHandlerID()

	c.groupRegisteredHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()

		delete(c.groupRegisteredHandlers, handlerID)
	})
}

// Connect initializes a local stub implementation of the chain
// interfaces for testing. It uses auto-generated operator key.
func Connect(
	groupSize int,
	honestThreshold int,
) *localChain {
	operatorPrivateKey, _, err := operator.GenerateKeyPair(DefaultCurve)
	if err != nil {
		panic(err)
	}

	return ConnectWithKey(groupSize, honestThreshold, operatorPrivateKey)
}

// ConnectWithKey initializes a local stub implementation of the chain
// interfaces for testing.
func ConnectWithKey(
	groupSize int,
	honestThreshold int,
	operatorPrivateKey *operator.PrivateKey,
) *localChain {
	bc, _ := BlockCounter()

	currentBlock, _ := bc.CurrentBlock()
	group := localGroup{
		groupPublicKey:          seedGroupPublicKey,
		registrationBlockHeight: currentBlock,
	}

	resultPublicationBlockStep := uint64(3)

	return &localChain{
		relayConfig: &beaconchain.Config{
			GroupSize:                  groupSize,
			HonestThreshold:            honestThreshold,
			ResultPublicationBlockStep: resultPublicationBlockStep,
			// #nosec G115 -- Local test-chain callers supply a nonnegative protocol group size.
			RelayEntryTimeout: resultPublicationBlockStep * uint64(groupSize),
		},
		relayEntryHandlers:       make(map[int]func(request *event.RelayEntrySubmitted)),
		relayRequestHandlers:     make(map[int]func(request *event.RelayEntryRequested)),
		groupRegisteredHandlers:  make(map[int]func(groupRegistration *event.GroupRegistration)),
		dkgStartedHandlers:       make(map[int]func(submission *event.DKGStarted)),
		resultSubmissionHandlers: make(map[int]func(submission *event.DKGResultSubmission)),
		blockCounter:             bc,
		groups:                   []localGroup{group},
		operatorPrivateKey:       operatorPrivateKey,
	}
}

func selectGroup(entry *big.Int, numberOfGroups int) int {
	if numberOfGroups == 0 {
		return 0
	}

	return int(new(big.Int).Mod(entry, big.NewInt(int64(numberOfGroups))).Int64())
}

func (c *localChain) IsStaleGroup(groupPublicKey []byte) (bool, error) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	bc, _ := BlockCounter()

	err := bc.WaitForBlockHeight(c.simulatedHeight)
	if err != nil {
		logger.Errorf("could not wait for block height: [%v]", err)
	}

	currentBlock, err := bc.CurrentBlock()

	if err != nil {
		return false, fmt.Errorf("could not determine current block: [%v]", err)
	}

	for _, group := range c.groups {
		if bytes.Equal(group.groupPublicKey, groupPublicKey) {
			return group.registrationBlockHeight+groupActiveTime+relayRequestTimeout < currentBlock, nil
		}
	}

	return true, nil
}

func (c *localChain) IsGroupRegistered(groupPublicKey []byte) (bool, error) {
	for _, group := range c.groups {
		if bytes.Equal(group.groupPublicKey, groupPublicKey) {
			return true, nil
		}
	}
	return false, nil
}

// SubmitDKGResult submits the result to a chain.
func (c *localChain) SubmitDKGResult(
	participantIndex beaconchain.GroupMemberIndex,
	resultToPublish *beaconchain.DKGResult,
	signatures map[beaconchain.GroupMemberIndex][]byte,
) error {
	if len(signatures) < c.relayConfig.HonestThreshold {
		return fmt.Errorf(
			"failed to submit result with [%v] signatures for honest threshold [%v]",
			len(signatures),
			c.relayConfig.HonestThreshold,
		)
	}

	currentBlock, err := c.blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot read current block: [%v]", err)
	}

	dkgResultPublicationEvent := &event.DKGResultSubmission{
		MemberIndex:    uint32(participantIndex),
		GroupPublicKey: resultToPublish.GroupPublicKey[:],
		Misbehaved:     resultToPublish.Misbehaved,
		BlockNumber:    currentBlock,
	}

	myGroup := localGroup{
		groupPublicKey:          resultToPublish.GroupPublicKey,
		registrationBlockHeight: currentBlock,
	}
	c.groups = append(c.groups, myGroup)
	c.lastSubmittedDKGResult = resultToPublish
	c.lastSubmittedDKGResultSignatures = signatures

	groupRegistrationEvent := &event.GroupRegistration{
		GroupPublicKey: resultToPublish.GroupPublicKey[:],
		BlockNumber:    currentBlock,
	}

	c.handlerMutex.Lock()
	for _, handler := range c.resultSubmissionHandlers {
		go func(handler func(*event.DKGResultSubmission), dkgResultPublication *event.DKGResultSubmission) {
			handler(dkgResultPublicationEvent)
		}(handler, dkgResultPublicationEvent)
	}

	for _, handler := range c.groupRegisteredHandlers {
		go func(handler func(*event.GroupRegistration), groupRegistration *event.GroupRegistration) {
			handler(groupRegistrationEvent)
		}(handler, groupRegistrationEvent)
	}
	c.handlerMutex.Unlock()

	return nil
}

func (c *localChain) OnDKGStarted(
	handler func(event *event.DKGStarted),
) subscription.EventSubscription {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	handlerID := GenerateHandlerID()
	c.dkgStartedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()

		delete(c.dkgStartedHandlers, handlerID)
	})
}

func (c *localChain) OnDKGResultSubmitted(
	handler func(dkgResultPublication *event.DKGResultSubmission),
) subscription.EventSubscription {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	handlerID := GenerateHandlerID()
	c.resultSubmissionHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()

		delete(c.resultSubmissionHandlers, handlerID)
	})
}

func (c *localChain) GetLastDKGResult() (
	*beaconchain.DKGResult,
	map[beaconchain.GroupMemberIndex][]byte,
) {
	return c.lastSubmittedDKGResult, c.lastSubmittedDKGResultSignatures
}

func (c *localChain) ReportRelayEntryTimeout() error {
	c.relayEntryTimeoutReportsMutex.Lock()
	defer c.relayEntryTimeoutReportsMutex.Unlock()

	currentBlock, err := c.blockCounter.CurrentBlock()
	if err != nil {
		return err
	}

	c.relayEntryTimeoutReports = append(c.relayEntryTimeoutReports, currentBlock)
	return nil
}

func (c *localChain) IsEntryInProgress() (bool, error) {
	panic("not implemented")
}

func (c *localChain) CurrentRequestStartBlock() (*big.Int, error) {
	panic("not implemented")
}

func (c *localChain) CurrentRequestPreviousEntry() ([]byte, error) {
	panic("not implemented")
}

func (c *localChain) CurrentRequestGroupPublicKey() ([]byte, error) {
	panic("not implemented")
}

func (c *localChain) GetRelayEntryTimeoutReports() []uint64 {
	return c.relayEntryTimeoutReports
}

// CalculateDKGResultHash calculates a 256-bit hash of the DKG result.
func (c *localChain) CalculateDKGResultHash(
	dkgResult *beaconchain.DKGResult,
) (beaconchain.DKGResultHash, error) {
	encodedDKGResult := fmt.Sprint(dkgResult)
	dkgResultHash := beaconchain.DKGResultHash(
		sha3.Sum256([]byte(encodedDKGResult)),
	)

	return dkgResultHash, nil
}

func (c *localChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	panic("unsupported")
}

func (c *localChain) EligibleStake(stakingProvider chain.Address) (*big.Int, error) {
	panic("unsupported")
}

func (c *localChain) IsPoolLocked() (bool, error) {
	panic("unsupported")
}

func (c *localChain) IsOperatorInPool() (bool, error) {
	panic("unsupported")
}

func (c *localChain) IsOperatorUpToDate() (bool, error) {
	panic("unsupported")
}

func (c *localChain) JoinSortitionPool() error {
	panic("unsupported")
}

func (c *localChain) UpdateOperatorStatus() error {
	panic("unsupported")
}

func (c *localChain) IsEligibleForRewards() (bool, error) {
	panic("unsupported")
}

func (c *localChain) CanRestoreRewardEligibility() (bool, error) {
	panic("unsupported")
}

func (c *localChain) RestoreRewardEligibility() error {
	panic("unsupported")
}

func (c *localChain) IsChaosnetActive() (bool, error) {
	panic("unsupported")
}

func (c *localChain) IsBetaOperator() (bool, error) {
	panic("unsupported")
}

func (c *localChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	panic("unsupported")
}

func GenerateHandlerID() int {
	// #nosec G404 (insecure random number source (rand))
	// Local chain implementation doesn't require secure randomness.
	return rand.Int()
}
