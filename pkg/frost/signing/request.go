package signing

import (
	"context"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// Request carries execution input for a FROST signing backend.
type Request struct {
	Message   *big.Int
	SessionID string
	// AuthorizationGuard revalidates an external authorization immediately
	// before secret nonce/share boundaries. It is nil for signing flows that do
	// not use an external authorization protocol.
	AuthorizationGuard func(context.Context) error
	// SigningIntent carries a narrowly typed authorization artifact for messages
	// that are not transaction sighashes. It is nil for generic and transaction
	// signing. Today the only supported value is a heartbeat intent created with
	// NewHeartbeatSigningIntent.
	SigningIntent *SigningIntent
	// RoastSessionID is the STABLE per-signing ROAST session id (derived from
	// message+root+startBlock, WITHOUT the attempt number), used for ROAST
	// orchestration, AttemptContext.SessionID, the transition-record registry,
	// the selector lookup, and the interactive engine session. SessionID stays
	// attempt-specific for the coarse/legacy execution path and its replay
	// isolation; this stable id lets cross-attempt ROAST state (the previous
	// attempt's transition record) be found by the next attempt's selector.
	// Empty when the caller does not drive ROAST orchestration; callers that
	// build an AttemptContext fall back to SessionID.
	RoastSessionID string
	MemberIndex    group.MemberIndex
	// SignerMaterial carries backend-specific signer material.
	// Legacy backend expects *tecdsa.PrivateKeyShare.
	SignerMaterial any
	// PrivateKeyShare is a deprecated legacy alias kept for backward
	// compatibility while migrating to backend-specific signer material.
	PrivateKeyShare *tecdsa.PrivateKeyShare
	// TaprootMerkleRoot carries the optional BIP-341 script merkle root used
	// to tweak a Taproot key-path signature.
	TaprootMerkleRoot   *[32]byte
	GroupSize           int
	DishonestThreshold  int
	Channel             net.BroadcastChannel
	MembershipValidator *group.MembershipValidator
	Attempt             *Attempt
}

func validateAuthorizationGuard(
	ctx context.Context,
	guard func(context.Context) error,
) error {
	if guard == nil {
		return nil
	}
	if err := guard(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf(
			"%w: external signing authorization is invalid: %v",
			ErrTerminalSigningFailure,
			err,
		)
	}
	return nil
}

// SigningIntent is a closed, immutable signing-intent value. Its fields stay
// private so callers cannot manufacture an unvalidated intent shape; exported
// constructors are the only way to create one.
type SigningIntent struct {
	heartbeatMessage *[16]byte
}

// NewHeartbeatSigningIntent authorizes the canonical heartbeat signing message
// derived from the exact 16-byte proposal payload. The payload is copied so it
// cannot be changed while a concurrent signing attempt is in flight.
func NewHeartbeatSigningIntent(message [16]byte) *SigningIntent {
	messageCopy := message
	return &SigningIntent{heartbeatMessage: &messageCopy}
}

// HeartbeatMessage returns the raw heartbeat proposal payload carried by this
// intent. The returned array is a copy.
func (si *SigningIntent) HeartbeatMessage() ([16]byte, bool) {
	if si == nil || si.heartbeatMessage == nil {
		return [16]byte{}, false
	}

	return *si.heartbeatMessage, true
}

func cloneSigningIntent(intent *SigningIntent) *SigningIntent {
	if intent == nil {
		return nil
	}

	cloned := *intent
	if intent.heartbeatMessage != nil {
		messageCopy := *intent.heartbeatMessage
		cloned.heartbeatMessage = &messageCopy
	}

	return &cloned
}

// LegacyPrivateKeyShare resolves the tECDSA private key share required by the
// transitional legacy execution backend.
//
// It first checks the deprecated Request.PrivateKeyShare field for backward
// compatibility, and then falls back to Request.SignerMaterial.
func (r *Request) LegacyPrivateKeyShare() (*tecdsa.PrivateKeyShare, error) {
	if r == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if r.PrivateKeyShare != nil {
		return r.PrivateKeyShare, nil
	}

	if r.SignerMaterial == nil {
		return nil, fmt.Errorf("legacy private key share is nil")
	}

	privateKeyShare, ok := r.SignerMaterial.(*tecdsa.PrivateKeyShare)
	if !ok {
		return nil, fmt.Errorf(
			"legacy signing material has wrong type: [%T]",
			r.SignerMaterial,
		)
	}

	if privateKeyShare == nil {
		return nil, fmt.Errorf("legacy private key share is nil")
	}

	return privateKeyShare, nil
}
