package tbtcpg

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"go.uber.org/zap"
)

var (
	// ErrMaxBtcTransferZero is the error returned when wallet max BTC transfer
	// parameter is zero.
	ErrMaxBtcTransferZero = fmt.Errorf(
		"wallet max BTC transfer must be positive",
	)

	// ErrNotEnoughTargetWallets is the error returned when the number of
	// gathered target wallets does not match the required target wallets count.
	ErrNotEnoughTargetWallets = fmt.Errorf("not enough target wallets")

	// ErrWrongCommitmentHash is the error returned when the hash calculated
	// from retrieved target wallets does not match the committed hash.
	ErrWrongCommitmentHash = fmt.Errorf(
		"target wallets hash must match commitment hash",
	)

	// ErrNoExecutingOperator is the error returned when the task executing
	// operator is not found among the wallet operator IDs.
	ErrNoExecutingOperator = fmt.Errorf(
		"task executing operator not found among wallet operators",
	)

	// ErrTransactionNotIncluded is the error returned when the commitment
	// submission transaction was not included in the Ethereum blockchain.
	ErrTransactionNotIncluded = fmt.Errorf(
		"transaction not included in blockchain",
	)

	// ErrFeeTooHigh is the error returned when the estimated fee exceeds the
	// maximum fee allowed for the moving funds transaction.
	ErrFeeTooHigh = fmt.Errorf("estimated fee exceeds the maximum fee")
)

// MovingFundsCommitmentLookBackBlocks is the look-back period in blocks used
// when searching for submitted moving funds commitment events. It's equal to
// 30 days assuming 12 seconds per block.
const MovingFundsCommitmentLookBackBlocks = uint64(216000)

// MovingFundsTask is a task that may produce a moving funds proposal.
type MovingFundsTask struct {
	chain    Chain
	btcChain bitcoin.Chain
}

func NewMovingFundsTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *MovingFundsTask {
	return &MovingFundsTask{
		chain:    chain,
		btcChain: btcChain,
	}
}

func (mft *MovingFundsTask) Run(request *tbtc.CoordinationProposalRequest) (
	tbtc.CoordinationProposal,
	bool,
	error,
) {
	walletPublicKeyHash := request.WalletPublicKeyHash

	taskLogger := logger.With(
		zap.String("task", mft.ActionType().String()),
		zap.String("walletPKH", fmt.Sprintf("0x%x", walletPublicKeyHash)),
	)

	// Check if the wallet is eligible for moving funds.
	walletChainData, err := mft.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get source wallet's chain data: [%w]",
			err,
		)
	}

	if walletChainData.State != tbtc.StateMovingFunds {
		taskLogger.Infof("source wallet not in MoveFunds state")
		return nil, false, nil
	}

	// Check the safety margin for moving funds early. This will prevent
	// commitment submission if the wallet is not safe to move funds.
	err = tbtc.ValidateMovingFundsSafetyMargin(walletPublicKeyHash, mft.chain)
	if err != nil {
		taskLogger.Infof("source wallet moving funds safety margin validation failed: [%v]", err)
		return nil, false, nil
	}

	if walletChainData.PendingRedemptionsValue > 0 {
		taskLogger.Infof("source wallet has pending redemptions")
		return nil, false, nil
	}

	if walletChainData.PendingMovedFundsSweepRequestsCount > 0 {
		taskLogger.Infof("source wallet has pending moved funds sweep requests")
		return nil, false, nil
	}

	// The wallet should not have any unswept deposits. It's enough to find at
	// least one unswept deposit. A single unswept deposit means that the wallet
	// should not move funds yet.
	unsweptDeposits, err := FindDeposits(
		mft.chain,
		mft.btcChain,
		walletPublicKeyHash,
		1,
		true,
		true,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot check wallet's unswept deposits: [%w]",
			err,
		)
	}

	if len(unsweptDeposits) > 0 {
		taskLogger.Infof("source wallet has unswept deposits")
		return nil, false, nil
	}

	walletMainUtxo, err := tbtc.DetermineWalletMainUtxo(
		walletPublicKeyHash,
		mft.chain,
		mft.btcChain,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get wallet's main UTXO: [%w]",
			err,
		)
	}

	walletBalance := int64(0)
	if walletMainUtxo != nil {
		walletBalance = walletMainUtxo.Value
	}

	if walletBalance <= 0 {
		// The wallet's balance cannot be `0`. Since we are dealing with
		// a signed integer we also check it's not negative just in case.
		taskLogger.Infof("source wallet does not have a positive balance")
		return nil, false, nil
	}

	liveWalletsCount, err := mft.chain.GetLiveWalletsCount()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get Live wallets count: [%w]",
			err,
		)
	}

	if liveWalletsCount == 0 {
		taskLogger.Infof("there are no Live wallets available")
		return nil, false, nil
	}

	targetWalletsCommitmentHash :=
		walletChainData.MovingFundsTargetWalletsCommitmentHash

	targetWallets, commitmentExists, err := mft.FindTargetWallets(
		taskLogger,
		walletPublicKeyHash,
		targetWalletsCommitmentHash,
		uint64(walletBalance),
		liveWalletsCount,
	)
	if err != nil {
		return nil, false, fmt.Errorf("cannot find target wallets: [%w]", err)
	}

	if !commitmentExists {
		walletMemberIDs, walletMemberIndex, err := mft.GetWalletMembersInfo(
			request.WalletOperators,
			request.ExecutingOperator,
		)
		if err != nil {
			return nil, false, fmt.Errorf(
				"cannot get wallet members IDs: [%w]",
				err,
			)
		}

		err = mft.SubmitMovingFundsCommitment(
			taskLogger,
			walletPublicKeyHash,
			walletMainUtxo,
			walletMemberIDs,
			walletMemberIndex,
			targetWallets,
		)
		if err != nil {
			return nil, false, fmt.Errorf(
				"error while submitting moving funds commitment: [%w]",
				err,
			)
		}
	}

	proposal, err := mft.ProposeMovingFunds(
		taskLogger,
		walletPublicKeyHash,
		walletMainUtxo,
		targetWallets,
		0,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot prepare moving funds proposal: [%w]",
			err,
		)
	}

	return proposal, true, nil
}

// FindTargetWallets returns a list of target wallets for the moving funds
// procedure. If the source wallet has not submitted moving funds commitment yet
// a new list of target wallets is prepared. If the source wallet has already
// submitted the commitment, the returned target wallet list is prepared based
// on the submitted commitment event.
func (mft *MovingFundsTask) FindTargetWallets(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	targetWalletsCommitmentHash [32]byte,
	walletBalance uint64,
	liveWalletsCount uint32,
) ([][20]byte, bool, error) {
	if targetWalletsCommitmentHash == [32]byte{} {
		targetWallets, err := mft.findNewTargetWallets(
			taskLogger,
			sourceWalletPublicKeyHash,
			walletBalance,
			liveWalletsCount,
		)

		return targetWallets, false, err
	} else {
		targetWallets, err := mft.retrieveCommittedTargetWallets(
			taskLogger,
			sourceWalletPublicKeyHash,
			targetWalletsCommitmentHash,
		)

		return targetWallets, true, err
	}
}

func (mft *MovingFundsTask) findNewTargetWallets(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	walletBalance uint64,
	liveWalletsCount uint32,
) ([][20]byte, error) {
	taskLogger.Infof(
		"commitment not submitted yet; looking for new target wallets",
	)

	_, _, _, _, _, walletMaxBtcTransfer, _, err := mft.chain.GetWalletParameters()
	if err != nil {
		return nil, fmt.Errorf("cannot get wallet parameters: [%w]", err)
	}

	if walletMaxBtcTransfer == 0 {
		return nil, ErrMaxBtcTransferZero
	}

	ceilingDivide := func(x, y uint64) uint64 {
		// The divisor must be positive, but we do not need to check it as
		// this function will be executed with wallet max BTC transfer as
		// the divisor and we already ensured it is positive.
		if x == 0 {
			return 0
		}
		return 1 + (x-1)/y
	}
	min := func(x, y uint64) uint64 {
		if x < y {
			return x
		}
		return y
	}

	targetWalletsCount := min(
		uint64(liveWalletsCount),
		ceilingDivide(walletBalance, walletMaxBtcTransfer),
	)

	// Prepare a list of target wallets using the new wallets registration
	// events. Retrieve only the necessary number of live wallets.
	// The iteration is started from the end of the list as the newest wallets
	// are located there and have the highest chance of being Live.
	events, err := mft.chain.PastNewWalletRegisteredEvents(nil)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past new wallet registered events: [%v]",
			err,
		)
	}

	targetWallets := make([][20]byte, 0)

	for i := len(events) - 1; i >= 0; i-- {
		walletPubKeyHash := events[i].WalletPublicKeyHash
		if walletPubKeyHash == sourceWalletPublicKeyHash {
			// Just in case make sure not to include the source wallet
			// itself.
			continue
		}

		wallet, err := mft.chain.GetWallet(walletPubKeyHash)
		if err != nil {
			taskLogger.Errorf(
				"failed to get wallet data for wallet with PKH [0x%x]: [%v]",
				walletPubKeyHash,
				err,
			)
			continue
		}

		if wallet.State == tbtc.StateLive {
			targetWallets = append(targetWallets, walletPubKeyHash)
		}
		if len(targetWallets) == int(targetWalletsCount) {
			// Stop the iteration if enough live wallets have been gathered.
			break
		}
	}

	if len(targetWallets) != int(targetWalletsCount) {
		return nil, fmt.Errorf(
			"%w: required [%v] target wallets; gathered [%v]",
			ErrNotEnoughTargetWallets,
			targetWalletsCount,
			len(targetWallets),
		)
	}

	// Sort the target wallets according to their numerical representation
	// as the on-chain contract expects.
	sort.Slice(targetWallets, func(i, j int) bool {
		bigIntI := new(big.Int).SetBytes(targetWallets[i][:])
		bigIntJ := new(big.Int).SetBytes(targetWallets[j][:])
		return bigIntI.Cmp(bigIntJ) < 0
	})

	logger.Infof("gathered [%v] target wallets", len(targetWallets))

	return targetWallets, nil
}

func (mft *MovingFundsTask) retrieveCommittedTargetWallets(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	targetWalletsCommitmentHash [32]byte,
) ([][20]byte, error) {
	taskLogger.Infof(
		"commitment already submitted; retrieving committed target wallets",
	)

	blockCounter, err := mft.chain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get block counter: [%w]",
			err,
		)
	}

	currentBlockNumber, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get current block number: [%w]",
			err,
		)
	}

	// When calculating the filter start block make sure the current block is
	// greater than the commitment look back blocks. This condition could be
	// unmet for example when running local tests. In that case keep the filter
	// start block at `0`.
	filterStartBlock := uint64(0)
	if currentBlockNumber > MovingFundsCommitmentLookBackBlocks {
		filterStartBlock = currentBlockNumber - MovingFundsCommitmentLookBackBlocks
	}

	filter := &tbtc.MovingFundsCommitmentSubmittedEventFilter{
		StartBlock:          filterStartBlock,
		WalletPublicKeyHash: [][20]byte{sourceWalletPublicKeyHash},
	}

	events, err := mft.chain.PastMovingFundsCommitmentSubmittedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past moving funds commitment submitted events: [%w]",
			err,
		)
	}

	// Moving funds commitment can be submitted only once for a given wallet.
	// Check just in case.
	if len(events) != 1 {
		return nil, fmt.Errorf(
			"unexpected number of moving funds commitment submitted events: [%v]",
			len(events),
		)
	}

	targetWallets := events[0].TargetWallets

	// Just in case check if the hash of the target wallets matches the moving
	// funds target wallets commitment hash.
	calculatedHash := mft.chain.ComputeMovingFundsCommitmentHash(targetWallets)
	if calculatedHash != targetWalletsCommitmentHash {
		return nil, ErrWrongCommitmentHash
	}

	return targetWallets, nil
}

// GetWalletMembersInfo returns the wallet member IDs based on the provided
// wallet operator addresses. Additionally, it returns the position of the
// moving funds task execution operator on the list.
func (mft *MovingFundsTask) GetWalletMembersInfo(
	walletOperators []chain.Address,
	executingOperator chain.Address,
) ([]uint32, uint32, error) {
	// Cache mapping operator addresses to their wallet member IDs. It helps to
	// limit the number of calls to the ETH client if some operator addresses
	// occur on the list multiple times.
	operatorIDCache := make(map[chain.Address]uint32)
	// TODO: Consider adding a global cache at the `ProposalGenerator` level.

	walletMemberIndex := 0
	walletMemberIDs := make([]uint32, 0)

	for index, operatorAddress := range walletOperators {
		// If the operator address is the address of the executing operator save
		// its position. Note that since the executing operator can control
		// multiple wallet members its address can occur on the list multiple
		// times. For clarity, we should save the first occurrence on the list,
		// i.e. when `walletMemberIndex` still holds the value of `0`.
		if operatorAddress == executingOperator && walletMemberIndex == 0 {
			// Increment the index by 1 as operator indexing starts at 1, not 0.
			// This ensures the operator's position is correctly identified in
			// the range [1, walletOperators.length].
			walletMemberIndex = index + 1
		}

		// Search for the operator address in the cache. Store the operator
		// address in the cache if it's not there.
		if operatorID, found := operatorIDCache[operatorAddress]; !found {
			fetchedOperatorID, err := mft.chain.GetOperatorID(operatorAddress)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to get operator ID: [%w]", err)
			}
			operatorIDCache[operatorAddress] = fetchedOperatorID
			walletMemberIDs = append(walletMemberIDs, fetchedOperatorID)
		} else {
			walletMemberIDs = append(walletMemberIDs, operatorID)
		}
	}

	// The task executing operator must always be on the wallet operators list.
	if walletMemberIndex == 0 {
		return nil, 0, ErrNoExecutingOperator
	}

	return walletMemberIDs, uint32(walletMemberIndex), nil
}

// SubmitMovingFundsCommitment submits the moving funds commitment and waits
// until the transaction has entered the Ethereum blockchain.
func (mft *MovingFundsTask) SubmitMovingFundsCommitment(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	walletMainUTXO *bitcoin.UnspentTransactionOutput,
	walletMembersIDs []uint32,
	walletMemberIndex uint32,
	targetWallets [][20]byte,
) error {
	err := mft.chain.SubmitMovingFundsCommitment(
		walletPublicKeyHash,
		*walletMainUTXO,
		walletMembersIDs,
		walletMemberIndex,
		targetWallets,
	)
	if err != nil {
		return fmt.Errorf(
			"error while submitting moving funds commitment to chain: [%w]",
			err,
		)
	}

	blockCounter, err := mft.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// Make sure the moving funds commitment transaction has been confirmed.
	// Give the transaction at most `6` blocks to enter the blockchain.
	for blockHeight := currentBlock + 1; blockHeight <= currentBlock+6; blockHeight++ {
		err := blockCounter.WaitForBlockHeight(blockHeight)
		if err != nil {
			return fmt.Errorf("error while waiting for block height [%w]", err)
		}

		walletData, err := mft.chain.GetWallet(walletPublicKeyHash)
		if err != nil {
			return fmt.Errorf("error wile getting wallet chain data [%w]", err)
		}

		// To verify the commitment transaction has entered the Ethereum
		// blockchain check that the commitment hash is not zero.
		if walletData.MovingFundsTargetWalletsCommitmentHash != [32]byte{} {
			taskLogger.Infof(
				"the moving funds commitment transaction successfully "+
					"confirmed at block: [%d]",
				blockHeight,
			)
			return nil
		}

		taskLogger.Infof(
			"the moving funds commitment transaction still not confirmed at "+
				"block: [%d]",
			blockHeight,
		)
	}

	taskLogger.Info(
		"failed to verify the moving funds commitment transaction submission",
	)

	return ErrTransactionNotIncluded
}

// ProposeMovingFunds returns a moving funds proposal.
func (mft *MovingFundsTask) ProposeMovingFunds(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	targetWallets [][20]byte,
	fee int64,
) (*tbtc.MovingFundsProposal, error) {
	if len(targetWallets) == 0 {
		return nil, fmt.Errorf("target wallets list is empty")
	}

	taskLogger.Infof("preparing a moving funds proposal")

	// Estimate fee if it's missing.
	if fee <= 0 {
		taskLogger.Infof("estimating moving funds transaction fee")

		txMaxTotalFee, _, _, _, _, _, _, _, _, _, _, err := mft.chain.GetMovingFundsParameters()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get moving funds tx max total fee: [%w]",
				err,
			)
		}

		estimatedFee, err := EstimateMovingFundsFee(
			mft.btcChain,
			len(targetWallets),
			txMaxTotalFee,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot estimate moving funds transaction fee: [%w]",
				err,
			)
		}

		fee = estimatedFee
	}

	taskLogger.Infof("moving funds transaction fee: [%d]", fee)

	proposal := &tbtc.MovingFundsProposal{
		TargetWallets:    targetWallets,
		MovingFundsTxFee: big.NewInt(fee),
	}

	taskLogger.Infof("validating the moving funds proposal")

	if err := tbtc.ValidateMovingFundsProposal(
		taskLogger,
		walletPublicKeyHash,
		mainUTXO,
		proposal,
		mft.chain,
	); err != nil {
		return nil, fmt.Errorf(
			"failed to verify moving funds proposal: [%w]",
			err,
		)
	}

	return proposal, nil
}

func (mft *MovingFundsTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionMovingFunds
}

// estimateCappedFee estimates the transaction fee for a transaction of the
// virtual size produced by the given size estimator. It returns feeTooHighErr
// if the estimated fee exceeds maxTotalFee.
func estimateCappedFee(
	btcChain bitcoin.Chain,
	sizeEstimator *bitcoin.TransactionSizeEstimator,
	maxTotalFee uint64,
	feeTooHighErr error,
) (int64, error) {
	transactionSize, err := sizeEstimator.VirtualSize()
	if err != nil {
		return 0, fmt.Errorf(
			"cannot estimate transaction virtual size: [%v]",
			err,
		)
	}

	feeEstimator := bitcoin.NewTransactionFeeEstimator(btcChain)

	totalFee, err := feeEstimator.EstimateFee(transactionSize)
	if err != nil {
		return 0, fmt.Errorf("cannot estimate transaction fee: [%v]", err)
	}

	if uint64(totalFee) > maxTotalFee {
		return 0, feeTooHighErr
	}

	return totalFee, nil
}

// EstimateMovingFundsFee estimates fee for the moving funds transaction that
// moves funds from the source wallet to target wallets.
func EstimateMovingFundsFee(
	btcChain bitcoin.Chain,
	targetWalletsCount int,
	txMaxTotalFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddPublicKeyHashOutputs(targetWalletsCount, true)

	return estimateCappedFee(btcChain, sizeEstimator, txMaxTotalFee, ErrFeeTooHigh)
}
