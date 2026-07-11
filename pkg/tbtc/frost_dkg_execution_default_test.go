//go:build !frost_native

package tbtc

import (
	"context"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestExecuteFrostDKGUnavailableBuildIsTerminal(t *testing.T) {
	completed := executeFrostDKGIfPossible(
		context.Background(),
		nil,
		nil,
		&FrostDKGStartedEvent{Seed: big.NewInt(100)},
		[]group.MemberIndex{1},
		nil,
	)

	if !completed {
		t.Fatal("unavailable build should complete local event handling")
	}
}
