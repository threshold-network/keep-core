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
// TestReservationAcceptanceTask_GetReservationError exercise the
// candidate-selection fail-open path (see production's documented
// deviation above the call site) while still letting
// proposeReservationAcceptance's later post-request GetReservation
// verification call succeed once the reservation actually exists.
func (ralc *reservationAcceptanceLocalChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	if ralc.getReservationErr != nil {
		err := ralc.getReservationErr
		ralc.getReservationErr = nil
		return nil, err
	}
	return ralc.LocalChain.GetReservation(reservationKey)
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
		ReservationMaxTotalAmount: 100000000,
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

// TestReservationAcceptanceTask_AmountCapBoundaries verifies the three
// amount-based eligibility caps (single-deposit, wallet-aggregate,
// global-total) at their exact boundary: a deposit that would land
// exactly at the cap is accepted, one satoshi over is rejected.
func TestReservationAcceptanceTask_AmountCapBoundaries(t *testing.T) {
	const depositAmount = uint64(1_000_000)

	tests := map[string]struct {
		singleCap        uint64
		walletCap        uint64
		walletExisting   uint64
		globalCap        uint64
		globalExisting   uint64
		expectAcceptance bool
	}{
		"single-deposit cap: exactly at cap is accepted": {
			singleCap:        depositAmount,
			walletCap:        depositAmount * 10,
			globalCap:        depositAmount * 10,
			expectAcceptance: true,
		},
		"single-deposit cap: one over cap is rejected": {
			singleCap:        depositAmount - 1,
			walletCap:        depositAmount * 10,
			globalCap:        depositAmount * 10,
			expectAcceptance: false,
		},
		"wallet-aggregate cap: exactly at cap is accepted": {
			singleCap:        depositAmount * 10,
			walletCap:        depositAmount,
			walletExisting:   0,
			globalCap:        depositAmount * 10,
			expectAcceptance: true,
		},
		"wallet-aggregate cap: one over cap is rejected": {
			singleCap:        depositAmount * 10,
			walletCap:        depositAmount,
			walletExisting:   1,
			globalCap:        depositAmount * 10,
			expectAcceptance: false,
		},
		"global-total cap: exactly at cap is accepted": {
			singleCap:        depositAmount * 10,
			walletCap:        depositAmount * 10,
			globalCap:        depositAmount,
			globalExisting:   0,
			expectAcceptance: true,
		},
		"global-total cap: one over cap is rejected": {
			singleCap:        depositAmount * 10,
			walletCap:        depositAmount * 10,
			globalCap:        depositAmount,
			globalExisting:   1,
			expectAcceptance: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ralc := newReservationAcceptanceLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			walletPublicKeyHash := hexToByte20(
				"8db50eb52063ea9d98b3eac91489a90f738986f6",
			)

			ralc.SetReservationParameters(tbtc.ReservationParameters{
				ReservationVault: chain.Address(
					"0xReservationVaultAddress1234567890abcdef12345678",
				),
				ReservationMinAmount:      1000,
				ReservationTxMaxFee:       5000,
				MaxReservationsPerWallet:  5,
				ReservationMaxTotalAmount: test.globalCap,
				ReservationTotalAmount:    test.globalExisting,
			})
			ralc.maxPerWalletAmount = test.walletCap
			ralc.maxSingleAmount = test.singleCap
			ralc.walletReservationsAmount = test.walletExisting
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

			fundingTxHash := fundingTxHashForTestName(testName)
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
					Amount:     depositAmount,
					RevealedAt: time.Now().Add(-2 * time.Hour),
					SweptAt:    time.Unix(0, 0),
					Vault: &[]chain.Address{chain.Address(
						"0xReservationVaultAddress1234567890abcdef12345678",
					)}[0],
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
			if test.expectAcceptance {
				if !shouldExecute || proposal == nil {
					t.Fatalf("expected proposal to be accepted at the exact cap boundary")
				}
			} else {
				if shouldExecute || proposal != nil {
					t.Fatalf("expected proposal to be rejected one unit over the cap")
				}
			}
		})
	}
}

// fundingTxHashForTestName derives a unique, deterministic funding tx hash
// per subtest name so parallel/sequential subtests never collide on the
// same fixture key.
func fundingTxHashForTestName(name string) bitcoin.Hash {
	sum := 0
	for _, r := range name {
		sum += int(r)
	}
	return hashFromString(fmt.Sprintf("%064x", sum+1))
}

// TestReservationAcceptanceTask_BoundedLookback verifies that the bounded
// look-back window is applied when the current block exceeds it.
func TestReservationAcceptanceTask_BoundedLookback(t *testing.T) {
	currentBlock := uint64(400000)

	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	// Register an event below the look-back start block (block 1).
	oldFundingTxHash := hashFromString(
		"1111111111111111111111111111111111111111111111111111111111111111",
	)
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          0,
			EndBlock:            &currentBlock,
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
	// is honored, by registering an eligible deposit exactly at that block
	// and confirming it is then found and accepted.
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

	// Register an eligible deposit at the look-back start block.
	fundingTxHash := setupEligibleDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		currentBlock,
		2000000,
	)

	// Second run: the deposit at the look-back start block must be found and accepted.
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
// that does not target the reservation vault is filtered out.
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
			SweptAt:    time.Unix(0, 0),
		},
	)

	filterStartBlock := uint64(0)
	if currentBlock > tbtcpg.ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - tbtcpg.ReservationAcceptanceLookBackBlocks
	}

	// Set event vault away from the configured reservation vault so it
	// actually exercises "not reserved".
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
			Vault: &[]chain.Address{chain.Address(
				"0xOtherVaultAddress1234567890abcdef12345678901234",
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
// error passthrough inside findReservationAcceptanceCandidate: a
// reserved deposit candidate is discovered and matches the reservation
// vault, but the candidate wallet's chain data fails to load.
func TestReservationAcceptanceTask_GetWalletError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	// getWalletErr forces GetWallet to fail for the candidate wallet.
	// Production logs and swallows the GetWallet error, so Run must
	// return (nil, false, nil).
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
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected no proposal, got %v", proposal)
	}
}

// TestReservationAcceptanceTask_GetReservationError verifies that when
// GetReservation fails, the task logs the error and falls through to
// RequestReservationAcceptance, continuing with proposal emission.
func TestReservationAcceptanceTask_GetReservationError(t *testing.T) {
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

	// Force GetReservation to return an error.
	ralc.getReservationErr = fmt.Errorf("simulated get reservation error")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if !shouldExecute {
		t.Errorf("expected shouldExecute=true, got false")
	}
	if proposal == nil {
		t.Fatalf("expected non-nil proposal")
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
	// GetReservation's error is intentionally fail-open (see production
	// comment above the call site): a brand-new candidate's first
	// acceptance request nonce defaults to 1.
	if actualProposal.RequestNonce != 1 {
		t.Errorf(
			"unexpected RequestNonce\nexpected: 1\nactual: %d",
			actualProposal.RequestNonce,
		)
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
// anything cached from a prior run on the same task instance: a
// governance-driven ReservationParameters change takes effect on the very
// next call, and an acceptance request recorded as a side effect of one
// Run() is visible to production's dedup guard on the next.
func TestReservationAcceptanceTask_ReservationParametersFetchedLive(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	t.Run("records acceptance request, skipping duplicate on subsequent run", func(t *testing.T) {
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

		// First run: min amount (1000) is well below the deposit (2000000) - must accept.
		_, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on first run: [%v]", err)
		}
		if !shouldExecute {
			t.Fatalf("expected shouldExecute=true on first run, got false")
		}

		// Second run on the same, now-requested deposit: RequestReservationAcceptance
		// recorded a ReservationAcceptanceRequestedEvent as a side effect of the
		// first run, so production's dedup guard (which queries
		// PastReservationAcceptanceRequestedEvents) must now find it and skip the
		// candidate. Without the fixture actually recording that event, this run
		// would (incorrectly) accept again and the dedup guard would go untested.
		_, shouldExecute, err = task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on second run: [%v]", err)
		}
		if shouldExecute {
			t.Fatalf("expected shouldExecute=false on second run due to existing acceptance request, got true")
		}
	})

	t.Run("with parameter mutation rejects on subsequent run", func(t *testing.T) {
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

		// First run: min amount (1000) is well below the deposit (2000000) - must accept.
		_, shouldExecute, err := task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on first run: [%v]", err)
		}
		if !shouldExecute {
			t.Fatalf("expected shouldExecute=true on first run, got false")
		}

		// Mutate the chain fake's parameters in place - same task instance,
		// same deposit, no new task created - then raise the min amount above
		// the deposit's value.
		ralc.SetReservationParameters(tbtc.ReservationParameters{
			ReservationVault:         testReservationVaultAddress,
			ReservationMinAmount:     3000000,
			ReservationTxMaxFee:      5000,
			MaxReservationsPerWallet: 5,
		})

		// Second run: if any part of the eligibility path retained the
		// first run's ReservationMinAmount=1000 instead of reading the
		// mutated live value, this would wrongly accept again.
		_, shouldExecute, err = task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on second run: [%v]", err)
		}
		if shouldExecute {
			t.Fatalf(
				"expected shouldExecute=false on second run after raising " +
					"ReservationMinAmount above the deposit's value",
			)
		}
	})
}

// TestReservationAcceptanceTask_BoundaryChecks exercises explicit
// at-limit/one-over-limit boundary crossings for the eligibility caps in
// checkReservationAcceptanceEligibility:
// - MaxReservationsPerWallet
// - ReservationMaxTotalAmount
// - ReservationMaxSingleAmount
// - MaxReservationsAmountPerWallet
// - ActiveReservationsCount
// ReservationMinAmount is not one of checkReservationAcceptanceEligibility's
// gates: the gross gate lives in findReservationAcceptanceCandidate, which
// requires depositAmount >= ReservationMinAmount; the same function
// additionally requires the net-of-fee value (deposit minus the estimated
// anchor fee) to clear it too.
func TestReservationAcceptanceTask_BoundaryChecks(t *testing.T) {
	tests := map[string]struct {
		depositAmount            uint64
		maxReservationsPerWallet uint32
		walletReservationsCount  uint32
		reservationMinAmount     uint64
		reservationTotal         uint64
		// maxSingleAmount, maxPerWalletAmount, maxActive, and
		// reservationMaxTotal are pointers so a test row can explicitly
		// request the cap-disabled value of 0 for the three caps where
		// production treats 0 as "unlimited", or explicitly exercise the
		// fail-closed misconfiguration path for the two caps
		// (maxActiveReservations, ReservationMaxTotalAmount) where
		// production treats 0 as "not configured" instead; nil means "use
		// this test's default cap" (a large, effectively-unlimited value).
		reservationMaxTotal      *uint64
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
			expectAccept:             true,
		},
		"MaxReservationsPerWallet: at limit rejects": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			walletReservationsCount:  5,
			reservationMinAmount:     1000,
			expectAccept:             false,
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
		"ReservationMinAmount: exactly at minimum accepts": {
			depositAmount:            100000 + testAnchorFeeSat,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			expectAccept:             true,
		},
		"ReservationMinAmount: gross clears but net-of-fee value does not": {
			depositAmount:            100050,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			expectAccept:             false,
		},
		"ReservationMinAmount: one below minimum rejects": {
			depositAmount:            99999,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     100000,
			expectAccept:             false,
		},
		"ReservationMaxTotalAmount: exactly at cap accepts": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			reservationTotal:         3000000,
			reservationMaxTotal:      uint64Ptr(5000000),
			expectAccept:             true,
		},
		"ReservationMaxTotalAmount: one over cap rejects": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			reservationTotal:         3000001,
			reservationMaxTotal:      uint64Ptr(5000000),
			expectAccept:             false,
		},
		// Unlike ReservationMaxSingleAmount and MaxReservationsAmountPerWallet
		// below, ReservationMaxTotalAmount == 0 is NOT treated as
		// "unlimited": checkReservationAcceptanceEligibility fails closed on
		// a misconfigured (zero) global cap rather than silently allowing
		// unbounded reservations.
		"ReservationMaxTotalAmount: cap of 0 fails closed (misconfiguration)": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			reservationMaxTotal:      uint64Ptr(0),
			expectAccept:             false,
		},
		"ReservationMaxSingleAmount: exactly at cap accepts": {
			depositAmount:            5000000,
			maxSingleAmount:          uint64Ptr(5000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"ReservationMaxSingleAmount: one over cap rejects": {
			depositAmount:            5000001,
			maxSingleAmount:          uint64Ptr(5000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
		},
		// A cap of 0 means "unlimited" in checkReservationAcceptanceEligibility
		// (reservationMaxSingleAmount > 0 gates the check); maxPerWalletAmount
		// is raised explicitly so it does not itself gate this deposit.
		"ReservationMaxSingleAmount: cap of 0 means unlimited": {
			depositAmount:            60000000,
			maxSingleAmount:          uint64Ptr(0),
			maxPerWalletAmount:       uint64Ptr(100000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"MaxReservationsAmountPerWallet: exactly at cap accepts": {
			depositAmount:            2000000,
			walletReservationsAmount: 3000000,
			maxPerWalletAmount:       uint64Ptr(5000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"MaxReservationsAmountPerWallet: one over cap rejects": {
			depositAmount:            2000000,
			walletReservationsAmount: 3000001,
			maxPerWalletAmount:       uint64Ptr(5000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
		},
		// A cap of 0 means "unlimited" (maxReservationsAmountPerWallet > 0
		// gates the check); maxSingleAmount is raised explicitly so it does
		// not itself gate this deposit.
		"MaxReservationsAmountPerWallet: cap of 0 means unlimited": {
			depositAmount:            2000000,
			walletReservationsAmount: 60000000,
			maxPerWalletAmount:       uint64Ptr(0),
			maxSingleAmount:          uint64Ptr(100000000),
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"ActiveReservationsCount: below limit accepts": {
			depositAmount:            2000000,
			maxActive:                uint32Ptr(10),
			activeCount:              9,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"ActiveReservationsCount: at limit rejects": {
			depositAmount:            2000000,
			maxActive:                uint32Ptr(10),
			activeCount:              10,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
		},
		// Unlike ReservationMaxSingleAmount and MaxReservationsAmountPerWallet
		// above, maxActiveReservations == 0 is NOT treated as "unlimited":
		// checkReservationAcceptanceEligibility fails closed on a
		// misconfigured (zero) active-reservations cap rather than silently
		// allowing unbounded active reservations.
		"ActiveReservationsCount: cap of 0 fails closed (misconfiguration)": {
			depositAmount:            2000000,
			maxActive:                uint32Ptr(0),
			activeCount:              1000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
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

			reservationMaxTotal := uint64(100000000)
			if test.reservationMaxTotal != nil {
				reservationMaxTotal = *test.reservationMaxTotal
			}
			ralc.SetReservationParameters(tbtc.ReservationParameters{
				ReservationVault:          testReservationVaultAddress,
				ReservationMinAmount:      test.reservationMinAmount,
				ReservationTxMaxFee:       5000,
				MaxReservationsPerWallet:  test.maxReservationsPerWallet,
				ReservationMaxTotalAmount: reservationMaxTotal,
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

			setupEligibleDeposit(
				t,
				ralc,
				btcChain,
				walletPublicKeyHash,
				currentBlock,
				test.depositAmount,
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

// TestReservationAcceptanceTask_Stateless_NoReRequest verifies that once a
// reservation has an existing acceptance requested event, subsequent Run calls
// on the same task instance do not produce a duplicate acceptance proposal.
func TestReservationAcceptanceTask_Stateless_NoReRequest(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
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

	// First run: deposit is eligible and proposed.
	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("first run error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf("expected proposal on first run")
	}

	// Simulate on-chain record: mark reservation as having an acceptance
	// requested event.
	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
	ralc.AddPastReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      depositKey,
		RequestNonce:        1,
		WalletPublicKeyHash: walletPublicKeyHash,
	})

	// Second run on the same task instance: must not produce a second proposal.
	proposal, shouldExecute, err = task.Run(request)
	if err != nil {
		t.Fatalf("second run error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal on second run due to existing acceptance event")
	}
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

// TestReservationAcceptanceTask_Stateless_DynamicMinAmount verifies that the
// minimum-amount filter is retryable: a deposit below minimum on Run 1 is skipped,
// but when governance lowers the minimum amount, Run 2 on the same task instance
// proposes it.
func TestReservationAcceptanceTask_Stateless_DynamicMinAmount(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	// Initial min amount is 5,000,000.
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

	// Deposit amount is 2,000,000 (below initial 5,000,000 min).
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

	// First run: deposit amount is below min, skipped.
	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("first run error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal when deposit is below min amount")
	}

	// Governance lowers min amount to 1,000,000.
	ralc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault:          testReservationVaultAddress,
		ReservationMinAmount:      1000000,
		ReservationTxMaxFee:       5000,
		MaxReservationsPerWallet:  5,
		ReservationMaxTotalAmount: 100000000,
	})

	// Second run on the same task instance: deposit is now above min and proposed.
	proposal, shouldExecute, err = task.Run(request)
	if err != nil {
		t.Fatalf("second run error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf("expected proposal on second run after min amount lowered")
	}
}

// TestReservationAcceptanceTask_Stateless_RequestNonceIncremented verifies that
// when an existing reservation record has RequestNonce = N, the generated proposal
// uses RequestNonce = N + 1.
func TestReservationAcceptanceTask_Stateless_RequestNonceIncremented(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)

	fundingTxHash := hashFromString(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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

	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
	// Set an existing reservation with StateUnknown (not active) and RequestNonce = 2.
	ralc.SetReservation(depositKey, &tbtc.Reservation{
		State:        tbtc.ReservationStateUnknown,
		RequestNonce: 2,
	})
	// Set previous action nonce 2 to TimedOut so hasPendingAction returns false.
	ralc.SetReservationAction(depositKey, 2, &tbtc.ReservationAction{
		State: tbtc.ReservationActionStateTimedOut,
	})

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("task error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf("expected proposal")
	}

	actualProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}
	if actualProposal.RequestNonce != 3 {
		t.Errorf(
			"unexpected RequestNonce\nexpected: 3\nactual: %d",
			actualProposal.RequestNonce,
		)
	}
}

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
// ValidateReservationAnchorProposal failure aborts proposal generation with
// a wrapped error, rather than being silently ignored.
func TestReservationAcceptanceTask_ValidateProposalError(t *testing.T) {
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

	ralc.validateErr = fmt.Errorf("simulated validation failure")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err == nil {
		t.Fatalf("expected a non-nil error, got nil")
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
	if proposal != nil {
		t.Errorf("expected nil proposal, got %v", proposal)
	}
}
