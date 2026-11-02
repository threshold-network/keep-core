package tbtc

import (
	"testing"
	"time"
)

func TestIsReanchorTargetAged(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	tests := map[string]struct {
		createdAt      time.Time
		creationPeriod uint32
		headTime       time.Time
		tolerance      time.Duration
		expectAged     bool
	}{
		"younger target": {
			createdAt:      base.Add(60 * time.Second),
			creationPeriod: 600,
			headTime:       base.Add(100 * time.Second),
			tolerance:      ReanchorTargetAgeTolerance,
			expectAged:     false,
		},
		"exactly at the boundary": {
			createdAt:      base,
			creationPeriod: 100,
			headTime:       base.Add(76 * time.Second),
			tolerance:      ReanchorTargetAgeTolerance,
			expectAged:     true,
		},
		"within tolerance": {
			createdAt:      base,
			creationPeriod: 90,
			headTime:       base.Add(70 * time.Second),
			tolerance:      ReanchorTargetAgeTolerance,
			expectAged:     true,
		},
		"beyond tolerance": {
			createdAt:      base,
			creationPeriod: 95,
			headTime:       base.Add(70 * time.Second),
			tolerance:      ReanchorTargetAgeTolerance,
			expectAged:     false,
		},
		"zero creation period": {
			createdAt:      base,
			creationPeriod: 0,
			headTime:       base.Add(70 * time.Second),
			tolerance:      ReanchorTargetAgeTolerance,
			expectAged:     true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsReanchorTargetAged(
				test.createdAt,
				test.creationPeriod,
				test.headTime,
				test.tolerance,
			)
			if got != test.expectAged {
				t.Errorf("expected aged=%v, got %v", test.expectAged, got)
			}
		})
	}
}
