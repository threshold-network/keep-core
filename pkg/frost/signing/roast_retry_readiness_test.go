package signing

import (
	"errors"
	"strings"
	"testing"
)

func TestEnsureRoastRetryReadinessOptIn_AcceptsTrue(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestEnsureRoastRetryReadinessOptIn_AcceptsTrueCaseInsensitive(t *testing.T) {
	cases := []string{"true", "True", "TRUE", "tRuE"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			t.Setenv(RoastRetryReadinessOptInEnvVar, value)
			if err := EnsureRoastRetryReadinessOptIn(); err != nil {
				t.Fatalf("expected nil error for %q, got %v", value, err)
			}
		})
	}
}

func TestEnsureRoastRetryReadinessOptIn_AcceptsTrimmedWhitespace(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "  true  ")
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		t.Fatalf("expected nil error for whitespace-padded 'true', got %v", err)
	}
}

func TestEnsureRoastRetryReadinessOptIn_RejectsUnset(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "")
	err := EnsureRoastRetryReadinessOptIn()
	if !errors.Is(err, ErrRoastRetryReadinessOptOut) {
		t.Fatalf("expected ErrRoastRetryReadinessOptOut, got %v", err)
	}
	if !strings.Contains(err.Error(), RoastRetryReadinessOptInEnvVar) {
		t.Fatalf(
			"error must mention the env var name to guide operators; got %v",
			err,
		)
	}
}

func TestEnsureRoastRetryReadinessOptIn_RejectsOtherValues(t *testing.T) {
	cases := []string{"false", "1", "yes", "TRUE_", "tru", "anything"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			t.Setenv(RoastRetryReadinessOptInEnvVar, value)
			err := EnsureRoastRetryReadinessOptIn()
			if !errors.Is(err, ErrRoastRetryReadinessOptOut) {
				t.Fatalf("expected error for %q, got nil", value)
			}
		})
	}
}

func TestRoastRetryReadinessOptInEnabled_MirrorsEnsureResult(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	if !RoastRetryReadinessOptInEnabled() {
		t.Fatal("expected true when env var set to true")
	}
	t.Setenv(RoastRetryReadinessOptInEnvVar, "false")
	if RoastRetryReadinessOptInEnabled() {
		t.Fatal("expected false when env var set to false")
	}
}

func TestRoastRetryReadinessOptInEnvVar_MatchesRFC(t *testing.T) {
	const expected = "KEEP_CORE_FROST_ROAST_RETRY_ENABLED"
	if RoastRetryReadinessOptInEnvVar != expected {
		t.Fatalf(
			"env var name drifted: got %q want %q (must match RFC-21 Phase 5)",
			RoastRetryReadinessOptInEnvVar,
			expected,
		)
	}
}
