//go:build frost_native

package signing

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// NativeTBTCSignerDistributedDKGEngine is the engine capability the distributed
// DKG orchestration needs but the base NativeTBTCSignerEngine interface does not
// expose: the stateless three-round FROST primitives plus the op that persists a
// seat's own key package as signing material. The node type-asserts
// CurrentNativeTBTCSignerEngine() to this.
type NativeTBTCSignerDistributedDKGEngine interface {
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
	PersistDistributedDKGKeyPackage(
		sessionID string,
		participantIdentifier uint16,
		threshold uint16,
		participantCount uint16,
		keyPackage *NativeFROSTKeyPackage,
		publicKeyPackage *NativeFROSTPublicKeyPackage,
	) (*NativeTBTCSignerDKGResult, error)
}

// NativeTBTCSignerDistributedDKGRetirementEngine durably removes every local
// key package for an exact DKG key group. It is intentionally separate from the
// execution interface so callers cannot silently assume an older native signer
// can clean up a failed DKG.
type NativeTBTCSignerDistributedDKGRetirementEngine interface {
	RetireDistributedDKGKeyPackages(keyGroup string) error
}

type distributedDKGSeatOutcome struct {
	member  group.MemberIndex
	persist *NativeTBTCSignerDKGResult
	err     error
}

// CanonicalFROSTIdentifier returns the canonical FROST identifier string for a
// participant: the identifier as a 32-byte big-endian scalar (value in the
// least-significant byte), hex-encoded and JSON-quoted. It matches the engine's
// participant_identifier_to_frost_identifier, which PersistDistributedDKGKeyPackage
// re-derives and rejects a mismatch against, and which the interactive signing
// path looks members up by. Callers build identifierByID over the FULL participant
// set with this (NOT the byte-0 scheme used by the stateless DKG tests).
func CanonicalFROSTIdentifier(participantIdentifier uint16) string {
	var id [32]byte
	binary.BigEndian.PutUint16(id[30:], participantIdentifier)
	return fmt.Sprintf("%q", hex.EncodeToString(id[:]))
}

// RunDistributedDKGForSeats runs a real distributed FROST DKG for every local
// seat of this node over the wallet broadcast channel and persists each seat's
// resulting key package as signing material, returning the per-seat persist
// result (all sharing the same group key). It is the node's wallet-DKG path
// (the transitional trusted-dealer path has been removed).
//
// memberIndexes is the FULL participant set (the final compact DKG member space);
// localMemberIndexes are this node's seats in that same space; identifierByID must
// map EVERY member in memberIndexes to its CanonicalFROSTIdentifier. Round-2 shares
// are ECIES-sealed to peers' per-DKG EPHEMERAL keys (generated here, one per local
// seat) learned from their authenticated round-1 broadcasts, so the channel MUST be
// the membership-validated wallet channel; the operator key stays bound to that
// channel and never reaches the DKG.
//
// One orchestrator runs per local seat. All local runners are constructed (and
// thereby subscribed to the shared bus) BEFORE any of them starts, so no
// co-located seat's round-1 broadcast is missed once the channel loops it back.
func RunDistributedDKGForSeats(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	engine NativeTBTCSignerDistributedDKGEngine,
	session string,
	memberIndexes []group.MemberIndex,
	localMemberIndexes []group.MemberIndex,
	identifierByID map[group.MemberIndex]string,
	threshold uint16,
	prebuffer *DKGMessagePrebuffer,
) (map[group.MemberIndex]*NativeTBTCSignerDKGResult, error) {
	if len(localMemberIndexes) == 0 {
		return nil, fmt.Errorf("no local seats to run the distributed DKG for")
	}

	bus, err := NewBroadcastChannelDKGBus(ctx, logger, channel, membershipValidator)
	if err != nil {
		return nil, fmt.Errorf("cannot create the distributed DKG bus: [%v]", err)
	}

	participantCount := uint16(len(memberIndexes))

	// Construct every local runner FIRST so all local seats are subscribed to the
	// bus before any of them broadcasts round 1.
	runners := make(map[group.MemberIndex]*distributedDKGRunner, len(localMemberIndexes))
	for _, seat := range localMemberIndexes {
		// A FRESH per-DKG ephemeral key pair per seat: peers seal this seat's round-2
		// shares to its ephemeral public key (learned from its authenticated round-1
		// broadcast) and it opens them with the ephemeral private key, which is
		// discarded when the DKG ends. This gives the shares two-sided forward
		// secrecy - a recorded broadcast plus a later operator-key leak reveals
		// nothing, because the sealing key never existed at rest.
		seatEphemeral, err := ephemeral.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("cannot generate the ephemeral key for seat [%v]: [%v]", seat, err)
		}
		runner, err := newDistributedDKGRunner(
			seat,
			session,
			memberIndexes,
			identifierByID,
			threshold,
			engine,
			bus,
			seatEphemeral.PrivateKey,
			seatEphemeral.PublicKey.Marshal(),
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create the distributed DKG runner for seat [%v]: [%w]", seat, err)
		}
		runners[seat] = runner
	}

	// Only now that every local seat is subscribed, begin delivering inbound
	// messages - so no peer's early round-1 is handled with no subscriber and
	// dropped (which the transport would not retransmit).
	bus.Start()

	// Hand the prebuffer off to the live bus. It caught round-1 messages a peer broadcast
	// after the readiness barrier released it but before this node reached Start (which
	// the transport would neither deliver - no subscriber yet - nor retransmit). Draining
	// and forwarding is race-free: the captured buffer is delivered AND any message the
	// prebuffer catches around the drain instant is forwarded live, so none is lost in the
	// handoff window. Deduped against live delivery by content hash.
	if prebuffer != nil {
		prebuffer.DrainAndForward(bus.Deliver)
	}

	outcomes := make(chan distributedDKGSeatOutcome, len(runners))
	for seat, runner := range runners {
		seat := seat
		runner := runner
		go func() {
			dkgResult, err := runner.Run(ctx)
			if err != nil {
				outcomes <- distributedDKGSeatOutcome{member: seat, err: fmt.Errorf("distributed DKG for seat [%v] failed: [%w]", seat, err)}
				return
			}
			// dkgResult.KeyPackage.Data is this seat's long-term SECRET share. The engine
			// holds the authoritative copy after PersistDistributedDKGKeyPackage; scrub the
			// Go-side copy when this goroutine returns (on the persist-success AND
			// persist-error paths) so it does not linger in the heap - and thus a later
			// core dump or swap - past persistence. The returned persist result carries
			// only public material (key group), so scrubbing does not affect it.
			if dkgResult.KeyPackage != nil {
				defer zeroBytes(dkgResult.KeyPackage.Data)
			}
			persisted, err := engine.PersistDistributedDKGKeyPackage(
				session,
				uint16(seat),
				threshold,
				participantCount,
				dkgResult.KeyPackage,
				dkgResult.PublicKeyPackage,
			)
			if err != nil {
				outcomes <- distributedDKGSeatOutcome{member: seat, err: fmt.Errorf("cannot persist the key package for seat [%v]: [%w]", seat, err)}
				return
			}
			outcomes <- distributedDKGSeatOutcome{member: seat, persist: persisted}
		}()
	}

	return collectDistributedDKGSeatOutcomes(outcomes, len(runners))
}

func collectDistributedDKGSeatOutcomes(
	outcomes <-chan distributedDKGSeatOutcome,
	count int,
) (map[group.MemberIndex]*NativeTBTCSignerDKGResult, error) {
	persistBySeat := make(map[group.MemberIndex]*NativeTBTCSignerDKGResult, count)
	var keyGroup string
	var firstErr error
	for range count {
		outcome := <-outcomes
		if outcome.err != nil {
			// Keep draining so no goroutine blocks on the channel, but remember the
			// first failure to return once all seats have reported.
			if firstErr == nil {
				firstErr = outcome.err
			}
			continue
		}
		// Record every successful durable write before checking agreement.
		// A mismatching handle is precisely the case where the caller needs all
		// persisted outcomes in order to retire every orphaned key group.
		persistBySeat[outcome.member] = outcome.persist
		if keyGroup == "" {
			keyGroup = outcome.persist.KeyGroup
		} else if outcome.persist.KeyGroup != keyGroup {
			if firstErr == nil {
				firstErr = fmt.Errorf(
					"local seats disagreed on the group key: [%s] != [%s]",
					outcome.persist.KeyGroup, keyGroup,
				)
			}
			continue
		}
	}
	if firstErr != nil {
		// Return successful durable writes alongside the error. The caller must
		// retire their shared key group before abandoning the DKG; discarding the
		// partial map here would strand an orphan when one local seat persists
		// before a sibling fails.
		return persistBySeat, firstErr
	}

	return persistBySeat, nil
}
