// tbtc_redemption.go: redemption request lifecycle for the TbtcChain adapter.
package ethereum

import (
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

func (tc *TbtcChain) PastRedemptionRequestedEvents(
	filter *tbtc.RedemptionRequestedEventFilter,
) ([]*tbtc.RedemptionRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var redeemers []common.Address
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock

		for _, r := range filter.Redeemer {
			redeemers = append(redeemers, common.HexToAddress(r.String()))
		}

		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastRedemptionRequestedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
		redeemers,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.RedemptionRequestedEvent, 0)
	for _, event := range events {
		redeemerOutputScript, err := bitcoin.NewScriptFromVarLenData(
			event.RedeemerOutputScript,
		)
		if err != nil {
			return nil, err
		}

		convertedEvent := &tbtc.RedemptionRequestedEvent{
			WalletPublicKeyHash:  event.WalletPubKeyHash,
			RedeemerOutputScript: redeemerOutputScript,
			Redeemer:             chain.Address(event.Redeemer.Hex()),
			RequestedAmount:      event.RequestedAmount,
			TreasuryFee:          event.TreasuryFee,
			TxMaxFee:             event.TreasuryFee,
			BlockNumber:          event.Raw.BlockNumber,
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

func (tc *TbtcChain) BuildRedemptionKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*big.Int, error) {
	return buildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
}

func (tc *TbtcChain) GetPendingRedemptionRequest(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*tbtc.RedemptionRequest, bool, error) {
	redemptionKey, err := buildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
	if err != nil {
		return nil, false, fmt.Errorf("cannot build redemption key: [%v]", err)
	}

	redemptionRequest, err := tc.bridge.PendingRedemptions(redemptionKey)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get pending redemption request for key [0x%x]: [%v]",
			redemptionKey,
			err,
		)
	}

	// Redemption not found.
	if redemptionRequest.RequestedAt == 0 {
		return nil, false, nil
	}

	return &tbtc.RedemptionRequest{
		Redeemer:             chain.Address(redemptionRequest.Redeemer.Hex()),
		RedeemerOutputScript: redeemerOutputScript,
		RequestedAmount:      redemptionRequest.RequestedAmount,
		TreasuryFee:          redemptionRequest.TreasuryFee,
		TxMaxFee:             redemptionRequest.TxMaxFee,
		RequestedAt:          time.Unix(int64(redemptionRequest.RequestedAt), 0),
	}, true, nil
}

func (tc *TbtcChain) SubmitRedemptionProofWithReimbursement(
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
	redemptionProof := tbtcabi.BitcoinTxProof2{
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

	gasEstimate, err := tc.maintainerProxy.SubmitRedemptionProofGasEstimate(
		bitcoinTxInfo,
		redemptionProof,
		utxo,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

	_, err = tc.maintainerProxy.SubmitRedemptionProof(
		bitcoinTxInfo,
		redemptionProof,
		utxo,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func buildRedemptionKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*big.Int, error) {
	// The Bridge contract builds the redemption key using the length-prefixed
	// redeemer output script.
	prefixedRedeemerOutputScript, err := redeemerOutputScript.ToVarLenData()
	if err != nil {
		return nil, fmt.Errorf("cannot build prefixed redeemer output script: [%v]", err)
	}

	redeemerOutputScriptHash := crypto.Keccak256Hash(prefixedRedeemerOutputScript)

	redemptionKey := crypto.Keccak256Hash(
		append(redeemerOutputScriptHash[:], walletPublicKeyHash[:]...),
	)

	return redemptionKey.Big(), nil
}

func (tc *TbtcChain) GetRedemptionParameters() (tbtc.RedemptionParameters, error) {
	parameters, err := tc.bridge.RedemptionParameters()
	if err != nil {
		return tbtc.RedemptionParameters{}, err
	}

	return tbtc.RedemptionParameters{
		DustThreshold:                   parameters.RedemptionDustThreshold,
		TreasuryFeeDivisor:              parameters.RedemptionTreasuryFeeDivisor,
		TxMaxFee:                        parameters.RedemptionTxMaxFee,
		TxMaxTotalFee:                   parameters.RedemptionTxMaxTotalFee,
		Timeout:                         parameters.RedemptionTimeout,
		TimeoutSlashingAmount:           parameters.RedemptionTimeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier: parameters.RedemptionTimeoutNotifierRewardMultiplier,
	}, nil
}

func (tc *TbtcChain) ValidateRedemptionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) error {
	abiProposal, err := convertRedemptionProposalToAbiType(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return fmt.Errorf("cannot convert proposal to abi type: [%v]", err)
	}

	valid, err := tc.walletProposalValidator.ValidateRedemptionProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateRedemptionProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func convertRedemptionProposalToAbiType(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) (tbtcabi.WalletProposalValidatorRedemptionProposal, error) {
	redeemersOutputScripts := make(
		[][]byte,
		len(proposal.RedeemersOutputScripts),
	)

	for i, script := range proposal.RedeemersOutputScripts {
		// The on-chain script representation must be prepended with the script's
		// byte-length while bitcoin.Script is not. We need to add the
		// length prefix.
		prefixedScript, err := script.ToVarLenData()
		if err != nil {
			return tbtcabi.WalletProposalValidatorRedemptionProposal{}, fmt.Errorf(
				"cannot convert redeemer output script: [%v]",
				err,
			)
		}

		redeemersOutputScripts[i] = prefixedScript
	}

	return tbtcabi.WalletProposalValidatorRedemptionProposal{
		WalletPubKeyHash:       walletPublicKeyHash,
		RedeemersOutputScripts: redeemersOutputScripts,
		RedemptionTxFee:        proposal.RedemptionTxFee,
	}, nil
}

func (tc *TbtcChain) GetRedemptionMaxSize() (uint16, error) {
	return tc.walletProposalValidator.REDEMPTIONMAXSIZE()
}

func (tc *TbtcChain) GetRedemptionRequestMinAge() (uint32, error) {
	return tc.walletProposalValidator.REDEMPTIONREQUESTMINAGE()
}

func (tc *TbtcChain) GetRedemptionDelay(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (time.Duration, error) {
	if tc.redemptionWatchtower == nil {
		return 0, nil
	}

	redemptionKey, err := tc.BuildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
	if err != nil {
		return 0, fmt.Errorf("cannot build redemption key: [%v]", err)
	}

	delay, err := tc.redemptionWatchtower.GetRedemptionDelay(redemptionKey)
	if err != nil {
		return 0, fmt.Errorf("cannot get redemption delay: [%v]", err)
	}

	return time.Duration(delay) * time.Second, nil
}
