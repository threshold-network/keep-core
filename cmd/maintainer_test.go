package cmd

import (
	"context"
	"net"
	"testing"

	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// stubBlockCounter is a minimal chain.BlockCounter implementation used to
// exercise the enabled path of initializeMaintainerMetrics without wiring a
// real Ethereum connection. Only CurrentBlock is ever invoked (from the
// registry's background eth-connectivity observer goroutine); the other
// methods exist solely to satisfy the interface.
type stubBlockCounter struct{}

func (stubBlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	return nil
}

func (stubBlockCounter) BlockHeightWaiter(
	blockNumber uint64,
) (<-chan uint64, error) {
	ch := make(chan uint64)
	close(ch)
	return ch, nil
}

func (stubBlockCounter) CurrentBlock() (uint64, error) {
	return 0, nil
}

func (stubBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	ch := make(chan uint64)
	close(ch)
	return ch
}

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
	recorder := initializeMaintainerMetrics(context.Background(), nil)

	if recorder != nil {
		t.Errorf(
			"expected a nil recorder when the client info port is unset, got [%v]",
			recorder,
		)
	}
}

// TestInitializeMaintainerMetricsEnabledWhenPortSet is the enabled-path
// counterpart to TestInitializeMaintainerMetricsDisabledWhenPortUnset: when
// the client info port is set, initializeMaintainerMetrics must return a
// live *clientinfo.PerformanceMetrics recorder wired to the registry it just
// created. A regression such as a dropped `return perfMetrics`, an early
// return after registry.RegisterMetricClientInfo, or constructing the
// recorder against the wrong registry would silently disable all production
// SPV proof-submission metrics, while every other test in the codebase
// (which construct fake recorders directly, bypassing this function) would
// keep passing.
func TestInitializeMaintainerMetricsEnabledWhenPortSet(t *testing.T) {
	// clientConfig is a package-level global; restore it so the test does not
	// leak state into other tests in this package.
	originalPort := clientConfig.ClientInfo.Port
	defer func() { clientConfig.ClientInfo.Port = originalPort }()

	// Reserve a genuinely free ephemeral port from the OS and release it
	// immediately. clientinfo.Initialize binds this port on
	// http.DefaultServeMux via an unowned ListenAndServe goroutine with no
	// shutdown handle, so a hardcoded port risks colliding with another
	// process or a parallel test run.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve a free port: [%v]", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("could not release reserved port: [%v]", err)
	}

	clientConfig.ClientInfo.Port = port

	// A stub block counter is used instead of nil: the enabled path starts a
	// background goroutine that immediately calls blockCounter.CurrentBlock,
	// which would panic on a nil chain.BlockCounter interface.
	recorder := initializeMaintainerMetrics(context.Background(), stubBlockCounter{})

	if recorder == nil {
		t.Fatal("expected a non-nil recorder when the client info port is set")
	}

	perfMetrics, ok := recorder.(*clientinfo.PerformanceMetrics)
	if !ok {
		t.Fatalf(
			"expected recorder to be a *clientinfo.PerformanceMetrics, got [%T]",
			recorder,
		)
	}

	perfMetrics.IncrementCounter(clientinfo.MetricDepositSweepProofSubmissionsTotal, 1)

	if value := perfMetrics.GetCounterValue(
		clientinfo.MetricDepositSweepProofSubmissionsTotal,
	); value != 1 {
		t.Errorf(
			"expected counter value [1] after increment, got [%v]",
			value,
		)
	}
}
