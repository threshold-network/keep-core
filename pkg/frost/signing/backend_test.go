package signing

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type mockExecutionBackend struct {
	name string

	executeCalls int
	lastRequest  *Request
	result       *Result
	err          error

	registerUnmarshallersCalls int
	lastChannel                net.BroadcastChannel
}

func (meb *mockExecutionBackend) Name() string {
	return meb.name
}

func (meb *mockExecutionBackend) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	meb.executeCalls++
	meb.lastRequest = request
	return meb.result, meb.err
}

func (meb *mockExecutionBackend) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	meb.registerUnmarshallersCalls++
	meb.lastChannel = channel
}

func TestCurrentExecutionBackendName_Default(t *testing.T) {
	ResetExecutionBackend()
	if CurrentExecutionBackendName() != legacyExecutionBackendName {
		t.Fatalf(
			"unexpected default backend name\nexpected: [%s]\nactual:   [%s]",
			legacyExecutionBackendName,
			CurrentExecutionBackendName(),
		)
	}
}

func TestSetExecutionBackend_Nil(t *testing.T) {
	if err := SetExecutionBackend(nil); err == nil {
		t.Fatal("expected nil backend error")
	}
}

func TestExecute_DelegatesToCurrentBackend(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	expectedResult := &Result{Signature: &frost.Signature{}}
	backend := &mockExecutionBackend{
		name:   "mock",
		result: expectedResult,
	}

	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("failed setting backend: [%v]", err)
	}

	attempt := &Attempt{
		Number:                 2,
		CoordinatorMemberIndex: 5,
		IncludedMembersIndexes: []group.MemberIndex{1, 2, 5},
		ExcludedMembersIndexes: []group.MemberIndex{3, 4, 6},
	}

	result, err := Execute(
		context.Background(),
		nil,
		big.NewInt(100),
		"session-id",
		1,
		nil,
		10,
		4,
		nil,
		nil,
		attempt,
	)
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if result != expectedResult {
		t.Fatalf(
			"unexpected result\nexpected: [%+v]\nactual:   [%+v]",
			expectedResult,
			result,
		)
	}

	if backend.executeCalls != 1 {
		t.Fatalf("unexpected execute calls count: [%d]", backend.executeCalls)
	}

	received := backend.lastRequest
	if received == nil {
		t.Fatal("expected backend request")
	}

	if received.Attempt == attempt {
		t.Fatal("expected request attempt clone, got same pointer")
	}

	if !reflect.DeepEqual(received.Attempt, attempt) {
		t.Fatalf(
			"unexpected request attempt\nexpected: [%+v]\nactual:   [%+v]",
			attempt,
			received.Attempt,
		)
	}

	received.Attempt.IncludedMembersIndexes[0] = 99
	if attempt.IncludedMembersIndexes[0] == 99 {
		t.Fatal("mutating backend request attempt should not mutate caller attempt")
	}
}

func TestRegisterUnmarshallers_DelegatesToCurrentBackend(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	backend := &mockExecutionBackend{name: "mock"}
	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("failed setting backend: [%v]", err)
	}

	RegisterUnmarshallers(nil)

	if backend.registerUnmarshallersCalls != 1 {
		t.Fatalf(
			"unexpected register unmarshallers calls count: [%d]",
			backend.registerUnmarshallersCalls,
		)
	}

	if backend.lastChannel != nil {
		t.Fatal("expected nil channel to be forwarded unchanged")
	}
}
