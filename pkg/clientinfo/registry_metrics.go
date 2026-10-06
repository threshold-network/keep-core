package clientinfo

import (
	"fmt"
	"strings"
)

type metric interface {
	expose() string
}

// Label represents an arbitrary information attached to the metrics.
type Label struct {
	name  string
	value string
}

// NewLabel creates a new label using the given name and value.
func NewLabel(name, value string) Label {
	return Label{name, value}
}

// Exposes all registered metrics in their text format.
func (r *Registry) exposeMetrics() string {
	r.metricsMutex.RLock()
	defer r.metricsMutex.RUnlock()

	metrics := make([]string, 0)
	for _, metric := range sortedKeys(r.metrics) {
		metrics = append(metrics, r.metrics[metric].expose())
	}

	return fmt.Sprintf("%s\n", strings.Join(metrics, "\n\n"))
}

// NewMetricGauge creates and registers a new gauge metric which will be exposed
// through the metrics server. In case a metric already exists, an error
// will be returned.
func (r *Registry) NewMetricGauge(
	name string,
	labels ...Label,
) (*Gauge, error) {
	r.metricsMutex.Lock()
	defer r.metricsMutex.Unlock()

	if _, exists := r.metrics[name]; exists {
		return nil, fmt.Errorf("metric [%v] already exists", name)
	}

	gauge := &Gauge{
		name:   name,
		labels: processLabels(labels),
	}

	r.metrics[name] = gauge
	return gauge, nil
}

// NewMetricGaugeObserver creates and registers a gauge just like `NewMetricGauge`
// method and wraps it with a ready to use observer of the provided input. This
// allows to easily create self-refreshing metrics.
func (r *Registry) NewMetricGaugeObserver(
	name string,
	input MetricObserverInput,
	labels ...Label,
) (*MetricObserver, error) {
	gauge, err := r.NewMetricGauge(name, labels...)
	if err != nil {
		return nil, err
	}

	return &MetricObserver{
		input:  input,
		output: gauge,
	}, nil
}

// NewMetricInfo creates and registers a new info metric which will be exposed
// through the metrics server. In case a metric already exists, an error
// will be returned.
func (r *Registry) NewMetricInfo(
	name string,
	labels []Label,
) (*Info, error) {
	r.metricsMutex.Lock()
	defer r.metricsMutex.Unlock()

	if _, exists := r.metrics[name]; exists {
		return nil, fmt.Errorf("metric [%v] already exists", name)
	}

	if len(labels) == 0 {
		return nil, fmt.Errorf("at least one label should be set")
	}

	info := &Info{
		name:   name,
		labels: processLabels(labels),
	}

	r.metrics[name] = info
	return info, nil
}

func processLabels(
	labels []Label,
) map[string]string {
	result := make(map[string]string)

	for _, label := range labels {
		if label.name == "" || label.value == "" {
			continue
		}

		result[label.name] = label.value
	}

	return result
}
