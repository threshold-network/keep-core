package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"

	"github.com/keep-network/keep-core/pkg/chain"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

var _ tbtc.WalletInactivityClaimChain = (*frostInactivityClaimChain)(nil)

// frostInactivityClaimChain binds inactivity operations to the FROST wallet
// registry and its independent sortition pool. The embedded TbtcChain supplies
// common Bridge, block-counter, and operator-signing operations.
type frostInactivityClaimChain struct {
	*TbtcChain
}

// InactivityClaimChainForWallet returns a registry-bound inactivity chain for
// the wallet scheme. The legacy TbtcChain implementation remains explicitly
// ECDSA-bound; FROST uses a view overriding every registry-specific operation.
func (tc *TbtcChain) InactivityClaimChainForWallet(
	walletScheme tbtc.WalletScheme,
) (tbtc.WalletInactivityClaimChain, error) {
	switch walletScheme {
	case tbtc.WalletSchemeECDSA:
		return tc, nil
	case tbtc.WalletSchemeFROST:
		if tc.frostWalletRegistry == nil || tc.frostSortitionPool == nil {
			return nil, fmt.Errorf("FrostWalletRegistry is not configured")
		}

		return &frostInactivityClaimChain{TbtcChain: tc}, nil
	default:
		return nil, fmt.Errorf("unsupported wallet scheme [%v]", walletScheme)
	}
}

func (fic *frostInactivityClaimChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	return fic.GetFrostOperatorID(operatorAddress)
}

func (fic *frostInactivityClaimChain) OnInactivityClaimed(
	handler func(event *tbtc.InactivityClaimedEvent),
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryInactivityClaimed)
	events := make(chan *tbtc.InactivityClaimedEvent)
	sub := fic.watchFrostInactivityClaimed(sink)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sink:
				if !ok {
					return
				}
				if event == nil {
					logger.Warn("received nil FROST InactivityClaimed event")
					continue
				}

				emitFrostInactivityClaimedEvent(
					ctx,
					events,
					convertFrostInactivityClaimedEvent(event),
				)
			}
		}
	}()
	go fic.monitorPastFrostInactivityClaimedEvents(ctx, events)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-events:
				if ctx.Err() != nil {
					return
				}
				handler(event)
			}
		}
	}()

	return subscription.NewEventSubscription(func() {
		cancelCtx()
		sub.Unsubscribe()
	})
}

func convertFrostInactivityClaimedEvent(
	event *frostabi.FrostWalletRegistryInactivityClaimed,
) *tbtc.InactivityClaimedEvent {
	if event == nil {
		return nil
	}

	return &tbtc.InactivityClaimedEvent{
		WalletID:    event.WalletID,
		Nonce:       event.Nonce,
		Notifier:    chain.Address(event.Notifier.Hex()),
		BlockNumber: event.Raw.BlockNumber,
	}
}

func (fic *frostInactivityClaimChain) watchFrostInactivityClaimed(
	sink chan<- *frostabi.FrostWalletRegistryInactivityClaimed,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return fic.frostWalletRegistry.WatchInactivityClaimed(
			&bind.WatchOpts{Context: ctx},
			sink,
			nil,
		)
	}

	return chainutil.WithResubscription(
		chainutil.SubscriptionBackoffMax,
		subscribeFn,
		chainutil.SubscriptionAlertThreshold,
		func(elapsed time.Duration) {
			logger.Warnf(
				"subscription to FROST InactivityClaimed had to be retried [%s] "+
					"since the last attempt; please inspect host chain connectivity",
				elapsed,
			)
		},
		func(err error) {
			logger.Errorf(
				"subscription to FROST InactivityClaimed failed with error: [%v]; "+
					"resubscription attempt will be performed",
				err,
			)
		},
	)
}

func (fic *frostInactivityClaimChain) monitorPastFrostInactivityClaimedEvents(
	ctx context.Context,
	events chan<- *tbtc.InactivityClaimedEvent,
) {
	ticker := time.NewTicker(chainutil.DefaultSubscribeOptsTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastBlock, err := fic.blockCounter.CurrentBlock()
			if err != nil {
				logger.Errorf(
					"FROST InactivityClaimed subscription failed to pull events: [%v]",
					err,
				)
				continue
			}

			iterator, err := fic.frostWalletRegistry.FilterInactivityClaimed(
				&bind.FilterOpts{
					Start:   frostSubscriptionMonitoringStartBlock(lastBlock),
					Context: ctx,
				},
				nil,
			)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Errorf(
					"FROST InactivityClaimed subscription failed to pull events: [%v]",
					err,
				)
				continue
			}

			for iterator.Next() {
				emitFrostInactivityClaimedEvent(
					ctx,
					events,
					convertFrostInactivityClaimedEvent(iterator.Event),
				)
			}
			if err := iterator.Error(); err != nil {
				logger.Errorf(
					"FROST InactivityClaimed subscription iterator failed: [%v]",
					err,
				)
			}
			if err := iterator.Close(); err != nil {
				logger.Warnf(
					"failed to close FROST InactivityClaimed iterator: [%v]",
					err,
				)
			}
		}
	}
}

func emitFrostInactivityClaimedEvent(
	ctx context.Context,
	events chan<- *tbtc.InactivityClaimedEvent,
	event *tbtc.InactivityClaimedEvent,
) {
	select {
	case <-ctx.Done():
	case events <- event:
	}
}

func (fic *frostInactivityClaimChain) SubmitInactivityClaim(
	claim *tbtc.InactivityClaim,
	nonce *big.Int,
	groupMembers []uint32,
) error {
	if nonce == nil {
		return fmt.Errorf("inactivity claim nonce is nil")
	}

	abiClaim, err := convertFrostInactivityClaimToABI(claim)
	if err != nil {
		return err
	}

	return fic.submitFrostWalletRegistryTransaction(
		"notifyOperatorInactivity",
		func(opts *bind.TransactOpts) (*types.Transaction, error) {
			return fic.frostWalletRegistry.NotifyOperatorInactivity(
				opts,
				abiClaim,
				nonce,
				groupMembers,
			)
		},
	)
}

func convertFrostInactivityClaimToABI(
	claim *tbtc.InactivityClaim,
) (frostabi.FrostInactivityClaim, error) {
	if claim == nil {
		return frostabi.FrostInactivityClaim{}, fmt.Errorf(
			"inactivity claim is nil",
		)
	}

	inactiveMembersIndices := make([]*big.Int, len(claim.InactiveMembersIndices))
	for i, memberIndex := range claim.InactiveMembersIndices {
		inactiveMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	signingMembersIndices := make([]*big.Int, len(claim.SigningMembersIndices))
	for i, memberIndex := range claim.SigningMembersIndices {
		signingMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	return frostabi.FrostInactivityClaim{
		WalletID:               claim.WalletID,
		InactiveMembersIndices: inactiveMembersIndices,
		HeartbeatFailed:        claim.HeartbeatFailed,
		Signatures:             claim.Signatures,
		SigningMembersIndices:  signingMembersIndices,
	}, nil
}

func (fic *frostInactivityClaimChain) CalculateInactivityClaimHash(
	claim *inactivity.ClaimPreimage,
) (inactivity.ClaimHash, error) {
	if claim == nil || claim.WalletPublicKey == nil || claim.WalletPublicKey.X == nil {
		return inactivity.ClaimHash{}, fmt.Errorf("wallet public key is nil")
	}
	if claim.Nonce == nil {
		return inactivity.ClaimHash{}, fmt.Errorf("inactivity claim nonce is nil")
	}

	xBytes := claim.WalletPublicKey.X.Bytes()
	if len(xBytes) > 32 {
		return inactivity.ClaimHash{}, fmt.Errorf("wrong x-only output key length")
	}

	var xOnlyOutputKey [32]byte
	copy(xOnlyOutputKey[32-len(xBytes):], xBytes)

	inactiveMembersIndexes := make([]*big.Int, len(claim.InactiveMembersIndexes))
	for i, index := range claim.InactiveMembersIndexes {
		inactiveMembersIndexes[i] = big.NewInt(int64(index))
	}

	return calculateFrostInactivityClaimHash(
		fic.chainID,
		claim.Nonce,
		xOnlyOutputKey,
		inactiveMembersIndexes,
		claim.HeartbeatFailed,
	)
}

func calculateFrostInactivityClaimHash(
	chainID *big.Int,
	nonce *big.Int,
	xOnlyOutputKey [32]byte,
	inactiveMembersIndexes []*big.Int,
	heartbeatFailed bool,
) (inactivity.ClaimHash, error) {
	uint256Type, err := abi.NewType("uint256", "uint256", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	bytes32Type, err := abi.NewType("bytes32", "bytes32", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	uint256SliceType, err := abi.NewType("uint256[]", "uint256[]", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	boolType, err := abi.NewType("bool", "bool", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	encoded, err := abi.Arguments{
		{Type: uint256Type},
		{Type: uint256Type},
		{Type: bytes32Type},
		{Type: uint256SliceType},
		{Type: boolType},
	}.Pack(
		chainID,
		nonce,
		xOnlyOutputKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	return inactivity.ClaimHash(crypto.Keccak256Hash(encoded)), nil
}

func (fic *frostInactivityClaimChain) GetInactivityClaimNonce(
	walletID [32]byte,
) (*big.Int, error) {
	nonce, err := fic.frostWalletRegistry.InactivityClaimNonce(
		&bind.CallOpts{From: fic.key.Address},
		walletID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get FROST inactivity claim nonce: [%w]",
			err,
		)
	}

	return nonce, nil
}

// onFrostWalletClosed subscribes to FROST WalletClosed events and marks their
// source scheme so the node can confirm closure against the correct registry.
func (tc *TbtcChain) onFrostWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	if tc.frostWalletRegistry == nil {
		return subscription.NewEventSubscription(func() {})
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	sink := make(chan *frostabi.FrostWalletRegistryWalletClosed)
	events := make(chan *tbtc.WalletClosedEvent)
	sub := tc.watchFrostWalletClosed(sink)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sink:
				if !ok {
					return
				}
				if event == nil {
					logger.Warn("received nil FROST WalletClosed event")
					continue
				}

				emitFrostWalletClosedEvent(
					ctx,
					events,
					convertFrostWalletClosedEvent(event),
				)
			}
		}
	}()
	go tc.monitorPastFrostWalletClosedEvents(ctx, events)
	go handleFrostWalletClosedEvents(ctx, events, handler)

	return subscription.NewEventSubscription(func() {
		cancelCtx()
		sub.Unsubscribe()
	})
}

func emitFrostWalletClosedEvent(
	ctx context.Context,
	events chan<- *tbtc.WalletClosedEvent,
	event *tbtc.WalletClosedEvent,
) {
	select {
	case <-ctx.Done():
	case events <- event:
	}
}

func handleFrostWalletClosedEvents(
	ctx context.Context,
	events <-chan *tbtc.WalletClosedEvent,
	handler func(event *tbtc.WalletClosedEvent),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}

			// If cancellation raced with event delivery, do not invoke the
			// subscriber after it has unsubscribed.
			if ctx.Err() != nil {
				return
			}

			handler(event)
		}
	}
}

func convertFrostWalletClosedEvent(
	event *frostabi.FrostWalletRegistryWalletClosed,
) *tbtc.WalletClosedEvent {
	if event == nil {
		return nil
	}

	return &tbtc.WalletClosedEvent{
		WalletID:    event.WalletID,
		Scheme:      tbtc.WalletSchemeFROST,
		BlockNumber: event.Raw.BlockNumber,
	}
}

func (tc *TbtcChain) watchFrostWalletClosed(
	sink chan<- *frostabi.FrostWalletRegistryWalletClosed,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return tc.frostWalletRegistry.WatchWalletClosed(
			&bind.WatchOpts{Context: ctx},
			sink,
			nil,
		)
	}

	return chainutil.WithResubscription(
		chainutil.SubscriptionBackoffMax,
		subscribeFn,
		chainutil.SubscriptionAlertThreshold,
		func(elapsed time.Duration) {
			logger.Warnf(
				"subscription to FROST WalletClosed had to be retried [%s] "+
					"since the last attempt; please inspect host chain connectivity",
				elapsed,
			)
		},
		func(err error) {
			logger.Errorf(
				"subscription to FROST WalletClosed failed with error: [%v]; "+
					"resubscription attempt will be performed",
				err,
			)
		},
	)
}

func (tc *TbtcChain) monitorPastFrostWalletClosedEvents(
	ctx context.Context,
	events chan<- *tbtc.WalletClosedEvent,
) {
	ticker := time.NewTicker(chainutil.DefaultSubscribeOptsTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastBlock, err := tc.blockCounter.CurrentBlock()
			if err != nil {
				logger.Errorf(
					"FROST WalletClosed subscription failed to pull events: [%v]",
					err,
				)
				continue
			}

			iterator, err := tc.frostWalletRegistry.FilterWalletClosed(
				&bind.FilterOpts{
					Start:   frostSubscriptionMonitoringStartBlock(lastBlock),
					Context: ctx,
				},
				nil,
			)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Errorf(
					"FROST WalletClosed subscription failed to pull events: [%v]",
					err,
				)
				continue
			}

			for iterator.Next() {
				emitFrostWalletClosedEvent(
					ctx,
					events,
					convertFrostWalletClosedEvent(iterator.Event),
				)
			}
			if err := iterator.Error(); err != nil {
				logger.Errorf(
					"FROST WalletClosed subscription iterator failed: [%v]",
					err,
				)
			}
			if err := iterator.Close(); err != nil {
				logger.Warnf(
					"failed to close FROST WalletClosed iterator: [%v]",
					err,
				)
			}
		}
	}
}
