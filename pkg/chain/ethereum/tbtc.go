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
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/cache"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
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

	// reservationRegistrationLookBackBlocks sizes the initial window of
	// the NewWalletRegistered event lookup performed by
	// earliestWalletRegistrationBlock: the last ~30 days (216000 blocks
	// at Ethereum's ~12s block time). That method walks the window
	// backwards, doubling it each time nothing is found, until it finds
	// the wallet's actual registration block or reaches genesis, then
	// memoizes the result - so this constant only controls how many
	// round trips the common case (a wallet closed shortly after
	// registering) needs, not the correctness of the final bound. This
	// mirrors the identical 30-day convention already used elsewhere in
	// the reservation feature for startup/catch-up scan bounds (see
	// pkg/maintainer/spv/reservation_wiring.go's
	// reservationDefaultLookBackBlocks) - it is duplicated here, with
	// its own doc comment, rather than imported, because
	// pkg/maintainer/spv already imports pkg/chain/ethereum and the
	// reverse import would create a cycle.
	reservationRegistrationLookBackBlocks = uint64(216000)

	// reservationRegistrationReorgSafetyBlocks is subtracted on top of
	// reservationRegistrationLookBackBlocks when bounding the
	// registration-event lookup, so a shallow reorg near the chain tip
	// cannot shift a wallet's actual registration block just past the
	// scan window's lower edge and silently drop it from the bound.
	reservationRegistrationReorgSafetyBlocks = uint64(12)
)

// TbtcChain represents a TBTC-specific chain handle.
type TbtcChain struct {
	*baseChain

	bridge                  *tbtccontract.Bridge
	maintainerProxy         *tbtccontract.MaintainerProxy
	walletRegistry          *ecdsacontract.WalletRegistry
	admission               tbtcAdmissionReader
	sortitionPool           *ecdsacontract.EcdsaSortitionPool
	walletProposalValidator *tbtccontract.WalletProposalValidator
	redemptionWatchtower    *tbtccontract.RedemptionWatchtower
	// reservationRouter is the abigen binding for ReservationRouter.sol's ABI
	// constructed against the Bridge address (see reservationRouterBinding for
	// the address invariant explanation).
	reservationRouter *tbtccontract.ReservationRouter
	// walletRegistrationBlockCache memoizes the result of
	// earliestWalletRegistrationBlock per wallet public key hash for the
	// lifetime of this chain adapter. NewWalletRegistered fires once per
	// wallet registration and a past registration is immutable, so a
	// resolved block never goes stale and repeat lookups for the same
	// wallet cost nothing beyond the first.
	walletRegistrationBlockCache sync.Map
	// vaultBindings caches the runtime-constructed ReservationVault and
	// TBTC token bindings keyed by vault address. A vault configured
	// through Bridge.reservationParameters is a runtime address that is
	// not in the static contract address config, so its binding is
	// built on first read and then memoized for the lifetime of this
	// chain adapter, the way walletRegistrationBlockCache memoizes
	// resolved registration blocks.
	vaultBindings sync.Map
	// ecdsaDkgValidatorAddress is the optional address of an ECDSA DKG
	// validator; when zero, TBTC uses defaultGroupParameters(network).
	ecdsaDkgValidatorAddress common.Address

	sweptDepositsCache *cache.GenericTimeCache[*tbtc.DepositChainRequest]
}

// tbtcAdmissionReader narrows the WalletRegistry down to the two reads the
// admission predicate decides on. *ecdsacontract.WalletRegistry satisfies it as
// it stands; the indirection exists so the predicate can be exercised without a
// chain behind it.
//
// EligibleStake is not admission's alone, though: TbtcChain.EligibleStake is
// routed through this same seam, and the heartbeat path reads a staking
// provider's stake through that accessor to decide whether the operator is
// unstaking. A reader substituted here is therefore answering both.
type tbtcAdmissionReader interface {
	OperatorToStakingProvider(operator common.Address) (common.Address, error)
	EligibleStake(stakingProvider common.Address) (*big.Int, error)
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
		admission:                walletRegistry,
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

// ReservationsVaultTrusted reports whether the Bridge currently trusts the
// reservation vault configured in the reservation parameters, which is the
// on-chain half of the reservations activation state: setVaultStatus(vault,
// true) makes the vault trusted and therefore makes reservations live.
//
// The C-4 activation ordering gate requires the on-chain activation to run
// at or after the client's activation block, so the client compares this
// value with its own compiled activation block at startup (see
// tbtc.EvaluateReservationsActivation).
//
// When no reservation vault is configured the vault is the zero address, the
// Bridge cannot trust it and reservations are disabled on-chain, so this
// method returns false with a nil error.
func (tc *TbtcChain) ReservationsVaultTrusted() (bool, error) {
	parameters, err := tc.ReservationParameters()
	if err != nil {
		return false, fmt.Errorf(
			"cannot get reservation parameters: [%w]",
			err,
		)
	}

	if parameters.ReservationVault == "" {
		// No vault configured: reservations are disabled on-chain.
		return false, nil
	}

	vaultAddress, err := decodeReservationVaultAddress(parameters.ReservationVault)
	if err != nil {
		return false, fmt.Errorf(
			"cannot decode reservation vault address: [%w]",
			err,
		)
	}
	if vaultAddress == (common.Address{}) {
		// No vault configured: reservations are disabled on-chain.
		return false, nil
	}

	trusted, err := tc.bridge.IsVaultTrusted(vaultAddress)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check whether the reservation vault is trusted: [%w]",
			err,
		)
	}

	return trusted, nil
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

	if abiReservation.AnchorAmount > math.MaxInt64 {
		return nil, fmt.Errorf(
			"reservation anchor amount [%v] does not fit in int64",
			abiReservation.AnchorAmount,
		)
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
		ReanchorCooldownUntil: abiReservation.ReanchorCooldownUntil,
	}, nil
}

// convertReservationActionFromAbiType converts the ReservationRouter-
// specific Reservation.ReservationAction ABI struct to the TBTC
// application `tbtc.ReservationAction` representation.
//
// Field omissions (intentional):
//
//   - `UsedRetryCredit`, `Watchtower{Default,LevelOne,LevelTwo}Delay`,
//     `RetryCreditSourceNonce`: written for governance / late-settlement
//     reconciliation but not read by the operator client in m1.
//   - `DissolutionDelay`: snapshotted by acceptance generations for the
//     dissolution path, which the operator client does not drive in m1.
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
		SourceAnchorUtxoHash:      abiAction.SourceAnchorUtxoHash,
		Redeemer:                  chain.Address(abiAction.Redeemer.String()),
		Amount:                    abiAction.Amount,
		RedeemerOutputScriptHash:  redeemerOutputScriptHash,
		ExpectedMainUtxoHash:      expectedMainUtxoHash,
		IsPartial:                 abiAction.IsPartial,
		TermSeconds:               abiAction.TermSeconds,
		MinAmount:                 abiAction.MinAmount,
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

// RequestReservationReanchor asks the Bridge (via its ReservationRouter
// delegatecall target) to start a new reservation re-anchor action generation
// for the given reservation, targeting the given wallet.
//
// walletMembersIDs and walletMemberIndex carry the caller's source-wallet
// member proof: the source wallet signing group's member IDs and this
// operator's 1-based position within them. A non-empty proof that validates
// grants the source-wallet-member exemption from the target's third-party
// re-anchor hold cap; an empty proof makes the request a plain third-party
// request that is subject to the cap.
func (tc *TbtcChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
	walletMembersIDs []uint32,
	walletMemberIndex uint32,
) error {
	gasEstimate, err := tc.reservationRouter.RequestReservationReanchorGasEstimate(
		reservationKey,
		targetWalletPublicKeyHash,
		walletMembersIDs,
		big.NewInt(int64(walletMemberIndex)),
	)
	if err != nil {
		return err
	}

	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

	_, err = tc.reservationRouter.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
		walletMembersIDs,
		big.NewInt(int64(walletMemberIndex)),
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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
// termination. If more than one of these events were ever emitted for
// the same wallet (not expected in normal Bridge operation, but not
// verified impossible by this method), this method returns the first
// found, in MovingFundsTimedOut -> MovedFundsSweepTimedOut ->
// FraudChallengeDefeatTimedOut priority order - not necessarily the most
// recently emitted one.
//
// The three event queries are bounded by earliestWalletRegistrationBlock
// (the same registration-block resolution WalletReservations uses,
// memoized per wallet - see that method's doc comment) instead of
// scanning from genesis: a wallet cannot have been terminated before it
// registered. This is not a single cold-path call made once per wallet
// close: pkg/maintainer/spv calls it from the stranding startup
// catch-up scan (up to 3 attempts per Closed/Terminated wallet found
// by that scan), that scan's second-chance retry pass for wallets it
// could not resolve the first time, the live OnWalletClosed
// subscription handler (immediately before a stranding notification,
// as originally documented), and - conditionally - from the 1-minute
// action-timeout poll loop's recheck path whenever a Reanchor-type
// reservation action times out against a wallet that is already
// Closed/Terminated. A failure to resolve the registration bound fails
// the call instead of widening the scan to genesis (StartBlock 0): the
// caller treats the cause as best-effort and the next pass retries.
func (tc *TbtcChain) WalletTerminationCause(
	walletPublicKeyHash [20]byte,
) (tbtc.WalletTerminationCause, error) {
	startBlock, err := registrationScanStartBlock(
		walletPublicKeyHash,
		tc.earliestWalletRegistrationBlock,
	)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, err
	}

	filter := [][20]byte{walletPublicKeyHash}

	movingFundsEvents, err := tc.bridge.PastMovingFundsTimedOutEvents(startBlock, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past MovingFundsTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if cause := resolveWalletTerminationCause(len(movingFundsEvents), 0, 0); cause != tbtc.WalletTerminationCauseUnknown {
		return cause, nil
	}

	movedFundsSweepEvents, err := tc.bridge.PastMovedFundsSweepTimedOutEvents(startBlock, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past MovedFundsSweepTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if cause := resolveWalletTerminationCause(0, len(movedFundsSweepEvents), 0); cause != tbtc.WalletTerminationCauseUnknown {
		return cause, nil
	}

	fraudChallengeEvents, err := tc.bridge.PastFraudChallengeDefeatTimedOutEvents(startBlock, nil, filter)
	if err != nil {
		return tbtc.WalletTerminationCauseUnknown, fmt.Errorf(
			"cannot get past FraudChallengeDefeatTimedOut events for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	return resolveWalletTerminationCause(0, 0, len(fraudChallengeEvents)), nil
}

// resolveWalletTerminationCause is the pure-logic core of
// TbtcChain.WalletTerminationCause: given how many of each
// pre-termination timeout event have been found for a wallet so far, it
// decides which termination cause to report, honoring the
// MovingFundsTimedOut -> MovedFundsSweepTimedOut ->
// FraudChallengeDefeatTimedOut priority order documented on
// WalletTerminationCause. Extracted as a standalone function - mirroring
// the existing pattern in this file (e.g. resolveCustodiedReservationKeys,
// buildReservationAnchorProposalAbi) - so that priority order can be unit
// tested directly, including the case where more than one count is
// non-zero at once, since the surrounding TbtcChain method depends on
// real go-ethereum simulated-backend infrastructure that does not exist
// anywhere in pkg/chain/ethereum today (a package-wide gap, explicitly
// deferred). WalletTerminationCause itself calls this at each of its
// three early-return points with only the count it has fetched so far
// (the other two passed as 0), so in production at most one argument is
// ever non-zero at a time; the multi-non-zero case exercised in tests
// exists to pin the documented priority contract independently of that
// short-circuiting.
func resolveWalletTerminationCause(
	movingFundsEventCount int,
	movedFundsSweepEventCount int,
	fraudChallengeEventCount int,
) tbtc.WalletTerminationCause {
	if movingFundsEventCount > 0 {
		return tbtc.WalletTerminationCauseMovingFundsTimeout
	}
	if movedFundsSweepEventCount > 0 {
		return tbtc.WalletTerminationCauseMovedFundsSweepTimeout
	}
	if fraudChallengeEventCount > 0 {
		return tbtc.WalletTerminationCauseFraudChallengeDefeat
	}
	return tbtc.WalletTerminationCauseUnknown
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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
		if mainUtxo.Value < 0 {
			return fmt.Errorf(
				"invalid main UTXO value: [%v]",
				mainUtxo.Value,
			)
		}

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
	gasEstimateWithMargin := gasEstimateWithMargin(gasEstimate)

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
// full-history event scan for them avoids a scan across the wallet's
// whole lifetime on every wallet-close notification for the common
// case. Wallets that do have reservations pay a reservation-event scan
// bounded by earliestWalletRegistrationBlock, which resolves the
// wallet's actual earliest registration block by walking
// NewWalletRegistered events backwards in doubling windows from the
// chain tip until one is found or genesis is reached, then memoizes
// the result for the lifetime of this chain adapter - see that
// method's doc comment. That bound decides how many blocks the scan
// covers, and scanWalletReservationEventsInChunks decides how the
// covered range is queried: in 10,000-block chunks, because providers
// cap the range of a single eth_getLogs and one unbounded query from
// the registration block to the tip would fail the whole call there.
// Governance-capped reservation volume keeps the reservation-having
// case rare in absolute terms, and correctness (never missing a real
// reservation) takes priority over narrowing the block range further
// here.
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
	// a wallet cannot have reservation events before it existed. A failed
	// registration lookup fails this call instead of widening the scan to
	// genesis: the caller retries on its next pass, whereas a genesis scan
	// is a 10,000-block chunk walk over the whole chain.
	startBlock, err := registrationScanStartBlock(
		walletPublicKeyHash,
		tc.earliestWalletRegistrationBlock,
	)
	if err != nil {
		return nil, err
	}

	currentBlock, err := tc.walletReservationScanEndBlock()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get the chain tip for the wallet [0x%x] reservation "+
				"scan: [%w]",
			walletPublicKeyHash,
			err,
		)
	}

	acceptanceEvents, reanchorEvents, err := scanWalletReservationEventsInChunks(
		walletPublicKeyHash,
		startBlock,
		currentBlock,
		func(chunkStartBlock, chunkEndBlock uint64) (
			[]*tbtc.ReservationAcceptanceRequestedEvent,
			error,
		) {
			return tc.PastReservationAcceptanceRequestedEvents(
				&tbtc.ReservationAcceptanceRequestedEventFilter{
					StartBlock:          chunkStartBlock,
					EndBlock:            &chunkEndBlock,
					WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				},
			)
		},
		func(chunkStartBlock, chunkEndBlock uint64) (
			[]*tbtc.ReservationReanchorRequestedEvent,
			error,
		) {
			return tc.PastReservationReanchorRequestedEvents(
				&tbtc.ReservationReanchorRequestedEventFilter{
					StartBlock:                chunkStartBlock,
					EndBlock:                  &chunkEndBlock,
					TargetWalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				},
			)
		},
	)
	if err != nil {
		return nil, err
	}

	return resolveCustodiedReservationKeys(
		walletPublicKeyHash,
		acceptanceEvents,
		reanchorEvents,
		tc.GetReservation,
	)
}

// walletReservationScanEndBlock returns the block the wallet reservation
// scan ends at.
//
// The height comes from the node over RPC (LatestBlockNumber), not from
// tc.blockCounter: that counter is advanced only by new-head subscription
// notifications (it re-reads a header at startup and when the subscription
// is re-established, and keeps returning its last known height in between),
// while WalletReservationsCount and every other reservation read go to the
// node's current state over ordinary RPC calls. A stalled or disconnected
// head subscription would otherwise bound the scan below the blocks that
// explain a non-zero count, and the scan would return an empty key set with
// no error - which the re-anchor and stranding consumers treat as a wallet
// with nothing left to move. A failing RPC read fails the scan instead, so
// the caller retries on its next pass rather than acting on a truncated
// answer.
func (tc *TbtcChain) walletReservationScanEndBlock() (uint64, error) {
	// The read timeout mirrors the adapter's other header lookups (see
	// baseChain.currentBlock).
	ctx, cancelCtx := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCtx()

	return tc.LatestBlockNumber(ctx)
}

// earliestWalletRegistrationBlock returns the actual earliest block at
// which the given wallet was registered. NewWalletRegistered events are
// walked backwards from the node's current chain tip - see
// resolveEarliestWalletRegistrationBlock for the doubling-window walk
// itself - until a registration event is found or genesis is reached. The
// tip comes from the node over RPC rather than from the subscription-cached
// height, for the reason given on walletReservationScanEndBlock: a lagging
// cache would bound the walk below a registration that has already
// happened.
// NewWalletRegistered fires once per wallet registration and a past
// registration is immutable, so the resolved block is memoized in
// tc.walletRegistrationBlockCache for the lifetime of this chain
// adapter instance: repeat lookups for the same wallet cost nothing
// beyond the first. Returns 0 (unmemoized) if the current block cannot
// be determined, or if no registration event is found anywhere back to
// genesis (caller falls back to a full-history scan of the reservation
// events themselves, which is always correct, just potentially
// slower). A wallet can register multiple times across its lifetime,
// so the earliest registration found by the walk is the safe lower
// bound for any reservation event for that wallet.
func (tc *TbtcChain) earliestWalletRegistrationBlock(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	if cached, ok := tc.walletRegistrationBlockCache.Load(walletPublicKeyHash); ok {
		return cached.(uint64), nil
	}

	currentBlock, err := tc.walletReservationScanEndBlock()
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get the chain tip to resolve registration block for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	registrationBlock, err := resolveEarliestWalletRegistrationBlock(
		currentBlock,
		func(startBlock, endBlock uint64) ([]*tbtc.NewWalletRegisteredEvent, error) {
			return tc.PastNewWalletRegisteredEvents(
				&tbtc.NewWalletRegisteredEventFilter{
					StartBlock:          startBlock,
					EndBlock:            &endBlock,
					WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				},
			)
		},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get past NewWalletRegistered events for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	if registrationBlock != 0 {
		tc.walletRegistrationBlockCache.Store(walletPublicKeyHash, registrationBlock)
	}

	return registrationBlock, nil
}

// registrationScanStartBlock returns the block from which a wallet's event
// scans start. A failed registration lookup is returned as an error and
// never degrades to block 0: the full-history scan it would trigger is a
// 10,000-block chunk walk from genesis whose exit is bounded only by the
// chain's height, so the caller retries on its next pass instead of paying
// a permanently unbounded number of round trips.
func registrationScanStartBlock(
	walletPublicKeyHash [20]byte,
	resolve func(walletPublicKeyHash [20]byte) (uint64, error),
) (uint64, error) {
	startBlock, err := resolve(walletPublicKeyHash)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot resolve registration block of wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return startBlock, nil
}

// resolveEarliestWalletRegistrationBlock is the pure-logic core of
// earliestWalletRegistrationBlock: given the current chain tip and a
// callback that fetches NewWalletRegistered events for one wallet from a
// given block range, it walks the start block backwards - beginning at the
// last reservationRegistrationLookBackBlocks blocks (plus a
// reservationRegistrationReorgSafetyBlocks reorg-safety margin), doubling
// the window and shifting further back each time nothing is found - until a
// registration event turns up or the walk reaches genesis (start block 0).
// Each window is fetched in ascending chunks of at most
// tbtc.DepositRevealLookupChunkBlocks blocks and the walk stops at the first
// chunk that yields a registration: that chunk holds the window's earliest
// registration, because the chunks ascend. A doubled window can span
// millions of blocks on an old wallet, so leaving the range unbounded would
// hand the provider the same oversized eth_getLogs the chunking exists to
// avoid.
//
// Extracted as a standalone function - mirroring the existing pattern in
// this file (e.g. resolveCustodiedReservationKeys,
// resolveWalletTerminationCause) - so the walk/doubling logic can be unit
// tested directly with a fake fetchEvents callback, since the surrounding
// TbtcChain method depends on real go-ethereum simulated-backend
// infrastructure that does not exist anywhere in pkg/chain/ethereum today
// (a package-wide gap, explicitly deferred). Returns 0 if genesis is reached
// without finding a registration event.
func resolveEarliestWalletRegistrationBlock(
	currentBlock uint64,
	fetchEvents func(startBlock, endBlock uint64) ([]*tbtc.NewWalletRegisteredEvent, error),
) (uint64, error) {
	windowSize := reservationRegistrationLookBackBlocks + reservationRegistrationReorgSafetyBlocks

	var startBlock uint64
	if currentBlock > windowSize {
		startBlock = currentBlock - windowSize
	}

	for {
		registrationBlock, err := earliestRegistrationEventBlockInRange(
			startBlock,
			currentBlock,
			fetchEvents,
		)
		if err != nil {
			return 0, err
		}

		if registrationBlock != 0 {
			return registrationBlock, nil
		}

		if startBlock == 0 {
			return 0, nil
		}

		if startBlock > windowSize {
			startBlock -= windowSize
		} else {
			startBlock = 0
		}
		windowSize *= 2
	}
}

// earliestRegistrationEventBlockInRange walks the inclusive block range
// [startBlock, endBlock] in ascending chunks and returns the earliest
// registration event block found, or 0 if no chunk yields one. It stops at
// the first chunk that yields events: chunks ascend, so that chunk holds the
// range's earliest registration and the remaining chunks cannot change the
// answer.
func earliestRegistrationEventBlockInRange(
	startBlock uint64,
	endBlock uint64,
	fetchEvents func(startBlock, endBlock uint64) ([]*tbtc.NewWalletRegisteredEvent, error),
) (uint64, error) {
	var earliest uint64
	err := fetchEventRangeInChunks(
		startBlock,
		endBlock,
		fetchEvents,
		func(chunkEvents []*tbtc.NewWalletRegisteredEvent) bool {
			earliest = earliestRegistrationEventBlock(chunkEvents)
			return earliest == 0
		},
	)
	if err != nil {
		return 0, err
	}
	return earliest, nil
}

// earliestRegistrationEventBlock is the pure-logic core of
// earliestWalletRegistrationBlock: given a set of NewWalletRegistered
// events already scoped to one wallet, it resolves the block number of
// the earliest one, ignoring nil entries and zero-value block numbers
// defensively. Extracted as a standalone function - mirroring the
// existing pattern in this file (e.g. resolveCustodiedReservationKeys) -
// so it can be unit tested directly with fake event slices, since the
// surrounding TbtcChain method depends on real go-ethereum
// simulated-backend infrastructure that does not exist anywhere in
// pkg/chain/ethereum today (a package-wide gap, explicitly deferred).
// Returns 0 if events is empty or every entry's BlockNumber is 0.
func earliestRegistrationEventBlock(events []*tbtc.NewWalletRegisteredEvent) uint64 {
	var earliest uint64 = math.MaxUint64
	for _, event := range events {
		if event != nil && event.BlockNumber > 0 && event.BlockNumber < earliest {
			earliest = event.BlockNumber
		}
	}
	if earliest == math.MaxUint64 {
		return 0
	}
	return earliest
}

// fetchEventRangeInChunks walks the inclusive block range
// [startBlock, endBlock] in chunks of at most
// tbtc.DepositRevealLookupChunkBlocks blocks, in ascending order, and calls
// onChunk with each chunk's events. onChunk reports whether the walk should
// continue, so a caller interested only in the first chunk that yields
// events can stop there. A fetch error stops the walk and is returned
// wrapped with the failing chunk's range; chunks already delivered to
// onChunk keep their effects, but events are no longer returned to the
// caller, so a failed walk never looks complete. An inverted range
// (startBlock > endBlock) is a no-op that calls neither fetch nor onChunk.
//
// The caller builds a fetch closure for the event type it needs, so one
// implementation serves the wallet reservation scans and the registration
// walk. Both start from a wallet's own past and can therefore span millions
// of blocks, which providers cap for a single eth_getLogs: chunking keeps
// every request inside the cap while still covering the whole range.
func fetchEventRangeInChunks[T any](
	startBlock uint64,
	endBlock uint64,
	fetch func(chunkStartBlock, chunkEndBlock uint64) ([]T, error),
	onChunk func(events []T) (continueWalk bool),
) error {
	if startBlock > endBlock {
		return nil
	}

	for chunkStartBlock := startBlock; ; {
		chunkEndBlock := chunkStartBlock + tbtc.DepositRevealLookupChunkBlocks - 1
		if chunkEndBlock > endBlock || chunkEndBlock < chunkStartBlock {
			chunkEndBlock = endBlock
		}

		chunkEvents, err := fetch(chunkStartBlock, chunkEndBlock)
		if err != nil {
			return fmt.Errorf(
				"fetching events in blocks [%d..%d]: [%w]",
				chunkStartBlock,
				chunkEndBlock,
				err,
			)
		}
		if !onChunk(chunkEvents) {
			return nil
		}

		if chunkEndBlock == endBlock {
			return nil
		}
		chunkStartBlock = chunkEndBlock + 1
	}
}

// scanWalletReservationEventsInChunks walks the inclusive block range
// [startBlock, endBlock] in chunks of at most
// tbtc.DepositRevealLookupChunkBlocks blocks, in ascending order, and
// returns the merged ReservationAcceptanceRequested and
// ReservationReanchorRequested events for the given wallet. The range
// starts at the wallet's earliest registration block, so for a wallet
// closed long after it registered it can span millions of blocks: a
// single eth_getLogs over that span is rejected by providers that cap
// the range, which would fail every re-anchor pass for wallets whose
// providers enforce a cap. Chunking the same way the other reservation
// scans in this repo do (see pkg/maintainer/spv/reservation_event_scan.go
// and the acceptance task's request scan) keeps each request inside the
// cap while still covering the whole range, so correctness (never
// missing a real reservation) is preserved rather than traded away.
//
// A failed chunk fails the scan, and with it this call: the events found
// by earlier chunks are discarded rather than returned as a partial set,
// because callers treat the returned key set as the wallet's complete
// custody. The caller retries on its next pass.
//
// Extracted as a standalone function - mirroring the existing pattern in
// this file (e.g. resolveCustodiedReservationKeys) - so the chunk
// boundaries and the merge can be unit tested directly with fake fetch
// callbacks, since the surrounding TbtcChain methods depend on real
// go-ethereum simulated-backend infrastructure that does not exist
// anywhere in pkg/chain/ethereum today (a package-wide gap, explicitly
// deferred).
func scanWalletReservationEventsInChunks(
	walletPublicKeyHash [20]byte,
	startBlock uint64,
	endBlock uint64,
	fetchAcceptance func(chunkStartBlock, chunkEndBlock uint64) (
		[]*tbtc.ReservationAcceptanceRequestedEvent,
		error,
	),
	fetchReanchor func(chunkStartBlock, chunkEndBlock uint64) (
		[]*tbtc.ReservationReanchorRequestedEvent,
		error,
	),
) ([]*tbtc.ReservationAcceptanceRequestedEvent, []*tbtc.ReservationReanchorRequestedEvent, error) {
	var acceptanceEvents []*tbtc.ReservationAcceptanceRequestedEvent
	err := fetchEventRangeInChunks(
		startBlock,
		endBlock,
		fetchAcceptance,
		func(chunkEvents []*tbtc.ReservationAcceptanceRequestedEvent) bool {
			acceptanceEvents = append(acceptanceEvents, chunkEvents...)
			return true
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot get past reservation acceptance events for [0x%x]: [%w]",
			walletPublicKeyHash,
			err,
		)
	}

	var reanchorEvents []*tbtc.ReservationReanchorRequestedEvent
	err = fetchEventRangeInChunks(
		startBlock,
		endBlock,
		fetchReanchor,
		func(chunkEvents []*tbtc.ReservationReanchorRequestedEvent) bool {
			reanchorEvents = append(reanchorEvents, chunkEvents...)
			return true
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot get past reservation reanchor events for [0x%x]: [%w]",
			walletPublicKeyHash,
			err,
		)
	}

	return acceptanceEvents, reanchorEvents, nil
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
	// matters for logging: the skipped keys of a partially failing
	// lookup must reproduce against the same input rather than
	// depending on Go map iteration order.
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
			// One unreadable reservation must not hide the wallet's other
			// reservations: log it and keep resolving the remaining keys.
			logger.Warnf(
				"skipping reservation [%v] of wallet [0x%x]: cannot get "+
					"reservation: [%v]",
				key,
				walletPublicKeyHash,
				err,
			)
			continue
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

// reservationVaultBindings holds the ReservationVault contract and the TBTC
// token contract the vault keeps its fee reserve in.
type reservationVaultBindings struct {
	vault *tbtccontract.ReservationVault
	tbtc  *tbtccontract.TBTC
}

// reservationVaultBindings returns the cached, per-vault-address bindings
// (the ReservationVault itself and the TBTC token it holds its fee
// reserve in) for the given vault address. Successful constructions are
// cached per vault address. Failures are not cached: they are usually
// transient RPC errors (for example reading tbtcToken()), and caching them
// would disable the fee gauges until the process restarts.
func (tc *TbtcChain) reservationVaultBindings(vaultAddress common.Address) (*reservationVaultBindings, error) {
	if bindings, ok := tc.vaultBindings.Load(vaultAddress); ok {
		return bindings.(*reservationVaultBindings), nil
	}
	vault, err := tbtccontract.NewReservationVault(
		vaultAddress,
		tc.chainID,
		tc.key,
		tc.client,
		tc.nonceManager,
		tc.miningWaiter,
		tc.blockCounter,
		tc.transactionMutex,
	)
	if err != nil {
		err = fmt.Errorf(
			"failed to attach to ReservationVault contract at %s: [%v]",
			vaultAddress,
			err,
		)
		return nil, err
	}
	tokenAddress, err := vault.TbtcToken()
	if err != nil {
		err = fmt.Errorf("failed to read TBTC token address from vault: [%v]", err)
		return nil, err
	}
	tbtcToken, err := tbtccontract.NewTBTC(
		tokenAddress,
		tc.chainID,
		tc.key,
		tc.client,
		tc.nonceManager,
		tc.miningWaiter,
		tc.blockCounter,
		tc.transactionMutex,
	)
	if err != nil {
		err = fmt.Errorf(
			"failed to attach to TBTC contract at %s: [%v]",
			tokenAddress,
			err,
		)
		return nil, err
	}
	bindings := &reservationVaultBindings{vault: vault, tbtc: tbtcToken}
	tc.vaultBindings.Store(vaultAddress, bindings)
	return bindings, nil
}

// decodeReservationVaultAddress decodes a reservation vault address read
// from the Bridge's reservation parameters. It returns the zero address
// when the vault is not configured (zero address in the parameters) and an
// error only when the address cannot be decoded as a valid 20-byte
// address, so callers can distinguish "no vault" (a skip, not an error)
// from a genuinely malformed configuration.
func decodeReservationVaultAddress(
	reservationVault chain.Address,
) (common.Address, error) {
	vaultAddressString := string(reservationVault)
	vaultAddressBytes, err := hexutil.Decode(vaultAddressString)
	if err != nil || len(vaultAddressBytes) != common.AddressLength {
		if err == nil {
			err = fmt.Errorf(
				"unexpected address length %d",
				len(vaultAddressBytes),
			)
		}
		return common.Address{}, fmt.Errorf(
			"cannot decode reservation vault address [%s]: [%v]",
			vaultAddressString,
			err,
		)
	}
	return common.BytesToAddress(vaultAddressBytes), nil
}

// ReservationVaultFeeDebtSat returns the given ReservationVault's
// outstanding in-kind fee debt in satoshi, read from the vault's
// inKindFeeDebtSat view. The caller passes the vault address it already
// read from the reservation parameters; when it is the zero address the
// vault is not configured and the method returns 0 with a nil error: the
// skip sentinel the metrics side consumes, not a chain error.
func (tc *TbtcChain) ReservationVaultFeeDebtSat(
	reservationVault chain.Address,
) (uint64, error) {
	vaultAddress, err := decodeReservationVaultAddress(reservationVault)
	if err != nil {
		return 0, err
	}
	if vaultAddress == (common.Address{}) {
		return 0, nil
	}
	bindings, err := tc.reservationVaultBindings(vaultAddress)
	if err != nil {
		return 0, err
	}
	debtSat, err := bindings.vault.InKindFeeDebtSat()
	if err != nil {
		return 0, fmt.Errorf("cannot read in-kind fee debt: [%v]", err)
	}
	return debtSat, nil
}

// ReservationVaultFeeReserveTbtcBaseUnits returns the given
// ReservationVault's TBTC fee-reserve balance in TBTC base units (whole
// TBTC x 1e18). The value is a token balance of a 1e18-scale token, which
// can exceed uint64 range, so it is returned as *big.Int. When the vault
// is not configured (zero address) the method returns zero with a nil
// error, mirroring ReservationVaultFeeDebtSat's skip sentinel.
func (tc *TbtcChain) ReservationVaultFeeReserveTbtcBaseUnits(
	reservationVault chain.Address,
) (*big.Int, error) {
	vaultAddress, err := decodeReservationVaultAddress(reservationVault)
	if err != nil {
		return new(big.Int), err
	}
	if vaultAddress == (common.Address{}) {
		return new(big.Int), nil
	}
	bindings, err := tc.reservationVaultBindings(vaultAddress)
	if err != nil {
		return new(big.Int), err
	}
	balance, err := bindings.tbtc.BalanceOf(vaultAddress)
	if err != nil {
		return new(big.Int), fmt.Errorf(
			"cannot read TBTC balance of vault: [%v]",
			err,
		)
	}
	return balance, nil
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
