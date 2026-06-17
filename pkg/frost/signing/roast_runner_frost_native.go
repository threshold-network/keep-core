//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// interactiveSigningRunner drives one node's participation in ONE interactive
// signing attempt over a RunnerBus: round-1 commitments, the coordinator's
// signing package, round-2 shares, and aggregation - terminating in
// MarkSucceeded + the BIP-340 signature on the happy path. The blame path (on
// an aggregate share-verification failure) lands separately.
//
// Every value the runner consumes for engine/collector calls is derived from
// the immutable ActiveRoastAttempt binding, never from peer messages, and the
// node records its OWN produced share into the collector explicitly rather than
// relying on bus self-echo.
//
// Transport assumptions - the in-process test bus meets the first; the real
// pkg/net transport MUST meet both:
//  1. RunnerMessage.Sender is the AUTHENTICATED peer identity. The sender==elected
//     and SubmitterID==Sender filters, and per-member commitment slotting, are
//     only as sound as that authentication.
//  2. Delivery must not let a slow or flooding peer block an honest broadcaster
//     indefinitely. The runner does not fully drain every stream - it bounds the
//     equivocation drains, and the coordinator never reads its own package
//     stream - so the transport must apply backpressure or drop, never block
//     forever, on an undrained or oversubscribed stream.
//
// Round-1 commitments are unsigned: with authenticated senders the worst case is
// a member's own bad or equivocated commitment, which surfaces as a round-2
// mismatch and retry, never a cross-member poison or signing breach. Blaming
// commitment equivocation would need signed commitments - a protocol decision
// for the design consult.
type interactiveSigningRunner struct {
	attempt *ActiveRoastAttempt
	member  group.MemberIndex
	// messageDigest is the message FROST signs: the binding's MessageDigest, NOT
	// a separate caller parameter, so the runner can never open/aggregate a
	// message inconsistent with the attempt it is bound to.
	messageDigest []byte
	threshold     uint16
	engine        interactiveSigningEngine
	collector     *roast.Round2Collector
	coordinator   roast.Coordinator
	signer        roast.Signer
	bus           RunnerBus
	// sub is established at construction (before any Run broadcasts) so a node
	// never misses a peer message broadcast before it subscribed.
	sub *RunnerBusSubscriber
}

func newInteractiveSigningRunner(
	attempt *ActiveRoastAttempt,
	member group.MemberIndex,
	threshold uint16,
	engine interactiveSigningEngine,
	collector *roast.Round2Collector,
	coordinator roast.Coordinator,
	signer roast.Signer,
	bus RunnerBus,
) (*interactiveSigningRunner, error) {
	switch {
	case attempt == nil:
		return nil, fmt.Errorf("roast runner: active attempt is nil")
	case engine == nil:
		return nil, fmt.Errorf("roast runner: engine is nil")
	case collector == nil:
		return nil, fmt.Errorf("roast runner: collector is nil")
	case coordinator == nil:
		return nil, fmt.Errorf("roast runner: coordinator is nil")
	case signer == nil:
		return nil, fmt.Errorf("roast runner: signer is nil")
	case bus == nil:
		return nil, fmt.Errorf("roast runner: bus is nil")
	case threshold == 0:
		return nil, fmt.Errorf("roast runner: threshold is zero")
	}
	attemptCtx := attempt.Context()
	if !memberInSet(member, attemptCtx.IncludedSet) {
		return nil, fmt.Errorf(
			"roast runner: member %d is not in the attempt's included set", member,
		)
	}
	// The signed message is the binding's MessageDigest, derived here rather than
	// accepted as a parameter: a caller-supplied message that diverged from the
	// digest the attempt (and its package/share envelopes) is bound to could mark
	// an attempt for digest A succeeded with a signature over digest B.
	return &interactiveSigningRunner{
		attempt:       attempt,
		member:        member,
		messageDigest: append([]byte(nil), attemptCtx.MessageDigest[:]...),
		threshold:     threshold,
		engine:        engine,
		collector:     collector,
		coordinator:   coordinator,
		signer:        signer,
		bus:           bus,
		sub:           bus.Subscribe(),
	}, nil
}

// Run executes the happy-path interactive signing flow for this node and
// returns the aggregated signature on success. It subscribes to the bus before
// broadcasting so no peer message is missed, and honors ctx cancellation while
// collecting commitments and shares.
func (r *interactiveSigningRunner) Run(ctx context.Context) ([]byte, error) {
	binding := r.attempt
	attemptCtx := binding.Context()
	includedSet := attemptCtx.IncludedSet
	contextHash := binding.ContextHash()
	elected := binding.ElectedCoordinator()

	// 1. Derive the canonical attempt context + per-participant FROST identifiers
	// from the engine (single source of truth - the runner never re-implements the
	// engine's domain-separated derivations). Cross-check the engine-derived
	// coordinator and included set against the binding's own RFC-21 election so a
	// divergence fails closed HERE rather than producing a signature bound to the
	// wrong coordinator/set; the two independent derivations must agree.
	derived, err := r.engine.DeriveInteractiveAttemptContext(
		binding.SessionID(),
		r.messageDigest,
		attemptCtx.KeyGroupID,
		r.threshold,
		attemptCtx.AttemptNumber,
		includedSetToUint16(includedSet),
	)
	if err != nil {
		return nil, fmt.Errorf("roast runner: derive attempt context: %w", err)
	}
	if group.MemberIndex(derived.AttemptContext.CoordinatorIdentifier) != elected {
		return nil, fmt.Errorf(
			"roast runner: engine-derived coordinator [%d] does not match the bound elected coordinator [%d]",
			derived.AttemptContext.CoordinatorIdentifier, elected,
		)
	}
	if !sameMemberSet(derived.AttemptContext.IncludedParticipants, includedSet) {
		return nil, fmt.Errorf("roast runner: engine-derived included set diverges from the bound attempt")
	}
	frostIdentifiers, err := frostIdentifierMap(derived.FrostIdentifiers, includedSet)
	if err != nil {
		return nil, fmt.Errorf("roast runner: %w", err)
	}

	// 2. Open the interactive session with the engine-derived context; the engine
	// returns the attempt id used for every subsequent round.
	open, err := r.engine.InteractiveSessionOpen(
		binding.SessionID(),
		uint16(r.member),
		r.messageDigest,
		attemptCtx.KeyGroupID,
		r.threshold,
		binding.TaprootMerkleRoot(),
		derived.AttemptContext,
	)
	if err != nil {
		return nil, fmt.Errorf("roast runner: open session: %w", err)
	}
	attemptID := open.AttemptID

	// Cleanup on conclusion, by outcome:
	//   - SUCCESS: prune this attempt's round-2 collector state per the
	//     prune-on-conclusion contract (nothing needs it), so a collector reused
	//     across attempts does not retain concluded attempts indefinitely.
	//   - FAILURE / early exit: abort the engine session so it drops this
	//     attempt's resident secret nonces, but do NOT prune the collector. A
	//     failure path (the root-divergence return below, or - once it lands - an
	//     aggregate share-verification failure) retains signed evidence that the
	//     blame/retry path must still read via CoordinatorPackageProofs /
	//     ClassifyCandidateCulprits; the caller prunes after snapshotting or
	//     propagating it.
	// Best-effort: neither may mask the run's real outcome.
	succeeded := false
	defer func() {
		if succeeded {
			r.collector.PruneAttempt(contextHash[:])
			return
		}
		_, _ = r.engine.InteractiveSessionAbort(binding.SessionID(), &attemptID)
	}()

	// 2. Begin collecting evidence for this attempt (elected coordinator from the
	// binding, not a peer).
	if err := r.collector.BeginAttempt(contextHash[:], elected, includedSet); err != nil {
		return nil, fmt.Errorf("roast runner: collector begin attempt: %w", err)
	}

	// 3. Round 1: our commitments, broadcast to the group (own kept locally).
	ownCommitments, err := r.engine.InteractiveRound1(binding.SessionID(), attemptID, uint16(r.member))
	if err != nil {
		return nil, fmt.Errorf("roast runner: round 1: %w", err)
	}
	r.broadcast(RunnerMsgCommitments, contextHash, ownCommitments)

	// 4. Collect every included member's commitments.
	commitments := map[group.MemberIndex][]byte{r.member: ownCommitments}
	if err := r.collectCommitments(ctx, r.sub.Commitments(), contextHash, includedSet, commitments); err != nil {
		return nil, fmt.Errorf("roast runner: collect commitments: %w", err)
	}

	// 5. The elected coordinator builds, signs, and broadcasts the signing
	// package; everyone else awaits it.
	signingPackageEnvelope, err := r.obtainSigningPackage(ctx, r.sub.SigningPackages(), elected, contextHash, commitments, includedSet, frostIdentifiers)
	if err != nil {
		return nil, err
	}
	pkg := &roast.SigningPackage{}
	if err := pkg.Unmarshal(signingPackageEnvelope); err != nil {
		return nil, fmt.Errorf("roast runner: unmarshal signing package: %w", err)
	}
	// Authenticate + retain the coordinator-signed package (Q1 boundary lives in
	// the collector, not here).
	if err := r.collector.RecordSigningPackage(pkg); err != nil {
		return nil, fmt.Errorf("roast runner: record signing package: %w", err)
	}
	// Retain any FURTHER body-different packages the elected coordinator already
	// broadcast for this attempt: the authoritative one is recorded above, so
	// these record as coordinator equivocation (EquivocationKindSigningPackageConflict,
	// surfaced via CoordinatorPackageProofs). obtainSigningPackage returns on the
	// first package, so without this the duplicates the bus deliberately preserves
	// would be lost. Non-coordinators only - the coordinator does not receive its
	// own broadcast.
	if r.member != elected {
		r.recordBufferedCoordinatorPackages(r.sub.SigningPackages(), elected, contextHash)
	}
	// Refuse a package whose taproot root diverges from the bound root: signing
	// Round 2 against it would sign for the WRONG tweak. The package was retained
	// above as evidence for the blame path; we just must not sign it.
	if !r.taprootRootMatches(pkg.TaprootMerkleRoot) {
		return nil, fmt.Errorf("roast runner: signing package taproot root diverges from the bound session root")
	}

	// 6. Round 2: our signature share, recorded locally and broadcast.
	ownShare, err := r.engine.InteractiveRound2(binding.SessionID(), attemptID, uint16(r.member), pkg.SigningPackageBytes)
	if err != nil {
		return nil, fmt.Errorf("roast runner: round 2: %w", err)
	}
	ownSubmission, ownSubmissionEnvelope, err := r.signShareSubmission(pkg, contextHash, elected, ownShare)
	if err != nil {
		return nil, err
	}
	if err := r.collector.RecordShareSubmission(ownSubmission); err != nil {
		return nil, fmt.Errorf("roast runner: record own share submission: %w", err)
	}
	r.broadcast(RunnerMsgShareSubmission, contextHash, ownSubmissionEnvelope)

	// 7. Collect a share from every signer in the package (own already in), as
	// inner FROST share bytes. The package was built from the whole responsive
	// included set, so the aggregate needs a share from each of them (silent-
	// member subsetting is the retry path's concern, not the happy flow).
	shares := map[group.MemberIndex][]byte{r.member: ownShare}
	if err := r.collectShares(ctx, r.sub.Shares(), contextHash, includedSet, shares); err != nil {
		return nil, fmt.Errorf("roast runner: collect shares: %w", err)
	}

	// 8. Aggregate. A share-verification failure surfaces the typed error with
	// candidate culprits for the (separate) blame path.
	signature, err := r.engine.InteractiveAggregate(
		binding.SessionID(),
		attemptID,
		pkg.SigningPackageBytes,
		toFrostSignatureShares(shares, frostIdentifiers),
		binding.TaprootMerkleRoot(),
	)
	if err != nil {
		return nil, fmt.Errorf("roast runner: aggregate: %w", err)
	}

	// 9. Mark the attempt succeeded so the cleanup path produces no transition
	// bundle for a completed attempt.
	if err := r.coordinator.MarkSucceeded(binding.Handle()); err != nil {
		return nil, fmt.Errorf("roast runner: mark succeeded: %w", err)
	}
	// Aggregation consumed the nonces and the attempt is finalized; suppress the
	// deferred abort.
	succeeded = true
	return signature, nil
}

func (r *interactiveSigningRunner) broadcast(t RunnerMessageType, attempt [attempt.MessageDigestLength]byte, payload []byte) {
	r.bus.Broadcast(RunnerMessage{Type: t, Sender: r.member, Attempt: attempt, Payload: payload})
}

// obtainSigningPackage returns the attempt's signing-package envelope: built and
// broadcast locally when this node is the elected coordinator, or awaited from
// the bus otherwise.
func (r *interactiveSigningRunner) obtainSigningPackage(
	ctx context.Context,
	stream <-chan RunnerMessage,
	elected group.MemberIndex,
	contextHash [attempt.MessageDigestLength]byte,
	commitments map[group.MemberIndex][]byte,
	includedSet []group.MemberIndex,
	frostIdentifiers map[group.MemberIndex]string,
) ([]byte, error) {
	if r.member != elected {
		// Accept ONLY the elected coordinator's package for THIS attempt. Without
		// this, any node could broadcast a garbage package; the honest member
		// would forward it to RecordSigningPackage, fail authentication, and abort
		// its run before the real package ever arrives.
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case msg := <-stream:
				if msg.Attempt != contextHash || msg.Sender != elected {
					continue
				}
				// Trust the SIGNED attempt hash, not just the unsigned outer bus
				// field. A package the coordinator legitimately signed for ANOTHER
				// attempt, rewrapped in a current-attempt message, must not drive
				// this flow - it could be recorded/authenticated under that other
				// attempt (if live in this shared collector) and we would sign the
				// wrong package. Keep waiting for the package signed for THIS attempt.
				pkg := &roast.SigningPackage{}
				if err := pkg.Unmarshal(msg.Payload); err != nil {
					continue
				}
				if !bytes.Equal(pkg.AttemptContextHash, contextHash[:]) {
					continue
				}
				return append([]byte(nil), msg.Payload...), nil
			}
		}
	}

	frostPackage, err := r.engine.NewSigningPackage(r.messageDigest, toFrostCommitments(commitments, includedSet, frostIdentifiers))
	if err != nil {
		return nil, fmt.Errorf("roast runner: new signing package: %w", err)
	}
	envelope, err := r.signSigningPackage(contextHash, elected, frostPackage)
	if err != nil {
		return nil, err
	}
	r.broadcast(RunnerMsgSigningPackage, contextHash, envelope)
	return envelope, nil
}

func (r *interactiveSigningRunner) signSigningPackage(
	contextHash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	frostPackage []byte,
) ([]byte, error) {
	pkg := &roast.SigningPackage{
		AttemptContextHash:  append([]byte(nil), contextHash[:]...),
		CoordinatorIDValue:  uint32(elected),
		SigningPackageBytes: frostPackage,
	}
	if root := r.attempt.TaprootMerkleRoot(); root != nil {
		pkg.TaprootMerkleRoot = append([]byte(nil), root[:]...)
	}
	payload, err := pkg.SignableBytes()
	if err != nil {
		return nil, fmt.Errorf("roast runner: signing package signable bytes: %w", err)
	}
	sig, err := r.signer.Sign(payload)
	if err != nil {
		return nil, fmt.Errorf("roast runner: sign signing package: %w", err)
	}
	pkg.CoordinatorSignature = sig
	envelope, err := pkg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("roast runner: marshal signing package: %w", err)
	}
	return envelope, nil
}

func (r *interactiveSigningRunner) signShareSubmission(
	pkg *roast.SigningPackage,
	contextHash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	share []byte,
) (*roast.ShareSubmission, []byte, error) {
	packageHash, err := pkg.BodyHash()
	if err != nil {
		return nil, nil, fmt.Errorf("roast runner: signing package body hash: %w", err)
	}
	sub := &roast.ShareSubmission{
		AttemptContextHash: append([]byte(nil), contextHash[:]...),
		SubmitterIDValue:   uint32(r.member),
		CoordinatorIDValue: uint32(elected),
		SigningPackageHash: append([]byte(nil), packageHash[:]...),
		SignatureShare:     append([]byte(nil), share...),
	}
	payload, err := sub.SignableBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("roast runner: share submission signable bytes: %w", err)
	}
	sig, err := r.signer.Sign(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("roast runner: sign share submission: %w", err)
	}
	sub.SubmitterSignature = sig
	envelope, err := sub.Marshal()
	if err != nil {
		return nil, nil, fmt.Errorf("roast runner: marshal share submission: %w", err)
	}
	return sub, envelope, nil
}

// collectCommitments fills `into` with every included member's commitments
// (own already seeded), taking the first body per sender.
func (r *interactiveSigningRunner) collectCommitments(
	ctx context.Context,
	stream <-chan RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	includedSet []group.MemberIndex,
	into map[group.MemberIndex][]byte,
) error {
	included := setOf(includedSet)
	for len(into) < len(included) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-stream:
			// Ignore messages for another attempt (the bus may carry several),
			// from non-included senders, and a sender already collected. Round-1
			// commitments are unsigned; a spoofed commitment for a member is
			// caught engine-side at Round2, which byte-checks the member's own
			// commitment against the package.
			if msg.Attempt != contextHash {
				continue
			}
			if _, want := included[msg.Sender]; !want {
				continue
			}
			if _, have := into[msg.Sender]; have {
				continue
			}
			into[msg.Sender] = append([]byte(nil), msg.Payload...)
		}
	}
	return nil
}

// collectShares fills `into` with each included member's inner FROST share
// bytes (own already seeded), counting the first accepted body per sender, and
// retains EVERY well-formed share in the collector - including a sender's later,
// body-different ones - which is where member equivocation is detected.
func (r *interactiveSigningRunner) collectShares(
	ctx context.Context,
	stream <-chan RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	includedSet []group.MemberIndex,
	into map[group.MemberIndex][]byte,
) error {
	included := setOf(includedSet)
	for len(into) < len(included) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-stream:
			r.recordShareMessage(msg, contextHash, included, into)
		}
	}
	// `into` is full, but the slot-filling share may have body-different
	// duplicates already queued behind it on the stream. Record ONLY those
	// buffered at entry: the bound is the queue length now, so a peer that keeps
	// the stream non-empty (e.g. flooding body-different shares) cannot starve a
	// receive-until-empty loop and livelock the drain, stalling aggregation. Late
	// arrivals are the blame path's concern.
	for i, n := 0, len(stream); i < n; i++ {
		select {
		case msg := <-stream:
			r.recordShareMessage(msg, contextHash, included, into)
		default:
			return nil
		}
	}
	return nil
}

// recordShareMessage validates a share-submission bus message, retains it in the
// collector, and counts it toward `into` when it is the sender's first accepted
// share. Recording BEFORE the already-collected check is what lets the collector
// observe member equivocation (a body-different second signed share ->
// EquivocationKindShareConflict / DivergentShare); a divergent / conflicting /
// unauthenticated share is retained where applicable but never counted.
// Retention is bounded (the collector keeps the first per submitter and only
// emits on the rest), and the bus delivers only body-different duplicates.
func (r *interactiveSigningRunner) recordShareMessage(
	msg RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	included map[group.MemberIndex]struct{},
	into map[group.MemberIndex][]byte,
) {
	if msg.Attempt != contextHash {
		return
	}
	if _, want := included[msg.Sender]; !want {
		return
	}
	var sub roast.ShareSubmission
	if err := sub.Unmarshal(msg.Payload); err != nil {
		return
	}
	// Bind the authenticated transport sender to the claimed submitter: a node
	// embedding another member's id would otherwise fill that honest member's
	// slot with garbage, drop their real share, and get them falsely blamed.
	if sub.SubmitterID() != msg.Sender {
		return
	}
	// Trust the SIGNED attempt hash, not just the unsigned outer bus field. The
	// collector keys RecordShareSubmission by sub.AttemptContextHash, so a share
	// the submitter signed for ANOTHER attempt - rewrapped in a current-attempt
	// message - would be recorded under that attempt (accepted, returning nil if
	// it is live in this shared collector) and counted toward THIS aggregate.
	if !bytes.Equal(sub.AttemptContextHash, contextHash[:]) {
		return
	}
	recordErr := r.collector.RecordShareSubmission(&sub)
	if _, have := into[msg.Sender]; have {
		return
	}
	if recordErr != nil {
		return
	}
	into[msg.Sender] = append([]byte(nil), sub.SignatureShare...)
}

// recordBufferedCoordinatorPackages records the signing packages the elected
// coordinator has already broadcast for this attempt that are buffered at entry,
// so the collector can surface coordinator equivocation. The caller MUST have
// recorded the authoritative package first, so a body-different one here records
// as the conflicting package (not the authoritative). Bounded by the queue
// length at entry so a flooding peer cannot livelock a receive-until-empty loop;
// continuous monitoring across a real transport is the blame path's concern.
// RecordSigningPackage authenticates the coordinator signature, so a
// forged-sender package is rejected rather than retained.
func (r *interactiveSigningRunner) recordBufferedCoordinatorPackages(
	stream <-chan RunnerMessage,
	elected group.MemberIndex,
	contextHash [attempt.MessageDigestLength]byte,
) {
	for i, n := 0, len(stream); i < n; i++ {
		select {
		case msg := <-stream:
			if msg.Attempt != contextHash || msg.Sender != elected {
				continue
			}
			pkg := &roast.SigningPackage{}
			if err := pkg.Unmarshal(msg.Payload); err != nil {
				continue
			}
			// Signed attempt hash, not the unsigned outer field (see
			// obtainSigningPackage): never record a package signed for another
			// attempt under it via this attempt's drain.
			if !bytes.Equal(pkg.AttemptContextHash, contextHash[:]) {
				continue
			}
			_ = r.collector.RecordSigningPackage(pkg)
		default:
			return
		}
	}
}

// includedSetToUint16 converts the binding's member-index set to the u16 list the
// engine's DeriveInteractiveAttemptContext expects.
func includedSetToUint16(includedSet []group.MemberIndex) []uint16 {
	out := make([]uint16, 0, len(includedSet))
	for _, m := range includedSet {
		out = append(out, uint16(m))
	}
	return out
}

// sameMemberSet reports whether the engine-derived participant list is the same
// SET as the binding's included members - the cross-check that the engine's
// canonicalization agrees with the bound attempt, independent of order.
func sameMemberSet(derived []uint16, included []group.MemberIndex) bool {
	if len(derived) != len(included) {
		return false
	}
	want := make(map[uint16]struct{}, len(included))
	for _, m := range included {
		want[uint16(m)] = struct{}{}
	}
	for _, d := range derived {
		if _, ok := want[d]; !ok {
			return false
		}
	}
	return true
}

// frostIdentifierMap indexes the engine-derived FROST identifiers by Go member
// index, requiring exactly one per included member. The engine returns one per
// participant and the bridge already verified the 1:1 correspondence, so a gap
// here is defensive against a future engine/bridge change.
func frostIdentifierMap(
	entries []NativeFROSTParticipantIdentifier,
	includedSet []group.MemberIndex,
) (map[group.MemberIndex]string, error) {
	out := make(map[group.MemberIndex]string, len(entries))
	for _, entry := range entries {
		out[group.MemberIndex(entry.ParticipantIdentifier)] = entry.FrostIdentifier
	}
	for _, m := range includedSet {
		if out[m] == "" {
			return nil, fmt.Errorf("missing FROST identifier for included member [%d]", m)
		}
	}
	return out, nil
}

// toFrostCommitments keys each collected commitment by the engine-derived FROST
// identifier for that member (frostIdentifiers), so NewSigningPackage builds the
// FROST BTreeMap<Identifier, _> the member's key share expects.
func toFrostCommitments(
	commitments map[group.MemberIndex][]byte,
	includedSet []group.MemberIndex,
	frostIdentifiers map[group.MemberIndex]string,
) []nativeFROSTCommitment {
	out := make([]nativeFROSTCommitment, 0, len(includedSet))
	for _, m := range includedSet {
		data, ok := commitments[m]
		if !ok {
			continue
		}
		out = append(out, nativeFROSTCommitment{
			Identifier: frostIdentifiers[m],
			Data:       append([]byte(nil), data...),
		})
	}
	return out
}

// toFrostSignatureShares keys each collected share by the engine-derived FROST
// identifier for that member, so InteractiveAggregate matches each share to the
// member's verifying share.
func toFrostSignatureShares(
	shares map[group.MemberIndex][]byte,
	frostIdentifiers map[group.MemberIndex]string,
) []nativeFROSTSignatureShare {
	out := make([]nativeFROSTSignatureShare, 0, len(shares))
	for m, data := range shares {
		out = append(out, nativeFROSTSignatureShare{
			Identifier: frostIdentifiers[m],
			Data:       append([]byte(nil), data...),
		})
	}
	return out
}

// taprootRootMatches reports whether a received package's taproot root equals
// the bound session root, honoring key-path (nil bound root <-> empty package
// root) semantics.
func (r *interactiveSigningRunner) taprootRootMatches(packageRoot []byte) bool {
	bound := r.attempt.TaprootMerkleRoot()
	if bound == nil {
		return len(packageRoot) == 0
	}
	return bytes.Equal(packageRoot, bound[:])
}

func memberInSet(member group.MemberIndex, set []group.MemberIndex) bool {
	for _, m := range set {
		if m == member {
			return true
		}
	}
	return false
}

func setOf(members []group.MemberIndex) map[group.MemberIndex]struct{} {
	out := make(map[group.MemberIndex]struct{}, len(members))
	for _, m := range members {
		out[m] = struct{}{}
	}
	return out
}
