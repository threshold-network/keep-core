package tbtc

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
)

// activationStatusTestChain is a chain.BlockCounter stub returning a fixed
// head block.
type activationStatusTestChain struct {
	head uint64
	err  error
}

func (c *activationStatusTestChain) CurrentBlock() (uint64, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.head, nil
}

func (c *activationStatusTestChain) WaitForBlockHeight(_ uint64) error {
	return nil
}

func (c *activationStatusTestChain) BlockHeightWaiter(_ uint64) (<-chan uint64, error) {
	ch := make(chan uint64)
	close(ch)
	return ch, nil
}

func (c *activationStatusTestChain) WatchBlocks(_ context.Context) <-chan uint64 {
	ch := make(chan uint64)
	close(ch)
	return ch
}

// activationStatusTestVault is a ReservationsActivationVaultReader stub
// returning a fixed trust state.
type activationStatusTestVault struct {
	trusted bool
	err     error
}

func (v *activationStatusTestVault) ReservationsVaultTrusted() (bool, error) {
	if v.err != nil {
		return false, v.err
	}
	return v.trusted, nil
}

// TestEvaluateReservationsActivation_NoActivationBlock verifies the two
// warnings raised when the running network has no entry in
// reservationsActivationBlocks: the client will never coordinate
// reservations on it.
//
// The severity differs by whether the contract is already active: with the
// contract active, reserved deposits can be accepted on-chain with no client
// to coordinate their anchors, which is the C-4 violation the activation
// block exists to prevent.
func TestEvaluateReservationsActivation_NoActivationBlock(t *testing.T) {
	tests := map[string]struct {
		status        ReservationsActivationStatus
		wantSeverity  ReservationsActivationSeverity
		wantSubstring string
	}{
		"contract inactive, no activation block": {
			status: ReservationsActivationStatus{
				Network:         ethereum.Mainnet,
				ActivationBlock: 0,
				Configured:      false,
				ChainHead:       100,
				ContractActive:  false,
			},
			wantSeverity:  ReservationsActivationWarning,
			wantSubstring: "no activation block for network [mainnet]",
		},
		"contract active, no activation block": {
			status: ReservationsActivationStatus{
				Network:         ethereum.Mainnet,
				ActivationBlock: 0,
				Configured:      false,
				ChainHead:       100,
				ContractActive:  true,
			},
			wantSeverity:  ReservationsActivationError,
			wantSubstring: "will never coordinate reservations",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, severity := EvaluateReservationsActivation(test.status)

			if severity != test.wantSeverity {
				t.Errorf(
					"severity = %v, want %v",
					severity,
					test.wantSeverity,
				)
			}

			if message == "" {
				t.Fatal("expected a warning message, got none")
			}

			if !strings.Contains(message, test.wantSubstring) {
				t.Errorf(
					"message %q does not contain %q",
					message,
					test.wantSubstring,
				)
			}
		})
	}
}

// TestEvaluateReservationsActivation_HeadPastBlockContractInactive verifies
// the warning raised when the chain head has reached the client's
// activation block but the contract does not report reservations as active:
// the on-chain activation is still pending.
func TestEvaluateReservationsActivation_HeadPastBlockContractInactive(t *testing.T) {
	status := ReservationsActivationStatus{
		Network:         ethereum.Sepolia,
		ActivationBlock: 1000,
		Configured:      true,
		ChainHead:       1000,
		ContractActive:  false,
	}

	message, severity := EvaluateReservationsActivation(status)

	if severity != ReservationsActivationWarning {
		t.Errorf("severity = %v, want %v", severity, ReservationsActivationWarning)
	}

	if !strings.Contains(message, "the on-chain activation is still pending") {
		t.Errorf("message %q does not report the pending activation", message)
	}

	// The head is exactly at the activation block: the block has been
	// reached, so the pending-activation warning applies.
	status.ChainHead = 999
	_, severity = EvaluateReservationsActivation(status)
	if severity != ReservationsActivationConsistent {
		t.Errorf(
			"severity below the activation block = %v, want %v",
			severity,
			ReservationsActivationConsistent,
		)
	}
}

// TestEvaluateReservationsActivation_ContractActiveBeforeBlock verifies the
// error raised when the contract already trusts the reservation vault but
// the chain head has not reached the client's activation block: the on-chain
// activation ran before this binary's activation block was reached, which is
// the C-4 ordering violation.
func TestEvaluateReservationsActivation_ContractActiveBeforeBlock(t *testing.T) {
	status := ReservationsActivationStatus{
		Network:         ethereum.Sepolia,
		ActivationBlock: 1000,
		Configured:      true,
		ChainHead:       999,
		ContractActive:  true,
	}

	message, severity := EvaluateReservationsActivation(status)

	if severity != ReservationsActivationError {
		t.Errorf("severity = %v, want %v", severity, ReservationsActivationError)
	}

	if !strings.Contains(message, "chain head [999] is below this client's activation block [1000]") {
		t.Errorf("message %q does not report the head/block mismatch", message)
	}
}

// TestEvaluateReservationsActivation_Consistent verifies that no warning is
// raised when the two agree: the contract is active and the chain head has
// reached the activation block, or the contract is inactive and the chain
// head has not.
func TestEvaluateReservationsActivation_Consistent(t *testing.T) {
	tests := map[string]ReservationsActivationStatus{
		"active and head past the block": {
			Network:         ethereum.Sepolia,
			ActivationBlock: 1000,
			Configured:      true,
			ChainHead:       1500,
			ContractActive:  true,
		},
		"inactive and head before the block": {
			Network:         ethereum.Sepolia,
			ActivationBlock: 1000,
			Configured:      true,
			ChainHead:       500,
			ContractActive:  false,
		},
	}

	for name, status := range tests {
		t.Run(name, func(t *testing.T) {
			message, severity := EvaluateReservationsActivation(status)

			if message != "" {
				t.Errorf("expected no warning, got %q", message)
			}

			if severity != ReservationsActivationConsistent {
				t.Errorf(
					"severity = %v, want %v",
					severity,
					ReservationsActivationConsistent,
				)
			}
		})
	}
}
// TestEvaluateReservationsActivation_HeadExactlyAtBlock verifies that
// the evaluation is consistent when the chain head is exactly at the
// activation block and the contract is already active. The inclusive
// comparison ChainHead < ActivationBlock means head == block is not
// an error; the pending-activation warning only applies when the
// contract is inactive.
func TestEvaluateReservationsActivation_HeadExactlyAtBlock(t *testing.T) {
	tests := map[string]struct {
		status       ReservationsActivationStatus
		wantSeverity ReservationsActivationSeverity
		wantMessage  string // empty means no warning
	}{
		"active and head exactly at the block": {
			status: ReservationsActivationStatus{
				Network:         ethereum.Sepolia,
				ActivationBlock: 1000,
				Configured:      true,
				ChainHead:       1000,
				ContractActive:  true,
			},
			wantSeverity: ReservationsActivationConsistent,
			wantMessage:  "",
		},
		// Negative control: contract inactive at head == B produces the
		// pending-activation warning (the inclusive comparison on the
		// !ContractActive path applies, and head == block triggers it).
		"inactive and head exactly at the block": {
			status: ReservationsActivationStatus{
				Network:         ethereum.Sepolia,
				ActivationBlock: 1000,
				Configured:      true,
				ChainHead:       1000,
				ContractActive:  false,
			},
			wantSeverity: ReservationsActivationWarning,
			wantMessage:  "the chain head [1000] has reached this client's activation block [1000] for network [sepolia] but the Bridge does not trust the reservation vault, so reservations are not live on-chain yet; the on-chain activation is still pending",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, severity := EvaluateReservationsActivation(test.status)

			if message != test.wantMessage {
				t.Errorf("message = %q, want %q", message, test.wantMessage)
			}

			if severity != test.wantSeverity {
				t.Errorf(
					"severity = %v, want %v",
					severity,
					test.wantSeverity,
				)
			}
		})
	}
}

// TestReservationsActivationBlockAndState verifies the activation block and
// the "will this network activate at all" flag for the networks that have
// no entry in reservationsActivationBlocks and for the developer network.
func TestReservationsActivationBlockAndState(t *testing.T) {
	tests := map[string]struct {
		network        ethereum.Network
		wantBlock      uint64
		wantConfigured bool
	}{
		"mainnet has no entry": {
			network:        ethereum.Mainnet,
			wantBlock:      math.MaxUint64,
			wantConfigured: false,
		},
		"sepolia has no entry": {
			network:        ethereum.Sepolia,
			wantBlock:      math.MaxUint64,
			wantConfigured: false,
		},
		"unknown has no entry": {
			network:        ethereum.Unknown,
			wantBlock:      math.MaxUint64,
			wantConfigured: false,
		},
		"developer activates at block 0": {
			network:        ethereum.Developer,
			wantBlock:      0,
			wantConfigured: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			block, configured := ReservationsActivationBlockAndState(test.network)

			if block != test.wantBlock {
				t.Errorf("activation block = %d, want %d", block, test.wantBlock)
			}

			if configured != test.wantConfigured {
				t.Errorf(
					"configured = %v, want %v",
					configured,
					test.wantConfigured,
				)
			}
		})
	}
}

// TestCollectReservationsActivationStatus verifies that the status the
// check compares is read from the chain head and the contract's trust
// state, and that a read failure is reported rather than silently turned
// into a consistent state.
func TestCollectReservationsActivationStatus(t *testing.T) {
	blockCounter := &activationStatusTestChain{head: 4242}
	vaultReader := &activationStatusTestVault{trusted: true}

	status, err := CollectReservationsActivationStatus(
		ethereum.Sepolia,
		blockCounter,
		vaultReader,
	)
	if err != nil {
		t.Fatalf("cannot collect status: [%v]", err)
	}

	if status.ChainHead != 4242 {
		t.Errorf("chain head = %d, want 4242", status.ChainHead)
	}

	if !status.ContractActive {
		t.Error("contract should be reported as active")
	}

	if status.Network != ethereum.Sepolia {
		t.Errorf("network = %v, want %v", status.Network, ethereum.Sepolia)
	}

	if status.Configured {
		t.Error("sepolia has no activation block entry, so it is not configured")
	}

	// A chain-head read failure must be reported: the check could not
	// establish that the two agree.
	_, err = CollectReservationsActivationStatus(
		ethereum.Sepolia,
		&activationStatusTestChain{err: errTest},
		vaultReader,
	)
	if err == nil {
		t.Error("expected an error when the chain head cannot be read")
	}

	// A contract read failure must be reported too.
	_, err = CollectReservationsActivationStatus(
		ethereum.Sepolia,
		blockCounter,
		&activationStatusTestVault{err: errTest},
	)
	if err == nil {
		t.Error("expected an error when the contract state cannot be read")
	}
}

// errTest is the sentinel error the status collection stubs return.
var errTest = errors.New("test error")

// compile-time assertion that the test counter satisfies the interface the
// production code reads the chain head through.
var _ chain.BlockCounter = (*activationStatusTestChain)(nil)
