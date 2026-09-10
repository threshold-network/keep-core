package clientinfo

import (
	"testing"
)

func TestRegistryNewMetricGauge(t *testing.T) {
	registry := NewRegistry()

	gauge, err := registry.NewMetricGauge("test-gauge")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = registry.NewMetricGauge("test-gauge"); err == nil {
		t.Fatalf("should fail when creating gauge with the same name")
	}

	if _, exists := registry.metrics[gauge.name]; !exists {
		t.Fatalf("metric with name [%v] should exist in the registry", gauge.name)
	}
}

func TestRegistryNewMetricGaugeObserver(t *testing.T) {
	registry := NewRegistry()

	input := func() float64 {
		return 1
	}

	_, err := registry.NewMetricGaugeObserver("test-gauge", input)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = registry.NewMetricGaugeObserver("test-gauge", input); err == nil {
		t.Fatalf("should fail when creating gauge with the same name")
	}

	if _, exists := registry.metrics["test-gauge"]; !exists {
		t.Fatalf("metric with name [%v] should exist in the registry", "test-gauge")
	}
}

func TestRegistryNewMetricInfo(t *testing.T) {
	registry := NewRegistry()

	if _, err := registry.NewMetricInfo("test-info", []Label{}); err == nil {
		t.Fatalf("should fail when creating info without labels")
	}

	label := NewLabel("label", "value")
	info, err := registry.NewMetricInfo("test-info", []Label{label})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = registry.NewMetricInfo("test-info", []Label{label}); err == nil {
		t.Fatalf("should fail when creating info with the same name")
	}

	if _, exists := registry.metrics[info.name]; !exists {
		t.Fatalf("metric with name [%v] should exist in the registry", info.name)
	}

	expectedLabelValue, exists := info.labels[label.name]
	if !exists {
		t.Fatalf("label with name [%v] should exist", label.name)
	}

	if expectedLabelValue != label.value {
		t.Fatalf(
			"label with name [%v] should have value [%v]",
			label.name,
			label.value,
		)
	}
}
