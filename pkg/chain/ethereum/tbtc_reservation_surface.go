// This file classifies a failed reservation-parameters read, which is what
// tells the deposit-sweep gate the difference between "reservations are not
// live on chain" and "this deployment predates reservations at all".
//
// The classification cannot be inferred from the bindings' error: every
// generated view call routes its failure through keep-common's error resolver,
// which re-issues the call and rewraps the outcome, so the caller never sees
// the node's own revert. The proof therefore comes from a dedicated raw call
// (reservationSurfaceAvailable) whose error is the node's, unimpeded.

package ethereum

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationReadFailure classifies a failed reservation-parameters read.
//
// It returns tbtc.ErrReservationsSurfaceUnavailable only when the failure
// proves the Bridge has no reservation surface: the reservation getter
// reverted without revert data AND the same Bridge answered a getter that
// predates the reservations upgrade. A contract that answers its
// pre-reservation getters and reverts a reservation selector without data has
// no function for it, which is what a pre-reservations deployment looks like.
//
// Every other failure is returned as an ordinary error: a transport failure
// or timeout never proves absence, a revert carrying data is a deliberate
// rejection by an implementation that has the function, and a Bridge that
// cannot answer its own pre-upgrade getters is not a usable deployment to
// reason about at all.
func (tc *TbtcChain) reservationReadFailure(readErr error) error {
	available, probeErr := tc.reservationSurfaceAvailable()
	if probeErr != nil {
		return fmt.Errorf(
			"cannot get reservation parameters: [%w]; the reservation "+
				"surface probe could not determine the deployment: [%v]",
			readErr,
			probeErr,
		)
	}
	if available {
		// The Bridge answered the reservation getter on its own, so the
		// failed read is a transient or unrelated fault, not a deployment
		// without reservations.
		return fmt.Errorf("cannot get reservation parameters: [%w]", readErr)
	}

	// The probe proved the reservation selector is absent. Confirm the Bridge
	// is a live deployment answering its pre-reservations API: a Bridge that
	// cannot is not something to infer the deployment's age from.
	if _, getterErr := tc.bridge.DepositParameters(); getterErr != nil {
		return fmt.Errorf(
			"cannot get reservation parameters: [%w]; the Bridge also "+
				"failed its pre-reservations deposit-parameters getter: [%v]",
			readErr,
			getterErr,
		)
	}

	return fmt.Errorf(
		"%w: [%v]",
		tbtc.ErrReservationsSurfaceUnavailable,
		readErr,
	)
}

// reservationSurfaceAvailable reports whether the Bridge answers the
// reservation getter reservationParameters().
//
// The probe issues the raw eth_call itself instead of going through the
// generated binding: the bindings wrap every failure in keep-common's error
// resolver, which re-issues the call and rewraps the result, so the caller
// never sees the node's own revert. The classification needs exactly that
// revert, because the only proof that a deployment has no reservation surface
// is a call to a reservation selector that reverts without revert data.
//
// It returns (true, nil) when the call returns data, (false, nil) when the
// call reverts without revert data - the shape of a selector the contract does
// not implement - and (false, err) for every other outcome, including a call
// that returns nothing at all (no code at the address).
func (tc *TbtcChain) reservationSurfaceAvailable() (bool, error) {
	call := ethereum.CallMsg{
		From: tc.key.Address,
		To:   &tc.bridgeAddress,
		Data: reservationParametersSelector[:],
	}

	data, err := tc.client.CallContract(context.Background(), call, nil)
	switch {
	case err != nil && isRevertWithoutData(err):
		return false, nil
	case err != nil:
		return false, err
	case len(data) == 0:
		return false, fmt.Errorf(
			"the Bridge returned no data for reservationParameters()",
		)
	default:
		return true, nil
	}
}

// reservationParametersSelector is the 4-byte selector of
// reservationParameters(), the ReservationRouter getter the Bridge answers
// through its fallback once reservations are deployed. Pinned against the
// generated ABI by TestReservationParametersSelector.
var reservationParametersSelector = [4]byte{0xf7, 0x5b, 0x4b, 0x1c}

// isRevertWithoutData reports whether err is an EVM execution revert that
// carries no revert data, which is the shape of a call to a selector the
// target contract does not implement: no matching function, and no fallback
// that answered it.
//
// Revert data disqualifies the error: a payload means an existing function
// rejected the call with a reason, a panic code or a custom error. So does
// every non-revert error: a transport failure, a timeout, a rate limit or a
// client-side decoding problem says nothing about which functions exist.
func isRevertWithoutData(err error) bool {
	if err == nil {
		return false
	}

	if revertDataPresent(err) {
		return false
	}

	// go-ethereum's JSON-RPC and in-process EVM both surface an empty-data
	// revert as an error whose whole message is "execution reverted"; a revert
	// with a reason decodes to "execution reverted: <reason>". Walk the wrap
	// chain so a caller that wrapped the binding error does not hide it.
	for candidate := err; candidate != nil; candidate = errors.Unwrap(candidate) {
		if strings.EqualFold(
			strings.TrimSpace(candidate.Error()),
			executionRevertedMessage,
		) {
			return true
		}
	}

	return false
}

// revertDataPresent reports whether err carries non-empty revert data. An
// explicitly empty payload is not data: some providers report an empty-data
// revert as the execution-reverted message with data "0x".
func revertDataPresent(err error) bool {
	var dataErr interface{ ErrorData() interface{} }
	if !errors.As(err, &dataErr) {
		return false
	}

	switch data := dataErr.ErrorData().(type) {
	case nil:
		return false
	case []byte:
		return len(data) > 0
	case string:
		hexData := strings.TrimSpace(strings.TrimPrefix(data, "0x"))
		return len(strings.Trim(hexData, "0")) > 0
	default:
		return true
	}
}

// executionRevertedMessage is the message go-ethereum uses for an EVM
// execution revert that carries no revert data.
const executionRevertedMessage = "execution reverted"
