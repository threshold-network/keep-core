//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"fmt"

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
// Round-2 packages carry secret-share material and MUST be delivered
// confidentially per recipient in production (Phase 1 encrypts them to the
// recipient's operator key, reusing the pkg/tecdsa/dkg ephemeral-ECDH envelope).
// Here they are addressed but not yet encrypted; the bus abstraction keeps that
// change local to the transport.

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
	// Sender is the authenticated originating member (the transport binds it;
	// the orchestrator never trusts an id inside Payload for routing).
	Sender group.MemberIndex
	// Recipient is the addressed member for a round-2 message; unused (0) for a
	// broadcast round-1 message.
	Recipient group.MemberIndex
	Payload   []byte
}

// DKGBus is the DKG round-exchange transport. The in-process implementation here
// drives the orchestrator's deterministic tests; production wraps the wallet
// broadcast channel (with round-2 encrypted to the recipient).
type DKGBus interface {
	// Broadcast delivers msg to every subscriber. A round-2 message is addressed
	// (Recipient set); subscribers keep only the ones addressed to them.
	Broadcast(msg dkgMessage)
	// Subscribe registers a receiver up front (before any Broadcast).
	Subscribe() *dkgBusSubscriber
}

// dkgBusSubscriber exposes one member's typed round streams.
type dkgBusSubscriber struct {
	round1 chan dkgMessage
	round2 chan dkgMessage
}

// distributedDKGRunner drives one member's participation in one distributed FROST
// DKG over a DKGBus. Every secret (round-1/round-2 secret packages, the final key
// package) stays local to this runner; only the public round-1 package and the
// per-recipient round-2 packages cross the bus.
type distributedDKGRunner struct {
	member         group.MemberIndex
	identifier     string
	memberIndexes  []group.MemberIndex
	identifierByID map[group.MemberIndex]string
	idByIdentifier map[string]group.MemberIndex
	threshold      uint16
	engine         distributedDKGEngine
	bus            DKGBus
	sub            *dkgBusSubscriber
}

func newDistributedDKGRunner(
	member group.MemberIndex,
	memberIndexes []group.MemberIndex,
	identifierByID map[group.MemberIndex]string,
	threshold uint16,
	engine distributedDKGEngine,
	bus DKGBus,
) (*distributedDKGRunner, error) {
	switch {
	case engine == nil:
		return nil, fmt.Errorf("distributed dkg: engine is nil")
	case bus == nil:
		return nil, fmt.Errorf("distributed dkg: bus is nil")
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
	idByIdentifier := make(map[string]group.MemberIndex, len(identifierByID))
	memberInSet := false
	for m, id := range identifierByID {
		idByIdentifier[id] = m
	}
	for _, m := range memberIndexes {
		if _, ok := identifierByID[m]; !ok {
			return nil, fmt.Errorf("distributed dkg: no identifier for member [%d] in set", m)
		}
		if m == member {
			memberInSet = true
		}
	}
	if !memberInSet {
		return nil, fmt.Errorf("distributed dkg: member [%d] is not in the member set", member)
	}
	return &distributedDKGRunner{
		member:         member,
		identifier:     identifier,
		memberIndexes:  memberIndexes,
		identifierByID: identifierByID,
		idByIdentifier: idByIdentifier,
		threshold:      threshold,
		engine:         engine,
		bus:            bus,
		// Subscribe at construction so no peer's round-1 broadcast is missed.
		sub: bus.Subscribe(),
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
		if err := r.broadcastPackage(dkgRound2Message, recipient, pkg); err != nil {
			return nil, err
		}
	}
	round2Packages, err := r.collectRound2(ctx, n-1)
	if err != nil {
		return nil, err
	}

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
		Type:      msgType,
		Sender:    r.member,
		Recipient: recipient,
		Payload:   payload,
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
			if msg.Sender == r.member {
				continue // never our own echo
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
			return nil, fmt.Errorf(
				"distributed dkg: collect round2: got [%d] of [%d] packages: %w",
				len(bySender), want, ctx.Err(),
			)
		case msg := <-r.sub.round2:
			if msg.Sender == r.member || msg.Recipient != r.member {
				continue // not addressed to us (or our own echo)
			}
			if _, have := bySender[msg.Sender]; have {
				continue
			}
			var pkg NativeFROSTDKGRound2Package
			if err := json.Unmarshal(msg.Payload, &pkg); err != nil {
				continue
			}
			// The package must be addressed to us; reject a misrouted one.
			if pkg.Identifier != r.identifier {
				continue
			}
			// Tag the sender: Part3 keys round-2 packages by sender while the
			// package itself carries the recipient identifier.
			pkg.SenderIdentifier = r.identifierByID[msg.Sender]
			pkgCopy := pkg
			bySender[msg.Sender] = &pkgCopy
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
	subscribers []*dkgBusSubscriber
	bufferSize  int
}

// NewInProcessDKGBus returns an in-process DKG bus with per-stream buffers of the
// given size.
func NewInProcessDKGBus(bufferSize int) DKGBus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &inProcessDKGBus{bufferSize: bufferSize}
}

func (b *inProcessDKGBus) Subscribe() *dkgBusSubscriber {
	s := &dkgBusSubscriber{
		round1: make(chan dkgMessage, b.bufferSize),
		round2: make(chan dkgMessage, b.bufferSize),
	}
	b.subscribers = append(b.subscribers, s)
	return s
}

func (b *inProcessDKGBus) Broadcast(msg dkgMessage) {
	for _, s := range b.subscribers {
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
