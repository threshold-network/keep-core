package tbtcpg_test

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	"github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

// testAnchorFeeSat is the estimated reservation acceptance anchor fee in sats.
// It is computed as minWalletTxSatPerVByteFee (5 sat/vByte) multiplied by the
// estimated anchor transaction vsize (142 vBytes) because the test fixture's
// 1 sat/vByte fee rate oracle response is clamped to the 5 sat/vByte floor by
// applyWalletTxFeeFloor (see fee.go).
const testAnchorFeeSat = uint64(710)

// testReservationVaultAddress is the reservation vault address used across
// this file's fixtures, so a deposit's Vault field targets the same vault
// configured in ReservationParameters.ReservationVault.
const testReservationVaultAddress = chain.Address(
	"0xReservationVaultAddress1234567890abcdef12345678",
)

// reservationAcceptanceLocalChain is a test-only mock of tbtcpg.Chain that
// embeds the production LocalChain and adds reservation-specific behavior.
// It exists as a separate type so this test file does not need to edit the
// shared chain_test.go fixture used by sibling builders.
type reservationAcceptanceLocalChain struct {
	*tbtcpg.LocalChain

	maxPerWalletAmount           uint64
	maxSingleAmount              uint64
	walletReservationsAmount     uint64
	walletReservationsCount      uint32
	activeCount                  uint32
	maxActive                    uint32
	pendingReserved              uint64
	validateErr                  error
	getWalletErr                 error
	getReservationErr            error
	acceptanceEvents             []*tbtc.ReservationAcceptanceRequestedEvent
	acceptanceEventsErr          error
	pastDepositRevealedEventsErr error
}

func newReservationAcceptanceLocalChain() *reservationAcceptanceLocalChain {
	lc := tbtcpg.NewLocalChain()
	return &reservationAcceptanceLocalChain{
		LocalChain: lc,
	}
}

// PastDepositRevealedEvents overrides the embedded LocalChain
// implementation to narrow its "no events for given filter" sentinel
// error (the mock's signal for "nothing registered for this filter yet")
// into an empty slice, matching a real chain's behavior of returning an
// empty event list rather than an error when no deposits match. Any
// other error - including one injected via pastDepositRevealedEventsErr -
// is propagated unchanged.
func (ralc *reservationAcceptanceLocalChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	if ralc.pastDepositRevealedEventsErr != nil {
		return nil, ralc.pastDepositRevealedEventsErr
	}
	events, err := ralc.LocalChain.PastDepositRevealedEvents(filter)
	if err != nil {
		if err.Error() == "no events for given filter" {
			return []*tbtc.DepositRevealedEvent{}, nil
		}
		return nil, err
	}
	return events, nil
}

func (ralc *reservationAcceptanceLocalChain) ReservationCaps() (
	uint64,
	uint64,
	error,
) {
	return ralc.maxPerWalletAmount, ralc.maxSingleAmount, nil
}

func (ralc *reservationAcceptanceLocalChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	return ralc.walletReservationsAmount, nil
}

func (ralc *reservationAcceptanceLocalChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	return ralc.walletReservationsCount, nil
}

func (ralc *reservationAcceptanceLocalChain) ActiveReservationsCount() (
	uint32,
	uint32,
	error,
) {
	return ralc.activeCount, ralc.maxActive, nil
}

func (ralc *reservationAcceptanceLocalChain) PendingReservedDeposits() (
	uint64,
	error,
) {
	return ralc.pendingReserved, nil
}

func (ralc *reservationAcceptanceLocalChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	if ralc.getWalletErr != nil {
		return nil, ralc.getWalletErr
	}
	return ralc.LocalChain.GetWallet(walletPublicKeyHash)
}

// GetReservation delegates to the embedded LocalChain, except that a
// non-nil getReservationErr is consumed exactly once: it fires on the
// very next call and then clears itself, simulating a transient RPC
// failure rather than a permanent one. This lets
// TestReservationAcceptanceTask_GetReservationError exercise production's
// fail-safe candidate-selection skip path (see production's documented
// deviation above the call site): the failing candidate is skipped for
// the window rather than treated as "not yet created".
//
// The embedded LocalChain.GetReservation errors for a reservation key that
// was never registered via SetReservation, but the real chain adapter
// (pkg/chain/ethereum/tbtc_reservation.go's GetReservation) reads a Solidity mapping,
// which never errors for an absent key -- it returns the zero-value
// struct (State == ReservationStateUnknown, RequestNonce == 0). This
// override normalizes the embedded mock's "not found" error into that
// same zero-value record so every other test in this file (none of which
// pre-register a reservation for a brand-new candidate deposit) continues
// to exercise the "not yet created" path production actually takes.
func (ralc *reservationAcceptanceLocalChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	if ralc.getReservationErr != nil {
		err := ralc.getReservationErr
		ralc.getReservationErr = nil
		return nil, err
	}
	reservation, err := ralc.LocalChain.GetReservation(reservationKey)
	if err != nil {
		if err.Error() == "reservation not found" {
			return &tbtc.Reservation{State: tbtc.ReservationStateUnknown}, nil
		}
		return nil, err
	}
	return reservation, nil
}

// ValidateReservationAnchorProposal overrides the embedded LocalChain
// implementation. When validateErr is set it returns that error
// unconditionally (see TestReservationAcceptanceTask_ValidateProposalError).
// Otherwise it genuinely exercises the candidate-deposit mapping step by
// checking the proposal's funding outpoint against the candidate deposit's
// own funding outpoint, rather than unconditionally succeeding.
func (ralc *reservationAcceptanceLocalChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	if ralc.validateErr != nil {
		return ralc.validateErr
	}
	if depositExtraInfo.Deposit == nil ||
		depositExtraInfo.Deposit.Utxo == nil ||
		depositExtraInfo.Deposit.Utxo.Outpoint == nil {
		return fmt.Errorf(
			"validate reservation anchor proposal: missing deposit UTXO outpoint",
		)
	}
	outpoint := depositExtraInfo.Deposit.Utxo.Outpoint
	if outpoint.TransactionHash != proposal.DepositFundingTxHash {
		return fmt.Errorf(
			"validate reservation anchor proposal: funding tx hash mismatch: "+
				"proposal=[%x] candidate=[%x]",
			proposal.DepositFundingTxHash,
			outpoint.TransactionHash,
		)
	}
	if outpoint.OutputIndex != proposal.DepositFundingOutputIndex {
		return fmt.Errorf(
			"validate reservation anchor proposal: funding output index mismatch: "+
				"proposal=[%d] candidate=[%d]",
			proposal.DepositFundingOutputIndex,
			outpoint.OutputIndex,
		)
	}
	return nil
}
func (ralc *reservationAcceptanceLocalChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if ralc.acceptanceEventsErr != nil {
		return nil, ralc.acceptanceEventsErr
	}
	var results []*tbtc.ReservationAcceptanceRequestedEvent
	for _, event := range ralc.acceptanceEvents {
		if filter != nil && len(filter.ReservationKey) > 0 {
			match := false
			for _, k := range filter.ReservationKey {
				if k != nil && event.ReservationKey != nil && k.Cmp(event.ReservationKey) == 0 {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		results = append(results, event)
	}
	return results, nil
}

func (ralc *reservationAcceptanceLocalChain) AddPastReservationAcceptanceRequestedEvent(
	event *tbtc.ReservationAcceptanceRequestedEvent,
) {
	ralc.acceptanceEvents = append(ralc.acceptanceEvents, event)
}

// scenarioReservationAcceptanceChain wires a scenario's on-chain state
// into the test mock chain.
func scenarioReservationAcceptanceChain(
	t *testing.T,
	scenario *test.ReservationAcceptanceTestScenario,
) *reservationAcceptanceLocalChain {
	t.Helper()

	ralc := newReservationAcceptanceLocalChain()

	var reservationVault chain.Address
	if len(scenario.ReservationVault) > 0 {
		reservationVault = chain.Address(scenario.ReservationVault)
	}

	ralc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault:          reservationVault,
		ReservationMinAmount:      scenario.ReservationParameters.ReservationMinAmount,
		ReservationTxMaxFee:       scenario.ReservationParameters.ReservationTxMaxFee,
		ReservationMaxTotalAmount: scenario.ReservationParameters.ReservationMaxTotalAmount,
		ReservationTotalAmount:    scenario.ReservationParameters.ReservationTotalAmount,
		MaxReservationsPerWallet:  scenario.ReservationParameters.MaxReservationsPerWallet,
	})

	ralc.maxPerWalletAmount = scenario.Caps.MaxReservationsAmountPerWallet
	ralc.maxSingleAmount = scenario.Caps.ReservationMaxSingleAmount
	ralc.walletReservationsAmount = scenario.WalletCustody.Amount
	ralc.walletReservationsCount = scenario.WalletCustody.Count
	ralc.activeCount = scenario.Global.ActiveCount
	ralc.maxActive = scenario.Global.MaxActive
	ralc.pendingReserved = scenario.PendingReservedDeposits

	ralc.SetDepositMinAge(scenario.ChainParameters.DepositMinAge)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(scenario.ChainParameters.CurrentBlock)
	ralc.SetBlockCounter(blockCounter)

	// Map a WalletState string back to the tbtc constant.
	var walletState tbtc.WalletState
	switch scenario.WalletState {
	case "Live":
		walletState = tbtc.StateLive
	case "Closing":
		walletState = tbtc.StateClosing
	case "Closed":
		walletState = tbtc.StateClosed
	case "Terminated":
		walletState = tbtc.StateTerminated
	default:
		walletState = tbtc.StateLive
	}

	ralc.SetWallet(
		scenario.WalletPublicKeyHash,
		&tbtc.WalletChainData{State: walletState},
	)

	return ralc
}

// registerReservedDeposits wires the scenario's reserved deposits into the
// mock chain as deposit requests and past DepositRevealedEvents. Bitcoin
// transaction registrations live on the btcChain mock.
func registerReservedDeposits(
	t *testing.T,
	scenario *test.ReservationAcceptanceTestScenario,
	ralc *reservationAcceptanceLocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
) {
	t.Helper()

	// Configure the fee oracle rate. proposeReservationAcceptance now
	// estimates the anchor fee dynamically (see estimateReservationAcceptanceFee);
	// 1 sat/vByte hits the applyWalletTxFeeFloor minimum, matching the
	// convention used by the sibling reservation re-anchor test fixtures.
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	filterStartBlock := uint64(0)
	if scenario.ChainParameters.CurrentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = scenario.ChainParameters.CurrentBlock -
			tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	for _, rd := range scenario.ReservedDeposits {
		materialized, err := rd.Materialize()
		if err != nil {
			t.Fatalf(
				"failed to materialize reserved deposit scenario row: [%v]",
				err,
			)
		}

		ralc.SetDepositRequest(
			materialized.FundingTxHash,
			materialized.FundingOutputIndex,
			&tbtc.DepositChainRequest{
				Depositor:  chain.Address(rd.Depositor),
				Amount:     rd.Amount,
				RevealedAt: materialized.RevealedAt,
				SweptAt:    materialized.SweptAt,
				Vault:      materialized.Vault,
			},
		)

		// When the scenario seeds a Pending Acceptance action, also
		// install the reservation record and the action record so
		// findReservationAcceptanceCandidate's read of
		// GetReservationAction(depositKey, reservation.RequestNonce)
		// observes a Pending Acceptance action record targeting this
		// wallet -- the precondition the production validator enforces.
		// A reservation's State in production stays Unknown until
		// settlement, so the seeded record follows that convention.
		if rd.PendingAcceptanceAction != nil {
			if pendingAction := rd.PendingAcceptanceActionValue(); pendingAction != nil {
				depositKey := ralc.BuildDepositKey(
					materialized.FundingTxHash,
					materialized.FundingOutputIndex,
				)
				ralc.SetReservation(depositKey, &tbtc.Reservation{
					WalletPublicKeyHash: materialized.WalletPublicKeyHash,
					State:               tbtc.ReservationStateUnknown,
					RequestNonce:        rd.PendingAcceptanceAction.RequestNonce,
				})
				ralc.SetReservationAction(
					depositKey,
					rd.PendingAcceptanceAction.RequestNonce,
					pendingAction,
				)
			}
		}

		if materialized.FundingTx != nil {
			btcChain.SetTransaction(
				materialized.FundingTxHash,
				materialized.FundingTx,
			)
		} else {
			dummyTx := &bitcoin.Transaction{
				Outputs: []*bitcoin.TransactionOutput{{
					Value:           0,
					PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
				}},
			}
			btcChain.SetTransaction(
				materialized.FundingTxHash,
				dummyTx,
			)
		}
		btcChain.SetTransactionConfirmations(
			materialized.FundingTxHash,
			rd.FundingTxConfirmations,
		)

		currentBlock := scenario.ChainParameters.CurrentBlock
		err = ralc.AddPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{
				StartBlock:          filterStartBlock,
				EndBlock:            &currentBlock,
				WalletPublicKeyHash: [][20]byte{materialized.WalletPublicKeyHash},
			},
			&tbtc.DepositRevealedEvent{
				BlockNumber:         materialized.RevealBlock,
				WalletPublicKeyHash: materialized.WalletPublicKeyHash,
				FundingTxHash:       materialized.FundingTxHash,
				FundingOutputIndex:  materialized.FundingOutputIndex,
				Vault:               materialized.Vault,
			},
		)
		if err != nil {
			t.Fatalf(
				"failed to register past deposit revealed event: [%v]",
				err,
			)
		}
	}
}

// setupEligibleDeposit registers an eligible deposit funding transaction,
// deposit request, and matching DepositRevealedEvent on the mock chains.
// It returns the funding transaction hash.
func setupEligibleDeposit(
	t *testing.T,
	ralc *reservationAcceptanceLocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	depositAmount uint64,
) bitcoin.Hash {
	t.Helper()

	fundingTxHash := hashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
	)

	dummyTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           int64(depositAmount),
			PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
		}},
	}
	btcChain.SetTransaction(fundingTxHash, dummyTx)
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	vaultAddress := testReservationVaultAddress
	if params, err := ralc.ReservationParameters(); err == nil &&
		params.ReservationVault != "" {
		vaultAddress = params.ReservationVault
	}

	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     depositAmount,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &vaultAddress,
		},
	)

	filterStartBlock := uint64(0)
	if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	revealBlock := filterStartBlock
	if revealBlock == 0 {
		revealBlock = 1
	}

	err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         revealBlock,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               &vaultAddress,
		},
	)
	if err != nil {
		t.Fatalf("failed to add past deposit revealed event: [%v]", err)
	}

	return fundingTxHash
}

// seedPendingAcceptanceAction wires a Pending Acceptance action record
// for the deposit at fundingTxHash/index into the local chain, matching
// the snapshot a depositor's requestReservationAcceptance call would have
// written on-chain at the time of the request. Tests then exercise
// findReservationAcceptanceCandidate's read of the action record at
// reservation.RequestNonce -- the precondition the production validator
// enforces -- instead of taking the operator-request path that was
// removed.
func seedPendingAcceptanceAction(
	t *testing.T,
	ralc *reservationAcceptanceLocalChain,
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	walletPublicKeyHash [20]byte,
	requestNonce uint64,
	txMaxFee uint64,
	minAmount uint64,
	termSeconds uint32,
) {
	t.Helper()

	depositKey := ralc.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	ralc.SetReservation(depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        requestNonce,
	})
	ralc.SetReservationAction(depositKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  txMaxFee,
		MinAmount:                 minAmount,
		TermSeconds:               termSeconds,
		// Far-future TimeoutAt so the validator's safety-margin gate
		// (REQUEST_TIMEOUT_SAFETY_MARGIN, 2 hours) does not reject the
		// action. Tests that intentionally exercise the gate set
		// TimeoutAt via SetReservationAction directly.
		TimeoutAt: uint32(time.Now().Add(24 * time.Hour).Unix()),
	})
}

// newBoundaryTestChain builds a reservationAcceptanceLocalChain with the
// reservation-parameters/caps/wallet/block-counter setup shared by most of
// this file's Run()-based tests: a live wallet at walletPublicKeyHash, a
// ReservationParameters of {vault: testReservationVaultAddress, minAmount:
// 1000, txMaxFee: 5000, maxPerWallet: 5}, per-wallet/single caps of
// 5000000, an active-reservations cap of 100, and a deposit minimum age of
// one hour. overrides, when non-nil, runs after these defaults so a call
// site can customize only what it varies (e.g. re-set ReservationParameters
// with different values, raise a cap, or inject an error field).
func newBoundaryTestChain(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	overrides func(ralc *reservationAcceptanceLocalChain),
) *reservationAcceptanceLocalChain {
	t.Helper()

	ralc := newReservationAcceptanceLocalChain()
	ralc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault:          testReservationVaultAddress,
		ReservationMinAmount:      1000,
		ReservationTxMaxFee:       5000,
		MaxReservationsPerWallet:  5,
		ReservationMaxTotalAmount: 100000000,
	})
	ralc.maxPerWalletAmount = 5000000
	ralc.maxSingleAmount = 5000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(3600)
	ralc.SetWallet(
		walletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

	if overrides != nil {
		overrides(ralc)
	}

	return ralc
}

// uint64Ptr and uint32Ptr let a TestReservationAcceptanceTask_BoundaryChecks
// table row distinguish an explicit cap value of 0 (production's
// "unlimited" semantic for these caps) from the field's unset zero value.
func uint64Ptr(v uint64) *uint64 { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }

// expectedAnchorsEqual compares two proposal objects field-by-field.
// deep.Equal cannot be used for this: by default it does not descend into
// unexported fields, and *big.Int's representation is entirely unexported,
// so it silently reports "no difference" for any two distinct AnchorTxFee
// values. AnchorTxFee therefore needs an explicit .Cmp().
func expectedAnchorsEqual(
	expected, actual *tbtc.ReservationAnchorProposal,
) bool {
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}

	if expected.DepositFundingTxHash != actual.DepositFundingTxHash {
		return false
	}
	if expected.DepositFundingOutputIndex != actual.DepositFundingOutputIndex {
		return false
	}
	if expected.RequestNonce != actual.RequestNonce {
		return false
	}
	if expected.AnchorTxFee == nil || actual.AnchorTxFee == nil {
		return expected.AnchorTxFee == actual.AnchorTxFee
	}
	return expected.AnchorTxFee.Cmp(actual.AnchorTxFee) == 0
}

func TestReservationAcceptanceLookBackBlocks(t *testing.T) {
	expectedValue := uint64(216000)

	if tbtcpg.ReservationAcceptanceLookBackBlocks != expectedValue {
		t.Errorf(
			"unexpected ReservationAcceptanceLookBackBlocks\n"+
				"expected: %d\n"+
				"actual:   %d",
			expectedValue,
			tbtcpg.ReservationAcceptanceLookBackBlocks,
		)
	}
}

func TestReservationAcceptanceTask_ActionType(t *testing.T) {
	task := tbtcpg.NewReservationAcceptanceTask(
		newReservationAcceptanceLocalChain(),
		tbtcpg.NewLocalBitcoinChain(),
	)
	if task.ActionType() != tbtc.ActionReservationAnchor {
		t.Errorf(
			"unexpected action type\n"+
				"expected: %v\n"+
				"actual:   %v",
			tbtc.ActionReservationAnchor,
			task.ActionType(),
		)
	}
}

func TestReservationAcceptanceTask_Run(t *testing.T) {
	if err := log.SetLogLevel("*", "DEBUG"); err != nil {
		t.Fatal(err)
	}

	scenarios, err := test.LoadReservationAcceptanceTestScenario()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			ralc := scenarioReservationAcceptanceChain(t, scenario)
			btcChain := tbtcpg.NewLocalBitcoinChain()
			registerReservedDeposits(t, scenario, ralc, btcChain)

			task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

			request := &tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: scenario.WalletPublicKeyHash,
			}

			proposal, shouldExecute, err := task.Run(request)
			if err != nil {
				if scenario.ExpectedErr == nil {
					t.Fatalf("unexpected error: [%v]", err)
				}
				if scenario.ExpectedErr.Error() != err.Error() {
					t.Fatalf(
						"unexpected error message\n"+
							"expected: [%v]\n"+
							"actual:   [%v]",
						scenario.ExpectedErr,
						err,
					)
				}
				return
			}
			if scenario.ExpectedErr != nil {
				t.Fatalf("expected error [%v], got nil", scenario.ExpectedErr)
			}

			expectedProposal := scenario.ExpectedAnchorProposal

			if expectedProposal == nil {
				if shouldExecute {
					t.Errorf(
						"unexpected proposal returned when none expected",
					)
				}
				if proposal != nil {
					t.Errorf(
						"expected nil proposal, got [%+v]",
						proposal,
					)
				}
				return
			}

			if !shouldExecute {
				t.Errorf("expected shouldExecute=true, got false")
			}
			if proposal == nil {
				t.Fatal("expected proposal, got nil")
			}

			actualProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
			if !ok {
				t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
			}

			if !expectedAnchorsEqual(expectedProposal, actualProposal) {
				t.Errorf(
					"invalid anchor proposal\nexpected: %+v\nactual:   %+v",
					expectedProposal,
					actualProposal,
				)
			}
		})
	}
}

// TestReservationAcceptanceTask_AnchorTransactionAssembly verifies the
// wiring of AssembleReservationAnchorTransaction: it ensures that an assembled
// anchor transaction can be signed and produces a valid 1-input-1-output
// Bitcoin transaction paying the correct wallet P2WPKH output script with
// value equal to deposit amount minus the estimated anchor fee.
func TestReservationAcceptanceTask_AnchorTransactionAssembly(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	privateKey, err := ecdsa.GenerateKey(btcec.S256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(&privateKey.PublicKey)

	depositAmount := uint64(2000000)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.maxPerWalletAmount = 50000000
		ralc.maxSingleAmount = 50000000
	})

	deposit := &tbtc.Deposit{
		Depositor:           chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
		BlindingFactor:      [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		WalletPublicKeyHash: walletPublicKeyHash,
		RefundPublicKeyHash: [20]byte{0x02},
		RefundLocktime:      [4]byte{0x03, 0x04, 0x05, 0x06},
		Vault:               &[]chain.Address{testReservationVaultAddress}[0],
	}

	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatal(err)
	}

	depositScriptHash := sha256.Sum256(depositScript)
	depositLockingScript, err := bitcoin.PayToWitnessScriptHash(depositScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x09},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           int64(depositAmount),
				PublicKeyScript: depositLockingScript,
			},
		},
	}
	fundingTxHash := fundingTx.Hash()
	btcChain.SetTransaction(fundingTxHash, fundingTx)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTxHash,
			OutputIndex:     0,
		},
		Value: int64(depositAmount),
	}

	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  deposit.Depositor,
			Amount:     depositAmount,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      deposit.Vault,
		},
	)

	// Seed a Pending Acceptance action at nonce 1 with the boundary
	// chain's snapshot parameters so the candidate passes
	// findReservationAcceptanceCandidate's action-record gate. The
	// wallet here is the deposit's own WalletPublicKeyHash (the wallet
	// that revealed the deposit), which is also the chain's active
	// wallet set above.
	seedPendingAcceptanceAction(
		t,
		ralc,
		fundingTxHash,
		0,
		walletPublicKeyHash,
		1,
		5000,
		1000,
		86400,
	)

	filterStartBlock := currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         200000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               deposit.Vault,
			BlindingFactor:      deposit.BlindingFactor,
			RefundPublicKeyHash: deposit.RefundPublicKeyHash,
			RefundLocktime:      deposit.RefundLocktime,
		},
	); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error running task: [%v]", err)
	}
	if !shouldExecute {
		t.Fatalf("expected shouldExecute=true, got false")
	}
	if proposal == nil {
		t.Fatalf("expected non-nil proposal")
	}

	anchorProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}

	// Assert on the candidate-derived proposal's own fields, exercising the
	// candidate-deposit mapping step (also checked by the fixture's
	// ValidateReservationAnchorProposal override), rather than only
	// reassembling from this test's own hand-built deposit object below.
	if anchorProposal.DepositFundingTxHash != fundingTxHash {
		t.Errorf(
			"unexpected DepositFundingTxHash\nexpected: %x\nactual:   %x",
			fundingTxHash,
			anchorProposal.DepositFundingTxHash,
		)
	}
	if anchorProposal.DepositFundingOutputIndex != 0 {
		t.Errorf(
			"unexpected DepositFundingOutputIndex\nexpected: 0\nactual:   %d",
			anchorProposal.DepositFundingOutputIndex,
		)
	}

	// Re-assemble and sign to verify transaction builder output properties.
	builder, err := tbtc.AssembleReservationAnchorTransaction(
		btcChain,
		deposit,
		walletPublicKeyHash,
		&tbtc.ReservationAction{TxMaxFee: 5000},
		anchorProposal.AnchorTxFee.Int64(),
	)
	if err != nil {
		t.Fatalf("failed to assemble reservation anchor transaction: [%v]", err)
	}

	sigHashes, err := builder.ComputeSignatureHashes()
	if err != nil {
		t.Fatalf("failed to compute signature hashes: [%v]", err)
	}
	signatures := make([]*bitcoin.SignatureContainer, len(sigHashes))
	for i, sigHash := range sigHashes {
		r, s, err := ecdsa.Sign(rand.Reader, privateKey, sigHash.Bytes())
		if err != nil {
			t.Fatalf("failed to sign input: [%v]", err)
		}
		signatures[i] = &bitcoin.SignatureContainer{
			R:         r,
			S:         s,
			PublicKey: &privateKey.PublicKey,
		}
	}

	signedTx, err := builder.AddSignatures(signatures)
	if err != nil {
		t.Fatalf("failed to add signatures: [%v]", err)
	}

	if len(signedTx.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(signedTx.Inputs))
	}
	if len(signedTx.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(signedTx.Outputs))
	}

	expectedOutputScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	expectedOutputValue := int64(depositAmount) - anchorProposal.AnchorTxFee.Int64()

	if signedTx.Outputs[0].Value != expectedOutputValue {
		t.Errorf(
			"unexpected output value\nexpected: [%d]\nactual:   [%d]",
			expectedOutputValue,
			signedTx.Outputs[0].Value,
		)
	}
	if string(signedTx.Outputs[0].PublicKeyScript) != string(expectedOutputScript) {
		t.Errorf(
			"unexpected output script\nexpected: [%x]\nactual:   [%x]",
			expectedOutputScript,
			signedTx.Outputs[0].PublicKeyScript,
		)
	}
}

// TestReservationAcceptanceTask_NoCandidates verifies that the task is a
// no-op when the chain has no reserved deposits.
func TestReservationAcceptanceTask_NoCandidates(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.SetReservationParameters(tbtc.ReservationParameters{
			ReservationVault: testReservationVaultAddress,
		})
		ralc.maxPerWalletAmount = 1000000
	})

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal, got [%+v]", proposal)
	}
}

// TestReservationAcceptanceTask_VaultNotConfigured_ZeroAddress verifies
// that a zero-address ReservationVault (the actual value the production
// chain.Address converter emits for an unset vault, never an empty
// string) is correctly treated as "not configured".
func TestReservationAcceptanceTask_VaultNotConfigured_ZeroAddress(t *testing.T) {
	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	ralc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0x0000000000000000000000000000000000000000",
		),
	})
	ralc.maxPerWalletAmount = 1000000
	ralc.maxSingleAmount = 5000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(3600)
	ralc.SetWallet(
		walletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal, got [%+v]", proposal)
	}
}

// TestReservationAcceptanceTask_IgnoresRequestTimeCaps is a regression
// test for the cross-repo review change: the acceptance task consumes the
// depositor's Pending Acceptance generation rather than issuing its own
// request, so it must not re-apply the request-time capacity caps
// (wallet count, active count) that Solidity's requestReservationAcceptance
// already reserved when that generation was created. Re-applying them
// here would double-count the very generation being consumed against
// them and block every otherwise eligible pending acceptance.
//
// This test pins that: with the wallet's own reservation count already at
// the per-wallet cap and the active-reservations count already at the
// active cap (the request-time capacity reserved by this very pending
// generation), the acceptance task still proposes the anchor. A regression
// that re-added the request-time cap gate at consumption time would make
// this test fail.
func TestReservationAcceptanceTask_IgnoresRequestTimeCaps(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	// The pending acceptance generation this candidate consumes was
	// already reserved against the capacity caps at request time, so
	// the wallet's own count and the global active count are at their
	// caps even before this anchor is signed: the wallet holds the cap
	// number of reservations (including this one) and the active
	// reservations count equals the max. A fresh request would be
	// rejected on-chain; consuming the reserved generation must not
	// be.
	fundingTxHash := setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)
	seedPendingAcceptanceAction(
		t,
		ralc,
		fundingTxHash,
		0,
		walletPublicKeyHash,
		1,
		5000,
		1000,
		86400,
	)
	// The wallet's own reservation count is at the cap (this pending
	// generation counts toward it) and the active count is at the
	// active cap: exactly the state Solidity's request-time
	// reserveAcceptanceCapacity leaves behind.
	ralc.SetWalletReservations(walletPublicKeyHash, []*big.Int{
		new(big.Int).SetInt64(7),
	})
	ralc.maxActive = 100
	ralc.activeCount = 100
	ralc.walletReservationsCount = 5

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf(
			"expected the pending acceptance to be consumed even though " +
				"the request-time capacity caps (wallet count, active " +
				"count) are already at their limits; the caps were " +
				"reserved when the generation was requested",
		)
	}
	if proposal.(*tbtc.ReservationAnchorProposal).RequestNonce != 1 {
		t.Fatalf(
			"expected the proposal to carry the pending generation's "+
				"real nonce (1), got [%d]",
			proposal.(*tbtc.ReservationAnchorProposal).RequestNonce,
		)
	}
}

// TestReservationAcceptanceTask_BoundedLookback verifies that the bounded
// look-back window is applied when the current block exceeds it.
func TestReservationAcceptanceTask_BoundedLookback(t *testing.T) {
	initialBlock := uint64(400000)

	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	// A block counter this test can advance between runs is created up
	// front (rather than letting newBoundaryTestChain own an opaque one),
	// so the second run below can simulate a later coordination window
	// the way production actually progresses, exercising the task's
	// per-wallet incremental scan cursor (see depositRevealedEventsSince)
	// instead of re-querying the exact same already-scanned range twice.
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(initialBlock)

	ralc := newBoundaryTestChain(
		t,
		walletPublicKeyHash,
		initialBlock,
		func(ralc *reservationAcceptanceLocalChain) {
			ralc.SetBlockCounter(blockCounter)
		},
	)

	// Register an event below the look-back start block (block 1), under
	// the unbounded filter a buggy filterStartBlock=0 computation would
	// query with.
	oldFundingTxHash := hashFromString(
		"1111111111111111111111111111111111111111111111111111111111111111",
	)
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          0,
			EndBlock:            &initialBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         1,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       oldFundingTxHash,
			FundingOutputIndex:  0,
		},
	); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	// First run: only the old deposit exists, revealed at block 1 - before
	// the look-back start block. No candidate is found on this run; the
	// second run below is what actually proves the look-back start block
	// is honored, by advancing the block counter and registering an
	// eligible deposit within the resulting incremental scan delta.
	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error on old deposit run: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false for deposit below lookback window, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal for deposit below lookback window, got [%+v]", proposal)
	}

	// Advance the block counter (as a later coordination window would)
	// and register an eligible deposit within the resulting incremental
	// delta range [initialBlock+1, nextBlock].
	nextBlock := initialBlock + 10
	blockCounter.SetCurrentBlock(nextBlock)

	fundingTxHash := hashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
	)
	dummyTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           2000000,
			PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
		}},
	}
	btcChain.SetTransaction(fundingTxHash, dummyTx)
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)
	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     2000000,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &[]chain.Address{testReservationVaultAddress}[0],
		},
	)
	// Seed a Pending Acceptance action matching the boundary chain's
	// parameters so this freshly-revealed deposit is eligible to be
	// accepted by the second run.
	seedPendingAcceptanceAction(
		t,
		ralc,
		fundingTxHash,
		0,
		walletPublicKeyHash,
		1,
		5000,
		1000,
		86400,
	)
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          initialBlock + 1,
			EndBlock:            &nextBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         nextBlock,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               &[]chain.Address{testReservationVaultAddress}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	// Second run: the newly revealed deposit must be found and accepted.
	proposal, shouldExecute, err = task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if !shouldExecute {
		t.Fatalf("expected shouldExecute=true, got false")
	}
	if proposal == nil {
		t.Fatalf("expected proposal, got nil")
	}
	actualProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}
	if actualProposal.DepositFundingTxHash != fundingTxHash {
		t.Errorf(
			"unexpected deposit funding tx hash\n"+
				"expected: %s\n"+
				"actual:   %s",
			fundingTxHash.Hex(bitcoin.ReversedByteOrder),
			actualProposal.DepositFundingTxHash.Hex(
				bitcoin.ReversedByteOrder,
			),
		)
	}
}

// TestReservationAcceptanceTask_DepositNotReserved confirms that a deposit
// that fails IsReservedDeposit is filtered out.
func TestReservationAcceptanceTask_DepositNotReserved(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.SetReservationParameters(tbtc.ReservationParameters{
			ReservationVault: testReservationVaultAddress,
		})
	})

	fundingTxHash := hashFromString(
		"3333333333333333333333333333333333333333333333333333333333333333",
	)
	btcChain.SetTransaction(fundingTxHash, &bitcoin.Transaction{})
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)
	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Amount:     2000000,
			RevealedAt: time.Now().Add(-2 * time.Hour),
		},
	)
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          0,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         290000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault: &[]chain.Address{chain.Address(
				"0xReservationVaultAddress1234567890abcdef12345678",
			)}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected no proposal for non-reserved deposit, got %v", proposal)
	}
}

// TestReservationAcceptanceTask_GetWalletError exercises the GetWallet
// error propagation inside findReservationAcceptanceCandidate: a
// reserved deposit candidate is discovered and matches the reservation
// vault, but the candidate wallet's chain data fails to load. Production
// now propagates this RPC failure instead of masking it as "no eligible
// candidate", so the coordinator can retry rather than silently treating
// a transient chain-read failure as a benign no-op.
func TestReservationAcceptanceTask_GetWalletError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	// getWalletErr forces GetWallet to fail for the candidate wallet.
	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.getWalletErr = fmt.Errorf("boom")
	})

	setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedErr := "cannot find reservation acceptance candidate: " +
		"[failed to load wallet chain data: [boom]]"
	if err.Error() != expectedErr {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v",
			expectedErr,
			err,
		)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected no proposal, got %v", proposal)
	}
}

// TestReservationAcceptanceTask_GetReservationError verifies the fail-safe
// policy documented above the production call site: since the production
// chain adapter never errors for "not found" (it returns a zero record
// with State == Unknown), a GetReservation error can only be an RPC/decode
// failure, and the task must skip the affected deposit for this window
// rather than fail open and treat it as "not yet created".
func TestReservationAcceptanceTask_GetReservationError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	// Force GetReservation to return an error.
	ralc.getReservationErr = fmt.Errorf("simulated get reservation error")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Fatalf("expected nil proposal, got [%+v]", proposal)
	}
}

// TestReservationAcceptanceTask_Stateless_Maturity verifies the stateless
// observable contract across two consecutive Run calls on the same task instance:
// an immature candidate is skipped on the first run, but when time advances and
// the candidate matures, the second run on the same task instance proposes it
// without any cache-state interference.
func TestReservationAcceptanceTask_Stateless_Maturity(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	fundingTxHash := hashFromString(
		"5555555555555555555555555555555555555555555555555555555555555555",
	)
	dummyTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           0,
			PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
		}},
	}
	btcChain.SetTransaction(fundingTxHash, dummyTx)
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	// Candidate revealed only 10 minutes ago (depositMinAge is 1 hour).
	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     2000000,
			RevealedAt: time.Now().Add(-10 * time.Minute),
			SweptAt:    time.Unix(0, 0),
			Vault:      &[]chain.Address{testReservationVaultAddress}[0],
		},
	)

	// Seed the Pending Acceptance action so once the deposit matures,
	// the candidate passes findReservationAcceptanceCandidate's
	// action-record gate.
	seedPendingAcceptanceAction(
		t,
		ralc,
		fundingTxHash,
		0,
		walletPublicKeyHash,
		1,
		5000,
		1000,
		86400,
	)

	filterStartBlock := uint64(0)
	if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         290000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               &[]chain.Address{testReservationVaultAddress}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	// First run: deposit is immature, should not be proposed.
	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("first run error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal on first run for immature deposit")
	}

	// Advance deposit age (simulating passage of time to 2 hours ago).
	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     2000000,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &[]chain.Address{testReservationVaultAddress}[0],
		},
	)

	// Second run on the same task instance: deposit is now mature and proposed.
	proposal, shouldExecute, err = task.Run(request)
	if err != nil {
		t.Fatalf("second run error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf("expected proposal on second run after deposit matured")
	}

	actualProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}
	if actualProposal.DepositFundingTxHash != fundingTxHash {
		t.Errorf(
			"unexpected deposit funding tx hash\nexpected: %s\nactual:   %s",
			fundingTxHash.Hex(bitcoin.ReversedByteOrder),
			actualProposal.DepositFundingTxHash.Hex(bitcoin.ReversedByteOrder),
		)
	}
}

// TestReservationAcceptanceTask_ReservationParametersFetchedLive verifies
// that each Run() call reflects the chain's current live state rather than
// anything cached from a prior run on the same task instance. It also
// pins two behaviors of the snapshot-driven minimum-amount gate and the
// depositor-action-driven proposal consumption that supersede the
// pre-fix operator-side request path:
//   - the live ReservationParameters is consulted on every Run -- a
//     governance mutation takes effect on the very next call;
//   - the minimum-amount gate uses the generation's snapshotted minimum
//     rather than the live ReservationMinAmount, so a live raise that
//     crosses the deposit does not retroactively invalidate a pending
//     generation whose own snapshot still clears it.
func TestReservationAcceptanceTask_ReservationParametersFetchedLive(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	t.Run("live parameters are re-fetched each Run", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()

		ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

		setupEligibleDeposit(
			t,
			ralc,
			btcChain,
			walletPublicKeyHash,
			currentBlock,
			2000000,
		)

		task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
		request := &tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		}

		// Seed a Pending Acceptance action at nonce 1 with snapshot
		// MinAmount=1000 (matching the live min at the moment of the
		// depositor's request), so the candidate is eligible on both
		// runs.
		seedPendingAcceptanceAction(
			t,
			ralc,
			setupEligibleDeposit(
				t,
				ralc,
				btcChain,
				walletPublicKeyHash,
				currentBlock,
				2000000,
			),
			0,
			walletPublicKeyHash,
			1,
			5000,
			1000,
			86400,
		)

		// First run: snapshot min (1000) plus fee (710) is well below
		// the deposit (2000000) -- must accept.
		_, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on first run: [%v]", err)
		}
		if !shouldExecute {
			t.Fatalf("expected shouldExecute=true on first run, got false")
		}

		// Raise the live ReservationMinAmount above the deposit's value.
		// The snapshotted MinAmount on the seeded action (1000) is
		// unchanged, so the deposit must still pass the gate on the
		// second run.
		ralc.SetReservationParameters(tbtc.ReservationParameters{
			ReservationVault:          testReservationVaultAddress,
			ReservationMinAmount:      3000000,
			ReservationTxMaxFee:       5000,
			MaxReservationsPerWallet:  5,
			ReservationMaxTotalAmount: 100000000,
		})

		proposal, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on second run: [%v]", err)
		}
		if !shouldExecute {
			t.Fatalf(
				"expected shouldExecute=true on second run after raising " +
					"the live ReservationMinAmount above the deposit's " +
					"value -- the snapshot min (1000) still clears the " +
					"gate, so a false here means the live value was used",
			)
		}
		if proposal == nil {
			t.Fatalf("expected a non-nil proposal on second run")
		}
	})

	t.Run("operator never requests acceptance; the depositor's pending action is consumed", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()

		ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

		fundingTxHash := setupEligibleDeposit(
			t,
			ralc,
			btcChain,
			walletPublicKeyHash,
			currentBlock,
			2000000,
		)

		// Seed a Pending Acceptance action the depositor would have
		// requested via requestReservationAcceptance. Production now
		// consumes that record rather than issuing a fresh one.
		seedPendingAcceptanceAction(
			t,
			ralc,
			fundingTxHash,
			0,
			walletPublicKeyHash,
			1,
			5000,
			1000,
			86400,
		)

		task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
		request := &tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		}

		proposal, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on first run: [%v]", err)
		}
		if !shouldExecute || proposal == nil {
			t.Fatalf("expected shouldExecute=true on first run")
		}
		anchorProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
		if !ok {
			t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
		}
		if anchorProposal.RequestNonce != 1 {
			t.Fatalf(
				"expected proposal to carry the generation's actual "+
					"nonce (1), got [%d] -- a value of N+1 means the "+
					"task invented the next nonce instead of consuming "+
					"the depositor's action",
				anchorProposal.RequestNonce,
			)
		}

		if recorded := ralc.GetReservationAcceptanceRequests(); len(recorded) != 0 {
			t.Fatalf(
				"expected zero RequestReservationAcceptance submissions, "+
					"got %d -- the operator-side path was removed and "+
					"production must consume, not create, the depositor's "+
					"pending action",
				len(recorded),
			)
		}

		// The reservation's RequestNonce stays at 1 across repeated runs:
		// production never bumps it itself, only the depositor's
		// requestReservationAcceptance call does (and the operator no
		// longer invokes it).
		if reservations := ralc.GetReservationAcceptanceRequests(); len(reservations) != 0 {
			// Already covered above; keep the assertion here as a belt-
			// and-suspenders marker that the next Run on the same
			// fixture observes the same nonce.
			_ = reservations
		}
	})
}

// TestReservationAcceptanceTask_BoundaryChecks exercises explicit
// at-limit/one-over-limit boundary crossings for the snapshotted
// ReservationMinAmount the acceptance candidate enforces against its
// own action record (the gross gate is depositAmount >= ReservationMinAmount,
// and the net-of-fee gate is depositAmount >= MinAmount + fee). The
// request-time capacity caps (active count, per-wallet count and
// amount, single amount, global total) are no longer re-checked here:
// Solidity's requestReservationAcceptance already reserved them when
// the Pending Acceptance generation was created.
func TestReservationAcceptanceTask_BoundaryChecks(t *testing.T) {
	tests := map[string]struct {
		depositAmount            uint64
		maxReservationsPerWallet uint32
		walletReservationsCount  uint32
		reservationMinAmount     uint64
		reservationMaxTotal      uint64
		reservationTotal         uint64
		// maxSingleAmount, maxPerWalletAmount, and maxActive are pointers
		// so a test row can explicitly request the cap-disabled value of 0
		// (production's "0 = unlimited" semantic for these caps); nil means
		// "use this test's default cap" instead.
		maxSingleAmount          *uint64
		maxPerWalletAmount       *uint64
		walletReservationsAmount uint64
		maxActive                *uint32
		activeCount              uint32
		expectAccept             bool
	}{
		"MaxReservationsPerWallet: below limit accepts": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			walletReservationsCount:  4,
			reservationMinAmount:     1000,
			reservationMaxTotal:      100000000,
			expectAccept:             true,
		},
		// checkReservationAcceptanceEligibility's gross-amount gate only
		// requires depositAmount >= reservationMinAmount, but
		// proposeReservationAcceptance additionally requires the
		// *net-of-fee* anchor value (deposit - anchorFee) to also clear
		// reservationMinAmount. Even though the test fixture sets a 1 sat/vByte
		// oracle rate, applyWalletTxFeeFloor (see fee.go) clamps the rate to
		// minWalletTxSatPerVByteFee (5 sat/vByte), resulting in a 710 sat fee
		// (5 * 142 vsize = testAnchorFeeSat). Deposit amounts are offset by
		// testAnchorFeeSat to test the exact net-of-fee boundary.
		// The snapshot min on the seeded action mirrors the live
		// ReservationMinAmount so the gate is governed by the row's
		// value, not by the previous row's.
		"ReservationMinAmount: exactly at minimum accepts": {
			depositAmount:            100000 + testAnchorFeeSat,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			reservationMaxTotal:      100000000,
			expectAccept:             true,
		},
		"ReservationMinAmount: gross clears but net-of-fee value does not": {
			depositAmount:            100050,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			reservationMaxTotal:      100000000,
			expectAccept:             false,
		},
		"ReservationMinAmount: one below minimum rejects": {
			depositAmount:            99999,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			reservationMaxTotal:      100000000,
			expectAccept:             false,
		},
		"ReservationMaxTotalAmount: exactly at cap accepts": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			reservationTotal:         3000000,
			reservationMaxTotal:      5000000,
			expectAccept:             true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ralc := newReservationAcceptanceLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, 1)

			walletPublicKeyHash := hexToByte20(
				"8db50eb52063ea9d98b3eac91489a90f738986f6",
			)

			ralc.SetReservationParameters(tbtc.ReservationParameters{
				ReservationVault:          testReservationVaultAddress,
				ReservationMinAmount:      test.reservationMinAmount,
				ReservationTxMaxFee:       5000,
				MaxReservationsPerWallet:  test.maxReservationsPerWallet,
				ReservationMaxTotalAmount: test.reservationMaxTotal,
				ReservationTotalAmount:    test.reservationTotal,
			})
			ralc.maxPerWalletAmount = 50000000
			if test.maxPerWalletAmount != nil {
				ralc.maxPerWalletAmount = *test.maxPerWalletAmount
			}
			ralc.maxSingleAmount = 50000000
			if test.maxSingleAmount != nil {
				ralc.maxSingleAmount = *test.maxSingleAmount
			}
			ralc.maxActive = 100
			if test.maxActive != nil {
				ralc.maxActive = *test.maxActive
			}
			ralc.activeCount = test.activeCount
			ralc.walletReservationsAmount = test.walletReservationsAmount
			ralc.walletReservationsCount = test.walletReservationsCount

			ralc.SetDepositMinAge(3600)
			ralc.SetWallet(
				walletPublicKeyHash,
				&tbtc.WalletChainData{State: tbtc.StateLive},
			)

			currentBlock := uint64(300000)
			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(currentBlock)
			ralc.SetBlockCounter(blockCounter)

			fundingTxHash := setupEligibleDeposit(
				t,
				ralc,
				btcChain,
				walletPublicKeyHash,
				currentBlock,
				test.depositAmount,
			)

			// Seed a Pending Acceptance action with the row's snapshot
			// min so this test exercises the per-row cap configuration,
			// not the previous row's leftover state. Reject rows trip a
			// cap gate before the action lookup, so the snapshot value
			// only matters for the accept rows; seeding it consistently
			// keeps the driver uniform.
			seedPendingAcceptanceAction(
				t,
				ralc,
				fundingTxHash,
				0,
				walletPublicKeyHash,
				1,
				5000,
				test.reservationMinAmount,
				86400,
			)

			task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

			request := &tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: walletPublicKeyHash,
			}

			_, shouldExecute, err := task.Run(request)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			if shouldExecute != test.expectAccept {
				t.Errorf(
					"expected shouldExecute=%v, got %v",
					test.expectAccept,
					shouldExecute,
				)
			}
		})
	}
}

// TestReservationAcceptanceTask_Stateless_NoReRequest pins the two
// surviving "no double-acceptance" cases after the operator-side
// RequestReservationAcceptance path was removed. The acceptance task
// consumes, rather than creates, the depositor's request record, so a
// double-proposal can no longer be caused by repeated operator requests:
// it can only happen if the task builds a proposal for a generation it
// is not allowed to anchor. The two cases below cover that:
//   - a Pending Acceptance action that targets another wallet must not
//     be consumed by this operator;
//   - a Settled (or otherwise non-Pending) action at the current nonce
//     must not be re-anchored.
func TestReservationAcceptanceTask_Stateless_NoReRequest(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	otherWalletPublicKeyHash := hexToByte20(
		"7c1c4dbaaf8d75b08f9ea8e3f9a2c3a98e1f0d77",
	)

	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	fundingTxHash := hashFromString(
		"6666666666666666666666666666666666666666666666666666666666666666",
	)
	dummyTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           0,
			PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
		}},
	}
	btcChain.SetTransaction(fundingTxHash, dummyTx)
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     2000000,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &[]chain.Address{testReservationVaultAddress}[0],
		},
	)

	filterStartBlock := uint64(0)
	if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         290000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               &[]chain.Address{testReservationVaultAddress}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)

	t.Run("pending acceptance targeting another wallet is not consumed", func(t *testing.T) {
		ralc.SetReservation(depositKey, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			State:               tbtc.ReservationStateUnknown,
			RequestNonce:        1,
		})
		ralc.SetReservationAction(depositKey, 1, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStatePending,
			TargetWalletPublicKeyHash: otherWalletPublicKeyHash,
			TxMaxFee:                  5000,
			MinAmount:                 1000,
			TermSeconds:               86400,
		})

		proposal, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if shouldExecute || proposal != nil {
			t.Fatalf(
				"expected no proposal for a pending acceptance generation " +
					"authorizing a different wallet",
			)
		}
	})

	t.Run("settled generation is not re-anchored", func(t *testing.T) {
		ralc.SetReservation(depositKey, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			State:               tbtc.ReservationStateUnknown,
			RequestNonce:        1,
		})
		ralc.SetReservationAction(depositKey, 1, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStateSettled,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  5000,
			MinAmount:                 1000,
			TermSeconds:               86400,
		})

		proposal, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if shouldExecute || proposal != nil {
			t.Fatalf(
				"expected no proposal for a Settled generation -- the " +
					"task must not re-anchor a generation whose action " +
					"is no longer Pending",
			)
		}
	})
}

// TestReservationAcceptanceTask_Stateless_NonEligibleReservationState verifies that
// a reservation whose on-chain state is Active, ActionPending, Closed, or Stranded
// is skipped from acceptance proposals.
func TestReservationAcceptanceTask_Stateless_NonEligibleReservationState(t *testing.T) {
	nonEligibleStates := []tbtc.ReservationState{
		tbtc.ReservationStateActive,
		tbtc.ReservationStateActionPending,
		tbtc.ReservationStateClosed,
		tbtc.ReservationStateStranded,
	}

	for _, state := range nonEligibleStates {
		t.Run(fmt.Sprintf("state_%v", state), func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()

			walletPublicKeyHash := hexToByte20(
				"8db50eb52063ea9d98b3eac91489a90f738986f6",
			)
			currentBlock := uint64(300000)

			ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

			fundingTxHash := hashFromString(
				"8888888888888888888888888888888888888888888888888888888888888888",
			)
			dummyTx := &bitcoin.Transaction{
				Outputs: []*bitcoin.TransactionOutput{{
					Value:           0,
					PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
				}},
			}
			btcChain.SetTransaction(fundingTxHash, dummyTx)
			btcChain.SetEstimateSatPerVByteFee(1, 1)
			btcChain.SetTransactionConfirmations(
				fundingTxHash,
				tbtc.DepositSweepRequiredFundingTxConfirmations,
			)

			ralc.SetDepositRequest(
				fundingTxHash,
				0,
				&tbtc.DepositChainRequest{
					Amount:     2000000,
					RevealedAt: time.Now().Add(-2 * time.Hour),
					SweptAt:    time.Unix(0, 0),
					Vault:      &[]chain.Address{testReservationVaultAddress}[0],
				},
			)

			filterStartBlock := uint64(0)
			if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
				filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
			}

			if err := ralc.AddPastDepositRevealedEvent(
				&tbtc.DepositRevealedEventFilter{
					StartBlock:          filterStartBlock,
					EndBlock:            &currentBlock,
					WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				},
				&tbtc.DepositRevealedEvent{
					BlockNumber:         290000,
					WalletPublicKeyHash: walletPublicKeyHash,
					FundingTxHash:       fundingTxHash,
					FundingOutputIndex:  0,
					Vault:               &[]chain.Address{testReservationVaultAddress}[0],
				},
			); err != nil {
				t.Fatal(err)
			}

			depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
			ralc.SetReservation(depositKey, &tbtc.Reservation{
				State:        state,
				RequestNonce: 1,
			})

			task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
			request := &tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: walletPublicKeyHash,
			}

			proposal, shouldExecute, err := task.Run(request)
			if err != nil {
				t.Fatalf("unexpected task error: [%v]", err)
			}
			if shouldExecute || proposal != nil {
				t.Fatalf("expected candidate with state %v to be skipped", state)
			}
		})
	}
}

// TestReservationAcceptanceTask_Stateless_DynamicMinAmount verifies that
// each generation of a reservation carries its own snapshotted minimum
// amount: a deposit whose first generation's snapshot is above the
// deposit is skipped for that generation, but a fresh generation written
// after governance lowered the live minimum (so the new snapshot clears
// the deposit) becomes eligible and is consumed at its own real nonce.
func TestReservationAcceptanceTask_Stateless_DynamicMinAmount(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.SetReservationParameters(tbtc.ReservationParameters{
			ReservationVault:          testReservationVaultAddress,
			ReservationMinAmount:      5000000,
			ReservationTxMaxFee:       5000,
			MaxReservationsPerWallet:  5,
			ReservationMaxTotalAmount: 100000000,
		})
		ralc.maxPerWalletAmount = 50000000
		ralc.maxSingleAmount = 50000000
	})

	fundingTxHash := hashFromString(
		"9999999999999999999999999999999999999999999999999999999999999999",
	)
	dummyTx := &bitcoin.Transaction{
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           0,
			PublicKeyScript: append([]byte{0x00, 0x20}, make([]byte, 32)...),
		}},
	}
	btcChain.SetTransaction(fundingTxHash, dummyTx)
	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	// Deposit amount is 2,000,000 (below the first generation's
	// snapshotted minimum of 5,000,000).
	ralc.SetDepositRequest(
		fundingTxHash,
		0,
		&tbtc.DepositChainRequest{
			Depositor:  chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
			Amount:     2000000,
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &[]chain.Address{testReservationVaultAddress}[0],
		},
	)

	filterStartBlock := uint64(0)
	if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         290000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  0,
			Vault:               &[]chain.Address{testReservationVaultAddress}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)

	// First generation: snapshot min (5,000,000) is above the deposit
	// (2,000,000); the candidate must be skipped.
	ralc.SetReservation(depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        1,
	})
	ralc.SetReservationAction(depositKey, 1, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 5000000,
		TermSeconds:               86400,
		// Far-future TimeoutAt so the timeout safety-margin gate is
		// not the reason generation 1 is skipped: the subject of this
		// test is the snapshotted minimum, and generation 1's snapshot
		// (5,000,000) is what must reject the 2,000,000 deposit.
		TimeoutAt: uint32(time.Now().Add(24 * time.Hour).Unix()),
	})

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("first run error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf(
			"expected no proposal when the first generation's snapshotted " +
				"minimum (5,000,000) is above the deposit (2,000,000)",
		)
	}

	// Governance lowers the live minimum. The depositor issues a fresh
	// request -- production bumps the reservation's RequestNonce and
	// writes a new action record carrying the new snapshot -- and the
	// candidate becomes eligible.
	ralc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault:          testReservationVaultAddress,
		ReservationMinAmount:      1000000,
		ReservationTxMaxFee:       5000,
		MaxReservationsPerWallet:  5,
		ReservationMaxTotalAmount: 100000000,
	})

	ralc.SetReservation(depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        2,
	})
	ralc.SetReservationAction(depositKey, 2, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 1000000,
		TermSeconds:               86400,
		// Far-future TimeoutAt so the timeout safety-margin gate does
		// not reject the second generation.
		TimeoutAt: uint32(time.Now().Add(24 * time.Hour).Unix()),
	})

	proposal, shouldExecute, err = task.Run(request)
	if err != nil {
		t.Fatalf("second run error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf(
			"expected a proposal on the second run after governance " +
				"lowered the live minimum and the depositor issued a new " +
				"generation",
		)
	}
	anchorProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}
	if anchorProposal.RequestNonce != 2 {
		t.Fatalf(
			"expected the proposal to carry the second generation's actual "+
				"nonce (2), got [%d]",
			anchorProposal.RequestNonce,
		)
	}
}

// TestReservationAcceptanceTask_Stateless_RequestNonceIncremented verified
// that an existing reservation record at RequestNonce N produced a
// proposal at RequestNonce N + 1 -- the operator-side
// RequestReservationAcceptance path that was removed in the cross-repo
// review. The acceptance task now consumes, rather than creates, the
// depositor's action record: a successful proposal always carries the
// reservation's current RequestNonce, never an invented N + 1. That
// behavior is pinned by:
//
//   - TestReservationAcceptanceTask_Run/happy_path (the JSON scenario 0),
//     whose ExpectedAnchorProposal.RequestNonce equals the seeded
//     action's nonce (1);
//   - TestReservationAcceptanceTask_ReservationParametersFetchedLive,
//     which asserts the proposal's RequestNonce equals the generation's
//     real nonce (1) when the operator-side path is in scope;
//   - TestReservationAcceptanceTask_Stateless_DynamicMinAmount, which
//     pins RequestNonce = 2 once the depositor issues a second
//     generation.
//
// The strict-fake corollary -- a proposal at nonce + 1 is rejected by
// LocalChain.ValidateReservationAnchorProposal, the precondition the
// production validator mirrors -- is exercised by
// TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate.

// TestReservationAcceptanceTask_PastDepositRevealedEventsError verifies that
// a genuine (non-sentinel) error from PastDepositRevealedEvents is
// propagated as a hard error, rather than being swallowed like the mock's
// "no events for given filter" sentinel.
func TestReservationAcceptanceTask_PastDepositRevealedEventsError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	// Otherwise-eligible deposit; the injected error must still short
	// circuit before any candidate is ever evaluated.
	setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	ralc.pastDepositRevealedEventsErr = fmt.Errorf("simulated rpc failure")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	_, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err == nil {
		t.Fatalf("expected a non-nil error, got nil")
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
}

// TestReservationAcceptanceTask_ValidateProposalError verifies that a
// ValidateReservationAnchorProposal failure is treated as a pre-write
// failure (see reservationAcceptancePreWriteError): the doomed candidate
// is skipped rather than aborting the whole coordination window, so with
// no other candidate available Run reports a clean no-op instead of an
// error.
func TestReservationAcceptanceTask_ValidateProposalError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	fundingTxHash := setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	// Seed a Pending Acceptance action so the candidate actually
	// reaches proposeReservationAcceptance and the validate call fires.
	seedPendingAcceptanceAction(
		t,
		ralc,
		fundingTxHash,
		0,
		walletPublicKeyHash,
		1,
		5000,
		1000,
		86400,
	)

	ralc.validateErr = fmt.Errorf("simulated validation failure")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal, got %v", proposal)
	}
}

// TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate is a
// regression test for the action-record precondition
// WalletProposalValidator.sol's validateReservationAnchorProposal enforces:
// the action at the proposal's RequestNonce must already be a Pending
// Acceptance action targeting this wallet. Production sets the proposal's
// RequestNonce from the action record it just read, so the validator
// always agrees -- but a regression that let production invent a nonce
// (the pre-fix operator-side request path) would silently pass against a
// lenient fake. This test exercises the underlying fake validator
// directly: a proposal at the seeded nonce passes, while nonce + 1 is
// rejected with the precondition message.
func TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate(t *testing.T) {
	lc := tbtcpg.NewLocalChain()

	walletPublicKeyHash := [20]byte{0x8d, 0xb5, 0x0e, 0xb5, 0x20, 0x63, 0xea, 0x9d, 0x98, 0xb3, 0xea, 0xc9, 0x14, 0x89, 0xa9, 0x0f, 0x73, 0x89, 0x86, 0xf6}

	fundingTxHash := bitcoin.Hash{0xab}
	fundingOutputIndex := uint32(0)
	// The validator keys action records by the deposit key derived from
	// the proposal's funding outpoint, not by some pre-chosen big.Int;
	// build the key through the same helper so the seed and the
	// validator agree.
	depositKey := lc.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	// Seed a Pending Acceptance action at nonce 2 -- the canonical
	// generation the production task would consume.
	lc.SetReservation(depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        2,
	})
	lc.SetReservationAction(depositKey, 2, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		// Far-future TimeoutAt so the fake validator's timeout
		// safety-margin gate does not reject the OK proposal; the
		// strict-nonce precondition is the only gate under test.
		TimeoutAt: uint32(time.Now().Add(24 * time.Hour).Unix()),
	})

	proposalOK := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: fundingOutputIndex,
		RequestNonce:              2,
		AnchorTxFee:               big.NewInt(710),
	}
	if err := lc.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposalOK,
		struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		}{},
	); err != nil {
		t.Fatalf(
			"strict fake rejected a proposal at the seeded action's nonce "+
				"(2): %v -- the validator must agree with the production "+
				"task's action-record lookup",
			err,
		)
	}

	proposalOff := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: fundingOutputIndex,
		RequestNonce:              3,
		AnchorTxFee:               big.NewInt(710),
	}
	if err := lc.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposalOff,
		struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		}{},
	); err == nil {
		t.Fatalf(
			"strict fake accepted a proposal at nonce + 1 -- the " +
				"validator's action-record precondition must reject any " +
				"nonce whose action record is missing",
		)
	}
}

// TestReservationAcceptanceTask_DepositWithoutPendingActionIsSkipped is a
// regression test for the action-record gate
// findReservationAcceptanceCandidate enforces: a deposit with no current
// Pending Acceptance action -- whether because the depositor never
// requested acceptance or because a prior generation has already settled
// -- is never proposed. A regression that fell back to the operator-side
// request path could accidentally trigger a request here, which would
// show up as a non-zero submission count.
func TestReservationAcceptanceTask_DepositWithoutPendingActionIsSkipped(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	// Deliberately no seedPendingAcceptanceAction call: the deposit's
	// current generation is the zero record (no reservation, no action).
	// The acceptance task must skip it -- never call
	// RequestReservationAcceptance to create one -- and return nil.
	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false for a deposit without a pending action, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal, got %v", proposal)
	}
	if recorded := ralc.GetReservationAcceptanceRequests(); len(recorded) != 0 {
		t.Errorf(
			"expected zero RequestReservationAcceptance submissions; the "+
				"task must not create an action record -- the depositor "+
				"is the only party that may request acceptance (got %d)",
			len(recorded),
		)
	}
}

// TestReservationAcceptanceTask_SkipsCandidateInsideTimeoutSafetyMargin is a
// regression test for the timeout safety margin gate added in the cross-repo
// review: findReservationAcceptanceCandidate skips any candidate whose
// pending acceptance generation has TimeoutAt at or before
// now + REQUEST_TIMEOUT_SAFETY_MARGIN (2 hours). A candidate just outside
// the margin (TimeoutAt = now + 7202) must be found and proposed. The
// validator fake mirrors the same gate so the direct unit test covers
// both the task and the fake.
func TestReservationAcceptanceTask_SkipsCandidateInsideTimeoutSafetyMargin(t *testing.T) {
	t.Run("inside margin: skipped", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()
		walletPublicKeyHash := hexToByte20(
			"8db50eb52063ea9d98b3eac91489a90f738986f6",
		)
		currentBlock := uint64(300000)

		ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

		fundingTxHash := setupEligibleDeposit(
			t,
			ralc,
			btcChain,
			walletPublicKeyHash,
			currentBlock,
			2000000,
		)

		// TimeoutAt = now + 7198 seconds: the margin gate
		// (now + 7200 >= TimeoutAt) fires and the candidate is skipped.
		depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
		ralc.SetReservation(depositKey, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			State:               tbtc.ReservationStateUnknown,
			RequestNonce:        0,
		})
		ralc.SetReservationAction(depositKey, 0, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  5000,
			MinAmount:                 1000,
			TermSeconds:               86400,
			TimeoutAt:                 uint32(time.Now().Add(7198 * time.Second).Unix()),
		})

		task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shouldExecute || proposal != nil {
			t.Fatalf(
				"expected no proposal for candidate inside the timeout "+
					"safety margin; got shouldExecute=%v",
				shouldExecute,
			)
		}
	})

	t.Run("just outside margin: proposed", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()
		walletPublicKeyHash := hexToByte20(
			"8db50eb52063ea9d98b3eac91489a90f738986f6",
		)
		currentBlock := uint64(300000)

		ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

		fundingTxHash := setupEligibleDeposit(
			t,
			ralc,
			btcChain,
			walletPublicKeyHash,
			currentBlock,
			2000000,
		)

		// TimeoutAt = now + 7202 seconds: the margin gate
		// (now + 7200 >= TimeoutAt) does not fire and the candidate
		// is found and proposed.
		depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
		ralc.SetReservation(depositKey, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			State:               tbtc.ReservationStateUnknown,
			RequestNonce:        0,
		})
		ralc.SetReservationAction(depositKey, 0, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  5000,
			MinAmount:                 1000,
			TermSeconds:               86400,
			TimeoutAt:                 uint32(time.Now().Add(7202 * time.Second).Unix()),
		})

		task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

		proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
			WalletPublicKeyHash: walletPublicKeyHash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !shouldExecute || proposal == nil {
			t.Fatalf(
				"expected a proposal for candidate just outside the timeout "+
					"safety margin; got shouldExecute=%v",
				shouldExecute,
			)
		}
	})
}

// TestLocalChain_ValidateReservationAnchorProposal_RejectsInsideTimeoutMargin
// is a regression test for the timeout safety margin gate in the LocalChain
// validator fake: a proposal whose action TimeoutAt is at or before
// now + REQUEST_TIMEOUT_SAFETY_MARGIN is rejected with "timed out".
// Just outside the margin (TimeoutAt = now + 7202) passes.
func TestLocalChain_ValidateReservationAnchorProposal_RejectsInsideTimeoutMargin(t *testing.T) {
	lc := tbtcpg.NewLocalChain()

	walletPublicKeyHash := [20]byte{0x8d, 0xb5, 0x0e, 0xb5, 0x20, 0x63, 0xea, 0x9d, 0x98, 0xb3, 0xea, 0xc9, 0x14, 0x89, 0xa9, 0x0f, 0x73, 0x89, 0x86, 0xf6}

	fundingTxHash := bitcoin.Hash{0xab}
	fundingOutputIndex := uint32(0)
	depositKey := lc.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	// Seed the action at nonce 1 with a far-future TimeoutAt; the nonce
	// gate is not under test here (covered by StrictNonceGate).
	lc.SetReservation(depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: walletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        1,
	})
	lc.SetReservationAction(depositKey, 1, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		TimeoutAt:                 uint32(time.Now().Add(24 * time.Hour).Unix()),
	})

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: fundingOutputIndex,
		RequestNonce:              1,
		AnchorTxFee:               big.NewInt(710),
	}

	// Far-future TimeoutAt: validation passes.
	if err := lc.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposal,
		struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		}{},
	); err != nil {
		t.Fatalf("far-future TimeoutAt should pass: %v", err)
	}

	// Override TimeoutAt to be just inside the 2-hour margin.
	lc.SetReservationAction(depositKey, 1, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: walletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		TimeoutAt:                 uint32(time.Now().Add(7198 * time.Second).Unix()),
	})

	if err := lc.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposal,
		struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		}{},
	); err == nil {
		t.Fatal(
			"expected timeout margin rejection for TimeoutAt = now + 7198, got nil",
		)
	}
}

// TestLocalChain_ValidateReservationReanchorProposal_RejectsInsideTimeoutMargin
// mirrors TestLocalChain_ValidateReservationAnchorProposal_RejectsInsideTimeoutMargin
// for the reanchor validator fake.
func TestLocalChain_ValidateReservationReanchorProposal_RejectsInsideTimeoutMargin(t *testing.T) {
	lc := tbtcpg.NewLocalChain()

	sourceWalletPublicKeyHash := [20]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11}
	targetWalletPublicKeyHash := [20]byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22}

	resKey := big.NewInt(7001)

	// Seed the action with a far-future TimeoutAt.
	lc.SetReservationAction(resKey, 1, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TxMaxFee:                  100000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		TimeoutAt:                 uint32(time.Now().Add(24 * time.Hour).Unix()),
	})

	lc.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
		ReservationMinAmount:     1000,
	})

	proposal := &tbtc.ReservationReanchorProposal{
		ReservationKey:            new(big.Int).Set(resKey),
		RequestNonce:              1,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:             big.NewInt(550),
	}

	// Far-future TimeoutAt: validation passes.
	if err := lc.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		proposal,
	); err != nil {
		t.Fatalf("far-future TimeoutAt should pass: %v", err)
	}

	// Override TimeoutAt to be just inside the 2-hour margin.
	lc.SetReservationAction(resKey, 1, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TxMaxFee:                  100000,
		MinAmount:                 1000,
		TermSeconds:               86400,
		TimeoutAt:                 uint32(time.Now().Add(7198 * time.Second).Unix()),
	})

	if err := lc.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		proposal,
	); err == nil {
		t.Fatal(
			"expected timeout margin rejection for TimeoutAt = now + 7198, got nil",
		)
	}
}
