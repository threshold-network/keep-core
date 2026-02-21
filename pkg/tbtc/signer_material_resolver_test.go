package tbtc

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

type staticSignerMaterialResolver struct {
	result any
	err    error
}

func (ssmr *staticSignerMaterialResolver) ResolveSignerMaterial(
	privateKeyShare *tecdsa.PrivateKeyShare,
) (any, error) {
	return ssmr.result, ssmr.err
}

func TestRegisterSignerMaterialResolver_Nil(t *testing.T) {
	err := RegisterSignerMaterialResolver(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterSignerMaterialResolverProviderForBuild_Nil(t *testing.T) {
	err := RegisterSignerMaterialResolverProviderForBuild(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSignerMaterial_DefaultResolver(t *testing.T) {
	UnregisterSignerMaterialResolver()
	t.Cleanup(UnregisterSignerMaterialResolver)

	privateKeyShare := createMockSigner(t).privateKeyShare

	result, err := resolveSignerMaterial(privateKeyShare)
	if err != nil {
		t.Fatalf("unexpected resolver error: [%v]", err)
	}

	resolvedPrivateKeyShare, ok := result.(*tecdsa.PrivateKeyShare)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			&tecdsa.PrivateKeyShare{},
			result,
		)
	}

	if resolvedPrivateKeyShare != privateKeyShare {
		t.Fatalf(
			"unexpected resolved private key share\nexpected: [%v]\nactual:   [%v]",
			privateKeyShare,
			resolvedPrivateKeyShare,
		)
	}
}

func TestResolveSignerMaterial_RegisteredResolver(t *testing.T) {
	UnregisterSignerMaterialResolver()
	t.Cleanup(UnregisterSignerMaterialResolver)

	expected := []byte{0xaa, 0xbb}
	err := RegisterSignerMaterialResolver(
		&staticSignerMaterialResolver{
			result: expected,
		},
	)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	result, err := resolveSignerMaterial(createMockSigner(t).privateKeyShare)
	if err != nil {
		t.Fatalf("unexpected resolver error: [%v]", err)
	}

	resultBytes, ok := result.([]byte)
	if !ok {
		t.Fatalf(
			"unexpected resolved signer material type\nexpected: [%T]\nactual:   [%T]",
			[]byte{},
			result,
		)
	}

	if len(resultBytes) != len(expected) ||
		resultBytes[0] != expected[0] ||
		resultBytes[1] != expected[1] {
		t.Fatalf(
			"unexpected resolved signer material\nexpected: [%x]\nactual:   [%x]",
			expected,
			resultBytes,
		)
	}
}

func TestResolveSignerMaterial_ResolverError(t *testing.T) {
	UnregisterSignerMaterialResolver()
	t.Cleanup(UnregisterSignerMaterialResolver)

	expectedErr := errors.New("resolver error")
	err := RegisterSignerMaterialResolver(
		&staticSignerMaterialResolver{
			err: expectedErr,
		},
	)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	_, err = resolveSignerMaterial(createMockSigner(t).privateKeyShare)
	if err == nil {
		t.Fatal("expected resolver error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected resolver error\nexpected: [%v]\nactual:   [%v]",
			expectedErr,
			err,
		)
	}
}
