//go:build !frost_native

package tbtc

import (
	"context"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func executeFrostDKGIfPossible(
	_ context.Context,
	_ *node,
	_ FrostDKGChain,
	event *FrostDKGStartedEvent,
	memberIndexes []group.MemberIndex,
	_ *GroupSelectionResult,
) {
	logger.Infof(
		"FROST DKG with seed [0x%x] selected this operator as member "+
			"indexes [%v], but native FROST DKG execution is unavailable "+
			"in this build",
		event.Seed,
		memberIndexes,
	)
}
