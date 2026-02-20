//go:build frost_native

package signing

import (
	"context"
	"strings"
	"testing"
)

func TestNativeExecutionBackend_FrostNativeBuildSelectable(t *testing.T) {
	ResetExecutionBackend()
	UnregisterNativeExecutionAdapter()
	RegisterNativeExecutionAdapterForBuild()
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

	adapter := &buildTaggedNativeExecutionAdapter{}

	_, err = adapter.Execute(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected request validation error")
	}

	if !strings.Contains(err.Error(), "request is nil") {
		t.Fatalf(
			"unexpected native execution error\nexpected substring: [%s]\nactual:             [%v]",
			"request is nil",
			err,
		)
	}
}
