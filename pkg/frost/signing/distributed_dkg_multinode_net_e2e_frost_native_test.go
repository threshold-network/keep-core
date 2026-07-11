//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/net"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This is the multi-NODE integration test for the distributed-DKG node wiring: n
// independent nodes, each with its OWN operator key, run the distributed FROST
// DKG over the REAL pkg/net broadcast transport through the exported
// RunDistributedDKGForSeats seam - the exact call the node's
// executeDistributedFrostDKG makes. Unlike the in-process-bus e2e, every round
// package crosses the real transport adapter and is membership-authenticated
// against the sender's operator key, and round-2 shares are ECIES-sealed to peers'
// per-DKG EPHEMERAL keys LEARNED from their authenticated round-1 broadcasts. So a
// passing run proves the production orchestration + net bus + key learning +
// encrypted round-2 all work together over real messaging.
//
// All nodes share ONE process-global engine, so each node's persist accumulates
// its seat's key package into one session; after every node persists, a threshold
// subset interactive-signs to a BIP-340 signature that verifies under the group
// key - proving the whole distributed DKG -> persist -> sign path over real
// transport.
func TestDistributedDKG_MultiNode_NetTransport(t *testing.T) {
	setupRealCgoSignerState(t)

	const n = 3
	const threshold uint16 = 2
	members := []group.MemberIndex{1, 2, 3}
	session := fmt.Sprintf("real-cgo-distributed-multinode-%d", realCgoSessionSeq.Add(1))
	message := bytesOf(0x42, 32)

	engine := &buildTaggedTBTCSignerEngine{}

	// One operator per seat; the MembershipValidator maps each seat to that
	// operator's address so the DKG bus authenticates every round message's claimed
	// seat against its authenticated sender key. The operator key stays bound to the
	// channel (transport signing) - the DKG itself uses fresh per-seat ephemeral
	// keys generated inside RunDistributedDKGForSeats, so no operator PRIVATE key is
	// handed to the DKG (recipient-side forward secrecy).
	chainSigning := local_v1.Connect(n, n).Signing()
	publicKeys := make([]*operator.PublicKey, n)
	addresses := make([]chain.Address, n)
	for i := 0; i < n; i++ {
		_, pub, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", i+1, err)
		}
		publicKeys[i] = pub
		addresses[i] = chainSigning.PublicKeyBytesToAddress(operator.MarshalUncompressed(pub))
	}
	validator := group.NewMembershipValidator(&testutils.MockLogger{}, addresses, chainSigning)

	// Canonical identifiers over the full participant set (as the node builds them).
	identifierByID := make(map[group.MemberIndex]string, n)
	for _, m := range members {
		identifierByID[m] = CanonicalFROSTIdentifier(uint16(m))
	}

	channelName := fmt.Sprintf("%s-frost-dkg", session)

	// Join the shared named channel for every node BEFORE running, so no node's
	// round-1 broadcast is lost to a not-yet-connected peer.
	channels := make([]net.BroadcastChannel, n)
	for i := 0; i < n; i++ {
		channel, err := netlocal.ConnectWithKey(publicKeys[i]).BroadcastChannelFor(channelName)
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", i+1, err)
		}
		channels[i] = channel
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type nodeOutcome struct {
		member  group.MemberIndex
		persist map[group.MemberIndex]*NativeTBTCSignerDKGResult
		err     error
	}
	outcomes := make(chan nodeOutcome, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		member := members[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			persist, err := RunDistributedDKGForSeats(
				ctx,
				&testutils.MockLogger{},
				channels[i],
				validator,
				engine,
				session,
				members,
				[]group.MemberIndex{member}, // one seat per node
				identifierByID,
				threshold,
				nil, // no prebuffer: this test subscribes before Start (no readiness barrier)
			)
			outcomes <- nodeOutcome{member: member, persist: persist, err: err}
		}()
	}
	wg.Wait()
	close(outcomes)

	results := make(map[group.MemberIndex]*NativeTBTCSignerDKGResult, n)
	for outcome := range outcomes {
		// Skips in dev when the lib is absent/stale, but FAILS under the require-cgo
		// gate (and fatals on any other error), so the cgo job cannot silently drop
		// this coverage.
		skipFrostUnavailable(
			t,
			fmt.Sprintf("multi-node distributed DKG (seat %d)", outcome.member),
			outcome.err,
		)
		seatResult, ok := outcome.persist[outcome.member]
		if !ok {
			t.Fatalf("node for seat %d returned no persist result for its own seat", outcome.member)
		}
		results[outcome.member] = seatResult
	}

	// Every node agreed on ONE group key over the real transport.
	var keyGroup string
	for _, m := range members {
		if keyGroup == "" {
			keyGroup = results[m].KeyGroup
		} else if results[m].KeyGroup != keyGroup {
			t.Fatalf("node for seat %d disagreed on the group key: %s != %s", m, results[m].KeyGroup, keyGroup)
		}
	}
	if len(keyGroup) != 66 {
		t.Fatalf("group key must be a 33-byte compressed key (66 hex chars); got %d", len(keyGroup))
	}
	t.Logf("%d nodes agreed on group key %s… over the real net transport", n, keyGroup[:16])

	// ---- Interactive threshold signing under a DISTINCT session. ----
	// The DKG persisted under `session`, but interactive ROAST signing runs under a
	// per-message RoastSessionID that is NOT the DKG session - the production shape.
	// The engine resolves the wallet key by keyGroup, so signing under a different
	// session must still work (a distributed-DKG wallet is signable ONLY this way).
	signingSession := session + "-roast-signing"
	signingMembers := members[:threshold]
	includedParticipants := make([]uint16, len(signingMembers))
	for i, m := range signingMembers {
		includedParticipants[i] = uint16(m)
	}

	derived, err := engine.DeriveInteractiveAttemptContext(
		signingSession, message, keyGroup, threshold, 0, includedParticipants,
	)
	if err != nil {
		t.Fatalf("derive interactive attempt context: %v", err)
	}
	frostIDByMember := make(map[group.MemberIndex]string, len(derived.FrostIdentifiers))
	for _, id := range derived.FrostIdentifiers {
		frostIDByMember[group.MemberIndex(id.ParticipantIdentifier)] = id.FrostIdentifier
	}

	var attemptID string
	commitments := make([]nativeFROSTCommitment, 0, len(signingMembers))
	for _, m := range signingMembers {
		open, err := engine.InteractiveSessionOpen(
			signingSession, uint16(m), message, keyGroup, threshold, nil, nil, derived.AttemptContext,
		)
		if err != nil {
			t.Fatalf("interactive session open (member %d): %v", m, err)
		}
		if attemptID == "" {
			attemptID = open.AttemptID
		}
		commitmentData, err := engine.InteractiveRound1(signingSession, open.AttemptID, uint16(m))
		if err != nil {
			t.Fatalf("interactive round 1 (member %d): %v", m, err)
		}
		commitments = append(commitments, nativeFROSTCommitment{
			Identifier: frostIDByMember[m],
			Data:       commitmentData,
		})
	}

	signingPackage, err := engine.NewSigningPackage(message, commitments)
	if err != nil {
		t.Fatalf("new signing package: %v", err)
	}

	shares := make([]nativeFROSTSignatureShare, 0, len(signingMembers))
	for _, m := range signingMembers {
		shareData, err := engine.InteractiveRound2(signingSession, attemptID, uint16(m), signingPackage)
		if err != nil {
			t.Fatalf("interactive round 2 (member %d): %v", m, err)
		}
		shares = append(shares, nativeFROSTSignatureShare{
			Identifier: frostIDByMember[m],
			Data:       shareData,
		})
	}

	signatureBytes, err := engine.InteractiveAggregate(signingSession, attemptID, signingPackage, shares, nil)
	if err != nil {
		t.Fatalf("interactive aggregate: %v", err)
	}
	if len(signatureBytes) != 64 {
		t.Fatalf("aggregate signature length = %d, want 64", len(signatureBytes))
	}

	// Verify against the 32-byte x-only group key (the compressed key without its
	// even-Y "02" prefix).
	groupKeyBytes, err := hex.DecodeString(keyGroup[2:])
	if err != nil {
		t.Fatalf("decode x-only group key: %v", err)
	}
	publicKey, err := schnorr.ParsePubKey(groupKeyBytes)
	if err != nil {
		t.Fatalf("parse group key: %v", err)
	}
	signature, err := schnorr.ParseSignature(signatureBytes)
	if err != nil {
		t.Fatalf("parse signature: %v", err)
	}
	if !signature.Verify(message, publicKey) {
		t.Fatal("interactive threshold signature does not verify under the multi-node DKG group key")
	}
	t.Logf(
		"multi-node distributed DKG -> persist -> interactive %d-of-%d signature verifies over real transport",
		threshold, n,
	)
}
