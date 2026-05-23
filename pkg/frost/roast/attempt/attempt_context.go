// Package attempt implements the AttemptContext type that binds every
// signing-protocol message to a deterministic, group-agreed context.
//
// This package is the Phase 1 deliverable from RFC-21 (ROAST Coordinator,
// Retry, and Transition Evidence). It introduces only the type, its
// deterministic seed derivation, and the canonical hash used to bind
// protocol messages to an attempt. No protocol behaviour changes in this
// phase; consumers are wired in later phases behind build tags.
package attempt

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// MessageDigestLength is the canonical byte length of a signing-message
// digest carried in AttemptContext. The protocol always uses SHA-256
// digests of the BIP-340 tag-bound payload, so 32 bytes is correct for
// every signing flow this package is concerned with.
const MessageDigestLength = 32

// AttemptSeedLength is the canonical byte length of the per-attempt
// participant-shuffle seed. The seed is derived, never chosen --
// see DeriveAttemptSeed.
const AttemptSeedLength = 32

// AttemptContext binds an in-flight ROAST signing attempt to a
// deterministic context. Every honest signer must construct the same
// AttemptContext for a given (session, key group, message, attempt
// number) and must reject any protocol message whose AttemptContextHash
// does not match the locally-computed context.
//
// AttemptContext fields are public so test fixtures can construct
// contexts directly, but production callers should use NewAttemptContext
// which validates inputs and derives the seed.
type AttemptContext struct {
	// SessionID identifies the signing session at the keep-core layer.
	// It is opaque to the ROAST coordinator; the coordinator only
	// requires it to be stable across the session's attempts.
	SessionID string
	// KeyGroupID identifies the FROST key group whose threshold share
	// will sign. It is opaque to the coordinator; equality across honest
	// signers is required.
	KeyGroupID string
	// MessageDigest is the 32-byte SHA-256 digest of the BIP-340
	// tag-bound signing message.
	MessageDigest [MessageDigestLength]byte
	// AttemptNumber is the zero-based ordinal of this attempt within
	// the session. Attempt 0 is the first attempt; later attempts are
	// driven by NextAttempt in the coordinator state machine
	// (introduced in later RFC-21 phases).
	AttemptNumber uint32
	// IncludedSet is the set of member indices that are eligible to
	// participate in this attempt. Must be sorted ascending. Must not
	// be empty.
	IncludedSet []group.MemberIndex
	// ExcludedSet is the set of member indices permanently excluded
	// from this attempt by the coordinator's transition-evidence
	// policy. Must be sorted ascending. May be empty. Permanent
	// exclusion follows from transport-blamable (overflow) or
	// validation-blamable (non-transport reject) evidence, never
	// from silence alone.
	ExcludedSet []group.MemberIndex
	// TransientlyParked is the set of member indices skipped from
	// THIS attempt only because they were silent (deadline expiry)
	// at the previous attempt. Parking is strictly transient: a
	// peer is unparked at the attempt after the one that skipped
	// them, so a falsely-silenced honest peer (network blip,
	// coordinator censorship caught at VerifyBundle) is reinstated
	// without intervention. Must be sorted ascending. May be empty.
	TransientlyParked []group.MemberIndex
	// AttemptSeed is derived from group-agreed inputs and binds the
	// attempt to inputs that no coordinator can manipulate. See
	// DeriveAttemptSeed.
	AttemptSeed [AttemptSeedLength]byte
}

// DeriveAttemptSeed computes the per-attempt seed from inputs the group
// already agrees on. The seed binds the attempt's participant selection
// to fixed session inputs so a coordinator cannot shape the shuffle by
// picking a favourable seed.
//
// The derivation is:
//
//	AttemptSeed = SHA256(
//	    DkgGroupPublicKey || SessionID || MessageDigest,
//	)
//
// Where SessionID is encoded as the raw UTF-8 bytes (the canonical
// representation used elsewhere in keep-core) and the other inputs are
// raw bytes.
func DeriveAttemptSeed(
	dkgGroupPublicKey []byte,
	sessionID string,
	messageDigest [MessageDigestLength]byte,
) [AttemptSeedLength]byte {
	h := sha256.New()
	h.Write(dkgGroupPublicKey)
	h.Write([]byte(sessionID))
	h.Write(messageDigest[:])
	var out [AttemptSeedLength]byte
	copy(out[:], h.Sum(nil))
	return out
}

// NewAttemptContext constructs an AttemptContext with the seed derived
// from group-agreed inputs. The IncludedSet and ExcludedSet are sorted
// ascending in the returned context regardless of input order; honest
// signers therefore produce identical contexts from identical input
// values.
//
// Returns an error if the included set is empty, if any member appears
// in both sets, or if either set contains duplicates.
//
// This is the seven-argument convenience that initialises an attempt
// with no TransientlyParked entries (the attempt-zero shape). For
// later attempts produced by the coordinator's NextAttempt policy,
// use NewAttemptContextWithParking.
func NewAttemptContext(
	sessionID string,
	keyGroupID string,
	dkgGroupPublicKey []byte,
	messageDigest [MessageDigestLength]byte,
	attemptNumber uint32,
	includedSet []group.MemberIndex,
	excludedSet []group.MemberIndex,
) (AttemptContext, error) {
	return NewAttemptContextWithParking(
		sessionID,
		keyGroupID,
		dkgGroupPublicKey,
		messageDigest,
		attemptNumber,
		includedSet,
		excludedSet,
		nil,
	)
}

// NewAttemptContextWithParking is the full constructor used by the
// coordinator's NextAttempt policy. It accepts a transientlyParked
// set in addition to the inputs of NewAttemptContext.
//
// Validation: included set non-empty; no duplicates in any set;
// included/excluded sets disjoint; included/parked sets disjoint;
// excluded/parked sets disjoint.
func NewAttemptContextWithParking(
	sessionID string,
	keyGroupID string,
	dkgGroupPublicKey []byte,
	messageDigest [MessageDigestLength]byte,
	attemptNumber uint32,
	includedSet []group.MemberIndex,
	excludedSet []group.MemberIndex,
	transientlyParked []group.MemberIndex,
) (AttemptContext, error) {
	if len(includedSet) == 0 {
		return AttemptContext{}, errors.New(
			"attempt context: included set must not be empty",
		)
	}
	included, err := canonicalMemberSet(includedSet, "included")
	if err != nil {
		return AttemptContext{}, err
	}
	excluded, err := canonicalMemberSet(excludedSet, "excluded")
	if err != nil {
		return AttemptContext{}, err
	}
	parked, err := canonicalMemberSet(transientlyParked, "transiently parked")
	if err != nil {
		return AttemptContext{}, err
	}
	if hasOverlap(included, excluded) {
		return AttemptContext{}, errors.New(
			"attempt context: included and excluded sets overlap",
		)
	}
	if hasOverlap(included, parked) {
		return AttemptContext{}, errors.New(
			"attempt context: included and transiently-parked sets overlap",
		)
	}
	if hasOverlap(excluded, parked) {
		return AttemptContext{}, errors.New(
			"attempt context: excluded and transiently-parked sets overlap",
		)
	}
	return AttemptContext{
		SessionID:         sessionID,
		KeyGroupID:        keyGroupID,
		MessageDigest:     messageDigest,
		AttemptNumber:     attemptNumber,
		IncludedSet:       included,
		ExcludedSet:       excluded,
		TransientlyParked: parked,
		AttemptSeed: DeriveAttemptSeed(
			dkgGroupPublicKey,
			sessionID,
			messageDigest,
		),
	}, nil
}

// Hash returns the canonical 32-byte hash of the attempt context. The
// hash is the SHA-256 of a length-prefixed, sorted-set canonical
// encoding so any two honest signers that construct semantically equal
// AttemptContexts produce byte-identical hashes regardless of input
// ordering.
//
// The hash is the value carried in protocol messages as
// AttemptContextHash. A receiver that computes a different hash than
// the one carried by an inbound message must reject the message: it
// belongs to a different attempt.
func (c AttemptContext) Hash() [MessageDigestLength]byte {
	h := sha256.New()
	writeLenPrefixed(h, []byte(c.SessionID))
	writeLenPrefixed(h, []byte(c.KeyGroupID))
	h.Write(c.MessageDigest[:])
	var attemptNumberBuf [4]byte
	binary.BigEndian.PutUint32(attemptNumberBuf[:], c.AttemptNumber)
	h.Write(attemptNumberBuf[:])
	writeMemberSet(h, c.IncludedSet)
	writeMemberSet(h, c.ExcludedSet)
	writeMemberSet(h, c.TransientlyParked)
	h.Write(c.AttemptSeed[:])
	var out [MessageDigestLength]byte
	copy(out[:], h.Sum(nil))
	return out
}

func canonicalMemberSet(
	members []group.MemberIndex,
	label string,
) ([]group.MemberIndex, error) {
	if len(members) == 0 {
		return []group.MemberIndex{}, nil
	}
	out := make([]group.MemberIndex, len(members))
	copy(out, members)
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	for i := 1; i < len(out); i++ {
		if out[i] == out[i-1] {
			return nil, fmt.Errorf(
				"attempt context: %s set contains duplicate member [%d]",
				label,
				out[i],
			)
		}
	}
	return out, nil
}

func hasOverlap(a, b []group.MemberIndex) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			return true
		}
	}
	return false
}

// byteWriter is the subset of io.Writer the canonical-encoding helpers
// need. Hash.Write (the only production implementation) is documented to
// never return an error, so the helpers discard the (int, error) result
// explicitly to make that contract reader-visible (and to satisfy gosec
// G104).
type byteWriter interface {
	Write(p []byte) (n int, err error)
}

func writeLenPrefixed(w byteWriter, data []byte) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	_, _ = w.Write(lenBuf[:])
	_, _ = w.Write(data)
}

func writeMemberSet(w byteWriter, members []group.MemberIndex) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(members)))
	_, _ = w.Write(lenBuf[:])
	for _, m := range members {
		_, _ = w.Write([]byte{byte(m)})
	}
}
