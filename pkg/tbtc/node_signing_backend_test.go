package tbtc

import (
	"errors"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestConfigureFrostSigningBackend_Default(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	t.Cleanup(frostsigning.ResetExecutionBackend)

	err := configureFrostSigningBackend(Config{})
	if err != nil {
		t.Fatalf("unexpected config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.LegacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.LegacyExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func TestConfigureFrostSigningBackend_NativeUnavailable(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	t.Cleanup(frostsigning.ResetExecutionBackend)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
	if err == nil {
		t.Fatal("expected native backend config error")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
}
