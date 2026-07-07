//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const testDKGSession = "dkg-test-session"

// noopDKGEngine satisfies distributedDKGEngine without touching the FFI, so the
// bus/collection logic is testable without the linked signer lib (no cgo).
type noopDKGEngine struct{}

func (noopDKGEngine) Part1(string, uint16, uint16) (*NativeFROSTDKGPart1Result, error) {
	return nil, nil
}

func (noopDKGEngine) Part2(
	*NativeFROSTDKGRound1SecretPackage,
	[]*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	return nil, nil
}

func (noopDKGEngine) Part3(
	*NativeFROSTDKGRound2SecretPackage,
	[]*NativeFROSTDKGRound1Package,
	[]*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	return nil, nil
}

// dkgTestKeys generates a per-member ephemeral keypair standing in for the
// members' operator keys (both secp256k1), returning the public keys the
// orchestrator seals round-2 shares to and the private keys it opens them with.
func dkgTestKeys(t *testing.T, members []group.MemberIndex) (
	map[group.MemberIndex]*ephemeral.PublicKey,
	map[group.MemberIndex]*ephemeral.PrivateKey,
) {
	t.Helper()
	pub := make(map[group.MemberIndex]*ephemeral.PublicKey, len(members))
	priv := make(map[group.MemberIndex]*ephemeral.PrivateKey, len(members))
	for _, m := range members {
		kp, err := ephemeral.GenerateKeyPair()
		if err != nil {
			t.Fatalf("key for member %d: %v", m, err)
		}
		pub[m] = kp.PublicKey
		priv[m] = kp.PrivateKey
	}
	return pub, priv
}

// sealTestShare produces the sealed round-2 payload the orchestrator broadcasts,
// so collection tests exercise the real open path.
func sealTestShare(t *testing.T, share []byte, recipient *ephemeral.PublicKey) []byte {
	t.Helper()
	sealed, err := sealRound2Share(share, recipient)
	if err != nil {
		t.Fatalf("seal test share: %v", err)
	}
	payload, err := json.Marshal(sealed)
	if err != nil {
		t.Fatalf("marshal sealed share: %v", err)
	}
	return payload
}

// TestDistributedDKGRunner_CollectRound1RejectsNonParticipants guards the
// shared-transport hazard: a message from an authenticated sender that is NOT in
// THIS DKG's member set must not be counted toward the round's collection target.
// If it were, it would fill a slot that a real peer's package is then dropped
// from (sortedRound1Packages iterates only the member set), and the runner would
// proceed to Part2/Part3 with incomplete packages. Feeding a non-member FIRST and
// then the two real peers must still collect both real peers' packages.
func TestDistributedDKGRunner_CollectRound1RejectsNonParticipants(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	// identifierByID is deliberately BROADER than the member set: member 4 belongs
	// to another group sharing the transport, but has a valid, well-formed id.
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "id-2", 3: "id-3", 4: "id-4",
	}
	pub, priv := dkgTestKeys(t, members)

	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	push := func(sender group.MemberIndex) {
		payload, err := json.Marshal(NativeFROSTDKGRound1Package{
			Identifier: identifierByID[sender],
			Data:       []byte{byte(sender)},
		})
		if err != nil {
			t.Fatalf("marshal round1 package: %v", err)
		}
		runner.sub.round1 <- dkgMessage{Type: dkgRound1Message, Session: testDKGSession, Sender: sender, Payload: payload}
	}
	// The non-member arrives FIRST; without the member-set gate it would fill a
	// slot and let collection finish before real peer 2 is read.
	push(4)
	push(1)
	push(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := runner.collectRound1(ctx, len(members)-1) // want 2 real peers
	if err != nil {
		t.Fatalf("collectRound1: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 real-peer round-1 packages, got %d (a non-member was counted early?)", len(got))
	}
	seen := map[string]bool{}
	for _, pkg := range got {
		seen[pkg.Identifier] = true
	}
	if !seen["id-1"] || !seen["id-2"] || seen["id-4"] {
		t.Fatalf("collected the wrong peers: %v", seen)
	}
}

// TestDistributedDKGRunner_CollectRound1IgnoresOtherSessions guards the
// retry hazard: retries for the same wallet seed share a broadcast channel, so a
// stale round-1 message from a FAILED prior attempt (a different session) must
// not be counted - otherwise Part2/Part3 would mix packages across attempts.
// Real-member messages carrying the WRONG session are ignored, so collection
// stalls to the deadline rather than accepting them.
func TestDistributedDKGRunner_CollectRound1IgnoresOtherSessions(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	pub, priv := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Both real peers' packages, but stamped with a DIFFERENT (prior-attempt)
	// session. They must be ignored.
	for _, sender := range []group.MemberIndex{1, 2} {
		payload, err := json.Marshal(NativeFROSTDKGRound1Package{Identifier: identifierByID[sender], Data: []byte{byte(sender)}})
		if err != nil {
			t.Fatalf("marshal round1 package: %v", err)
		}
		runner.sub.round1 <- dkgMessage{Type: dkgRound1Message, Session: "prior-attempt-session", Sender: sender, Payload: payload}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound1(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stale-session round-1 messages must be ignored (timeout expected); got %v", err)
	}
}

// TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers guards the routing map:
// two members sharing an identifier would collapse round-2 routing, so the
// constructor must fail closed.
func TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "dup", 3: "dup", // 2 and 3 collide
	}
	pub, priv := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	if _, err := newDistributedDKGRunner(1, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[1]); err == nil {
		t.Fatal("expected a duplicate-identifier error, got nil")
	}
}

// TestNewDistributedDKGRunner_FailsClosedOnMissingInputs covers the fail-closed
// construction guards: a missing sealing key and an empty session.
func TestNewDistributedDKGRunner_FailsClosedOnMissingInputs(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	bus := NewInProcessDKGBus(16)

	pub, priv := dkgTestKeys(t, members)
	delete(pub, 2) // member 2 has no sealing key
	if _, err := newDistributedDKGRunner(1, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[1]); err == nil {
		t.Fatal("expected a missing-sealing-key error, got nil")
	}

	fullPub, fullPriv := dkgTestKeys(t, members)
	if _, err := newDistributedDKGRunner(1, "", members, identifierByID, 2, noopDKGEngine{}, bus, fullPub, fullPriv[1]); err == nil {
		t.Fatal("expected an empty-session error, got nil")
	}
}

// TestDistributedDKGRunner_CollectRound2RejectsNonParticipants is the round-2
// analogue of the round-1 guard: a round-2 message from a non-participant that is
// (spuriously) addressed to us must not be counted toward the round's target,
// which would drop a real peer's incoming share and hand Part3 incomplete input.
func TestDistributedDKGRunner_CollectRound2RejectsNonParticipants(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "id-2", 3: "id-3", 4: "id-4",
	}
	pub, priv := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	// self = 3; it collects round-2 shares addressed to it from members 1, 2.
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	push := func(sender group.MemberIndex) {
		// A share sealed to us (member 3); the sender labels who it claims to be.
		payload := sealTestShare(t, []byte{byte(sender)}, pub[3])
		runner.sub.round2 <- dkgMessage{
			Type:      dkgRound2Message,
			Session:   testDKGSession,
			Sender:    sender,
			Recipient: 3,
			Payload:   payload,
		}
	}
	// A non-member addressed to us arrives first, then the two real peers.
	push(4)
	push(1)
	push(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := runner.collectRound2(ctx, len(members)-1) // want 2 real peers
	if err != nil {
		t.Fatalf("collectRound2: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 real-peer round-2 packages, got %d (a non-member was counted early?)", len(got))
	}
	seen := map[string]bool{}
	for _, pkg := range got {
		seen[pkg.SenderIdentifier] = true
	}
	if !seen["id-1"] || !seen["id-2"] || seen["id-4"] {
		t.Fatalf("collected the wrong senders: %v", seen)
	}
}

// TestDistributedDKGRunner_CollectRound2SkipsUnopenableShare pins the
// bad-share-into-retry contract: a round-2 message addressed to us but sealed to
// a DIFFERENT key cannot be opened, so it must be skipped (not counted), leaving
// the round to time out rather than accepting a share we cannot decrypt.
func TestDistributedDKGRunner_CollectRound2SkipsUnopenableShare(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	pub, priv := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// A share sealed to member 1's key (NOT ours) but routed to us: we cannot open
	// it, so it must be skipped.
	runner.sub.round2 <- dkgMessage{
		Type:      dkgRound2Message,
		Session:   testDKGSession,
		Sender:    2,
		Recipient: 3,
		Payload:   sealTestShare(t, []byte{2}, pub[1]),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound2(ctx, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an unopenable share must be skipped (timeout expected); got %v", err)
	}
}

// TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage pins the
// fail-into-retry contract: when an expected round-2 package never arrives, the
// collection returns a deadline error (which fails the DKG into the existing
// retry/challenge path) rather than hanging.
func TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	pub, priv := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, pub, priv[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Only ONE of the two expected round-2 shares arrives.
	runner.sub.round2 <- dkgMessage{
		Type:      dkgRound2Message,
		Session:   testDKGSession,
		Sender:    1,
		Recipient: 3,
		Payload:   sealTestShare(t, []byte{1}, pub[3]),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if _, err := runner.collectRound2(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline-exceeded error when a round-2 package never arrives; got %v", err)
	}
}
