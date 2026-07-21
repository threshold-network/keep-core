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
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	attempt *Attempt,
) (*Result, error) {
	request := &Request{
		Message:             message,
		SessionID:           sessionID,
		MemberIndex:         memberIndex,
		SignerMaterial:      privateKeyShare,
		PrivateKeyShare:     privateKeyShare,
		GroupSize:           groupSize,
		DishonestThreshold:  dishonestThreshold,
		Channel:             channel,
		MembershipValidator: membershipValidator,
		Attempt:             attempt,
	}

	return ExecuteRequest(ctx, logger, request)
}

// ExecuteRequest runs signing using a fully-populated request object.
// It clones mutable request metadata needed for execution safety.
func ExecuteRequest(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	clonedRequest := *request
	clonedRequest.Attempt = cloneAttempt(request.Attempt)
	if err := validateAuthorizationGuard(ctx, clonedRequest.AuthorizationGuard); err != nil {
		return nil, err
	}

	return currentExecutionBackend().Execute(
		ctx,
		logger,
		&clonedRequest,
	)
}

// RegisterUnmarshallers initializes all required message unmarshallers.
// For now, signing transport message formats are delegated to the legacy
// engine implementation.
func RegisterUnmarshallers(channel net.BroadcastChannel) {
	currentExecutionBackend().RegisterUnmarshallers(channel)
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
