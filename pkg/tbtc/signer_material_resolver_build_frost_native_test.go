//go:build frost_native

package tbtc

import (
	"errors"
	"testing"
)

func TestRegisterSignerMaterialResolverForBuild_UsesRegisteredProvider(
	t *testing.T,
) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	expected := []byte{0xaa, 0xbb}
	err := RegisterSignerMaterialResolverProviderForBuild(
		func() (SignerMaterialResolver, error) {
			return &staticSignerMaterialResolver{
				result: expected,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected provider registration error: [%v]", err)
	}

	err = RegisterSignerMaterialResolverForBuild()
	if err != nil {
		t.Fatalf("unexpected build resolver registration error: [%v]", err)
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

func TestRegisterSignerMaterialResolverForBuild_ProviderError(t *testing.T) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	expectedErr := errors.New("provider error")
	err := RegisterSignerMaterialResolverProviderForBuild(
		func() (SignerMaterialResolver, error) {
			return nil, expectedErr
		},
	)
	if err != nil {
		t.Fatalf("unexpected provider registration error: [%v]", err)
	}

	err = RegisterSignerMaterialResolverForBuild()
	if err == nil {
		t.Fatal("expected build resolver registration error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected build resolver registration error\nexpected: [%v]\nactual:   [%v]",
			expectedErr,
			err,
		)
	}
}

func TestRegisterSignerMaterialResolverForBuild_ProviderReturnsNilResolver(
	t *testing.T,
) {
	UnregisterSignerMaterialResolver()
	UnregisterSignerMaterialResolverProviderForBuild()
	t.Cleanup(UnregisterSignerMaterialResolver)
	t.Cleanup(UnregisterSignerMaterialResolverProviderForBuild)

	err := RegisterSignerMaterialResolverProviderForBuild(
		func() (SignerMaterialResolver, error) {
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected provider registration error: [%v]", err)
	}

	err = RegisterSignerMaterialResolverForBuild()
	if err == nil {
		t.Fatal("expected build resolver registration error")
	}
}
