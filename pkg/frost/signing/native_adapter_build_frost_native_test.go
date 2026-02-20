//go:build frost_native

package signing

import (
	"context"
	"errors"
	"testing"
)

func TestNativeExecutionBackend_FrostNativeBuildSelectable(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	registerNativeExecutionAdapterForBuild()
	t.Cleanup(ResetExecutionBackend)
	t.Cleanup(UnregisterNativeExecutionAdapter)

	err := SetExecutionBackendByName("native")
	if err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if CurrentExecutionBackendName() != NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			NativeExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}

	_, err = Execute(
		context.Background(),
		nil,
		nil,
		"session-id",
		1,
		nil,
		10,
		4,
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected placeholder native execution error")
	}

	if !errors.Is(err, ErrNativeExecutionBackendNotImplemented) {
		t.Fatalf(
			"unexpected native execution error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeExecutionBackendNotImplemented,
			err,
		)
	}
}
