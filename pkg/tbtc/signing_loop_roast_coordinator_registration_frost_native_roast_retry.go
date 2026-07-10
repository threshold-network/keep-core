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

// Sign returns a self-describing envelope carrying this operator's public key
// alongside the raw operator-key signature over the canonical ROAST payload. The
// coordinator treats the bytes as opaque; memberKeyedRoastSignatureVerifier is the
// only component that interprets them, using the embedded public key to bind the
// signature to a member seat (the node knows seats by operator ADDRESS, not public
// key, so the key must travel with the signature). See
// roast_operator_signature_verifier_frost_native_roast_retry.go.
func (s operatorKeyRoastSigner) Sign(payload []byte) ([]byte, error) {
	rawSignature, err := s.signing.Sign(payload)
	if err != nil {
		return nil, err
	}
	return encodeRoastSignatureEnvelope(s.signing.PublicKey(), rawSignature)
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
// member. The operator-key Signer and the member-keyed Verifier are shared across
// seats: one operator identity signs, and verification is a pure function of the
// wallet's seat -> operator-address list plus the chain verification primitives.
//
// The Verifier authenticates the retry/blame layer: it decodes the signer's
// public-key envelope, binds it to the claimed seat via
// PublicKeyBytesToAddress == signingGroupOperators[member-1], and checks the
// signature with chain.Signing().VerifyWithPublicKey (see
// roast_operator_signature_verifier_frost_native_roast_retry.go). The happy path
// never verifies evidence signatures (BeginAttempt -> engine.InteractiveAggregate
// -> MarkSucceeded touches neither Signer nor Verifier), so this switch from the
// former no-op verifier is inert until an actual ROAST retry occurs — but it must
// be adopted by the whole operator set together, because an upgraded verifier
// rejects a peer that still emits bare (non-enveloped) signatures.
//
// KNOWN LIMITATION (single-wallet only): the retry registry is keyed by member
// index alone (RegisterRoastRetryCoordinatorForMember), not by wallet+member. A
// node controlling more than one FROST wallet would register overlapping seat
// indices (each wallet's group is 1..N), and the later registration replaces the
// earlier — so this wiring is correct only while a node controls a single FROST
// wallet, which matches the current experimental deployment. Making the registry
// wallet-scoped is a prerequisite for multi-wallet FROST.
func registerRoastRetryCoordinatorForSeats(n *node, signers []*signer) {
	if len(signers) == 0 {
		return
	}

	operatorSigner := operatorKeyRoastSigner{signing: n.chain.Signing()}
	// All signers belong to one wallet, so any signer's seat -> operator-address
	// list is the whole group's. Copy it so the verifier can never be affected by
	// later mutation of the wallet's slice.
	operatorAddresses := append(
		[]chain.Address(nil),
		signers[0].wallet.signingGroupOperators...,
	)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           n.chain.Signing(),
		operatorAddresses: operatorAddresses,
	}

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
