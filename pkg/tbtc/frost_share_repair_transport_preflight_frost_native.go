//go:build frost_native

package tbtc

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const frostShareRepairTransportPreflightMaximumBytes int64 = 256 * 1024

// runFrostShareRepairTransportPreflight is the non-networked first half of the
// recovery ceremony. It runs only after manifest verification, state-anchor
// reconciliation, and installation of the native output barrier. The emitted
// artifact is public but immutable/owner-only so the offline authority can
// authenticate its operator source and bind the exact seat, store, and native
// transport key in the signed roster.
func runFrostShareRepairTransportPreflight(
	authorizationPath string,
	outputPath string,
	manifest FrostPreSignActivationRuntimeManifest,
	node *node,
	operatorAddress chain.Address,
) (bool, error) {
	if authorizationPath == "" && outputPath == "" {
		return false, nil
	}
	if authorizationPath == "" || outputPath == "" {
		return true, fmt.Errorf(
			"share-repair transport preflight input and output paths must be configured together",
		)
	}
	if node == nil || node.walletRegistry == nil ||
		manifest.ActivationAuthorityPublicKey == [32]byte{} {
		return true, fmt.Errorf("share-repair transport preflight dependencies are incomplete")
	}

	payload, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
		authorizationPath,
		frostShareRepairTransportPreflightMaximumBytes,
	)
	if err != nil {
		return true, fmt.Errorf("cannot read share-repair preflight authorization: %w", err)
	}
	authorization, err := frostsigning.DecodeShareRepairAuthorization(payload)
	if err != nil {
		return true, err
	}
	digest, err := frostsigning.ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return true, err
	}
	walletIDBytes, err := hex.DecodeString(authorization.WalletID[2:])
	if err != nil || len(walletIDBytes) != 32 {
		return true, fmt.Errorf("share-repair preflight wallet ID is invalid")
	}
	var walletID [32]byte
	copy(walletID[:], walletIDBytes)
	if err := validateFrostKeyGroupForWallet(authorization.KeyGroup, walletID); err != nil {
		return true, fmt.Errorf("share-repair preflight key group is invalid: %w", err)
	}

	wallet, found := node.walletRegistry.getWalletByID(walletID)
	if !found {
		return true, fmt.Errorf("authorized share-repair wallet is not active in the local registry")
	}
	if len(wallet.signingGroupOperators) != int(authorization.ParticipantCount) {
		return true, fmt.Errorf("authorized participant count differs from the local wallet")
	}
	signers := node.walletRegistry.getSigners(wallet.publicKey)
	if len(signers) == 0 {
		return true, fmt.Errorf("authorized share-repair wallet has no local signers")
	}
	participants := make(map[group.MemberIndex]struct{}, len(authorization.HelperIdentifiers)+1)
	for _, helper := range authorization.HelperIdentifiers {
		participants[group.MemberIndex(helper)] = struct{}{}
	}
	participants[group.MemberIndex(authorization.TargetIdentifier)] = struct{}{}
	localMemberIndexes := make([]group.MemberIndex, 0, len(signers))
	seenLocal := make(map[group.MemberIndex]struct{}, len(signers))
	for _, signer := range signers {
		material, ok := nativeSignerMaterialFromSigner(signer)
		if !ok {
			return true, fmt.Errorf("share-repair wallet contains non-native signer material")
		}
		keyGroup, err := frostsigning.KeyGroupIDFromSignerMaterial(material)
		if err != nil || keyGroup != authorization.KeyGroup {
			return true, fmt.Errorf("local signer material differs from the authorized key group")
		}
		member := signer.signingGroupMemberIndex
		if member == 0 || int(member) > len(wallet.signingGroupOperators) ||
			wallet.signingGroupOperators[int(member)-1] != operatorAddress {
			return true, fmt.Errorf("local share-repair seat is not owned by this operator")
		}
		if _, duplicate := seenLocal[member]; duplicate {
			return true, fmt.Errorf("local share-repair wallet contains a duplicate seat")
		}
		seenLocal[member] = struct{}{}
		if _, participating := participants[member]; participating {
			localMemberIndexes = append(localMemberIndexes, member)
		}
	}
	if len(localMemberIndexes) == 0 {
		return true, fmt.Errorf("this operator controls no seat named by the repair authorization")
	}
	slices.Sort(localMemberIndexes)

	engine, ok := frostsigning.CurrentNativeTBTCSignerEngine().(frostsigning.NativeTBTCSignerShareRepairEngine)
	if !ok || engine == nil {
		return true, fmt.Errorf("registered native signer does not support share repair")
	}
	entries := make(
		[]frostsigning.ShareRepairTransportPublicKey,
		0,
		len(localMemberIndexes),
	)
	for _, member := range localMemberIndexes {
		entry, err := frostsigning.PrepareShareRepairTransportRosterEntry(
			engine,
			authorization,
			ed25519.PublicKey(manifest.ActivationAuthorityPublicKey[:]),
			uint16(member),
		)
		if err != nil {
			return true, fmt.Errorf(
				"prepare native share-repair transport entry for seat [%d]: %w",
				member,
				err,
			)
		}
		entries = append(entries, *entry)
	}
	artifact, err := json.Marshal(frostsigning.ShareRepairTransportPreflight{
		Schema:                frostsigning.ShareRepairTransportPreflightSchema,
		AuthorizationDigest:   "0x" + hex.EncodeToString(digest[:]),
		ParticipantPublicKeys: entries,
	})
	if err != nil {
		return true, fmt.Errorf("encode share-repair transport preflight: %w", err)
	}
	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		outputPath,
		artifact,
	); err != nil {
		return true, fmt.Errorf("publish share-repair transport preflight: %w", err)
	}
	logger.Infof(
		"published FROST share-repair transport preflight for authorization [0x%x] with [%d] local seat(s); remove preflight paths before restart",
		digest,
		len(entries),
	)
	return true, nil
}
