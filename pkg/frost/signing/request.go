package signing

import (
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
