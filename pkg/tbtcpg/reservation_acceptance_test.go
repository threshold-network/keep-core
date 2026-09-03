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
	getWalletErr             error
	getReservationErr        error
	acceptanceEvents         []*tbtc.ReservationAcceptanceRequestedEvent
	acceptanceEventsErr      error
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
	if ralc.reservationParameters == nil {
		return nil, nil
	}
	paramsCopy := *ralc.reservationParameters
	return &paramsCopy, nil
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

func (ralc *reservationAcceptanceLocalChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	if ralc.getWalletErr != nil {
		return nil, ralc.getWalletErr
	}
	return ralc.LocalChain.GetWallet(walletPublicKeyHash)
}

func (ralc *reservationAcceptanceLocalChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	if ralc.getReservationErr != nil {
		return nil, ralc.getReservationErr
	}
	return ralc.LocalChain.GetReservation(reservationKey)
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

		depositKey := ralc.BuildDepositKey(
			materialized.FundingTxHash,
			materialized.FundingOutputIndex,
		)
		// kept for parity with registerReservedDeposits; not read by ReservationAcceptanceTask
		ralc.reservedDeposits[depositKey.Text(16)] = true
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

	vaultAddress := chain.Address(
		"0xReservationVaultAddress1234567890abcdef12345678",
	)
	if ralc.reservationParameters != nil && ralc.reservationParameters.ReservationVault != "" {
		vaultAddress = ralc.reservationParameters.ReservationVault
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
	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	privateKey, err := ecdsa.GenerateKey(btcec.S256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(&privateKey.PublicKey)

	depositAmount := uint64(2000000)
	currentBlock := uint64(300000)

	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
		ReservationMinAmount:     1000,
		ReservationTxMaxFee:      5000,
		MaxReservationsPerWallet: 5,
	}
	ralc.maxPerWalletAmount = 50000000
	ralc.maxSingleAmount = 50000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(3600)
	ralc.SetWallet(
		walletPublicKeyHash,
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

	deposit := &tbtc.Deposit{
		Depositor:           chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637"),
		BlindingFactor:      [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		WalletPublicKeyHash: walletPublicKeyHash,
		RefundPublicKeyHash: [20]byte{0x02},
		RefundLocktime:      [4]byte{0x03, 0x04, 0x05, 0x06},
		Vault: &[]chain.Address{chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		)}[0],
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

// TestReservationAcceptanceTask_BoundedLookback verifies that the bounded
// look-back window is applied when the current block exceeds it.
func TestReservationAcceptanceTask_BoundedLookback(t *testing.T) {
	currentBlock := uint64(400000)

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

	// First run: only the old deposit exists below the look-back window.
	// Because the candidate satisfies all other conditions (valid wallet,
	// clear caps), the shouldExecute=false result is strictly attributable
	// to exclusion by the look-back start block filter.
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

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
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
	// No SetWallet call: GetWallet fails for the candidate wallet, and
	// getWalletErr forces the exact error to assert against.
	ralc.getWalletErr = fmt.Errorf("boom")

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
}

// TestReservationAcceptanceTask_Stateless_Maturity verifies the stateless
// observable contract across two consecutive Run calls on the same task instance:
// an immature candidate is skipped on the first run, but when time advances and
// the candidate matures, the second run on the same task instance proposes it
// without any cache-state interference.
func TestReservationAcceptanceTask_Stateless_Maturity(t *testing.T) {
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

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

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
			Vault: &[]chain.Address{chain.Address(
				"0xReservationVaultAddress1234567890abcdef12345678",
			)}[0],
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
// that ReservationParameters() is read fresh on every Run() call: a
// governance-driven parameter change must take effect on the very next
// call to the same task instance, with no leftover value from a prior run
// observable anywhere in the eligibility decision.
func TestReservationAcceptanceTask_ReservationParametersFetchedLive(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)
	currentBlock := uint64(300000)

	t.Run("without parameter mutation accepts on subsequent run", func(t *testing.T) {
		ralc := newReservationAcceptanceLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

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

		// Second run without mutation: must accept again.
		_, shouldExecute, err = task.Run(request)
		if err != nil {
			t.Fatalf("unexpected error on second run: [%v]", err)
		}
		if !shouldExecute {
			t.Fatalf("expected shouldExecute=true on second run without mutation, got false")
		}
	})

	t.Run("with parameter mutation rejects on subsequent run", func(t *testing.T) {
		ralc := newReservationAcceptanceLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

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
		ralc.reservationParameters.ReservationMinAmount = 3000000

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
// at-limit/one-over-limit boundary crossings for the eligibility
// caps in checkReservationAcceptanceEligibility:
// - MaxReservationsPerWallet
// - ReservationMinAmount
// - ReservationMaxTotalAmount
// - ReservationMaxSingleAmount
// - MaxReservationsAmountPerWallet
// - ActiveReservationsCount
// as well as the net-of-fee minimum check in proposeReservationAcceptance.
func TestReservationAcceptanceTask_BoundaryChecks(t *testing.T) {
	tests := map[string]struct {
		depositAmount            uint64
		maxReservationsPerWallet uint32
		walletReservationsCount  uint32
		reservationMinAmount     uint64
		reservationMaxTotal      uint64
		reservationTotal         uint64
		maxSingleAmount          uint64
		maxPerWalletAmount       uint64
		walletReservationsAmount uint64
		maxActive                uint32
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
			reservationMaxTotal:      5000000,
			expectAccept:             true,
		},
		"ReservationMaxTotalAmount: one over cap rejects": {
			depositAmount:            2000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			reservationTotal:         3000001,
			reservationMaxTotal:      5000000,
			expectAccept:             false,
		},
		"ReservationMaxSingleAmount: exactly at cap accepts": {
			depositAmount:            5000000,
			maxSingleAmount:          5000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"ReservationMaxSingleAmount: one over cap rejects": {
			depositAmount:            5000001,
			maxSingleAmount:          5000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
		},
		"MaxReservationsAmountPerWallet: exactly at cap accepts": {
			depositAmount:            2000000,
			walletReservationsAmount: 3000000,
			maxPerWalletAmount:       5000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"MaxReservationsAmountPerWallet: one over cap rejects": {
			depositAmount:            2000000,
			walletReservationsAmount: 3000001,
			maxPerWalletAmount:       5000000,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             false,
		},
		"ActiveReservationsCount: below limit accepts": {
			depositAmount:            2000000,
			maxActive:                10,
			activeCount:              9,
			maxReservationsPerWallet: 5,
			reservationMinAmount:     1000,
			expectAccept:             true,
		},
		"ActiveReservationsCount: at limit rejects": {
			depositAmount:            2000000,
			maxActive:                10,
			activeCount:              10,
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

			ralc.reservationParameters = &tbtc.ReservationParameters{
				ReservationVault: chain.Address(
					"0xReservationVaultAddress1234567890abcdef12345678",
				),
				ReservationMinAmount:      test.reservationMinAmount,
				ReservationTxMaxFee:       5000,
				MaxReservationsPerWallet:  test.maxReservationsPerWallet,
				ReservationMaxTotalAmount: test.reservationMaxTotal,
				ReservationTotalAmount:    test.reservationTotal,
			}
			ralc.maxPerWalletAmount = 50000000
			if test.maxPerWalletAmount != 0 {
				ralc.maxPerWalletAmount = test.maxPerWalletAmount
			}
			ralc.maxSingleAmount = 50000000
			if test.maxSingleAmount != 0 {
				ralc.maxSingleAmount = test.maxSingleAmount
			}
			ralc.maxActive = 100
			if test.maxActive != 0 {
				ralc.maxActive = test.maxActive
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

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

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

// TestReservationAcceptanceTask_Stateless_PastEventsError verifies that an
// RPC failure querying past acceptance requested events fails closed (skips candidate).
func TestReservationAcceptanceTask_Stateless_PastEventsError(t *testing.T) {
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

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

	fundingTxHash := hashFromString(
		"7777777777777777777777777777777777777777777777777777777777777777",
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

	// Force an error on PastReservationAcceptanceRequestedEvents.
	ralc.acceptanceEventsErr = fmt.Errorf("rpc failure")

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	request := &tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}

	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("unexpected task error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected candidate to be skipped when past events check fails closed")
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

			currentBlock := uint64(300000)
			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(currentBlock)
			ralc.SetBlockCounter(blockCounter)

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
	ralc := newReservationAcceptanceLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	walletPublicKeyHash := hexToByte20(
		"8db50eb52063ea9d98b3eac91489a90f738986f6",
	)

	// Initial min amount is 5,000,000.
	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
		ReservationMinAmount:     5000000,
		ReservationTxMaxFee:      5000,
		MaxReservationsPerWallet: 5,
	}
	ralc.maxPerWalletAmount = 50000000
	ralc.maxSingleAmount = 50000000
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

	// First run: deposit amount is below min, skipped.
	proposal, shouldExecute, err := task.Run(request)
	if err != nil {
		t.Fatalf("first run error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal when deposit is below min amount")
	}

	// Governance lowers min amount to 1,000,000.
	ralc.reservationParameters = &tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
		ReservationMinAmount:     1000000,
		ReservationTxMaxFee:      5000,
		MaxReservationsPerWallet: 5,
	}

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

	currentBlock := uint64(300000)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	ralc.SetBlockCounter(blockCounter)

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
