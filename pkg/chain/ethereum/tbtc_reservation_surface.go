// This file identifies a Bridge deployed before the reservations upgrade,
// which is what tells the deposit-sweep gate the difference between
// "reservations are not live on chain" and "this deployment cannot hold a
// reserved deposit at all".
//
// The identification is a property of the deployment, never of a failed read.
// An error shape cannot establish which functions a contract has: the
// reservations-build Bridge answers reservation selectors through a fallback
// that delegatecalls the reservation router and bubbles the child's result, so
// any child failure (a delegated call that runs out of gas, for instance)
// surfaces as an execution revert with no returndata, and the generated
// bindings rewrap failures through keep-common's error resolver anyway.
//
// The proof therefore comes from the deployed code. getReservationRouter() is
// declared on the Bridge itself only by the reservations build (the
// reservation getters served through the fallback are not part of the marker),
// so its selector appears in that build's dispatcher and in no earlier one, and
// the absence of the selector from every contract in the deployment's
// implementation chain proves the deployment cannot serve the reservation
// surface. Measured against the live deployment: the pre-reservations
// implementation carries the depositParameters() selector and not this one,
// while a reservations-build artifact carries both.

package ethereum

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationBuildSelector is the 4-byte selector of getReservationRouter(),
// the one reservation API the reservations build declares on the Bridge
// itself. Pinned against the generated Bridge ABI by
// TestReservationBuildSelector.
var reservationBuildSelector = []byte{0x51, 0x57, 0xec, 0x0a}

// eip1967ImplementationSlot is the storage slot EIP-1967 reserves for a
// proxy's implementation address:
// keccak256("eip1967.proxy.implementation") - 1. The mainnet Bridge is a
// proxy, so the dispatcher that answers its calls lives one contract further
// along; reading only the Bridge address would find no function selectors at
// all.
var eip1967ImplementationSlot = common.HexToHash(
	"0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc",
)

// maxImplementationDepth bounds the implementation-chain walk. The Bridge is
// one proxy over one implementation, so the walk is two contract reads in
// practice; a deployment nesting deeper than this is not a shape this code can
// classify, and it refuses rather than guessing.
const maxImplementationDepth = 4

// reservationReadFailure classifies a failed reservation-parameters read.
//
// It returns tbtc.ErrReservationsSurfaceUnavailable only when the deployed
// code proves the Bridge predates the reservations build: no contract along
// the Bridge's implementation chain carries the reservations-build dispatcher
// selector, and the chain ends at a contract with no implementation of its
// own. Everything else is an ordinary error, so the sweep gate refuses the
// sweep: a reservations-build deployment whose read failed for any reason, and
// equally every deployment whose code could not be read or classified.
func (tc *TbtcChain) reservationReadFailure(readErr error) error {
	preReservations, err := tc.reservationSurfaceAbsent()
	if err != nil {
		return fmt.Errorf(
			"cannot get reservation parameters: [%w]; the deployment could "+
				"not be classified: [%v]",
			readErr,
			err,
		)
	}
	if !preReservations {
		return fmt.Errorf("cannot get reservation parameters: [%w]", readErr)
	}

	return fmt.Errorf(
		"%w: [%v]",
		tbtc.ErrReservationsSurfaceUnavailable,
		readErr,
	)
}

// reservationSurfaceAbsent reports whether the deployed contract at the
// Bridge address provably predates the reservations build.
//
// It reads the code at the Bridge address and then at each EIP-1967
// implementation that address points at, and reports true only when every step
// had code, no step carried the reservations-build selector, and the chain
// ended at a contract with no implementation of its own. A code or storage
// read that fails is an error and never an absence: it says nothing about
// which functions exist.
func (tc *TbtcChain) reservationSurfaceAbsent() (bool, error) {
	ctx := context.Background()
	address := tc.bridgeAddress

	for range maxImplementationDepth {
		code, err := tc.client.CodeAt(ctx, address, nil)
		if err != nil {
			return false, fmt.Errorf(
				"cannot read the code of %s: [%v]",
				address.Hex(),
				err,
			)
		}
		if len(code) == 0 {
			return false, fmt.Errorf("%s has no code", address.Hex())
		}
		if bytes.Contains(code, reservationBuildSelector) {
			return false, nil
		}

		slot, err := tc.client.StorageAt(
			ctx,
			address,
			eip1967ImplementationSlot,
			nil,
		)
		if err != nil {
			return false, fmt.Errorf(
				"cannot read the EIP-1967 implementation slot of %s: [%v]",
				address.Hex(),
				err,
			)
		}
		if len(slot) != common.HashLength {
			return false, fmt.Errorf(
				"the EIP-1967 implementation slot of %s is %d bytes wide",
				address.Hex(),
				len(slot),
			)
		}

		implementation := common.BytesToAddress(slot)
		if implementation == (common.Address{}) {
			return true, nil
		}
		address = implementation
	}

	return false, fmt.Errorf(
		"the implementation chain from %s is deeper than %d contracts",
		tc.bridgeAddress.Hex(),
		maxImplementationDepth,
	)
}
