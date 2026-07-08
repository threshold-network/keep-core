//go:build frost_native

package signing

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// NativeTBTCSignerDistributedDKGEngine is the engine capability the distributed
// DKG orchestration needs but the base NativeTBTCSignerEngine interface does not
// expose: the stateless three-round FROST primitives plus the op that persists a
// seat's own key package as signing material. The node type-asserts
// CurrentNativeTBTCSignerEngine() to this, mirroring how the transitional dealer
// path asserts NativeTBTCSignerSeededDKGEngine for RunDKGWithSeed.
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
// result (all sharing the same group key). It replaces the transitional
// trusted-dealer RunDKGWithSeed for the node's wallet DKG.
//
// memberIndexes is the FULL participant set (the final compact DKG member space);
// localMemberIndexes are this node's seats in that same space; identifierByID must
// map EVERY member in memberIndexes to its CanonicalFROSTIdentifier. selfOperatorKey
// is this node's operator key (shared by all its local seats): round-2 shares are
// ECIES-sealed to peers' operator keys learned from their authenticated round-1
// broadcasts, so the channel MUST be the membership-validated wallet channel.
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
	selfOperatorKey *operator.PrivateKey,
) (map[group.MemberIndex]*NativeTBTCSignerDKGResult, error) {
	if len(localMemberIndexes) == 0 {
		return nil, fmt.Errorf("no local seats to run the distributed DKG for")
	}
	if selfOperatorKey == nil {
		return nil, fmt.Errorf("self operator key is nil")
	}

	// One operator key for every local seat: peers learn each seat's round-2
	// sealing key from the authenticated operator key on its round-1 broadcasts.
	selfKey, err := operatorPrivateKeyToEphemeral(selfOperatorKey)
	if err != nil {
		return nil, fmt.Errorf("cannot convert operator private key to ephemeral: [%v]", err)
	}
	selfPublicKey := operator.MarshalUncompressed(&selfOperatorKey.PublicKey)

	bus, err := NewBroadcastChannelDKGBus(ctx, logger, channel, membershipValidator)
	if err != nil {
		return nil, fmt.Errorf("cannot create the distributed DKG bus: [%v]", err)
	}

	participantCount := uint16(len(memberIndexes))

	// Construct every local runner FIRST so all local seats are subscribed to the
	// bus before any of them broadcasts round 1.
	runners := make(map[group.MemberIndex]*distributedDKGRunner, len(localMemberIndexes))
	for _, seat := range localMemberIndexes {
		runner, err := newDistributedDKGRunner(
			seat,
			session,
			memberIndexes,
			identifierByID,
			threshold,
			engine,
			bus,
			selfKey,
			selfPublicKey,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create the distributed DKG runner for seat [%v]: [%v]", seat, err)
		}
		runners[seat] = runner
	}

	type seatOutcome struct {
		member  group.MemberIndex
		persist *NativeTBTCSignerDKGResult
		err     error
	}
	outcomes := make(chan seatOutcome, len(runners))
	for seat, runner := range runners {
		seat := seat
		runner := runner
		go func() {
			dkgResult, err := runner.Run(ctx)
			if err != nil {
				outcomes <- seatOutcome{member: seat, err: fmt.Errorf("distributed DKG for seat [%v] failed: [%v]", seat, err)}
				return
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
				outcomes <- seatOutcome{member: seat, err: fmt.Errorf("cannot persist the key package for seat [%v]: [%v]", seat, err)}
				return
			}
			outcomes <- seatOutcome{member: seat, persist: persisted}
		}()
	}

	persistBySeat := make(map[group.MemberIndex]*NativeTBTCSignerDKGResult, len(runners))
	var keyGroup string
	var firstErr error
	for range runners {
		outcome := <-outcomes
		if outcome.err != nil {
			// Keep draining so no goroutine blocks on the channel, but remember the
			// first failure to return once all seats have reported.
			if firstErr == nil {
				firstErr = outcome.err
			}
			continue
		}
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
		persistBySeat[outcome.member] = outcome.persist
	}
	if firstErr != nil {
		return nil, firstErr
	}

	return persistBySeat, nil
}
