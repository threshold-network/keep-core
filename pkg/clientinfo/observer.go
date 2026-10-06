package clientinfo

import (
	"context"
	"time"
)

// MetricObserverInput defines a source of metric data.
type MetricObserverInput func() float64

// MetricObserverOutput defines a destination of collected metric data.
type MetricObserverOutput interface {
	Set(value float64)
}

// MetricObserver represent a definition of a cyclic metric observation process.
type MetricObserver struct {
	input  MetricObserverInput
	output MetricObserverOutput
}

// Observe triggers a cyclic metric observation process.
func (o *MetricObserver) Observe(
	ctx context.Context,
	tick time.Duration,
) {
	go func() {
		o.output.Set(o.input()) // execute the first check immediately

		ticker := time.NewTicker(tick)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				o.output.Set(o.input())
			case <-ctx.Done():
				return
			}
		}
	}()
}
