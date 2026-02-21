package tbtc

import (
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// SignerMaterialResolver derives signer material from a legacy private key
// share. Implementations can provide backend-native signer material while
// preserving fallback compatibility.
type SignerMaterialResolver interface {
	ResolveSignerMaterial(privateKeyShare *tecdsa.PrivateKeyShare) (any, error)
}

// SignerMaterialResolverProviderForBuild produces a signer material resolver
// bound to the current build/runtime flavor.
type SignerMaterialResolverProviderForBuild func() (SignerMaterialResolver, error)

type legacyPrivateKeyShareSignerMaterialResolver struct{}

func (lpkssmr *legacyPrivateKeyShareSignerMaterialResolver) ResolveSignerMaterial(
	privateKeyShare *tecdsa.PrivateKeyShare,
) (any, error) {
	if privateKeyShare == nil {
		return nil, fmt.Errorf("private key share is nil")
	}

	return privateKeyShare, nil
}

var (
	signerMaterialResolverMutex            sync.RWMutex
	signerMaterialResolver                 SignerMaterialResolver = &legacyPrivateKeyShareSignerMaterialResolver{}
	signerMaterialResolverProviderForBuild SignerMaterialResolverProviderForBuild
)

// RegisterSignerMaterialResolver registers a signer material resolver used by
// DKG signer construction.
func RegisterSignerMaterialResolver(resolver SignerMaterialResolver) error {
	if resolver == nil {
		return fmt.Errorf("signer material resolver is nil")
	}

	signerMaterialResolverMutex.Lock()
	defer signerMaterialResolverMutex.Unlock()

	signerMaterialResolver = resolver

	return nil
}

// UnregisterSignerMaterialResolver restores the default legacy resolver.
func UnregisterSignerMaterialResolver() {
	signerMaterialResolverMutex.Lock()
	defer signerMaterialResolverMutex.Unlock()

	signerMaterialResolver = &legacyPrivateKeyShareSignerMaterialResolver{}
}

// RegisterSignerMaterialResolverProviderForBuild registers a provider used by
// RegisterSignerMaterialResolverForBuild.
func RegisterSignerMaterialResolverProviderForBuild(
	provider SignerMaterialResolverProviderForBuild,
) error {
	if provider == nil {
		return fmt.Errorf("signer material resolver provider is nil")
	}

	signerMaterialResolverMutex.Lock()
	defer signerMaterialResolverMutex.Unlock()

	signerMaterialResolverProviderForBuild = provider

	return nil
}

// UnregisterSignerMaterialResolverProviderForBuild clears build-scoped resolver
// provider registration.
func UnregisterSignerMaterialResolverProviderForBuild() {
	signerMaterialResolverMutex.Lock()
	defer signerMaterialResolverMutex.Unlock()

	signerMaterialResolverProviderForBuild = nil
}

func currentSignerMaterialResolver() SignerMaterialResolver {
	signerMaterialResolverMutex.RLock()
	defer signerMaterialResolverMutex.RUnlock()

	return signerMaterialResolver
}

func currentSignerMaterialResolverProviderForBuild() SignerMaterialResolverProviderForBuild {
	signerMaterialResolverMutex.RLock()
	defer signerMaterialResolverMutex.RUnlock()

	return signerMaterialResolverProviderForBuild
}

func resolveSignerMaterial(privateKeyShare *tecdsa.PrivateKeyShare) (any, error) {
	resolver := currentSignerMaterialResolver()
	if resolver == nil {
		return nil, fmt.Errorf("signer material resolver is nil")
	}

	return resolver.ResolveSignerMaterial(privateKeyShare)
}
