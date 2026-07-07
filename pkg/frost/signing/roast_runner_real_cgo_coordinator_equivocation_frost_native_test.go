//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test closes the fourth real-crypto-under-failure gap: a COORDINATOR
// EQUIVOCATION - the elected coordinator distributing two VALID, coordinator-
// signed signing packages with the same attempt context but different bytes -
// bypasses the f+1 accuser gate and forces INSTANT PERMANENT exclusion of the
// coordinator (verifiedCoordinatorEquivocations, next_attempt.go). Two authentic
// bodies signed by the coordinator's own operator key are unforgeable proof, so
// a single honest observer's evidence suffices.
//
// The "real crypto" that matters here is the Go-side operator-key ECDSA
// signature over each package body (pkg/chain), NOT FROST: the equivocation
// adjudication is a pure Go-side operation, so no FROST signing rounds run. The
// existing policy-level test (next_attempt_coordinator_equivocation_test.go)
// exercises this with a SHA-256 fakeVerifier; this test uses a REAL secp256k1
// operator-key Signer/Verifier, so the two packages must genuinely authenticate
// under the same verifier the coordinator instance carries. A non-vacuous
// negative control (a package with a corrupted signature is rejected by the real
// verifier and does NOT trigger exclusion) proves the verifier is genuinely
// cryptographic and the positive exclusion is caused by two AUTHENTIC distinct
// bodies, not a stub that accepts anything.
//
// A real DKG is run (as in the sibling real-cgo tests) only to produce an
// authentic key group / seed; it is not load-bearing for the equivocation logic.

// operatorKeyRoastSigner is a real roast.Signer backed by one member's
// secp256k1 operator private key (via local_v1's chain.Signing). Unlike the
// fixedTestSigner used elsewhere, its signatures actually verify.
type operatorKeyRoastSigner struct{ signing chain.Signing }

func (s operatorKeyRoastSigner) Sign(payload []byte) ([]byte, error) {
	return s.signing.Sign(payload)
}

// operatorKeyRoastVerifier is a real roast.SignatureVerifier: it verifies a
// signature attributed to a member against that member's uncompressed operator
// public key. Any local_v1 signer serves as the verification engine because
// VerifyWithPublicKey verifies against the SUPPLIED public key.
type operatorKeyRoastVerifier struct {
	signing    chain.Signing
	publicKeys map[group.MemberIndex][]byte // uncompressed operator pubkey bytes
}

func (v operatorKeyRoastVerifier) Verify(payload, signature []byte, signer group.MemberIndex) error {
	pub, ok := v.publicKeys[signer]
	if !ok {
		return fmt.Errorf("%w: no operator public key for member %d", roast.ErrSignatureInvalid, signer)
	}
	valid, err := v.signing.VerifyWithPublicKey(payload, signature, pub)
	if err != nil {
		return fmt.Errorf("%w: member %d: %s", roast.ErrSignatureInvalid, signer, err.Error())
	}
	if !valid {
		return fmt.Errorf("%w: member %d", roast.ErrSignatureInvalid, signer)
	}
	return nil
}

func TestRealCgoInteractiveSigning_CoordinatorEquivocationForcesInstantPermanentExclusion(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}
	sessionID := fmt.Sprintf("real-cgo-coord-equivocation-%d", realCgoSessionSeq.Add(1))

	const n = 3
	const threshold uint16 = 2
	participantIDs := []byte{1, 2, 3}
	included := []group.MemberIndex{1, 2, 3}

	// Authentic key group + seed (family consistency; not load-bearing here).
	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)
	keyGroupSeed := []byte(keyGroup)

	var messageDigest [attempt.MessageDigestLength]byte
	for i := range messageDigest {
		messageDigest[i] = 0x42
	}
	attempt1Ctx, err := attempt.NewAttemptContext(
		sessionID, keyGroup, keyGroupSeed, messageDigest, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt 1 context: %v", err)
	}

	// Real per-member operator keys + a real Signer/Verifier over all members.
	privKeys := make(map[group.MemberIndex]*operator.PrivateKey, n)
	pubKeyBytes := make(map[group.MemberIndex][]byte, n)
	for _, m := range included {
		priv, pub, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", m, err)
		}
		privKeys[m] = priv
		pubKeyBytes[m] = operator.MarshalUncompressed(pub)
	}
	verifier := operatorKeyRoastVerifier{
		signing:    local_v1.NewSigner(privKeys[included[0]]), // engine only; verifies against supplied pubkey
		publicKeys: pubKeyBytes,
	}

	// Resolve attempt 1's deterministic elected coordinator via a probe binding.
	probeSigner := operatorKeyRoastSigner{signing: local_v1.NewSigner(privKeys[included[0]])}
	probeCoord := roast.NewInMemoryCoordinatorWithSigning(included[0], probeSigner, verifier)
	probeHandle, err := probeCoord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	probeBinding, err := NewActiveRoastAttempt(probeCoord, probeHandle, attempt1Ctx, sessionID, nil, keyGroupSeed)
	if err != nil {
		t.Fatalf("probe binding: %v", err)
	}
	coordinator := probeBinding.ElectedCoordinator()

	nonCoord := make([]group.MemberIndex, 0, n-1)
	for _, m := range included {
		if m != coordinator {
			nonCoord = append(nonCoord, m)
		}
	}
	sort.Slice(nonCoord, func(i, j int) bool { return nonCoord[i] < nonCoord[j] })
	t.Logf("coordinator(equivocator)=%d observers=%v", coordinator, nonCoord)

	// The REAL coordinator package signer uses the coordinator's own operator key.
	coordSigner := operatorKeyRoastSigner{signing: local_v1.NewSigner(privKeys[coordinator])}

	prevHash := attempt1Ctx.Hash()
	newSignedPkg := func(root []byte) *roast.SigningPackage {
		p := &roast.SigningPackage{
			AttemptContextHash:  append([]byte(nil), prevHash[:]...),
			CoordinatorIDValue:  uint32(coordinator),
			SigningPackageBytes: []byte("frost-signing-package-bytes"),
			TaprootMerkleRoot:   root,
		}
		if err := roast.SignSigningPackage(coordSigner, p); err != nil {
			t.Fatalf("sign package: %v", err)
		}
		return p
	}
	// Two body-DISTINCT packages, SAME attempt context, SAME coordinator. The
	// bodies differ via the taproot root (0 vs 32 bytes) - signing the same body
	// twice would give a different ECDSA signature but the SAME BodyHash, which is
	// NOT an equivocation.
	pkgA := newSignedPkg(nil)                                                       // key-path spend
	pkgB := newSignedPkg(bytes.Repeat([]byte{0xab}, roast.TaprootMerkleRootLength)) // script-path spend

	// ---- FAULT REACHED: a genuine equivocation, not a synthetic pass. ----
	if err := roast.AuthenticateSigningPackage(verifier, pkgA, coordinator, prevHash[:]); err != nil {
		t.Fatalf("pkgA must authenticate under the real verifier: %v", err)
	}
	if err := roast.AuthenticateSigningPackage(verifier, pkgB, coordinator, prevHash[:]); err != nil {
		t.Fatalf("pkgB must authenticate under the real verifier: %v", err)
	}
	if pkgA.CoordinatorID() != coordinator || pkgB.CoordinatorID() != coordinator {
		t.Fatalf("both packages must name the elected coordinator %d", coordinator)
	}
	bhA, err := pkgA.BodyHash()
	if err != nil {
		t.Fatalf("pkgA body hash: %v", err)
	}
	bhB, err := pkgB.BodyHash()
	if err != nil {
		t.Fatalf("pkgB body hash: %v", err)
	}
	if bhA == bhB {
		t.Fatalf("packages must be byte-distinct to be an equivocation; both hashed to %x", bhA)
	}
	if !bytes.Equal(pkgA.AttemptContextHash, pkgB.AttemptContextHash) ||
		!bytes.Equal(pkgA.AttemptContextHash, prevHash[:]) {
		t.Fatalf("both packages must bind the same live attempt context")
	}
	if len(pkgA.CoordinatorSignature) == 0 || len(pkgB.CoordinatorSignature) == 0 {
		t.Fatalf("both packages must carry a real coordinator signature")
	}

	proofA, err := pkgA.Marshal()
	if err != nil {
		t.Fatalf("marshal pkgA: %v", err)
	}
	proofB, err := pkgB.Marshal()
	if err != nil {
		t.Fatalf("marshal pkgB: %v", err)
	}

	// Split the two proofs across two honest observers (the targeted case: no
	// single observer saw both). All live members are senders so nobody is
	// silence-parked; excluding the coordinator leaves threshold=2 members
	// (feasible for n=3, t=2). Every snapshot carries NO reject/conflict/overflow
	// evidence, so the ONLY mechanism that can exclude the coordinator is the
	// equivocation path (not an f+1 tally).
	snaps := []roast.LocalEvidenceSnapshot{
		{SenderIDValue: uint32(coordinator), AttemptContextHash: append([]byte(nil), prevHash[:]...)},
		{SenderIDValue: uint32(nonCoord[0]), AttemptContextHash: append([]byte(nil), prevHash[:]...), CoordinatorPackageProofs: [][]byte{proofA}},
		{SenderIDValue: uint32(nonCoord[1]), AttemptContextHash: append([]byte(nil), prevHash[:]...), CoordinatorPackageProofs: [][]byte{proofB}},
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].SenderIDValue < snaps[j].SenderIDValue })
	for i := range snaps {
		if len(snaps[i].Rejects) != 0 || len(snaps[i].Conflicts) != 0 || len(snaps[i].Overflows) != 0 {
			t.Fatalf("snapshot %d must carry no accusation entries (pin causation to equivocation)", i)
		}
	}
	bundle := &roast.TransitionMessage{
		AttemptContextHash: append([]byte(nil), prevHash[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             snaps,
	}

	// Drive the REAL policy through the REAL verifier.
	coord := roast.NewInMemoryCoordinatorWithSigning(coordinator, coordSigner, verifier)
	handle, err := coord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	next, err := coord.NextAttempt(handle, bundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt: %v", err)
	}

	// ---- Assert INSTANT PERMANENT exclusion of the coordinator. ----
	if !containsMember(next.ExcludedSet, coordinator) {
		t.Fatalf("equivocating coordinator %d must be permanently excluded; excluded=%v", coordinator, next.ExcludedSet)
	}
	if containsMember(next.IncludedSet, coordinator) {
		t.Fatalf("excluded coordinator %d must drop from the next included set %v", coordinator, next.IncludedSet)
	}
	if containsMember(next.TransientlyParked, coordinator) {
		t.Fatalf("coordinator %d must be excluded, not merely parked; parked=%v", coordinator, next.TransientlyParked)
	}
	wantIncluded := append([]group.MemberIndex{}, nonCoord...)
	gotIncluded := append([]group.MemberIndex{}, next.IncludedSet...)
	sort.Slice(gotIncluded, func(i, j int) bool { return gotIncluded[i] < gotIncluded[j] })
	if !memberSlicesEqualLocal(gotIncluded, wantIncluded) {
		t.Fatalf("attempt 2 included = %v, want %v", gotIncluded, wantIncluded)
	}
	t.Logf("coordinator %d instantly + permanently excluded on two authentic distinct packages", coordinator)

	// ---- NON-VACUOUS negative control: same verifier, corrupted signature. ----
	// A single AUTHENTIC package plus a package whose real signature is corrupted
	// (rejected by the real verifier) is only ONE distinct authentic body -> no
	// equivocation, no exclusion. This proves the verifier is genuinely
	// cryptographic (not NoOp) and the positive exclusion above required TWO
	// authentic bodies.
	pkgBad := newSignedPkg(bytes.Repeat([]byte{0xcd}, roast.TaprootMerkleRootLength)) // third distinct body
	pkgBad.CoordinatorSignature[0] ^= 0x01                                            // break the real signature
	if err := roast.AuthenticateSigningPackage(verifier, pkgBad, coordinator, prevHash[:]); !errors.Is(err, roast.ErrSignatureInvalid) {
		t.Fatalf("real verifier must reject a corrupted signature (proves it is not NoOp); got %v", err)
	}
	proofBad, err := pkgBad.Marshal()
	if err != nil {
		t.Fatalf("marshal pkgBad: %v", err)
	}
	negSnaps := []roast.LocalEvidenceSnapshot{
		{SenderIDValue: uint32(coordinator), AttemptContextHash: append([]byte(nil), prevHash[:]...)},
		{SenderIDValue: uint32(nonCoord[0]), AttemptContextHash: append([]byte(nil), prevHash[:]...), CoordinatorPackageProofs: [][]byte{proofA}},
		{SenderIDValue: uint32(nonCoord[1]), AttemptContextHash: append([]byte(nil), prevHash[:]...), CoordinatorPackageProofs: [][]byte{proofBad}},
	}
	sort.Slice(negSnaps, func(i, j int) bool { return negSnaps[i].SenderIDValue < negSnaps[j].SenderIDValue })
	negBundle := &roast.TransitionMessage{
		AttemptContextHash: append([]byte(nil), prevHash[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             negSnaps,
	}
	handle2, err := coord.BeginAttempt(attempt1Ctx)
	if err != nil {
		t.Fatalf("begin attempt (negative control): %v", err)
	}
	nextNeg, err := coord.NextAttempt(handle2, negBundle, uint(threshold), keyGroupSeed)
	if err != nil {
		t.Fatalf("NextAttempt (negative control): %v", err)
	}
	if containsMember(nextNeg.ExcludedSet, coordinator) {
		t.Fatalf("only one AUTHENTIC distinct body: coordinator %d must NOT be excluded; excluded=%v", coordinator, nextNeg.ExcludedSet)
	}
	t.Logf("negative control: one authentic + one corrupted package did NOT exclude the coordinator (verifier is real)")
}
