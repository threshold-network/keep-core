package signing

import (
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
	PrivateKeyShare     *tecdsa.PrivateKeyShare
	GroupSize           int
	DishonestThreshold  int
	Channel             net.BroadcastChannel
	MembershipValidator *group.MembershipValidator
	Attempt             *Attempt
}
