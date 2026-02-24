package signing

import (
	"fmt"
	"sync"
)

// NativeTBTCSignerCoarseSignatureEvent describes successful coarse-path
// signature production for tbtc-signer payloads.
type NativeTBTCSignerCoarseSignatureEvent struct {
	SessionID      string
	KeyGroupSource string
	EngineVersion  string
}

// NativeTBTCSignerCoarseSignatureObserver consumes coarse-signature telemetry
// events.
type NativeTBTCSignerCoarseSignatureObserver func(
	event NativeTBTCSignerCoarseSignatureEvent,
)

var (
	nativeTBTCSignerCoarseSignatureObserverMutex sync.RWMutex
	nativeTBTCSignerCoarseSignatureObserver      NativeTBTCSignerCoarseSignatureObserver
)

// RegisterNativeTBTCSignerCoarseSignatureObserver registers a process-wide
// observer used to report tbtc-signer coarse-signature success events.
func RegisterNativeTBTCSignerCoarseSignatureObserver(
	observer NativeTBTCSignerCoarseSignatureObserver,
) error {
	if observer == nil {
		return fmt.Errorf("native tbtc-signer coarse signature observer is nil")
	}

	nativeTBTCSignerCoarseSignatureObserverMutex.Lock()
	defer nativeTBTCSignerCoarseSignatureObserverMutex.Unlock()

	nativeTBTCSignerCoarseSignatureObserver = observer

	return nil
}

// UnregisterNativeTBTCSignerCoarseSignatureObserver clears coarse-signature
// observer registration.
func UnregisterNativeTBTCSignerCoarseSignatureObserver() {
	nativeTBTCSignerCoarseSignatureObserverMutex.Lock()
	defer nativeTBTCSignerCoarseSignatureObserverMutex.Unlock()

	nativeTBTCSignerCoarseSignatureObserver = nil
}

func emitNativeTBTCSignerCoarseSignatureEvent(
	event NativeTBTCSignerCoarseSignatureEvent,
) {
	nativeTBTCSignerCoarseSignatureObserverMutex.RLock()
	observer := nativeTBTCSignerCoarseSignatureObserver
	nativeTBTCSignerCoarseSignatureObserverMutex.RUnlock()

	if observer == nil {
		return
	}

	observer(event)
}
