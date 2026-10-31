package spv

import (
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Behavior tests written against the go-mutesting survivor list for the
// two keeper observer files. They reuse the fixtures and fakes from
// reservation_keeper_observer_test.go and chain_test.go, and are named so
// the existing TestReservationKeeperObserver_ filter runs them.

// mutationObserverChain wraps the observer's fixture chain and injects
// errors into the chain reads the localChain has no error-injection
// field for: the reservation parameters, the wallet parameters, and the
// two event-fetch methods the observer's incremental scans issue.
type mutationObserverChain struct {
	*localChain
	reservationParametersErr error
	walletParametersErr      error
	acceptanceEventsErr      error
	revealEventsErr          error
}

func (c *mutationObserverChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	if c.reservationParametersErr != nil {
		return nil, c.reservationParametersErr
	}
	return c.localChain.ReservationParameters()
}

func (c *mutationObserverChain) GetWalletParameters() (
	tbtc.WalletParameters,
	error,
) {
	if c.walletParametersErr != nil {
		return tbtc.WalletParameters{}, c.walletParametersErr
	}
	return c.localChain.GetWalletParameters()
}

func (c *mutationObserverChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if c.acceptanceEventsErr != nil {
		return nil, c.acceptanceEventsErr
	}
	return c.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

func (c *mutationObserverChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	if c.revealEventsErr != nil {
		return nil, c.revealEventsErr
	}
	return c.localChain.PastDepositRevealedEvents(filter)
}

// walletCountingChain wraps the fixture chain and counts the GetWallet
// calls the wallet-state refresh issues, so a test can assert which
// cache entries a pass re-read.
type walletCountingChain struct {
	*localChain
	walletFetches int
}

func (c *walletCountingChain) GetWallet(walletPublicKeyHash [20]byte) (
	*tbtc.WalletChainData,
	error,
) {
	c.walletFetches++
	return c.localChain.GetWallet(walletPublicKeyHash)
}

// setGetReservationErr installs a chain-wide error for GetReservation.
func (lc *localChain) setGetReservationErr(err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.getReservationErr = err
}

// confirmedTip returns the endBlock a reservation scan stops at for a
// given current block (the current block minus the confirmation lag).
func confirmedTip(currentBlock uint64) uint64 {
	if currentBlock <= reservationEventScanConfirmationBlocks {
		return 0
	}

	return currentBlock - reservationEventScanConfirmationBlocks
}

// newObserverForTest builds a fresh observer over the fixture chain with
// the given activation block and recorder.
func newObserverForTest(
	spvChain *localChain,
	activationBlock uint64,
	recorder MetricsRecorder,
) *ReservationKeeperObserver {
	return NewReservationKeeperObserver(
		spvChain,
		activationBlock,
		recorder,
	)
}
