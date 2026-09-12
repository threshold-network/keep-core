package group

import "fmt"

// MemberIndexFromUint32 converts a wire or persisted member index without
// truncating it, accepting any value in the range [0, MaxMemberIndex]. Zero
// retains its existing meaning at each protocol boundary; validation of
// group membership belongs to the protocol using the index. Persisted-signer
// callers must use MemberIndexFromUint32NonZero instead, since a persisted
// signer's own index is never re-checked against group membership after
// being loaded.
func MemberIndexFromUint32(value uint32) (MemberIndex, error) {
	if value > MaxMemberIndex {
		return 0, fmt.Errorf("invalid member index value: [%v]", value)
	}
	return MemberIndex(value), nil
}

// MemberIndexFromUint32NonZero converts a persisted member index, rejecting
// both out-of-range values and zero. Persisted-signer decode paths never
// re-check group membership after loading their own index, so a zero index
// must be rejected here: MembershipValidator.IsValidMembership computes
// int(memberID-1), which silently wraps a MemberIndex(0) to 255 and matches
// no real group position.
func MemberIndexFromUint32NonZero(value uint32) (MemberIndex, error) {
	memberIndex, err := MemberIndexFromUint32(value)
	if err != nil {
		return 0, err
	}
	if memberIndex == 0 {
		return 0, fmt.Errorf("invalid member index value: [%v]", value)
	}
	return memberIndex, nil
}
