//go:build frost_native

package tbtc

import (
	"fmt"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func registerSignerMaterialResolverForBuild() error {
	provider := currentSignerMaterialResolverProviderForBuild()
	if provider == nil {
		provider = defaultSignerMaterialResolverProviderForBuild
	}

	resolver, err := provider()
	if err != nil {
		return err
	}

	if resolver == nil {
		return fmt.Errorf("signer material resolver is nil")
	}

	return RegisterSignerMaterialResolver(resolver)
}

func defaultSignerMaterialResolverProviderForBuild() (SignerMaterialResolver, error) {
	return &buildTaggedNativeSignerMaterialResolver{}, nil
}

// buildTaggedNativeSignerMaterialResolver derives transitional native signer
// material from a legacy private key share for frost_native builds.
type buildTaggedNativeSignerMaterialResolver struct{}

func (btnsmr *buildTaggedNativeSignerMaterialResolver) ResolveSignerMaterial(
	privateKeyShare *tecdsa.PrivateKeyShare,
) (any, error) {
	if privateKeyShare == nil {
		return nil, fmt.Errorf("private key share is nil")
	}

	payload, err := privateKeyShare.Marshal()
	if err != nil {
		return nil, fmt.Errorf("cannot marshal private key share: [%w]", err)
	}

	return &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
		Payload: payload,
	}, nil
}
