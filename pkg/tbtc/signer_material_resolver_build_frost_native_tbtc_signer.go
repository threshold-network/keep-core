//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import "fmt"

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
	// Legacy tECDSA shares identify existing wallets that still need the legacy
	// signing bridge during migration. Native FROST DKG supplies and persists its
	// own signer material directly, so it does not need legacy shares converted
	// into scaffold-era FROST material here.
	return &legacyPrivateKeyShareSignerMaterialResolver{}, nil
}
