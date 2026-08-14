//go:build frost_native

package tbtc

import (
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestCurrentFrostInteractiveSigningReadinessRejectsAbsentAndPartialRuntime(
	t *testing.T,
) {
	frostsigning.ResetInteractiveSigningEngineProviderForTest()
	t.Cleanup(frostsigning.ResetInteractiveSigningEngineProviderForTest)

	t.Run("all flags and engine absent", func(t *testing.T) {
		t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "")
		t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "")
		t.Setenv(frostsigning.InteractiveSigningOnlyEnvVar, "")
		if currentFrostInteractiveSigningReadiness() {
			t.Fatal("production readiness accepted an absent interactive runtime")
		}
	})

	t.Run("only interactive opt-in", func(t *testing.T) {
		t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "true")
		t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "")
		t.Setenv(frostsigning.InteractiveSigningOnlyEnvVar, "")
		if currentFrostInteractiveSigningReadiness() {
			t.Fatal("production readiness accepted partial interactive flags")
		}
	})

	t.Run("missing interactive-only gate", func(t *testing.T) {
		t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "true")
		t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "true")
		t.Setenv(frostsigning.InteractiveSigningOnlyEnvVar, "")
		if currentFrostInteractiveSigningReadiness() {
			t.Fatal("production readiness accepted coarse-fallback mode")
		}
	})

	t.Run("engine absent with every flag", func(t *testing.T) {
		t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "true")
		t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "true")
		t.Setenv(frostsigning.InteractiveSigningOnlyEnvVar, "true")
		frostsigning.ResetInteractiveSigningEngineProviderForTest()
		if currentFrostInteractiveSigningReadiness() {
			t.Fatal("production readiness accepted an absent interactive engine")
		}
	})
}
