package signing

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestFromTECDSASignature(t *testing.T) {
	signature := &tecdsa.Signature{
		R: big.NewInt(0x1234),
		S: big.NewInt(0xabcd),
	}

	result, err := FromTECDSASignature(signature)
	if err != nil {
		t.Fatalf("conversion failed: [%v]", err)
	}

	if result.R[30] != 0x12 || result.R[31] != 0x34 {
		t.Fatalf("unexpected R component bytes")
	}

	if result.S[30] != 0xab || result.S[31] != 0xcd {
		t.Fatalf("unexpected S component bytes")
	}
}

func TestFromTECDSASignature_ValidationErrors(t *testing.T) {
	testData := []struct {
		name      string
		signature *tecdsa.Signature
	}{
		{
			name:      "nil signature",
			signature: nil,
		},
		{
			name: "nil R",
			signature: &tecdsa.Signature{
				R: nil,
				S: big.NewInt(1),
			},
		},
		{
			name: "nil S",
			signature: &tecdsa.Signature{
				R: big.NewInt(1),
				S: nil,
			},
		},
		{
			name: "negative R",
			signature: &tecdsa.Signature{
				R: big.NewInt(-1),
				S: big.NewInt(1),
			},
		},
		{
			name: "negative S",
			signature: &tecdsa.Signature{
				R: big.NewInt(1),
				S: big.NewInt(-1),
			},
		},
	}

	for _, tc := range testData {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromTECDSASignature(tc.signature)
			if err == nil {
				t.Fatal("expected conversion error")
			}
		})
	}
}

func TestExecuteRequest_NilRequest(t *testing.T) {
	_, err := ExecuteRequest(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected request validation error")
	}
}

func TestExecuteRequest_ClonesAttempt(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	backend := &mockExecutionBackend{
		name:   "mock",
		result: &Result{},
	}

	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("unexpected backend setup error: [%v]", err)
	}

	request := &Request{
		Attempt: &Attempt{
			Number:                 2,
			CoordinatorMemberIndex: 3,
			IncludedMembersIndexes: []group.MemberIndex{1, 3, 5},
			ExcludedMembersIndexes: []group.MemberIndex{2, 4},
		},
	}

	if _, err := ExecuteRequest(context.Background(), nil, request); err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if backend.lastRequest == request {
		t.Fatal("expected request clone before backend execution")
	}

	if backend.lastRequest.Attempt == request.Attempt {
		t.Fatal("expected attempt clone before backend execution")
	}

	if !reflect.DeepEqual(backend.lastRequest.Attempt, request.Attempt) {
		t.Fatalf(
			"unexpected attempt clone\nexpected: [%+v]\nactual:   [%+v]",
			request.Attempt,
			backend.lastRequest.Attempt,
		)
	}
}

func TestExecute_PopulatesSignerMaterialAndLegacyAlias(t *testing.T) {
	ResetExecutionBackend()
	t.Cleanup(ResetExecutionBackend)

	backend := &mockExecutionBackend{
		name:   "mock",
		result: &Result{},
	}

	if err := SetExecutionBackend(backend); err != nil {
		t.Fatalf("unexpected backend setup error: [%v]", err)
	}

	privateKeyShare := new(tecdsa.PrivateKeyShare)

	_, err := Execute(
		context.Background(),
		nil,
		big.NewInt(42),
		"session-id",
		group.MemberIndex(7),
		privateKeyShare,
		10,
		3,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected execute error: [%v]", err)
	}

	if backend.lastRequest == nil {
		t.Fatal("expected backend request")
	}

	if backend.lastRequest.SignerMaterial != privateKeyShare {
		t.Fatalf(
			"unexpected signer material\nexpected: [%v]\nactual:   [%v]",
			privateKeyShare,
			backend.lastRequest.SignerMaterial,
		)
	}

	if backend.lastRequest.PrivateKeyShare != privateKeyShare {
		t.Fatalf(
			"unexpected legacy private key share alias\nexpected: [%v]\nactual:   [%v]",
			privateKeyShare,
			backend.lastRequest.PrivateKeyShare,
		)
	}
}
