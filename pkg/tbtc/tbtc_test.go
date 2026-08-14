package tbtc

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestShouldMonitorLegacySortitionPool(t *testing.T) {
	if !shouldMonitorLegacySortitionPool(Config{}) {
		t.Fatal("expected legacy sortition pool monitoring to be enabled by default")
	}

	if shouldMonitorLegacySortitionPool(Config{
		DisableLegacySortitionPoolMonitoring: true,
	}) {
		t.Fatal("expected legacy sortition pool monitoring to be disabled")
	}

	if shouldMonitorLegacySortitionPool(Config{
		DisableLegacyECDSA: true,
	}) {
		t.Fatal("expected FROST-only mode to disable legacy sortition pool monitoring")
	}
}

func TestShouldRunLegacyECDSA(t *testing.T) {
	if !shouldRunLegacyECDSA(Config{}) {
		t.Fatal("expected legacy ECDSA to run by default")
	}

	if shouldRunLegacyECDSA(Config{DisableLegacyECDSA: true}) {
		t.Fatal("expected legacy ECDSA to be disabled")
	}
}

func TestValidateWalletSchemeConfig(t *testing.T) {
	registryAvailable := Connect()
	registryAvailable.frostWalletRegistryAvailable = true

	registryMissing := Connect()

	// The default configuration runs legacy ECDSA, so an absent FROST registry is
	// a legitimate legacy-only deployment.
	if err := validateWalletSchemeConfig(Config{}, registryMissing); err != nil {
		t.Fatalf("expected default legacy-only config to be accepted: [%v]", err)
	}

	// FROST-only with the registry configured is the intended FROST deployment.
	if err := validateWalletSchemeConfig(
		Config{DisableLegacyECDSA: true},
		registryAvailable,
	); err != nil {
		t.Fatalf("expected FROST-only config with registry to be accepted: [%v]", err)
	}

	// The trap: both schemes off. Neither switch fails on its own, so this must be
	// rejected here or the node starts and creates no wallets at all.
	err := validateWalletSchemeConfig(
		Config{DisableLegacyECDSA: true},
		registryMissing,
	)
	if err == nil {
		t.Fatal(
			"expected FROST-only config without a FROST wallet registry to be " +
				"rejected; such a node can create no wallets of either scheme",
		)
	}
	if !strings.Contains(err.Error(), "disableLegacyECDSA") {
		t.Fatalf("expected error to name the offending flag, got: [%v]", err)
	}
}

func TestProcessWalletClosureEventRetriesAfterFailure(t *testing.T) {
	deduplicator := newDeduplicator()
	event := &WalletClosedEvent{
		WalletID: [32]byte{0xaa},
		Scheme:   WalletSchemeFROST,
	}

	attempts := 0
	handle := func(_ [32]byte, scheme WalletScheme) error {
		attempts++
		if scheme != WalletSchemeFROST {
			t.Fatalf("unexpected wallet scheme [%v]", scheme)
		}
		if attempts == 1 {
			return errors.New("transient failure")
		}

		return nil
	}

	processed, err := processWalletClosureEvent(deduplicator, event, handle)
	if !processed || err == nil {
		t.Fatalf("expected first handling attempt to fail, got [%v, %v]", processed, err)
	}

	processed, err = processWalletClosureEvent(deduplicator, event, handle)
	if !processed || err != nil {
		t.Fatalf("expected replay to succeed, got [%v, %v]", processed, err)
	}

	processed, err = processWalletClosureEvent(deduplicator, event, handle)
	if processed || err != nil {
		t.Fatalf("expected successful event to be deduplicated, got [%v, %v]", processed, err)
	}
	if attempts != 2 {
		t.Fatalf("unexpected handling attempts [%v]", attempts)
	}
}

func TestProcessWalletClosureEventScopesDeduplicationByScheme(t *testing.T) {
	deduplicator := newDeduplicator()
	walletID := [32]byte{0xbb}
	handledSchemes := make([]WalletScheme, 0, 2)
	handle := func(_ [32]byte, scheme WalletScheme) error {
		handledSchemes = append(handledSchemes, scheme)
		return nil
	}

	for _, scheme := range []WalletScheme{WalletSchemeFROST, WalletSchemeUnknown} {
		processed, err := processWalletClosureEvent(
			deduplicator,
			&WalletClosedEvent{WalletID: walletID, Scheme: scheme},
			handle,
		)
		if !processed || err != nil {
			t.Fatalf("expected scheme [%v] to be processed, got [%v, %v]", scheme, processed, err)
		}
	}

	expected := []WalletScheme{WalletSchemeFROST, WalletSchemeECDSA}
	for i := range expected {
		if handledSchemes[i] != expected[i] {
			t.Fatalf(
				"unexpected handled scheme at index [%v]: expected [%v], got [%v]",
				i,
				expected[i],
				handledSchemes[i],
			)
		}
	}
}

func TestProcessWalletClosureEventPreservesConcurrentReplay(t *testing.T) {
	deduplicator := newDeduplicator()
	event := &WalletClosedEvent{
		WalletID: [32]byte{0xcc},
		Scheme:   WalletSchemeFROST,
	}

	firstAttemptStarted := make(chan struct{})
	releaseFirstAttempt := make(chan struct{})
	attempts := 0
	handle := func(_ [32]byte, _ WalletScheme) error {
		attempts++
		if attempts == 1 {
			close(firstAttemptStarted)
			<-releaseFirstAttempt
			return errors.New("transient failure")
		}

		return nil
	}

	type processingResult struct {
		processed bool
		err       error
	}
	result := make(chan processingResult, 1)
	go func() {
		processed, err := processWalletClosureEvent(deduplicator, event, handle)
		result <- processingResult{processed: processed, err: err}
	}()

	<-firstAttemptStarted
	processed, err := processWalletClosureEvent(
		deduplicator,
		event,
		func(_ [32]byte, _ WalletScheme) error {
			t.Fatal("concurrent replay must not start a second handler")
			return nil
		},
	)
	if processed || err != nil {
		t.Fatalf("expected concurrent replay to be queued, got [%v, %v]", processed, err)
	}

	close(releaseFirstAttempt)
	select {
	case actual := <-result:
		if !actual.processed || actual.err != nil {
			t.Fatalf(
				"expected queued replay to recover the failure, got [%v, %v]",
				actual.processed,
				actual.err,
			)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued wallet closure replay")
	}

	if attempts != 2 {
		t.Fatalf("expected two handling attempts, got [%v]", attempts)
	}
}
