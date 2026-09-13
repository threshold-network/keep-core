package clientinfo

import (
	"testing"
)

func TestGaugeSet(t *testing.T) {
	gauge := &Gauge{
		name:   "test_gauge",
		labels: map[string]string{"label": "value"},
	}

	if gauge.value != 0 {
		t.Fatal("incorrect gauge initial value")
	}

	if gauge.timestamp != 0 {
		t.Fatal("incorrect gauge initial timestamp")
	}

	newGaugeValue := float64(500)

	gauge.Set(newGaugeValue)

	if gauge.value != newGaugeValue {
		t.Fatalf(
			"incorrect gauge value:\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			newGaugeValue,
			gauge.value,
		)
	}

	if gauge.timestamp == 0 {
		t.Fatal("timestamp should be set")
	}
}

func TestGaugeExpose(t *testing.T) {
	gauge := &Gauge{
		name:      "test_gauge",
		labels:    map[string]string{"label": "value"},
		value:     500,
		timestamp: 1000,
	}

	actualText := gauge.expose()

	expectedText := "# TYPE test_gauge gauge\ntest_gauge{label=\"value\"} 500 1000"

	if actualText != expectedText {
		t.Fatalf(
			"incorrect gauge expose text:\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedText,
			actualText,
		)
	}
}

func TestGaugeWithoutLabelsExpose(t *testing.T) {
	gauge := &Gauge{
		name:      "test_gauge",
		labels:    map[string]string{},
		value:     500,
		timestamp: 1000,
	}

	actualText := gauge.expose()

	expectedText := "# TYPE test_gauge gauge\ntest_gauge 500 1000"

	if actualText != expectedText {
		t.Fatalf(
			"incorrect gauge expose text:\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedText,
			actualText,
		)
	}
}
