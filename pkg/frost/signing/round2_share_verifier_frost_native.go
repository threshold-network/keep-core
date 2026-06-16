//go:build frost_native

package signing

import (
	"bytes"
	"fmt"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

var round2ShareVerifierLogger = log.Logger("keep-frost-signing-blame")

// Round2ShareVerifyingEngine is the slice of the native tbtc-signer engine the
// EngineRound2ShareVerifier needs: a single round-2 FROST share re-verification.
//
// It is defined at the consumer (interface segregation) so the verifier can be
// unit-tested with a one-method fake - no cgo and no full NativeTBTCSignerEngine.
// The registered NativeTBTCSignerEngine is a superset and satisfies it.
type Round2ShareVerifyingEngine interface {
	VerifySignatureShare(
		sessionID string,
		signingPackage []byte,
		signatureShare []byte,
		memberIdentifier uint16,
		taprootMerkleRoot *[32]byte,
	) (NativeShareVerificationVerdict, error)
}

// Round2ShareVerificationBinding pins an EngineRound2ShareVerifier to ONE
// attempt.
//
// The engine resolves the group key from SessionID and tweaks the verification
// by TaprootMerkleRoot; it cannot tell a valid-but-WRONG session/root from the
// right one, so a mis-bound verifier would make an HONEST share verify invalid
// and produce FALSE blame. The verifier therefore checks the retained package's
// own AttemptContextHash and TaprootMerkleRoot against this binding before it
// calls the engine, refusing (Indeterminate) on any mismatch.
//
// SessionID itself cannot be self-checked by the verifier (it cannot re-derive
// the attempt-context hash here), so the construction site MUST supply a
// SessionID consistent with AttemptContextHash - the RFC-21 attempt-context
// derivation includes the session, so an orchestrator that holds both can assert
// it. That is a hard construction-time contract.
type Round2ShareVerificationBinding struct {
	// SessionID names the engine DKG session whose group key the share is
	// verified against. Must be non-empty and consistent with AttemptContextHash.
	SessionID string
	// AttemptContextHash is the attempt this verifier adjudicates (32 bytes).
	AttemptContextHash [32]byte
	// TaprootMerkleRoot is the taproot script-tree root the signature is tweaked
	// by, or nil for a key-path spend. Copied at construction.
	TaprootMerkleRoot *[32]byte
}

// EngineRound2ShareVerifier is the engine-backed roast.Round2ShareVerifier: it
// re-verifies a retained round-2 signature share against an attempt's signing
// package using the Rust engine's pure FROST share verification (the frozen Q1
// crypto-only boundary - it never inspects operator-signed envelopes for blame;
// that is Round2Collector's job). Immutable after construction and safe for
// concurrent use.
type EngineRound2ShareVerifier struct {
	engine             Round2ShareVerifyingEngine
	sessionID          string
	attemptContextHash [32]byte
	taprootMerkleRoot  *[32]byte
}

var _ roast.Round2ShareVerifier = (*EngineRound2ShareVerifier)(nil)

// NewEngineRound2ShareVerifier binds an engine-backed verifier to one attempt.
// It errors on a nil engine or an empty SessionID, and copies the binding's
// taproot root so the verifier is immutable and cannot be mutated through the
// caller's pointer.
func NewEngineRound2ShareVerifier(
	engine Round2ShareVerifyingEngine,
	binding Round2ShareVerificationBinding,
) (*EngineRound2ShareVerifier, error) {
	if engine == nil {
		return nil, fmt.Errorf(
			"roast: EngineRound2ShareVerifier requires a non-nil engine",
		)
	}
	if binding.SessionID == "" {
		return nil, fmt.Errorf(
			"roast: EngineRound2ShareVerifier requires a non-empty session id",
		)
	}

	var taprootMerkleRoot *[32]byte
	if binding.TaprootMerkleRoot != nil {
		root := *binding.TaprootMerkleRoot
		taprootMerkleRoot = &root
	}

	return &EngineRound2ShareVerifier{
		engine:             engine,
		sessionID:          binding.SessionID,
		attemptContextHash: binding.AttemptContextHash,
		taprootMerkleRoot:  taprootMerkleRoot,
	}, nil
}

// VerifyRetainedShare implements roast.Round2ShareVerifier. It unwraps the
// retained, collector-authenticated envelopes, confirms the package belongs to
// THIS verifier's bound attempt and root, then asks the engine to re-verify the
// member's inner FROST share. It fails closed against blame
// (ShareIndeterminate) on every not-the-member's-fault condition; only the
// engine's `invalid` verdict - the member's own share is mathematically invalid
// or undecodable, judged after the engine establishes session/DKG/group/package
// context - yields ShareInvalid.
func (v *EngineRound2ShareVerifier) VerifyRetainedShare(
	signingPackageEnvelope []byte,
	shareEnvelope []byte,
	submitter group.MemberIndex,
) roast.ShareVerificationResult {
	// A zero member index is never a valid FROST participant; do not blame.
	if submitter == 0 {
		v.logIndeterminate("submitter member index is zero", submitter, nil)
		return roast.ShareIndeterminate
	}

	// The retained package/share envelopes are collector-produced (authenticated,
	// then re-marshaled): the member controls only the inner FROST bytes, not the
	// envelope framing, so an unmarshal failure here is an internal inconsistency,
	// not member fault -> Indeterminate, and the engine is NOT called.
	var signingPackage roast.SigningPackage
	if err := signingPackage.Unmarshal(signingPackageEnvelope); err != nil {
		v.logIndeterminate("retained signing-package envelope did not unmarshal", submitter, err)
		return roast.ShareIndeterminate
	}

	// Refuse to adjudicate a package that is not THIS attempt's, or whose taproot
	// root differs from the bound root: a mis-bound or cross-attempt verifier must
	// never turn an honest share into false blame. Fail closed (Indeterminate).
	if !bytes.Equal(signingPackage.AttemptContextHash, v.attemptContextHash[:]) {
		v.logIndeterminate("retained package attempt-context hash does not match the bound attempt", submitter, nil)
		return roast.ShareIndeterminate
	}
	if !v.taprootMerkleRootMatches(signingPackage.TaprootMerkleRoot) {
		v.logIndeterminate("retained package taproot root does not match the bound root", submitter, nil)
		return roast.ShareIndeterminate
	}

	var shareSubmission roast.ShareSubmission
	if err := shareSubmission.Unmarshal(shareEnvelope); err != nil {
		v.logIndeterminate("retained share-submission envelope did not unmarshal", submitter, err)
		return roast.ShareIndeterminate
	}

	// The retained share envelope must actually belong to the member being
	// adjudicated: if its own SubmitterID disagrees with submitter, that is a
	// caller inconsistency, not member fault. Never verify one member's identity
	// against another member's share bytes (that would manufacture false blame).
	if shareSubmission.SubmitterID() != submitter {
		v.logIndeterminate("retained share submitter id does not match the adjudicated member", submitter, nil)
		return roast.ShareIndeterminate
	}

	// submitter (group.MemberIndex == uint8) widens to the engine's uint16
	// losslessly. The engine classifies the inner FROST bytes: undecodable or
	// mathematically invalid member bytes become `invalid` only AFTER it
	// establishes session/DKG/group/package context, so passing the raw inner
	// bytes through is what makes a garbage-share submitter blamable.
	verdict, err := v.engine.VerifySignatureShare(
		v.sessionID,
		signingPackage.SigningPackageBytes,
		shareSubmission.SignatureShare,
		uint16(submitter),
		v.taprootMerkleRoot,
	)
	if err != nil {
		// FFI transport / engine-unavailable / decode failure: not member fault.
		v.logIndeterminate("engine verify_signature_share failed", submitter, err)
		return roast.ShareIndeterminate
	}

	switch verdict {
	case NativeShareVerdictValid:
		return roast.ShareValid
	case NativeShareVerdictInvalid:
		return roast.ShareInvalid
	case NativeShareVerdictIndeterminate:
		// Not the member's fault (per the engine's own tri-state); stay silent -
		// this is an expected, benign outcome, not a system failure.
		return roast.ShareIndeterminate
	default:
		// An unrecognized verdict must never read as blame.
		v.logIndeterminate("engine returned an unrecognized verdict", submitter, nil)
		return roast.ShareIndeterminate
	}
}

// taprootMerkleRootMatches reports whether the retained package's taproot root
// equals the bound root, honoring key-path (nil bound root <-> empty package
// root) semantics.
func (v *EngineRound2ShareVerifier) taprootMerkleRootMatches(packageRoot []byte) bool {
	if v.taprootMerkleRoot == nil {
		// Bound to a key-path spend: the package must also carry no root.
		return len(packageRoot) == 0
	}
	return bytes.Equal(packageRoot, v.taprootMerkleRoot[:])
}

// logIndeterminate records a fail-closed Indeterminate for diagnostics so a
// system failure (FFI/transport, mis-binding, malformed retained bytes) is
// distinguishable from a benign engine `indeterminate`. It logs the member,
// session, and reason (and the error, when any), but NEVER the raw package or
// share bytes.
func (v *EngineRound2ShareVerifier) logIndeterminate(
	reason string,
	submitter group.MemberIndex,
	err error,
) {
	if err != nil {
		round2ShareVerifierLogger.Warnf(
			"round-2 share verification indeterminate for member [%d] on session [%s]: %s: [%v]",
			submitter, v.sessionID, reason, err,
		)
		return
	}
	round2ShareVerifierLogger.Warnf(
		"round-2 share verification indeterminate for member [%d] on session [%s]: %s",
		submitter, v.sessionID, reason,
	)
}
