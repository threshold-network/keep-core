package signing

import (
	"context"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/protocol/state"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// Execute runs the tECDSA signing protocol, given a message to sign,
// broadcast channel to mediate with, a block counter used for time tracking,
// a member index to use in the group, private key share, dishonest threshold,
// and block height when signing protocol should start.
//
// This function also supports signing execution with a subset of the signing
// group by passing a non-empty excludedMembers slice holding the members that
// should be excluded.
func Execute(
	ctx context.Context,
	logger log.StandardLogger,
	message *big.Int,
	sessionID string,
	memberIndex group.MemberIndex,
	privateKeyShare *tecdsa.PrivateKeyShare,
	groupSize int,
	dishonestThreshold int,
	excludedMembersIndexes []group.MemberIndex,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	fullBytesLen int,
	legacyHistoricalBobCompatibility bool,
) (*Result, error) {
	logger.Debugf("[member:%v] initializing member", memberIndex)

	member := newMember(
		logger,
		memberIndex,
		groupSize,
		dishonestThreshold,
		membershipValidator,
		sessionID,
		message,
		privateKeyShare,
		fullBytesLen,
		legacyHistoricalBobCompatibility,
	)

	// Mark excluded members as disqualified in order to not exchange messages
	// with them.
	for _, excludedMemberIndex := range excludedMembersIndexes {
		if excludedMemberIndex != member.id {
			member.group.MarkMemberAsDisqualified(excludedMemberIndex)
		}
	}

	initialState := &ephemeralKeyPairGenerationState{
		BaseAsyncState: state.NewBaseAsyncState(),
		channel:        channel,
		member:         member.initializeEphemeralKeysGeneration(),
	}

	stateMachine := state.NewAsyncMachine(logger, ctx, channel, initialState)

	lastState, err := stateMachine.Execute()
	if err != nil {
		return nil, err
	}

	finalizationState, ok := lastState.(*finalizationState)
	if !ok {
		return nil, fmt.Errorf("execution ended on state: %T", lastState)
	}

	return finalizationState.result(), nil
}

// RegisterUnmarshallers initializes the given broadcast channel to be able to
// perform signing protocol interactions by registering all the required
// protocol message unmarshallers.
func RegisterUnmarshallers(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &ephemeralPublicKeyMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundOneMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundTwoMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundThreeMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundFourMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundFiveMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundSixMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundSevenMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundEightMessage{}
	})
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &tssRoundNineMessage{}
	})
}
