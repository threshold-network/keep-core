package signing

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	legacySigning "github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

// Execute runs signing and returns a Schnorr-shaped 64-byte signature.
//
// Transitional note:
// This implementation currently delegates group coordination and cryptographic
// operations to the legacy tECDSA engine and converts the resulting (R, S)
// components to the fixed-width Schnorr signature container.
func Execute(
	ctx context.Context,
	logger log.StandardLogger,
	message *big.Int,
	sessionID string,
	memberIndex group.MemberIndex,
	privateKeyShare *tecdsa.PrivateKeyShare,
	groupSize int,
	dishonestThreshold int,
	excludedMembersIndexes []group.MemberIndex,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	attempt *Attempt,
) (*Result, error) {
	if attempt != nil {
		logger.Infof(
			"[member:%v] executing FROST signing attempt [%v] "+
				"with coordinator [%v] (included: [%v], excluded: [%v])",
			memberIndex,
			attempt.Number,
			attempt.CoordinatorMemberIndex,
			attempt.IncludedMembersIndexes,
			attempt.ExcludedMembersIndexes,
		)
	}

	legacyExcludedMembersIndexes := excludedMembersIndexes
	if attempt != nil && len(attempt.ExcludedMembersIndexes) > 0 {
		legacyExcludedMembersIndexes = attempt.ExcludedMembersIndexes
	}

	legacyResult, err := legacySigning.Execute(
		ctx,
		logger,
		message,
		sessionID,
		memberIndex,
		privateKeyShare,
		groupSize,
		dishonestThreshold,
		legacyExcludedMembersIndexes,
		channel,
		membershipValidator,
	)
	if err != nil {
		return nil, err
	}

	signature, err := FromTECDSASignature(legacyResult.Signature)
	if err != nil {
		return nil, err
	}

	return &Result{
		Signature: signature,
		Attempt:   cloneAttempt(attempt),
	}, nil
}

// RegisterUnmarshallers initializes all required message unmarshallers.
// For now, signing transport message formats are delegated to the legacy
// engine implementation.
func RegisterUnmarshallers(channel net.BroadcastChannel) {
	legacySigning.RegisterUnmarshallers(channel)
}

// FromTECDSASignature maps a legacy signature to the fixed-width Schnorr
// signature container by preserving R/S values and dropping RecoveryID.
func FromTECDSASignature(signature *tecdsa.Signature) (*frost.Signature, error) {
	if signature == nil {
		return nil, fmt.Errorf("signature is nil")
	}

	if signature.R == nil || signature.S == nil {
		return nil, fmt.Errorf("signature components cannot be nil")
	}

	if signature.R.Sign() < 0 || signature.S.Sign() < 0 {
		return nil, fmt.Errorf("signature components cannot be negative")
	}

	rBytes := signature.R.Bytes()
	sBytes := signature.S.Bytes()

	if len(rBytes) > frost.SignatureComponentSize {
		return nil, fmt.Errorf(
			"R component too large: [%d] bytes",
			len(rBytes),
		)
	}

	if len(sBytes) > frost.SignatureComponentSize {
		return nil, fmt.Errorf(
			"S component too large: [%d] bytes",
			len(sBytes),
		)
	}

	frostSignature := &frost.Signature{}
	copy(
		frostSignature.R[frost.SignatureComponentSize-len(rBytes):],
		rBytes,
	)
	copy(
		frostSignature.S[frost.SignatureComponentSize-len(sBytes):],
		sBytes,
	)

	return frostSignature, nil
}
