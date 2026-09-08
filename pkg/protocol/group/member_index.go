package group

import "fmt"

// MemberIndexFromUint32 converts a wire or persisted member index without
// truncating it. Zero retains its existing meaning at each protocol boundary;
// validation of group membership belongs to the protocol using the index.
func MemberIndexFromUint32(value uint32) (MemberIndex, error) {
	if value > MaxMemberIndex {
		return 0, fmt.Errorf("invalid member index value: [%v]", value)
	}
	return MemberIndex(value), nil
}
