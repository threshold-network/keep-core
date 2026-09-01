package tbtc

import (
	"fmt"
	"math/big"

	"golang.org/x/crypto/sha3"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

const (
	// reservationAnchorProposalValidityBlocks determines the reservation
	// anchor proposal validity time expressed in blocks.
	reservationAnchorProposalValidityBlocks = 600
	// reservedRedemptionProposalValidityBlocks determines the reserved
	// redemption proposal validity time expressed in blocks.
	reservedRedemptionProposalValidityBlocks = 600
	// reservationReanchorProposalValidityBlocks determines the reservation
	// re-anchor proposal validity time expressed in blocks.
	reservationReanchorProposalValidityBlocks = 600
	// reservationDissolutionProposalValidityBlocks determines the reservation
	// dissolution proposal validity time expressed in blocks.
	reservationDissolutionProposalValidityBlocks = 600
)

// ReservationState represents the state of an on-chain UTXO reservation.
type ReservationState uint8

const (
	// ReservationStateUnknown means the reservation is unknown to the Bridge.
	ReservationStateUnknown ReservationState = iota
	// ReservationStateActive means the reservation's anchor outpoint is under
	// wallet custody with no action in flight.
	ReservationStateActive
	// ReservationStateActionPending means a redemption, re-anchor, or
	// dissolution action is pending. The action details are held in the
	// nonce-keyed ReservationAction record.
	ReservationStateActionPending
	// ReservationStateClosed means the reservation was closed by an in-kind
	// redemption, dissolution, or late settlement.
	ReservationStateClosed
	// ReservationStateStranded means the custodying wallet was terminated
	// while the anchor was outstanding and the anchor is no longer tracked.
	ReservationStateStranded
)

// Reservation represents an on-chain UTXO reservation record. A reservation
// is a deposit that was anchored by the wallet - spent in a 1-input-1-output
// transaction into a fresh wallet-controlled output with no refund path -
// instead of being swept into the wallet main UTXO. The anchor outpoint is
// custodied without ever commingling with the pooled supply and is
// redeemable in-kind by the reservation owner.
type Reservation struct {
	// Owner is the reservation owner's address on the host chain.
	Owner chain.Address
	// MintedAmount is the gross amount in satoshi credited to the owner at
	// acceptance time.
	MintedAmount uint64
	// AcceptedAt is the UNIX timestamp the reservation was accepted at.
	AcceptedAt uint32
	// WalletPublicKeyHash is the 20-byte public key hash of the wallet
	// custodying the current anchor outpoint.
	WalletPublicKeyHash [20]byte
	// AnchorUtxo is the reservation's current anchor outpoint, i.e. the
	// wallet-controlled output holding the reserved coins.
	AnchorUtxo *bitcoin.UnspentTransactionOutput
	// ExpiresAt is the UNIX timestamp the custody term expires at.
	ExpiresAt uint32
	// State is the current state of the reservation.
	State ReservationState
	// RequestNonce is the current monotonic reservation action generation.
	RequestNonce uint64
	// RetryCredit indicates the owner has a single-use fee-free redemption
	// retry entitlement after a fee-paid redemption timed out.
	RetryCredit bool
	// DissolutionEligibleAt is the UNIX timestamp at which the current term
	// becomes eligible for dissolution.
	DissolutionEligibleAt uint32
}

// ReservationActionType represents the type of a reservation action
// generation.
type ReservationActionType uint8

const (
	ReservationActionTypeNone ReservationActionType = iota
	ReservationActionTypeAcceptance
	ReservationActionTypeRedemption
	ReservationActionTypeReanchor
	ReservationActionTypeDissolution
)

// ReservationActionState represents the settlement state of a reservation
// action generation.
type ReservationActionState uint8

const (
	ReservationActionStateUnknown ReservationActionState = iota
	ReservationActionStatePending
	ReservationActionStateSettled
	ReservationActionStateTimedOut
	ReservationActionStateVetoed
	ReservationActionStateSuperseded
)

// ReservationAction represents one nonce-bound generation of a reservation
// action. All authorization data used to construct and settle the action is
// snapshotted when the generation is requested.
type ReservationAction struct {
	// TargetWalletPublicKeyHash is the wallet an acceptance, re-anchor, or
	// dissolution output must pay to. It is zero for redemptions.
	TargetWalletPublicKeyHash [20]byte
	// RequestedAt is the UNIX timestamp the action was requested at.
	RequestedAt uint32
	// TimeoutAt is the UNIX timestamp after which the action may time out.
	TimeoutAt uint32
	// TxMaxFee is the snapshotted maximum Bitcoin transaction fee in satoshi.
	TxMaxFee uint64
	// ActionType is the type of this action generation.
	ActionType ReservationActionType
	// State is the settlement state of this action generation.
	State ReservationActionState
	// FeePaid indicates the generation was created through a fee-paying vault
	// entry point.
	FeePaid bool
	// Redeemer is the address that can reclaim escrow after a redemption
	// timeout. It is empty for other action types.
	Redeemer chain.Address
	// Amount is the satoshi amount associated with the action generation.
	Amount uint64
	// RedeemerOutputScriptHash is the keccak256 hash of the length-prefixed
	// output script authorized for a redemption.
	RedeemerOutputScriptHash [32]byte
	// ExpectedMainUtxoHash identifies the wallet main UTXO snapshotted for a
	// dissolution. It is zero for other action types and no-main-UTXO wallets.
	ExpectedMainUtxoHash [32]byte
	// IsPartial indicates a redemption spends only Amount and must re-anchor
	// the remaining reservation value back to the custodying wallet.
	IsPartial bool
}

// ReservationParameters represents the on-chain values of the Bridge
// reservation parameters.
type ReservationParameters struct {
	// ReservationVault is the address of the reservation vault. Deposits
	// revealed with this vault address are treated as UTXO reservations.
	ReservationVault chain.Address
	// ReservationMinAmount is the minimal anchor output amount in satoshi
	// accepted for a reservation.
	ReservationMinAmount uint64
	// ReservationTxMaxFee is the maximum transaction fee in satoshi for a
	// single reservation lifecycle transaction.
	ReservationTxMaxFee uint64
	// ReservationTermSeconds is the custody term length in seconds.
	ReservationTermSeconds uint32
	// ReservationDissolutionDelay is the delay snapshotted after term expiry
	// before a reservation becomes dissolvable.
	ReservationDissolutionDelay uint32
	// ReservationMaxTotalAmount is the maximum total amount of all active
	// reservations in satoshi.
	ReservationMaxTotalAmount uint64
	// ReservationTotalAmount is the current total amount of all active
	// reservations in satoshi.
	ReservationTotalAmount uint64
	// MaxReservationsPerWallet is the maximum number of reservations a wallet
	// may custody.
	MaxReservationsPerWallet uint32
	// ReservationActionTimeout is the timeout for reservation actions in
	// seconds.
	ReservationActionTimeout uint32
	// ReservationRenewalWindowSeconds is the period before expiry during which
	// a reservation can be renewed.
	ReservationRenewalWindowSeconds uint32
}

// ReservationAnchorProposal represents a reservation anchor proposal issued
// by a wallet's coordination leader.
type ReservationAnchorProposal struct {
	// DepositFundingTxHash is the funding transaction hash of the reserved
	// deposit to anchor.
	DepositFundingTxHash bitcoin.Hash
	// DepositFundingOutputIndex is the funding output index of the reserved
	// deposit to anchor.
	DepositFundingOutputIndex uint32
	// RequestNonce is the acceptance authorization generation being executed.
	RequestNonce uint64
	// AnchorTxFee is the proposed BTC fee for the anchor transaction.
	AnchorTxFee *big.Int
}

// ActionType returns the specific type of the walletAction being subject
// of this proposal.
func (rap *ReservationAnchorProposal) ActionType() WalletActionType {
	return ActionReservationAnchor
}

// ValidityBlocks returns the number of blocks for which the proposal is valid.
func (rap *ReservationAnchorProposal) ValidityBlocks() uint64 {
	return reservationAnchorProposalValidityBlocks
}

// ReservedRedemptionProposal represents a reserved redemption proposal
// issued by a wallet's coordination leader.
type ReservedRedemptionProposal struct {
	// ReservationKey is the key of the reservation with the pending reserved
	// redemption.
	ReservationKey *big.Int
	// RequestNonce is the redemption request generation being executed.
	RequestNonce uint64
	// RedemptionTxFee is the proposed BTC fee for the reserved redemption
	// transaction.
	RedemptionTxFee *big.Int
}

// ActionType returns the specific type of the walletAction being subject
// of this proposal.
func (rrp *ReservedRedemptionProposal) ActionType() WalletActionType {
	return ActionReservedRedemption
}

// ValidityBlocks returns the number of blocks for which the proposal is valid.
func (rrp *ReservedRedemptionProposal) ValidityBlocks() uint64 {
	return reservedRedemptionProposalValidityBlocks
}

// ReservationReanchorProposal represents a reservation re-anchor proposal
// issued by a wallet's coordination leader, moving a reservation's anchor
// outpoint to another wallet (e.g. during wallet migration).
type ReservationReanchorProposal struct {
	// ReservationKey is the key of the reservation to re-anchor.
	ReservationKey *big.Int
	// RequestNonce is the re-anchor authorization generation being executed.
	RequestNonce uint64
	// TargetWalletPublicKeyHash is the 20-byte public key hash of the wallet
	// receiving the anchor.
	TargetWalletPublicKeyHash [20]byte
	// ReanchorTxFee is the proposed BTC fee for the re-anchor transaction.
	ReanchorTxFee *big.Int
}

// ActionType returns the specific type of the walletAction being subject
// of this proposal.
func (rrp *ReservationReanchorProposal) ActionType() WalletActionType {
	return ActionReservationReanchor
}

// ValidityBlocks returns the number of blocks for which the proposal is valid.
func (rrp *ReservationReanchorProposal) ValidityBlocks() uint64 {
	return reservationReanchorProposalValidityBlocks
}

// ReservationDissolutionProposal represents a reservation dissolution
// proposal issued by a wallet's coordination leader once the reservation's
// custody term and grace period elapsed.
type ReservationDissolutionProposal struct {
	// ReservationKey is the key of the reservation to dissolve.
	ReservationKey *big.Int
	// RequestNonce is the dissolution authorization generation being executed.
	RequestNonce uint64
	// DissolutionTxFee is the proposed BTC fee for the dissolution
	// transaction.
	DissolutionTxFee *big.Int
}

// ActionType returns the specific type of the walletAction being subject
// of this proposal.
func (rdp *ReservationDissolutionProposal) ActionType() WalletActionType {
	return ActionReservationDissolution
}

// ValidityBlocks returns the number of blocks for which the proposal is valid.
func (rdp *ReservationDissolutionProposal) ValidityBlocks() uint64 {
	return reservationDissolutionProposalValidityBlocks
}

// assembleReservationAnchorTransaction constructs an unsigned reservation
// anchor transaction: a 1-input-1-output spend of the given reserved deposit
// into a fresh output controlled by the given wallet. The anchor mirrors the
// sweep's refund-disabling role without its consolidating role: the Bridge
// credits the reservation owner only against the SPV proof of this
// transaction.
func assembleReservationAnchorTransaction(
	bitcoinChain bitcoin.Chain,
	deposit *Deposit,
	walletPublicKeyHash [20]byte,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if deposit == nil {
		return nil, fmt.Errorf("deposit is required")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	depositScript, err := deposit.Script()
	if err != nil {
		return nil, fmt.Errorf("cannot get deposit script: [%v]", err)
	}

	err = builder.AddScriptHashInput(deposit.Utxo, depositScript)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to deposit UTXO: [%v]",
			err,
		)
	}

	anchorValue := deposit.Utxo.Value - fee
	if anchorValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the deposit value",
		)
	}

	anchorScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot compute anchor script: [%v]", err)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           anchorValue,
		PublicKeyScript: anchorScript,
	})

	return builder, nil
}

// assembleReservedRedemptionTransaction constructs an unsigned reserved
// redemption transaction for the given nonce-bound action. A whole redemption
// is a 1-input-1-output spend to the redeemer. A partial redemption is a
// 1-input-2-output spend whose first output pays the authorized amount less
// the miner fee to the redeemer and whose second output re-anchors the exact
// remainder to the custodying wallet.
func assembleReservedRedemptionTransaction(
	bitcoinChain bitcoin.Chain,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
	action *ReservationAction,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}
	if len(redeemerOutputScript) == 0 {
		return nil, fmt.Errorf("redeemer output script is required")
	}
	if action == nil {
		return nil, fmt.Errorf("reservation action is required")
	}
	if action.ActionType != ReservationActionTypeRedemption {
		return nil, fmt.Errorf("reservation action is not a redemption")
	}
	if action.State != ReservationActionStatePending {
		return nil, fmt.Errorf("reservation action is not pending")
	}
	if anchorUtxo.Value <= 0 {
		return nil, fmt.Errorf("anchor UTXO value must be positive")
	}
	if action.Amount == 0 {
		return nil, fmt.Errorf("redemption amount must be positive")
	}
	if action.Amount > uint64(anchorUtxo.Value) {
		return nil, fmt.Errorf("redemption amount exceeds the anchor value")
	}
	if fee <= 0 {
		return nil, fmt.Errorf("transaction fee must be positive")
	}
	if uint64(fee) > action.TxMaxFee {
		return nil, fmt.Errorf("transaction fee exceeds the action fee limit")
	}

	redeemerOutputScriptHash, err := computeReservationRedeemerOutputScriptHash(
		redeemerOutputScript,
	)
	if err != nil {
		return nil, err
	}
	if redeemerOutputScriptHash != action.RedeemerOutputScriptHash {
		return nil, fmt.Errorf("redeemer output script is not authorized")
	}

	if action.IsPartial {
		if action.Amount == uint64(anchorUtxo.Value) {
			return nil, fmt.Errorf(
				"partial redemption amount must be less than the anchor value",
			)
		}
	} else if action.Amount != uint64(anchorUtxo.Value) {
		return nil, fmt.Errorf(
			"whole redemption amount must equal the anchor value",
		)
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	err = builder.AddPublicKeyHashInput(anchorUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to anchor UTXO: [%v]",
			err,
		)
	}

	redemptionAmount := anchorUtxo.Value
	if action.IsPartial {
		redemptionAmount = int64(action.Amount)
	}

	redemptionValue := redemptionAmount - fee
	if redemptionValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the redemption amount",
		)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           redemptionValue,
		PublicKeyScript: redeemerOutputScript,
	})

	if action.IsPartial {
		remainderScript, err := bitcoin.PayToWitnessPublicKeyHash(
			walletPublicKeyHash,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot compute remainder script: [%v]", err)
		}

		builder.AddOutput(&bitcoin.TransactionOutput{
			Value:           anchorUtxo.Value - int64(action.Amount),
			PublicKeyScript: remainderScript,
		})
	}

	return builder, nil
}

// computeReservationRedeemerOutputScriptHash computes the authorization hash
// stored in a reservation action. The Bridge hashes the Bitcoin output script
// including its CompactSize length prefix.
func computeReservationRedeemerOutputScriptHash(
	redeemerOutputScript bitcoin.Script,
) ([32]byte, error) {
	prefixedScript, err := redeemerOutputScript.ToVarLenData()
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"cannot build prefixed redeemer output script: [%v]",
			err,
		)
	}

	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(prefixedScript)

	var result [32]byte
	copy(result[:], hasher.Sum(nil))

	return result, nil
}

// assembleReservationReanchorTransaction constructs an unsigned reservation
// re-anchor transaction: a 1-input-1-output spend of the reservation's
// anchor outpoint into a fresh output controlled by the target wallet. Used
// during wallet migration so reservations never pin retiring wallets.
func assembleReservationReanchorTransaction(
	bitcoinChain bitcoin.Chain,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	targetWalletPublicKeyHash [20]byte,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	err := builder.AddPublicKeyHashInput(anchorUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to anchor UTXO: [%v]",
			err,
		)
	}

	reanchorValue := anchorUtxo.Value - fee
	if reanchorValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the anchor value",
		)
	}

	reanchorScript, err := bitcoin.PayToWitnessPublicKeyHash(
		targetWalletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot compute re-anchor script: [%v]", err)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           reanchorValue,
		PublicKeyScript: reanchorScript,
	})

	return builder, nil
}

// assembleReservationDissolutionTransaction constructs an unsigned
// reservation dissolution transaction for the given nonce-bound action. The
// anchor outpoint is the first input. The wallet main UTXO is the second input
// only when it is present in the action snapshot and matches that snapshot
// exactly. The single output pays back to the custodying wallet.
func assembleReservationDissolutionTransaction(
	bitcoinChain bitcoin.Chain,
	bridgeChain BridgeChain,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
	action *ReservationAction,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}
	if action == nil {
		return nil, fmt.Errorf("reservation action is required")
	}
	if action.ActionType != ReservationActionTypeDissolution {
		return nil, fmt.Errorf("reservation action is not a dissolution")
	}
	if action.State != ReservationActionStatePending {
		return nil, fmt.Errorf("reservation action is not pending")
	}
	if action.TargetWalletPublicKeyHash != walletPublicKeyHash {
		return nil, fmt.Errorf("dissolution action targets a different wallet")
	}
	if anchorUtxo.Value <= 0 {
		return nil, fmt.Errorf("anchor UTXO value must be positive")
	}
	if action.Amount != uint64(anchorUtxo.Value) {
		return nil, fmt.Errorf(
			"dissolution action amount does not match the anchor value",
		)
	}
	if fee <= 0 {
		return nil, fmt.Errorf("transaction fee must be positive")
	}
	if uint64(fee) > action.TxMaxFee {
		return nil, fmt.Errorf("transaction fee exceeds the action fee limit")
	}

	mainUtxoExpected := action.ExpectedMainUtxoHash != [32]byte{}
	if mainUtxoExpected {
		if bridgeChain == nil {
			return nil, fmt.Errorf("bridge chain is required")
		}
		if walletMainUtxo == nil {
			return nil, fmt.Errorf(
				"wallet main UTXO is required by the dissolution action",
			)
		}
		if bridgeChain.ComputeMainUtxoHash(walletMainUtxo) !=
			action.ExpectedMainUtxoHash {
			return nil, fmt.Errorf(
				"wallet main UTXO does not match the dissolution action snapshot",
			)
		}
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	// The Bridge requires the anchor outpoint to be the first input.
	err := builder.AddPublicKeyHashInput(anchorUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to anchor UTXO: [%v]",
			err,
		)
	}

	totalInputsValue := anchorUtxo.Value

	if mainUtxoExpected {
		err = builder.AddPublicKeyHashInput(walletMainUtxo)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot add input pointing to wallet main UTXO: [%v]",
				err,
			)
		}
		totalInputsValue += walletMainUtxo.Value
	}

	dissolutionValue := totalInputsValue - fee
	if dissolutionValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the total inputs value",
		)
	}

	dissolutionScript, err := bitcoin.PayToWitnessPublicKeyHash(
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot compute dissolution script: [%v]", err)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           dissolutionValue,
		PublicKeyScript: dissolutionScript,
	})

	return builder, nil
}
