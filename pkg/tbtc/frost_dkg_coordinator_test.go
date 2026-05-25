package tbtc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/registry"
)

func TestChallengeInvalidFrostDKGResultRetriesUntilStateLeavesChallenge(t *testing.T) {
	localChain := Connect(time.Millisecond)
	node := &node{chain: localChain}

	frostChain := &retryingFrostDKGChallengeChain{
		state:            Challenge,
		successOnAttempt: 2,
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
	defer cancelCtx()

	challengeInvalidFrostDKGResult(
		ctx,
		node,
		frostChain,
		&FrostDKGResultSubmittedEvent{
			ResultHash: [32]byte{0x01},
			Result:     &registry.Result{},
		},
	)

	if frostChain.challengeCount != 2 {
		t.Fatalf(
			"unexpected challenge count\nexpected: [2]\nactual:   [%d]",
			frostChain.challengeCount,
		)
	}

	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		t.Fatalf("unexpected state error: [%v]", err)
	}
	if state == Challenge {
		t.Fatal("expected challenge loop to leave Challenge state")
	}
}

type retryingFrostDKGChallengeChain struct {
	FrostDKGChain

	mutex            sync.Mutex
	state            DKGState
	challengeCount   int
	successOnAttempt int
}

func (rfdgcc *retryingFrostDKGChallengeChain) GetFrostDKGState() (DKGState, error) {
	rfdgcc.mutex.Lock()
	defer rfdgcc.mutex.Unlock()

	return rfdgcc.state, nil
}

func (rfdgcc *retryingFrostDKGChallengeChain) ChallengeFrostDKGResult(
	*registry.Result,
) error {
	rfdgcc.mutex.Lock()
	defer rfdgcc.mutex.Unlock()

	rfdgcc.challengeCount++
	if rfdgcc.challengeCount >= rfdgcc.successOnAttempt {
		rfdgcc.state = Idle
	}

	return nil
}
