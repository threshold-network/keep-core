// tbtc_deposit.go: deposit lifecycle (request, reveal, funding) for the TbtcChain adapter.
package ethereum

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func (tc *TbtcChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var depositor []common.Address
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock

		for _, d := range filter.Depositor {
			depositor = append(depositor, common.HexToAddress(d.String()))
		}

		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastDepositRevealedEvents(
		startBlock,
		endBlock,
		depositor,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.DepositRevealedEvent, 0)
	for _, event := range events {
		var vault *chain.Address
		if event.Vault != [20]byte{} {
			v := chain.Address(event.Vault.Hex())
			vault = &v
		}

		convertedEvent := &tbtc.DepositRevealedEvent{
			// We can map the event.FundingTxHash field directly to the
			// bitcoin.Hash type. This is because event.FundingTxHash is
			// a [32]byte type representing a hash in the bitcoin.InternalByteOrder,
			// just as bitcoin.Hash assumes.
			FundingTxHash:       event.FundingTxHash,
			FundingOutputIndex:  event.FundingOutputIndex,
			Depositor:           chain.Address(event.Depositor.Hex()),
			Amount:              event.Amount,
			BlindingFactor:      event.BlindingFactor,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			RefundPublicKeyHash: event.RefundPubKeyHash,
			RefundLocktime:      event.RefundLocktime,
			Vault:               vault,
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

func (tc *TbtcChain) GetDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) (*tbtc.DepositChainRequest, bool, error) {
	depositKey := buildDepositKey(fundingTxHash, fundingOutputIndex)
	depositCacheKey := depositKey.Text(16)

	tc.sweptDepositsCache.Sweep()
	if cachedRequest, ok := tc.sweptDepositsCache.Get(depositCacheKey); ok {
		return cachedRequest, true, nil
	}

	chainRequest, err := tc.bridge.Deposits(depositKey)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get deposit request for key [0x%x]: [%v]",
			depositKey.Text(16),
			err,
		)
	}

	// Deposit not found.
	if chainRequest.RevealedAt == 0 {
		return nil, false, nil
	}

	var vault *chain.Address
	if chainRequest.Vault != [20]byte{} {
		v := chain.Address(chainRequest.Vault.Hex())
		vault = &v
	}

	var extraData *[32]byte
	if chainRequest.ExtraData != [32]byte{} {
		extraData = &chainRequest.ExtraData
	}

	request := &tbtc.DepositChainRequest{
		Depositor:   chain.Address(chainRequest.Depositor.Hex()),
		Amount:      chainRequest.Amount,
		RevealedAt:  time.Unix(int64(chainRequest.RevealedAt), 0),
		Vault:       vault,
		TreasuryFee: chainRequest.TreasuryFee,
		SweptAt:     time.Unix(int64(chainRequest.SweptAt), 0),
		ExtraData:   extraData,
	}

	// If the request was swept on-chain, there is a guarantee that no
	// further changes will occur regarding its parameters.
	// Such a request can be cached.
	if isSwept := request.SweptAt.Unix() != 0; isSwept {
		tc.sweptDepositsCache.Add(depositCacheKey, request)
	}

	return request, true, nil
}

func (tc *TbtcChain) BuildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	return buildDepositKey(fundingTxHash, fundingOutputIndex)
}

func (tc *TbtcChain) GetDepositParameters() (tbtc.DepositParameters, error) {
	parameters, err := tc.bridge.DepositParameters()
	if err != nil {
		return tbtc.DepositParameters{}, err
	}

	return tbtc.DepositParameters{
		DustThreshold:      parameters.DepositDustThreshold,
		TreasuryFeeDivisor: parameters.DepositTreasuryFeeDivisor,
		TxMaxFee:           parameters.DepositTxMaxFee,
		RevealAheadPeriod:  parameters.DepositRevealAheadPeriod,
	}, nil
}

func (tc *TbtcChain) SubmitDepositSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	vault common.Address,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	sweepProof := tbtcabi.BitcoinTxProof2{
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

	gasEstimate, err := tc.maintainerProxy.SubmitDepositSweepProofGasEstimate(
		bitcoinTxInfo,
		sweepProof,
		utxo,
		vault,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

	_, err = tc.maintainerProxy.SubmitDepositSweepProof(
		bitcoinTxInfo,
		sweepProof,
		utxo,
		vault,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func buildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	fundingOutputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fundingOutputIndexBytes, fundingOutputIndex)

	depositKey := crypto.Keccak256Hash(
		append(fundingTxHash[:], fundingOutputIndexBytes...),
	)

	return depositKey.Big()
}

func convertDepositSweepProposalToAbiType(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
) tbtcabi.WalletProposalValidatorDepositSweepProposal {
	depositsKeys := make(
		[]tbtcabi.WalletProposalValidatorDepositKey,
		len(proposal.DepositsKeys),
	)

	for i, depositKey := range proposal.DepositsKeys {
		// We can map the depositKey.FundingTxHash field directly to the
		// [32]byte type. This is because depositKey.FundingTxHash is
		// a bitcoin.Hash type representing a hash in the
		// bitcoin.InternalByteOrder, just as the on-chain contract assumes.
		depositsKeys[i] = tbtcabi.WalletProposalValidatorDepositKey{
			FundingTxHash:      depositKey.FundingTxHash,
			FundingOutputIndex: depositKey.FundingOutputIndex,
		}
	}

	return tbtcabi.WalletProposalValidatorDepositSweepProposal{
		WalletPubKeyHash:     walletPublicKeyHash,
		DepositsKeys:         depositsKeys,
		SweepTxFee:           proposal.SweepTxFee,
		DepositsRevealBlocks: proposal.DepositsRevealBlocks,
	}
}

func (tc *TbtcChain) ValidateDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
	depositsExtraInfo []struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	dei := make([]tbtcabi.WalletProposalValidatorDepositExtraInfo, len(depositsExtraInfo))
	for i, depositExtraInfo := range depositsExtraInfo {
		fundingTx := tbtcabi.BitcoinTxInfo2{
			Version:      depositExtraInfo.FundingTx.SerializeVersion(),
			InputVector:  depositExtraInfo.FundingTx.SerializeInputs(),
			OutputVector: depositExtraInfo.FundingTx.SerializeOutputs(),
			Locktime:     depositExtraInfo.FundingTx.SerializeLocktime(),
		}

		dei[i] = tbtcabi.WalletProposalValidatorDepositExtraInfo{
			FundingTx:        fundingTx,
			BlindingFactor:   depositExtraInfo.Deposit.BlindingFactor,
			WalletPubKeyHash: depositExtraInfo.Deposit.WalletPublicKeyHash,
			RefundPubKeyHash: depositExtraInfo.Deposit.RefundPublicKeyHash,
			RefundLocktime:   depositExtraInfo.Deposit.RefundLocktime,
		}
	}

	valid, err := tc.walletProposalValidator.ValidateDepositSweepProposal(
		convertDepositSweepProposalToAbiType(walletPublicKeyHash, proposal),
		dei,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateDepositSweepProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) GetDepositSweepMaxSize() (uint16, error) {
	return tc.walletProposalValidator.DEPOSITSWEEPMAXSIZE()
}

func (tc *TbtcChain) GetDepositMinAge() (uint32, error) {
	return tc.walletProposalValidator.DEPOSITMINAGE()
}
