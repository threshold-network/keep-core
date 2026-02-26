package signing

import (
	"fmt"
	"sync"
)

// NativeTBTCSignerFallbackEvent describes a single fallback from the
// tbtc-signer coarse path to the legacy signing path.
type NativeTBTCSignerFallbackEvent struct {
	SessionID                   string
	Reason                      string
	KeyGroupSource              string
	LegacyPrivateKeyShareExists bool
}

// NativeTBTCSignerFallbackObserver consumes fallback telemetry events.
type NativeTBTCSignerFallbackObserver func(event NativeTBTCSignerFallbackEvent)

var (
	nativeTBTCSignerFallbackObserverMutex sync.RWMutex
	nativeTBTCSignerFallbackObserver      NativeTBTCSignerFallbackObserver
)

// RegisterNativeTBTCSignerFallbackObserver registers a process-wide observer
// used to report tbtc-signer fallback events.
// Only a single observer is supported.
func RegisterNativeTBTCSignerFallbackObserver(
	observer NativeTBTCSignerFallbackObserver,
) error {
	if observer == nil {
		return fmt.Errorf("native tbtc-signer fallback observer is nil")
	}

	nativeTBTCSignerFallbackObserverMutex.Lock()
	defer nativeTBTCSignerFallbackObserverMutex.Unlock()

	if nativeTBTCSignerFallbackObserver != nil {
		return fmt.Errorf("native tbtc-signer fallback observer is already registered")
	}

	nativeTBTCSignerFallbackObserver = observer

	return nil
}

// UnregisterNativeTBTCSignerFallbackObserver clears fallback-observer
// registration.
func UnregisterNativeTBTCSignerFallbackObserver() {
	nativeTBTCSignerFallbackObserverMutex.Lock()
	defer nativeTBTCSignerFallbackObserverMutex.Unlock()

	nativeTBTCSignerFallbackObserver = nil
}

func emitNativeTBTCSignerFallbackEvent(event NativeTBTCSignerFallbackEvent) {
	nativeTBTCSignerFallbackObserverMutex.RLock()
	observer := nativeTBTCSignerFallbackObserver
	nativeTBTCSignerFallbackObserverMutex.RUnlock()

	if observer == nil {
		return
	}

	observer(event)
}
