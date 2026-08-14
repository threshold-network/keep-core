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
) bool {
	logger.Infof(
		"FROST DKG with seed [0x%x] selected this operator as member "+
			"indexes [%v], but native FROST DKG execution is unavailable "+
			"in this build",
		event.Seed,
		memberIndexes,
	)

	// This binary cannot gain native DKG support during its lifetime. Mark this
	// event complete locally so periodic past-event polling does not repeat the
	// full coordinator RPC sequence forever. A restart with a frost_native build
	// gets a fresh in-memory deduplication cache and can handle a still-live DKG.
	return true
}
