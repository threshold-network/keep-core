package tbtc

import "testing"

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
