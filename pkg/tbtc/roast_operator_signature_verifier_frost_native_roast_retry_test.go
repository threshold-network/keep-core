//go:build frost_native && frost_roast_retry

package tbtc

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// newTestOperatorSigning returns a real operator-key chain.Signing (secp256k1,
// genuine sign/verify) so these tests exercise the actual crypto path the
// production verifier relies on, not a stub.
func newTestOperatorSigning(t *testing.T) chain.Signing {
	t.Helper()
	privateKey, _, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatalf("generate operator key pair: %v", err)
	}
	return local_v1.NewSigner(privateKey)
}

// buildSeatSignings returns n distinct operator signings and the seat ->
// operator-address list they induce (address[i] is the operator at member i+1).
func buildSeatSignings(t *testing.T, n int) ([]chain.Signing, []chain.Address) {
	t.Helper()
	signings := make([]chain.Signing, n)
	addresses := make([]chain.Address, n)
	for i := 0; i < n; i++ {
		signings[i] = newTestOperatorSigning(t)
		addresses[i] = signings[i].Address()
	}
	return signings, addresses
}

func TestRoastSignatureEnvelope_RoundTrip(t *testing.T) {
	pub := []byte{0x04, 0x11, 0x22, 0x33}
	sig := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee}

	envelope, err := encodeRoastSignatureEnvelope(pub, sig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	gotPub, gotSig, err := decodeRoastSignatureEnvelope(envelope)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(gotPub, pub) {
		t.Errorf("public key round-trip mismatch: got %x want %x", gotPub, pub)
	}
	if !bytes.Equal(gotSig, sig) {
		t.Errorf("signature round-trip mismatch: got %x want %x", gotSig, sig)
	}
}

func TestRoastSignatureEnvelope_EncodeRejectsEmptyParts(t *testing.T) {
	if _, err := encodeRoastSignatureEnvelope(nil, []byte{0x01}); err == nil {
		t.Error("expected error for empty public key")
	}
	if _, err := encodeRoastSignatureEnvelope([]byte{0x01}, nil); err == nil {
		t.Error("expected error for empty signature")
	}
}

func TestRoastSignatureEnvelope_DecodeRejectsMalformed(t *testing.T) {
	valid, err := encodeRoastSignatureEnvelope([]byte{0x04, 0x11}, []byte{0xaa})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	cases := map[string][]byte{
		"empty":                  {},
		"header only":            {roastSignatureEnvelopeVersion, 0x00},
		"unsupported version":    append([]byte{0x02}, valid[1:]...),
		"pubkey len overruns":    {roastSignatureEnvelopeVersion, 0xFF, 0xFF, 0x04},
		"no signature remainder": {roastSignatureEnvelopeVersion, 0x00, 0x02, 0x04, 0x11},
		"zero pubkey len":        {roastSignatureEnvelopeVersion, 0x00, 0x00, 0xaa},
	}
	for name, envelope := range cases {
		if _, _, err := decodeRoastSignatureEnvelope(envelope); err == nil {
			t.Errorf("%s: expected decode error, got nil", name)
		}
	}
}

func TestMemberKeyedVerifier_AcceptsCorrectSeat(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0], // verification uses pure methods; any signing works
		operatorAddresses: addresses,
	}

	payload := []byte("roast evidence snapshot bytes")
	for seat := 1; seat <= 3; seat++ {
		signer := operatorKeyRoastSigner{signing: signings[seat-1]}
		envelope, err := signer.Sign(payload)
		if err != nil {
			t.Fatalf("seat %d sign: %v", seat, err)
		}
		if err := verifier.Verify(payload, envelope, group.MemberIndex(seat)); err != nil {
			t.Errorf("seat %d: expected valid, got %v", seat, err)
		}
	}
}

func TestMemberKeyedVerifier_RejectsWrongSeatAttribution(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	payload := []byte("roast evidence snapshot bytes")
	// A cryptographically VALID signature by seat 2's operator, but attributed to
	// seat 1. The binding check (pubkey -> address != seat 1's operator) must
	// reject it even though the signature itself verifies.
	envelope, err := operatorKeyRoastSigner{signing: signings[1]}.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := verifier.Verify(payload, envelope, 1); err == nil {
		t.Fatal("expected rejection of seat-2 signature attributed to seat 1")
	}
	// Sanity: the same signature is accepted for its true seat.
	if err := verifier.Verify(payload, envelope, 2); err != nil {
		t.Fatalf("expected acceptance for true seat 2, got %v", err)
	}
}

func TestMemberKeyedVerifier_RejectsOutsiderKey(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	payload := []byte("roast evidence snapshot bytes")
	// An operator NOT seated in the group signs and claims seat 2.
	outsider := newTestOperatorSigning(t)
	envelope, err := operatorKeyRoastSigner{signing: outsider}.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := verifier.Verify(payload, envelope, 2); err == nil {
		t.Fatal("expected rejection of an out-of-group operator key")
	}
}

func TestMemberKeyedVerifier_RejectsTamperedPayload(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	envelope, err := operatorKeyRoastSigner{signing: signings[1]}.Sign([]byte("original payload"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Correct seat, correct key, but the payload the verifier checks differs.
	if err := verifier.Verify([]byte("tampered payload"), envelope, 2); err == nil {
		t.Fatal("expected rejection when the verified payload differs from the signed one")
	}
}

func TestMemberKeyedVerifier_RejectsOutOfRangeMember(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	envelope, err := operatorKeyRoastSigner{signing: signings[0]}.Sign([]byte("payload"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	for _, seat := range []group.MemberIndex{0, 4, 255} {
		if err := verifier.Verify([]byte("payload"), envelope, seat); err == nil {
			t.Errorf("seat %d: expected out-of-range rejection", seat)
		}
	}
}

func TestMemberKeyedVerifier_RejectsBareSignature(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	payload := []byte("payload")
	// A pre-envelope (bare) operator signature — the format a not-yet-upgraded peer
	// would emit. It must be rejected, not misparsed.
	bare, err := signings[1].Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := verifier.Verify(payload, bare, 2); err == nil {
		t.Fatal("expected rejection of a bare (non-enveloped) signature")
	}
}

// TestMemberKeyedVerifier_IntegratesWithCoordinatorRecordEvidence drives the
// production Signer and Verifier through a REAL roast coordinator's RecordEvidence
// — the exact retry/blame seam that verifies a peer's evidence snapshot — proving
// the switch from the no-op verifier authenticates evidence end to end.
func TestMemberKeyedVerifier_IntegratesWithCoordinatorRecordEvidence(t *testing.T) {
	signings, addresses := buildSeatSignings(t, 3)
	verifier := memberKeyedRoastSignatureVerifier{
		signing:           signings[0],
		operatorAddresses: addresses,
	}

	// A real in-memory coordinator (self member 1) using the production Signer and
	// the new member-keyed Verifier.
	coord := roast.NewInMemoryCoordinatorWithSigning(
		1,
		operatorKeyRoastSigner{signing: signings[0]},
		verifier,
	)

	included := []group.MemberIndex{1, 2, 3}
	ctx, err := attempt.NewAttemptContext(
		"session-1", "key-group-1", []byte{0x01, 0x02},
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}
	handle, err := coord.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	attemptHash := ctx.Hash()

	signSnapshot := func(seat group.MemberIndex, signingIdx int) *roast.LocalEvidenceSnapshot {
		t.Helper()
		snapshot := roast.NewLocalEvidenceSnapshot(seat, attemptHash, attempt.Evidence{})
		payload, err := snapshot.SignableBytes()
		if err != nil {
			t.Fatalf("snapshot signable bytes: %v", err)
		}
		sig, err := operatorKeyRoastSigner{signing: signings[signingIdx]}.Sign(payload)
		if err != nil {
			t.Fatalf("sign snapshot: %v", err)
		}
		snapshot.OperatorSignature = sig
		return snapshot
	}

	// Seat 2 authors and signs its own snapshot with seat 2's operator key.
	if err := coord.RecordEvidence(handle, signSnapshot(2, 1)); err != nil {
		t.Fatalf("valid seat-2 snapshot rejected by coordinator: %v", err)
	}

	// A snapshot attributed to seat 3 but signed with seat 1's key. Seat 3 has no
	// prior record, so the only thing that can reject it is signature verification.
	err = coord.RecordEvidence(handle, signSnapshot(3, 0))
	if err == nil {
		t.Fatal("coordinator accepted a snapshot signed by the wrong operator")
	}
	if !errors.Is(err, roast.ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}
