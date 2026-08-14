package roast

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/net/libp2p"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This file measures the wire size of the largest transition bundle a *legitimate*
// production signing group can produce, and pins the two limits that must admit it:
// the parser's rejection ceiling (MaxSignedTransitionMessageBytes) and the pubsub
// transport's per-message limit (MaxTransitionMessageTransportBytes).
//
// The composed worst case MaxSnapshotsPerBundle*MaxSignedLocalEvidenceSnapshotBytes
// is not a reachable shape: it assumes 256 snapshots each carrying two 1 MiB
// coordinator package proofs. A real proof is a real signed FROST signing package,
// and a real bundle carries at most one snapshot per group member.

// productionGroupSize is the mainnet tBTC group size (see
// defaultGroupParameters in pkg/tbtc/tbtc.go). The bundle carries at most one
// snapshot per member, so this - not MaxSnapshotsPerBundle - sets the real width.
const productionGroupSize = 100

// frostCommitmentBytes models one participant's round-1 entry inside a serialized
// FROST signing package: two compressed secp256k1 points (33 bytes each) plus the
// participant identifier and field framing.
const frostCommitmentBytes = 72

// signingPackageBytesForGroup returns a payload the size of a real serialized FROST
// signing package for an n-member signing subset: one commitment entry per
// participant plus the 32-byte message being signed.
func signingPackageBytesForGroup(n int) []byte {
	return make([]byte, n*frostCommitmentBytes+32)
}

// worstCaseProof builds the largest coordinator package proof a snapshot can
// legitimately carry: a fully signed signing package over the whole group.
func worstCaseProof(t *testing.T) []byte {
	t.Helper()

	signerIDs := make([]uint32, 0, productionGroupSize)
	for i := 1; i <= productionGroupSize; i++ {
		signerIDs = append(signerIDs, uint32(i))
	}

	pkg := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  uint32(testElectedCoordinator),
		SigningPackageBytes: signingPackageBytesForGroup(productionGroupSize),
		TaprootMerkleRoot:   make([]byte, 32),
		SignerIDsValue:      signerIDs,
	}
	if err := SignSigningPackage(&fakeSigner{id: testElectedCoordinator}, pkg); err != nil {
		t.Fatalf("sign package: %v", err)
	}
	wire, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("marshal package: %v", err)
	}
	return wire
}

// saturatedEvidence returns the widest evidence a single attempt can report:
// every other member named in every evidence class.
func saturatedEvidence() attempt.Evidence {
	evidence := attempt.Evidence{
		Overflows: make(map[group.MemberIndex]uint, productionGroupSize),
		Rejects:   make(map[group.MemberIndex][]attempt.RejectEntry, productionGroupSize),
		Conflicts: make(map[group.MemberIndex]uint, productionGroupSize),
	}
	for i := 1; i <= productionGroupSize; i++ {
		member := group.MemberIndex(i)
		evidence.Overflows[member] = 64
		evidence.Conflicts[member] = 64
		evidence.Rejects[member] = []attempt.RejectEntry{
			{Reason: "unknown-sender", Count: 64},
			{Reason: "stale-attempt", Count: 64},
		}
	}
	return evidence
}

// TestTransitionMessage_WorstCaseProductionBundleFitsLimits measures the largest
// bundle a legitimate production group can emit and asserts both the parser
// ceiling and the transport limit admit it. It is the regression guard on
// evidence-payload growth: adding a field to the snapshot or widening the proof
// shape moves the measured size, and this test fails before the growth reaches a
// production group that can no longer gossip its transitions.
func TestTransitionMessage_WorstCaseProductionBundleFitsLimits(t *testing.T) {
	proofA := worstCaseProof(t)
	proofB := append(append([]byte(nil), proofA...), 0x01)
	evidence := saturatedEvidence()

	bundle := make([]LocalEvidenceSnapshot, 0, productionGroupSize)
	for i := 1; i <= productionGroupSize; i++ {
		snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(
			group.MemberIndex(i), pinnedContextHash, evidence, proofA, proofB,
		))
		bundle = append(bundle, *snap)
	}

	msg := &TransitionMessage{
		AttemptContextHash:   append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:   uint32(testElectedCoordinator),
		Bundle:               bundle,
		CoordinatorSignature: make([]byte, 72),
	}
	wire, err := msg.Marshal()
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	measured := len(wire)
	t.Logf(
		"worst-case production bundle: %d members x 2 proofs = %d bytes (%.2f MiB)",
		productionGroupSize, measured, float64(measured)/(1024*1024),
	)

	// The parser must admit a legitimate worst case; otherwise a real group
	// silently loses the ability to transition under maximum evidence.
	if measured > MaxSignedTransitionMessageBytes {
		t.Fatalf(
			"worst-case legitimate bundle is %d bytes, above the parser ceiling "+
				"MaxSignedTransitionMessageBytes = %d",
			measured, MaxSignedTransitionMessageBytes,
		)
	}

	// The transport must carry what the parser accepts. A transport limit below
	// the parser ceiling drops legitimate bundles at the pubsub layer, where the
	// failure is invisible to the ROAST retry path.
	if measured > libp2p.MaxMessageSize {
		t.Fatalf(
			"worst-case legitimate bundle is %d bytes, above the pubsub transport "+
				"limit libp2p.MaxMessageSize = %d",
			measured, libp2p.MaxMessageSize,
		)
	}

	// Both limits must be reachable from the measurement, not orders of magnitude
	// above it: an unreachable ceiling is not a pre-allocation guard.
	if MaxSignedTransitionMessageBytes > 8*measured {
		t.Fatalf(
			"MaxSignedTransitionMessageBytes = %d is more than 8x the worst-case "+
				"legitimate bundle (%d bytes); the ceiling no longer bounds allocation",
			MaxSignedTransitionMessageBytes, measured,
		)
	}
}

// TestTransitionMessage_TransportLimitMatchesParserCeiling pins the two limits
// together: the transport must accept exactly what the parser will accept, so a
// bundle is never dropped by one layer and admitted by the other.
func TestTransitionMessage_TransportLimitMatchesParserCeiling(t *testing.T) {
	if libp2p.MaxMessageSize < MaxSignedTransitionMessageBytes {
		t.Fatalf(
			"transport limit %d is below the parser ceiling %d: legitimate bundles "+
				"would be dropped by pubsub before the parser sees them",
			libp2p.MaxMessageSize, MaxSignedTransitionMessageBytes,
		)
	}
}
