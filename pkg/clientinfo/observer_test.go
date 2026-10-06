package clientinfo

import (
	"context"
	"testing"
	"time"
)

func TestObserve(t *testing.T) {
	input := func() float64 {
		return 5000
	}
	gauge := &Gauge{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	observer := &MetricObserver{input, gauge}

	observer.Observe(ctx, 1*time.Millisecond)

	<-ctx.Done()

	expectedGaugeValue := float64(5000)
	if gauge.value != expectedGaugeValue {
		t.Fatalf(
			"incorrect gauge value:\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedGaugeValue,
			gauge.value,
		)
	}

	if gauge.timestamp == 0 {
		t.Fatal("timestamp should be set")
	}
}
