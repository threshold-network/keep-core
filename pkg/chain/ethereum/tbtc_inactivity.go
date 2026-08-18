// tbtc_inactivity.go: inactivity-claim lifecycle for the TbtcChain adapter.
package ethereum

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/pkg/chain"
	ecdsaabi "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func (tc *TbtcChain) OnInactivityClaimed(
	handler func(event *tbtc.InactivityClaimedEvent),
) subscription.EventSubscription {
	onEvent := func(
		walletID [32]byte,
		nonce *big.Int,
		notifier common.Address,
		blockNumber uint64,
	) {
		handler(&tbtc.InactivityClaimedEvent{
			WalletID:    walletID,
			Nonce:       nonce,
			Notifier:    chain.Address(notifier.Hex()),
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.InactivityClaimedEvent(nil, nil).OnEvent(onEvent)
}

func (tc *TbtcChain) AssembleInactivityClaim(
	walletID [32]byte,
	inactiveMembersIndices []group.MemberIndex,
	signatures map[group.MemberIndex][]byte,
	heartbeatFailed bool,
) (
	*tbtc.InactivityClaim,
	error,
) {
	signingMemberIndices, signatureBytes, err := convertSignaturesToChainFormat(
		signatures,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not convert signatures to chain format: [%v]",
			err,
		)
	}

	return &tbtc.InactivityClaim{
		WalletID:               walletID,
		InactiveMembersIndices: inactiveMembersIndices,
		HeartbeatFailed:        heartbeatFailed,
		Signatures:             signatureBytes,
		SigningMembersIndices:  signingMemberIndices,
	}, nil
}

// convertInactivityClaimToAbiType converts the TBTC-specific inactivity claim
// to the format applicable for the WalletRegistry ABI.
func convertInactivityClaimToAbiType(
	claim *tbtc.InactivityClaim,
) ecdsaabi.EcdsaInactivityClaim {
	inactiveMembersIndices := make([]*big.Int, len(claim.InactiveMembersIndices))
	for i, memberIndex := range claim.InactiveMembersIndices {
		inactiveMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	signingMembersIndices := make([]*big.Int, len(claim.SigningMembersIndices))
	for i, memberIndex := range claim.SigningMembersIndices {
		signingMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	return ecdsaabi.EcdsaInactivityClaim{
		WalletID:               claim.WalletID,
		InactiveMembersIndices: inactiveMembersIndices,
		HeartbeatFailed:        claim.HeartbeatFailed,
		Signatures:             claim.Signatures,
		SigningMembersIndices:  signingMembersIndices,
	}
}

func (tc *TbtcChain) SubmitInactivityClaim(
	claim *tbtc.InactivityClaim,
	nonce *big.Int,
	groupMembers []uint32,
) error {
	_, err := tc.walletRegistry.NotifyOperatorInactivity(
		convertInactivityClaimToAbiType(claim),
		nonce,
		groupMembers,
	)

	return err
}

func (tc *TbtcChain) CalculateInactivityClaimHash(
	claim *inactivity.ClaimPreimage,
) (inactivity.ClaimHash, error) {
	walletPublicKeyBytes := secp256k1.Marshal(claim.WalletPublicKey)
	// Crop the 04 prefix as the calculateInactivityClaimHash function expects
	// an unprefixed 64-byte public key,
	unprefixedGroupPublicKeyBytes := walletPublicKeyBytes[1:]

	// The type representing inactive member index should be `big.Int` as the
	// smart contract reading the calculated hash uses `uint256` for inactive
	// member indexes.
	inactiveMembersIndexes := make([]*big.Int, len(claim.InactiveMembersIndexes))
	for i, index := range claim.InactiveMembersIndexes {
		inactiveMembersIndexes[i] = big.NewInt(int64(index))
	}

	return calculateInactivityClaimHash(
		tc.chainID,
		claim.Nonce,
		unprefixedGroupPublicKeyBytes,
		inactiveMembersIndexes,
		claim.HeartbeatFailed,
	)
}

func calculateInactivityClaimHash(
	chainID *big.Int,
	nonce *big.Int,
	walletPublicKey []byte,
	inactiveMembersIndexes []*big.Int,
	heartbeatFailed bool,
) (inactivity.ClaimHash, error) {
	publicKeySize := 64

	if len(walletPublicKey) != publicKeySize {
		return inactivity.ClaimHash{}, fmt.Errorf(
			"wrong wallet public key length",
		)
	}

	uint256Type, err := abi.NewType("uint256", "uint256", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	bytesType, err := abi.NewType("bytes", "bytes", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	uint256SliceType, err := abi.NewType("uint256[]", "uint256[]", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	boolType, err := abi.NewType("bool", "bool", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	bytes, err := abi.Arguments{
		{Type: uint256Type},
		{Type: uint256Type},
		{Type: bytesType},
		{Type: uint256SliceType},
		{Type: boolType},
	}.Pack(
		chainID,
		nonce,
		walletPublicKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	return inactivity.ClaimHash(crypto.Keccak256Hash(bytes)), nil
}

func (tc *TbtcChain) GetInactivityClaimNonce(
	walletID [32]byte,
) (*big.Int, error) {
	nonce, err := tc.walletRegistry.InactivityClaimNonce(walletID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get inactivity claim nonce: [%w]",
			err,
		)
	}

	return nonce, nil
}
