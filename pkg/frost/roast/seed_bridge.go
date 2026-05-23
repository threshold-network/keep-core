package roast

import (
	"encoding/binary"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// foldAttemptSeed reduces an RFC-21 [32]byte AttemptSeed to the legacy
// int64 seed accepted by SelectCoordinator. The reduction takes the
// first 8 bytes of the seed as a big-endian uint64 and re-interprets
// the bits as int64.
//
// This is a sterile, named adapter, *not* a cryptographic reduction.
// Its only contract is determinism: byte-identical input must produce
// byte-identical int64 output on every honest signer, so the
// SelectCoordinator shuffle remains in agreement across the network.
//
// The remaining 24 bytes of the seed are deliberately ignored. They
// are still part of the seed binding (so any change to those bytes is
// detected at the AttemptContext.Hash() layer, which protocol
// messages already verify in Phase 1B), but they do not influence the
// shuffle. SelectCoordinator's math.Rand source is non-cryptographic
// and 64 bits of entropy are sufficient for its purpose.
//
// Callers must not compose foldAttemptSeed with additional hashing.
// If a future RFC requires a different reduction it must be a new
// named bridge with its own tests and migration story.
func foldAttemptSeed(seed [attempt.AttemptSeedLength]byte) int64 {
	// #nosec G115 -- intentional uint64-to-int64 reinterpretation; the
	// downstream rand.Source accepts any int64, including negative.
	return int64(binary.BigEndian.Uint64(seed[:8]))
}
