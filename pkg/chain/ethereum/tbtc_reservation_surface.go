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
// The proof therefore comes from the deployment's code and its EIP-1967 proxy
// pointers. getReservationRouter() is declared on the Bridge itself only by the
// reservations build (the reservation getters served through the fallback are
// not part of the marker), so a PUSH4 of its selector in that build's
// dispatcher identifies the reservations build, and its absence from every
// contract in the implementation chain - with no beacon-resolved or otherwise
// unresolved proxy shape - proves the deployment cannot serve the reservation
// surface.
//
// Measured against the live deployment: the mainnet Bridge proxy carries no
// such dispatch and points its implementation slot at the pre-reservations
// implementation, which carries depositParameters()'s dispatch and not the
// reservations one, and whose implementation, admin and beacon slots are all
// zero. A reservations-build Bridge artifact carries the dispatch.

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
// itself. TestReservationBuildSelector pins it to that signature.
var reservationBuildSelector = []byte{0x51, 0x57, 0xec, 0x0a}

// EIP-1967 proxy storage slots: keccak256(name) - 1 for each reserved name.
// The mainnet Bridge is a proxy, so the dispatcher that answers its calls lives
// one contract further along; reading only the Bridge address would find no
// function selectors at all.
var (
	// eip1967ImplementationSlot holds a proxy's implementation address.
	eip1967ImplementationSlot = common.HexToHash(
		"0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc",
	)
	// eip1967AdminSlot holds a transparent proxy's admin address.
	eip1967AdminSlot = common.HexToHash(
		"0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103",
	)
	// eip1967BeaconSlot holds a beacon proxy's beacon address. A beacon proxy
	// deliberately leaves the implementation slot empty and resolves its
	// implementation through the beacon's own implementation() call, so a
	// zero implementation slot says nothing about the target behind it.
	eip1967BeaconSlot = common.HexToHash(
		"0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50",
	)
)

// maxImplementationDepth bounds the implementation-chain walk. The Bridge is
// one proxy over one implementation, so the walk is two contract reads in
// practice; a deployment nesting deeper than this is not a shape this code can
// classify, and it refuses rather than guessing.
const maxImplementationDepth = 4

// EVM opcodes the dispatch walk distinguishes.
const (
	push1Opcode  = 0x60
	push4Opcode  = 0x63
	push32Opcode = 0x7f
)

// reservationReadFailure classifies a failed reservation-parameters read.
//
// It returns tbtc.ErrReservationsSurfaceUnavailable only when the deployed
// code proves the Bridge predates the reservations build: no contract along
// the Bridge's implementation chain dispatches the reservations-build selector,
// and the chain ends at a contract with no implementation, admin or beacon of
// its own. Everything else is an ordinary error, so the sweep gate refuses the
// sweep: a reservations-build deployment whose read failed for any reason, and
// equally every deployment whose code or proxy shape could not be read or
// classified.
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
// had code, no step dispatched the reservations-build selector, and the chain
// ended at a contract with no implementation, admin or beacon of its own. The
// admin and beacon checks are there because a zero implementation slot does not
// mean "no implementation": a beacon proxy keeps its target in the beacon slot,
// and a proxy carrying an admin but no implementation is a shape this code
// cannot resolve. A code or storage read that fails is an error and never an
// absence: it says nothing about which functions exist.
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
		if dispatchesReservationBuild(code) {
			return false, nil
		}

		implementation, err := tc.eip1967Slot(ctx, address, eip1967ImplementationSlot)
		if err != nil {
			return false, err
		}
		if implementation != (common.Address{}) {
			address = implementation
			continue
		}

		// No implementation pointer. That means either this is the deployed
		// contract itself, or the delegation resolves through something this
		// code cannot follow; only the former is evidence about the functions
		// the deployment serves, so the other shapes refuse the sweep.
		beacon, err := tc.eip1967Slot(ctx, address, eip1967BeaconSlot)
		if err != nil {
			return false, err
		}
		if beacon != (common.Address{}) {
			return false, fmt.Errorf(
				"%s is an EIP-1967 beacon proxy (beacon %s) whose "+
					"implementation this classifier does not resolve",
				address.Hex(),
				beacon.Hex(),
			)
		}

		admin, err := tc.eip1967Slot(ctx, address, eip1967AdminSlot)
		if err != nil {
			return false, err
		}
		if admin != (common.Address{}) {
			return false, fmt.Errorf(
				"%s carries an EIP-1967 admin (%s) but no implementation "+
					"pointer, which is a proxy shape this classifier does "+
					"not resolve",
				address.Hex(),
				admin.Hex(),
			)
		}

		return true, nil
	}

	return false, fmt.Errorf(
		"the implementation chain from %s is deeper than %d contracts",
		tc.bridgeAddress.Hex(),
		maxImplementationDepth,
	)
}

// eip1967Slot reads one EIP-1967 storage word of a contract and returns the
// address it holds.
func (tc *TbtcChain) eip1967Slot(
	ctx context.Context,
	address common.Address,
	slot common.Hash,
) (common.Address, error) {
	word, err := tc.client.StorageAt(ctx, address, slot, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf(
			"cannot read the EIP-1967 slot %s of %s: [%v]",
			slot.Hex(),
			address.Hex(),
			err,
		)
	}
	if len(word) != common.HashLength {
		return common.Address{}, fmt.Errorf(
			"the EIP-1967 slot %s of %s is %d bytes wide",
			slot.Hex(),
			address.Hex(),
			len(word),
		)
	}

	return common.BytesToAddress(word), nil
}

// dispatchesReservationBuild reports whether runtime code contains an
// executable dispatch on the reservations-build selector.
//
// A raw substring match is not enough: the same four bytes can appear inside a
// PUSH immediate, in embedded data, or in the CBOR metadata tail solc appends
// to every deployed contract, and none of those makes a function callable. The
// walk therefore moves instruction by instruction, skipping the operands of
// PUSH1..PUSH32, stops before the metadata tail, and only accepts a PUSH4
// instruction whose immediate is the selector - the shape a compiler emits for
// a dispatcher entry.
func dispatchesReservationBuild(code []byte) bool {
	return hasDispatcherEntry(code, reservationBuildSelector)
}

// hasDispatcherEntry reports whether code contains a PUSH4 instruction whose
// immediate is selector, outside the metadata tail.
func hasDispatcherEntry(code, selector []byte) bool {
	end := executableCodeLength(code)

	for pc := 0; pc < end; pc++ {
		switch op := code[pc]; {
		case op == push4Opcode:
			// A trailing, truncated PUSH4 cannot be a dispatch entry.
			if pc+4 < end && bytes.Equal(code[pc+1:pc+5], selector) {
				return true
			}
			pc += 4
		case op >= push1Opcode && op <= push32Opcode:
			pc += int(op-push1Opcode) + 1
		}
	}

	return false
}

// executableCodeLength returns the length of runtime code without the CBOR
// metadata tail solc appends to it. The final two bytes hold the tail's length,
// including those two bytes; a value that cannot describe a tail leaves the
// whole code executable.
func executableCodeLength(code []byte) int {
	if len(code) < 2 {
		return len(code)
	}

	tail := int(code[len(code)-2])<<8 | int(code[len(code)-1])
	if tail < 2 || tail > len(code) {
		return len(code)
	}

	return len(code) - tail
}
