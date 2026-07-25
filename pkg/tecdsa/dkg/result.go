package dkg

import (
	"crypto/ecdsa"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// Result of distributed key generation protocol.
type Result struct {
	// Group represents the group state, including members, disqualified,
	// and inactive members.
	Group *group.Group
	// PrivateKeyShare is the tECDSA private key share required to operate
	// in the signing group generated as result of the DKG protocol.
	PrivateKeyShare *tecdsa.PrivateKeyShare
}

// GroupPublicKey returns the public key corresponding to the private
// key share generated during the DKG protocol execution.
func (r *Result) GroupPublicKey() (*ecdsa.PublicKey, error) {
	if r.PrivateKeyShare == nil {
		return nil, fmt.Errorf(
			"cannot retrieve group public key as private key share is nil",
		)
	}

	return r.PrivateKeyShare.PublicKey(), nil
}

// GroupPublicKeyBytes returns the public key corresponding to the private
// key share generated during the DKG protocol execution. The resulting
// slice has 65 bytes and starts with the 04 prefix denoting an uncompressed
// key.
func (r *Result) GroupPublicKeyBytes() ([]byte, error) {
	publicKey, err := r.GroupPublicKey()
	if err != nil {
		return nil, err
	}

	return secp256k1.Marshal(publicKey), nil
}

// MisbehavedMembersIndexes returns the indexes of group members that misbehaved
// during the DKG procedure. The indexes are sorted.
func (r *Result) MisbehavedMembersIndexes() []group.MemberIndex {
	// Merge inactive and disqualified member indexes into 'misbehaved' set.
	misbehaving := make(map[group.MemberIndex]bool)
	for _, inactiveMemberIndex := range r.Group.InactiveMemberIndexes() {
		misbehaving[inactiveMemberIndex] = true
	}
	for _, disqualifiedMemberIndex := range r.Group.DisqualifiedMemberIndexes() {
		misbehaving[disqualifiedMemberIndex] = true
	}

	// Convert misbehaving member indexes set into sorted list.
	var sorted []group.MemberIndex
	for m := range misbehaving {
		sorted = append(sorted, m)
	}
	sort.Slice(sorted[:], func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	return sorted
}

const ResultSignatureHashByteSize = 32

// ResultSignatureHash is a signature hash of the DKG Result. The hashing
// algorithm used depends on the client code.
type ResultSignatureHash [ResultSignatureHashByteSize]byte

// ResultSignatureHashFromBytes converts bytes slice to ResultSignatureHash.
// It requires provided bytes slice size to be exactly
// ResultSignatureHashByteSize.
func ResultSignatureHashFromBytes(bytes []byte) (ResultSignatureHash, error) {
	var hash ResultSignatureHash

	if len(bytes) != ResultSignatureHashByteSize {
		return hash, fmt.Errorf(
			"bytes length is not equal %v", ResultSignatureHashByteSize,
		)
	}
	copy(hash[:], bytes[:])

	return hash, nil
}
