package firewall

import (
	"fmt"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
)

// Application defines functionalities for operator verification in the firewall.
type Application interface {
	// IsRecognized returns true if the application recognizes the operator
	// as one participating in the application.
	IsRecognized(operatorPublicKey *operator.PublicKey) (bool, error)
}

// Disabled is an empty Firewall implementation enforcing no rules
// on the connection.
var Disabled = &noFirewall{}

type noFirewall struct{}

func (nf *noFirewall) Validate(remotePeerPublicKey *operator.PublicKey) error {
	return nil
}

// AllowList represents a list of operator public keys that are not checked
// against the firewall rules and are always valid peers.
type AllowList struct {
	allowedPublicKeys map[string]bool
}

// NewAllowList creates a new firewall's allowlist based on the given public
// key list.
func NewAllowList(operatorPublicKeys []*operator.PublicKey) *AllowList {
	allowedPublicKeys := make(map[string]bool, len(operatorPublicKeys))

	for _, operatorPublicKey := range operatorPublicKeys {
		allowedPublicKeys[operatorPublicKey.String()] = true
	}

	return &AllowList{allowedPublicKeys}
}

func (al *AllowList) Contains(operatorPublicKey *operator.PublicKey) bool {
	return al.allowedPublicKeys[operatorPublicKey.String()]
}

// emptyAllowList is the singleton empty allowlist used in production.
// All peers must pass IsRecognized checks; no bypass is available.
var emptyAllowList = NewAllowList([]*operator.PublicKey{})

// EmptyAllowList returns the empty firewall allowlist. In production, this
// ensures all peers are subject to on-chain staking verification with no
// AllowList bypass.
func EmptyAllowList() *AllowList {
	return emptyAllowList
}

const (
	// NegativeIsRecognizedCachePeriod is the time period the cache maintains
	// the negative result of the last `IsRecognized` checks.
	// We use the cache to minimize calls to the on-chain client.
	NegativeIsRecognizedCachePeriod = 1 * time.Hour
)

var errNotRecognized = fmt.Errorf(
	"remote peer has not been recognized by any application",
)

func AnyApplicationPolicy(
	applications []Application,
	allowList *AllowList,
) net.Firewall {
	return &anyApplicationPolicy{
		applications:        applications,
		allowList:           allowList,
		negativeResultCache: cache.NewTimeCache(NegativeIsRecognizedCachePeriod),
	}
}

type anyApplicationPolicy struct {
	applications        []Application
	allowList           *AllowList
	negativeResultCache *cache.TimeCache
}

// Validate checks whether the given operator meets the conditions to join
// the network. The operator can join the network if it is an allowlisted node
// or it is a non-allowlisted node but it is recognized as eligible by any of
// the applications. Nil is returned on a successful validation, error otherwise.
// Due to performance reasons, negative validation results for non-allowlisted
// nodes are stored in a cache for a certain amount of time. Positive results
// are intentionally not cached so that operator revocations take effect on the
// next Validate call rather than after a cache TTL.
func (aap *anyApplicationPolicy) Validate(
	remotePeerPublicKey *operator.PublicKey,
) error {
	// If the peer is on the allowlist, consider it validated.
	if aap.allowList.Contains(remotePeerPublicKey) {
		return nil
	}

	// First, check in the in-memory time cache to minimize hits to the ETH client.
	// If the client is in the negative result cache it means it hasn't been
	// recognized when the last `IsRecognized` was executed and caching period
	// has not elapsed yet.
	//
	// If the caching period elapsed, cache checks will return false and we
	// have to ask the chain about the current status.
	aap.negativeResultCache.Sweep()

	remotePeerPublicKeyHex := remotePeerPublicKey.String()

	if aap.negativeResultCache.Has(remotePeerPublicKeyHex) {
		return errNotRecognized
	}

	validationSuccessful := false
	for _, application := range aap.applications {
		isRecognized, err := application.IsRecognized(remotePeerPublicKey)
		if err != nil {
			return fmt.Errorf(
				"could not validate if remote peer is recognized by application: [%w]",
				err,
			)
		}
		if isRecognized {
			validationSuccessful = true
			break
		}
	}

	if !validationSuccessful {
		// Add this address to the negative result cache.
		// `IsRecognized` will not be called again for the entire caching period.
		aap.negativeResultCache.Add(remotePeerPublicKeyHex)
		return errNotRecognized
	}

	return nil
}
