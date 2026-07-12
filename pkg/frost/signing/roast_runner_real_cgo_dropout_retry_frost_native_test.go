//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test closes the highest-value ROAST-retry coverage gap: NO test combined
// REAL threshold crypto with an INDUCED signer failure AND a retry. The real-cgo
// e2es (roast_runner_real_cgo_multinode_e2e_frost_native_test.go) are happy-path
// only; the retry/parking machinery (next_attempt.go) is unit-tested with fakes.
// This wires them together end to end:
//
//	attempt 1 (REAL crypto): a SELECTED signer withholds its round-2 share ->
//	  the elected coordinator (the aggregator) starves and fails;
//	NextAttempt (REAL policy): the silent member is missing from the transition
//	  bundle senders, so it is transiently PARKED;
//	attempt 2 (REAL crypto): the reshuffled subset (silent member excluded)
//	  aggregates a real BIP-340 signature.
//
// DETERMINISM in the single-process shared-engine harness. Subset selection is
// first-come-first-served over the responsive committers, and the one engine is
// shared by all local seats. To make attempt 1 fail deterministically (rather
// than depend on which seat commits first, or let a co-resident seat aggregate),
// attempt 1 runs ONLY {coordinator, target}: the target is therefore necessarily
// the coordinator's sole co-signer, and by withholding its share it starves the
// coordinator's collection. We assert on the COORDINATOR's failure (the
// aggregator) - a co-resident target may aggregate locally off the shared engine,
// which is a harness artifact of the shared engine, not a real outcome (a real
// per-node deployment gives the faulty node its own engine and it broadcasts
// nothing). The offline third seat is absent from attempt 1 and is supplied as a
// live bundle sender for the transition, exactly as a real non-selected-but-live
// member would broadcast its evidence snapshot.

// shareDroppingBus wraps a RunnerBus and silently drops the wrapped seat's own
// round-2 share submissions, modelling a signer that goes silent mid-collection
// after being selected. All other traffic (commitments, the coordinator package,
// evidence) passes through, and inbound delivery via Subscribe is untouched.
type shareDroppingBus struct{ inner RunnerBus }

func (b shareDroppingBus) Broadcast(msg RunnerMessage) {
	if msg.Type == RunnerMsgShareSubmission {
		return
	}
	b.inner.Broadcast(msg)
}

func (b shareDroppingBus) Subscribe() *RunnerBusSubscriber { return b.inner.Subscribe() }

// dropoutSeat bundles one seat's coordinator, attempt handle, binding, and runner
// so the test can run a chosen subset and later drive NextAttempt from the
// elected coordinator's coordinator instance.
type dropoutSeat struct {
	member  group.MemberIndex
	coord   roast.Coordinator
	handle  roast.AttemptHandle
	binding *ActiveRoastAttempt
	runner  *interactiveSigningRunner
}

func TestRealCgoInteractiveSigning_DropoutForcesNextAttemptAndReshuffledSubsetFinalizes(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-dropout-retry-%d", realCgoSessionSeq.Add(1))

	const n = 3
	const threshold uint16 = 2
	participantIDs := []byte{1, 2, 3}
	included := []group.MemberIndex{1, 2, 3}

	// One real DKG; both attempts sign under the same key group.
	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)
	keyGroupSeed := []byte(keyGroup)

	var messageDigest [attempt.MessageDigestLength]byte
	for i := range messageDigest {
		messageDigest[i] = 0x42
	}

	attempt1Ctx, err := attempt.NewAttemptContext(
		sessionID, keyGroup, keyGroupSeed, messageDigest, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt 1 context: %v", err)
	}

	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	logger := &testutils.MockLogger{}

	// Resolve attempt 1's elected coordinator from a probe binding (the election
	// is a property of the attempt context), so we can designate a NON-coordinator
	// target to withhold its share and the offline seat.
	probeCoord := roast.NewInMemoryCoordinatorWithSigning(1, signer, verifier)
	probeHandle, err := probeCoord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	probeBinding, err := NewActiveRoastAttempt(probeCoord, probeHandle, attempt1Ctx, sessionID, nil, keyGroupSeed)
	if err != nil {
		t.Fatalf("probe binding: %v", err)
	}
	coordinator := probeBinding.ElectedCoordinator()
	nonCoordinators := make([]group.MemberIndex, 0, n-1)
	for _, m := range included {
		if m != coordinator {
			nonCoordinators = append(nonCoordinators, m)
		}
	}
	sort.Slice(nonCoordinators, func(i, j int) bool { return nonCoordinators[i] < nonCoordinators[j] })
	target := nonCoordinators[0]  // withholds its share in attempt 1
	offline := nonCoordinators[1] // absent in attempt 1, live sender for the transition
	t.Logf("attempt 1: coordinator=%d target(silent)=%d offline=%d", coordinator, target, offline)

	// Shared operator identities + membership validator over all 3 seats, so any
	// subset's broadcasts authenticate.
	chainSigning := local_v1.Connect(n, n).Signing()
	publicKeys := make(map[group.MemberIndex]*operator.PublicKey, n)
	addresses := make([]chain.Address, 0, n)
	for _, m := range included {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", m, err)
		}
		publicKeys[m] = publicKey
		addresses = append(addresses, chainSigning.PublicKeyBytesToAddress(
			operator.MarshalUncompressed(publicKey),
		))
	}
	validator := group.NewMembershipValidator(&testutils.MockLogger{}, addresses, chainSigning)

	newSeat := func(ctx context.Context, member group.MemberIndex, attemptCtx attempt.AttemptContext, dropShares bool) *dropoutSeat {
		channel, err := netlocal.ConnectWithKey(publicKeys[member]).
			BroadcastChannelFor("frost-roast-interactive-signing")
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", member, err)
		}
		var bus RunnerBus
		bus, err = NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
		if err != nil {
			t.Fatalf("runner bus (seat %d): %v", member, err)
		}
		if dropShares {
			bus = shareDroppingBus{inner: bus}
		}
		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(attemptCtx)
		if err != nil {
			t.Fatalf("begin attempt (seat %d): %v", member, err)
		}
		binding, err := NewActiveRoastAttempt(coord, handle, attemptCtx, sessionID, nil, keyGroupSeed)
		if err != nil {
			t.Fatalf("active attempt (seat %d): %v", member, err)
		}
		collector := roast.NewRound2Collector(verifier)
		runner, err := newInteractiveSigningRunner(
			binding, member, threshold, nil, engine, collector, coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (seat %d): %v", member, err)
		}
		return &dropoutSeat{member: member, coord: coord, handle: handle, binding: binding, runner: runner}
	}

	// ---- Attempt 1: coordinator + silent target only; coordinator must fail. ----
	ctx1, cancel1 := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel1()

	coordSeat := newSeat(ctx1, coordinator, attempt1Ctx, false)
	targetSeat := newSeat(ctx1, target, attempt1Ctx, true) // withholds its share

	coordErr := runSeatAsync(coordSeat, ctx1)
	targetErr := runSeatAsync(targetSeat, ctx1)
	<-coordErr.done
	<-targetErr.done

	// The coordinator must fail SPECIFICALLY by starving on the missing share: it
	// built + broadcast the package, produced its own round-2 share, then timed
	// out collecting the target's. Pinning the failure to share collection (not a
	// generic early error) is what makes this the withheld-share path.
	if coordErr.err == nil {
		t.Fatalf(
			"attempt 1: the coordinator aggregated a signature despite the selected co-signer (seat %d) "+
				"withholding its share; expected the coordinator's collection to starve and fail",
			target,
		)
	}
	if !strings.Contains(coordErr.err.Error(), "collect shares") {
		t.Fatalf(
			"attempt 1: coordinator must starve at round-2 share collection; got a different failure: %v",
			coordErr.err,
		)
	}
	t.Logf("attempt 1 coordinator starved at share collection as expected: %v", coordErr.err)

	// Crucially, the target must actually have REACHED round 2 and produced the
	// share it withheld -- otherwise the coordinator's timeout would be vacuous (an
	// early target failure, e.g. an undelivered signing package or a pre-round-2
	// engine error, would starve the coordinator identically). Because only the
	// target's OUTBOUND share is dropped, the target still receives the
	// coordinator's share and aggregates locally off the shared engine; that local
	// success -- or, failing that, its own collect-shares timeout -- is the proof
	// it produced (and the bus withheld) a genuine round-2 share. A failure BEFORE
	// round 2 must fail this test.
	if targetErr.err != nil && !strings.Contains(targetErr.err.Error(), "collect shares") {
		t.Fatalf(
			"attempt 1: target seat %d must reach round 2 and produce its withheld share, but it "+
				"failed before round 2 (so the coordinator's starvation would be vacuous): %v",
			target, targetErr.err,
		)
	}
	t.Logf("attempt 1 target reached round 2 and produced its withheld share (err=%v sigLen=%d)",
		targetErr.err, len(targetErr.sig))

	// ---- Transition: build the bundle from the LIVE senders (target absent). ----
	prevHash := attempt1Ctx.Hash()
	senders := []group.MemberIndex{coordinator, offline}
	sort.Slice(senders, func(i, j int) bool { return senders[i] < senders[j] })
	bundleSnapshots := make([]roast.LocalEvidenceSnapshot, 0, len(senders))
	for _, s := range senders {
		bundleSnapshots = append(bundleSnapshots, roast.LocalEvidenceSnapshot{
			SenderIDValue:      uint32(s),
			AttemptContextHash: append([]byte{}, prevHash[:]...),
		})
	}
	bundle := &roast.TransitionMessage{
		AttemptContextHash: append([]byte{}, prevHash[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             bundleSnapshots,
	}

	attempt2Ctx, err := coordSeat.coord.NextAttempt(
		coordSeat.handle, bundle, uint(threshold), keyGroupSeed,
	)
	if err != nil {
		t.Fatalf("NextAttempt: %v", err)
	}

	// ---- Assert the real parking policy excluded the silent target. ----
	if !containsMember(attempt2Ctx.TransientlyParked, target) {
		t.Fatalf("silent target %d must be transiently parked; parked=%v", target, attempt2Ctx.TransientlyParked)
	}
	if containsMember(attempt2Ctx.IncludedSet, target) {
		t.Fatalf("silent target %d must not be in attempt 2's included set %v", target, attempt2Ctx.IncludedSet)
	}
	wantIncluded := []group.MemberIndex{coordinator, offline}
	sort.Slice(wantIncluded, func(i, j int) bool { return wantIncluded[i] < wantIncluded[j] })
	gotIncluded := append([]group.MemberIndex{}, attempt2Ctx.IncludedSet...)
	sort.Slice(gotIncluded, func(i, j int) bool { return gotIncluded[i] < gotIncluded[j] })
	if !memberSlicesEqualLocal(gotIncluded, wantIncluded) {
		t.Fatalf("attempt 2 included set = %v, want %v (silent target parked)", gotIncluded, wantIncluded)
	}
	if attempt2Ctx.AttemptNumber != attempt1Ctx.AttemptNumber+1 {
		t.Fatalf("attempt 2 number = %d, want %d", attempt2Ctx.AttemptNumber, attempt1Ctx.AttemptNumber+1)
	}

	// ---- Attempt 2: reshuffled subset {coordinator, offline} finalizes for real. ----
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	var seats2 []*dropoutSeat
	for _, m := range attempt2Ctx.IncludedSet {
		seats2 = append(seats2, newSeat(ctx2, m, attempt2Ctx, false))
	}
	results := make([]seatResult, len(seats2))
	dones := make([]*seatRunResult, len(seats2))
	for i, s := range seats2 {
		dones[i] = runSeatAsync(s, ctx2)
	}
	for i := range dones {
		<-dones[i].done
		results[i] = seatResult{sig: dones[i].sig, err: dones[i].err}
	}

	// With the per-(session,attempt) aggregate memo, the first co-resident seat runs
	// the single real engine aggregate and every sibling receives that same result, so
	// EVERY participating attempt-2 seat succeeds with the identical signature and none
	// surfaces interactive_attempt_already_aggregated (that error now fails the test).
	winners := 0
	var sharedSignature []byte
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("attempt 2 seat %d failed: %v", seats2[i].member, r.err)
		}
		if len(r.sig) != 64 {
			t.Fatalf("attempt 2 seat %d: want a 64-byte BIP-340 signature, got %d bytes", seats2[i].member, len(r.sig))
		}
		if sharedSignature == nil {
			sharedSignature = r.sig
		} else if !bytes.Equal(r.sig, sharedSignature) {
			t.Fatalf("attempt 2 seat %d produced a different signature than another aggregating seat", seats2[i].member)
		}
		winners++
		state, err := seats2[i].coord.State(seats2[i].handle)
		if err != nil {
			t.Fatalf("attempt 2 seat %d state: %v", seats2[i].member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("attempt 2 seat %d aggregated but is not Succeeded: %v", seats2[i].member, state)
		}
	}
	if winners != len(seats2) {
		t.Fatalf("attempt 2: expected every reshuffled-subset seat to aggregate the shared signature, got %d of %d", winners, len(seats2))
	}
}

type seatResult struct {
	sig []byte
	err error
}

type seatRunResult struct {
	done chan struct{}
	sig  []byte
	err  error
}

// runSeatAsync runs a seat's runner in a goroutine and signals completion.
func runSeatAsync(s *dropoutSeat, ctx context.Context) *seatRunResult {
	r := &seatRunResult{done: make(chan struct{})}
	go func() {
		r.sig, r.err = s.runner.Run(ctx)
		close(r.done)
	}()
	return r
}

func containsMember(set []group.MemberIndex, m group.MemberIndex) bool {
	for _, x := range set {
		if x == m {
			return true
		}
	}
	return false
}

func memberSlicesEqualLocal(a, b []group.MemberIndex) bool {
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
