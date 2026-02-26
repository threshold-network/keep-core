package signing

import (
	"testing"
)

func TestRegisterNativeTBTCSignerFallbackObserverRejectsNil(t *testing.T) {
	UnregisterNativeTBTCSignerFallbackObserver()
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)

	err := RegisterNativeTBTCSignerFallbackObserver(nil)
	if err == nil {
		t.Fatal("expected registration error")
	}
}

func TestRegisterNativeTBTCSignerFallbackObserverRejectsDuplicate(t *testing.T) {
	UnregisterNativeTBTCSignerFallbackObserver()
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)

	firstErr := RegisterNativeTBTCSignerFallbackObserver(
		func(NativeTBTCSignerFallbackEvent) {},
	)
	if firstErr != nil {
		t.Fatalf("unexpected first registration error: [%v]", firstErr)
	}

	secondErr := RegisterNativeTBTCSignerFallbackObserver(
		func(NativeTBTCSignerFallbackEvent) {},
	)
	if secondErr == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestEmitNativeTBTCSignerFallbackEvent(t *testing.T) {
	UnregisterNativeTBTCSignerFallbackObserver()
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)

	var (
		received bool
		actual   NativeTBTCSignerFallbackEvent
	)

	err := RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			received = true
			actual = event
		},
	)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	expected := NativeTBTCSignerFallbackEvent{
		SessionID:                   "session-1",
		Reason:                      "fallback reason",
		KeyGroupSource:              "legacy-wallet-pubkey",
		LegacyPrivateKeyShareExists: true,
	}

	emitNativeTBTCSignerFallbackEvent(expected)

	if !received {
		t.Fatal("expected fallback event to be delivered")
	}

	if actual != expected {
		t.Fatalf(
			"unexpected fallback event\nexpected: [%+v]\nactual:   [%+v]",
			expected,
			actual,
		)
	}
}
