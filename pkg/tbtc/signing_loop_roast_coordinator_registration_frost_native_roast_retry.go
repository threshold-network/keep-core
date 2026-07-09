//go:build frost_native && frost_roast_retry

package tbtc

import (
	"fmt"

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
// The registry is scoped by the wallet's FROST key-group handle, so a node
// controlling more than one FROST wallet does not collide on the 1..N seat indices
// each wallet's group reuses: the interactive drive's lookup is keyed by the
// attempt's KeyGroupID, and this registration keys by the same handle derived from
// the wallet's signer material (see registrationKeyGroupIDForSigner).
func registerRoastRetryCoordinatorForSeats(n *node, signers []*signer) {
	if len(signers) == 0 {
		return
	}

	// Derive the wallet's FROST key-group handle once (all seats of a wallet share
	// it). Without it the registration cannot be scoped to match the drive's
	// wallet-scoped lookup, so leave the wallet on the legacy path rather than
	// register a coordinator the drive can never find.
	keyGroupID, err := registrationKeyGroupIDForSigner(signers[0])
	if err != nil {
		logger.Warnf(
			"skipping ROAST-retry coordinator registration: cannot derive FROST "+
				"key-group id for wallet: [%v]",
			err,
		)
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
				KeyGroupID:  keyGroupID,
			},
		)
	}
}

// registrationKeyGroupIDForSigner returns the wallet's FROST key-group handle from
// a seat's signer material — the same string BuildAttemptContextFromRequest stores
// as AttemptContext.KeyGroupID, so registration and the interactive drive's lookup
// agree on the registry key. Errors if the seat carries non-native (e.g. legacy
// tECDSA) material, which has no FROST key-group handle.
func registrationKeyGroupIDForSigner(s *signer) (string, error) {
	material, ok := s.signingMaterial().(*signing.NativeSignerMaterial)
	if !ok {
		return "", fmt.Errorf(
			"signer material is not native FROST signer material (got %T)",
			s.signingMaterial(),
		)
	}
	return signing.KeyGroupIDFromSignerMaterial(material)
}
