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
	Message             *big.Int
	SessionID           string
	MemberIndex         group.MemberIndex
	// SignerMaterial carries backend-specific signer material.
	// Legacy backend expects *tecdsa.PrivateKeyShare.
	SignerMaterial any
	// PrivateKeyShare is a deprecated legacy alias kept for backward
	// compatibility while migrating to backend-specific signer material.
	PrivateKeyShare     *tecdsa.PrivateKeyShare
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
