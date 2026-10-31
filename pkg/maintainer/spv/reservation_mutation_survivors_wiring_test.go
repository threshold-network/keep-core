package spv

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"

	log2 "github.com/ipfs/go-log/v2"
)

// TestReservationOperatorStaggerOffset_MutationSurvivors_Derivation pins the
// stagger offset derivation: the offset is keccak256(operator || key)
// reduced modulo the interval in whole seconds, so it is spread across
// [0, interval) and disabled for an interval that is shorter than one
// second.
func TestReservationOperatorStaggerOffset_MutationSurvivors_Derivation(
	t *testing.T,
) {
	operatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000c0")
	const interval = 10 * time.Minute
	intervalSeconds := uint64(interval / time.Second)

	t.Run("sub-second interval disables staggering", func(t *testing.T) {
		if offset := reservationOperatorStaggerOffset(
			operatorAddress,
			"unique-key",
			500*time.Millisecond,
		); offset != 0 {
			t.Fatalf("expected a sub-second interval to disable staggering, got %d", offset)
		}
	})

	t.Run("positive interval reduces the hash modulo the interval", func(t *testing.T) {
		// Pick a key whose derived offset is non-zero, so a mutant that
		// disables staggering outright cannot pass.
		uniqueKey := "unique-key"
		var expected uint64
		for i := 0; expected == 0; i++ {
			uniqueKey = "unique-key-" + string(rune('a'+i))
			hash := crypto.Keccak256(
				append(operatorAddress.Bytes(), []byte(uniqueKey)...),
			)
			expected = binary.BigEndian.Uint64(hash[:8]) % intervalSeconds
		}

		got := reservationOperatorStaggerOffset(operatorAddress, uniqueKey, interval)
		if uint64(got) != expected {
			t.Fatalf(
				"expected offset %d (hash modulo the %ds interval), got %d",
				expected,
				intervalSeconds,
				got,
			)
		}
	})
}

// transientGetWalletChain fails GetWallet for one wallet a bounded number
// of times before delegating, so the startup scan sees a transient error
// that clears within its three attempts.
type transientGetWalletChain struct {
	*localChain
	wallet       [20]byte
	failuresLeft atomic.Int32
}

func (c *transientGetWalletChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	if walletPublicKeyHash == c.wallet && c.failuresLeft.Load() > 0 {
		c.failuresLeft.Add(-1)
		return nil, errors.New("transient RPC failure")
	}
	return c.localChain.GetWallet(walletPublicKeyHash)
}

// TestWireReservationWatchers_MutationSurvivors_TransientWalletErrorIsRetried
// pins the startup scan's GetWallet retry: a wallet read that fails once
// and then succeeds must be checked in the same startup scan, not queued
// for the 30-second second-chance pass. Giving up on the first failure
// delays a Closed wallet's stranding notifications by the whole retry
// delay.
func TestWireReservationWatchers_MutationSurvivors_TransientWalletErrorIsRetried(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(5000)
	base.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x21)
	reservationKey := reservationKey(0xC100)

	base.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x21},
		WalletPublicKeyHash: wallet,
	})
	base.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	base.setWalletReservations(wallet, []*big.Int{reservationKey})
	base.setReservation(reservationKey, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	chain := &transientGetWalletChain{localChain: base, wallet: wallet}
	chain.failuresLeft.Store(2)

	if err := WireReservationWatchers(
		ctx,
		&mockWalletClosedChain{},
		chain,
		true,
		nil,
		ethereum.Unknown,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notified := base.getSubmittedReservationStrandedKeys()
	if len(notified) != 1 {
		t.Fatalf(
			"expected the wallet whose read failed twice to be checked in "+
				"the startup scan, got %d notifications",
			len(notified),
		)
	}
	if notified[0].Cmp(reservationKey) != 0 {
		t.Errorf("unexpected notified key: %v", notified[0])
	}
}

// acceptanceScanErrorChain makes the action-timeout watcher's initial
// acceptance-request scan fail.
type acceptanceScanErrorChain struct {
	*localChain
}

func (c *acceptanceScanErrorChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	return nil, errors.New("RPC failure")
}

// spvLogCapture collects the messages the spv package logger emits while
// the test runs, so tests can wait for an asynchronous log line.
type spvLogCapture struct {
	mutex    sync.Mutex
	messages []string
}

func startSpvLogCapture(t *testing.T) *spvLogCapture {
	t.Helper()

	for _, loggerName := range []string{
		"keep-maintainer-spv",
		"keep-maintainer-spv-reservations",
		"keep-maintainer-spv-reservations-keeper-observer",
	} {
		if err := log2.SetLogLevel(loggerName, "debug"); err != nil {
			t.Fatal(err)
		}
	}

	pipe := log2.NewPipeReader()
	capture := &spvLogCapture{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		decoder := json.NewDecoder(pipe)
		for {
			var entry struct {
				Logger  string `json:"logger"`
				Message string `json:"msg"`
			}
			if err := decoder.Decode(&entry); err != nil {
				return
			}
			if !strings.HasPrefix(entry.Logger, "keep-maintainer-spv") {
				continue
			}
			capture.mutex.Lock()
			capture.messages = append(capture.messages, entry.Message)
			capture.mutex.Unlock()
		}
	}()

	t.Cleanup(func() {
		if err := pipe.Close(); err != nil {
			t.Logf("could not close log pipe reader: %v", err)
		}
		<-done
	})

	return capture
}

func (c *spvLogCapture) contains(message string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, captured := range c.messages {
		if strings.Contains(captured, message) {
			return true
		}
	}
	return false
}

// count returns how many captured messages contain message.
func (c *spvLogCapture) count(message string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	count := 0
	for _, captured := range c.messages {
		if strings.Contains(captured, message) {
			count++
		}
	}
	return count
}

// waitForCount waits, bounded, until at least want captured messages
// contain message.
func (c *spvLogCapture) waitForCount(
	t *testing.T,
	message string,
	want int,
	timeout time.Duration,
) bool {
	t.Helper()

	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if c.count(message) >= want {
			return true
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return c.count(message) >= want
		}
	}
}

// waitFor waits, bounded, for a captured message containing message.
func (c *spvLogCapture) waitFor(
	t *testing.T,
	message string,
	timeout time.Duration,
) bool {
	t.Helper()

	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if c.contains(message) {
			return true
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return c.contains(message)
		}
	}
}

const selfCheckMisconfiguredMessage = "reservation watchers wired but found zero reservation activity"

// TestWireReservationWatchers_MutationSurvivors_FailedScanIsNotMisconfiguration
// pins the self-check's tri-state handling of each scan: a scan that
// errored makes its zero count unreliable, so it must not be reported as
// a confirmed misconfiguration. Only an all-scans-succeeded zero-activity
// signal is a misconfiguration.
func TestWireReservationWatchers_MutationSurvivors_FailedScanIsNotMisconfiguration(
	t *testing.T,
) {
	newChain := func() *localChain {
		spvChain := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)
		return spvChain
	}

	t.Run("positive control: successful zero-activity scans report", func(t *testing.T) {
		capture := startSpvLogCapture(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			newChain(),
			false,
			nil,
			ethereum.Developer,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !capture.waitFor(t, selfCheckMisconfiguredMessage, 5*time.Second) {
			t.Fatal(
				"expected the misconfiguration self-check to report a " +
					"confirmed zero-activity wiring",
			)
		}
	})

	t.Run("failed action-timeout scan is not a confirmed misconfiguration", func(t *testing.T) {
		capture := startSpvLogCapture(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		spvChain := &acceptanceScanErrorChain{localChain: newChain()}
		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			spvChain,
			false,
			nil,
			ethereum.Developer,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capture.waitFor(t, selfCheckMisconfiguredMessage, 500*time.Millisecond) {
			t.Fatal(
				"expected a failed action-timeout scan to keep the " +
					"misconfiguration self-check silent",
			)
		}
	})

	t.Run("failed registration scan is not a confirmed misconfiguration", func(t *testing.T) {
		capture := startSpvLogCapture(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		spvChain := newChain()
		spvChain.setPastNewWalletRegisteredEventsErr(errors.New("RPC failure"))
		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			spvChain,
			false,
			nil,
			ethereum.Developer,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capture.waitFor(t, selfCheckMisconfiguredMessage, 500*time.Millisecond) {
			t.Fatal(
				"expected a failed registration scan to keep the " +
					"misconfiguration self-check silent",
			)
		}
	})
}

// TestWireReservationWatchers_MutationSurvivors_RunErrorRecordsWatcherDeath
// pins that each watcher goroutine records a watcher death when its Run
// loop returns an error (here: a zero poll interval, which Run rejects):
// a dead watcher must be observable in metrics, and a watcher that merely
// stops on context cancellation must not be counted as dead.
func TestWireReservationWatchers_MutationSurvivors_RunErrorRecordsWatcherDeath(
	t *testing.T,
) {
	newChain := func() *localChain {
		spvChain := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)
		return spvChain
	}

	waitForDeaths := func(t *testing.T, recorder *recordingMetricsRecorder, want float64) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			if got := recorder.Counter(clientinfo.MetricSpvReservationWatcherDeathsTotal); got >= want {
				return
			}
			select {
			case <-ticker.C:
			case <-deadline:
				t.Fatalf(
					"expected at least %v watcher deaths, got %v",
					want,
					recorder.Counter(clientinfo.MetricSpvReservationWatcherDeathsTotal),
				)
			}
		}
	}

	t.Run("stale-deposit watcher", func(t *testing.T) {
		setReservationStaleDepositPollInterval(0)
		t.Cleanup(func() {
			setReservationStaleDepositPollInterval(DefaultReservationStaleDepositPollInterval)
		})
		// A long restart backoff keeps the death count at one.
		setReservationWatcherRestartBackoff(time.Minute)
		t.Cleanup(func() {
			setReservationWatcherRestartBackoff(DefaultReservationWatcherRestartBackoff)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			newChain(),
			true,
			recorder,
			ethereum.Unknown,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		waitForDeaths(t, recorder, 1)

		if got := recorder.Counter(clientinfo.MetricSpvReservationWatcherDeathsTotal); got != 1 {
			t.Fatalf(
				"expected exactly one watcher death from the stale-deposit "+
					"watcher's zero interval, got %v",
				got,
			)
		}
	})

	t.Run("action-timeout watcher and keeper observer", func(t *testing.T) {
		setReservationActionTimeoutPollInterval(0)
		t.Cleanup(func() {
			setReservationActionTimeoutPollInterval(DefaultReservationActionTimeoutPollInterval)
		})
		setReservationWatcherRestartBackoff(time.Minute)
		t.Cleanup(func() {
			setReservationWatcherRestartBackoff(DefaultReservationWatcherRestartBackoff)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			newChain(),
			true,
			recorder,
			ethereum.Unknown,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		waitForDeaths(t, recorder, 2)

		if got := recorder.Counter(clientinfo.MetricSpvReservationWatcherDeathsTotal); got != 2 {
			t.Fatalf(
				"expected exactly two watcher deaths (action-timeout and "+
					"keeper observer) from the zero interval, got %v",
				got,
			)
		}
	})
}

// TestWireReservationWatchers_MutationSurvivors_RestartsAfterRunError pins
// the restart decision: a watcher goroutine whose Run loop returned an
// error sets shouldRestart, and the loop must then restart it after the
// backoff. Returning instead leaves the watcher set permanently short a
// watcher after a single fatal poll error.
func TestWireReservationWatchers_MutationSurvivors_RestartsAfterRunError(t *testing.T) {
	newChain := func() *localChain {
		spvChain := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(1000)
		spvChain.setBlockCounter(blockCounter)
		return spvChain
	}

	t.Run("stale-deposit watcher", func(t *testing.T) {
		capture := startSpvLogCapture(t)

		// A zero interval makes Run return an error on every attempt, so
		// an original build logs and restarts repeatedly.
		setReservationStaleDepositPollInterval(0)
		t.Cleanup(func() {
			setReservationStaleDepositPollInterval(DefaultReservationStaleDepositPollInterval)
		})
		setReservationWatcherRestartBackoff(5 * time.Millisecond)
		t.Cleanup(func() {
			setReservationWatcherRestartBackoff(DefaultReservationWatcherRestartBackoff)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			newChain(),
			true,
			nil,
			ethereum.Unknown,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		message := "reservation stale-deposit watcher is not running"
		if !capture.waitForCount(t, message, 3, 5*time.Second) {
			t.Fatalf(
				"expected the stale-deposit watcher to be restarted after "+
					"its Run error (%q logged %d times)",
				message,
				capture.count(message),
			)
		}
	})

	t.Run("keeper observer", func(t *testing.T) {
		capture := startSpvLogCapture(t)

		setReservationActionTimeoutPollInterval(0)
		t.Cleanup(func() {
			setReservationActionTimeoutPollInterval(DefaultReservationActionTimeoutPollInterval)
		})
		setReservationWatcherRestartBackoff(5 * time.Millisecond)
		t.Cleanup(func() {
			setReservationWatcherRestartBackoff(DefaultReservationWatcherRestartBackoff)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := WireReservationWatchers(
			ctx,
			&mockWalletClosedChain{},
			newChain(),
			true,
			nil,
			ethereum.Unknown,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		message := "reservation keeper observer is not running"
		if !capture.waitForCount(t, message, 2, 5*time.Second) {
			t.Fatalf(
				"expected the keeper observer to be restarted after its Run "+
					"error (%q logged %d times)",
				message,
				capture.count(message),
			)
		}
	})
}
