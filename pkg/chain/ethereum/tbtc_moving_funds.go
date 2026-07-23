package ethereum

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func (tc *TbtcChain) ComputeMovingFundsCommitmentHash(
	targetWallets [][20]byte,
) [32]byte {
	return computeMovingFundsCommitmentHash(targetWallets)
}

func computeMovingFundsCommitmentHash(targetWallets [][20]byte) [32]byte {
	packedWallets := []byte{}

	for _, wallet := range targetWallets {
		packedWallets = append(packedWallets, wallet[:]...)
		// Each wallet hash must be padded with 12 zero bytes following the
		// actual hash.
		packedWallets = append(packedWallets, make([]byte, 12)...)
	}

	return crypto.Keccak256Hash(packedWallets)
}

func (tc *TbtcChain) PastMovingFundsCommitmentSubmittedEvents(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
) ([]*tbtc.MovingFundsCommitmentSubmittedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastMovingFundsCommitmentSubmittedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.MovingFundsCommitmentSubmittedEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.MovingFundsCommitmentSubmittedEvent{
			WalletPublicKeyHash: event.WalletPubKeyHash,
			TargetWallets:       event.TargetWallets,
			Submitter:           chain.Address(event.Submitter.Hex()),
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

func (tc *TbtcChain) PastMovingFundsCompletedEvents(
	filter *tbtc.MovingFundsCompletedEventFilter,
) ([]*tbtc.MovingFundsCompletedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastMovingFundsCompletedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.MovingFundsCompletedEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.MovingFundsCompletedEvent{
			WalletPublicKeyHash: event.WalletPubKeyHash,
			MovingFundsTxHash:   event.MovingFundsTxHash,
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

func (tc *TbtcChain) SubmitMovingFundsCommitment(
	walletPublicKeyHash [20]byte,
	walletMainUTXO bitcoin.UnspentTransactionOutput,
	walletMembersIDs []uint32,
	walletMemberIndex uint32,
	targetWallets [][20]byte,
) error {
	mainUtxo := tbtcabi.BitcoinTxUTXO{
		TxHash:        walletMainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: walletMainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(walletMainUTXO.Value),
	}
	_, err := tc.bridge.SubmitMovingFundsCommitment(
		walletPublicKeyHash,
		mainUtxo,
		walletMembersIDs,
		big.NewInt(int64(walletMemberIndex)),
		targetWallets,
	)
	return err
}

func (tc *TbtcChain) SubmitMovingFundsProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	movingFundsProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitMovingFundsProofGasEstimate(
		bitcoinTxInfo,
		movingFundsProof,
		utxo,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitMovingFundsProof(
		bitcoinTxInfo,
		movingFundsProof,
		utxo,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) SubmitMovedFundsSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	movedFundsSweepProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitMovedFundsSweepProofGasEstimate(
		bitcoinTxInfo,
		movedFundsSweepProof,
		utxo,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitMovedFundsSweepProof(
		bitcoinTxInfo,
		movedFundsSweepProof,
		utxo,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) ValidateMovedFundsSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.MovedFundsSweepProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorMovedFundsSweepProposal{
		WalletPubKeyHash:         walletPublicKeyHash,
		MovingFundsTxHash:        proposal.MovingFundsTxHash,
		MovingFundsTxOutputIndex: proposal.MovingFundsTxOutputIndex,
		MovedFundsSweepTxFee:     proposal.SweepTxFee,
	}

	valid, err := tc.walletProposalValidator.ValidateMovedFundsSweepProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateMovedFundsSweepProposal` returns
	// true or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) GetMovingFundsParameters() (tbtc.MovingFundsParameters, error) {
	parameters, err := tc.bridge.MovingFundsParameters()
	if err != nil {
		return tbtc.MovingFundsParameters{}, err
	}

	return tbtc.MovingFundsParameters{
		TxMaxTotalFee:                        parameters.MovingFundsTxMaxTotalFee,
		DustThreshold:                        parameters.MovingFundsDustThreshold,
		TimeoutResetDelay:                    parameters.MovingFundsTimeoutResetDelay,
		Timeout:                              parameters.MovingFundsTimeout,
		TimeoutSlashingAmount:                parameters.MovingFundsTimeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier:      parameters.MovingFundsTimeoutNotifierRewardMultiplier,
		CommitmentGasOffset:                  parameters.MovingFundsCommitmentGasOffset,
		SweepTxMaxTotalFee:                   parameters.MovedFundsSweepTxMaxTotalFee,
		SweepTimeout:                         parameters.MovedFundsSweepTimeout,
		SweepTimeoutSlashingAmount:           parameters.MovedFundsSweepTimeoutSlashingAmount,
		SweepTimeoutNotifierRewardMultiplier: parameters.MovedFundsSweepTimeoutNotifierRewardMultiplier,
	}, nil
}

func (tc *TbtcChain) GetMovedFundsSweepRequest(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) (*tbtc.MovedFundsSweepRequest, bool, error) {
	movedFundsKey := buildMovedFundsKey(
		movingFundsTxHash,
		movingFundsTxOutpointIndex,
	)

	movedFundsSweepRequest, err := tc.bridge.MovedFundsSweepRequests(
		movedFundsKey,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get moved funds sweep request for key [0x%x]: [%v]",
			movedFundsKey.Text(16),
			err,
		)
	}

	// Moved funds sweep request not found.
	if movedFundsSweepRequest.CreatedAt == 0 {
		return nil, false, nil
	}

	state, err := parseMovedFundsSweepRequestState(movedFundsSweepRequest.State)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot parse state for moved funds sweep request [0x%x]: [%v]",
			movedFundsKey.Text(16),
			err,
		)
	}

	return &tbtc.MovedFundsSweepRequest{
		WalletPublicKeyHash: movedFundsSweepRequest.WalletPubKeyHash,
		Value:               movedFundsSweepRequest.Value,
		CreatedAt:           time.Unix(int64(movedFundsSweepRequest.CreatedAt), 0),
		State:               state,
	}, true, nil
}

func parseMovedFundsSweepRequestState(value uint8) (
	tbtc.MovedFundsSweepRequestState,
	error,
) {
	switch value {
	case 0:
		return tbtc.MovedFundsStateUnknown, nil
	case 1:
		return tbtc.MovedFundsStatePending, nil
	case 2:
		return tbtc.MovedFundsStateProcessed, nil
	case 3:
		return tbtc.MovedFundsStateTimedOut, nil
	default:
		return 0, fmt.Errorf(
			"unexpected moved funds sweep request state value: [%v]",
			value,
		)
	}
}

func buildMovedFundsKey(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) *big.Int {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, movingFundsTxOutpointIndex)

	movedFundsKey := crypto.Keccak256Hash(
		append(movingFundsTxHash[:], indexBytes...),
	)

	return movedFundsKey.Big()
}

func (tc *TbtcChain) ValidateMovingFundsProposal(
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *tbtc.MovingFundsProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorMovingFundsProposal{
		WalletPubKeyHash: walletPublicKeyHash,
		TargetWallets:    proposal.TargetWallets,
		MovingFundsTxFee: proposal.MovingFundsTxFee,
	}
	abiMainUTXO := tbtcabi.BitcoinTxUTXO3{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	valid, err := tc.walletProposalValidator.ValidateMovingFundsProposal(
		abiProposal,
		abiMainUTXO,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateMovingFundsProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}
