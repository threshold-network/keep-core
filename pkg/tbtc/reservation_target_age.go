package tbtc

import "time"

// ReanchorTargetAgeTolerance is the maximum allowed lag between the leader's
// and a follower's Ethereum chain head when evaluating whether a re-anchor
// target wallet has aged out of its creation period. The leader and the
// follower read the chain head independently; they can be one or two blocks
// apart. A 24 s tolerance (two average 12 s blocks) keeps the two views
// from disagreeing on a target that sits right at the cutoff.
const ReanchorTargetAgeTolerance = 24 * time.Second

// IsReanchorTargetAged reports whether a re-anchor target wallet has aged out
// of its creation period as measured against the chain head time.
//
// A target is "aged" when createdAt + creationPeriod seconds <= headTime +
// tolerance. Callers pass the chain head block timestamp (from
// BridgeChain.GetLatestBlockTimestamp) for headTime, never the host wall
// clock.
func IsReanchorTargetAged(
	createdAt time.Time,
	creationPeriod uint32,
	headTime time.Time,
	tolerance time.Duration,
) bool {
	deadline := createdAt.Add(time.Duration(creationPeriod) * time.Second)
	return !deadline.After(headTime.Add(tolerance))
}
