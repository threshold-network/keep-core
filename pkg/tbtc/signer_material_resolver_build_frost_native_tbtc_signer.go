//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// buildTaggedNativeSignerMaterialResolver derives transitional signer material
// for frost_tbtc_signer builds. It carries a deterministic key-group handle and
// embeds legacy private-key-share bytes to preserve temporary Go-side fallback.
type buildTaggedNativeSignerMaterialResolver struct{}

func (btnsmr *buildTaggedNativeSignerMaterialResolver) ResolveSignerMaterial(
	privateKeyShare *tecdsa.PrivateKeyShare,
) (any, error) {
	if privateKeyShare == nil {
		return nil, fmt.Errorf("private key share is nil")
	}

	legacyPrivateKeySharePayload, err := privateKeyShare.Marshal()
	if err != nil {
		return nil, fmt.Errorf("cannot marshal private key share: [%w]", err)
	}

	walletPublicKeyBytes, err := marshalPublicKey(privateKeyShare.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("cannot marshal wallet public key: [%w]", err)
	}

	keyGroupDigest := sha256.Sum256(walletPublicKeyBytes)

	// Scaffold-era key-group derivation: the current value identifies
	// placeholder material derived from the legacy wallet public-key hash,
	// not the output of a real FROST DKG run. Refuse to surface that material
	// at all unless the operator has explicitly opted in via
	// AcceptScaffoldKeyGroupEnvVar — production deployments must never set
	// this. See native_tbtc_signer_material.go for the env-var contract.
	if !frostsigning.AcceptScaffoldKeyGroupEnabled() {
		return nil, fmt.Errorf(
			"refusing to build scaffold-era %q signer material; set %s=true to "+
				"opt in for local/CI use only, never in production",
			frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			frostsigning.AcceptScaffoldKeyGroupEnvVar,
		)
	}

	// TODO: Replace this placeholder key-group derivation with Rust DKG output.
	// The current value identifies scaffold-era material only.
	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:                 hex.EncodeToString(keyGroupDigest[:]),
		KeyGroupSource:           frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
		LegacyPrivateKeyShareHex: hex.EncodeToString(legacyPrivateKeySharePayload),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal tbtc signer material payload: [%w]", err)
	}

	return &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}, nil
}
