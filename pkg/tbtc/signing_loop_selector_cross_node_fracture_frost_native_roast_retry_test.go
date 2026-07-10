//go:build frost_native && frost_roast_retry

package tbtc

import (
	"errors"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test closes the cross-node ROAST/legacy fracture gap: two honest nodes
// with DIVERGENT selection state selecting for the same committed attempt would
// broadcast DIFFERENT NextAttempt included sets (a ROAST-consumed set vs a legacy
// shuffle) -- the fracture class that splits the signing group -- and the
// fail-closed guard (signing_loop_selector_frost_roast_retry.go:83 via
// signing.ConsumeRoastTransitionForSelection) must make BOTH fail closed
// deterministically instead. Prior coverage tested the fail-closed branches in
// isolation with fakes; nothing proved the two configs would ACTUALLY diverge
// and that the guard collapses that divergence to a deterministic fail-closed
// decision on both nodes.
//
// The "two nodes with divergent config" are two seats sharing the process-global
// ROAST registries: node A (the elected coordinator) is registered and ROAST-
// active, node B is an unregistered seat that -- absent the guard -- would fall
// back to the legacy shuffle. The proof is a before/after flip on node B's
// IDENTICAL Select call: with the registry empty it returns a concrete legacy
// set; after node A registers (RoastRetryActive true) the same call FAILS CLOSED
// and does NOT return that set. Node A, whose expected transition is absent for
// the fail-closed session, also fails closed. Neither emits a NextAttempt set, so
// they cannot converge on divergent ones: the network fails closed rather than
// splitting.
//
// Pure-Go: no FROST rounds, bus, or block timing. The whole test is synchronous
// direct calls into the selector and the two package-global registries; the only
// determinism that matters is that the fail-closed decision is a deterministic
// per-seat function of registry state (asserted by repeating each call).

// fracFixedSigner returns a fixed non-empty signature; the wire encoder rejects
// an empty signature during AggregateBundle and the NoOp verifier accepts this
// one. Defined locally because pkg/frost/signing's fixedSigner is not exported to
// this package.
type fracFixedSigner struct{}

func (fracFixedSigner) Sign(_ []byte) ([]byte, error) { return []byte{0x01}, nil }

func fracSignedSnapshot(
	member group.MemberIndex,
	hash [attempt.MessageDigestLength]byte,
) *roast.LocalEvidenceSnapshot {
	snap := roast.NewLocalEvidenceSnapshot(member, hash, attempt.Evidence{})
	payload, _ := snap.SignableBytes()
	sig, _ := fracFixedSigner{}.Sign(payload)
	snap.OperatorSignature = sig
	return snap
}

func fracSameMembers(a, b []group.MemberIndex) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestROASTSelector_CrossNodeLegacyFractureFailsClosed(t *testing.T) {
	t.Setenv(signing.RoastRetryReadinessOptInEnvVar, "true")
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)

	roastSel := roastSigningParticipantSelector{}
	ready := []group.MemberIndex{1, 2, 3, 4, 5}
	ops := selectorTestMembers()
	const seed int64 = 42
	const honestThreshold uint = 3
	dkgKey := []byte{0x0a, 0x0b}
	var digest [attempt.MessageDigestLength]byte
	digest[0] = 0xaa

	// ---- Phase A: prove the two configs produce DIVERGENT NextAttempt sets. ----
	recordedSession := "frac-recorded-session"
	prevCtx, err := attempt.NewAttemptContextWithParking(
		recordedSession, "frac-key-group", dkgKey, digest, 0, ready, nil, nil,
	)
	if err != nil {
		t.Fatalf("build prev attempt context: %v", err)
	}
	hash := prevCtx.Hash()

	// Discover the deterministically elected coordinator for prevCtx.
	probe := roast.NewInMemoryCoordinatorWithSigning(0, fracFixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, err := probe.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	elected, err := probe.SelectedCoordinator(probeHandle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}
	// Node B is any seat that is NOT elected; it is left UNREGISTERED.
	var memberB group.MemberIndex
	for _, m := range ready {
		if m != elected {
			memberB = m
			break
		}
	}
	t.Logf("elected(node A)=%d unregistered(node B)=%d", elected, memberB)

	// Register node A's coordinator and build a real, verifiable transition record
	// so the ROAST selection path is live for recordedSession.
	coordA := roast.NewInMemoryCoordinatorWithSigning(elected, fracFixedSigner{}, roast.NoOpSignatureVerifier())
	registerNodeA := func() {
		signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
			Coordinator: coordA,
			Signer:      fracFixedSigner{},
			Verifier:    roast.NoOpSignatureVerifier(),
			SelfMember:  uint32(elected),
			// The selector resolves the coordinator by the transition record's key
			// group (prevCtx uses "frac-key-group"); register under the same handle so
			// the wallet-scoped lookup resolves for recordedSession.
			KeyGroupID: "frac-key-group",
		})
	}
	registerNodeA()
	if !signing.RoastRetryActive() {
		t.Skip("requires a transition producer (frost_native) for the fail-closed path")
	}
	handle, err := coordA.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("node A begin attempt: %v", err)
	}
	for _, m := range ready {
		if err := coordA.RecordEvidence(handle, fracSignedSnapshot(m, hash)); err != nil {
			t.Fatalf("record evidence for member %d: %v", m, err)
		}
	}
	bundle, err := coordA.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate bundle: %v", err)
	}
	signing.RecordRoastTransition(recordedSession, elected, signing.RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   prevCtx,
		DkgGroupPublicKey: dkgKey,
	})

	// ROAST-config node: consumes the transition -> the full 5-member set (all
	// submitted evidence, none silence-parked).
	sRoast, err := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, recordedSession, elected)
	if err != nil {
		t.Fatalf("ROAST selection must succeed with a fresh record: %v", err)
	}
	if len(sRoast.includedMembersIndexes) != len(ready) {
		t.Fatalf("expected the full %d-member ROAST set, got %v", len(ready), sRoast.includedMembersIndexes)
	}
	// legacy-config node: trims to the honest threshold via a seeded shuffle.
	sLegacy, err := legacySigningParticipantSelector{}.Select(ready, ops, seed, 1, 1, honestThreshold, recordedSession, elected)
	if err != nil {
		t.Fatalf("legacy selection failed: %v", err)
	}
	if len(sLegacy.includedMembersIndexes) != int(honestThreshold) {
		t.Fatalf("expected a %d-member legacy set, got %v", honestThreshold, sLegacy.includedMembersIndexes)
	}
	// FAULT REACHED: the two node configs genuinely select DIFFERENT NextAttempt
	// sets -- exactly the fracture the guard exists to prevent.
	if fracSameMembers(sRoast.includedMembersIndexes, sLegacy.includedMembersIndexes) {
		t.Fatalf("fracture not induced: ROAST and legacy selected the same set %v", sRoast.includedMembersIndexes)
	}
	t.Logf("divergence induced: ROAST set %v vs legacy set %v", sRoast.includedMembersIndexes, sLegacy.includedMembersIndexes)

	// ---- Phase B: node B WOULD diverge to a concrete legacy set (guard OFF). ----
	freshSess := "frac-fail-closed-session"
	signing.ResetRoastRetryRegistrationForTest() // RoastRetryActive() now false
	signing.ResetRoastTransitionRegistryForTest()
	if signing.RoastRetryActive() {
		t.Fatalf("precondition: registry must be empty (RoastRetryActive false)")
	}
	legacyB, err := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, freshSess, memberB)
	if err != nil {
		t.Fatalf("node B must fall back to legacy when ROAST is inactive: %v", err)
	}
	if len(legacyB.includedMembersIndexes) != int(honestThreshold) {
		t.Fatalf("expected node B legacy set of size %d, got %v", honestThreshold, legacyB.includedMembersIndexes)
	}
	t.Logf("node B would broadcast the divergent legacy set %v (before the guard engages)", legacyB.includedMembersIndexes)

	// ---- Phase C: guard ON -> the SAME node-B call, and node A, FAIL CLOSED. ----
	registerNodeA()
	if !signing.RoastRetryActive() {
		t.Fatalf("precondition: ROAST must be active after registering node A")
	}

	assertFailClosed := func(who string, sel participantSelection, err error, wantSubstr string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s must FAIL CLOSED, got selection %v", who, sel.includedMembersIndexes)
		}
		if errors.Is(err, signing.ErrRoastSelectionFallBackToLegacy) {
			t.Fatalf("%s must fail closed, NOT fall back to legacy: %v", who, err)
		}
		if len(sel.includedMembersIndexes) != 0 {
			t.Fatalf("%s fail-closed selection must be empty, got %v", who, sel.includedMembersIndexes)
		}
		if !strings.Contains(err.Error(), wantSubstr) {
			t.Fatalf("%s error %q missing %q", who, err.Error(), wantSubstr)
		}
	}

	// Node B: unregistered seat under active ROAST -> partial-registration fail-closed.
	gotB, errB := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, freshSess, memberB)
	assertFailClosed("node B (would-be legacy)", gotB, errB, "no registered coordinator")
	// The guard specifically suppressed the divergent legacy set node B had emitted.
	if fracSameMembers(gotB.includedMembersIndexes, legacyB.includedMembersIndexes) {
		t.Fatalf("guard failed: node B still emitted the divergent legacy set %v", legacyB.includedMembersIndexes)
	}

	// Node A: registered ROAST seat, but no record for freshSess -> missing-record fail-closed.
	gotA, errA := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, freshSess, elected)
	assertFailClosed("node A (ROAST-active)", gotA, errA, "no transition record")

	// NEITHER produced a NextAttempt set, so they cannot have converged on
	// divergent (or each other's) sets: the network fails closed instead of
	// splitting.
	if len(gotA.includedMembersIndexes) != 0 || len(gotB.includedMembersIndexes) != 0 {
		t.Fatalf("both nodes must emit an EMPTY selection when failing closed; A=%v B=%v",
			gotA.includedMembersIndexes, gotB.includedMembersIndexes)
	}

	// Determinism: the fail-closed decision is a stable per-seat function of
	// registry state, not order/timing dependent.
	gotB2, errB2 := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, freshSess, memberB)
	assertFailClosed("node B (repeat)", gotB2, errB2, "no registered coordinator")
	gotA2, errA2 := roastSel.Select(ready, ops, seed, 1, 1, honestThreshold, freshSess, elected)
	assertFailClosed("node A (repeat)", gotA2, errA2, "no transition record")
	t.Logf("both nodes fail closed deterministically (node A: missing record; node B: unregistered seat)")
}
