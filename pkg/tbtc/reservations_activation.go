package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
)

// ReservationsActivationSeverity classifies how serious a reservations
// activation inconsistency found at startup is.
type ReservationsActivationSeverity uint8

const (
	// ReservationsActivationConsistent means the client's compiled
	// activation block and the contract's reported reservation state agree.
	ReservationsActivationConsistent ReservationsActivationSeverity = iota
	// ReservationsActivationWarning means the two disagree in a way that
	// does not yet endanger funds but needs an operator's attention.
	ReservationsActivationWarning
	// ReservationsActivationError means the C-4 activation ordering gate
	// has been violated: reserved deposits can be accepted on-chain while
	// no client coordinates their anchors.
	ReservationsActivationError
)

// String returns the human-readable name of the severity.
func (s ReservationsActivationSeverity) String() string {
	switch s {
	case ReservationsActivationWarning:
		return "warning"
	case ReservationsActivationError:
		return "error"
	default:
		return "consistent"
	}
}

// ReservationsActivationStatus is the set of inputs the reservations
// activation check compares.
//
// ActivationBlock is the network's activation block as compiled into this
// binary; it is math.MaxUint64 when the network has no entry in
// reservationsActivationBlocks, which is the "never activates" fallback of
// ReservationsActivationBlock. Configured distinguishes that fallback from
// a network that really will activate at the given block.
//
// ChainHead is the current Ethereum block number.
//
// ContractActive reports whether the contract considers reservations live:
// the Bridge trusts the reservation vault (setVaultStatus was executed for
// it). It is false when no reservation vault is configured as well, since
// reservations are then disabled on-chain.
type ReservationsActivationStatus struct {
	Network         ethereum.Network
	ActivationBlock uint64
	Configured      bool
	ChainHead       uint64
	ContractActive  bool
}

// ReservationsActivationVaultReader supplies the on-chain half of the
// activation check: whether the Bridge currently trusts the reservation
// vault configured in the reservation parameters. It is satisfied by
// *ethereum.TbtcChain.
type ReservationsActivationVaultReader interface {
	// ReservationsVaultTrusted reports whether the Bridge trusts the
	// reservation vault configured in the reservation parameters. It
	// returns false with a nil error when no reservation vault is
	// configured: reservations are then disabled on-chain.
	ReservationsVaultTrusted() (bool, error)
}

// ReservationsActivationBlockAndState returns the network's activation block
// as compiled into this binary and whether the network will activate at all.
//
// The second return value is false for every network without an explicit
// entry in reservationsActivationBlocks (including ethereum.Unknown): such a
// network never activates, and the returned block is math.MaxUint64. It is
// true for ethereum.Developer, which activates immediately at block 0.
func ReservationsActivationBlockAndState(
	network ethereum.Network,
) (uint64, bool) {
	if network == ethereum.Developer {
		return 0, true
	}

	if block, ok := reservationsActivationBlocks[network]; ok {
		return block, true
	}

	return ReservationsActivationBlock(network), false
}

// EvaluateReservationsActivation compares the client's compiled activation
// block for the running network with the contract's reported reservation
// state and returns the startup warning that describes the inconsistency.
//
// The returned message is empty when the two agree. The C-4 ordering gate
// requires the client carrying the activation block to be deployed before
// the contract activates the reservations vault, so:
//
//   - The chain head is past the activation block but the contract is not
//     active: the client is ready and the on-chain activation has not
//     happened yet. Reserved deposits cannot be revealed as reserved, so
//     no funds are at risk, but the operator should know the on-chain
//     activation is still pending.
//
//   - The contract is active but the chain head is below the activation
//     block: the on-chain activation ran before this binary's activation
//     block was reached. Reserved deposits can now be revealed and
//     accepted on-chain while this client does not coordinate their
//     anchors, which is the C-4 violation the activation block exists to
//     prevent.
//
//   - The network has no activation block at all: the client will never
//     coordinate reservations on it, whatever the contract does.
func EvaluateReservationsActivation(
	status ReservationsActivationStatus,
) (string, ReservationsActivationSeverity) {
	network := status.Network

	if !status.Configured {
		if status.ContractActive {
			return fmt.Sprintf(
				"reservations are compiled into this client but there is "+
					"no activation block for network [%s], so this client "+
					"will never coordinate reservations, while the Bridge "+
					"already trusts the reservation vault; reserved "+
					"deposits can be accepted on-chain with no client to "+
					"coordinate their anchors; set the network's "+
					"activation block in reservationsActivationBlocks and "+
					"ship a release carrying it before any further "+
					"reservation activity",
				network,
			), ReservationsActivationError
		}

		return fmt.Sprintf(
			"reservations are compiled into this client but there is no "+
				"activation block for network [%s], so reservations will "+
				"never activate on this network; the release owner must "+
				"choose the network's activation block and ship a release "+
				"carrying it before the contract activates the reservation "+
				"vault",
			network,
		), ReservationsActivationWarning
	}

	if status.ContractActive && status.ChainHead < status.ActivationBlock {
		return fmt.Sprintf(
			"the Bridge already trusts the reservation vault (reservations "+
				"are active on-chain) but the chain head [%d] is below this "+
				"client's activation block [%d] for network [%s]; reserved "+
				"deposits can be revealed and accepted on-chain before this "+
				"client starts coordinating their anchors; the activation "+
				"block must not be reached after the on-chain activation",
			status.ChainHead,
			status.ActivationBlock,
			network,
		), ReservationsActivationError
	}

	if !status.ContractActive && status.ChainHead >= status.ActivationBlock {
		return fmt.Sprintf(
			"the chain head [%d] has reached this client's activation "+
				"block [%d] for network [%s] but the Bridge does not "+
				"trust the reservation vault, so reservations are not live "+
				"on-chain yet; the on-chain activation is still pending",
			status.ChainHead,
			status.ActivationBlock,
			network,
		), ReservationsActivationWarning
	}

	return "", ReservationsActivationConsistent
}

// CollectReservationsActivationStatus reads the chain head and the
// contract's reported reservation state for the given network and returns
// the activation status the check compares.
//
// The activation block is the one compiled into this binary (see
// ReservationsActivationBlockAndState); the chain head is read through the
// given block counter and the contract state through the given vault reader.
func CollectReservationsActivationStatus(
	network ethereum.Network,
	blockCounter chain.BlockCounter,
	vaultReader ReservationsActivationVaultReader,
) (ReservationsActivationStatus, error) {
	activationBlock, configured := ReservationsActivationBlockAndState(network)

	status := ReservationsActivationStatus{
		Network:         network,
		ActivationBlock: activationBlock,
		Configured:      configured,
	}

	if blockCounter != nil {
		head, err := blockCounter.CurrentBlock()
		if err != nil {
			return status, fmt.Errorf(
				"cannot get the chain head: [%w]",
				err,
			)
		}
		status.ChainHead = head
	}

	if vaultReader != nil {
		trusted, err := vaultReader.ReservationsVaultTrusted()
		if err != nil {
			return status, fmt.Errorf(
				"cannot check whether the reservation vault is trusted: [%w]",
				err,
			)
		}
		status.ContractActive = trusted
	}

	return status, nil
}
