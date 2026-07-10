//go:build frost_native && frost_roast_retry

package signing

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func signedForcedSnapshot(
	member group.MemberIndex,
	hash [attempt.MessageDigestLength]byte,
) *roast.LocalEvidenceSnapshot {
	snap := roast.NewLocalEvidenceSnapshot(member, hash, attempt.Evidence{})
	payload, _ := snap.SignableBytes()
	sig, _ := fixedSigner{}.Sign(payload)
	snap.OperatorSignature = sig
	return snap
}

func resetSelectionRegistries(t *testing.T) {
	t.Helper()
	ResetRoastRetryRegistrationForTest()
	ResetRoastTransitionRegistryForTest()
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)
	t.Cleanup(ResetObservedAttemptRegistryForTest)
}

func TestConsumeRoastTransitionForSelection_FallbackInitialAttempt(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	if _, _, err := ConsumeRoastTransitionForSelection("session", 1, 0, 3, ""); !errors.Is(
		err, ErrRoastSelectionFallBackToLegacy,
	) {
		t.Fatalf("retry 0 must request the legacy fallback, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FallbackInactiveRoast(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	// No coordinator registered -> inactive -> uniform legacy fallback.
	if _, _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3, ""); !errors.Is(
		err, ErrRoastSelectionFallBackToLegacy,
	) {
		t.Fatalf("inactive ROAST must request the legacy fallback, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FailsClosedNoRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
		// The selector scopes its coordinator lookup by the transition record's key
		// group (newExchangeTestContext uses "exchange-key-group"); register under
		// the same handle so tests with a record reach their intended assertion.
		KeyGroupID: "exchange-key-group",
	})

	// Active ROAST, a retry, but no record -> a transition was expected -> fail
	// closed (NOT the legacy-fallback sentinel).
	_, _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3, "exchange-key-group")
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a missing expected transition must fail closed, got %v", err)
	}
}

// TestConsumeRoastTransitionForSelection_PartialRegistrationFailsClosed asserts the
// multi-seat partial-activation fracture guard (Codex P2-1): when ROAST is active
// for the process (one local seat registered) but the SELECTING seat has no
// registered coordinator, selection FAILS CLOSED rather than falling back to legacy
// -- a legacy fallback for the unregistered seat while a sibling selects from the
// transition would split the included set.
func TestConsumeRoastTransitionForSelection_PartialRegistrationFailsClosed(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)
	// One of the operator's seats (member 1) is registered; member 2 is NOT, so ROAST
	// retry is active for the process but member 2 has no coordinator.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
		// The selector scopes its coordinator lookup by the transition record's key
		// group (newExchangeTestContext uses "exchange-key-group"); register under
		// the same handle so tests with a record reach their intended assertion.
		KeyGroupID: "exchange-key-group",
	})

	_, _, err := ConsumeRoastTransitionForSelection("session", 2, 1, 3, "exchange-key-group")
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("partial registration must fail closed, not fall back to legacy; got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FailsClosedStaleRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
		// The selector scopes its coordinator lookup by the transition record's key
		// group (newExchangeTestContext uses "exchange-key-group"); register under
		// the same handle so tests with a record reach their intended assertion.
		KeyGroupID: "exchange-key-group",
	})

	// A record fresh for retry 1 (prev attempt 0) but we select for retry 3.
	prevCtx := newExchangeTestContext(t, "session", []group.MemberIndex{1, 2, 3}, []byte{0x01})
	RecordRoastTransition("session", 1, RoastTransitionRecord{
		Bundle:          &roast.TransitionMessage{CoordinatorIDValue: 1},
		PreviousContext: prevCtx,
	})

	_, _, err := ConsumeRoastTransitionForSelection("session", 1, 3, 3, "exchange-key-group")
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a stale record must fail closed, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_CarriesParking(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	roastSessionID := "consume-parking-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0c}
	prevCtx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := prevCtx.Hash()

	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, _ := probe.BeginAttempt(prevCtx)
	elected, _ := probe.SelectedCoordinator(probeHandle)

	// Pick a parked member that is NOT the elected coordinator, so the coordinator
	// is in the bundle and can aggregate; omit its forced snapshot so NextAttempt
	// silence-parks it (absent-from-bundle -> transient park).
	var parkedMember group.MemberIndex
	for _, m := range included {
		if m != elected {
			parkedMember = m
			break
		}
	}

	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
		KeyGroupID:  "exchange-key-group",
	})
	handle, _ := coord.BeginAttempt(prevCtx)
	for _, m := range included {
		if m == parkedMember {
			continue // omit -> silence-parked by NextAttempt
		}
		if err := coord.RecordEvidence(handle, signedForcedSnapshot(m, hash)); err != nil {
			t.Fatalf("record evidence for member %d: %v", m, err)
		}
	}
	bundle, err := coord.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate bundle: %v", err)
	}
	RecordRoastTransition(roastSessionID, elected, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   prevCtx,
		DkgGroupPublicKey: dkgKey,
	})

	// threshold 2 so the 2-member next set stays feasible.
	includedSet, parked, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 2, "exchange-key-group")
	if err != nil {
		t.Fatalf("consume must succeed: %v", err)
	}
	// The absent member is carried as parked (reinstated next attempt, not
	// permanently excluded) and is not in the included set.
	if len(parked) != 1 || parked[0] != parkedMember {
		t.Fatalf("expected parked [%d], got %v", parkedMember, parked)
	}
	for _, m := range includedSet {
		if m == parkedMember {
			t.Fatalf("parked member %d must not be in the included set %v", parkedMember, includedSet)
		}
	}
}

func TestConsumeRoastTransitionForSelection_ConsumesFreshRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	roastSessionID := "consume-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0a, 0x0b}
	prevCtx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := prevCtx.Hash()

	// Determine the deterministically elected coordinator for prevCtx.
	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, err := probe.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	elected, err := probe.SelectedCoordinator(probeHandle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}

	// Register a coordinator bound to the elected member and build a real bundle.
	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
		KeyGroupID:  "exchange-key-group",
	})
	handle, err := coord.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	for _, m := range included {
		if err := coord.RecordEvidence(handle, signedForcedSnapshot(m, hash)); err != nil {
			t.Fatalf("record evidence for member %d: %v", m, err)
		}
	}
	bundle, err := coord.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate bundle: %v", err)
	}
	RecordRoastTransition(roastSessionID, elected, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   prevCtx,
		DkgGroupPublicKey: dkgKey,
	})

	// Consume for roast attempt 1 (prevCtx.AttemptNumber 0 + 1 == 1).
	includedSet, parked, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 3, "exchange-key-group")
	if err != nil {
		t.Fatalf("consume must succeed for a fresh record: %v", err)
	}
	// Every included member submitted a proof-of-attendance snapshot, so none is
	// silence-parked: the next included set equals the prior included set and the
	// parked set is empty.
	if len(includedSet) != len(included) {
		t.Fatalf("expected the full included set %v, got %v", included, includedSet)
	}
	if len(parked) != 0 {
		t.Fatalf("expected no parked members, got %v", parked)
	}
}

// TestConsumeRoastTransitionForSelection_UsesRecordKeyGroupCoordinator is the
// multi-wallet regression for the selector: when two wallets seat this operator at
// the SAME member index, selection must drive NextAttempt on the coordinator of the
// wallet the transition record belongs to -- not an arbitrary same-seat entry. The
// record's PreviousHandle was minted by that wallet's coordinator, so a wrong
// coordinator would reject the handle (ErrUnknownAttempt) and fail closed. A
// seat-only scan would pick a coordinator by map order, making this flaky; the
// wallet-scoped lookup makes it deterministic.
func TestConsumeRoastTransitionForSelection_UsesRecordKeyGroupCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	roastSessionID := "multiwallet-consume-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0a, 0x0b}
	prevCtx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := prevCtx.Hash()

	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, err := probe.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	elected, err := probe.SelectedCoordinator(probeHandle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}

	// The RIGHT wallet's coordinator, under the record's key group.
	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
		KeyGroupID:  "exchange-key-group",
	})
	// A DECOY wallet's coordinator at the SAME seat under a DIFFERENT key group. It
	// never saw this attempt, so if selection wrongly picked it NextAttempt would
	// fail with ErrUnknownAttempt.
	decoy := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: decoy,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
		KeyGroupID:  "other-wallet-key-group",
	})

	handle, err := coord.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	for _, m := range included {
		if err := coord.RecordEvidence(handle, signedForcedSnapshot(m, hash)); err != nil {
			t.Fatalf("record evidence for member %d: %v", m, err)
		}
	}
	bundle, err := coord.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate bundle: %v", err)
	}
	RecordRoastTransition(roastSessionID, elected, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   prevCtx, // KeyGroupID == "exchange-key-group"
		DkgGroupPublicKey: dkgKey,
	})

	// Selection must resolve coord (the record's key group), not the decoy; if it
	// used the decoy, NextAttempt would fail closed with an unknown handle.
	if _, _, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 3, "exchange-key-group"); err != nil {
		t.Fatalf("selection must use the record's key-group coordinator and succeed; got %v", err)
	}
}

// TestConsumeRoastTransitionForSelection_UnregisteredWalletFallsBackToLegacy is the
// Codex P2 regression: a wallet whose ROAST coordinator registration was skipped
// (non-native/malformed material -> keyGroupID with no registrations) must fall back
// to LEGACY selection on its retries, NOT fail closed -- even though a SIBLING wallet
// on the same node is ROAST-registered and makes activation true process-wide. The
// per-key-group count is what keeps the sibling from forcing this wallet closed.
func TestConsumeRoastTransitionForSelection_UnregisteredWalletFallsBackToLegacy(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	// Wallet A IS ROAST-registered, so RoastRetryActive() is true process-wide.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
		KeyGroupID:  "wallet-A",
	})

	// Wallet B (key group "wallet-B") has NO registration. Its retry (attempt 1) must
	// return the legacy-fallback sentinel, not a fail-closed error.
	_, _, err := ConsumeRoastTransitionForSelection("wallet-b-session", 1, 1, 3, "wallet-B")
	if !errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf(
			"an unregistered wallet must fall back to legacy despite a sibling ROAST wallet; got %v",
			err,
		)
	}
}

// TestRoastRetryActiveForKeyGroupMember_ScopesByKeyGroup pins the Codex P2 fix: the
// active-attempt/exchange gate is per-wallet, so a seat registered under one wallet
// does NOT make the SAME member index "active" for a different wallet that reuses it.
func TestRoastRetryActiveForKeyGroupMember_ScopesByKeyGroup(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	// Member 1 registered ONLY under wallet-A.
	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		SelfMember:  1,
		KeyGroupID:  "wallet-A",
	})

	if !RoastRetryActiveForKeyGroupMember("wallet-A", 1) {
		t.Fatal("seat 1 must be active for its own key group wallet-A")
	}
	// The SAME seat under a DIFFERENT wallet is NOT active -- a sibling wallet sharing
	// the member index must not flip this wallet's numbering/exchange decision.
	if RoastRetryActiveForKeyGroupMember("wallet-B", 1) {
		t.Fatal("seat 1 must NOT be active for wallet-B, which never registered it")
	}
	// Contrast: the seat-only registry scan still finds seat 1 under SOME key group,
	// which is exactly why activation must NOT key off it -- it cannot tell wallet-A
	// from wallet-B (hence there is no seat-only RoastRetryActive*ForMember predicate).
	if _, ok := RegisteredRoastRetryCoordinatorForMember(1); !ok {
		t.Fatal("seat-only scan must still find seat 1 under some key group")
	}
}
