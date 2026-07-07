//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

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

	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, members, identifierByID, 2, noopDKGEngine{}, bus)
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
		runner.sub.round1 <- dkgMessage{Type: dkgRound1Message, Sender: sender, Payload: payload}
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

// TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers guards the routing map:
// two members sharing an identifier would collapse round-2 routing, so the
// constructor must fail closed.
func TestNewDistributedDKGRunner_RejectsDuplicateIdentifiers(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{
		1: "id-1", 2: "dup", 3: "dup", // 2 and 3 collide
	}
	bus := NewInProcessDKGBus(16)
	if _, err := newDistributedDKGRunner(1, members, identifierByID, 2, noopDKGEngine{}, bus); err == nil {
		t.Fatal("expected a duplicate-identifier error, got nil")
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
	bus := NewInProcessDKGBus(16)
	// self = 3; it collects round-2 packages addressed to id-3 from members 1, 2.
	runner, err := newDistributedDKGRunner(3, members, identifierByID, 2, noopDKGEngine{}, bus)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	push := func(sender group.MemberIndex) {
		payload, err := json.Marshal(NativeFROSTDKGRound2Package{
			Identifier: identifierByID[3], // addressed to us
			Data:       []byte{byte(sender)},
		})
		if err != nil {
			t.Fatalf("marshal round2 package: %v", err)
		}
		runner.sub.round2 <- dkgMessage{
			Type:      dkgRound2Message,
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

// TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage pins the
// fail-into-retry contract: when an expected round-2 package never arrives, the
// collection returns a deadline error (which fails the DKG into the existing
// retry/challenge path) rather than hanging.
func TestDistributedDKGRunner_CollectRound2TimesOutOnMissingPackage(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3}
	identifierByID := map[group.MemberIndex]string{1: "id-1", 2: "id-2", 3: "id-3"}
	bus := NewInProcessDKGBus(16)
	runner, err := newDistributedDKGRunner(3, members, identifierByID, 2, noopDKGEngine{}, bus)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Only ONE of the two expected round-2 packages arrives.
	payload, err := json.Marshal(NativeFROSTDKGRound2Package{Identifier: identifierByID[3], Data: []byte{1}})
	if err != nil {
		t.Fatalf("marshal round2 package: %v", err)
	}
	runner.sub.round2 <- dkgMessage{Type: dkgRound2Message, Sender: 1, Recipient: 3, Payload: payload}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if _, err := runner.collectRound2(ctx, len(members)-1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline-exceeded error when a round-2 package never arrives; got %v", err)
	}
}
