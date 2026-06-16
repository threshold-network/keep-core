//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// 1. Open the interactive session; the engine returns the attempt id used
	// for every subsequent round.
	open, err := r.engine.InteractiveSessionOpen(
		binding.SessionID(),
		uint16(r.member),
		r.messageDigest,
		attemptCtx.KeyGroupID,
		r.threshold,
		binding.TaprootMerkleRoot(),
		nativeAttemptContext(binding),
	)
	if err != nil {
		return nil, fmt.Errorf("roast runner: open session: %w", err)
	}
	attemptID := open.AttemptID

	// Cleanup on conclusion - the attempt concludes for this runner on any exit:
	//   - Always prune this attempt's round-2 collector state, per the collector's
	//     prune-on-conclusion contract. A collector reused across attempts would
	//     otherwise retain every concluded attempt's package/share envelopes
	//     indefinitely. A no-op if the attempt was never begun, and (when the
	//     blame path lands) it extracts its evidence into the transition bundle
	//     within Run, before this defer fires.
	//   - On an EARLY exit only (ctx cancel, or an error before round 2 consumed
	//     the nonces) abort the engine session so it drops this attempt's resident
	//     secret nonces; a clean success consumed them and clears the flag first.
	// Both are best-effort: they must not mask the run's real outcome.
	succeeded := false
	defer func() {
		r.collector.PruneAttempt(contextHash[:])
		if !succeeded {
			_, _ = r.engine.InteractiveSessionAbort(binding.SessionID(), &attemptID)
		}
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
	signingPackageEnvelope, err := r.obtainSigningPackage(ctx, r.sub.SigningPackages(), elected, contextHash, commitments, includedSet)
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
		toFrostSignatureShares(shares),
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
				return append([]byte(nil), msg.Payload...), nil
			}
		}
	}

	frostPackage, err := r.engine.NewSigningPackage(r.messageDigest, toFrostCommitments(commitments, includedSet))
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

// collectShares fills `into` with at least `need` members' inner FROST share
// bytes (own already seeded), unwrapping each ShareSubmission envelope and
// taking the first accepted body per sender.
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
			if msg.Attempt != contextHash {
				continue
			}
			if _, want := included[msg.Sender]; !want {
				continue
			}
			if _, have := into[msg.Sender]; have {
				continue
			}
			var sub roast.ShareSubmission
			if err := sub.Unmarshal(msg.Payload); err != nil {
				continue
			}
			// Bind the authenticated transport sender to the claimed submitter:
			// a node embedding another member's id would otherwise fill that
			// honest member's slot with garbage, drop their real share, and get
			// them falsely blamed.
			if sub.SubmitterID() != msg.Sender {
				continue
			}
			// Authenticate + retain via the collector. Only an ACCEPTED share
			// (operator-sig valid, binds the elected coordinator AND the
			// authoritative package) counts toward aggregation; a divergent or
			// conflicting share is retained for the blame path but not counted
			// (ErrShareRetainedNotAccepted / ErrShareConflict), and an
			// unauthenticated one is dropped. Retaining peer shares here is also
			// what lets the blame path corroborate engine culprits.
			if err := r.collector.RecordShareSubmission(&sub); err != nil {
				continue
			}
			into[msg.Sender] = append([]byte(nil), sub.SignatureShare...)
		}
	}
	return nil
}

// nativeAttemptContext maps the binding's RFC-21 attempt context to the engine's
// wire shape. AttemptNumber stays 0-based (the bridge converts to the 1-based
// wire value).
//
// IncludedParticipantsFingerprint and AttemptID are PLACEHOLDERS, inert here:
// the only engine #4076 wires is the fake, which ignores them, and no production
// path constructs the real cgo engine yet. They are NOT engine-valid. Strict-mode
// validate_attempt_context (engine roast.rs) recomputes both from canonical
// inputs and rejects a mismatch before round 1:
//   - fingerprint := roast_included_participants_fingerprint_hex(participants)
//     (domain-separated hash of the framed u16 set), and
//   - attempt_id   := roast_attempt_id_hex(session_id, message_digest_hex,
//     attempt_number, coordinator_id, fingerprint_hex).
//
// Producing these byte-for-byte is a cross-impl derivation (the seed-divergence
// class of bug); deriving-in-Go vs exposing-from-engine is the open design fork
// for the real-engine attempt-context wiring increment. The runner already drives
// subsequent rounds with the attempt id the engine RETURNS, not this field.
func nativeAttemptContext(binding *ActiveRoastAttempt) NativeInteractiveAttemptContext {
	ctx := binding.Context()
	included := make([]uint16, 0, len(ctx.IncludedSet))
	for _, m := range ctx.IncludedSet {
		included = append(included, uint16(m))
	}
	hash := binding.ContextHash()
	return NativeInteractiveAttemptContext{
		AttemptNumber:                   ctx.AttemptNumber,
		CoordinatorIdentifier:           uint16(binding.ElectedCoordinator()),
		IncludedParticipants:            included,
		IncludedParticipantsFingerprint: hex.EncodeToString(includedSetFingerprint(ctx.IncludedSet)),
		AttemptID:                       hex.EncodeToString(hash[:]),
	}
}

func includedSetFingerprint(includedSet []group.MemberIndex) []byte {
	h := sha256.New()
	for _, m := range includedSet {
		h.Write([]byte{byte(m)})
	}
	return h.Sum(nil)
}

func toFrostCommitments(commitments map[group.MemberIndex][]byte, includedSet []group.MemberIndex) []nativeFROSTCommitment {
	out := make([]nativeFROSTCommitment, 0, len(includedSet))
	for _, m := range includedSet {
		data, ok := commitments[m]
		if !ok {
			continue
		}
		out = append(out, nativeFROSTCommitment{
			Identifier: memberFrostIdentifier(m),
			Data:       append([]byte(nil), data...),
		})
	}
	return out
}

func toFrostSignatureShares(shares map[group.MemberIndex][]byte) []nativeFROSTSignatureShare {
	out := make([]nativeFROSTSignatureShare, 0, len(shares))
	for m, data := range shares {
		out = append(out, nativeFROSTSignatureShare{
			Identifier: memberFrostIdentifier(m),
			Data:       append([]byte(nil), data...),
		})
	}
	return out
}

// memberFrostIdentifier maps a Go member index to the FROST identifier the
// engine keys signing-package commitments and signature shares by.
//
// This decimal form is a PLACEHOLDER, inert against the fake (which ignores it)
// and never reaching the real engine in #4076. The engine's canonical encoding
// (codec.rs participant_identifier_to_frost_identifier + frost_identifier_to_go_string)
// is the u16 serialized as the secp256k1 scalar - 32-byte big-endian - hex
// encoded; the real engine deserializes exactly that to build the
// BTreeMap<Identifier, _> in the FROST signing package, so "1" would not match
// the member's key share. Replaced with the canonical encoding in the
// real-engine wiring increment.
func memberFrostIdentifier(member group.MemberIndex) string {
	return fmt.Sprintf("%d", member)
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
