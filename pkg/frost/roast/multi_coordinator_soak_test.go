package roast

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sort"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// The soak harness models the production deployment: every signer
// runs its own Coordinator instance bound to its own selfMember,
// shares the same signer/verifier scheme (here a deterministic
// SHA-256 stand-in), and must compute byte-identical next contexts
// given the same verified TransitionMessage.
//
// The harness exercises RFC-21 Layer A (overflow exclusion), Layer
// B (silence parking + reinstatement), and the policy's
// infeasibility floor under synthetic fault injection. The receive
// loops are bypassed -- they are unit-tested elsewhere; what the
// soak harness adds is the multi-instance-agreement property.

// soakSigner produces SHA-256(member || payload) signatures. The
// matching soakVerifier accepts any signature byte-identical to
// the recomputation, so cross-instance verification works without
// real crypto.
type soakSigner struct {
	id group.MemberIndex
}

func (s *soakSigner) Sign(payload []byte) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte{byte(s.id)})
	h.Write(payload)
	return h.Sum(nil), nil
}

type soakVerifier struct{}

func (soakVerifier) Verify(payload, signature []byte, signer group.MemberIndex) error {
	h := sha256.New()
	h.Write([]byte{byte(signer)})
	h.Write(payload)
	want := h.Sum(nil)
	if !bytes.Equal(want, signature) {
		return errors.New("soakVerifier: signature does not match recomputation")
	}
	return nil
}

// soakNode bundles one signer's Coordinator instance, its self
// signer, and the snapshot it submits each attempt.
type soakNode struct {
	self   group.MemberIndex
	coord  Coordinator
	signer *soakSigner
}

// newSoakHarness initialises N coordinator instances bound to
// member indices 1..N, ready to BeginAttempt against a shared
// AttemptContext. Returns the nodes plus a deterministic
// shared-state baseline attempt context.
func newSoakHarness(
	t *testing.T,
	members []group.MemberIndex,
) []*soakNode {
	t.Helper()
	nodes := make([]*soakNode, 0, len(members))
	for _, m := range members {
		signer := &soakSigner{id: m}
		node := &soakNode{
			self:   m,
			coord:  NewInMemoryCoordinatorWithSigning(m, signer, soakVerifier{}),
			signer: signer,
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// soakAttempt drives a full attempt across every node:
//
//  1. Every node calls BeginAttempt with the shared context.
//  2. Every node produces a signed snapshot per the fault map
//     (silent members produce nil; overflowing members produce
//     snapshots with overflow events).
//  3. Every node receives every other node's snapshot via
//     RecordEvidence.
//  4. The elected coordinator's node calls AggregateBundle.
//  5. Every non-coordinator node calls VerifyBundle.
//  6. Every node calls NextAttempt against the same verified
//     bundle.
//
// Returns the next AttemptContext computed by every node (all must
// be byte-identical) and the elected coordinator's identity for
// the *current* attempt.
//
// silenceFor and overflowFor are maps that let the test inject
// faults. overflowFor[observer] = [senders the observer reports
// having overflowed].
func soakAttempt(
	t *testing.T,
	nodes []*soakNode,
	ctx attempt.AttemptContext,
	silenceFor map[group.MemberIndex]bool,
	overflowFor map[group.MemberIndex][]group.MemberIndex,
	threshold uint,
) (attempt.AttemptContext, group.MemberIndex) {
	t.Helper()

	type beginResult struct {
		node   *soakNode
		handle AttemptHandle
	}
	begins := make([]beginResult, 0, len(nodes))
	for _, n := range nodes {
		h, err := n.coord.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("node %d BeginAttempt: %v", n.self, err)
		}
		begins = append(begins, beginResult{node: n, handle: h})
	}

	// Elect coordinator: each node has the same SelectCoordinator
	// result for this context, so it doesn't matter which node we
	// ask. Use begins[0].
	elected, err := begins[0].node.coord.SelectedCoordinator(begins[0].handle)
	if err != nil {
		t.Fatalf("SelectedCoordinator: %v", err)
	}

	// Each node produces a snapshot unless silent.
	type signedSnap struct {
		from     group.MemberIndex
		snapshot *LocalEvidenceSnapshot
	}
	snaps := make([]signedSnap, 0, len(nodes))
	for _, n := range nodes {
		if silenceFor[n.self] {
			continue
		}
		evidence := attempt.Evidence{
			Overflows: map[group.MemberIndex]uint{},
		}
		for _, sender := range overflowFor[n.self] {
			evidence.Overflows[sender]++
		}
		snap := NewLocalEvidenceSnapshot(n.self, ctx.Hash(), evidence)
		payload, _ := CanonicalSnapshotBytes(snap)
		sig, _ := n.signer.Sign(payload)
		snap.OperatorSignature = sig
		snaps = append(snaps, signedSnap{from: n.self, snapshot: snap})
	}

	// Every node receives every snapshot.
	for _, b := range begins {
		for _, s := range snaps {
			if err := b.node.coord.RecordEvidence(b.handle, s.snapshot); err != nil {
				t.Fatalf(
					"node %d RecordEvidence from %d: %v",
					b.node.self, s.from, err,
				)
			}
		}
	}

	// Find the elected coordinator's node and aggregate.
	var aggregator beginResult
	for _, b := range begins {
		if b.node.self == elected {
			aggregator = b
			break
		}
	}
	if aggregator.node == nil {
		t.Fatalf("elected coordinator %d not in nodes", elected)
	}
	bundle, err := aggregator.node.coord.AggregateBundle(aggregator.handle)
	if err != nil {
		t.Fatalf("AggregateBundle on elected node %d: %v", elected, err)
	}

	// Every non-coordinator node verifies the bundle.
	for _, b := range begins {
		if b.node.self == elected {
			continue
		}
		if err := b.node.coord.VerifyBundle(b.handle, bundle); err != nil {
			t.Fatalf("node %d VerifyBundle: %v", b.node.self, err)
		}
	}

	// Every node computes NextAttempt.
	dkgPub := []byte{0xab, 0xcd, 0xef}
	nextContexts := make([]attempt.AttemptContext, 0, len(nodes))
	for _, b := range begins {
		next, err := b.node.coord.NextAttempt(
			b.handle,
			bundle,
			threshold,
			dkgPub,
		)
		if err != nil {
			t.Fatalf("node %d NextAttempt: %v", b.node.self, err)
		}
		nextContexts = append(nextContexts, next)
	}

	// All nodes must produce byte-identical next contexts.
	for i := 1; i < len(nextContexts); i++ {
		if nextContexts[i].Hash() != nextContexts[0].Hash() {
			t.Fatalf(
				"multi-instance agreement violated: node 0 hash %x, node %d hash %x",
				nextContexts[0].Hash(),
				i,
				nextContexts[i].Hash(),
			)
		}
	}

	return nextContexts[0], elected
}

func soakStartingContext(
	t *testing.T,
	included []group.MemberIndex,
) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"soak-session",
		"soak-key-group",
		[]byte{0xab, 0xcd, 0xef},
		[attempt.MessageDigestLength]byte{0x99},
		0,
		included,
		nil,
	)
	if err != nil {
		t.Fatalf("starting ctx: %v", err)
	}
	return ctx
}

func TestSoak_CleanAttemptPreservesIncludedSet(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	nodes := newSoakHarness(t, members)
	prev := soakStartingContext(t, members)

	next, _ := soakAttempt(t, nodes, prev, nil, nil, 3)

	if len(next.IncludedSet) != len(members) {
		t.Fatalf(
			"clean attempt must preserve IncludedSet size; got %d want %d",
			len(next.IncludedSet), len(members),
		)
	}
	if len(next.ExcludedSet) != 0 {
		t.Fatalf("clean attempt must not exclude anyone; got %v", next.ExcludedSet)
	}
	if len(next.TransientlyParked) != 0 {
		t.Fatalf("clean attempt must not park anyone; got %v", next.TransientlyParked)
	}
}

func TestSoak_OverflowEvidenceExcludesPermanently(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	nodes := newSoakHarness(t, members)
	prev := soakStartingContext(t, members)

	// Four observers report 1 overflow each against member 3.
	// Total 4 = OverflowExclusionThreshold.
	overflow := map[group.MemberIndex][]group.MemberIndex{
		1: {3},
		2: {3},
		4: {3},
		5: {3},
	}
	next, _ := soakAttempt(t, nodes, prev, nil, overflow, 3)

	if !containsMember(next.ExcludedSet, 3) {
		t.Fatalf("member 3 must be excluded; got %v", next.ExcludedSet)
	}
	if containsMember(next.IncludedSet, 3) {
		t.Fatal("member 3 must not be in next IncludedSet")
	}
}

func TestSoak_SilenceParksTransiently(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	nodes := newSoakHarness(t, members)
	prev := soakStartingContext(t, members)

	silence := map[group.MemberIndex]bool{3: true}
	next, _ := soakAttempt(t, nodes, prev, silence, nil, 3)

	if !containsMember(next.TransientlyParked, 3) {
		t.Fatalf("silent member 3 must be parked; got %v", next.TransientlyParked)
	}
	if containsMember(next.ExcludedSet, 3) {
		t.Fatal("silent member 3 must not be permanently excluded")
	}
	if containsMember(next.IncludedSet, 3) {
		t.Fatal("silent member 3 must not be in next IncludedSet")
	}
}

func TestSoak_ParkedMemberIsReinstatedNextAttempt(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	nodes := newSoakHarness(t, members)
	prev := soakStartingContext(t, members)

	// Attempt N: member 3 silent → parked at N+1.
	silenceN := map[group.MemberIndex]bool{3: true}
	contextN1, _ := soakAttempt(t, nodes, prev, silenceN, nil, 3)
	if !containsMember(contextN1.TransientlyParked, 3) {
		t.Fatalf("setup: N+1 must park member 3; got %v", contextN1.TransientlyParked)
	}

	// Attempt N+1: member 3 cannot submit (parked). Other 4 members
	// do submit. Need a fresh harness because each node's
	// Coordinator already transitioned its previous attempt.
	nextNodes := newSoakHarness(t, members)
	silenceN1 := map[group.MemberIndex]bool{
		3: true, // parked by design, cannot submit
	}
	contextN2, _ := soakAttempt(t, nextNodes, contextN1, silenceN1, nil, 3)

	if !containsMember(contextN2.IncludedSet, 3) {
		t.Fatalf("member 3 must be reinstated at N+2; got %v", contextN2.IncludedSet)
	}
	if containsMember(contextN2.TransientlyParked, 3) {
		t.Fatal("member 3 must not be re-parked at N+2")
	}
	if containsMember(contextN2.ExcludedSet, 3) {
		t.Fatal("member 3 must not be permanently excluded at N+2")
	}
}

func TestSoak_InfeasibilityWhenBelowThreshold(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	nodes := newSoakHarness(t, members)
	prev := soakStartingContext(t, members)

	// Threshold = 5 (all members required). Silence two members.
	// Next attempt's IncludedSet would be 3 (= 5 - 2 silenced), below 5.
	// NextAttempt must return ErrAttemptInfeasible.
	silence := map[group.MemberIndex]bool{
		4: true,
		5: true,
	}
	// Build the bundle manually because soakAttempt panics on
	// NextAttempt error. Walk the same steps but skip the post-
	// aggregate verify on infeasibility.
	type beginResult struct {
		node   *soakNode
		handle AttemptHandle
	}
	begins := make([]beginResult, 0, len(nodes))
	for _, n := range nodes {
		h, _ := n.coord.BeginAttempt(prev)
		begins = append(begins, beginResult{node: n, handle: h})
	}
	for _, n := range nodes {
		if silence[n.self] {
			continue
		}
		snap := NewLocalEvidenceSnapshot(n.self, prev.Hash(), attempt.Evidence{})
		payload, _ := CanonicalSnapshotBytes(snap)
		sig, _ := n.signer.Sign(payload)
		snap.OperatorSignature = sig
		for _, b := range begins {
			_ = b.node.coord.RecordEvidence(b.handle, snap)
		}
	}
	elected, _ := begins[0].node.coord.SelectedCoordinator(begins[0].handle)
	var aggregator beginResult
	for _, b := range begins {
		if b.node.self == elected {
			aggregator = b
			break
		}
	}
	bundle, _ := aggregator.node.coord.AggregateBundle(aggregator.handle)

	// Verify each non-coordinator's NextAttempt returns infeasible.
	for _, b := range begins {
		_, err := b.node.coord.NextAttempt(b.handle, bundle, 5, []byte{0x01})
		if !errors.Is(err, ErrAttemptInfeasible) {
			t.Fatalf(
				"node %d NextAttempt: expected ErrAttemptInfeasible; got %v",
				b.node.self, err,
			)
		}
	}
}

func TestSoak_OriginalSignerSetIsPreservedAcrossThreeTransitions(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5}
	prev := soakStartingContext(t, members)

	// Three attempts back-to-back, with fresh harnesses each
	// (real signers run one attempt per Coordinator instance).
	for i := 0; i < 3; i++ {
		nodes := newSoakHarness(t, members)
		next, _ := soakAttempt(t, nodes, prev, nil, nil, 3)
		if sz := len(next.IncludedSet) + len(next.ExcludedSet) + len(next.TransientlyParked); sz != len(members) {
			t.Fatalf(
				"attempt %d: |Inc|+|Exc|+|Park| = %d, want %d",
				i, sz, len(members),
			)
		}
		prev = next
	}
}

func containsMember(slice []group.MemberIndex, target group.MemberIndex) bool {
	for _, m := range slice {
		if m == target {
			return true
		}
	}
	return false
}

// silence the unused-import warning for sort if no test references
// it directly.
var _ = sort.Slice
