package tbtcpg_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	"github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

// reservationAcceptanceLocalChain is a test-only mock of tbtcpg.Chain that
// embeds the production LocalChain and adds reservation-specific behavior.
// It exists as a separate type so this test file does not need to edit the
// shared chain_test.go fixture used by sibling builders.
type reservationAcceptanceLocalChain struct {
	*tbtcpg.LocalChain

	reservationParameters    *tbtc.ReservationParameters
	maxPerWalletAmount       uint64
	maxSingleAmount          uint64
	walletReservationsAmount uint64
	walletReservationsCount  uint32
	activeCount              uint32
	maxActive                uint32
	pendingReserved          uint64
	reservedDeposits         map[string]bool
	validateErr              error
}

func newReservationAcceptanceLocalChain() *reservationAcceptanceLocalChain {
	lc := tbtcpg.NewLocalChain()
	return &reservationAcceptanceLocalChain{
		LocalChain:       lc,
		reservedDeposits: make(map[string]bool),
	}
}

// PastDepositRevealedEvents overrides the embedded LocalChain
// implementation to return an empty slice (rather than an error) when no
// events are registered for the filter. A real chain returns an empty
// event list when no deposits match; the in-memory mock's panic-stub
// "no events for given filter" error is a fixture bug that this override
// papers over without touching shared test infrastructure.
func (ralc *reservationAcceptanceLocalChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	events, err := ralc.LocalChain.PastDepositRevealedEvents(filter)
	if err != nil {
		return []*tbtc.DepositRevealedEvent{}, nil
	}
	return events, nil
}

func (ralc *reservationAcceptanceLocalChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	return ralc.reservationParameters, nil
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

func (ralc *reservationAcceptanceLocalChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	if depositKey == nil {
		return false, nil
	}
	return ralc.reservedDeposits[depositKey.Text(16)], nil
}

func (ralc *reservationAcceptanceLocalChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	return ralc.validateErr
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

	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault:          reservationVault,
		ReservationMinAmount:      scenario.ReservationParameters.ReservationMinAmount,
		ReservationTxMaxFee:       scenario.ReservationParameters.ReservationTxMaxFee,
		ReservationMaxTotalAmount: scenario.ReservationParameters.ReservationMaxTotalAmount,
		ReservationTotalAmount:    scenario.ReservationParameters.ReservationTotalAmount,
		MaxReservationsPerWallet:  scenario.ReservationParameters.MaxReservationsPerWallet,
	}

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
// mock chain as deposit requests and past DepositRevealedEvents. It also
// marks them as reserved via IsReservedDeposit. Bitcoin transaction
// registrations live on the btcChain mock.
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

		err = ralc.AddPastDepositRevealedEvent(
			&tbtc.DepositRevealedEventFilter{
				StartBlock:          filterStartBlock,
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

		depositKey := ralc.BuildDepositKey(
			materialized.FundingTxHash,
			materialized.FundingOutputIndex,
		)
		ralc.reservedDeposits[depositKey.Text(16)] = true
	}
}

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

// TestReservationAcceptanceTask_NoCandidates verifies that the task is a
// no-op when the chain has no reserved deposits.
func TestReservationAcceptanceTask_NoCandidates(t *testing.T) {
	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
	}
	ralc.maxPerWalletAmount = 1000000
	ralc.maxSingleAmount = 5000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(3600)
	ralc.SetWallet(
		walletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
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

// TestReservationAcceptanceTask_BoundedLookback verifies that the bounded
// look-back window is applied when the current block exceeds it.
func TestReservationAcceptanceTask_BoundedLookback(t *testing.T) {
	currentBlock := uint64(400000)
	expectedStartBlock := currentBlock -
		tbtcpg.ReservationAcceptanceLookBackBlocks

	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
		ReservationMinAmount:     1000,
		ReservationTxMaxFee:      5000,
		MaxReservationsPerWallet: 5,
	}
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

	// Event below the look-back start block must NOT be returned.
	oldFundingTxHash := hashFromString(
		"1111111111111111111111111111111111111111111111111111111111111111",
	)
	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          0,
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

	// Event at the look-back start block must be returned. Mark it as
	// reserved and provide a deposit request.
	fundingTxHash := hashFromString(
		"2222222222222222222222222222222222222222222222222222222222222222",
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
			Vault: &[]chain.Address{chain.Address(
				"0xReservationVaultAddress1234567890abcdef12345678",
			)}[0],
		},
	)
	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)
	ralc.reservedDeposits[depositKey.Text(16)] = true

	if err := ralc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          expectedStartBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         expectedStartBlock,
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
	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
	}
	ralc.maxPerWalletAmount = 5000000
	ralc.maxSingleAmount = 5000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(3600)
	ralc.SetWallet(
		walletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
	ralc.SetBlockCounter(blockCounter)

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
		t.Errorf("expected nil proposal, got [%+v]", proposal)
	}
}

var _ = fmt.Sprintf
