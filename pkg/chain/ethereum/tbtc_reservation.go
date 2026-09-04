// tbtc_reservation.go: TbtcChain reservation adapter. Implements the
// reservation-router surface - reservations/actions/parameters reads,
// request and notify transactions, and reservation event subscriptions -
// via the reservationRouter abigen binding constructed against the Bridge
// address (see reservationRouterBinding below).
package ethereum

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationRouterBinding constructs the ReservationRouter abigen binding
// pointed at the Bridge address. The router code only ever executes via
// Bridge.fallback's delegatecall, so the binding MUST be constructed against
// the Bridge address: the deployed router contract holds its own empty
// storage, so any call routed to its standalone address would either revert
// (writes) or return zeros (views); events emitted by router code carry the
// Bridge's address in their log because delegatecall preserves the caller's
// address context. The router's own deployment address is only needed for
// the one-time governance Bridge.setReservationRouter(routerAddress) call,
// which is out of scope here.
func reservationRouterBinding(
	bridgeAddress common.Address,
	baseChain *baseChain,
) (*tbtccontract.ReservationRouter, error) {
	return tbtccontract.NewReservationRouter(
		bridgeAddress,
		baseChain.chainID,
		baseChain.key,
		baseChain.client,
		baseChain.nonceManager,
		baseChain.miningWaiter,
		baseChain.blockCounter,
		baseChain.transactionMutex,
	)
}

// GetReservation returns the on-chain reservation record for the given
// reservation key via the reservationRouter binding (see reservationRouterBinding).
func (tc *TbtcChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	abiReservation, err := tc.reservationRouter.Reservations(reservationKey)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation [0x%x]: [%v]",
			reservationKey,
			err,
		)
	}

	reservation, err := convertReservationFromAbiType(abiReservation)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation [0x%x] from abi type: [%v]",
			reservationKey,
			err,
		)
	}

	return reservation, nil
}

// GetReservationAction returns the on-chain action record for the given
// reservation key and request nonce.
func (tc *TbtcChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	abiAction, err := tc.reservationRouter.ReservationActions(
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation action [0x%x:%d]: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	action, err := convertReservationActionFromAbiType(abiAction)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation action [0x%x:%d] from abi type: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	return action, nil
}

// ReservationParameters returns the current on-chain Bridge reservation
// parameters (10-tuple) via the reservationRouter binding (see reservationRouterBinding).
func (tc *TbtcChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	abiParameters, err := tc.reservationRouter.ReservationParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation parameters: [%v]",
			err,
		)
	}

	return convertReservationParametersFromAbiType(abiParameters), nil
}

// TODO(test-coverage): ValidateReservationAnchorProposal has no direct unit
// test coverage. It requires go-ethereum simulated-backend infrastructure
// that does not exist anywhere in pkg/chain/ethereum today; blocked on that
// infra landing. See PR #4280 and its linked gap-analysis doc.
// ValidateReservationAnchorProposal asks the WalletProposalValidator
// whether the given anchor proposal is valid for the given wallet and
// reserved deposit. The validator is a separate contract reached at its
// own deployed address.
func (tc *TbtcChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	abiProposal, abiExtraInfo := buildReservationAnchorProposalAbi(
		walletPublicKeyHash,
		proposal,
		depositExtraInfo,
	)

	valid, err := tc.walletProposalValidator.ValidateReservationAnchorProposal(
		abiProposal,
		abiExtraInfo,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateReservationAnchorProposal`
	// returns true or reverts (returns an error) but do the check just in
	// case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

// buildReservationAnchorProposalAbi constructs the ABI-struct arguments
// for WalletProposalValidator.ValidateReservationAnchorProposal from their
// application-level representations. Extracted as a pure function from
// ValidateReservationAnchorProposal so the field mapping can be unit
// tested directly, mirroring the reverse-direction converters below
// (convertReservationFromAbiType et al.).
func buildReservationAnchorProposalAbi(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) (
	tbtcabi.WalletProposalValidatorReservationAnchorProposal,
	tbtcabi.WalletProposalValidatorDepositExtraInfo,
) {
	// WalletProposalValidator's DepositExtraInfo.FundingTx is typed as
	// BitcoinTxInfo2 because the BitcoinTxInfo struct is renamed via the
	// collision hook in gen/Makefile (Bridge keeps the un-suffixed name;
	// WalletProposalValidator gets the 2-suffix; MaintainerProxy the 3;
	// ReservationRouter the 4). Mirroring the existing
	// ValidateDepositSweepProposal pattern.
	fundingTx := tbtcabi.BitcoinTxInfo2{
		Version:      depositExtraInfo.FundingTx.SerializeVersion(),
		InputVector:  depositExtraInfo.FundingTx.SerializeInputs(),
		OutputVector: depositExtraInfo.FundingTx.SerializeOutputs(),
		Locktime:     depositExtraInfo.FundingTx.SerializeLocktime(),
	}

	depositKey := tbtcabi.WalletProposalValidatorDepositKey{
		FundingTxHash:      proposal.DepositFundingTxHash,
		FundingOutputIndex: proposal.DepositFundingOutputIndex,
	}

	abiExtraInfo := tbtcabi.WalletProposalValidatorDepositExtraInfo{
		FundingTx:        fundingTx,
		BlindingFactor:   depositExtraInfo.Deposit.BlindingFactor,
		WalletPubKeyHash: depositExtraInfo.Deposit.WalletPublicKeyHash,
		RefundPubKeyHash: depositExtraInfo.Deposit.RefundPublicKeyHash,
		RefundLocktime:   depositExtraInfo.Deposit.RefundLocktime,
	}

	abiProposal := tbtcabi.WalletProposalValidatorReservationAnchorProposal{
		WalletPubKeyHash: walletPublicKeyHash,
		DepositKey:       depositKey,
		AnchorTxFee:      proposal.AnchorTxFee,
	}

	return abiProposal, abiExtraInfo
}

// ValidateReservationReanchorProposal asks the WalletProposalValidator
// whether the given re-anchor proposal is valid for the given source
// wallet. The validator is a separate contract reached at its own deployed
// address.
func (tc *TbtcChain) ValidateReservationReanchorProposal(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) error {
	abiProposal := buildReservationReanchorProposalAbi(
		sourceWalletPublicKeyHash,
		proposal,
	)

	valid, err := tc.walletProposalValidator.ValidateReservationReanchorProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateReservationReanchorProposal`
	// returns true or reverts (returns an error) but do the check just in
	// case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

// buildReservationReanchorProposalAbi constructs the ABI-struct argument
// for WalletProposalValidator.ValidateReservationReanchorProposal from its
// application-level representation. Extracted as a pure function from
// ValidateReservationReanchorProposal so the field mapping can be unit
// tested directly.
func buildReservationReanchorProposalAbi(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) tbtcabi.WalletProposalValidatorReservationReanchorProposal {
	return tbtcabi.WalletProposalValidatorReservationReanchorProposal{
		SourceWalletPubKeyHash: sourceWalletPublicKeyHash,
		ReservationKey:         proposal.ReservationKey,
		TargetWalletPubKeyHash: proposal.TargetWalletPublicKeyHash,
		ReanchorTxFee:          proposal.ReanchorTxFee,
	}
}

// convertReservationFromAbiType converts the ReservationRouter-specific
// Reservation.ReservationRequest ABI struct to the TBTC application
// `tbtc.Reservation` representation.
//
// Field omissions (intentional, mirroring the Solidity-to-Go struct shrink):
//
//   - `CumulativeReanchorFee`: written by every re-anchor hop but not
//     exposed through the Go-side reservation; m1 has no fee-ceiling
//     enforcement, so the field is dropped on the Go boundary. A later
//     milestone that adds a fee ceiling should re-export this field on
//     `tbtc.Reservation`.
//
// Anchor shape reassembly: the on-chain request splits the anchor UTXO into
// `anchorAmount`, `anchorTxHash`, and `anchorTxOutputIndex`; the Go-side
// representation folds those three back into a single
// `bitcoin.UnspentTransactionOutput` for consistency with the rest of the
// reservation API.
func convertReservationFromAbiType(
	abiReservation tbtcabi.ReservationReservationRequest,
) (*tbtc.Reservation, error) {
	state, err := parseReservationState(abiReservation.State)
	if err != nil {
		return nil, fmt.Errorf("cannot parse reservation state: [%v]", err)
	}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: abiReservation.AnchorTxHash,
			OutputIndex:     abiReservation.AnchorTxOutputIndex,
		},
		Value: int64(abiReservation.AnchorAmount),
	}

	return &tbtc.Reservation{
		Owner:                 chain.Address(abiReservation.Owner.String()),
		MintedAmount:          abiReservation.MintedAmount,
		AcceptedAt:            abiReservation.AcceptedAt,
		WalletPublicKeyHash:   abiReservation.WalletPubKeyHash,
		AnchorUtxo:            anchorUtxo,
		ExpiresAt:             abiReservation.ExpiresAt,
		State:                 state,
		RequestNonce:          abiReservation.RequestNonce,
		RetryCredit:           abiReservation.RetryCredit,
		DissolutionEligibleAt: abiReservation.DissolutionEligibleAt,
	}, nil
}

// convertReservationActionFromAbiType converts the ReservationRouter-
// specific Reservation.ReservationAction ABI struct to the TBTC
// application `tbtc.ReservationAction` representation.
//
// Field omissions (intentional):
//
//   - `SourceAnchorUtxoHash`, `UsedRetryCredit`,
//     `Watchtower{Default,LevelOne,LevelTwo}Delay`,
//     `RetryCreditSourceNonce`: written for governance / late-settlement
//     reconciliation but not read by the operator client in m1.
//
// The on-chain `actionDataHash` field is polymorphic across action types:
// it carries the keccak256 of the redeemer output script for redemptions,
// the wallet main UTXO hash for dissolutions, and is zero otherwise. The
// Go-side struct splits that polymorphism into two named fields
// (`RedeemerOutputScriptHash` for redemptions, `ExpectedMainUtxoHash`
// for dissolutions); we route `actionDataHash` to the field that matches
// the action's type and zero the other.
func convertReservationActionFromAbiType(
	abiAction tbtcabi.ReservationReservationAction,
) (*tbtc.ReservationAction, error) {
	actionType, err := parseReservationActionType(abiAction.ActionType)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action type: [%v]",
			err,
		)
	}

	state, err := parseReservationActionState(abiAction.State)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action state: [%v]",
			err,
		)
	}

	var (
		redeemerOutputScriptHash [32]byte
		expectedMainUtxoHash     [32]byte
	)
	switch actionType {
	case tbtc.ReservationActionTypeRedemption:
		redeemerOutputScriptHash = abiAction.ActionDataHash
	case tbtc.ReservationActionTypeDissolution:
		expectedMainUtxoHash = abiAction.ActionDataHash
	}

	return &tbtc.ReservationAction{
		TargetWalletPublicKeyHash: abiAction.TargetWalletPubKeyHash,
		RequestedAt:               abiAction.RequestedAt,
		TimeoutAt:                 abiAction.TimeoutAt,
		TxMaxFee:                  abiAction.TxMaxFee,
		ActionType:                actionType,
		State:                     state,
		FeePaid:                   abiAction.FeePaid,
		Redeemer:                  chain.Address(abiAction.Redeemer.String()),
		Amount:                    abiAction.Amount,
		RedeemerOutputScriptHash:  redeemerOutputScriptHash,
		ExpectedMainUtxoHash:      expectedMainUtxoHash,
		IsPartial:                 abiAction.IsPartial,
	}, nil
}

// convertReservationParametersFromAbiType converts the ReservationRouter
// 10-tuple to the `tbtc.ReservationParameters` representation.
func convertReservationParametersFromAbiType(
	abiParameters struct {
		ReservationVault                common.Address
		ReservationMinAmount            uint64
		ReservationTxMaxFee             uint64
		ReservationTermSeconds          uint32
		ReservationDissolutionDelay     uint32
		ReservationMaxTotalAmount       uint64
		ReservationTotalAmount          uint64
		MaxReservationsPerWallet        uint32
		ReservationActionTimeout        uint32
		ReservationRenewalWindowSeconds uint32
	},
) *tbtc.ReservationParameters {
	return &tbtc.ReservationParameters{
		ReservationVault:                chain.Address(abiParameters.ReservationVault.String()),
		ReservationMinAmount:            abiParameters.ReservationMinAmount,
		ReservationTxMaxFee:             abiParameters.ReservationTxMaxFee,
		ReservationTermSeconds:          abiParameters.ReservationTermSeconds,
		ReservationDissolutionDelay:     abiParameters.ReservationDissolutionDelay,
		ReservationMaxTotalAmount:       abiParameters.ReservationMaxTotalAmount,
		ReservationTotalAmount:          abiParameters.ReservationTotalAmount,
		MaxReservationsPerWallet:        abiParameters.MaxReservationsPerWallet,
		ReservationActionTimeout:        abiParameters.ReservationActionTimeout,
		ReservationRenewalWindowSeconds: abiParameters.ReservationRenewalWindowSeconds,
	}
}

// parseReservationState converts the on-chain ReservationState enum
// (uint8) to the tbtc.ReservationState value. Values match the Solidity
// declaration one-for-one (Unknown=0, Active=1, ActionPending=2,
// Closed=3, Stranded=4).
func parseReservationState(value uint8) (tbtc.ReservationState, error) {
	switch value {
	case 0:
		return tbtc.ReservationStateUnknown, nil
	case 1:
		return tbtc.ReservationStateActive, nil
	case 2:
		return tbtc.ReservationStateActionPending, nil
	case 3:
		return tbtc.ReservationStateClosed, nil
	case 4:
		return tbtc.ReservationStateStranded, nil
	default:
		return 0, fmt.Errorf("unexpected reservation state value: [%d]", value)
	}
}

// parseReservationActionType converts the on-chain ActionType enum
// (uint8) to the tbtc.ReservationActionType value. Values match the
// Solidity declaration one-for-one (None=0, Acceptance=1, Redemption=2,
// Reanchor=3, Dissolution=4).
func parseReservationActionType(value uint8) (tbtc.ReservationActionType, error) {
	switch value {
	case 0:
		return tbtc.ReservationActionTypeNone, nil
	case 1:
		return tbtc.ReservationActionTypeAcceptance, nil
	case 2:
		return tbtc.ReservationActionTypeRedemption, nil
	case 3:
		return tbtc.ReservationActionTypeReanchor, nil
	case 4:
		return tbtc.ReservationActionTypeDissolution, nil
	default:
		return 0, fmt.Errorf("unexpected reservation action type value: [%d]", value)
	}
}

// parseReservationActionState converts the on-chain ActionState enum
// (uint8) to the tbtc.ReservationActionState value. Values match the
// Solidity declaration one-for-one (Unknown=0, Pending=1, Settled=2,
// TimedOut=3, Vetoed=4, Superseded=5).
func parseReservationActionState(value uint8) (tbtc.ReservationActionState, error) {
	switch value {
	case 0:
		return tbtc.ReservationActionStateUnknown, nil
	case 1:
		return tbtc.ReservationActionStatePending, nil
	case 2:
		return tbtc.ReservationActionStateSettled, nil
	case 3:
		return tbtc.ReservationActionStateTimedOut, nil
	case 4:
		return tbtc.ReservationActionStateVetoed, nil
	case 5:
		return tbtc.ReservationActionStateSuperseded, nil
	default:
		return 0, fmt.Errorf("unexpected reservation action state value: [%d]", value)
	}
}

// RequestReservationAcceptance calls the Bridge (via reservationRouter binding,
// see reservationRouterBinding) to start a new reservation acceptance action
// generation for the given reservation.
func (tc *TbtcChain) RequestReservationAcceptance(
	reservationKey *big.Int,
	walletPublicKeyHash [20]byte,
) error {
	gasEstimate, err := tc.reservationRouter.RequestReservationAcceptanceGasEstimate(
		reservationKey,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.RequestReservationAcceptance(
		reservationKey,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// RequestReservationReanchor asks the Bridge (via its ReservationRouter
// delegatecall target) to start a new reservation re-anchor action generation
// for the given reservation, targeting the given wallet.
func (tc *TbtcChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
) error {
	gasEstimate, err := tc.reservationRouter.RequestReservationReanchorGasEstimate(
		reservationKey,
		targetWalletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// SubmitReservationProof submits an SPV proof for the given reservation
// action generation to the Bridge. The proof path is onlySpvMaintainer on
// the router; the call goes through Bridge.fallback's delegatecall so the
// router code reads the Bridge's isSpvMaintainer mapping at the Bridge's
// address.
func (tc *TbtcChain) SubmitReservationProof(
	proofType uint8,
	txInfo *tbtc.BitcoinTxInfo,
	proof *tbtc.BitcoinTxProof,
	mainUtxo *tbtc.BitcoinTxUTXO,
	reservationKey *big.Int,
	requestNonce uint64,
) error {
	abiTxInfo := tbtcabi.BitcoinTxInfo4{
		Version:      txInfo.Version,
		InputVector:  txInfo.InputVector,
		OutputVector: txInfo.OutputVector,
		Locktime:     txInfo.Locktime,
	}
	abiProof := tbtcabi.BitcoinTxProof3{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   proof.TxIndexInBlock,
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	abiUtxo := tbtcabi.BitcoinTxUTXO4{
		TxHash:        mainUtxo.TxHash,
		TxOutputIndex: mainUtxo.TxOutputIndex,
		TxOutputValue: mainUtxo.TxOutputValue,
	}

	gasEstimate, err := tc.reservationRouter.SubmitReservationProofGasEstimate(
		proofType,
		abiTxInfo,
		abiProof,
		abiUtxo,
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low; the
	// reservation proof path dispatches into ReservationProofs.submit*Proof,
	// which performs a non-trivial amount of storage I/O. Apply a 20%
	// margin mirroring the existing SubmitRedemptionProofWithReimbursement
	// pattern in this file.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.SubmitReservationProof(
		proofType,
		abiTxInfo,
		abiProof,
		abiUtxo,
		reservationKey,
		requestNonce,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyReservationActionTimeout notifies the Bridge that the timeout for
// the given reservation action generation has elapsed.
func (tc *TbtcChain) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationActionTimeoutGasEstimate(
		reservationKey,
		walletMembersIDs,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationActionTimeout(
		reservationKey,
		walletMembersIDs,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyStaleReservedDeposit notifies the Bridge that the given reserved
// deposit's wallet did not anchor it within the reservation-action timeout.
func (tc *TbtcChain) NotifyStaleReservedDeposit(
	depositKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyStaleReservedDepositGasEstimate(
		depositKey,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyStaleReservedDeposit(
		depositKey,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyReservationStranded notifies the Bridge that the wallet custodying
// the given reservation has been closed or terminated.
func (tc *TbtcChain) NotifyReservationStranded(
	reservationKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationStrandedGasEstimate(
		reservationKey,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationStranded(
		reservationKey,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyMovingFundsBelowDust notifies the Bridge that the given wallet's
// main UTXO has fallen below the moving funds dust threshold, ending the
// moving funds process and starting wallet closing immediately. This call
// is permissionless on-chain (MovingFunds.sol's notifyMovingFundsBelowDust
// carries no caller restriction), so it is submitted directly through the
// Bridge rather than routed through MaintainerProxy for reimbursement,
// mirroring the other reservation notify/request calls in this file.
func (tc *TbtcChain) NotifyMovingFundsBelowDust(
	walletPublicKeyHash [20]byte,
	mainUtxo *bitcoin.UnspentTransactionOutput,
) error {
	var utxo tbtcabi.BitcoinTxUTXO
	if mainUtxo != nil {
		utxo = tbtcabi.BitcoinTxUTXO{
			TxHash:        mainUtxo.Outpoint.TransactionHash,
			TxOutputIndex: mainUtxo.Outpoint.OutputIndex,
			TxOutputValue: uint64(mainUtxo.Value),
		}
	}

	gasEstimate, err := tc.bridge.NotifyMovingFundsBelowDustGasEstimate(
		walletPublicKeyHash,
		utxo,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems, mirroring the
	// other reservation notify calls in this file.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.bridge.NotifyMovingFundsBelowDust(
		walletPublicKeyHash,
		utxo,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// ReservationCaps returns the cap parameters that gate reservation
// acceptance via the reservationRouter binding (see reservationRouterBinding).
func (tc *TbtcChain) ReservationCaps() (
	uint64,
	uint64,
	error,
) {
	caps, err := tc.reservationRouter.ReservationCaps()
	if err != nil {
		return 0, 0, fmt.Errorf(
			"cannot get reservation caps: [%v]",
			err,
		)
	}

	return caps.MaxReservationsAmountPerWallet, caps.ReservationMaxSingleAmount, nil
}

// WalletReservationsAmount returns the aggregate satoshi amount currently
// anchored by the given wallet across all of its reservations.
func (tc *TbtcChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	amount, err := tc.reservationRouter.WalletReservationsAmount(walletPublicKeyHash)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get wallet reservations amount for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return amount, nil
}

// WalletReservationsCount returns the number of reservations currently
// custodied by the given wallet.
func (tc *TbtcChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	count, err := tc.reservationRouter.WalletReservationsCount(walletPublicKeyHash)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get wallet reservations count for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return count, nil
}

// WalletReservations returns the reservation keys for all reservations
// currently custodied by the given wallet.
func (tc *TbtcChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	keys, err := tc.reservationRouter.WalletReservations(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get wallet reservations for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return keys, nil
}

// ReservedDepositWallet returns the wallet public key hash to which the
// given reserved deposit was revealed. Returns the zero hash if the
// deposit is not a reserved deposit.
func (tc *TbtcChain) ReservedDepositWallet(
	depositKey *big.Int,
) ([20]byte, error) {
	walletPublicKeyHash, err := tc.reservationRouter.ReservedDepositWallet(depositKey)
	if err != nil {
		return [20]byte{}, fmt.Errorf(
			"cannot get reserved deposit wallet for [0x%x]: [%v]",
			depositKey,
			err,
		)
	}

	return walletPublicKeyHash, nil
}

// ActiveReservationsCount returns the current count of active reservations
// across all wallets and the cap on that count.
func (tc *TbtcChain) ActiveReservationsCount() (uint32, uint32, error) {
	activeReservationsCount, err := tc.reservationRouter.ActiveReservationsCount()
	if err != nil {
		return 0, 0, fmt.Errorf(
			"cannot get active reservations count: [%v]",
			err,
		)
	}

	return activeReservationsCount.Count, activeReservationsCount.MaxActive, nil
}

// IsReservedDeposit returns true if the given deposit was revealed with
// the reservation vault address and is therefore a reservation rather than
// a default deposit.
func (tc *TbtcChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	isReserved, err := tc.bridge.IsReservedDeposit(depositKey)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check if deposit [0x%x] is reserved: [%v]",
			depositKey,
			err,
		)
	}

	return isReserved, nil
}

// OnReservationAcceptanceRequested registers a callback that is invoked
// when an on-chain ReservationAcceptanceRequested event is seen. The
// subscription filters against the Bridge's address (the binding is bound
// to the Bridge address; delegatecall preserves the caller's address
// context so events emitted by router code carry the Bridge's address).
func (tc *TbtcChain) OnReservationAcceptanceRequested(
	handler func(event *tbtc.ReservationAcceptanceRequestedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		walletPublicKeyHash [20]byte,
		depositAmount uint64,
		txMaxFee uint64,
		timeoutAt uint32,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      reservationKey,
			RequestNonce:        requestNonce,
			WalletPublicKeyHash: walletPublicKeyHash,
			DepositAmount:       depositAmount,
			TxMaxFee:            txMaxFee,
			TimeoutAt:           timeoutAt,
			BlockNumber:         blockNumber,
		})
	}

	return tc.reservationRouter.ReservationAcceptanceRequestedEvent(
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationAcceptanceRequestedEvents fetches past
// ReservationAcceptanceRequested events according to the provided filter
// or unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.reservationRouter.PastReservationAcceptanceRequestedEvents(
		startBlock,
		endBlock,
		reservationKey,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationAcceptanceRequestedEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      event.ReservationKey,
			RequestNonce:        event.RequestNonce,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			DepositAmount:       event.DepositAmount,
			TxMaxFee:            event.TxMaxFee,
			TimeoutAt:           event.TimeoutAt,
			BlockNumber:         event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationReanchorRequested registers a callback that is invoked
// when an on-chain ReservationReanchorRequested event is seen.
func (tc *TbtcChain) OnReservationReanchorRequested(
	handler func(event *tbtc.ReservationReanchorRequestedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		sourceWalletPublicKeyHash [20]byte,
		targetWalletPublicKeyHash [20]byte,
		txMaxFee uint64,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            reservationKey,
			RequestNonce:              requestNonce,
			SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
			TxMaxFee:                  txMaxFee,
			BlockNumber:               blockNumber,
		})
	}

	return tc.reservationRouter.ReservationReanchorRequestedEvent(
		nil,
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationReanchorRequestedEvents fetches past
// ReservationReanchorRequested events according to the provided filter or
// unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var sourceWalletPublicKeyHash [][20]byte
	var targetWalletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		sourceWalletPublicKeyHash = filter.SourceWalletPublicKeyHash
		targetWalletPublicKeyHash = filter.TargetWalletPublicKeyHash
	}

	events, err := tc.reservationRouter.PastReservationReanchorRequestedEvents(
		startBlock,
		endBlock,
		reservationKey,
		sourceWalletPublicKeyHash,
		targetWalletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationReanchorRequestedEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            event.ReservationKey,
			RequestNonce:              event.RequestNonce,
			SourceWalletPublicKeyHash: event.SourceWalletPubKeyHash,
			TargetWalletPublicKeyHash: event.TargetWalletPubKeyHash,
			TxMaxFee:                  event.TxMaxFee,
			BlockNumber:               event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}
