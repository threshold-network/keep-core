//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"fmt"
	"sort"

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
//     equivocation drains, the coordinator never reads its own package stream,
//     and for t-of-included finalize the coordinator stops reading commitments
//     once t have arrived while non-coordinators never read commitments at all -
//     so the transport must apply backpressure or drop, never block forever, on
//     an undrained or oversubscribed stream.
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
	// includedMembers is the attempt's included set as a lookup, cached at
	// construction. It gates which shares the collector retains as evidence (any
	// included member's, even a non-signer observer's divergent share), distinct
	// from the per-attempt signer set that gates which shares count toward the
	// aggregate.
	includedMembers map[group.MemberIndex]struct{}
	engine          interactiveSigningEngine
	collector       *roast.Round2Collector
	coordinator     roast.Coordinator
	signer          roast.Signer
	bus             RunnerBus
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
	// The coordinator's round-1 collection proceeds until t responsive committers,
	// so t must not exceed the included set - otherwise the subset can never form
	// and the coordinator would block to the ctx deadline on every attempt. A
	// well-formed attempt always selects at least threshold members; fail fast at
	// construction rather than silently degrade into timeout-driven retries.
	if int(threshold) > len(attemptCtx.IncludedSet) {
		return nil, fmt.Errorf(
			"roast runner: threshold [%d] exceeds the attempt's included set size [%d]",
			threshold, len(attemptCtx.IncludedSet),
		)
	}
	// The signed message is the binding's MessageDigest, derived here rather than
	// accepted as a parameter: a caller-supplied message that diverged from the
	// digest the attempt (and its package/share envelopes) is bound to could mark
	// an attempt for digest A succeeded with a signature over digest B.
	return &interactiveSigningRunner{
		attempt:         attempt,
		member:          member,
		messageDigest:   append([]byte(nil), attemptCtx.MessageDigest[:]...),
		threshold:       threshold,
		includedMembers: setOf(attemptCtx.IncludedSet),
		engine:          engine,
		collector:       collector,
		coordinator:     coordinator,
		signer:          signer,
		bus:             bus,
		sub:             bus.Subscribe(),
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
	// Compare in uint16 space (widening the uint8 elected is lossless): a
	// truncating group.MemberIndex(...) cast would let a malformed engine
	// coordinator > 255 alias an honest member (e.g. 257 -> 1) and falsely match.
	if derived.AttemptContext.CoordinatorIdentifier != uint16(elected) {
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
	//   - SUCCESS as a SIGNER: prune this attempt's round-2 collector state per the
	//     prune-on-conclusion contract (nothing needs it), so a collector reused
	//     across attempts does not retain concluded attempts indefinitely. Round 2
	//     already consumed our round-1 nonces, so no abort is needed.
	//   - SUCCESS as an OBSERVER: a member that committed in round 1 but was not in
	//     the chosen signing subset obtains the signature by aggregating the
	//     subset's broadcast shares WITHOUT signing, so its round-1 nonces are
	//     still resident in the engine. Prune the collector AND abort the session
	//     to drop those nonces - the success branch otherwise suppresses the abort.
	//   - FAILURE / early exit: abort the engine session so it drops this attempt's
	//     resident secret nonces, but do NOT prune the collector. A failure path
	//     (the root-divergence return below, or an aggregate share-verification
	//     failure) retains signed evidence that the blame/retry path must still
	//     read via CoordinatorPackageProofs / ClassifyCandidateCulprits; the caller
	//     prunes after snapshotting or propagating it.
	// Best-effort: neither may mask the run's real outcome.
	succeeded := false
	// signedRound2 records whether this node ran round 2 and thereby consumed its
	// round-1 nonces; an observer never does, so it must still abort on success.
	signedRound2 := false
	defer func() {
		if succeeded {
			r.collector.PruneAttempt(contextHash[:])
			if !signedRound2 {
				_, _ = r.engine.InteractiveSessionAbort(binding.SessionID(), &attemptID)
			}
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

	// 4. Only the elected coordinator collects commitments - it alone builds the
	// signing package. It gathers the first t responsive committers (its own
	// already seeded) and builds the package over exactly that subset, so a member
	// past the t-th to commit (slow or offline) never stalls the attempt. Every
	// other member broadcast its own commitment above and now awaits the package.
	commitments := map[group.MemberIndex][]byte{r.member: ownCommitments}
	if r.member == elected {
		if err := r.collectCommitments(ctx, r.sub.Commitments(), contextHash, includedSet, commitments); err != nil {
			return nil, fmt.Errorf("roast runner: collect commitments: %w", err)
		}
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

	// 6. The package names the chosen signing subset (the first t responsive
	// committers the coordinator built it over). A local member IN that subset is
	// a signer; one NOT in it is an OBSERVER - it committed in round 1 but was not
	// chosen, so it does not sign and only aggregates the subset's broadcast
	// shares. An empty signer set is the full-included flow (back-compat / no
	// oversizing): every included member signs. SignerIDs() is safe here -
	// Unmarshal and AuthenticateSigningPackage (via RecordSigningPackage) both
	// Validate, so each id is a real, ascending, distinct member index.
	signerSet := pkg.SignerIDs()
	if len(signerSet) == 0 {
		signerSet = includedSet
	}

	// 7. Round 2 (signers only): our signature share, recorded locally and
	// broadcast. An observer skips round 2 entirely, leaving its round-1 nonces
	// resident - the cleanup defer aborts them on success.
	shares := map[group.MemberIndex][]byte{}
	if memberInSet(r.member, signerSet) {
		ownShare, err := r.engine.InteractiveRound2(binding.SessionID(), attemptID, uint16(r.member), pkg.SigningPackageBytes)
		if err != nil {
			return nil, fmt.Errorf("roast runner: round 2: %w", err)
		}
		// Round 2 consumed our round-1 nonces: a successful signer prunes without
		// aborting; only a non-signing observer still needs the abort.
		signedRound2 = true
		ownSubmission, ownSubmissionEnvelope, err := r.signShareSubmission(pkg, contextHash, elected, ownShare)
		if err != nil {
			return nil, err
		}
		if err := r.collector.RecordShareSubmission(ownSubmission); err != nil {
			return nil, fmt.Errorf("roast runner: record own share submission: %w", err)
		}
		r.broadcast(RunnerMsgShareSubmission, contextHash, ownSubmissionEnvelope)
		shares[r.member] = ownShare
	}

	// 8. Collect a round-2 share from every member in the chosen signing subset (a
	// signer already has its own; an observer collects all t), as inner FROST
	// share bytes. A subset signer that never broadcasts a share stalls the
	// attempt to the ctx deadline -> fail -> the existing ROAST retry path.
	if err := r.collectShares(ctx, r.sub.Shares(), contextHash, signerSet, shares); err != nil {
		return nil, fmt.Errorf("roast runner: collect shares: %w", err)
	}

	// 9. Aggregate the subset's shares. Aggregation is a public operation over the
	// package and the t broadcast shares, so signers and observers alike obtain
	// the same signature; an observer aggregates against its own open session
	// without having contributed a share. A share-verification failure surfaces
	// the typed error with candidate culprits for the (separate) blame path.
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

	// 10. Mark the attempt succeeded so the cleanup path produces no transition
	// bundle for a completed attempt.
	if err := r.coordinator.MarkSucceeded(binding.Handle()); err != nil {
		return nil, fmt.Errorf("roast runner: mark succeeded: %w", err)
	}
	// The attempt is finalized; the cleanup defer prunes. A signer's nonces are
	// spent so it does not abort; an observer aborts via the !signedRound2 branch.
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
	// The chosen signing subset is exactly the members whose commitments the
	// package was built over (the first t responsive committers). Carry it in
	// signer_ids so non-coordinators learn which members to await shares from and
	// which committed members are observers. The FROST SigningPackageBytes is the
	// cryptographic source of truth, so this is a liveness hint, never blame.
	signerIDs := sortedMemberIndices(commitments)
	envelope, err := r.signSigningPackage(contextHash, elected, signerIDs, frostPackage)
	if err != nil {
		return nil, err
	}
	r.broadcast(RunnerMsgSigningPackage, contextHash, envelope)
	return envelope, nil
}

func (r *interactiveSigningRunner) signSigningPackage(
	contextHash [attempt.MessageDigestLength]byte,
	elected group.MemberIndex,
	signerIDs []group.MemberIndex,
	frostPackage []byte,
) ([]byte, error) {
	pkg := &roast.SigningPackage{
		AttemptContextHash:  append([]byte(nil), contextHash[:]...),
		CoordinatorIDValue:  uint32(elected),
		SigningPackageBytes: frostPackage,
		SignerIDsValue:      memberIndicesToUint32(signerIDs),
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

// collectCommitments (coordinator only) fills `into` with the first t responsive
// committers' round-1 commitments - the coordinator's own already seeded, then
// the first t-1 included peers to arrive - and STOPS at t (r.threshold). The
// coordinator builds the FROST package over exactly this t-subset, so a member
// past the t-th to commit (slow or offline) never stalls the attempt: collection
// proceeds the instant t have committed. If ctx expires first (fewer than t ever
// commit), the run fails into the existing ROAST retry path. t <= len(included)
// always, so the loop terminates once t included members have committed.
func (r *interactiveSigningRunner) collectCommitments(
	ctx context.Context,
	stream <-chan RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	includedSet []group.MemberIndex,
	into map[group.MemberIndex][]byte,
) error {
	included := setOf(includedSet)
	for len(into) < int(r.threshold) {
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

// collectShares fills `into` with each share from the chosen signing subset
// (`signerSet`) as inner FROST share bytes - a signer's own already seeded, an
// observer's none - counting the first accepted body per sender, and retains
// EVERY well-formed share in the collector (including a sender's later,
// body-different ones) which is where member equivocation is detected. It
// collects over the signer set, NOT the full included set: a committed member
// the coordinator did not choose is an observer that contributes no share, so
// waiting for it would stall every attempt that omitted an offline member.
func (r *interactiveSigningRunner) collectShares(
	ctx context.Context,
	stream <-chan RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	signerSet []group.MemberIndex,
	into map[group.MemberIndex][]byte,
) error {
	signers := setOf(signerSet)
	for len(into) < len(signers) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-stream:
			r.recordShareMessage(msg, contextHash, signers, into)
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
			r.recordShareMessage(msg, contextHash, signers, into)
		default:
			return nil
		}
	}
	return nil
}

// recordShareMessage validates a share-submission bus message, retains it in the
// collector, and counts it toward `into` only when the sender is in the chosen
// signing subset and it is the sender's first accepted share. Retention is gated
// by INCLUDED-set membership, not the signer set: the collector deliberately
// keeps an included non-signer's DIVERGENT share - a targeted coordinator-
// equivocation victim that signed a different package - as
// EquivocationKindDivergentShare evidence for the f+1 blame/transition path, so
// the runner must hand any included member's well-formed share to the collector
// even though only signer-set shares count toward the aggregate. Recording BEFORE
// the already-collected check is what lets the collector observe member
// equivocation (a body-different second signed share ->
// EquivocationKindShareConflict / DivergentShare); a divergent / conflicting /
// unauthenticated share is retained where applicable but never counted. Retention
// is bounded (the collector keeps the first per submitter and only emits on the
// rest), and the bus delivers only body-different duplicates.
func (r *interactiveSigningRunner) recordShareMessage(
	msg RunnerMessage,
	contextHash [attempt.MessageDigestLength]byte,
	signers map[group.MemberIndex]struct{},
	into map[group.MemberIndex][]byte,
) {
	if msg.Attempt != contextHash {
		return
	}
	// Retain shares from any INCLUDED member, not just the signing subset: an
	// included non-signer's divergent share is targeted-equivocation evidence the
	// collector keeps. An outsider (not in the included set) is dropped here rather
	// than handed to the collector, which would reject it as not-included anyway.
	if _, included := r.includedMembers[msg.Sender]; !included {
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
	// Only the chosen signing subset's shares count toward this aggregate; a
	// committed-but-unchosen observer's share is retained above as evidence but
	// never counted.
	if _, isSigner := signers[msg.Sender]; !isSigner {
		return
	}
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
// canonicalization agrees with the bound attempt, independent of order. It
// rejects duplicates in `derived` (consuming each expected member at most once),
// so a malformed list like [1,1] for included [1,2] does NOT falsely match
// despite the equal length.
func sameMemberSet(derived []uint16, included []group.MemberIndex) bool {
	if len(derived) != len(included) {
		return false
	}
	remaining := make(map[uint16]struct{}, len(included))
	for _, m := range included {
		remaining[uint16(m)] = struct{}{}
	}
	for _, d := range derived {
		if _, ok := remaining[d]; !ok {
			return false
		}
		delete(remaining, d)
	}
	return len(remaining) == 0
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

// sortedMemberIndices returns the keys of a member-indexed map in ascending
// order - the chosen signing subset the coordinator carries in the package's
// signer_ids, which SigningPackage.Validate requires strictly ascending (the map
// keys are distinct, so sorting yields a strictly-ascending list).
func sortedMemberIndices(m map[group.MemberIndex][]byte) []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(m))
	for member := range m {
		out = append(out, member)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// memberIndicesToUint32 widens an ascending member-index slice to the wire
// (uint32) form SigningPackage.SignerIDsValue carries. Widening uint8 ->
// uint32 is lossless, and ascending/distinct order is preserved.
func memberIndicesToUint32(members []group.MemberIndex) []uint32 {
	out := make([]uint32, 0, len(members))
	for _, member := range members {
		out = append(out, uint32(member))
	}
	return out
}
