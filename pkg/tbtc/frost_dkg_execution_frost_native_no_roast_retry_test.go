//go:build frost_native && !frost_roast_retry

package tbtc

import (
	"context"
	"math/big"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestExecuteFrostDKGIfPossible_RequiresRoastRetryBuild(t *testing.T) {
	t.Setenv(frostsigning.InteractiveSigningOptInEnvVar, "true")
	t.Setenv(frostsigning.RoastRetryReadinessOptInEnvVar, "true")
	registerFrostDKGReadinessTestEngine(t)

	executed := executeFrostDKGIfPossible(
		context.Background(),
		nil,
		nil,
		&FrostDKGStartedEvent{Seed: big.NewInt(100)},
		[]group.MemberIndex{1},
		nil,
	)

	if executed {
		t.Fatal("DKG must not execute in a build without the ROAST retry path")
	}
}
