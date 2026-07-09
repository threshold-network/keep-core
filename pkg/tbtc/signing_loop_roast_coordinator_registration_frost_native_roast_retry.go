//go:build frost_native && frost_roast_retry

package tbtc

import (
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/signing"
)

// operatorKeyRoastSigner adapts the node's operator chain signer to the
// roast.Signer interface. It signs the RFC-21 ROAST coordination evidence
// (transition bundles and local snapshots) with the same operator key the node
// uses for FROST DKG-result signing (see frost_dkg_result_signing.go). It does
// NOT sign the FROST threshold signature itself — that is produced by the
// interactive engine; this only authenticates the retry/blame layer.
type operatorKeyRoastSigner struct {
	signing chain.Signing
}

// Sign returns the operator-key signature over the canonical ROAST payload. The
// coordinator treats the bytes as opaque; the SignatureVerifier interprets them.
func (s operatorKeyRoastSigner) Sign(payload []byte) ([]byte, error) {
	return s.signing.Sign(payload)
}

// registerRoastRetryCoordinatorForSeats registers a ROAST-retry coordinator for
// every local seat of a wallet's signing group so the interactive FROST signing
// path (driveInteractiveRoastSigningIfEnabled) can run on a live node. Without
// this, the executor's BeginOrchestrationForSession finds no coordinator, the
// interactive drive never starts, and interactive-only signing fails closed.
//
// Each seat gets its OWN coordinator bound to that seat's member index — the
// registrar enforces deps.SelfMember == member and drops any mismatch, so a
// mis-bound seat safely stays on legacy rather than aggregating as the wrong
// member. The operator-key Signer is shared across seats (one operator identity).
//
// The Verifier is currently a no-op. On the happy path the coordinator never
// verifies evidence signatures (BeginAttempt -> engine.InteractiveAggregate ->
// MarkSucceeded touches neither Signer nor Verifier), so a valid BIP-340
// signature is produced without it. A real member-keyed verifier
// (member index -> operator public key -> chain.Signing().VerifyWithPublicKey)
// is a required follow-up before the retry/blame path's integrity can be
// trusted, and it must be flipped from no-op to real atomically across the
// operator set so the retry-path signature format stays consistent.
//
// KNOWN LIMITATION (single-wallet only): the retry registry is keyed by member
// index alone (RegisterRoastRetryCoordinatorForMember), not by wallet+member. A
// node controlling more than one FROST wallet would register overlapping seat
// indices (each wallet's group is 1..N), and the later registration replaces the
// earlier — so this wiring is correct only while a node controls a single FROST
// wallet, which matches the current experimental deployment. Making the registry
// wallet-scoped is a prerequisite for multi-wallet FROST and is tracked with the
// verifier follow-up.
func registerRoastRetryCoordinatorForSeats(n *node, signers []*signer) {
	operatorSigner := operatorKeyRoastSigner{signing: n.chain.Signing()}
	verifier := roast.NoOpSignatureVerifier()

	for _, s := range signers {
		member := s.signingGroupMemberIndex
		coordinator := roast.NewInMemoryCoordinatorWithSigning(
			member,
			operatorSigner,
			verifier,
		)
		signing.RegisterRoastRetryCoordinatorForMember(
			member,
			signing.RoastRetryDeps{
				Coordinator: coordinator,
				Signer:      operatorSigner,
				Verifier:    verifier,
				SelfMember:  uint32(member),
			},
		)
	}
}
