// Tests for the reservation code paths that the M1 coverage measurement
// left uncovered. Each test drives the real code path, so it fails on a
// build that drops the guard it pins.
package tbtc

import (
	"errors"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// TestReservationEnumStrings pins the human-readable names of the three
// reservation enums, including the fallback arm of each String method.
//
// The three String methods are the label source for the reservation
// notification and action-log paths: a wrong label would make an
// operator read "ReservationActionType(1)" where "Acceptance" belongs,
// and a wrong fallback would print an out-of-range value under a known
// name. Each method is checked for every declared constant and for one
// value outside the declared range, which is the arm a constant added
// later would fall into.
func TestReservationEnumStrings(t *testing.T) {
	t.Run("action type", func(t *testing.T) {
		tests := map[ReservationActionType]string{
			ReservationActionTypeNone:        "None",
			ReservationActionTypeAcceptance:  "Acceptance",
			ReservationActionTypeRedemption:  "Redemption",
			ReservationActionTypeReanchor:    "Reanchor",
			ReservationActionTypeDissolution: "Dissolution",
		}

		for actionType, expected := range tests {
			if actual := actionType.String(); actual != expected {
				t.Errorf(
					"unexpected reservation action type string\nexpected: [%v]\nactual:   [%v]",
					expected,
					actual,
				)
			}
		}

		outOfRange := ReservationActionType(200)
		expected := "ReservationActionType(200)"
		if actual := outOfRange.String(); actual != expected {
			t.Errorf(
				"unexpected out-of-range reservation action type string\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	})

	t.Run("termination cause", func(t *testing.T) {
		tests := map[WalletTerminationCause]string{
			WalletTerminationCauseUnknown:                "Unknown",
			WalletTerminationCauseMovingFundsTimeout:     "MovingFundsTimeout",
			WalletTerminationCauseMovedFundsSweepTimeout: "MovedFundsSweepTimeout",
			WalletTerminationCauseFraudChallengeDefeat:    "FraudChallengeDefeat",
		}

		for cause, expected := range tests {
			if actual := cause.String(); actual != expected {
				t.Errorf(
					"unexpected wallet termination cause string\nexpected: [%v]\nactual:   [%v]",
					expected,
					actual,
				)
			}
		}

		outOfRange := WalletTerminationCause(201)
		expected := "WalletTerminationCause(201)"
		if actual := outOfRange.String(); actual != expected {
			t.Errorf(
				"unexpected out-of-range wallet termination cause string\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	})

	t.Run("action state", func(t *testing.T) {
		tests := map[ReservationActionState]string{
			ReservationActionStateUnknown:    "Unknown",
			ReservationActionStatePending:    "Pending",
			ReservationActionStateSettled:    "Settled",
			ReservationActionStateTimedOut:   "TimedOut",
			ReservationActionStateVetoed:     "Vetoed",
			ReservationActionStateSuperseded: "Superseded",
		}

		for state, expected := range tests {
			if actual := state.String(); actual != expected {
				t.Errorf(
					"unexpected reservation action state string\nexpected: [%v]\nactual:   [%v]",
					expected,
					actual,
				)
			}
		}

		outOfRange := ReservationActionState(202)
		expected := "ReservationActionState(202)"
		if actual := outOfRange.String(); actual != expected {
			t.Errorf(
				"unexpected out-of-range reservation action state string\nexpected: [%v]\nactual:   [%v]",
				expected,
				actual,
			)
		}
	})
}

// TestReservationEnumStringFallbacksAreNotAliased is the negative control
// for TestReservationEnumStrings: it fails if any fallback arm repeats a
// declared constant's name.
//
// The fallback arm is the only place a value outside the declared range
// gets a label. A fallback that aliased a declared name would make an
// out-of-range value indistinguishable from a valid one in an operator's
// log, which is exactly the failure the fallback exists to prevent.
func TestReservationEnumStringFallbacksAreNotAliased(t *testing.T) {
	declaredActionTypes := map[string]struct{}{
		"None":        {},
		"Acceptance":  {},
		"Redemption":  {},
		"Reanchor":    {},
		"Dissolution": {},
	}
	declaredCauses := map[string]struct{}{
		"Unknown":                {},
		"MovingFundsTimeout":     {},
		"MovedFundsSweepTimeout": {},
		"FraudChallengeDefeat":   {},
	}
	declaredStates := map[string]struct{}{
		"Unknown":    {},
		"Pending":    {},
		"Settled":    {},
		"TimedOut":   {},
		"Vetoed":     {},
		"Superseded": {},
	}

	if fallback := ReservationActionType(200).String(); fallback != "ReservationActionType(200)" {
		if _, aliased := declaredActionTypes[fallback]; aliased {
			t.Errorf("reservation action type fallback aliases a declared name: %v", fallback)
		}
	}

	if fallback := WalletTerminationCause(201).String(); fallback != "WalletTerminationCause(201)" {
		if _, aliased := declaredCauses[fallback]; aliased {
			t.Errorf("wallet termination cause fallback aliases a declared name: %v", fallback)
		}
	}

	if fallback := ReservationActionState(202).String(); fallback != "ReservationActionState(202)" {
		if _, aliased := declaredStates[fallback]; aliased {
			t.Errorf("reservation action state fallback aliases a declared name: %v", fallback)
		}
	}
}

// anchorFundingTxErrorChain is a rangeRevealChain whose
// GetDepositRequest read fails on demand.
type anchorFundingTxErrorChain struct {
	*rangeRevealChain
	depositRequestErr error
}

func (c *anchorFundingTxErrorChain) GetDepositRequest(
	txHash bitcoin.Hash,
	outputIndex uint32,
) (*DepositChainRequest, bool, error) {
	if c.depositRequestErr != nil {
		return nil, false, c.depositRequestErr
	}
	return c.localChain.GetDepositRequest(txHash, outputIndex)
}

// TestReservationAnchorAction_RejectsDepositRequestReadError pins the
// GetDepositRequest error return in the anchor execute path.
//
// The anchor action reads the deposit request to learn the deposit's
// reveal timestamp, which anchors the reveal-event lookup window. A
// failed read must fail the action with the wrap text, not pass a zero
// timestamp into the lookup and scan the wrong block range.
func TestReservationAnchorAction_RejectsDepositRequestReadError(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, signExecutor, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nil,
		nil,
	)
	inner := action.chain.(*rangeRevealChain)
	action.chain = &anchorFundingTxErrorChain{
		rangeRevealChain:  inner,
		depositRequestErr: errors.New("deposit request read failed"),
	}

	err := action.execute()
	if err == nil {
		t.Fatal("expected the deposit request read error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot fetch deposit request") {
		t.Fatalf("expected the deposit request wrap text, got: [%v]", err)
	}
	if got := signExecutor.startCalls(); got != 0 {
		t.Errorf(
			"expected signing to be unreached, got [%d] sign calls",
			got,
		)
	}
}

// anchorMissingDepositRequestChain is a rangeRevealChain whose
// GetDepositRequest read succeeds but reports that no request exists.
type anchorMissingDepositRequestChain struct {
	*rangeRevealChain
}

func (c *anchorMissingDepositRequestChain) GetDepositRequest(
	txHash bitcoin.Hash,
	outputIndex uint32,
) (*DepositChainRequest, bool, error) {
	return nil, false, nil
}

// TestReservationAnchorAction_RejectsMissingDepositRequest pins the
// "deposit request not found" return in the anchor execute path.
//
// A deposit request that the chain does not have is not the same as a
// failed read: the read succeeded and found nothing. The action must
// refuse to proceed, because without the reveal timestamp the reveal
// lookup has no window to scan.
func TestReservationAnchorAction_RejectsMissingDepositRequest(t *testing.T) {
	custody, walletPKH := newCustodyWallet(t)
	action, signExecutor, _ := newAnchorErrorFixture(
		t,
		custody,
		walletPKH,
		100000,
		nil,
		nil,
	)
	inner := action.chain.(*rangeRevealChain)
	// Make the deposit request read succeed but report that no request
	// exists for this funding outpoint.
	action.chain = &anchorMissingDepositRequestChain{rangeRevealChain: inner}

	err := action.execute()
	if err == nil {
		t.Fatal("expected the missing deposit request error, got nil")
	}
	if !strings.Contains(err.Error(), "deposit request not found") {
		t.Fatalf("expected the missing deposit request text, got: [%v]", err)
	}
	if got := signExecutor.startCalls(); got != 0 {
		t.Errorf(
			"expected signing to be unreached, got [%d] sign calls",
			got,
		)
	}
}
