//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This file wires the real DISTRIBUTED FROST DKG (three-round Pedersen DKG:
// Part1 -> Part2 -> Part3) into a per-member, bus-driven orchestrator - the shape
// the tBTC node needs. Today the node's wallet-DKG path
// (pkg/tbtc executeTBTCSignerFROSTDKG -> RunDKGWithSeed) is a transitional
// TRUSTED-DEALER key generation (frost::keys::generate_with_dealer seeded by the
// public on-chain seed), which the Rust signer HARD-DISABLES under
// TBTC_SIGNER_PROFILE=production ("production requires distributed DKG wiring").
// The distributed-DKG crypto already exists (engine Part1/Part2/Part3 over the
// tbtc-signer FFI); what was missing is an orchestrator that exchanges the round
// packages between nodes so each node ends up with its OWN secret share of a
// t-of-n key that NO node ever holds in full. This is that orchestrator, driven
// over a message bus so it can later run over the real wallet broadcast channel.
//
// Round structure (frost-core / RFC-9591), for member m with identifier id_m:
//   Round 1: Part1(id_m, n, t) -> a local round-1 SECRET package (never leaves
//     the node) + a public round-1 package, BROADCAST to all. Collect the other
//     n-1 members' round-1 packages.
//   Round 2: Part2(secret1, others' round-1 packages) -> a local round-2 secret
//     package + one round-2 package PER OTHER member (each carrying its
//     recipient's identifier and secret-share material). Send each to its
//     recipient. Collect the n-1 round-2 packages addressed to me.
//   Round 3: Part3(secret2, others' round-1 packages, round-2 packages addressed
//     to me) -> this node's long-term KeyPackage (its secret share) + the group
//     PublicKeyPackage (agreed identically by every honest member).
//
// Round-2 packages carry secret-share material and are delivered confidentially
// per recipient: each share is ECIES-sealed to the recipient's STATIC operator
// key (a fresh sender ephemeral per message; see
// distributed_dkg_round2_seal_frost_native.go). NOTE the forward-secrecy scope:
// this gives per-message secrecy on the SENDER side only. Unlike pkg/tecdsa/dkg,
// which runs a two-sided per-DKG EPHEMERAL key exchange, the recipient key here
// is long-lived, so an observer who records the round-2 broadcasts and LATER
// obtains recipients' operator private keys can recover the shares addressed to
// them. Closing that gap would require an ephemeral-key-exchange round (a
// deliberate trade-off deferred here); the bus abstraction keeps any such change
// local to the transport.

// distributedDKGEngine is the subset of the native FROST engine the DKG
// orchestrator needs. *buildTaggedTBTCSignerEngine satisfies it.
type distributedDKGEngine interface {
	Part1(participantIdentifier string, maxSigners, minSigners uint16) (*NativeFROSTDKGPart1Result, error)
	Part2(
		secretPackage *NativeFROSTDKGRound1SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
	) (*NativeFROSTDKGPart2Result, error)
	Part3(
		secretPackage *NativeFROSTDKGRound2SecretPackage,
		round1Packages []*NativeFROSTDKGRound1Package,
		round2Packages []*NativeFROSTDKGRound2Package,
	) (*NativeFROSTDKGResult, error)
}

// dkgMessageType tags a DKG round message so subscribers route it.
type dkgMessageType int

const (
	// dkgRound1Message carries a member's public round-1 package (broadcast).
	dkgRound1Message dkgMessageType = iota
	// dkgRound2Message carries a round-2 package addressed to one recipient.
	dkgRound2Message
)

// dkgMessage is one DKG round message on the bus. Payload is the JSON-encoded
// round-1 or round-2 package; the bus treats it as opaque.
type dkgMessage struct {
	Type dkgMessageType
	// Session binds a message to ONE DKG attempt. Retries for the same wallet seed
	// share a broadcast channel, so without this discriminator a stale round-1 or
	// round-2 message from a failed prior attempt (from a retained member) would
	// pass the sender/recipient checks, fill that sender's slot in the retry, and
	// mix packages across attempts in Part2/Part3. Collectors accept only messages
	// carrying the current attempt's session.
	Session string
	// Sender is the authenticated originating member (the transport binds it;
	// the orchestrator never trusts an id inside Payload for routing).
	Sender group.MemberIndex
	// SenderPublicKey is the sender's AUTHENTICATED operator public key, set by
	// the transport (the net bus overwrites any claimed value with the key it
	// authenticated the sender against). The orchestrator learns each member's
	// round-2 sealing key from the round-1 message it broadcasts, since peer
	// operator public keys are not known up front (group selection carries only
	// addresses).
	SenderPublicKey []byte
	// Recipient is the addressed member for a round-2 message; unused (0) for a
	// broadcast round-1 message.
	Recipient group.MemberIndex
	Payload   []byte
}

// DKGBus is the DKG round-exchange transport. The in-process implementation here
// drives the orchestrator's deterministic tests; production wraps the wallet
// broadcast channel (with round-2 encrypted to the recipient).
type DKGBus interface {
	// Broadcast delivers a round-1 message to every subscriber and a round-2
	// message only to the subscriber for its addressed recipient.
	Broadcast(msg dkgMessage)
	// Subscribe registers a receiver for the given seat up front (before any
	// Broadcast); round-2 messages addressed to that seat are routed to it.
	Subscribe(member group.MemberIndex) *dkgBusSubscriber
	// Start begins delivering inbound messages. It MUST be called only after every
	// local seat has Subscribed: a message handled before any subscriber exists is
	// dropped, and the transport may have already marked it seen and never
	// retransmit it, stalling the DKG.
	Start()
}

// dkgBusSubscriber exposes one member's typed round streams. member is the seat
// this subscriber belongs to: round-1 messages are broadcast to every
// subscriber, but a round-2 message - which is ADDRESSED to one recipient - is
// delivered only to the matching subscriber, so a subscriber's round-2 stream
// holds O(n) messages (the ones addressed to it) rather than the O(n^2) whole-
// group fan-out (~9900 for a 100-seat group), which would overflow the buffer.
type dkgBusSubscriber struct {
	member group.MemberIndex
	round1 chan dkgMessage
	round2 chan dkgMessage

	// mu and seen back the net bus's per-subscriber dedup of byte-identical
	// retransmissions (keyed by full message content hash); the in-process bus
	// leaves them unused.
	mu   sync.Mutex
	seen map[[32]byte]struct{}
}

// distributedDKGRunner drives one member's participation in one distributed FROST
// DKG over a DKGBus. Every secret (round-1/round-2 secret packages, the final key
// package) stays local to this runner; only the public round-1 package and the
// per-recipient round-2 packages cross the bus.
type distributedDKGRunner struct {
	member        group.MemberIndex
	session       string
	identifier    string
	memberIndexes []group.MemberIndex
	// memberSet is the DKG participant set as a lookup. A round message from an
	// authenticated sender NOT in this set (a shared transport may carry other
	// groups' traffic) must be rejected BEFORE it is counted toward a round's
	// collection target - otherwise it fills a slot that a real peer's package is
	// then dropped from, and the runner proceeds with incomplete packages.
	memberSet      map[group.MemberIndex]struct{}
	identifierByID map[group.MemberIndex]string
	idByIdentifier map[string]group.MemberIndex
	threshold      uint16
	engine         distributedDKGEngine
	bus            DKGBus
	sub            *dkgBusSubscriber
	// recipientKeys is LEARNED during round 1: each member's authenticated
	// operator public key (from the SenderPublicKey on its round-1 message) is the
	// key its round-2 share is SEALED to. It is written only by collectRound1 and
	// read only by the later round-2 send, both on the single Run goroutine.
	recipientKeys map[group.MemberIndex]*ephemeral.PublicKey
	// selfKey is this node's operator private key, used to OPEN the round-2 shares
	// sealed to us. selfPublicKey is its marshaled operator public key, stamped on
	// our broadcasts so peers learn the key to seal our share to. Over the net bus
	// the stamp is dropped and re-derived from the AUTHENTICATED transport key, so
	// this trusted stamp path is used only by the in-process test bus.
	selfKey       *ephemeral.PrivateKey
	selfPublicKey []byte
}

func newDistributedDKGRunner(
	member group.MemberIndex,
	session string,
	memberIndexes []group.MemberIndex,
	identifierByID map[group.MemberIndex]string,
	threshold uint16,
	engine distributedDKGEngine,
	bus DKGBus,
	selfKey *ephemeral.PrivateKey,
	selfPublicKey []byte,
) (*distributedDKGRunner, error) {
	switch {
	case engine == nil:
		return nil, fmt.Errorf("distributed dkg: engine is nil")
	case bus == nil:
		return nil, fmt.Errorf("distributed dkg: bus is nil")
	case selfKey == nil:
		return nil, fmt.Errorf("distributed dkg: self key is nil")
	case len(selfPublicKey) == 0:
		return nil, fmt.Errorf("distributed dkg: self public key is empty")
	case session == "":
		return nil, fmt.Errorf("distributed dkg: session is empty")
	case threshold == 0:
		return nil, fmt.Errorf("distributed dkg: threshold is zero")
	case int(threshold) > len(memberIndexes):
		return nil, fmt.Errorf(
			"distributed dkg: threshold [%d] exceeds member count [%d]",
			threshold, len(memberIndexes),
		)
	}
	identifier, ok := identifierByID[member]
	if !ok {
		return nil, fmt.Errorf("distributed dkg: no identifier for member [%d]", member)
	}
	// Build the participant set and the identifier<->member routing maps from the
	// DKG member set only (not the whole identifierByID, which may be broader).
	// Identifiers must be distinct across participants, or round-2 routing would
	// silently collapse two members onto one; fail closed on a collision.
	memberSet := make(map[group.MemberIndex]struct{}, len(memberIndexes))
	idByIdentifier := make(map[string]group.MemberIndex, len(memberIndexes))
	memberInSet := false
	for _, m := range memberIndexes {
		id, ok := identifierByID[m]
		if !ok {
			return nil, fmt.Errorf("distributed dkg: no identifier for member [%d] in set", m)
		}
		if existing, dup := idByIdentifier[id]; dup {
			return nil, fmt.Errorf(
				"distributed dkg: members [%d] and [%d] share identifier %q",
				existing, m, id,
			)
		}
		idByIdentifier[id] = m
		memberSet[m] = struct{}{}
		if m == member {
			memberInSet = true
		}
	}
	if !memberInSet {
		return nil, fmt.Errorf("distributed dkg: member [%d] is not in the member set", member)
	}
	return &distributedDKGRunner{
		member:         member,
		session:        session,
		identifier:     identifier,
		memberIndexes:  memberIndexes,
		memberSet:      memberSet,
		identifierByID: identifierByID,
		idByIdentifier: idByIdentifier,
		threshold:      threshold,
		engine:         engine,
		bus:            bus,
		recipientKeys:  make(map[group.MemberIndex]*ephemeral.PublicKey, len(memberIndexes)),
		selfKey:        selfKey,
		selfPublicKey:  selfPublicKey,
		// Subscribe at construction so no peer's round-1 broadcast is missed;
		// round-2 messages addressed to this seat route here.
		sub: bus.Subscribe(member),
	}, nil
}

// Run executes the three DKG rounds for this member and returns its DKG result
// (secret KeyPackage + the group PublicKeyPackage). It honors ctx cancellation
// while collecting each round's packages: a member that never broadcasts stalls
// the round to the deadline, failing into the (existing) DKG retry/challenge
// path rather than hanging forever.
func (r *distributedDKGRunner) Run(ctx context.Context) (*NativeFROSTDKGResult, error) {
	n := len(r.memberIndexes)

	// ---- Round 1: our public package broadcast; secret kept local. ----
	part1, err := r.engine.Part1(r.identifier, uint16(n), r.threshold)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: part1: %w", err)
	}
	if part1.Package == nil || part1.SecretPackage == nil {
		return nil, fmt.Errorf("distributed dkg: part1 returned an incomplete result")
	}
	// Scrub the round-1 secret package on return: Part2 consumes it below, but it
	// must not linger in the heap afterward (the engine only zeroes its own
	// transport buffers; the copies handed to the Go caller are ours to scrub).
	defer zeroBytes(part1.SecretPackage.Data)
	if err := r.broadcastPackage(dkgRound1Message, 0, part1.Package); err != nil {
		return nil, err
	}
	round1Packages, err := r.collectRound1(ctx, n-1)
	if err != nil {
		return nil, err
	}

	// ---- Round 2: a package per other member, addressed to its recipient. ----
	part2, err := r.engine.Part2(part1.SecretPackage, round1Packages)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: part2: %w", err)
	}
	if part2.SecretPackage == nil {
		return nil, fmt.Errorf("distributed dkg: part2 returned an incomplete result")
	}
	// Scrub the round-2 secret package AND the OUTGOING per-recipient share
	// packages on return: both carry secret material and are consumed only within
	// this function (Part3 consumes the RECEIVED packages, not these). Registered
	// BEFORE the count check so an unexpected-count return still scrubs them.
	defer zeroBytes(part2.SecretPackage.Data)
	defer func() {
		for _, pkg := range part2.Packages {
			if pkg != nil {
				zeroBytes(pkg.Data)
			}
		}
	}()
	if len(part2.Packages) != n-1 {
		return nil, fmt.Errorf(
			"distributed dkg: part2 produced [%d] packages, want [%d]",
			len(part2.Packages), n-1,
		)
	}
	for _, pkg := range part2.Packages {
		recipient, ok := r.idByIdentifier[pkg.Identifier]
		if !ok {
			return nil, fmt.Errorf(
				"distributed dkg: part2 package addressed to unknown identifier",
			)
		}
		// Seal the share to the recipient's key - learned from its round-1 message -
		// before it crosses the group-visible channel: only the recipient can open
		// it. The key is present because we accept a round-1 sender only after
		// learning its key, and Part2 produces packages only for round-1 senders.
		recipientKey, ok := r.recipientKeys[recipient]
		if !ok {
			return nil, fmt.Errorf(
				"distributed dkg: no learned round-2 sealing key for recipient [%d]", recipient,
			)
		}
		sealed, err := sealRound2Share(pkg.Data, recipientKey)
		if err != nil {
			return nil, fmt.Errorf(
				"distributed dkg: seal round-2 share for member [%d]: %w", recipient, err,
			)
		}
		if err := r.broadcastPackage(dkgRound2Message, recipient, sealed); err != nil {
			return nil, err
		}
	}
	round2Packages, err := r.collectRound2(ctx, n-1)
	if err != nil {
		return nil, err
	}
	// The received round-2 packages carry incoming secret-share material; scrub
	// them once Part3 (below) has consumed them.
	defer func() {
		for _, pkg := range round2Packages {
			if pkg != nil {
				zeroBytes(pkg.Data)
			}
		}
	}()

	// ---- Round 3: our long-term secret share + the agreed group key. ----
	result, err := r.engine.Part3(part2.SecretPackage, round1Packages, round2Packages)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: part3: %w", err)
	}
	if result.KeyPackage == nil || result.PublicKeyPackage == nil {
		return nil, fmt.Errorf("distributed dkg: part3 returned an incomplete result")
	}
	return result, nil
}

// broadcastPackage JSON-encodes a round package and broadcasts it on the bus.
func (r *distributedDKGRunner) broadcastPackage(
	msgType dkgMessageType,
	recipient group.MemberIndex,
	pkg any,
) error {
	payload, err := json.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("distributed dkg: marshal package: %w", err)
	}
	r.bus.Broadcast(dkgMessage{
		Type:            msgType,
		Session:         r.session,
		Sender:          r.member,
		SenderPublicKey: r.selfPublicKey,
		Recipient:       recipient,
		Payload:         payload,
	})
	return nil
}

// collectRound1 gathers the other members' public round-1 packages (excluding
// our own), keyed by sender so each member is counted at most once, until `want`
// distinct senders have arrived or ctx expires. The package's embedded identifier
// must match its authenticated sender, so a member cannot smuggle another's slot.
func (r *distributedDKGRunner) collectRound1(
	ctx context.Context,
	want int,
) ([]*NativeFROSTDKGRound1Package, error) {
	bySender := make(map[group.MemberIndex]*NativeFROSTDKGRound1Package, want)
	for len(bySender) < want {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"distributed dkg: collect round1: got [%d] of [%d] packages: %w",
				len(bySender), want, ctx.Err(),
			)
		case msg := <-r.sub.round1:
			if msg.Session != r.session {
				continue // a stale message from a different DKG attempt
			}
			if msg.Sender == r.member {
				continue // never our own echo
			}
			if _, ok := r.memberSet[msg.Sender]; !ok {
				// Not a participant in THIS DKG. A shared transport may carry other
				// groups' traffic; counting it would fill a slot a real peer is then
				// dropped from, so we would proceed with incomplete packages.
				continue
			}
			if _, have := bySender[msg.Sender]; have {
				continue
			}
			var pkg NativeFROSTDKGRound1Package
			if err := json.Unmarshal(msg.Payload, &pkg); err != nil {
				continue
			}
			// Bind the package's own identifier to its authenticated sender: a
			// member must not contribute a round-1 package under another's id.
			if pkg.Identifier != r.identifierByID[msg.Sender] {
				continue
			}
			// Learn this sender's round-2 sealing key from its authenticated operator
			// public key. A sender whose key we cannot parse is skipped, not counted:
			// we could not seal a round-2 share to it, so it must not fill a slot.
			recipientKey, err := ephemeral.UnmarshalPublicKey(msg.SenderPublicKey)
			if err != nil {
				continue
			}
			r.recipientKeys[msg.Sender] = recipientKey
			pkgCopy := pkg
			bySender[msg.Sender] = &pkgCopy
		}
	}
	return sortedRound1Packages(r.memberIndexes, r.member, bySender), nil
}

// collectRound2 gathers the round-2 packages ADDRESSED to us, one per other
// member, tagging each with its authenticated sender's identifier (Part3 keys
// round-2 packages by sender). Packages addressed to other recipients are
// ignored here.
func (r *distributedDKGRunner) collectRound2(
	ctx context.Context,
	want int,
) ([]*NativeFROSTDKGRound2Package, error) {
	bySender := make(map[group.MemberIndex]*NativeFROSTDKGRound2Package, want)
	for len(bySender) < want {
		select {
		case <-ctx.Done():
			// Scrub the partial shares accumulated before the deadline: a missing
			// round-2 package is an expected dropout/retry path, and each held
			// package carries incoming secret-share material. len(bySender) is
			// unaffected by zeroing the payloads, so the count below stays accurate.
			for _, pkg := range bySender {
				zeroBytes(pkg.Data)
			}
			return nil, fmt.Errorf(
				"distributed dkg: collect round2: got [%d] of [%d] packages: %w",
				len(bySender), want, ctx.Err(),
			)
		case msg := <-r.sub.round2:
			if msg.Session != r.session {
				continue // a stale message from a different DKG attempt
			}
			if msg.Sender == r.member || msg.Recipient != r.member {
				continue // not addressed to us (or our own echo)
			}
			if _, ok := r.memberSet[msg.Sender]; !ok {
				continue // not a participant in THIS DKG (shared transport)
			}
			if _, have := bySender[msg.Sender]; have {
				continue
			}
			var sealed sealedRound2Share
			if err := json.Unmarshal(msg.Payload, &sealed); err != nil {
				continue
			}
			// Open the share sealed to us (only we can, via our private key). A
			// share we cannot open - corrupt, tampered, or sealed to someone else -
			// is skipped, not counted: the sender's slot stays empty and the round
			// fails into the retry path if a valid one never arrives.
			share, err := openRound2Share(&sealed, r.selfKey)
			if err != nil {
				continue
			}
			// Reconstruct the round-2 package Part3 consumes: addressed to us (our
			// identifier) and tagged with its authenticated sender (Part3 keys
			// round-2 packages by sender while the package carries the recipient).
			bySender[msg.Sender] = &NativeFROSTDKGRound2Package{
				Identifier:       r.identifier,
				SenderIdentifier: r.identifierByID[msg.Sender],
				Data:             share,
			}
		}
	}
	return sortedRound2Packages(r.memberIndexes, r.member, bySender), nil
}

// sortedRound1Packages returns the collected round-1 packages in ascending
// member order (deterministic input to Part2/Part3), excluding our own.
func sortedRound1Packages(
	memberIndexes []group.MemberIndex,
	self group.MemberIndex,
	bySender map[group.MemberIndex]*NativeFROSTDKGRound1Package,
) []*NativeFROSTDKGRound1Package {
	out := make([]*NativeFROSTDKGRound1Package, 0, len(bySender))
	for _, m := range memberIndexes {
		if m == self {
			continue
		}
		if pkg, ok := bySender[m]; ok {
			out = append(out, pkg)
		}
	}
	return out
}

// sortedRound2Packages returns the round-2 packages addressed to us in ascending
// sender order, excluding our own.
func sortedRound2Packages(
	memberIndexes []group.MemberIndex,
	self group.MemberIndex,
	bySender map[group.MemberIndex]*NativeFROSTDKGRound2Package,
) []*NativeFROSTDKGRound2Package {
	out := make([]*NativeFROSTDKGRound2Package, 0, len(bySender))
	for _, m := range memberIndexes {
		if m == self {
			continue
		}
		if pkg, ok := bySender[m]; ok {
			out = append(out, pkg)
		}
	}
	return out
}

// inProcessDKGBus is the deterministic in-process DKGBus for orchestrator tests.
// Broadcast delivers synchronously into every subscriber's buffered round
// streams; the buffer is sized above a whole DKG's message volume so an honest
// run never blocks.
type inProcessDKGBus struct {
	mu          sync.Mutex
	subscribers []*dkgBusSubscriber
	bufferSize  int
}

// NewInProcessDKGBus returns an in-process DKG bus with per-stream buffers of the
// given size. Sends are blocking, so bufferSize MUST exceed a subscriber's honest
// per-stream volume (round-1: one per member; round-2: one per OTHER member, as
// round-2 is delivered only to its addressed recipient) or a Broadcast can block.
// It is sized generously for the small groups the orchestrator tests use;
// production runs over the net broadcast channel, not this bus.
func NewInProcessDKGBus(bufferSize int) DKGBus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &inProcessDKGBus{bufferSize: bufferSize}
}

func (b *inProcessDKGBus) Subscribe(member group.MemberIndex) *dkgBusSubscriber {
	s := &dkgBusSubscriber{
		member: member,
		round1: make(chan dkgMessage, b.bufferSize),
		round2: make(chan dkgMessage, b.bufferSize),
	}
	b.mu.Lock()
	b.subscribers = append(b.subscribers, s)
	b.mu.Unlock()
	return s
}

// Start is a no-op: the in-process bus delivers synchronously in Broadcast, so
// there is no inbound-receive handler to defer.
func (b *inProcessDKGBus) Start() {}

func (b *inProcessDKGBus) Broadcast(msg dkgMessage) {
	b.mu.Lock()
	subscribers := append([]*dkgBusSubscriber(nil), b.subscribers...)
	b.mu.Unlock()
	for _, s := range subscribers {
		// An addressed round-2 message goes only to its recipient's subscriber.
		if msg.Type == dkgRound2Message && s.member != msg.Recipient {
			continue
		}
		// Own the payload per delivery so no receiver can mutate another's view.
		delivered := msg
		delivered.Payload = append([]byte(nil), msg.Payload...)
		switch msg.Type {
		case dkgRound1Message:
			s.round1 <- delivered
		case dkgRound2Message:
			s.round2 <- delivered
		}
	}
}
