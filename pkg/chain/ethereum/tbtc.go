// tbtc.go: TbtcChain adapter construction and shared state. See tbtc_*.go for
// per-concern implementations (tbtc_deposit.go, tbtc_dkg.go, tbtc_moving_funds.go,
// tbtc_redemption.go, tbtc_wallet.go, tbtc_sortition.go, tbtc_inactivity.go).
//
// These files were split out of a single monolithic tbtc.go with no rename
// markers git can detect (each file is a fresh addition, not a tracked move),
// so a plain `git revert` of the split commit cannot be applied cleanly on
// top of any later commit that also touches this package: it would re-delete
// the per-concern files and reintroduce the old tbtc.go, silently dropping
// whatever those later commits changed. Reconstructing the pre-split state
// requires a manual merge, not a mechanical revert.
package ethereum

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Definitions of contract names.
const (
	// TODO: The WalletRegistry address is taken from the Bridge contract.
	//       Remove the possibility of passing it through the config.
	WalletRegistryContractName          = "WalletRegistry"
	BridgeContractName                  = "Bridge"
	MaintainerProxyContractName         = "MaintainerProxy"
	WalletProposalValidatorContractName = "WalletProposalValidator"
	// EcdsaDkgValidatorContractName is optional: when set under
	// ethereum contract addresses or developer.ecdsaDkgValidatorAddress
	// alias in config, TBTC ECDSA sizing is read via eth_call instead of only
	// network defaults from defaultGroupParameters.
	EcdsaDkgValidatorContractName = "EcdsaDkgValidator"
)

const (
	sweptDepositsCachePeriod = 7 * 24 * time.Hour
)

// TbtcChain represents a TBTC-specific chain handle.
type TbtcChain struct {
	*baseChain

	bridge                  *tbtccontract.Bridge
	maintainerProxy         *tbtccontract.MaintainerProxy
	walletRegistry          *ecdsacontract.WalletRegistry
	sortitionPool           *ecdsacontract.EcdsaSortitionPool
	walletProposalValidator *tbtccontract.WalletProposalValidator
	redemptionWatchtower    *tbtccontract.RedemptionWatchtower
	// reservationRouter is the abigen binding for ReservationRouter.sol's ABI
	// (functions, events, errors). It is NOT bound to the deployed router
	// address -- the router contract holds its own empty storage and only
	// ever executes via Bridge.fallback's delegatecall. The binding is
	// constructed against the Bridge address so every read, write, and log
	// filter goes through Bridge.fallback, which dispatches to the router
	// code with the Bridge's storage and emits events under the Bridge's
	// address. Calling the binding against the router's standalone address
	// would invoke its empty storage and either revert (writes) or return
	// zeros (views).
	reservationRouter *tbtccontract.ReservationRouter
	// ecdsaDkgValidatorAddress optional; when zero, TBTC uses defaultGroupParameters(network).
	ecdsaDkgValidatorAddress common.Address

	sweptDepositsCache *cache.GenericTimeCache[*tbtc.DepositChainRequest]
}

// NewTbtcChain construct a new instance of the TBTC-specific Ethereum
// chain handle.
func newTbtcChain(
	config ethereum.Config,
	baseChain *baseChain,
) (*TbtcChain, error) {
	bridgeAddress, err := config.ContractAddress(BridgeContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			BridgeContractName,
			err,
		)
	}

	bridge, err :=
		tbtccontract.NewBridge(
			bridgeAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to Bridge contract: [%v]",
			err,
		)
	}

	maintainerProxyAddress, err := config.ContractAddress(MaintainerProxyContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			MaintainerProxyContractName,
			err,
		)
	}

	maintainerProxy, err :=
		tbtccontract.NewMaintainerProxy(
			maintainerProxyAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to MaintainerProxy contract: [%v]",
			err,
		)
	}

	references, err := bridge.ContractReferences()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get contract references from Bridge: [%v]",
			err,
		)
	}

	walletRegistryAddress := references.EcdsaWalletRegistry

	walletRegistry, err :=
		ecdsacontract.NewWalletRegistry(
			walletRegistryAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to WalletRegistry contract: [%v]",
			err,
		)
	}

	sortitionPoolAddress, err := walletRegistry.SortitionPool()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get sortition pool address: [%v]",
			err,
		)
	}

	sortitionPool, err :=
		ecdsacontract.NewEcdsaSortitionPool(
			sortitionPoolAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to EcdsaSortitionPool contract: [%v]",
			err,
		)
	}

	walletProposalValidatorAddress, err := config.ContractAddress(
		WalletProposalValidatorContractName,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			WalletProposalValidatorContractName,
			err,
		)
	}

	walletProposalValidator, err :=
		tbtccontract.NewWalletProposalValidator(
			walletProposalValidatorAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to WalletProposalValidator contract: [%v]",
			err,
		)
	}

	redemptionWatchtowerAddress, err := bridge.GetRedemptionWatchtower()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get RedemptionWatchtower address from Bridge: [%v]",
			err,
		)
	}

	// The RedemptionWatchtower contract is an additional component
	// that implements the redemption veto mechanism. It is optional
	// and may not be present in the system. This code must be able
	// to handle the case when the RedemptionWatchtower contract is
	// not set.
	var redemptionWatchtower *tbtccontract.RedemptionWatchtower
	if redemptionWatchtowerAddress != [20]byte{} {
		redemptionWatchtower, err =
			tbtccontract.NewRedemptionWatchtower(
				redemptionWatchtowerAddress,
				baseChain.chainID,
				baseChain.key,
				baseChain.client,
				baseChain.nonceManager,
				baseChain.miningWaiter,
				baseChain.blockCounter,
				baseChain.transactionMutex,
			)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to attach to RedemptionWatchtower contract: [%v]",
				err,
			)
		}
	}

	var ecdsaDkgValidatorAddress common.Address
	validatorAddr, err := config.ContractAddress(EcdsaDkgValidatorContractName)
	switch {
	case err == nil:
		ecdsaDkgValidatorAddress = validatorAddr
	case errors.Is(err, ethereum.ErrAddressNotConfigured):
		logger.Warnf(
			"%s contract address is not configured; TBTC group parameters "+
				"will fall back to network defaults instead of on-chain values",
			EcdsaDkgValidatorContractName,
		)
	default:
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%w]",
			EcdsaDkgValidatorContractName,
			err,
		)
	}

	reservationRouter, err := reservationRouterBinding(bridgeAddress, baseChain)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to ReservationRouter binding: [%v]",
			err,
		)
	}

	return &TbtcChain{
		baseChain:                baseChain,
		bridge:                   bridge,
		maintainerProxy:          maintainerProxy,
		walletRegistry:           walletRegistry,
		sortitionPool:            sortitionPool,
		walletProposalValidator:  walletProposalValidator,
		redemptionWatchtower:     redemptionWatchtower,
		reservationRouter:        reservationRouter,
		ecdsaDkgValidatorAddress: ecdsaDkgValidatorAddress,
		sweptDepositsCache:       cache.NewGenericTimeCache[*tbtc.DepositChainRequest](sweptDepositsCachePeriod),
	}, nil
}

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

// convertPubKeyToChainFormat takes X and Y coordinates of a signer's public key
// and concatenates it to a 64-byte long array. If any of coordinates is shorter
// than 32-byte it is preceded with zeros.
func convertPubKeyToChainFormat(publicKey *ecdsa.PublicKey) ([64]byte, error) {
	var serialized [64]byte

	x, err := byteutils.LeftPadTo32Bytes(publicKey.X.Bytes())
	if err != nil {
		return serialized, err
	}

	y, err := byteutils.LeftPadTo32Bytes(publicKey.Y.Bytes())
	if err != nil {
		return serialized, err
	}

	serializedBytes := append(x, y...)

	copy(serialized[:], serializedBytes)

	return serialized, nil
}

// buildTxOutpointKey computes keccak256(txHash || uint32BE(outputIndex)) and
// returns it as a *big.Int. Used by both the deposit and moved-funds request
// lookup paths; the contract-side mapping is identical for both.
func buildTxOutpointKey(txHash bitcoin.Hash, outputIndex uint32) *big.Int {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, outputIndex)

	return crypto.Keccak256Hash(append(txHash[:], indexBytes...)).Big()
}

func (tc *TbtcChain) TxProofDifficultyFactor() (*big.Int, error) {
	return tc.bridge.TxProofDifficultyFactor()
}

// GetReservation returns the on-chain reservation record for the given
// reservation key. The reservation router code is reached via
// Bridge.fallback's delegatecall; the reservationRouter binding is bound to
// the Bridge address so this call routes through the fallback into the
// router code that reads the Bridge's reservation storage.
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
// parameters (10-tuple). The reservationRouter binding routes this read
// through Bridge.fallback into the router's reservationParameters view.
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

// ValidateReservedRedemptionProposal asks the WalletProposalValidator
// whether the given reserved redemption proposal is valid for the given
// wallet. The m1 bridge-integration surface does not expose a
// `validateReservedRedemptionProposal` entry on the WalletProposalValidator
// (only anchor and re-anchor validators are present at this milestone), so
// the interface stub returns an explicit error rather than calling a
// non-existent binding. Downstream tasks replacing this body will receive
// the bridge-integration Solidity once that validator lands.
func (tc *TbtcChain) ValidateReservedRedemptionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservedRedemptionProposal,
) error {
	return fmt.Errorf(
		"reserved redemption proposal validator is not exposed on " +
			"the m1 bridge-integration surface",
	)
}

// ValidateReservationReanchorProposal asks the WalletProposalValidator
// whether the given re-anchor proposal is valid for the given source
// wallet. The validator is a separate contract reached at its own deployed
// address.
func (tc *TbtcChain) ValidateReservationReanchorProposal(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorReservationReanchorProposal{
		SourceWalletPubKeyHash: sourceWalletPublicKeyHash,
		ReservationKey:         proposal.ReservationKey,
		TargetWalletPubKeyHash: proposal.TargetWalletPublicKeyHash,
		ReanchorTxFee:          proposal.ReanchorTxFee,
	}

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

// ValidateReservationDissolutionProposal asks the WalletProposalValidator
// whether the given dissolution proposal is valid for the given wallet.
// The m1 bridge-integration surface does not expose a
// `validateReservationDissolutionProposal` entry on the
// WalletProposalValidator (only anchor and re-anchor validators are present
// at this milestone), so the interface stub returns an explicit error
// rather than calling a non-existent binding.
func (tc *TbtcChain) ValidateReservationDissolutionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationDissolutionProposal,
) error {
	return fmt.Errorf(
		"reservation dissolution proposal validator is not exposed on " +
			"the m1 bridge-integration surface",
	)
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
