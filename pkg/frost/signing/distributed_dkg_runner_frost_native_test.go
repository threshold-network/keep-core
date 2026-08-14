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
// members' operator keys (both secp256k1): the private keys open round-2 shares,
// the marshaled public keys are the "operator public keys" peers learn from
// round-1 messages and seal round-2 shares to.
func dkgTestKeys(t *testing.T, members []group.MemberIndex) (
	map[group.MemberIndex]*ephemeral.PrivateKey,
	map[group.MemberIndex][]byte,
) {
	t.Helper()
	priv := make(map[group.MemberIndex]*ephemeral.PrivateKey, len(members))
	pub := make(map[group.MemberIndex][]byte, len(members))
	for _, m := range members {
		kp, err := ephemeral.GenerateKeyPair()
		if err != nil {
			t.Fatalf("key for member %d: %v", m, err)
		}
		priv[m] = kp.PrivateKey
		pub[m] = kp.PublicKey.Marshal()
	}
	return priv, pub
}

// sealTestShareToKey seals a round-2 share for a marshaled recipient key, as the
// orchestrator does before broadcasting.
func sealTestShareToKey(t *testing.T, share []byte, recipientMarshaled []byte) []byte {
	t.Helper()
	recipient, err := ephemeral.UnmarshalPublicKey(recipientMarshaled)
	if err != nil {
		t.Fatalf("parse recipient key: %v", err)
	}
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
// Feeding a non-member FIRST and then the two real peers must still collect both
// real peers' packages.
func TestDistributedDKGRunner_CollectRound1RejectsNonParticipants(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "id-2", 3: "id-3", 4: "id-4",
	}
	// Keys for all senders that appear (including non-member 4).
	priv, pub := dkgTestKeys(t, []group.MemberIndex{1, 2, 3, 4})

	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
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
		runner.sub.round1 <- dkgMessage{
			Type: dkgRound1Message, Session: testDKGSession, Sender: sender, SenderPublicKey: pub[sender], Payload: payload,
		}
	}
	push(4) // non-member first
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
	// The real peers' sealing keys must have been learned from their round-1 keys.
	if runner.recipientKeys[1] == nil || runner.recipientKeys[2] == nil {
		t.Fatalf("expected round-2 sealing keys learned for members 1 and 2")
	}
}

// TestDistributedDKGRunner_CollectRound1IgnoresOtherSessions guards the retry
// hazard: a stale round-1 message from a FAILED prior attempt (a different
// session) sharing the channel must not be counted, so Part2/Part3 never mix
// packages across attempts. Wrong-session messages are ignored -> timeout.
func TestDistributedDKGRunner_CollectRound1IgnoresOtherSessions(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	for _, sender := range []group.MemberIndex{1, 2} {
		payload, err := json.Marshal(NativeFROSTDKGRound1Package{Identifier: identifierByID[sender], Data: []byte{byte(sender)}})
		if err != nil {
			t.Fatalf("marshal round1 package: %v", err)
		}
		runner.sub.round1 <- dkgMessage{
			Type: dkgRound1Message, Session: "prior-attempt-session", Sender: sender, SenderPublicKey: pub[sender], Payload: payload,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound1(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stale-session round-1 messages must be ignored (timeout expected); got %v", err)
	}
}

// TestDistributedDKGRunner_CollectRound1SkipsUnparseableKey ensures a round-1
// sender whose operator key cannot be parsed is skipped (we could not seal a
// round-2 share to it), so it does not fill a slot.
func TestDistributedDKGRunner_CollectRound1SkipsUnparseableKey(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	payload, _ := json.Marshal(NativeFROSTDKGRound1Package{Identifier: identifierByID[1], Data: []byte{1}})
	// Member 1 with a garbage operator key; member 2 with a good one.
	runner.sub.round1 <- dkgMessage{Type: dkgRound1Message, Session: testDKGSession, Sender: 1, SenderPublicKey: []byte{0x00, 0x01}, Payload: payload}
	payload2, _ := json.Marshal(NativeFROSTDKGRound1Package{Identifier: identifierByID[2], Data: []byte{2}})
	runner.sub.round1 <- dkgMessage{Type: dkgRound1Message, Session: testDKGSession, Sender: 2, SenderPublicKey: pub[2], Payload: payload2}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound1(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a round-1 sender with an unparseable key must be skipped (timeout expected); got %v", err)
	}
}

// TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers guards the routing map.
func TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "dup", 3: "dup"}
	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	if _, err := newDistributedDKGRunner(1, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[1], pub[1]); err == nil {
		t.Fatal("expected a duplicate-identifier error, got nil")
	}
}

// TestNewDistributedDKGRunner_FailsClosedOnMissingInputs covers the fail-closed
// construction guards: an empty self public key and an empty session.
func TestNewDistributedDKGRunner_FailsClosedOnMissingInputs(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	bus := NewInProcessDKGBus(16)
	priv, pub := dkgTestKeys(t, members)

	if _, err := newDistributedDKGRunner(1, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[1], nil); err == nil {
		t.Fatal("expected an empty self-public-key error, got nil")
	}
	if _, err := newDistributedDKGRunner(1, "", members, identifierByID, 2, noopDKGEngine{}, bus, priv[1], pub[1]); err == nil {
		t.Fatal("expected an empty-session error, got nil")
	}
}

// TestDistributedDKGRunner_CollectRound2RejectsNonParticipants is the round-2
// analogue of the round-1 guard.
func TestDistributedDKGRunner_CollectRound2RejectsNonParticipants(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "id-2", 3: "id-3", 4: "id-4",
	}
	priv, pub := dkgTestKeys(t, []group.MemberIndex{1, 2, 3, 4})
	bus := NewInProcessDKGBus(16)
	// self = 3; it collects round-2 shares addressed to it from members 1, 2.
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	push := func(sender group.MemberIndex) {
		payload := sealTestShareToKey(t, []byte{byte(sender)}, pub[3]) // sealed to us
		runner.sub.round2 <- dkgMessage{
			Type: dkgRound2Message, Session: testDKGSession, Sender: sender, Recipient: 3, Payload: payload,
		}
	}
	push(4) // non-member addressed to us first
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
// a DIFFERENT key cannot be opened, so it is skipped (timeout), not accepted.
func TestDistributedDKGRunner_CollectRound2SkipsUnopenableShare(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Sealed to member 1's key (NOT ours) but routed to us: we cannot open it.
	runner.sub.round2 <- dkgMessage{
		Type: dkgRound2Message, Session: testDKGSession, Sender: 2, Recipient: 3, Payload: sealTestShareToKey(t, []byte{2}, pub[1]),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound2(ctx, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an unopenable share must be skipped (timeout expected); got %v", err)
	}
}

// TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage pins the
// fail-into-retry contract for a missing round-2 package.
func TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, testDKGSession, members, identifierByID, 2, noopDKGEngine{}, bus, priv[3], pub[3])
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Only ONE of the two expected round-2 shares arrives.
	runner.sub.round2 <- dkgMessage{
		Type: dkgRound2Message, Session: testDKGSession, Sender: 1, Recipient: 3, Payload: sealTestShareToKey(t, []byte{1}, pub[3]),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := runner.collectRound2(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline-exceeded error when a round-2 package never arrives; got %v", err)
	}
}
