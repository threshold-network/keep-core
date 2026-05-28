package registry

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/frost"
)

const (
	// ResultDigestVersion is the literal version tag used by
	// FrostDkgValidator.resultDigest.
	ResultDigestVersion = "tbtc-frost-dkg-result-v1"
)

var (
	uint32ArrayType = mustABIType("uint32[]")
	uint8ArrayType  = mustABIType("uint8[]")
	uint256Type     = mustABIType("uint256")
	addressType     = mustABIType("address")
	bytes32Type     = mustABIType("bytes32")
	stringType      = mustABIType("string")
)

// FullMembers is the full selected group returned by the FROST sortition pool.
//
// This slice is used in the v4 digest and submitted result. Do not filter
// misbehaved members before using it for those purposes.
type FullMembers []uint32

// ActiveMembers is the filtered group after excluding 1-based misbehaved
// member indices. This slice is used for the submitted membersHash only.
type ActiveMembers []uint32

// MisbehavedMemberIndices holds sorted, unique, 1-based indices into
// FullMembers.
type MisbehavedMemberIndices []uint8

// Result contains the FROST DKG result fields submitted to FrostWalletRegistry.
type Result struct {
	SubmitterMemberIndex     uint64
	XOnlyOutputKey           frost.OutputKey
	MembersHash              [32]byte
	MisbehavedMembersIndices MisbehavedMemberIndices
	Signatures               []byte
	SigningMembersIndices    []uint64
	Members                  FullMembers
}

// ActiveMembersFromMisbehaved returns the filtered active set used to compute
// Result.MembersHash.
func ActiveMembersFromMisbehaved(
	members FullMembers,
	misbehaved MisbehavedMemberIndices,
) (ActiveMembers, error) {
	if err := validateMisbehavedMemberIndices(len(members), misbehaved); err != nil {
		return nil, err
	}

	if len(misbehaved) == 0 {
		active := make(ActiveMembers, len(members))
		copy(active, members)
		return active, nil
	}

	active := make(ActiveMembers, 0, len(members)-len(misbehaved))
	misbehavedCursor := 0
	for i, member := range members {
		memberIndex := uint8(i + 1)
		if misbehavedCursor < len(misbehaved) &&
			memberIndex == misbehaved[misbehavedCursor] {
			misbehavedCursor++
			continue
		}

		active = append(active, member)
	}

	return active, nil
}

// AssembleResult builds a Result while keeping full and active member sets
// distinct. The full members list is persisted in the result and signed in the
// v4 digest; the filtered active members list is hashed into membersHash.
func AssembleResult(
	submitterMemberIndex uint64,
	xOnlyOutputKey frost.OutputKey,
	members FullMembers,
	misbehaved MisbehavedMemberIndices,
	signatures []byte,
	signingMembersIndices []uint64,
) (*Result, error) {
	activeMembers, err := ActiveMembersFromMisbehaved(members, misbehaved)
	if err != nil {
		return nil, err
	}

	membersHash, err := ActiveMembersHash(activeMembers)
	if err != nil {
		return nil, err
	}

	result := &Result{
		SubmitterMemberIndex:     submitterMemberIndex,
		XOnlyOutputKey:           xOnlyOutputKey,
		MembersHash:              membersHash,
		MisbehavedMembersIndices: append(MisbehavedMemberIndices{}, misbehaved...),
		Signatures:               append([]byte{}, signatures...),
		SigningMembersIndices:    append([]uint64{}, signingMembersIndices...),
		Members:                  append(FullMembers{}, members...),
	}

	return result, nil
}

// FullMembersHash returns keccak256(abi.encode(uint32[])).
func FullMembersHash(members FullMembers) ([32]byte, error) {
	return uint32ArrayHash([]uint32(members))
}

// ActiveMembersHash returns keccak256(abi.encode(uint32[])).
func ActiveMembersHash(members ActiveMembers) ([32]byte, error) {
	return uint32ArrayHash([]uint32(members))
}

// MisbehavedMembersHash returns keccak256(abi.encode(uint8[])).
func MisbehavedMembersHash(
	misbehaved MisbehavedMemberIndices,
) ([32]byte, error) {
	args := abi.Arguments{{Type: uint8ArrayType}}
	encoded, err := args.Pack([]uint8(misbehaved))
	if err != nil {
		return [32]byte{}, err
	}

	return crypto.Keccak256Hash(encoded), nil
}

// ResultDigest computes the pre-EIP-191 v4 digest expected by
// FrostDkgValidator.resultDigest.
func ResultDigest(
	chainID *big.Int,
	bridge common.Address,
	registry common.Address,
	seed *big.Int,
	xOnlyOutputKey frost.OutputKey,
	members FullMembers,
	misbehaved MisbehavedMemberIndices,
) ([32]byte, error) {
	if chainID == nil {
		return [32]byte{}, fmt.Errorf("chain ID is nil")
	}
	if bridge == (common.Address{}) {
		return [32]byte{}, fmt.Errorf("bridge address is zero")
	}
	if registry == (common.Address{}) {
		return [32]byte{}, fmt.Errorf("registry address is zero")
	}
	if seed == nil {
		return [32]byte{}, fmt.Errorf("seed is nil")
	}

	fullMembersHash, err := FullMembersHash(members)
	if err != nil {
		return [32]byte{}, err
	}

	misbehavedHash, err := MisbehavedMembersHash(misbehaved)
	if err != nil {
		return [32]byte{}, err
	}

	args := abi.Arguments{
		{Type: stringType},
		{Type: uint256Type},
		{Type: addressType},
		{Type: addressType},
		{Type: uint256Type},
		{Type: bytes32Type},
		{Type: bytes32Type},
		{Type: bytes32Type},
	}

	encoded, err := args.Pack(
		ResultDigestVersion,
		chainID,
		bridge,
		registry,
		seed,
		[32]byte(xOnlyOutputKey),
		fullMembersHash,
		misbehavedHash,
	)
	if err != nil {
		return [32]byte{}, err
	}

	return crypto.Keccak256Hash(encoded), nil
}

// EthereumSignedMessageHash returns the go-ethereum personal-sign hash:
// keccak256("\x19Ethereum Signed Message:\n32" || digest).
func EthereumSignedMessageHash(digest [32]byte) [32]byte {
	prefixed := make([]byte, 0, 28+len(digest))
	prefixed = append(prefixed, []byte("\x19Ethereum Signed Message:\n32")...)
	prefixed = append(prefixed, digest[:]...)

	return crypto.Keccak256Hash(prefixed)
}

func uint32ArrayHash(members []uint32) ([32]byte, error) {
	args := abi.Arguments{{Type: uint32ArrayType}}
	encoded, err := args.Pack(members)
	if err != nil {
		return [32]byte{}, err
	}

	return crypto.Keccak256Hash(encoded), nil
}

func validateMisbehavedMemberIndices(
	groupSize int,
	misbehaved MisbehavedMemberIndices,
) error {
	if groupSize > 255 {
		return fmt.Errorf("group size [%d] exceeds uint8 member index capacity", groupSize)
	}

	var previous uint8
	for i, memberIndex := range misbehaved {
		if memberIndex == 0 || int(memberIndex) > groupSize {
			return fmt.Errorf(
				"misbehaved member index [%d] out of range [1, %d]",
				memberIndex,
				groupSize,
			)
		}

		if i > 0 && memberIndex <= previous {
			return fmt.Errorf("misbehaved member indices must be sorted and unique")
		}

		previous = memberIndex
	}

	return nil
}

func mustABIType(name string) abi.Type {
	t, err := abi.NewType(name, "", nil)
	if err != nil {
		panic(err)
	}

	return t
}
