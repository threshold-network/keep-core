package tbtc

import "math"

// UnixSecondsToUint32 converts a UNIX timestamp in seconds to the uint32
// representation the reservation contracts use for their timestamps and
// deadlines. Validation and narrowing stay together in this function, so a
// caller cannot check one value and convert another.
//
// A timestamp outside the uint32 range (before 1970, or after 2106-02-07)
// saturates at the nearer bound instead of wrapping into the opposite half of
// the range: every caller compares the result against an on-chain deadline,
// and a wrapped clock would silently read as the wrong side of that deadline
// (or as a time decades in the past). Real values are returned unchanged.
func UnixSecondsToUint32(seconds int64) uint32 {
	if seconds < 0 {
		return 0
	}
	if seconds > math.MaxUint32 {
		return math.MaxUint32
	}

	return uint32(seconds)
}
