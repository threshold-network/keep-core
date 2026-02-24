package signing

import "testing"

func TestRegisterNativeTBTCSignerCoarseSignatureObserverRejectsNil(t *testing.T) {
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	err := RegisterNativeTBTCSignerCoarseSignatureObserver(nil)
	if err == nil {
		t.Fatal("expected registration error")
	}
}

func TestEmitNativeTBTCSignerCoarseSignatureEvent(t *testing.T) {
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	var (
		received bool
		actual   NativeTBTCSignerCoarseSignatureEvent
	)

	err := RegisterNativeTBTCSignerCoarseSignatureObserver(
		func(event NativeTBTCSignerCoarseSignatureEvent) {
			received = true
			actual = event
		},
	)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	expected := NativeTBTCSignerCoarseSignatureEvent{
		SessionID:      "session-1",
		KeyGroupSource: "legacy-wallet-pubkey",
		EngineVersion:  "tbtc-signer/0.1.0-bootstrap",
	}

	emitNativeTBTCSignerCoarseSignatureEvent(expected)

	if !received {
		t.Fatal("expected coarse signature event to be delivered")
	}

	if actual != expected {
		t.Fatalf(
			"unexpected coarse signature event\nexpected: [%+v]\nactual:   [%+v]",
			expected,
			actual,
		)
	}
}
