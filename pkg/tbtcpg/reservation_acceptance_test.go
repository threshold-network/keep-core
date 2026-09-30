package tbtcpg_test

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
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

// testDepositor is the depositor of every deposit built by this file's
// fixtures.
const testDepositor = chain.Address("934b98637ca318a4d6e7ca6ffd1690b8e77df637")

// testReservationTermSeconds is the snapshotted custody term of the
// fixtures' pending acceptance generations: MIN_RESERVATION_TERM (90
// days), the shortest term Reservation.sol accepts.
const testReservationTermSeconds = uint32(90 * 24 * 60 * 60)

// testDepositMinAgeSeconds is WalletProposalValidator.sol's DEPOSIT_MIN_AGE
// (2 hours).
const testDepositMinAgeSeconds = uint32(7200)

// reservationAcceptanceLocalChain is a test-only mock of tbtcpg.Chain that
// embeds the production LocalChain and adds reservation-specific behavior.
// Its PastDepositRevealedEvents and PastReservationAcceptanceRequestedEvents
// answer range queries the way an Ethereum node does, from events
// registered on this type.
type reservationAcceptanceLocalChain struct {
	*tbtcpg.LocalChain

	maxPerWalletAmount       uint64
	maxSingleAmount          uint64
	walletReservationsAmount uint64
	walletReservationsCount  uint32
	activeCount              uint32
	maxActive                uint32
	pendingReserved          uint64
	validateErr              error
	getWalletErr             error
	getReservationErr        error

	acceptanceEvents       []*tbtc.ReservationAcceptanceRequestedEvent
	acceptanceEventsErr    error
	acceptanceEventFilters []tbtc.ReservationAcceptanceRequestedEventFilter

	revealEvents                 []*tbtc.DepositRevealedEvent
	pastDepositRevealedEventsErr error

	getReservationCalls int
	validateCalls       int
	feeDebtVaults       []chain.Address
	feeReserveVaults    []chain.Address
}

func newReservationAcceptanceLocalChain() *reservationAcceptanceLocalChain {
	return &reservationAcceptanceLocalChain{
		LocalChain: tbtcpg.NewLocalChain(),
	}
}

// PastDepositRevealedEvents returns every registered reveal whose block is
// in [StartBlock, EndBlock] and whose wallet matches the filter. It
// rejects unbounded queries and queries wider than
// tbtc.DepositRevealLookupChunkBlocks, the range many providers cap log
// queries at, so a lookup that stops chunking fails loudly.
func (ralc *reservationAcceptanceLocalChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	if ralc.pastDepositRevealedEventsErr != nil {
		return nil, ralc.pastDepositRevealedEventsErr
	}
	if filter == nil || filter.EndBlock == nil {
		return nil, fmt.Errorf("unbounded deposit revealed events query")
	}
	if *filter.EndBlock < filter.StartBlock ||
		*filter.EndBlock-filter.StartBlock+1 > tbtc.DepositRevealLookupChunkBlocks {
		return nil, fmt.Errorf(
			"invalid deposit revealed events block range [%d, %d]",
			filter.StartBlock,
			*filter.EndBlock,
		)
	}

	var result []*tbtc.DepositRevealedEvent
	for _, event := range ralc.revealEvents {
		if event.BlockNumber < filter.StartBlock ||
			event.BlockNumber > *filter.EndBlock {
			continue
		}
		if !walletMatches(filter.WalletPublicKeyHash, event.WalletPublicKeyHash) {
			continue
		}
		result = append(result, event)
	}
	return result, nil
}

// PastReservationAcceptanceRequestedEvents returns every registered request
// matching the filter's block range, wallets and reservation keys, and
// records the filter.
func (ralc *reservationAcceptanceLocalChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if ralc.acceptanceEventsErr != nil {
		return nil, ralc.acceptanceEventsErr
	}
	if filter == nil {
		return nil, fmt.Errorf("unfiltered acceptance requested events query")
	}
	ralc.acceptanceEventFilters = append(ralc.acceptanceEventFilters, *filter)

	var results []*tbtc.ReservationAcceptanceRequestedEvent
	for _, event := range ralc.acceptanceEvents {
		if event.BlockNumber < filter.StartBlock {
			continue
		}
		if filter.EndBlock != nil && event.BlockNumber > *filter.EndBlock {
			continue
		}
		if !walletMatches(filter.WalletPublicKeyHash, event.WalletPublicKeyHash) {
			continue
		}
		if len(filter.ReservationKey) > 0 {
			match := false
			for _, k := range filter.ReservationKey {
				if k != nil && k.Cmp(event.ReservationKey) == 0 {
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

func walletMatches(filter [][20]byte, wallet [20]byte) bool {
	if len(filter) == 0 {
		return true
	}
	for _, w := range filter {
		if w == wallet {
			return true
		}
	}
	return false
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

// GetReservation delegates to the embedded LocalChain and counts calls,
// except that a non-nil getReservationErr is consumed exactly once: it
// fires on the very next call and then clears itself, simulating a
// transient RPC failure rather than a permanent one.
//
// The embedded LocalChain.GetReservation errors for a reservation key that
// was never registered via SetReservation, but the real chain adapter
// reads a Solidity mapping, which never errors for an absent key -- it
// returns the zero-value struct (State == ReservationStateUnknown,
// RequestNonce == 0). This override normalizes the embedded mock's "not
// found" error into that same zero-value record.
func (ralc *reservationAcceptanceLocalChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	ralc.getReservationCalls++
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

// ValidateReservationAnchorProposal counts calls and returns validateErr
// when set. Otherwise it checks the proposal's funding outpoint against
// the candidate deposit's own outpoint (the candidate-deposit mapping
// step) and then applies the embedded LocalChain's strict validator,
// which mirrors every precondition of
// WalletProposalValidator.validateReservationAnchorProposal.
func (ralc *reservationAcceptanceLocalChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	ralc.validateCalls++
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
	if outpoint.TransactionHash != proposal.DepositFundingTxHash ||
		outpoint.OutputIndex != proposal.DepositFundingOutputIndex {
		return fmt.Errorf(
			"validate reservation anchor proposal: funding outpoint mismatch",
		)
	}
	return ralc.LocalChain.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposal,
		depositExtraInfo,
	)
}

// ReservationVaultFeeDebtSat records the vault it was asked about and
// delegates to the embedded LocalChain.
func (ralc *reservationAcceptanceLocalChain) ReservationVaultFeeDebtSat(
	reservationVault chain.Address,
) (uint64, error) {
	ralc.feeDebtVaults = append(ralc.feeDebtVaults, reservationVault)
	return ralc.LocalChain.ReservationVaultFeeDebtSat(reservationVault)
}

// ReservationVaultFeeReserveTbtcBaseUnits records the vault it was asked
// about and delegates to the embedded LocalChain.
func (ralc *reservationAcceptanceLocalChain) ReservationVaultFeeReserveTbtcBaseUnits(
	reservationVault chain.Address,
) (*big.Int, error) {
	ralc.feeReserveVaults = append(ralc.feeReserveVaults, reservationVault)
	return ralc.LocalChain.ReservationVaultFeeReserveTbtcBaseUnits(reservationVault)
}

// futureRefundLocktime returns a refund locktime 180 days ahead,
// little-endian as the deposit script and the on-chain validator hold it,
// so the validator's 24-hour refund safety margin holds.
func futureRefundLocktime() [4]byte {
	var locktime [4]byte
	binary.LittleEndian.PutUint32(
		locktime[:],
		uint32(time.Now().Add(180*24*time.Hour).Unix()),
	)
	return locktime
}

// buildReservedDeposit builds a deposit revealed to walletPublicKeyHash
// through the test reservation vault, together with a funding transaction
// whose output 0 locks amount with the deposit's P2WSH script. seed makes
// the deposit, and so its funding transaction hash, unique.
func buildReservedDeposit(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	amount uint64,
	seed byte,
	vault chain.Address,
) (*tbtc.Deposit, *bitcoin.Transaction) {
	t.Helper()

	deposit := &tbtc.Deposit{
		Depositor:           testDepositor,
		BlindingFactor:      [8]byte{seed, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		WalletPublicKeyHash: walletPublicKeyHash,
		RefundPublicKeyHash: [20]byte{0x02, seed},
		RefundLocktime:      futureRefundLocktime(),
		Vault:               &vault,
	}

	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatal(err)
	}
	depositLockingScript, err := bitcoin.PayToWitnessScriptHash(
		bitcoin.WitnessScriptHash(depositScript),
	)
	if err != nil {
		t.Fatal(err)
	}

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x09, seed},
				OutputIndex:     0,
			},
			Sequence: 0xffffffff,
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           int64(amount),
			PublicKeyScript: depositLockingScript,
		}},
	}

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: int64(amount),
	}

	return deposit, fundingTx
}

// pendingDepositOptions describes a reserved deposit whose depositor
// requested acceptance, as addPendingDeposit seeds it.
type pendingDepositOptions struct {
	amount        uint64
	seed          byte
	revealBlock   uint64
	requestBlock  uint64
	revealedAt    time.Time
	confirmations uint
	requestNonce  uint64
	txMaxFee      uint64
	minAmount     uint64
	termSeconds   uint32
	timeoutAt     uint32
	actionState   tbtc.ReservationActionState
	// withoutAction seeds the request event but no reservation or
	// action record, as when the generation's record is gone.
	withoutAction bool
	// revealVault, when set, overrides the vault in the reveal event.
	revealVault *chain.Address
}

// defaultPendingDepositOptions returns options for a mature, confirmed
// 2,000,000 sat deposit revealed 1000 blocks and requested 10 blocks
// before currentBlock, whose nonce-1 Pending Acceptance generation
// snapshots a 5000 sat max fee, a 1000 sat minimum, a 90-day term and a
// timeout 24 hours ahead.
func defaultPendingDepositOptions(currentBlock uint64) pendingDepositOptions {
	return pendingDepositOptions{
		amount:        2000000,
		seed:          0x01,
		revealBlock:   currentBlock - 1000,
		requestBlock:  currentBlock - 10,
		revealedAt:    time.Now().Add(-3 * time.Hour),
		confirmations: tbtc.DepositSweepRequiredFundingTxConfirmations,
		requestNonce:  1,
		txMaxFee:      5000,
		minAmount:     1000,
		termSeconds:   testReservationTermSeconds,
		timeoutAt:     uint32(time.Now().Add(24 * time.Hour).Unix()),
		actionState:   tbtc.ReservationActionStatePending,
	}
}

// pendingDeposit is a deposit seeded by addPendingDeposit.
type pendingDeposit struct {
	fundingTxHash bitcoin.Hash
	depositKey    *big.Int
	deposit       *tbtc.Deposit
}

// addPendingDeposit seeds everything the chain and the Bitcoin chain hold
// for a reserved deposit whose depositor requested acceptance by
// walletPublicKeyHash: the funding transaction and its confirmations, the
// deposit request, the reserved flag, the DepositRevealed event, the
// reservation and its acceptance action record, and the
// ReservationAcceptanceRequested event.
func addPendingDeposit(
	t *testing.T,
	ralc *reservationAcceptanceLocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
	walletPublicKeyHash [20]byte,
	opts pendingDepositOptions,
) *pendingDeposit {
	t.Helper()

	vault := testReservationVaultAddress
	if params, err := ralc.ReservationParameters(); err == nil &&
		params.ReservationVault != "" {
		vault = params.ReservationVault
	}

	deposit, fundingTx := buildReservedDeposit(
		t,
		walletPublicKeyHash,
		opts.amount,
		opts.seed,
		vault,
	)
	fundingTxHash := fundingTx.Hash()
	depositKey := ralc.BuildDepositKey(fundingTxHash, 0)

	btcChain.SetEstimateSatPerVByteFee(1, 1)
	btcChain.SetTransaction(fundingTxHash, fundingTx)
	btcChain.SetTransactionConfirmations(fundingTxHash, opts.confirmations)

	ralc.SetDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Depositor:  testDepositor,
		Amount:     opts.amount,
		RevealedAt: opts.revealedAt,
		SweptAt:    time.Unix(0, 0),
		Vault:      &vault,
	})
	ralc.SetReservedDeposit(depositKey, true)

	revealVault := &vault
	if opts.revealVault != nil {
		revealVault = opts.revealVault
	}
	ralc.revealEvents = append(ralc.revealEvents, &tbtc.DepositRevealedEvent{
		BlockNumber:         opts.revealBlock,
		WalletPublicKeyHash: walletPublicKeyHash,
		FundingTxHash:       fundingTxHash,
		FundingOutputIndex:  0,
		Depositor:           testDepositor,
		Amount:              opts.amount,
		BlindingFactor:      deposit.BlindingFactor,
		RefundPublicKeyHash: deposit.RefundPublicKeyHash,
		RefundLocktime:      deposit.RefundLocktime,
		Vault:               revealVault,
	})

	if !opts.withoutAction {
		ralc.SetReservation(depositKey, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			State:               tbtc.ReservationStateUnknown,
			RequestNonce:        opts.requestNonce,
		})
		ralc.SetReservationAction(depositKey, opts.requestNonce, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     opts.actionState,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  opts.txMaxFee,
			MinAmount:                 opts.minAmount,
			TermSeconds:               opts.termSeconds,
			TimeoutAt:                 opts.timeoutAt,
			Amount:                    opts.amount,
		})
	}

	ralc.acceptanceEvents = append(
		ralc.acceptanceEvents,
		&tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      depositKey,
			RequestNonce:        opts.requestNonce,
			WalletPublicKeyHash: walletPublicKeyHash,
			DepositAmount:       opts.amount,
			TxMaxFee:            opts.txMaxFee,
			TimeoutAt:           opts.timeoutAt,
			BlockNumber:         opts.requestBlock,
		},
	)

	return &pendingDeposit{
		fundingTxHash: fundingTxHash,
		depositKey:    depositKey,
		deposit:       deposit,
	}
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
		ReservationTermSeconds:    testReservationTermSeconds,
		ReservationActionTimeout:  24 * 60 * 60,
	})

	ralc.maxPerWalletAmount = scenario.Caps.MaxReservationsAmountPerWallet
	ralc.maxSingleAmount = scenario.Caps.ReservationMaxSingleAmount
	ralc.walletReservationsAmount = scenario.WalletCustody.Amount
	ralc.walletReservationsCount = scenario.WalletCustody.Count
	ralc.activeCount = scenario.Global.ActiveCount
	ralc.maxActive = scenario.Global.MaxActive
	ralc.pendingReserved = scenario.PendingReservedDeposits

	ralc.SetDepositMinAge(scenario.ChainParameters.DepositMinAge)
	ralc.SetAverageBlockTime(scenario.ChainParameters.AverageBlockTime)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(scenario.ChainParameters.CurrentBlock)
	ralc.SetBlockCounter(blockCounter)

	// Map a WalletState string back to the tbtc constant. An unknown
	// string fails the test rather than silently running as another
	// state.
	var walletState tbtc.WalletState
	switch scenario.WalletState {
	case "Live":
		walletState = tbtc.StateLive
	case "MovingFunds":
		walletState = tbtc.StateMovingFunds
	case "Closing":
		walletState = tbtc.StateClosing
	case "Closed":
		walletState = tbtc.StateClosed
	case "Terminated":
		walletState = tbtc.StateTerminated
	default:
		t.Fatalf("unknown scenario wallet state [%s]", scenario.WalletState)
	}

	ralc.SetWallet(
		scenario.WalletPublicKeyHash,
		&tbtc.WalletChainData{State: walletState},
	)

	return ralc
}

// registerReservedDeposits wires the scenario's reserved deposits into the
// mock chains: the funding transaction, the deposit request, the reserved
// flag, the DepositRevealed event and, for rows with a
// PendingAcceptanceAction, the reservation, the action record and the
// ReservationAcceptanceRequested event (requested 10 blocks after the
// reveal). It returns the map from each row's label hash to its real
// funding transaction hash.
func registerReservedDeposits(
	t *testing.T,
	scenario *test.ReservationAcceptanceTestScenario,
	ralc *reservationAcceptanceLocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
) map[bitcoin.Hash]bitcoin.Hash {
	t.Helper()

	// proposeReservationAcceptance estimates the anchor fee dynamically;
	// 1 sat/vByte hits the applyWalletTxFeeFloor minimum.
	btcChain.SetEstimateSatPerVByteFee(1, 1)

	hashesByLabel := make(map[bitcoin.Hash]bitcoin.Hash)

	for _, rd := range scenario.ReservedDeposits {
		materialized, err := rd.Materialize()
		if err != nil {
			t.Fatalf(
				"failed to materialize reserved deposit scenario row: [%v]",
				err,
			)
		}
		hashesByLabel[materialized.LabelFundingTxHash] = materialized.FundingTxHash

		depositKey := ralc.BuildDepositKey(
			materialized.FundingTxHash,
			materialized.FundingOutputIndex,
		)

		ralc.SetDepositRequest(
			materialized.FundingTxHash,
			materialized.FundingOutputIndex,
			&tbtc.DepositChainRequest{
				Depositor:  materialized.Depositor,
				Amount:     materialized.Amount,
				RevealedAt: materialized.RevealedAt,
				SweptAt:    materialized.SweptAt,
				Vault:      materialized.Vault,
			},
		)
		ralc.SetReservedDeposit(depositKey, true)

		btcChain.SetTransaction(
			materialized.FundingTxHash,
			materialized.FundingTx,
		)
		btcChain.SetTransactionConfirmations(
			materialized.FundingTxHash,
			rd.FundingTxConfirmations,
		)

		ralc.revealEvents = append(ralc.revealEvents, &tbtc.DepositRevealedEvent{
			BlockNumber:         materialized.RevealBlock,
			WalletPublicKeyHash: materialized.WalletPublicKeyHash,
			FundingTxHash:       materialized.FundingTxHash,
			FundingOutputIndex:  materialized.FundingOutputIndex,
			Depositor:           materialized.Depositor,
			Amount:              materialized.Amount,
			BlindingFactor:      materialized.BlindingFactor,
			RefundPublicKeyHash: materialized.RefundPublicKeyHash,
			RefundLocktime:      materialized.RefundLocktime,
			Vault:               materialized.Vault,
		})

		pendingAction := rd.PendingAcceptanceActionValue()
		if pendingAction == nil {
			continue
		}
		// A reservation's State in production stays Unknown until
		// settlement, so the seeded record follows that convention.
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
		ralc.acceptanceEvents = append(
			ralc.acceptanceEvents,
			&tbtc.ReservationAcceptanceRequestedEvent{
				ReservationKey:      depositKey,
				RequestNonce:        rd.PendingAcceptanceAction.RequestNonce,
				WalletPublicKeyHash: pendingAction.TargetWalletPublicKeyHash,
				DepositAmount:       materialized.Amount,
				TxMaxFee:            pendingAction.TxMaxFee,
				TimeoutAt:           pendingAction.TimeoutAt,
				BlockNumber:         materialized.RevealBlock + 10,
			},
		)
	}

	return hashesByLabel
}

// newBoundaryTestChain builds a reservationAcceptanceLocalChain with the
// reservation-parameters/caps/wallet/block-counter setup shared by most of
// this file's Run()-based tests: a live wallet at walletPublicKeyHash, a
// ReservationParameters of {vault: testReservationVaultAddress, minAmount:
// 1000, txMaxFee: 5000, maxPerWallet: 5, term: 90 days, action timeout: 24
// hours}, per-wallet/single caps of 5000000, an active-reservations cap of
// 100, a deposit minimum age of DEPOSIT_MIN_AGE (2 hours) and a 12-second
// block time. overrides, when non-nil, runs after these defaults so a
// call site can customize only what it varies.
func newBoundaryTestChain(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	overrides func(ralc *reservationAcceptanceLocalChain),
) *reservationAcceptanceLocalChain {
	t.Helper()

	ralc := newReservationAcceptanceLocalChain()
	ralc.SetReservationParameters(defaultTestReservationParameters())
	ralc.maxPerWalletAmount = 5000000
	ralc.maxSingleAmount = 5000000
	ralc.maxActive = 100

	ralc.SetDepositMinAge(testDepositMinAgeSeconds)
	ralc.SetAverageBlockTime(12 * time.Second)
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

// defaultTestReservationParameters returns the reservation parameters
// newBoundaryTestChain installs.
func defaultTestReservationParameters() tbtc.ReservationParameters {
	return tbtc.ReservationParameters{
		ReservationVault:          testReservationVaultAddress,
		ReservationMinAmount:      1000,
		ReservationTxMaxFee:       5000,
		MaxReservationsPerWallet:  5,
		ReservationMaxTotalAmount: 100000000,
		ReservationTermSeconds:    testReservationTermSeconds,
		ReservationActionTimeout:  24 * 60 * 60,
	}
}

// runTask runs a fresh acceptance task once for walletPublicKeyHash.
func runTask(
	t *testing.T,
	ralc *reservationAcceptanceLocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
	walletPublicKeyHash [20]byte,
) (*tbtc.ReservationAnchorProposal, bool, error) {
	t.Helper()

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	return runExistingTask(t, task, walletPublicKeyHash)
}

// runExistingTask runs the given acceptance task once for
// walletPublicKeyHash.
func runExistingTask(
	t *testing.T,
	task *tbtcpg.ReservationAcceptanceTask,
	walletPublicKeyHash [20]byte,
) (*tbtc.ReservationAnchorProposal, bool, error) {
	t.Helper()

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if proposal == nil {
		return nil, shouldExecute, err
	}
	anchorProposal, ok := proposal.(*tbtc.ReservationAnchorProposal)
	if !ok {
		t.Fatalf("expected *ReservationAnchorProposal, got %T", proposal)
	}
	return anchorProposal, shouldExecute, err
}

// expectProposalFor fails the test unless the run produced a proposal for
// the given deposit at the given nonce.
func expectProposalFor(
	t *testing.T,
	proposal *tbtc.ReservationAnchorProposal,
	shouldExecute bool,
	err error,
	deposit *pendingDeposit,
	requestNonce uint64,
) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf("expected a proposal, got shouldExecute=%v", shouldExecute)
	}
	if proposal.DepositFundingTxHash != deposit.fundingTxHash {
		t.Fatalf(
			"unexpected deposit funding tx hash\nexpected: %s\nactual:   %s",
			deposit.fundingTxHash.Hex(bitcoin.ReversedByteOrder),
			proposal.DepositFundingTxHash.Hex(bitcoin.ReversedByteOrder),
		)
	}
	if proposal.RequestNonce != requestNonce {
		t.Fatalf(
			"expected the proposal to carry the pending generation's nonce "+
				"[%d], got [%d]",
			requestNonce,
			proposal.RequestNonce,
		)
	}
}

// expectNoProposal fails the test unless the run was a clean no-op.
func expectNoProposal(
	t *testing.T,
	proposal *tbtc.ReservationAnchorProposal,
	shouldExecute bool,
	err error,
) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if shouldExecute || proposal != nil {
		t.Fatalf(
			"expected no proposal, got shouldExecute=%v proposal=%+v",
			shouldExecute,
			proposal,
		)
	}
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

var testWalletPublicKeyHash = hexToByte20(
	"8db50eb52063ea9d98b3eac91489a90f738986f6",
)

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
			hashesByLabel := registerReservedDeposits(t, scenario, ralc, btcChain)

			proposal, shouldExecute, err := runTask(
				t,
				ralc,
				btcChain,
				scenario.WalletPublicKeyHash,
			)
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

			if scenario.ExpectedAnchorProposal == nil {
				expectNoProposal(t, proposal, shouldExecute, nil)
				return
			}

			if !shouldExecute {
				t.Errorf("expected shouldExecute=true, got false")
			}
			if proposal == nil {
				t.Fatal("expected proposal, got nil")
			}

			expectedProposal := *scenario.ExpectedAnchorProposal
			realHash, ok := hashesByLabel[expectedProposal.DepositFundingTxHash]
			if !ok {
				t.Fatalf("expected proposal refers to an unknown deposit label")
			}
			expectedProposal.DepositFundingTxHash = realHash

			if !expectedAnchorsEqual(&expectedProposal, proposal) {
				t.Errorf(
					"invalid anchor proposal\nexpected: %+v\nactual:   %+v",
					expectedProposal,
					proposal,
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

	privateKey, err := ecdsa.GenerateKey(btcec.S256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(&privateKey.PublicKey)

	depositAmount := uint64(2000000)
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)
	deposit := addPendingDeposit(
		t,
		ralc,
		btcChain,
		walletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)

	anchorProposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
	expectProposalFor(t, anchorProposal, shouldExecute, err, deposit, 1)
	if anchorProposal.DepositFundingOutputIndex != 0 {
		t.Errorf(
			"unexpected DepositFundingOutputIndex\nexpected: 0\nactual:   %d",
			anchorProposal.DepositFundingOutputIndex,
		)
	}

	// Re-assemble and sign to verify transaction builder output properties.
	builder, err := tbtc.AssembleReservationAnchorTransaction(
		btcChain,
		deposit.deposit,
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
// no-op when no depositor requested acceptance by the wallet.
func TestReservationAcceptanceTask_NoCandidates(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, 300000, nil)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
}

// TestReservationAcceptanceTask_VaultNotConfigured_ZeroAddress verifies
// that a zero-address ReservationVault (the actual value the production
// chain.Address converter emits for an unset vault, never an empty
// string) is correctly treated as "not configured".
func TestReservationAcceptanceTask_VaultNotConfigured_ZeroAddress(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	// A request exists, but the vault is unset: the task must not look.
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	params := defaultTestReservationParameters()
	params.ReservationVault = chain.Address(
		"0x0000000000000000000000000000000000000000",
	)
	ralc.SetReservationParameters(params)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
	if len(ralc.acceptanceEventFilters) != 0 {
		t.Errorf("expected no acceptance request scan for an unset vault")
	}
}

// TestReservationAcceptanceTask_IgnoresRequestTimeCaps pins that the
// acceptance task, which consumes the depositor's Pending Acceptance
// generation rather than issuing its own request, does not re-apply the
// request-time capacity caps (wallet count, active count) that Solidity's
// requestReservationAcceptance already reserved when that generation was
// created. Re-applying them would double-count the very generation being
// consumed and block every otherwise eligible pending acceptance.
func TestReservationAcceptanceTask_IgnoresRequestTimeCaps(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	deposit := addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	// The wallet's own reservation count is at the cap (this pending
	// generation counts toward it) and the active count is at the
	// active cap: exactly the state Solidity's request-time
	// reserveAcceptanceCapacity leaves behind.
	ralc.SetWalletReservations(testWalletPublicKeyHash, []*big.Int{
		new(big.Int).SetInt64(7),
	})
	ralc.maxActive = 100
	ralc.activeCount = 100
	ralc.walletReservationsCount = 5

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)
}

// TestReservationAcceptanceTask_RequestLookBackWindow pins how far back
// acceptance requests are scanned. Solidity sets a generation's timeoutAt
// to request time + the action timeout, so a generation that can still be
// signed was requested within that timeout: the scan covers the on-chain
// action timeout in blocks plus a one-day margin, filtered by wallet and
// split into 10000-block chunks, and a request inside that window is
// proposed.
func TestReservationAcceptanceTask_RequestLookBackWindow(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	// 24-hour action timeout at 12 s per block is 7200 blocks; the
	// margin adds another 7200.
	expectedStartBlock := currentBlock - 14400

	opts := defaultPendingDepositOptions(currentBlock)
	opts.requestBlock = expectedStartBlock
	opts.revealBlock = expectedStartBlock - 100
	deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)

	// The 14400-block window is scanned in 10000-block chunks.
	expectedRanges := [][2]uint64{
		{expectedStartBlock, expectedStartBlock + 9999},
		{expectedStartBlock + 10000, currentBlock},
	}
	if len(ralc.acceptanceEventFilters) != len(expectedRanges) {
		t.Fatalf(
			"expected %d acceptance request scan chunks, got %d",
			len(expectedRanges),
			len(ralc.acceptanceEventFilters),
		)
	}
	for i, expected := range expectedRanges {
		filter := ralc.acceptanceEventFilters[i]
		if filter.StartBlock != expected[0] ||
			filter.EndBlock == nil || *filter.EndBlock != expected[1] {
			t.Fatalf(
				"unexpected scan chunk %d: start=%d end=%v, expected [%d, %d]",
				i,
				filter.StartBlock,
				filter.EndBlock,
				expected[0],
				expected[1],
			)
		}
	}
	filter := ralc.acceptanceEventFilters[0]
	if len(filter.WalletPublicKeyHash) != 1 ||
		filter.WalletPublicKeyHash[0] != testWalletPublicKeyHash {
		t.Fatalf("expected the scan to be filtered by the wallet")
	}
}

// TestReservationAcceptanceTask_RevealLongBeforeRequest is a regression
// test for acceptance requests made long after the reveal. A depositor may
// request acceptance up to one term after revealing, so a deposit revealed
// 300000 blocks (about 41 days) before its request must be found and
// proposed. The reveal sits exactly on the oldest block of a 10000-block
// lookup chunk to catch off-by-one errors at chunk edges.
func TestReservationAcceptanceTask_RevealLongBeforeRequest(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(1000000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	opts := defaultPendingDepositOptions(currentBlock)
	opts.requestBlock = currentBlock - 10
	// Chunks run backward from the request block: the 30th covers
	// [requestBlock - 300000 + 1, requestBlock - 290000].
	opts.revealBlock = opts.requestBlock - 300000 + 1
	opts.revealedAt = time.Now().Add(-300000 * 12 * time.Second)
	deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)
}

// TestReservationAcceptanceTask_DepositNotRoutedToReservationVault
// confirms that a candidate whose reveal routed the deposit to another
// vault is skipped.
func TestReservationAcceptanceTask_DepositNotRoutedToReservationVault(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	otherVault := chain.Address("0xOtherVaultAddress1234567890abcdef123456789012")
	opts := defaultPendingDepositOptions(currentBlock)
	opts.revealVault = &otherVault
	addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
	if ralc.validateCalls != 0 {
		t.Errorf("expected no validation, got %d calls", ralc.validateCalls)
	}
}

// TestReservationAcceptanceTask_GetWalletError exercises the GetWallet
// error propagation: the candidate wallet's chain data fails to load and
// the RPC failure is propagated instead of being masked as "no eligible
// candidate", so the coordinator can retry.
func TestReservationAcceptanceTask_GetWalletError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
		ralc.getWalletErr = fmt.Errorf("boom")
	})
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
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
	if shouldExecute || proposal != nil {
		t.Errorf("expected no proposal")
	}
}

// TestReservationAcceptanceTask_GetReservationError verifies the fail-safe
// policy: since the production chain adapter never errors for "not found"
// (it returns a zero record with State == Unknown), a GetReservation error
// can only be an RPC/decode failure, and the task must skip the affected
// deposit for this window rather than fail open.
func TestReservationAcceptanceTask_GetReservationError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	ralc.getReservationErr = fmt.Errorf("simulated get reservation error")

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
}

// TestReservationAcceptanceTask_Stateless_Maturity verifies that an
// immature candidate is skipped on one run, and proposed by a later run of
// the same task instance once it matures, without cache interference.
func TestReservationAcceptanceTask_Stateless_Maturity(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	// Revealed only 10 minutes ago (DEPOSIT_MIN_AGE is 2 hours).
	opts := defaultPendingDepositOptions(currentBlock)
	opts.revealedAt = time.Now().Add(-10 * time.Minute)
	deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := runExistingTask(t, task, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)

	// Time passes: the deposit was revealed 3 hours ago.
	ralc.SetDepositRequest(deposit.fundingTxHash, 0, &tbtc.DepositChainRequest{
		Depositor:  testDepositor,
		Amount:     opts.amount,
		RevealedAt: time.Now().Add(-3 * time.Hour),
		SweptAt:    time.Unix(0, 0),
		Vault:      &[]chain.Address{testReservationVaultAddress}[0],
	})

	proposal, shouldExecute, err = runExistingTask(t, task, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)
}

// TestReservationAcceptanceTask_ReservationParametersFetchedLive verifies
// that each Run() call reflects the chain's current live state, and that
// the minimum-amount gate uses the generation's snapshotted minimum rather
// than the live ReservationMinAmount: a live raise that crosses the
// deposit does not retroactively invalidate a pending generation whose own
// snapshot still clears it.
func TestReservationAcceptanceTask_ReservationParametersFetchedLive(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	deposit := addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := runExistingTask(t, task, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)

	// Raise the live ReservationMinAmount above the deposit's value. The
	// snapshotted MinAmount on the seeded action (1000) is unchanged, so
	// the deposit must still pass the gate on the second run.
	params := defaultTestReservationParameters()
	params.ReservationMinAmount = 3000000
	ralc.SetReservationParameters(params)

	proposal, shouldExecute, err = runExistingTask(t, task, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)
}

// TestReservationAcceptanceTask_BoundaryChecks exercises explicit
// at-limit/one-over-limit boundary crossings for the snapshotted minimum
// the acceptance candidate enforces against its own action record
// (depositAmount >= MinAmount + anchor fee). A generation below that bound
// cannot be created on-chain (requestReservationAcceptance requires
// amount >= MinAmount + txMaxFee and the anchor fee never exceeds
// txMaxFee), so the rejecting rows pin a defensive check. The
// request-time capacity caps are not re-checked here: Solidity's
// requestReservationAcceptance already reserved them.
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
		// The fixture's 1 sat/vByte oracle rate is clamped by
		// applyWalletTxFeeFloor (see fee.go) to 5 sat/vByte, a 710 sat
		// fee (testAnchorFeeSat). Deposit amounts are offset by
		// testAnchorFeeSat to test the exact net-of-fee boundary.
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
			btcChain := tbtcpg.NewLocalBitcoinChain()
			currentBlock := uint64(300000)

			ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, func(ralc *reservationAcceptanceLocalChain) {
				params := defaultTestReservationParameters()
				params.ReservationMinAmount = test.reservationMinAmount
				params.MaxReservationsPerWallet = test.maxReservationsPerWallet
				params.ReservationMaxTotalAmount = test.reservationMaxTotal
				params.ReservationTotalAmount = test.reservationTotal
				ralc.SetReservationParameters(params)
				ralc.maxPerWalletAmount = 50000000
				if test.maxPerWalletAmount != nil {
					ralc.maxPerWalletAmount = *test.maxPerWalletAmount
				}
				ralc.maxSingleAmount = 50000000
				if test.maxSingleAmount != nil {
					ralc.maxSingleAmount = *test.maxSingleAmount
				}
				if test.maxActive != nil {
					ralc.maxActive = *test.maxActive
				}
				ralc.activeCount = test.activeCount
				ralc.walletReservationsAmount = test.walletReservationsAmount
				ralc.walletReservationsCount = test.walletReservationsCount
			})

			opts := defaultPendingDepositOptions(currentBlock)
			opts.amount = test.depositAmount
			opts.minAmount = test.reservationMinAmount
			addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

			_, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
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
// "no double-acceptance" cases: the acceptance task consumes, rather than
// creates, the depositor's request record, so a double proposal can only
// happen if it builds a proposal for a generation it may not anchor:
//   - a Pending Acceptance action that targets another wallet;
//   - a Settled (or otherwise non-Pending) action at the current nonce.
func TestReservationAcceptanceTask_Stateless_NoReRequest(t *testing.T) {
	otherWalletPublicKeyHash := hexToByte20(
		"7c1c4dbaaf8d75b08f9ea8e3f9a2c3a98e1f0d77",
	)
	currentBlock := uint64(300000)

	tests := map[string]func(action *tbtc.ReservationAction){
		"pending acceptance targeting another wallet is not consumed": func(action *tbtc.ReservationAction) {
			action.TargetWalletPublicKeyHash = otherWalletPublicKeyHash
		},
		"settled generation is not re-anchored": func(action *tbtc.ReservationAction) {
			action.State = tbtc.ReservationActionStateSettled
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()
			ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
			deposit := addPendingDeposit(
				t,
				ralc,
				btcChain,
				testWalletPublicKeyHash,
				defaultPendingDepositOptions(currentBlock),
			)
			action, err := ralc.GetReservationAction(deposit.depositKey, 1)
			if err != nil {
				t.Fatal(err)
			}
			mutated := *action
			mutate(&mutated)
			ralc.SetReservationAction(deposit.depositKey, 1, &mutated)

			proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
			expectNoProposal(t, proposal, shouldExecute, err)
		})
	}
}

// TestReservationAcceptanceTask_Stateless_NonEligibleReservationState
// verifies that a reservation whose on-chain state is Active,
// ActionPending, Closed, or Stranded, with no pending acceptance action
// at its current nonce, is skipped from acceptance proposals.
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
			currentBlock := uint64(300000)

			ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
			opts := defaultPendingDepositOptions(currentBlock)
			opts.withoutAction = true
			deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)
			ralc.SetReservation(deposit.depositKey, &tbtc.Reservation{
				State:        state,
				RequestNonce: 2,
			})

			proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
			expectNoProposal(t, proposal, shouldExecute, err)
		})
	}
}

// TestReservationAcceptanceTask_Stateless_DynamicMinAmount verifies that
// each generation of a reservation carries its own snapshotted minimum
// amount: a deposit whose first generation's snapshot is above the
// deposit is skipped for that generation, but a fresh generation written
// after governance lowered the live minimum becomes eligible and is
// consumed at its own real nonce.
func TestReservationAcceptanceTask_Stateless_DynamicMinAmount(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	// First generation: snapshot min (5,000,000) is above the deposit
	// (2,000,000); the candidate must be skipped.
	opts := defaultPendingDepositOptions(currentBlock)
	opts.minAmount = 5000000
	deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)

	proposal, shouldExecute, err := runExistingTask(t, task, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)

	// The depositor issues a fresh request: the reservation's nonce is
	// bumped, a new action record carries the new snapshot, and a new
	// ReservationAcceptanceRequested event is emitted.
	timeoutAt := uint32(time.Now().Add(24 * time.Hour).Unix())
	ralc.SetReservation(deposit.depositKey, &tbtc.Reservation{
		WalletPublicKeyHash: testWalletPublicKeyHash,
		State:               tbtc.ReservationStateUnknown,
		RequestNonce:        2,
	})
	ralc.SetReservationAction(deposit.depositKey, 2, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeAcceptance,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: testWalletPublicKeyHash,
		TxMaxFee:                  5000,
		MinAmount:                 1000000,
		TermSeconds:               testReservationTermSeconds,
		TimeoutAt:                 timeoutAt,
	})
	ralc.acceptanceEvents = append(ralc.acceptanceEvents, &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      deposit.depositKey,
		RequestNonce:        2,
		WalletPublicKeyHash: testWalletPublicKeyHash,
		DepositAmount:       2000000,
		TxMaxFee:            5000,
		TimeoutAt:           timeoutAt,
		BlockNumber:         currentBlock - 5,
	})

	proposal, shouldExecute, err = runExistingTask(t, task, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, 2)
}

// TestReservationAcceptanceTask_AcceptanceRequestedEventsError verifies
// that a failure to fetch the wallet's acceptance requests aborts the Run
// with an error, rather than being reported as "no candidate".
func TestReservationAcceptanceTask_AcceptanceRequestedEventsError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	ralc.acceptanceEventsErr = fmt.Errorf("simulated rpc failure")

	_, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	if err == nil {
		t.Fatalf("expected a non-nil error, got nil")
	}
	if shouldExecute {
		t.Errorf("expected shouldExecute=false, got true")
	}
}

// TestReservationAcceptanceTask_RevealLookupErrorSkipsCandidate verifies
// that a failed DepositRevealed lookup for one candidate skips that
// candidate for the window instead of aborting the Run.
func TestReservationAcceptanceTask_RevealLookupErrorSkipsCandidate(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	ralc.pastDepositRevealedEventsErr = fmt.Errorf("simulated rpc failure")

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
}

// TestReservationAcceptanceTask_ValidateProposalError verifies that a
// ValidateReservationAnchorProposal failure skips the candidate rather
// than aborting the coordination window: with no other candidate, Run
// reports a clean no-op instead of an error.
func TestReservationAcceptanceTask_ValidateProposalError(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	addPendingDeposit(
		t,
		ralc,
		btcChain,
		testWalletPublicKeyHash,
		defaultPendingDepositOptions(currentBlock),
	)
	ralc.validateErr = fmt.Errorf("simulated validation failure")

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
	if ralc.validateCalls != 1 {
		t.Errorf("expected the validator to be called once, got %d", ralc.validateCalls)
	}
}

// strictValidatorFixture seeds a bare LocalChain with everything its
// strict anchor validator checks for a reserved deposit revealed to
// walletPublicKeyHash, with a Pending Acceptance action at nonce 2, and
// returns a valid proposal and its deposit extra info.
func strictValidatorFixture(
	t *testing.T,
	lc *tbtcpg.LocalChain,
	walletPublicKeyHash [20]byte,
) (
	*tbtc.ReservationAnchorProposal,
	struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
	*big.Int,
) {
	t.Helper()

	vault := testReservationVaultAddress
	deposit, fundingTx := buildReservedDeposit(t, walletPublicKeyHash, 2000000, 0x33, vault)
	fundingTxHash := fundingTx.Hash()
	depositKey := lc.BuildDepositKey(fundingTxHash, 0)

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
		TermSeconds:               testReservationTermSeconds,
		TimeoutAt:                 uint32(time.Now().Add(24 * time.Hour).Unix()),
	})
	lc.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault: vault,
	})
	lc.SetDepositRequest(fundingTxHash, 0, &tbtc.DepositChainRequest{
		Depositor:  testDepositor,
		Amount:     2000000,
		RevealedAt: time.Now().Add(-48 * time.Hour),
		SweptAt:    time.Unix(0, 0),
		Vault:      &vault,
	})
	lc.SetReservedDeposit(depositKey, true)
	lc.SetDepositMinAge(testDepositMinAgeSeconds)

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      fundingTxHash,
		DepositFundingOutputIndex: 0,
		RequestNonce:              2,
		AnchorTxFee:               big.NewInt(710),
	}
	extraInfo := struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	}{Deposit: deposit, FundingTx: fundingTx}

	return proposal, extraInfo, depositKey
}

// TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate is a
// regression test for the action-record precondition
// WalletProposalValidator.sol's validateReservationAnchorProposal enforces:
// the action at the proposal's RequestNonce must already be a Pending
// Acceptance action targeting this wallet. A proposal at the seeded nonce
// passes, while nonce + 1 is rejected.
func TestLocalChain_ValidateReservationAnchorProposal_StrictNonceGate(t *testing.T) {
	lc := tbtcpg.NewLocalChain()
	proposal, extraInfo, _ := strictValidatorFixture(t, lc, testWalletPublicKeyHash)

	if err := lc.ValidateReservationAnchorProposal(
		testWalletPublicKeyHash,
		proposal,
		extraInfo,
	); err != nil {
		t.Fatalf(
			"strict fake rejected a proposal at the seeded action's nonce "+
				"(2): %v",
			err,
		)
	}

	proposalOff := *proposal
	proposalOff.RequestNonce = 3
	if err := lc.ValidateReservationAnchorProposal(
		testWalletPublicKeyHash,
		&proposalOff,
		extraInfo,
	); err == nil {
		t.Fatalf(
			"strict fake accepted a proposal at nonce + 1 -- the " +
				"validator's action-record precondition must reject any " +
				"nonce whose action record is missing",
		)
	}
}

// TestLocalChain_ValidateReservationAnchorProposal_DepositExtraInfo pins
// the strict fake's mirror of validateDepositExtraInfo: a funding
// transaction that does not hash to the proposal's funding transaction
// hash, or whose output does not carry the deposit script rebuilt from
// the extra info's reveal fields, is rejected.
func TestLocalChain_ValidateReservationAnchorProposal_DepositExtraInfo(t *testing.T) {
	t.Run("another funding transaction", func(t *testing.T) {
		lc := tbtcpg.NewLocalChain()
		proposal, extraInfo, _ := strictValidatorFixture(t, lc, testWalletPublicKeyHash)
		_, otherFundingTx := buildReservedDeposit(
			t,
			testWalletPublicKeyHash,
			2000000,
			0x44,
			testReservationVaultAddress,
		)
		extraInfo.FundingTx = otherFundingTx

		err := lc.ValidateReservationAnchorProposal(
			testWalletPublicKeyHash,
			proposal,
			extraInfo,
		)
		if err == nil || err.Error() != "extra info funding tx hash does not match" {
			t.Fatalf("expected a funding tx hash mismatch, got [%v]", err)
		}
	})

	t.Run("reveal fields that do not rebuild the locking script", func(t *testing.T) {
		lc := tbtcpg.NewLocalChain()
		proposal, extraInfo, _ := strictValidatorFixture(t, lc, testWalletPublicKeyHash)
		wrongDeposit := *extraInfo.Deposit
		wrongDeposit.BlindingFactor = [8]byte{0xff}
		extraInfo.Deposit = &wrongDeposit

		err := lc.ValidateReservationAnchorProposal(
			testWalletPublicKeyHash,
			proposal,
			extraInfo,
		)
		if err == nil || err.Error() != "extra info funding output script does not match" {
			t.Fatalf("expected a funding output script mismatch, got [%v]", err)
		}
	})
}

// TestReservationAcceptanceTask_DepositWithoutPendingActionIsSkipped is a
// regression test for the action-record gate: a requested deposit whose
// current generation has no Pending Acceptance action -- because it timed
// out, settled, or its record is gone -- is never proposed.
func TestReservationAcceptanceTask_DepositWithoutPendingActionIsSkipped(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
	opts := defaultPendingDepositOptions(currentBlock)
	opts.withoutAction = true
	addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)
}

// TestReservationAcceptanceTask_SkipsCandidateInsideTimeoutSafetyMargin is a
// regression test for the timeout safety margin gate: a candidate whose
// pending acceptance generation has TimeoutAt at or before
// now + REQUEST_TIMEOUT_SAFETY_MARGIN (2 hours) is skipped before any
// proposal is validated. The request event's own timeout drops such a
// generation without any chain read; the action record, which is
// authoritative, is checked again. A candidate comfortably outside the
// margin is proposed.
func TestReservationAcceptanceTask_SkipsCandidateInsideTimeoutSafetyMargin(t *testing.T) {
	currentBlock := uint64(300000)

	t.Run("inside margin: skipped without chain reads", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()
		ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
		opts := defaultPendingDepositOptions(currentBlock)
		opts.timeoutAt = uint32(time.Now().Add(7198 * time.Second).Unix())
		addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
		expectNoProposal(t, proposal, shouldExecute, err)
		if ralc.getReservationCalls != 0 {
			t.Errorf(
				"expected no reservation read for a request inside the "+
					"margin, got %d",
				ralc.getReservationCalls,
			)
		}
		if ralc.validateCalls != 0 {
			t.Errorf("expected no validation, got %d calls", ralc.validateCalls)
		}
	})

	t.Run("action record inside margin: skipped", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()
		ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
		deposit := addPendingDeposit(
			t,
			ralc,
			btcChain,
			testWalletPublicKeyHash,
			defaultPendingDepositOptions(currentBlock),
		)
		action, err := ralc.GetReservationAction(deposit.depositKey, 1)
		if err != nil {
			t.Fatal(err)
		}
		insideMargin := *action
		insideMargin.TimeoutAt = uint32(time.Now().Add(7198 * time.Second).Unix())
		ralc.SetReservationAction(deposit.depositKey, 1, &insideMargin)

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
		expectNoProposal(t, proposal, shouldExecute, err)
		if ralc.validateCalls != 0 {
			t.Errorf("expected no validation, got %d calls", ralc.validateCalls)
		}
	})

	t.Run("outside margin: proposed", func(t *testing.T) {
		btcChain := tbtcpg.NewLocalBitcoinChain()
		ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)
		opts := defaultPendingDepositOptions(currentBlock)
		// 100 seconds of slack past the 7200-second margin, so a slow
		// host cannot push the candidate inside it.
		opts.timeoutAt = uint32(time.Now().Add(7300 * time.Second).Unix())
		deposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
		expectProposalFor(t, proposal, shouldExecute, err, deposit, 1)
		if ralc.validateCalls != 1 {
			t.Errorf("expected one validation, got %d calls", ralc.validateCalls)
		}
	})
}

// TestLocalChain_ValidateReservationAnchorProposal_RejectsInsideTimeoutMargin
// is a regression test for the timeout safety margin gate in the LocalChain
// validator fake: a proposal whose action TimeoutAt is at or before
// now + REQUEST_TIMEOUT_SAFETY_MARGIN is rejected.
func TestLocalChain_ValidateReservationAnchorProposal_RejectsInsideTimeoutMargin(t *testing.T) {
	lc := tbtcpg.NewLocalChain()
	proposal, extraInfo, depositKey := strictValidatorFixture(t, lc, testWalletPublicKeyHash)

	// Far-future TimeoutAt: validation passes.
	if err := lc.ValidateReservationAnchorProposal(
		testWalletPublicKeyHash,
		proposal,
		extraInfo,
	); err != nil {
		t.Fatalf("far-future TimeoutAt should pass: %v", err)
	}

	action, err := lc.GetReservationAction(depositKey, 2)
	if err != nil {
		t.Fatal(err)
	}
	insideMargin := *action
	insideMargin.TimeoutAt = uint32(time.Now().Add(7198 * time.Second).Unix())
	lc.SetReservationAction(depositKey, 2, &insideMargin)

	if err := lc.ValidateReservationAnchorProposal(
		testWalletPublicKeyHash,
		proposal,
		extraInfo,
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

	// Seed the reservation record the source wallet custodies so the
	// fake's custody check and the rest of the validator's
	// preconditions hold; only the timeout margin is under test.
	lc.SetReservation(resKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		State:               tbtc.ReservationStateActive,
		RequestNonce:        0,
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

// TestReservationAcceptanceTask_BudgetCountsOnlyPendingAcceptances is a
// regression test for budget starvation: the per-run candidate budget
// (50) must only be spent on generations the on-chain check confirms are
// a Pending Acceptance targeting this wallet. Here 51 requested deposits
// whose generations have already settled sort ahead of one real pending
// acceptance; if they consumed the budget, the real request would never
// be reached and would time out.
func TestReservationAcceptanceTask_BudgetCountsOnlyPendingAcceptances(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	// Candidates are tried in timeout order, so the settled generations
	// get the earlier timeouts. They were accepted before their timeout,
	// so their request events are still inside the scan window.
	for i := 0; i < 51; i++ {
		opts := defaultPendingDepositOptions(currentBlock)
		opts.seed = byte(0x40 + i)
		opts.timeoutAt = uint32(time.Now().Add(20 * time.Hour).Unix())
		opts.actionState = tbtc.ReservationActionStateSettled
		addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, opts)
	}
	realOpts := defaultPendingDepositOptions(currentBlock)
	realOpts.seed = 0x01
	realOpts.timeoutAt = uint32(time.Now().Add(23 * time.Hour).Unix())
	real := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, realOpts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, real, 1)
}

// TestReservationAcceptanceTask_FeeErrorSkipsCandidate is a regression test
// for per-generation fee caps: a generation whose snapshotted max fee is
// below the current anchor fee estimate is skipped, and the next
// generation, with a higher cap, is proposed instead of the whole window
// aborting.
func TestReservationAcceptanceTask_FeeErrorSkipsCandidate(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	lowCap := defaultPendingDepositOptions(currentBlock)
	lowCap.seed = 0x10
	lowCap.txMaxFee = 500 // below the 710 sat estimate
	lowCap.timeoutAt = uint32(time.Now().Add(20 * time.Hour).Unix())
	addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, lowCap)

	highCap := defaultPendingDepositOptions(currentBlock)
	highCap.seed = 0x11
	highCap.timeoutAt = uint32(time.Now().Add(23 * time.Hour).Unix())
	viable := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, highCap)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, viable, 1)
}

// gaugeRecorder counts SetGauge calls per gauge.
type gaugeRecorder struct {
	calls map[string]int
}

func (g *gaugeRecorder) SetGauge(name string, value float64) {
	g.calls[name]++
}

// TestReservationAcceptanceTask_GaugesPublishedOncePerRun pins the cost of
// the vault fee gauges: they are read once per Run, with the vault address
// the task already read from the reservation parameters, even when an
// earlier candidate fails validation and the task moves on to the next.
func TestReservationAcceptanceTask_GaugesPublishedOncePerRun(t *testing.T) {
	btcChain := tbtcpg.NewLocalBitcoinChain()
	currentBlock := uint64(300000)

	ralc := newBoundaryTestChain(t, testWalletPublicKeyHash, currentBlock, nil)

	// The first candidate carries a funding transaction whose output does
	// not hold its deposit script, so the strict validator rejects it.
	broken := defaultPendingDepositOptions(currentBlock)
	broken.seed = 0x20
	broken.timeoutAt = uint32(time.Now().Add(20 * time.Hour).Unix())
	brokenDeposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, broken)
	for _, event := range ralc.revealEvents {
		if event.FundingTxHash == brokenDeposit.fundingTxHash {
			event.BlindingFactor = [8]byte{0xff}
		}
	}

	good := defaultPendingDepositOptions(currentBlock)
	good.seed = 0x21
	good.timeoutAt = uint32(time.Now().Add(23 * time.Hour).Unix())
	goodDeposit := addPendingDeposit(t, ralc, btcChain, testWalletPublicKeyHash, good)

	task := tbtcpg.NewReservationAcceptanceTask(ralc, btcChain)
	recorder := &gaugeRecorder{calls: make(map[string]int)}
	task.SetMetricsRecorderForTest(recorder)

	proposal, shouldExecute, err := runExistingTask(t, task, testWalletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, goodDeposit, 1)

	if ralc.validateCalls != 2 {
		t.Fatalf("expected both candidates to be validated, got %d", ralc.validateCalls)
	}
	if len(ralc.feeDebtVaults) != 1 || len(ralc.feeReserveVaults) != 1 {
		t.Fatalf(
			"expected one read per vault fee gauge, got debt=%d reserve=%d",
			len(ralc.feeDebtVaults),
			len(ralc.feeReserveVaults),
		)
	}
	if ralc.feeDebtVaults[0] != testReservationVaultAddress ||
		ralc.feeReserveVaults[0] != testReservationVaultAddress {
		t.Fatalf(
			"expected the fee reads to use the parameters' vault, got %v / %v",
			ralc.feeDebtVaults,
			ralc.feeReserveVaults,
		)
	}
	for _, gauge := range []string{
		"wallet_reservations_count",
		"active_reservations_count",
		"max_active_reservations",
		"reservation_vault_fee_debt_sat",
		"reservation_vault_fee_reserve_tbtc_base_units",
	} {
		if recorder.calls[gauge] != 1 {
			t.Errorf("expected gauge %s published once, got %d", gauge, recorder.calls[gauge])
		}
	}
}
