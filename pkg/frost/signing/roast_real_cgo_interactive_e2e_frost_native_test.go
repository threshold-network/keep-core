//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/operator"
)

// This file is the REAL-cgo interactive signing end-to-end test: a full FROST DKG
// that PERSISTS a key group, followed by an interactive ROAST signing round
// driven through the engine's INTERACTIVE session API
// (DeriveInteractiveAttemptContext -> InteractiveSessionOpen -> InteractiveRound1
// -> NewSigningPackage -> InteractiveRound2 -> InteractiveAggregate) with real
// frost-secp256k1-tr cryptography. It complements:
//   - TestBuildTaggedTBTCSignerInteractiveFROSTBridge_WithLinkedSigner, which
//     proves the real crypto over the LOW-LEVEL Sign/Aggregate API by feeding
//     KeyPackage bytes directly; and
//   - the fake-engine runner suite, which proves the Go-side orchestration
//     without crypto.
// This is the missing combination: the INTERACTIVE session API the runner uses,
// with real crypto, the prerequisite toward coarse-path retirement.
//
// The DKG -> interactive keyGroup glue is RunDKG: InteractiveSessionOpen resolves
// the signing key by a keyGroup IDENTIFIER (engine-internal persisted material),
// not KeyPackage bytes - so the low-level DKG (Part1/2/3) result, which the test
// holds as bytes, is NOT loadable as an interactive keyGroup. RunDKG runs the full
// DKG and PERSISTS the result under a returned keyGroup the interactive (and
// coarse) signing path then resolves - the same flow production uses (the wallet
// layer runs DKG, gets a keyGroup, then signs).
//
// Correctness is proven by the engine's SUCCESSFUL InteractiveAggregate: FROST
// validates the signature shares and the aggregate internally, so a non-error
// 64-byte BIP-340 signature is a valid threshold signature over the message under
// the keyGroup's group key. An ADDITIONAL external schnorr.Verify is intentionally
// NOT done here: the engine exposes no API to retrieve a keyGroup's group public
// key (RunDKG returns only the keyGroup id, and ExtractDkgGroupPublicKeyFromMaterial
// needs the persisted material bytes the test does not hold), so external
// verification would need a new keyGroup->group-pubkey accessor. That is the one
// remaining nicety; the e2e itself is complete via the internal validation.
//
// To run it, link the signer library so the frost_tbtc_* symbols resolve, e.g.:
//
//	CGO_ENABLED=1 \
//	  CGO_LDFLAGS="-L<dir> -lfrost_tbtc -Wl,-rpath,<dir>" \
//	  go test -tags "frost_native frost_tbtc_signer" \
//	    -run TestRealCgoInteractiveSigning_EndToEnd ./pkg/frost/signing/
//
// Every engine call is guarded by skipFrostUnavailable: when the lib is absent, or
// present but stale (an older dylib missing a newer symbol such as
// frost_tbtc_derive_interactive_attempt_context), the test SKIPS with a message
// naming the operation, rather than failing - so it is inert in CI builds that do
// not link the Rust signer and runs to completion only against a current lib.

func TestRealCgoInteractiveSigning_EndToEnd(t *testing.T) {
	t.Setenv("TBTC_SIGNER_PROFILE", "development")
	t.Setenv("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false")
	// RunDKG persists the DKG result in the signer's ENCRYPTED state, so a linked
	// signer needs a state encryption key and an ISOLATED, fresh state path. Without
	// them a clean linked environment fails before signing (missing key), and a
	// shared/default state path makes reruns conflict on the fixed session id.
	// t.TempDir() yields a fresh path per run, so each run starts clean.
	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}
	t.Setenv("TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX", hex.EncodeToString(stateKey))
	t.Setenv("TBTC_SIGNER_STATE_PATH", filepath.Join(t.TempDir(), "signer-state"))

	engine := &buildTaggedTBTCSignerEngine{}

	const threshold = 2
	participantIDs := []byte{1, 2, 3}
	signingMembers := []byte{1, 2}
	message := bytesOf(0x42, 32)

	// 1. Full DKG that persists a key group. RunDKG runs the whole DKG over the
	// participants and returns a keyGroup the signing path resolves.
	keyGroup := runRealCgoDKGKeyGroup(t, engine, participantIDs, threshold)

	// 2. Interactive signing over the chosen t-subset, driven through the engine's
	// interactive session API - the same calls the interactiveSigningRunner makes,
	// here with real crypto against the DKG-persisted keyGroup.
	derived, err := engine.DeriveInteractiveAttemptContext(
		"real-cgo-session-1",
		message,
		keyGroup,
		threshold,
		0, // 0-based attempt number; the bridge converts to the 1-based wire value
		uint16sOf(signingMembers),
	)
	skipFrostUnavailable(t, "derive interactive attempt context", err)
	frostIDByMember := map[byte]string{}
	for _, id := range derived.FrostIdentifiers {
		frostIDByMember[byte(id.ParticipantIdentifier)] = id.FrostIdentifier
	}

	attemptIDByMember := make(map[byte]string, len(signingMembers))
	commitments := make([]nativeFROSTCommitment, 0, len(signingMembers))
	for _, member := range signingMembers {
		open, err := engine.InteractiveSessionOpen(
			"real-cgo-session-1",
			uint16(member),
			message,
			keyGroup,
			threshold,
			nil, // key-path spend
			derived.AttemptContext,
		)
		skipFrostUnavailable(t, fmt.Sprintf("interactive session open (member %d)", member), err)
		attemptIDByMember[member] = open.AttemptID

		commitmentData, err := engine.InteractiveRound1(
			"real-cgo-session-1", open.AttemptID, uint16(member),
		)
		skipFrostUnavailable(t, fmt.Sprintf("interactive round 1 (member %d)", member), err)
		commitments = append(commitments, nativeFROSTCommitment{
			Identifier: frostIDByMember[member],
			Data:       commitmentData,
		})
	}

	signingPackage, err := engine.NewSigningPackage(message, commitments)
	skipFrostUnavailable(t, "new signing package", err)

	signatureShares := make([]nativeFROSTSignatureShare, 0, len(signingMembers))
	for _, member := range signingMembers {
		shareData, err := engine.InteractiveRound2(
			"real-cgo-session-1",
			attemptIDByMember[member],
			uint16(member),
			signingPackage,
		)
		skipFrostUnavailable(t, fmt.Sprintf("interactive round 2 (member %d)", member), err)
		signatureShares = append(signatureShares, nativeFROSTSignatureShare{
			Identifier: frostIDByMember[member],
			Data:       shareData,
		})
	}

	// A failed aggregate (other than an unavailable symbol) IS the verification:
	// real FROST rejects invalid shares or a key/package mismatch here, so a
	// non-error result is the proof the interactive round produced a valid
	// threshold signature.
	signatureBytes, err := engine.InteractiveAggregate(
		"real-cgo-session-1",
		attemptIDByMember[signingMembers[0]],
		signingPackage,
		signatureShares,
		nil,
	)
	skipFrostUnavailable(t, "interactive aggregate", err)

	// 3. The successful aggregate is a valid 64-byte BIP-340 signature (the engine
	// validated the shares and the aggregate internally). External schnorr.Verify
	// is omitted only for want of a keyGroup->group-pubkey accessor (see the file
	// comment); assert well-formedness.
	if len(signatureBytes) != 64 {
		t.Fatalf("unexpected interactive signature length: %d", len(signatureBytes))
	}
}

// skipFrostUnavailable turns an engine-call error into the right outcome: a missing
// FFI symbol (lib absent, or present but stale and missing a newer symbol) SKIPS
// the test naming the operation, while any other error is a real failure. nil is a
// no-op. Centralizing this makes every step of the e2e robust to an incomplete lib
// rather than only the first (RunDKG) call.
func skipFrostUnavailable(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Skipf(
			"linked tbtc-signer FFI symbol for %s unavailable (lib absent or stale; "+
				"rebuild libfrost_tbtc): %v",
			op, err,
		)
	}
	t.Fatalf("%s: %v", op, err)
}

// runRealCgoDKGKeyGroup runs a full real FROST DKG over the participants via
// RunDKG, which persists the result and returns the keyGroup the signing path
// resolves. It skips the test if the linked tbtc-signer FFI symbols are absent.
// Each participant carries a freshly generated operator public key (the DKG's
// per-participant identifying key), so the request is well-formed.
func runRealCgoDKGKeyGroup(
	t *testing.T,
	engine *buildTaggedTBTCSignerEngine,
	participantIDs []byte,
	threshold uint16,
) string {
	t.Helper()

	participants := make([]NativeTBTCSignerDKGParticipant, 0, len(participantIDs))
	for _, id := range participantIDs {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (participant %d): %v", id, err)
		}
		participants = append(participants, NativeTBTCSignerDKGParticipant{
			Identifier:   uint16(id),
			PublicKeyHex: hex.EncodeToString(operator.MarshalUncompressed(publicKey)),
		})
	}

	result, err := engine.RunDKG("real-cgo-dkg-session-1", participants, threshold)
	skipFrostUnavailable(t, "run DKG", err)
	if result.KeyGroup == "" {
		t.Fatal("RunDKG returned an empty key group")
	}
	return result.KeyGroup
}

// uint16sOf widens member ids to the uint16 participant list the engine's
// DeriveInteractiveAttemptContext expects.
func uint16sOf(members []byte) []uint16 {
	out := make([]uint16, 0, len(members))
	for _, member := range members {
		out = append(out, uint16(member))
	}
	return out
}
