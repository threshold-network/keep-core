//go:build frost_native

package tbtc

import "fmt"

func registerSignerMaterialResolverForBuild() error {
	provider := currentSignerMaterialResolverProviderForBuild()
	if provider == nil {
		return nil
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
