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
	"unicode/utf8"

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

// EVM opcodes the dispatch walk distinguishes. A dispatcher compares the
// calldata selector with a pushed constant, so the compare is what turns the
// constant into a callable entry point.
const (
	eqOpcode     = 0x14
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
// PUSH1..PUSH32, stops before the metadata tail - and only when that tail
// decodes completely as Solidity metadata - and accepts only PUSH4 <selector> immediately
// followed by EQ, the compare a compiler emits at a dispatcher entry.
func dispatchesReservationBuild(code []byte) bool {
	return hasDispatcherEntry(code, reservationBuildSelector)
}

// hasDispatcherEntry reports whether code dispatches on selector: a PUSH4
// instruction whose immediate is the selector, immediately followed by EQ.
//
// The compare is what makes the constant an entry point. A bare PUSH4 - for
// example PUSH4 <selector>; POP - is a pushed constant, and embedded data can
// spell one too, so neither proves that a call can reach a function behind it.
func hasDispatcherEntry(code, selector []byte) bool {
	end := executableCodeLength(code)

	for pc := 0; pc < end; pc++ {
		op := code[pc]
		if op < push1Opcode || op > push32Opcode {
			continue
		}

		if op == push4Opcode &&
			len(selector) == 4 &&
			pc+4 < end &&
			bytes.Equal(code[pc+1:pc+5], selector) &&
			pc+5 < end &&
			code[pc+5] == eqOpcode {
			return true
		}

		pc += int(op-push1Opcode) + 1
	}

	return false
}

// executableCodeLength returns the length of runtime code without the CBOR
// metadata tail solc appends to it.
//
// The trailing two bytes hold the metadata payload's length, which excludes
// those two bytes, so a conventional 51-byte metadata map ends in 0x0033. The
// tail is only excluded when it decodes, in full, as such a map: a contract's last two
// bytes can describe anything, and trimming executable bytes because they
// happen to look like a length would erase an entry point. Unrecognised input
// keeps the whole code in the search window, where the worst case is a refusal.
func executableCodeLength(code []byte) int {
	length, ok := metadataPayloadLength(code)
	if !ok {
		return len(code)
	}

	return len(code) - length - 2
}

// metadataPayloadLength returns the length of the CBOR metadata payload the
// code ends with, and whether that payload decodes, in full, as the metadata
// map Solidity writes.
func metadataPayloadLength(code []byte) (int, bool) {
	if len(code) < 3 {
		return 0, false
	}

	length := int(code[len(code)-2])<<8 | int(code[len(code)-1])
	if length == 0 || length+2 > len(code) {
		return 0, false
	}

	payload := code[len(code)-length-2 : len(code)-2]
	if !isSolidityMetadata(payload) {
		return 0, false
	}

	return length, true
}

// CBOR major types the metadata map uses (RFC 8949, section 3.1).
const (
	cborByteString = 2
	cborTextString = 3
	cborMap        = 5
	cborSimple     = 7

	// cborFalse and cborTrue are the encodings of the simple values false and
	// true: major type 7, additional information 20 and 21.
	cborFalse = cborSimple<<5 | 20
	cborTrue  = cborSimple<<5 | 21

	// maxMetadataEntries is the most keys Solidity writes into the map: ipfs
	// or a bzzr hash, solc, and experimental.
	maxMetadataEntries = 5
)

// isSolidityMetadata reports whether payload is, byte for byte, the CBOR map
// Solidity appends to deployed code:
//
//   - a definite-length map of 1 to 5 entries;
//   - every key a text string from {ipfs, bzzr0, bzzr1, solc, experimental},
//     none of them repeated;
//   - ipfs a 34-byte byte string, bzzr0 and bzzr1 32-byte byte strings, solc
//     either a 3-byte byte string or a text string, experimental a boolean;
//   - no byte left over once the entries are read.
//
// Every length and count must also use its shortest encoding, which is what
// Solidity emits. Anything else is not metadata, and the caller keeps the
// bytes in the search window.
func isSolidityMetadata(payload []byte) bool {
	header, next, ok := readCBORHeader(payload, 0)
	if !ok || header.major != cborMap ||
		header.value < 1 || header.value > maxMetadataEntries {
		return false
	}

	seen := make(map[string]bool, header.value)
	pos := next

	for range header.value {
		keyHeader, valuePos, ok := readCBORHeader(payload, pos)
		if !ok || keyHeader.major != cborTextString ||
			valuePos+keyHeader.value > len(payload) {
			return false
		}

		key := string(payload[valuePos : valuePos+keyHeader.value])
		if seen[key] {
			return false
		}
		seen[key] = true

		pos, ok = skipMetadataValue(payload, key, valuePos+keyHeader.value)
		if !ok {
			return false
		}
	}

	return pos == len(payload)
}

// skipMetadataValue checks that the value of key at payload[pos:] has the type
// and size Solidity gives that key, and returns the position just past it. An
// unknown key is rejected here.
func skipMetadataValue(payload []byte, key string, pos int) (int, bool) {
	if pos >= len(payload) {
		return 0, false
	}

	if key == "experimental" {
		if payload[pos] != cborFalse && payload[pos] != cborTrue {
			return 0, false
		}
		return pos + 1, true
	}

	header, next, ok := readCBORHeader(payload, pos)
	if !ok || next+header.value > len(payload) {
		return 0, false
	}

	end := next + header.value

	switch key {
	case "ipfs":
		return end, header.major == cborByteString && header.value == 34
	case "bzzr0", "bzzr1":
		return end, header.major == cborByteString && header.value == 32
	case "solc":
		switch header.major {
		case cborByteString:
			return end, header.value == 3
		case cborTextString:
			return end, utf8.Valid(payload[next:end])
		}
	}

	return 0, false
}

// cborHeader is a decoded CBOR item head: its major type and its argument,
// which is the entry count of a map and the byte length of a string.
type cborHeader struct {
	major byte
	value int
}

// readCBORHeader decodes the item head at data[pos:] and returns it with the
// position of the first byte after it. It accepts only definite-length heads
// in their shortest form, with an argument that fits in two bytes; indefinite
// lengths, eight-byte and four-byte arguments and padded arguments are
// rejected, as is a head cut short by the end of data.
func readCBORHeader(data []byte, pos int) (cborHeader, int, bool) {
	if pos >= len(data) {
		return cborHeader{}, 0, false
	}

	major := data[pos] >> 5
	info := int(data[pos] & 0x1f)
	pos++

	switch {
	case info < 24:
		return cborHeader{major, info}, pos, true
	case info == 24:
		if pos+1 > len(data) || data[pos] < 24 {
			return cborHeader{}, 0, false
		}
		return cborHeader{major, int(data[pos])}, pos + 1, true
	case info == 25:
		if pos+2 > len(data) {
			return cborHeader{}, 0, false
		}
		value := int(data[pos])<<8 | int(data[pos+1])
		if value < 256 {
			return cborHeader{}, 0, false
		}
		return cborHeader{major, value}, pos + 2, true
	}

	return cborHeader{}, 0, false
}
