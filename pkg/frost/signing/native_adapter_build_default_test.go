//go:build !frost_native

package signing

import (
	"errors"
	"testing"
)

func TestNativeExecutionBackend_DefaultBuildUnavailable(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	err := SetExecutionBackendByName("native")
	if err == nil {
		t.Fatal("expected native backend unavailable error")
	}

	if !errors.Is(err, ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected native backend error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
}
