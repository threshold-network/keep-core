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

	payload, err := json.Marshal(tbtcSignerMaterialPayload{
		KeyGroup:                 hex.EncodeToString(keyGroupDigest[:]),
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
