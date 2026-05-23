//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestRoastRetryRecorderForCollect_RecordsOverflowWhenRegistered(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	rec := roastRetryRecorderForCollect()
	const sender group.MemberIndex = 3
	rec.RecordOverflow(sender)
	rec.RecordOverflow(sender)
	snap := rec.Snapshot()
	if got := snap.Overflows[sender]; got != 2 {
		t.Fatalf(
			"expected bounded recorder to accumulate overflows; got %d for sender %d",
			got, sender,
		)
	}
}

func TestRoastRetryRecorderForCollect_FallsBackToNoOpAfterReset(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})
	ResetRoastRetryRegistrationForTest()

	rec := roastRetryRecorderForCollect()
	rec.RecordOverflow(5)
	if got := rec.Snapshot().Overflows[5]; got != 0 {
		t.Fatalf(
			"after reset the recorder must be NoOp; got count %d",
			got,
		)
	}
}
