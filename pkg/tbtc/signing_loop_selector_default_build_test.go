//go:build !frost_roast_retry

package tbtc

import "testing"

func TestDefaultSigningParticipantSelector_IsLegacyInDefaultBuild(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(legacySigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector in default build must return legacy implementation; got %T",
			sel,
		)
	}
}
