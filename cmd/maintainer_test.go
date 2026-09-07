package cmd

import (
	"context"
	"testing"
)

// TestInitializeMaintainerMetricsDisabledWhenPortUnset verifies the metrics
// on/off gate: when the client info port is 0 (unset), the maintainer boot path
// must report the endpoint as not configured and return a genuinely nil
// recorder, leaving SPV metrics recording disabled. This is the sole production
// switch that turns the SPV proof-submission counters on or off, so inverting
// the gate would silently change maintainer behavior.
func TestInitializeMaintainerMetricsDisabledWhenPortUnset(t *testing.T) {
	// clientConfig is a package-level global; restore it so the test does not
	// leak state into other tests in this package.
	originalPort := clientConfig.ClientInfo.Port
	defer func() { clientConfig.ClientInfo.Port = originalPort }()

	clientConfig.ClientInfo.Port = 0

	// The block counter is only touched on the configured (enabled) path, so a
	// nil value is safe for the disabled path under test.
	recorder := initializeMaintainerMetrics(context.Background(), nil, nil)

	if recorder != nil {
		t.Errorf(
			"expected a nil recorder when the client info port is unset, got [%v]",
			recorder,
		)
	}
}
