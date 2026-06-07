//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func bindAttemptContextHashForExchangeTest(
	t *testing.T,
	sessionID string,
	members []group.MemberIndex,
) {
	t.Helper()

	var messageDigest [attempt.MessageDigestLength]byte
	copy(messageDigest[:], []byte("bound-attempt-context-exchange"))

	ctx, err := attempt.NewAttemptContext(
		sessionID,
		"key-group",
		[]byte{0x01, 0x02, 0x03},
		messageDigest,
		0,
		members,
		nil,
	)
	if err != nil {
		t.Fatalf("failed creating attempt context: [%v]", err)
	}

	SetCurrentAttemptHandleForSession(sessionID, roast.AttemptHandle{}, ctx)
}

func TestBuildTaggedTBTCSignerBootstrapCoarseRound_BoundAttemptContextHashExchange(
	t *testing.T,
) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor(
		"tbtc-signer-bootstrap-bound-attempt-context-test",
	)
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	sessionID := "tbtc-signer-bound-attempt-context"
	includedMembers := []group.MemberIndex{1, 2}
	bindAttemptContextHashForExchangeTest(t, sessionID, includedMembers)

	engineByMember := map[group.MemberIndex]*deterministicBuildTaggedTBTCSignerBootstrapRoundEngine{
		1: {
			roundState: &NativeTBTCSignerRoundState{
				SessionID:             sessionID,
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 1,
					Data:       []byte{0x11, 0x01},
				},
			},
		},
		2: {
			roundState: &NativeTBTCSignerRoundState{
				SessionID:             sessionID,
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 2,
					Data:       []byte{0x22, 0x02},
				},
			},
		},
	}

	requestByMember := map[group.MemberIndex]*NativeExecutionFFISigningRequest{
		1: {
			Message:            big.NewInt(123),
			SessionID:          sessionID,
			MemberIndex:        1,
			GroupSize:          2,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: includedMembers,
			},
		},
		2: {
			Message:            big.NewInt(123),
			SessionID:          sessionID,
			MemberIndex:        2,
			GroupSize:          2,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: includedMembers,
			},
		},
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	signingErrors := make(chan error, len(requestByMember))
	var wg sync.WaitGroup
	wg.Add(len(requestByMember))

	for memberIndex, request := range requestByMember {
		engine := engineByMember[memberIndex]
		go func(
			signingRequest *NativeExecutionFFISigningRequest,
			signingEngine NativeTBTCSignerEngine,
		) {
			defer wg.Done()

			signingErrors <- executeBuildTaggedTBTCSignerBootstrapCoarseRound(
				ctx,
				signingRequest,
				"group-1",
				signingEngine,
				nil,
				nil,
			)
		}(request, engine)
	}

	wg.Wait()
	close(signingErrors)

	for signErr := range signingErrors {
		if signErr != nil {
			t.Fatalf("unexpected signing error: [%v]", signErr)
		}
	}
}
