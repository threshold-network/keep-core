//go:build frost_native

package signing

// interactiveSigningEngine is the slice of the native tbtc-signer engine the
// interactive runner drives for one attempt. It is defined at the runner
// boundary (interface segregation) so the runner is exercised with a
// programmable fake under frost_native alone - no cgo, deterministic - per the
// design consult. The cgo-backed buildTaggedTBTCSignerEngine satisfies it; that
// satisfaction is asserted in the cgo wiring layer, not here, so this file does
// not pull in frost_tbtc_signer && cgo.
//
// Secret nonces never cross this boundary: the engine generates, holds, and
// zeroizes them keyed by (session_id, attempt_id); the runner exchanges only
// the public commitments, the coordinator's signing package, and the signature
// shares returned here.
type interactiveSigningEngine interface {
	// InteractiveSessionOpen opens (or idempotently re-opens) the attempt and
	// returns the engine's canonical attempt id.
	InteractiveSessionOpen(
		sessionID string,
		memberIdentifier uint16,
		message []byte,
		keyGroup string,
		threshold uint16,
		taprootMerkleRoot *[32]byte,
		attemptContext NativeInteractiveAttemptContext,
	) (*NativeInteractiveSessionOpenResult, error)

	// InteractiveRound1 returns this member's public round-1 commitments.
	InteractiveRound1(
		sessionID string,
		attemptID string,
		memberIdentifier uint16,
	) ([]byte, error)

	// NewSigningPackage builds the FROST signing package from the responsive
	// subset's commitments (the elected coordinator calls this).
	NewSigningPackage(
		message []byte,
		commitments []nativeFROSTCommitment,
	) ([]byte, error)

	// InteractiveRound2 consumes this member's nonces against the coordinator's
	// signing package and returns its signature share.
	InteractiveRound2(
		sessionID string,
		attemptID string,
		memberIdentifier uint16,
		signingPackage []byte,
	) ([]byte, error)

	// InteractiveAggregate aggregates the collected shares into the BIP-340
	// signature, or fails (with an InteractiveAggregateShareVerificationError
	// carrying candidate culprits) when a share does not verify.
	InteractiveAggregate(
		sessionID string,
		attemptID string,
		signingPackage []byte,
		signatureShares []nativeFROSTSignatureShare,
		taprootMerkleRoot *[32]byte,
	) ([]byte, error)

	// InteractiveSessionAbort tells the engine to drop the attempt's held
	// secret nonces/session state. The runner defers it for early exits so an
	// attempt abandoned before aggregation does not leave nonce material
	// resident.
	InteractiveSessionAbort(
		sessionID string,
		attemptID *string,
	) (*NativeInteractiveSessionAbortResult, error)
}
