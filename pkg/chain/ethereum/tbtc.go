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
	"github.com/keep-network/keep-common/pkg/cache"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
	"github.com/keep-network/keep-core/pkg/subscription"
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
	// constructed against the Bridge address (see reservationRouterBinding for
	// the address invariant explanation).
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
// that does not exist anywhere in pkg/chain/ethereum today. PR #4280
// explicitly deferred this pending that infra (see 01-gap-analysis.md's
// Minor row); the infra itself is not yet built and has no owning PR. This
// is a package-wide gap, not specific to reservations: the analogous
// ValidateDepositSweepProposal has never had a direct test either for the
// same reason. A follow-up should build the minimal simulated-backend
// infrastructure once, covering every proposal validator in this package,
// rather than deferring it again per future PR.
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
// tested directly (TestBuildReservationAnchorProposalAbi), mirroring the
// reverse-direction converters below (convertReservationFromAbiType et
// al.). Unlike the wrapper above, this builder makes no chain call and
// so needs no simulated-backend infrastructure to test.
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
		RequestNonce:     proposal.RequestNonce,
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
		RequestNonce:           proposal.RequestNonce,
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
//   - CumulativeReanchorFee: maintained and enforced by the reservation contracts; keep-core does not consume it, so it is omitted from tbtc.Reservation.
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

// SubmitReservationAcceptanceProof submits an SPV proof for the given
// reservation acceptance action generation to the Bridge. The proof path
// is onlySpvMaintainer on the router; the call goes through
// Bridge.fallback's delegatecall so the router code reads the Bridge's
// isSpvMaintainer mapping at the Bridge's address.
func (tc *TbtcChain) SubmitReservationAcceptanceProof(
	txInfo *tbtc.BitcoinTxInfo,
	proof *tbtc.BitcoinTxProof,
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

	gasEstimate, err := tc.reservationRouter.SubmitReservationAcceptanceProofGasEstimate(
		abiTxInfo,
		abiProof,
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low; the
	// reservation proof path dispatches into ReservationProofs.submit*Proof,
	// which performs a non-trivial amount of storage I/O. Apply a 20%
	// margin, mirroring the existing pattern in
	// SubmitRedemptionProofWithReimbursement (tbtc_redemption.go).
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.SubmitReservationAcceptanceProof(
		abiTxInfo,
		abiProof,
		reservationKey,
		requestNonce,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// SubmitReservationReanchorProof submits an SPV proof for the given
// reservation re-anchor action generation to the Bridge. The proof path
// is onlySpvMaintainer on the router; the call goes through
// Bridge.fallback's delegatecall so the router code reads the Bridge's
// isSpvMaintainer mapping at the Bridge's address.
func (tc *TbtcChain) SubmitReservationReanchorProof(
	txInfo *tbtc.BitcoinTxInfo,
	proof *tbtc.BitcoinTxProof,
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

	gasEstimate, err := tc.reservationRouter.SubmitReservationReanchorProofGasEstimate(
		abiTxInfo,
		abiProof,
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return err
	}

	// See the margin rationale on SubmitReservationAcceptanceProof above.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.SubmitReservationReanchorProof(
		abiTxInfo,
		abiProof,
		reservationKey,
		requestNonce,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyReservationActionTimeout notifies the Bridge that the timeout for
// the given Reanchor-type reservation action generation has elapsed.
func (tc *TbtcChain) NotifyReservationActionTimeout(
	reservationKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationActionTimeoutGasEstimate(
		reservationKey,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationActionTimeout(
		reservationKey,
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

// WalletTerminationCause returns the on-chain reason the given wallet was
// most recently terminated. The Bridge's WalletTerminated event itself
// carries no cause field; the cause is instead inferred from which of the
// three pre-termination timeout events was emitted for the wallet -
// MovingFundsTimedOut, MovedFundsSweepTimedOut, or
// FraudChallengeDefeatTimedOut, each of which unconditionally leads to
// termination and never more than one of which fires for a given wallet.
// The search is unbounded (from block 0): each of the three events fires
// at most once per wallet in the wallet's entire history, so this call is
// cheap and only ever made once per wallet close, immediately before a
// stranding notification - not on any hot or polled path.
func (tc *TbtcChain) WalletTerminationCause(
	walletPublicKeyHash [20]byte,
) (tbtc.WalletTerminationCause, error) {
	filter := [][20]byte{walletPublicKeyHash}

	movingFundsEvents, err := tc.bridge.PastMovingFundsTimedOutEvents(0, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past MovingFundsTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if len(movingFundsEvents) > 0 {
		return tbtc.WalletTerminationCauseMovingFundsTimeout, nil
	}

	movedFundsSweepEvents, err := tc.bridge.PastMovedFundsSweepTimedOutEvents(0, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past MovedFundsSweepTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if len(movedFundsSweepEvents) > 0 {
		return tbtc.WalletTerminationCauseMovedFundsSweepTimeout, nil
	}

	fraudChallengeEvents, err := tc.bridge.PastFraudChallengeDefeatTimedOutEvents(0, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past FraudChallengeDefeatTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if len(fraudChallengeEvents) > 0 {
		return tbtc.WalletTerminationCauseFraudChallengeDefeat, nil
	}

	return tbtc.WalletTerminationCauseUnknown, nil
}

// NotifyReservationAcceptanceTimedOut notifies the Bridge that the
// acceptance-type action timeout has elapsed for the given reservation.
// Connect must be called before this method is used.
func (tc *TbtcChain) NotifyReservationAcceptanceTimedOut(
	reservationKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationAcceptanceTimedOutGasEstimate(
		reservationKey,
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationAcceptanceTimedOut(
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
//
// The real ReservationRouter has no walletReservations(bytes20) view -
// only walletReservationsAmount/walletReservationsCount, which return
// aggregates, not the key set. The key set is instead derived from the
// PastReservationAcceptanceRequestedEvents (initial custody) and
// PastReservationReanchorRequestedEvents (custody transferred TO this
// wallet) event logs, deduplicated, and filtered down to reservations
// this wallet CURRENTLY custodies via GetReservation - a reservation may
// have since re-anchored away to a different wallet.
//
// WalletReservationsCount is checked first as a cheap short-circuit: the
// vast majority of wallets never custody a reservation, and skipping the
// full-history event scan for them avoids an unbounded eth_getLogs query
// (genesis to tip) on every wallet-close notification for the common
// case. Wallets that do have reservations still pay the full-range scan
// - governance-capped reservation volume keeps this rare and bounded in
// absolute terms, and correctness (never missing a real reservation)
// takes priority over narrowing the block range here.
func (tc *TbtcChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	count, err := tc.reservationRouter.WalletReservationsCount(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get wallet reservations count for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if count == 0 {
		return nil, nil
	}

	// Bound the two event queries to the wallet's own registration block
	// (earliest if it registered multiple times - recovery, re-activation):
	// a wallet cannot have reservation events before it existed. Falls
	// back to a full-history scan only when the registration lookup
	// itself fails, so a transient RPC error degrades to the previous
	// behavior rather than skipping real reservations.
	startBlock, err := tc.earliestWalletRegistrationBlock(walletPublicKeyHash)
	if err != nil {
		startBlock = 0
	}

	acceptanceEvents, err := tc.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{
			StartBlock:          startBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get past reservation acceptance events for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	reanchorEvents, err := tc.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			StartBlock:                startBlock,
			TargetWalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get past reservation reanchor events for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return resolveCustodiedReservationKeys(
		walletPublicKeyHash,
		acceptanceEvents,
		reanchorEvents,
		tc.GetReservation,
	)
}

// earliestWalletRegistrationBlock returns the earliest block at which the
// given wallet was registered. Returns 0 if no registration event is
// found (caller falls back to full-history scan). A wallet can register
// multiple times across its lifetime, so the earliest registration is
// the safe lower bound for any reservation event for that wallet.
func (tc *TbtcChain) earliestWalletRegistrationBlock(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	registrationEvents, err := tc.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get past NewWalletRegistered events for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if len(registrationEvents) == 0 {
		return 0, nil
	}
	var earliest uint64 = math.MaxUint64
	for _, event := range registrationEvents {
		if event != nil && event.BlockNumber > 0 && event.BlockNumber < earliest {
			earliest = event.BlockNumber
		}
	}
	if earliest == math.MaxUint64 {
		return 0, nil
	}
	return earliest, nil
}

// resolveCustodiedReservationKeys is the pure-logic core of
// TbtcChain.WalletReservations: deduplicate the union of acceptance
// and reanchor-REQUESTED events for the wallet down to one entry per
// reservation key, confirm each candidate still custodies the wallet
// via the lookup callback, and return the surviving keys sorted in
// deterministic order. Extracted as a standalone function so it can be
// unit-tested with fake event slices and a fake reservation lookup -
// mirroring the existing pattern in this file (e.g.
// buildReservationAnchorProposalAbi) - since the surrounding
// TbtcChain methods depend on real go-ethereum simulated-backend
// infrastructure that does not exist anywhere in pkg/chain/ethereum
// today (a package-wide gap, explicitly deferred).
func resolveCustodiedReservationKeys(
	walletPublicKeyHash [20]byte,
	acceptanceEvents []*tbtc.ReservationAcceptanceRequestedEvent,
	reanchorEvents []*tbtc.ReservationReanchorRequestedEvent,
	reservationLookup func(key *big.Int) (*tbtc.Reservation, error),
) ([]*big.Int, error) {
	candidateKeys := make(map[string]*big.Int)
	for _, event := range acceptanceEvents {
		if event == nil || event.ReservationKey == nil {
			continue
		}
		candidateKeys[event.ReservationKey.String()] = event.ReservationKey
	}
	for _, event := range reanchorEvents {
		if event == nil || event.ReservationKey == nil {
			continue
		}
		candidateKeys[event.ReservationKey.String()] = event.ReservationKey
	}

	// Sort the candidate keys BEFORE the per-key lookup loop so the
	// order in which GetReservation is called is deterministic. This
	// matters for the partial-failure case: if the loop returns on
	// the first lookup error, that error must reproduce against the
	// same input rather than depending on Go map iteration order.
	keys := make([]*big.Int, 0, len(candidateKeys))
	for _, key := range candidateKeys {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].Cmp(keys[j]) < 0
	})

	filteredKeys := make([]*big.Int, 0, len(keys))
	for _, key := range keys {
		reservation, err := reservationLookup(key)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get reservation [%v]: [%v]",
				key,
				err,
			)
		}

		if reservation.WalletPublicKeyHash != walletPublicKeyHash {
			// The reservation was once tied to this wallet (accepted here,
			// or re-anchored here) but has since re-anchored away; it is no
			// longer custodied by this wallet.
			continue
		}

		filteredKeys = append(filteredKeys, key)
	}

	return filteredKeys, nil
}

// ReservationByAnchorUtxo returns the reservation key whose anchor outpoint
// is the given Bitcoin transaction output, or an empty value if no
// reservation is anchored there.
func (tc *TbtcChain) ReservationByAnchorUtxo(
	anchorTxHash [32]byte,
	anchorTxOutputIndex uint32,
) (*big.Int, error) {
	key, err := tc.reservationRouter.ReservationByAnchorUtxo(
		anchorTxHash,
		anchorTxOutputIndex,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation by anchor utxo [0x%x:%d]: [%v]",
			anchorTxHash,
			anchorTxOutputIndex,
			err,
		)
	}

	return key, nil
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
// Connect must be called before this method is used.
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
// Connect must be called before this method is used.
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
