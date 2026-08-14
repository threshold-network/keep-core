//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const defaultFrostShareRepairMaintenanceTimeout = 10 * time.Minute

func runFrostShareRepairMaintenance(
	ctx context.Context,
	authorizationPath string,
	timeout time.Duration,
	manifest FrostPreSignActivationRuntimeManifest,
	node *node,
	operatorAddress chain.Address,
) (bool, error) {
	if authorizationPath == "" {
		return false, nil
	}
	if ctx == nil || node == nil || node.walletRegistry == nil ||
		node.netProvider == nil || manifest.ActivationAuthorityPublicKey == [32]byte{} {
		return true, fmt.Errorf("share-repair maintenance dependencies are incomplete")
	}
	if timeout == 0 {
		timeout = defaultFrostShareRepairMaintenanceTimeout
	}
	if timeout < 10*time.Second || timeout > time.Hour {
		return true, fmt.Errorf("share-repair maintenance timeout must be from 10s through 1h")
	}

	payload, err := readSecureFrostActivationFile(authorizationPath, 256*1024)
	if err != nil {
		return true, fmt.Errorf("cannot read secure share-repair recovery bundle: %w", err)
	}
	bundle, err := frostsigning.DecodeShareRepairRecoveryBundle(payload)
	if err != nil {
		return true, err
	}
	authorization := &bundle.Authorization
	digest, err := frostsigning.ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return true, err
	}
	walletIDBytes, err := hex.DecodeString(authorization.WalletID[2:])
	if err != nil || len(walletIDBytes) != 32 {
		return true, fmt.Errorf("share-repair authorization wallet ID is invalid")
	}
	var walletID [32]byte
	copy(walletID[:], walletIDBytes)
	if err := validateFrostKeyGroupForWallet(authorization.KeyGroup, walletID); err != nil {
		return true, fmt.Errorf("share-repair authorization key group is invalid: %w", err)
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

	membershipValidator := group.NewMembershipValidator(
		logger,
		wallet.signingGroupOperators,
		node.chain.Signing(),
	)
	rosterDigest, err := frostsigning.ComputeShareRepairTransportRosterDigest(
		&bundle.TransportRoster,
		authorization,
	)
	if err != nil {
		return true, err
	}
	channelName := fmt.Sprintf(
		"%s-frost-share-repair-%x-%x",
		ProtocolName,
		digest,
		rosterDigest,
	)
	channel, err := node.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return true, fmt.Errorf("cannot open share-repair broadcast channel: %w", err)
	}
	if err := channel.SetFilter(membershipValidator.IsInGroup); err != nil {
		return true, fmt.Errorf("cannot authenticate share-repair broadcast channel: %w", err)
	}
	engine, ok := frostsigning.CurrentNativeTBTCSignerEngine().(frostsigning.NativeTBTCSignerShareRepairEngine)
	if !ok || engine == nil {
		return true, fmt.Errorf("registered native signer does not support share repair")
	}

	repairContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := frostsigning.RunShareRepair(
		repairContext,
		logger,
		channel,
		membershipValidator,
		engine,
		authorization,
		&bundle.TransportRoster,
		ed25519.PublicKey(manifest.ActivationAuthorityPublicKey[:]),
		localMemberIndexes,
	)
	if err != nil {
		return true, err
	}
	if result == nil {
		logger.Infof(
			"FROST share-repair helper maintenance completed for authorization [0x%x]",
			digest,
		)
	} else {
		logger.Infof(
			"FROST share-repair target seat [%d] durably installed and anchor-acknowledged authorization [0x%x]; production remains disabled pending old-store tombstone and signed activation",
			result.TargetIdentifier,
			digest,
		)
	}
	return true, nil
}
