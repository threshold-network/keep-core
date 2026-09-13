package libp2p

import (
	"testing"
	"time"
)

// TestProviderSatisfiesMetricsSetterAssertion pins the wiring contract used by
// cmd.start to inject the performance metrics recorder into the provider. That
// wiring is a structural type assertion against an anonymous interface, so the
// provider's SetMetricsRecorder parameter must remain that exact anonymous
// interface. If it is changed to a named type, the assertion silently fails at
// runtime (ok == false) and provider metrics stop being recorded, which no
// build error or other test would catch.
func TestProviderSatisfiesMetricsSetterAssertion(t *testing.T) {
	var np interface{} = &provider{}
	if _, ok := np.(interface {
		SetMetricsRecorder(recorder interface {
			IncrementCounter(name string, value float64)
			SetGauge(name string, value float64)
			RecordDuration(name string, duration time.Duration)
		})
	}); !ok {
		t.Fatal(
			"provider no longer satisfies the metrics-setter assertion used " +
				"by cmd.start; provider metrics wiring is broken",
		)
	}
}
