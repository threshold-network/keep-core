package tbtc

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

const (
	// reservationAnchorProposalValidityBlocks determines the reservation anchor
	// proposal validity time expressed in blocks. In other words, this is the
	// worst-case time for a reservation anchor during which the wallet is busy
	// and cannot take another action. The value of 600 blocks is roughly 2
	// hours, assuming 12 seconds per block.
	reservationAnchorProposalValidityBlocks = 600
	// reservedRedemptionProposalValidityBlocks determines the reserved
	// redemption proposal validity time expressed in blocks. In other words,
	// this is the worst-case time for a reserved redemption during which the
	// wallet is busy and cannot take another action. The value of 600 blocks
	// is roughly 2 hours, assuming 12 seconds per block.
	reservedRedemptionProposalValidityBlocks = 600
	// reservationReanchorProposalValidityBlocks determines the reservation
	// re-anchor proposal validity time expressed in blocks. In other words,
	// this is the worst-case time for a reservation re-anchor during which the
	// wallet is busy and cannot take another action. The value of 600 blocks
	// is roughly 2 hours, assuming 12 seconds per block.
	reservationReanchorProposalValidityBlocks = 600
	// reservationDissolutionProposalValidityBlocks determines the reservation
	// dissolution proposal validity time expressed in blocks. In other words,
	// this is the worst-case time for a reservation dissolution during which
	// the wallet is busy and cannot take another action. The value of 600
	// blocks is roughly 2 hours, assuming 12 seconds per block.
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
// redeemable in-kind by the reservation owner. Dissolution is the sole
// terminal exception: it returns the anchor value to the wallet's pooled
// main UTXO, ending the reservation.
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
	ReservationActionTypeNone        ReservationActionType = iota // No action in flight.
	ReservationActionTypeAcceptance                               // The anchor/acceptance action.
	ReservationActionTypeRedemption                               // A reserved redemption.
	ReservationActionTypeReanchor                                 // A wallet-migration re-anchor.
	ReservationActionTypeDissolution                              // The terminal dissolution.
)

// ReservationActionState represents the settlement state of a reservation
// action generation (models the anticipated two-phase authorize-then-prove
// redesign tracked in tbtc-v2#1088's own review findings, not the
// currently-reviewed single-phase contract; the enum values may change
// before Ethereum bindings are implemented).
type ReservationActionState uint8

const (
	ReservationActionStateUnknown    ReservationActionState = iota // No action generation exists yet.
	ReservationActionStatePending                                  // Awaiting settlement.
	ReservationActionStateSettled                                  // Confirmed on-chain.
	ReservationActionStateTimedOut                                 // Expired without settlement.
	ReservationActionStateVetoed                                   // Rejected.
	ReservationActionStateSuperseded                               // Replaced by a later generation.
)

// ReservationAction represents one nonce-bound generation of a reservation
// action. All authorization data used to construct and settle the action is
// snapshotted when the generation is requested.
//
// The nonce-keyed lookup (see chain.go's GetReservationAction) and the
// terminal ReservationActionState values below model the anticipated
// two-phase authorize-then-prove redesign tracked in tbtc-v2#1088's own
// review findings, not the currently-reviewed single-phase contract; this
// interface may change before Ethereum bindings are implemented.
type ReservationAction struct {
	// TargetWalletPublicKeyHash is the wallet an acceptance, re-anchor, or
	// dissolution output must pay to. It is zero for redemptions.
	TargetWalletPublicKeyHash [20]byte
	// RequestedAt is the UNIX timestamp the action was requested at.
	RequestedAt uint32
	// TimeoutAt is the UNIX timestamp at or after which the action times out.
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
	// Amount is the satoshi amount associated with the action generation - it
	// tracks the current anchor value being acted on, not Reservation.MintedAmount's
	// gross minted claim.
	Amount uint64
	// RedeemerOutputScriptHash is the keccak256 hash of the length-prefixed
	// output script authorized for a redemption.
	RedeemerOutputScriptHash [32]byte
	// ExpectedMainUtxoHash identifies the wallet main UTXO snapshotted for a
	// dissolution. It is zero for other action types and no-main-UTXO wallets.
	ExpectedMainUtxoHash [32]byte
}

// ReservationAnchorProposal represents a reservation anchor proposal issued
// by a wallet's coordination leader.
type ReservationAnchorProposal struct {
	// DepositFundingTxHash is the funding transaction hash of the reserved
	// deposit to anchor.
	DepositFundingTxHash bitcoin.Hash `json:"depositFundingTxHash"`
	// DepositFundingOutputIndex is the funding output index of the reserved
	// deposit to anchor.
	DepositFundingOutputIndex uint32 `json:"depositFundingOutputIndex"`
	// RequestNonce is the anchor authorization generation being executed.
	RequestNonce uint64 `json:"requestNonce"`
	// AnchorTxFee is the proposed BTC fee for the anchor transaction.
	AnchorTxFee *big.Int `json:"anchorTxFee"`
}

func (rap *ReservationAnchorProposal) ActionType() WalletActionType {
	return ActionReservationAnchor
}

func (rap *ReservationAnchorProposal) ValidityBlocks() uint64 {
	return reservationAnchorProposalValidityBlocks
}

// ReservedRedemptionProposal represents a reserved redemption proposal
// issued by a wallet's coordination leader.
type ReservedRedemptionProposal struct {
	// ReservationKey is the key of the reservation with the pending reserved
	// redemption.
	ReservationKey *big.Int `json:"reservationKey"`
	// RequestNonce is the redemption request generation being executed.
	RequestNonce uint64 `json:"requestNonce"`
	// RedeemerOutputScript is the Bitcoin output script the redemption pays to.
	RedeemerOutputScript bitcoin.Script `json:"redeemerOutputScript"`
	// RedemptionTxFee is the proposed BTC fee for the reserved redemption
	// transaction.
	RedemptionTxFee *big.Int `json:"redemptionTxFee"`
}

func (rrp *ReservedRedemptionProposal) ActionType() WalletActionType {
	return ActionReservedRedemption
}

func (rrp *ReservedRedemptionProposal) ValidityBlocks() uint64 {
	return reservedRedemptionProposalValidityBlocks
}

// ReservationReanchorProposal represents a reservation re-anchor proposal
// issued by a wallet's coordination leader, moving a reservation's anchor
// outpoint to another wallet (e.g. during wallet migration).
type ReservationReanchorProposal struct {
	// ReservationKey is the key of the reservation to re-anchor.
	ReservationKey *big.Int `json:"reservationKey"`
	// RequestNonce is the re-anchor authorization generation being executed.
	RequestNonce uint64 `json:"requestNonce"`
	// TargetWalletPublicKeyHash is the 20-byte public key hash of the wallet
	// receiving the anchor.
	TargetWalletPublicKeyHash [20]byte `json:"targetWalletPublicKeyHash"`
	// ReanchorTxFee is the proposed BTC fee for the re-anchor transaction.
	ReanchorTxFee *big.Int `json:"reanchorTxFee"`
}

func (rrp *ReservationReanchorProposal) ActionType() WalletActionType {
	return ActionReservationReanchor
}

func (rrp *ReservationReanchorProposal) ValidityBlocks() uint64 {
	return reservationReanchorProposalValidityBlocks
}

// ReservationDissolutionProposal represents a reservation dissolution
// proposal issued by a wallet's coordination leader once the reservation's
// custody term and grace period elapsed.
type ReservationDissolutionProposal struct {
	// ReservationKey is the key of the reservation to dissolve.
	ReservationKey *big.Int `json:"reservationKey"`
	// RequestNonce is the dissolution authorization generation being executed.
	RequestNonce uint64 `json:"requestNonce"`
	// DissolutionTxFee is the proposed BTC fee for the dissolution
	// transaction.
	DissolutionTxFee *big.Int `json:"dissolutionTxFee"`
}

func (rdp *ReservationDissolutionProposal) ActionType() WalletActionType {
	return ActionReservationDissolution
}

func (rdp *ReservationDissolutionProposal) ValidityBlocks() uint64 {
	return reservationDissolutionProposalValidityBlocks
}

func requireReservationAction(
	action *ReservationAction,
	expectedType ReservationActionType,
	now uint32,
	label string,
) error {
	if action == nil {
		return fmt.Errorf("reservation action is required")
	}
	if action.ActionType != expectedType {
		return fmt.Errorf("reservation action is not %s", label)
	}
	if action.State != ReservationActionStatePending {
		return fmt.Errorf("reservation action has already been settled")
	}
	if action.TimeoutAt == 0 {
		return fmt.Errorf("reservation action timeout is required")
	}
	if now >= action.TimeoutAt {
		return fmt.Errorf("reservation action has timed out")
	}
	return nil
}

func requireValidActionFee(fee int64, maxFee uint64) error {
	if fee <= 0 {
		return fmt.Errorf("transaction fee must be positive")
	}
	if uint64(fee) > maxFee {
		return fmt.Errorf("transaction fee exceeds the action fee limit")
	}
	return nil
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
	walletPublicKey *ecdsa.PublicKey,
	action *ReservationAction,
	reservationMinAmount uint64,
	fee int64,
	now uint32,
) (*bitcoin.TransactionBuilder, error) {
	if walletPublicKey == nil {
		return nil, fmt.Errorf("wallet public key is required")
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	if err := requireReservationAction(action, ReservationActionTypeAcceptance, now, "an acceptance"); err != nil {
		return nil, err
	}
	if action.TargetWalletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("target wallet public key hash is required")
	}
	if action.TargetWalletPublicKeyHash != walletPublicKeyHash {
		return nil, fmt.Errorf("acceptance action targets a different wallet")
	}
	if err := requireValidActionFee(fee, action.TxMaxFee); err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("transaction fee exceeds the deposit value")
	}
	if anchorValue < int64(reservationMinAmount) {
		return nil, fmt.Errorf("anchor value is below the reservation minimum amount")
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
// redemption transaction for the given nonce-bound action: a 1-input-1-output
// spend of the anchor UTXO to the redeemer output script.
func assembleReservedRedemptionTransaction(
	bitcoinChain bitcoin.Chain,
	bridgeChain interface {
		ComputeReservationRedeemerOutputScriptHash(redeemerOutputScript bitcoin.Script) ([32]byte, error)
	},
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	redeemerOutputScript bitcoin.Script,
	action *ReservationAction,
	fee int64,
	now uint32,
	expectedAnchorOutpoint *bitcoin.TransactionOutpoint,
) (*bitcoin.TransactionBuilder, error) {
	if bridgeChain == nil {
		return nil, fmt.Errorf("bridge chain is required")
	}
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}
	if expectedAnchorOutpoint == nil {
		return nil, fmt.Errorf("expected anchor outpoint is required")
	}
	if anchorUtxo.Outpoint == nil {
		return nil, fmt.Errorf("anchor UTXO outpoint is required")
	}
	if *anchorUtxo.Outpoint != *expectedAnchorOutpoint {
		return nil, fmt.Errorf("anchor UTXO outpoint does not match the action snapshot")
	}
	if len(redeemerOutputScript) == 0 {
		return nil, fmt.Errorf("redeemer output script is required")
	}
	if err := requireReservationAction(action, ReservationActionTypeRedemption, now, "a redemption"); err != nil {
		return nil, err
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
	if err := requireValidActionFee(fee, action.TxMaxFee); err != nil {
		return nil, err
	}

	redeemerOutputScriptHash, err := bridgeChain.ComputeReservationRedeemerOutputScriptHash(redeemerOutputScript)
	if err != nil {
		return nil, fmt.Errorf("cannot compute redeemer output script hash: [%v]", err)
	}

	if redeemerOutputScriptHash != action.RedeemerOutputScriptHash {
		return nil, fmt.Errorf("redeemer output script is not authorized")
	}

	if action.Amount != uint64(anchorUtxo.Value) {
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

	redemptionValue := anchorUtxo.Value - fee
	if redemptionValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the redemption amount",
		)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           redemptionValue,
		PublicKeyScript: redeemerOutputScript,
	})

	return builder, nil
}

// assembleReservationReanchorTransaction constructs an unsigned reservation
// re-anchor transaction: a 1-input-1-output spend of the reservation's
// anchor outpoint into a fresh output controlled by the target wallet. Used
// during wallet migration so reservations never pin retiring wallets.
func assembleReservationReanchorTransaction(
	bitcoinChain bitcoin.Chain,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	targetWalletPublicKeyHash [20]byte,
	action *ReservationAction,
	reservationMinAmount uint64,
	fee int64,
	now uint32,
	expectedAnchorOutpoint *bitcoin.TransactionOutpoint,
) (*bitcoin.TransactionBuilder, error) {
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}
	if expectedAnchorOutpoint == nil {
		return nil, fmt.Errorf("expected anchor outpoint is required")
	}
	if anchorUtxo.Outpoint == nil {
		return nil, fmt.Errorf("anchor UTXO outpoint is required")
	}
	if *anchorUtxo.Outpoint != *expectedAnchorOutpoint {
		return nil, fmt.Errorf("anchor UTXO outpoint does not match the action snapshot")
	}
	if err := requireReservationAction(action, ReservationActionTypeReanchor, now, "a re-anchor"); err != nil {
		return nil, err
	}
	if action.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
		return nil, fmt.Errorf("reanchor action targets a different wallet")
	}
	if action.TargetWalletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("target wallet public key hash is required")
	}
	if err := requireValidActionFee(fee, action.TxMaxFee); err != nil {
		return nil, err
	}

	if anchorUtxo.Value <= 0 {
		return nil, fmt.Errorf("anchor UTXO value must be positive")
	}
	if action.Amount != uint64(anchorUtxo.Value) {
		return nil, fmt.Errorf("reanchor action amount does not match the anchor value")
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
		return nil, fmt.Errorf("transaction fee exceeds the anchor value")
	}
	if reanchorValue < int64(reservationMinAmount) {
		return nil, fmt.Errorf("re-anchor value is below the reservation minimum amount")
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
// reservation dissolution transaction for the given nonce-bound action.
// The anchor outpoint is the first input. The wallet main UTXO is the
// second input only when it is present in the action snapshot and matches
// that snapshot exactly. The single output pays back to the custodying
// wallet.
//
// The Bridge's dissolution proof accepts either input order (see
// Reservation.sol's dissolution-input verification, which matches by
// outpoint hash, not position); this assembler's anchor-first ordering
// is one of the two accepted orders.
func assembleReservationDissolutionTransaction(
	bitcoinChain bitcoin.Chain,
	bridgeChain interface {
		ComputeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte
	},
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	walletPublicKey *ecdsa.PublicKey,
	action *ReservationAction,
	fee int64,
	now uint32,
	expectedAnchorOutpoint *bitcoin.TransactionOutpoint,
) (*bitcoin.TransactionBuilder, error) {
	if walletPublicKey == nil {
		return nil, fmt.Errorf("wallet public key is required")
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	if bridgeChain == nil {
		return nil, fmt.Errorf("bridge chain is required")
	}
	if anchorUtxo == nil {
		return nil, fmt.Errorf("anchor UTXO is required")
	}
	if expectedAnchorOutpoint == nil {
		return nil, fmt.Errorf("expected anchor outpoint is required")
	}
	if anchorUtxo.Outpoint == nil {
		return nil, fmt.Errorf("anchor UTXO outpoint is required")
	}
	if *anchorUtxo.Outpoint != *expectedAnchorOutpoint {
		return nil, fmt.Errorf("anchor UTXO outpoint does not match the action snapshot")
	}
	if err := requireReservationAction(action, ReservationActionTypeDissolution, now, "a dissolution"); err != nil {
		return nil, err
	}
	if action.TargetWalletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("target wallet public key hash is required")
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
	if err := requireValidActionFee(fee, action.TxMaxFee); err != nil {
		return nil, err
	}

	mainUtxoExpected := action.ExpectedMainUtxoHash != [32]byte{}
	if mainUtxoExpected {
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
	} else if walletMainUtxo != nil {
		return nil, fmt.Errorf("wallet main UTXO must not be provided when the dissolution action has no expected main UTXO snapshot")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	// The anchor outpoint is added first (one of the two input orders the
	// Bridge's dissolution proof accepts; see the function doc comment).
	err := builder.AddPublicKeyHashInput(anchorUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to anchor UTXO: [%v]",
			err,
		)
	}

	if mainUtxoExpected {
		err = builder.AddPublicKeyHashInput(walletMainUtxo)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot add input pointing to wallet main UTXO: [%v]",
				err,
			)
		}
	}

	dissolutionValue := builder.TotalInputsValue() - fee
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
