package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-core/pkg/chain"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	frostvalidatorabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/validatorabi"
	"github.com/keep-network/keep-core/pkg/frost"
	frostregistry "github.com/keep-network/keep-core/pkg/frost/registry"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// FrostWalletRegistryAvailable reports whether the chain handle is configured
// with a FROST wallet registry address.
func (tc *TbtcChain) FrostWalletRegistryAvailable() bool {
	return tc.frostWalletRegistry != nil
}

// OnBridgeNewWalletRequested registers a callback for Bridge.NewWalletRequested.
func (tc *TbtcChain) OnBridgeNewWalletRequested(
	handler func(event *tbtc.BridgeNewWalletRequestedEvent),
) subscription.EventSubscription {
	return tc.bridge.NewWalletRequestedEvent(nil).OnEvent(
		func(blockNumber uint64) {
			handler(&tbtc.BridgeNewWalletRequestedEvent{
				BlockNumber: blockNumber,
			})
		},
	)
}

// OnFrostDKGStarted registers a callback for FrostWalletRegistry.DkgStarted.
func (tc *TbtcChain) OnFrostDKGStarted(
	handler func(event *tbtc.FrostDKGStartedEvent),
) subscription.EventSubscription {
	if tc.frostWalletRegistry == nil {
		return subscription.NewEventSubscription(func() {})
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryDkgStarted)

	sub, err := tc.frostWalletRegistry.WatchDkgStarted(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil,
	)
	if err != nil {
		cancelCtx()
		logger.Errorf("failed to watch FROST DKG started events: [%v]", err)
		return subscription.NewEventSubscription(func() {})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-sub.Err():
				if !ok {
					return
				}
				logger.Errorf("FROST DKG started subscription error: [%v]", err)
			case event, ok := <-sink:
				if !ok {
					return
				}
				handler(&tbtc.FrostDKGStartedEvent{
					Seed:        event.Seed,
					BlockNumber: event.Raw.BlockNumber,
				})
			}
		}
	}()

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

// PastFrostDKGStartedEvents fetches past FROST DKG started events.
func (tc *TbtcChain) PastFrostDKGStartedEvents(
	filter *tbtc.FrostDKGStartedEventFilter,
) ([]*tbtc.FrostDKGStartedEvent, error) {
	if tc.frostWalletRegistry == nil {
		return nil, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	var startBlock uint64
	var endBlock *uint64
	var seed []*big.Int

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		seed = filter.Seed
	}

	iterator, err := tc.frostWalletRegistry.FilterDkgStarted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		seed,
	)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	events := make([]*tbtc.FrostDKGStartedEvent, 0)
	for iterator.Next() {
		events = append(events, &tbtc.FrostDKGStartedEvent{
			Seed:        iterator.Event.Seed,
			BlockNumber: iterator.Event.Raw.BlockNumber,
		})
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].BlockNumber < events[j].BlockNumber
	})

	return events, nil
}

// OnFrostDKGResultSubmitted registers a callback for FROST DKG submissions.
func (tc *TbtcChain) OnFrostDKGResultSubmitted(
	handler func(event *tbtc.FrostDKGResultSubmittedEvent),
) subscription.EventSubscription {
	if tc.frostWalletRegistry == nil {
		return subscription.NewEventSubscription(func() {})
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryDkgResultSubmitted)

	sub, err := tc.frostWalletRegistry.WatchDkgResultSubmitted(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil,
		nil,
	)
	if err != nil {
		cancelCtx()
		logger.Errorf("failed to watch FROST DKG result submitted events: [%v]", err)
		return subscription.NewEventSubscription(func() {})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-sub.Err():
				if !ok {
					return
				}
				logger.Errorf("FROST DKG result submitted subscription error: [%v]", err)
			case event, ok := <-sink:
				if !ok {
					return
				}
				result, err := convertFrostDKGResultFromABI(event.Result)
				if err != nil {
					logger.Errorf("unexpected FROST DKG result in event: [%v]", err)
					continue
				}

				handler(&tbtc.FrostDKGResultSubmittedEvent{
					Seed:        event.Seed,
					ResultHash:  event.ResultHash,
					Result:      result,
					BlockNumber: event.Raw.BlockNumber,
				})
			}
		}
	}()

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

// PastFrostDKGResultSubmittedEvents fetches past FROST DKG submitted events.
func (tc *TbtcChain) PastFrostDKGResultSubmittedEvents(
	filter *tbtc.FrostDKGResultSubmittedEventFilter,
) ([]*tbtc.FrostDKGResultSubmittedEvent, error) {
	if tc.frostWalletRegistry == nil {
		return nil, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	var startBlock uint64
	var endBlock *uint64
	var resultHash [][32]byte
	var seed []*big.Int

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		for _, hash := range filter.ResultHash {
			resultHash = append(resultHash, [32]byte(hash))
		}
		seed = filter.Seed
	}

	iterator, err := tc.frostWalletRegistry.FilterDkgResultSubmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHash,
		seed,
	)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	events := make([]*tbtc.FrostDKGResultSubmittedEvent, 0)
	for iterator.Next() {
		result, err := convertFrostDKGResultFromABI(iterator.Event.Result)
		if err != nil {
			return nil, err
		}

		events = append(events, &tbtc.FrostDKGResultSubmittedEvent{
			Seed:        iterator.Event.Seed,
			ResultHash:  iterator.Event.ResultHash,
			Result:      result,
			BlockNumber: iterator.Event.Raw.BlockNumber,
		})
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].BlockNumber < events[j].BlockNumber
	})

	return events, nil
}

// OnFrostDKGResultChallenged registers a callback for FROST DKG challenges.
func (tc *TbtcChain) OnFrostDKGResultChallenged(
	handler func(event *tbtc.FrostDKGResultChallengedEvent),
) subscription.EventSubscription {
	if tc.frostWalletRegistry == nil {
		return subscription.NewEventSubscription(func() {})
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryDkgResultChallenged)

	sub, err := tc.frostWalletRegistry.WatchDkgResultChallenged(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil,
		nil,
	)
	if err != nil {
		cancelCtx()
		logger.Errorf("failed to watch FROST DKG challenged events: [%v]", err)
		return subscription.NewEventSubscription(func() {})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-sub.Err():
				if !ok {
					return
				}
				logger.Errorf("FROST DKG challenged subscription error: [%v]", err)
			case event, ok := <-sink:
				if !ok {
					return
				}
				handler(&tbtc.FrostDKGResultChallengedEvent{
					ResultHash:  event.ResultHash,
					Challenger:  chain.Address(event.Challenger.String()),
					Reason:      event.Reason,
					BlockNumber: event.Raw.BlockNumber,
				})
			}
		}
	}()

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

// OnFrostDKGResultApproved registers a callback for FROST DKG approvals.
func (tc *TbtcChain) OnFrostDKGResultApproved(
	handler func(event *tbtc.FrostDKGResultApprovedEvent),
) subscription.EventSubscription {
	if tc.frostWalletRegistry == nil {
		return subscription.NewEventSubscription(func() {})
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryDkgResultApproved)

	sub, err := tc.frostWalletRegistry.WatchDkgResultApproved(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil,
		nil,
	)
	if err != nil {
		cancelCtx()
		logger.Errorf("failed to watch FROST DKG approved events: [%v]", err)
		return subscription.NewEventSubscription(func() {})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-sub.Err():
				if !ok {
					return
				}
				logger.Errorf("FROST DKG approved subscription error: [%v]", err)
			case event, ok := <-sink:
				if !ok {
					return
				}
				handler(&tbtc.FrostDKGResultApprovedEvent{
					ResultHash:  event.ResultHash,
					Approver:    chain.Address(event.Approver.String()),
					BlockNumber: event.Raw.BlockNumber,
				})
			}
		}
	}()

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

// SelectFrostGroup returns the currently selected FROST DKG group.
func (tc *TbtcChain) SelectFrostGroup() (*tbtc.GroupSelectionResult, error) {
	if tc.frostWalletRegistry == nil || tc.frostSortitionPool == nil {
		return nil, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	operatorsIDs, err := tc.frostWalletRegistry.SelectGroup(
		&bind.CallOpts{From: tc.key.Address},
	)
	if err != nil {
		return nil, err
	}

	operatorsAddresses, err := tc.frostSortitionPool.GetIDOperators(operatorsIDs)
	if err != nil {
		return nil, err
	}

	ids := make([]chain.OperatorID, len(operatorsIDs))
	addresses := make([]chain.Address, len(operatorsIDs))
	for i := range ids {
		ids[i] = operatorsIDs[i]
		addresses[i] = chain.Address(operatorsAddresses[i].String())
	}

	return &tbtc.GroupSelectionResult{
		OperatorsIDs:       ids,
		OperatorsAddresses: addresses,
	}, nil
}

// GetFrostDKGState returns the current FROST wallet creation state.
func (tc *TbtcChain) GetFrostDKGState() (tbtc.DKGState, error) {
	if tc.frostWalletRegistry == nil {
		return 0, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	state, err := tc.frostWalletRegistry.GetWalletCreationState(
		&bind.CallOpts{From: tc.key.Address},
	)
	if err != nil {
		return 0, err
	}

	return tbtc.DKGState(state), nil
}

// IsFrostDKGResultValid validates the submitted FROST DKG result using the
// registry-level view. This intentionally avoids passing seed/startBlock from
// off-chain code.
func (tc *TbtcChain) IsFrostDKGResultValid(
	result *frostregistry.Result,
) (bool, string, error) {
	if tc.frostWalletRegistry == nil {
		return false, "", fmt.Errorf("FrostWalletRegistry is not configured")
	}

	abiResult, err := convertFrostDKGResultToABI(result)
	if err != nil {
		return false, "", err
	}

	return tc.frostWalletRegistry.IsDkgResultValid(
		&bind.CallOpts{From: tc.key.Address},
		abiResult,
	)
}

// CalculateFrostDKGResultDigest computes the pre-EIP-191 result digest and,
// when the validator view is configured, checks it against the on-chain
// FrostDkgValidator.resultDigest implementation.
func (tc *TbtcChain) CalculateFrostDKGResultDigest(
	seed *big.Int,
	result *frostregistry.Result,
) ([32]byte, error) {
	if result == nil {
		return [32]byte{}, fmt.Errorf("FROST DKG result is nil")
	}
	if tc.frostWalletRegistry == nil {
		return [32]byte{}, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	localDigest, err := frostregistry.ResultDigest(
		tc.chainID,
		tc.bridgeAddress,
		tc.frostWalletRegistryAddr,
		seed,
		result.XOnlyOutputKey,
		result.Members,
		result.MisbehavedMembersIndices,
	)
	if err != nil {
		return [32]byte{}, err
	}

	if tc.frostDkgValidator == nil {
		return localDigest, nil
	}

	validatorResult, err := convertFrostDKGResultToValidatorABI(result)
	if err != nil {
		return [32]byte{}, err
	}

	onChainDigest, err := tc.frostDkgValidator.ResultDigest(
		&bind.CallOpts{From: tc.key.Address},
		validatorResult,
		seed,
		tc.bridgeAddress,
		tc.frostWalletRegistryAddr,
	)
	if err != nil {
		return [32]byte{}, err
	}

	if localDigest != onChainDigest {
		return [32]byte{}, fmt.Errorf(
			"local FROST DKG digest [0x%x] does not match validator digest [0x%x]",
			localDigest,
			onChainDigest,
		)
	}

	return localDigest, nil
}

// SubmitFrostDKGResult submits a FROST DKG result. Submission is optimistic on
// chain; callers should pre-validate before invoking this method.
func (tc *TbtcChain) SubmitFrostDKGResult(result *frostregistry.Result) error {
	if tc.frostWalletRegistry == nil {
		return fmt.Errorf("FrostWalletRegistry is not configured")
	}

	abiResult, err := convertFrostDKGResultToABI(result)
	if err != nil {
		return err
	}

	return tc.submitFrostWalletRegistryTransaction(
		"submitDkgResult",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return tc.frostWalletRegistry.SubmitDkgResult(opts, abiResult)
		},
	)
}

// ChallengeFrostDKGResult challenges an invalid FROST DKG result. The on-chain
// function requires msg.sender == tx.origin, which this EOA chain handle
// satisfies directly.
func (tc *TbtcChain) ChallengeFrostDKGResult(result *frostregistry.Result) error {
	if tc.frostWalletRegistry == nil {
		return fmt.Errorf("FrostWalletRegistry is not configured")
	}

	abiResult, err := convertFrostDKGResultToABI(result)
	if err != nil {
		return err
	}

	return tc.submitFrostWalletRegistryTransaction(
		"challengeDkgResult",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return tc.frostWalletRegistry.ChallengeDkgResult(opts, abiResult)
		},
	)
}

// ApproveFrostDKGResult approves a FROST DKG result after the challenge window.
// The contract gates submitter precedence using submitterPrecedencePeriodLength.
func (tc *TbtcChain) ApproveFrostDKGResult(result *frostregistry.Result) error {
	if tc.frostWalletRegistry == nil {
		return fmt.Errorf("FrostWalletRegistry is not configured")
	}

	abiResult, err := convertFrostDKGResultToABI(result)
	if err != nil {
		return err
	}

	return tc.submitFrostWalletRegistryTransaction(
		"approveDkgResult",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return tc.frostWalletRegistry.ApproveDkgResult(opts, abiResult)
		},
	)
}

// FrostDKGParameters gets the current FROST DKG timing parameters.
func (tc *TbtcChain) FrostDKGParameters() (*tbtc.DKGParameters, error) {
	if tc.frostWalletRegistry == nil {
		return nil, fmt.Errorf("FrostWalletRegistry is not configured")
	}

	params, err := tc.frostWalletRegistry.DkgParameters(
		&bind.CallOpts{From: tc.key.Address},
	)
	if err != nil {
		return nil, err
	}

	return &tbtc.DKGParameters{
		SubmissionTimeoutBlocks:       params.ResultSubmissionTimeout.Uint64(),
		ChallengePeriodBlocks:         params.ResultChallengePeriodLength.Uint64(),
		ApprovePrecedencePeriodBlocks: params.SubmitterPrecedencePeriodLength.Uint64(),
	}, nil
}

func convertFrostDKGResultFromABI(
	result frostabi.Struct0,
) (*frostregistry.Result, error) {
	submitterMemberIndex, err := uint256ToUint64(
		result.SubmitterMemberIndex,
		"submitter member index",
	)
	if err != nil {
		return nil, err
	}

	signingMembersIndices := make([]uint64, len(result.SigningMembersIndices))
	for i, signingMemberIndex := range result.SigningMembersIndices {
		signingMembersIndices[i], err = uint256ToUint64(
			signingMemberIndex,
			"signing member index",
		)
		if err != nil {
			return nil, err
		}
	}

	outputKey := frost.OutputKey(result.XOnlyOutputKey)

	return &frostregistry.Result{
		SubmitterMemberIndex:     submitterMemberIndex,
		XOnlyOutputKey:           outputKey,
		MembersHash:              result.MembersHash,
		MisbehavedMembersIndices: append(frostregistry.MisbehavedMemberIndices{}, result.MisbehavedMembersIndices...),
		Signatures:               append([]byte{}, result.Signatures...),
		SigningMembersIndices:    signingMembersIndices,
		Members:                  append(frostregistry.FullMembers{}, result.Members...),
	}, nil
}

func convertFrostDKGResultToABI(
	result *frostregistry.Result,
) (frostabi.Struct0, error) {
	if result == nil {
		return frostabi.Struct0{}, fmt.Errorf("FROST DKG result is nil")
	}

	signingMembersIndices := make([]*big.Int, len(result.SigningMembersIndices))
	for i, signingMemberIndex := range result.SigningMembersIndices {
		signingMembersIndices[i] = new(big.Int).SetUint64(signingMemberIndex)
	}

	return frostabi.Struct0{
		SubmitterMemberIndex:     new(big.Int).SetUint64(result.SubmitterMemberIndex),
		XOnlyOutputKey:           [32]byte(result.XOnlyOutputKey),
		MembersHash:              result.MembersHash,
		MisbehavedMembersIndices: append([]uint8{}, result.MisbehavedMembersIndices...),
		Signatures:               append([]byte{}, result.Signatures...),
		SigningMembersIndices:    signingMembersIndices,
		Members:                  append([]uint32{}, result.Members...),
	}, nil
}

func convertFrostDKGResultToValidatorABI(
	result *frostregistry.Result,
) (frostvalidatorabi.Struct0, error) {
	if result == nil {
		return frostvalidatorabi.Struct0{}, fmt.Errorf("FROST DKG result is nil")
	}

	signingMembersIndices := make([]*big.Int, len(result.SigningMembersIndices))
	for i, signingMemberIndex := range result.SigningMembersIndices {
		signingMembersIndices[i] = new(big.Int).SetUint64(signingMemberIndex)
	}

	return frostvalidatorabi.Struct0{
		SubmitterMemberIndex:     new(big.Int).SetUint64(result.SubmitterMemberIndex),
		XOnlyOutputKey:           [32]byte(result.XOnlyOutputKey),
		MembersHash:              result.MembersHash,
		MisbehavedMembersIndices: append([]uint8{}, result.MisbehavedMembersIndices...),
		Signatures:               append([]byte{}, result.Signatures...),
		SigningMembersIndices:    signingMembersIndices,
		Members:                  append([]uint32{}, result.Members...),
	}, nil
}

func (tc *TbtcChain) submitFrostWalletRegistryTransaction(
	method string,
	submitFn func(opts *bind.TransactOpts) (*types.Transaction, error),
) error {
	tc.transactionMutex.Lock()
	defer tc.transactionMutex.Unlock()

	transactorOptions, err := bind.NewKeyedTransactorWithChainID(
		tc.key.PrivateKey,
		tc.chainID,
	)
	if err != nil {
		return fmt.Errorf("failed to instantiate transactor: [%v]", err)
	}

	nonce, err := tc.nonceManager.CurrentNonce()
	if err != nil {
		return fmt.Errorf("failed to retrieve account nonce: [%v]", err)
	}
	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := submitFn(transactorOptions)
	if err != nil {
		return fmt.Errorf("failed to submit %s transaction: [%w]", method, err)
	}

	logger.Infof(
		"submitted transaction %s with id: [%s] and nonce [%v]",
		method,
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			transaction, err := submitFn(newTransactorOptions)
			if err != nil {
				return nil, err
			}

			logger.Infof(
				"submitted transaction %s with id: [%s] and nonce [%v]",
				method,
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tc.nonceManager.IncrementNonce()

	return nil
}

func uint256ToUint64(value *big.Int, fieldName string) (uint64, error) {
	if value == nil {
		return 0, fmt.Errorf("%s is nil", fieldName)
	}

	if !value.IsUint64() {
		return 0, fmt.Errorf("%s [%s] overflows uint64", fieldName, value.String())
	}

	return value.Uint64(), nil
}
