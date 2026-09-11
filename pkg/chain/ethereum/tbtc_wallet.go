// tbtc_wallet.go: wallet registry read/write methods for the TbtcChain adapter.
package ethereum

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func (tc *TbtcChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var ecdsaWalletID [][32]byte
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		ecdsaWalletID = filter.EcdsaWalletID
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastNewWalletRegisteredEvents(
		startBlock,
		endBlock,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.NewWalletRegisteredEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       event.EcdsaWalletID,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			BlockNumber:         event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func (tc *TbtcChain) CalculateWalletID(
	walletPublicKey *ecdsa.PublicKey,
) ([32]byte, error) {
	return calculateWalletID(walletPublicKey)
}

func calculateWalletID(walletPublicKey *ecdsa.PublicKey) ([32]byte, error) {
	walletPublicKeyBytes, err := convertPubKeyToChainFormat(walletPublicKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"error while converting wallet public key to chain format: [%v]",
			err,
		)
	}

	return crypto.Keccak256Hash(walletPublicKeyBytes[:]), nil
}

func (tc *TbtcChain) IsWalletRegistered(EcdsaWalletID [32]byte) (bool, error) {
	isWalletRegistered, err := tc.walletRegistry.IsWalletRegistered(
		EcdsaWalletID,
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check if wallet with ECDSA ID [0x%x] is registered: [%v]",
			EcdsaWalletID,
			err,
		)
	}

	return isWalletRegistered, nil
}

func (tc *TbtcChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	wallet, err := tc.bridge.Wallets(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get wallet for public key hash [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	// Wallet not found.
	if wallet.CreatedAt == 0 {
		return nil, fmt.Errorf(
			"no wallet for public key hash [0x%x]",
			walletPublicKeyHash,
		)
	}

	walletState, err := parseWalletState(wallet.State)
	if err != nil {
		return nil, fmt.Errorf("cannot parse wallet state: [%v]", err)
	}

	return &tbtc.WalletChainData{
		EcdsaWalletID:                          wallet.EcdsaWalletID,
		MainUtxoHash:                           wallet.MainUtxoHash,
		PendingRedemptionsValue:                wallet.PendingRedemptionsValue,
		CreatedAt:                              time.Unix(int64(wallet.CreatedAt), 0),
		MovingFundsRequestedAt:                 time.Unix(int64(wallet.MovingFundsRequestedAt), 0),
		ClosingStartedAt:                       time.Unix(int64(wallet.ClosingStartedAt), 0),
		PendingMovedFundsSweepRequestsCount:    wallet.PendingMovedFundsSweepRequestsCount,
		State:                                  walletState,
		MovingFundsTargetWalletsCommitmentHash: wallet.MovingFundsTargetWalletsCommitmentHash,
	}, nil
}

func (tc *TbtcChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	onEvent := func(
		walletID [32]byte,
		blockNumber uint64,
	) {
		handler(&tbtc.WalletClosedEvent{
			WalletID:    walletID,
			BlockNumber: blockNumber,
		})
	}
	return tc.walletRegistry.WalletClosedEvent(nil, nil).OnEvent(onEvent)
}

func (tc *TbtcChain) ComputeMainUtxoHash(
	mainUtxo *bitcoin.UnspentTransactionOutput,
) [32]byte {
	return computeMainUtxoHash(mainUtxo)
}

func computeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte {
	outputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(outputIndexBytes, mainUtxo.Outpoint.OutputIndex)

	valueBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBytes, uint64(mainUtxo.Value))

	mainUtxoHash := crypto.Keccak256Hash(
		append(
			append(
				mainUtxo.Outpoint.TransactionHash[:],
				outputIndexBytes...,
			), valueBytes...,
		),
	)

	return mainUtxoHash
}

func (tc *TbtcChain) GetWalletParameters() (tbtc.WalletParameters, error) {
	parameters, err := tc.bridge.WalletParameters()
	if err != nil {
		return tbtc.WalletParameters{}, err
	}

	return tbtc.WalletParameters{
		CreationPeriod:        parameters.WalletCreationPeriod,
		CreationMinBtcBalance: parameters.WalletCreationMinBtcBalance,
		CreationMaxBtcBalance: parameters.WalletCreationMaxBtcBalance,
		ClosureMinBtcBalance:  parameters.WalletClosureMinBtcBalance,
		MaxAge:                parameters.WalletMaxAge,
		MaxBtcTransfer:        parameters.WalletMaxBtcTransfer,
		ClosingPeriod:         parameters.WalletClosingPeriod,
	}, nil
}

func (tc *TbtcChain) GetLiveWalletsCount() (uint32, error) {
	return tc.bridge.LiveWalletsCount()
}

func parseWalletState(value uint8) (tbtc.WalletState, error) {
	switch value {
	case 0:
		return tbtc.StateUnknown, nil
	case 1:
		return tbtc.StateLive, nil
	case 2:
		return tbtc.StateMovingFunds, nil
	case 3:
		return tbtc.StateClosing, nil
	case 4:
		return tbtc.StateClosed, nil
	case 5:
		return tbtc.StateTerminated, nil
	default:
		return 0, fmt.Errorf("unexpected wallet state value: [%v]", value)
	}
}

func (tc *TbtcChain) ValidateHeartbeatProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.HeartbeatProposal,
) error {
	valid, err := tc.walletProposalValidator.ValidateHeartbeatProposal(
		tbtcabi.WalletProposalValidatorHeartbeatProposal{
			WalletPubKeyHash: walletPublicKeyHash,
			Message:          proposal.Message[:],
		},
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateHeartbeatProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}
