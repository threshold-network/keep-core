//go:build frost_native

package signing

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

var errTestEngine = errors.New("engine boom")

// fakeShareVerifyingEngine is a one-method Round2ShareVerifyingEngine: it
// records the call (to assert the engine is/ isn't invoked and with what
// arguments) and returns a programmed verdict/error.
type fakeShareVerifyingEngine struct {
	verdict NativeShareVerificationVerdict
	err     error

	called              bool
	gotSessionID        string
	gotSigningPackage   []byte
	gotSignatureShare   []byte
	gotMemberIdentifier uint16
	gotTaprootRoot      *[32]byte
}

func (f *fakeShareVerifyingEngine) VerifySignatureShare(
	sessionID string,
	signingPackage []byte,
	signatureShare []byte,
	memberIdentifier uint16,
	taprootMerkleRoot *[32]byte,
) (NativeShareVerificationVerdict, error) {
	f.called = true
	f.gotSessionID = sessionID
	f.gotSigningPackage = signingPackage
	f.gotSignatureShare = signatureShare
	f.gotMemberIdentifier = memberIdentifier
	f.gotTaprootRoot = taprootMerkleRoot
	return f.verdict, f.err
}

// testSigningPackage builds a signing package and returns both its on-wire
// envelope and its body hash, so a share can commit to the SAME package the
// verifier is bound to (SigningPackageHash == BodyHash).
func testSigningPackage(
	t *testing.T,
	attemptContextHash [32]byte,
	taprootMerkleRoot []byte,
	frostSigningPackage []byte,
) (envelope []byte, bodyHash []byte) {
	t.Helper()
	pkg := &roast.SigningPackage{
		AttemptContextHash:   append([]byte(nil), attemptContextHash[:]...),
		CoordinatorIDValue:   1,
		SigningPackageBytes:  frostSigningPackage,
		TaprootMerkleRoot:    taprootMerkleRoot,
		CoordinatorSignature: []byte{0x01}, // dummy; the verifier never authenticates
	}
	envelope, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal test signing package: [%v]", err)
	}
	hash, err := pkg.BodyHash()
	if err != nil {
		t.Fatalf("cannot hash test signing package body: [%v]", err)
	}
	return envelope, hash[:]
}

func testShareSubmissionEnvelope(
	t *testing.T,
	attemptContextHash [32]byte,
	submitter uint32,
	signingPackageHash []byte,
	frostSignatureShare []byte,
) []byte {
	t.Helper()
	sub := &roast.ShareSubmission{
		AttemptContextHash: append([]byte(nil), attemptContextHash[:]...),
		SubmitterIDValue:   submitter,
		CoordinatorIDValue: 1,
		SigningPackageHash: signingPackageHash,
		SignatureShare:     frostSignatureShare,
		SubmitterSignature: []byte{0x01}, // dummy
	}
	envelope, err := sub.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal test share submission: [%v]", err)
	}
	return envelope
}

func TestNewEngineRound2ShareVerifier_RejectsNilEngine(t *testing.T) {
	_, err := NewEngineRound2ShareVerifier(nil, Round2ShareVerificationBinding{
		SessionID: "session-1",
	})
	if err == nil {
		t.Fatal("expected a nil engine to be rejected")
	}
}

func TestNewEngineRound2ShareVerifier_RejectsEmptySessionID(t *testing.T) {
	_, err := NewEngineRound2ShareVerifier(&fakeShareVerifyingEngine{}, Round2ShareVerificationBinding{
		SessionID: "",
	})
	if err == nil {
		t.Fatal("expected an empty session id to be rejected")
	}
}

// The happy path for each native verdict, with a bound taproot root, asserting
// both the mapping and that the engine receives the unwrapped inner bytes, the
// widened member id, the bound session id, and the bound root.
func TestEngineRound2ShareVerifier_VerdictMappingWithRoot(t *testing.T) {
	attempt := [32]byte{0xaa}
	root := [32]byte{0xbb}
	frostPackage := []byte{0xde, 0xad}
	frostShare := []byte{0xbe, 0xef}

	tests := map[string]struct {
		verdict  NativeShareVerificationVerdict
		expected roast.ShareVerificationResult
	}{
		"valid":         {NativeShareVerdictValid, roast.ShareValid},
		"invalid":       {NativeShareVerdictInvalid, roast.ShareInvalid},
		"indeterminate": {NativeShareVerdictIndeterminate, roast.ShareIndeterminate},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			engine := &fakeShareVerifyingEngine{verdict: test.verdict}
			verifier, err := NewEngineRound2ShareVerifier(engine, Round2ShareVerificationBinding{
				SessionID:          "session-1",
				AttemptContextHash: attempt,
				TaprootMerkleRoot:  &root,
			})
			if err != nil {
				t.Fatalf("unexpected constructor error: [%v]", err)
			}

			pkgEnv, pkgHash := testSigningPackage(t, attempt, root[:], frostPackage)
			shareEnv := testShareSubmissionEnvelope(t, attempt, 2, pkgHash, frostShare)

			result := verifier.VerifyRetainedShare(pkgEnv, shareEnv, 2)
			if result != test.expected {
				t.Fatalf("unexpected result\nexpected: [%v]\nactual:   [%v]", test.expected, result)
			}
			if !engine.called {
				t.Fatal("expected the engine to be called")
			}
			if engine.gotSessionID != "session-1" {
				t.Fatalf("engine got wrong session id: [%s]", engine.gotSessionID)
			}
			if !bytes.Equal(engine.gotSigningPackage, frostPackage) {
				t.Fatalf("engine got wrong inner signing package: [%x]", engine.gotSigningPackage)
			}
			if !bytes.Equal(engine.gotSignatureShare, frostShare) {
				t.Fatalf("engine got wrong inner signature share: [%x]", engine.gotSignatureShare)
			}
			if engine.gotMemberIdentifier != 2 {
				t.Fatalf("engine got wrong member id: [%d]", engine.gotMemberIdentifier)
			}
			if engine.gotTaprootRoot == nil || *engine.gotTaprootRoot != root {
				t.Fatalf("engine got wrong taproot root: [%v]", engine.gotTaprootRoot)
			}
		})
	}
}

// A key-path (nil root) verifier accepts a package carrying no root and passes a
// nil root to the engine.
func TestEngineRound2ShareVerifier_KeyPathNilRoot(t *testing.T) {
	attempt := [32]byte{0xaa}
	engine := &fakeShareVerifyingEngine{verdict: NativeShareVerdictValid}
	verifier, err := NewEngineRound2ShareVerifier(engine, Round2ShareVerificationBinding{
		SessionID:          "session-1",
		AttemptContextHash: attempt,
		TaprootMerkleRoot:  nil,
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}

	pkgEnv, pkgHash := testSigningPackage(t, attempt, nil, []byte{0xde, 0xad})
	shareEnv := testShareSubmissionEnvelope(t, attempt, 2, pkgHash, []byte{0xbe, 0xef})

	if result := verifier.VerifyRetainedShare(pkgEnv, shareEnv, 2); result != roast.ShareValid {
		t.Fatalf("expected ShareValid, got [%v]", result)
	}
	if engine.gotTaprootRoot != nil {
		t.Fatalf("expected a nil taproot root passed to the engine, got [%v]", engine.gotTaprootRoot)
	}
}

func TestEngineRound2ShareVerifier_EngineErrorIsIndeterminate(t *testing.T) {
	attempt := [32]byte{0xaa}
	engine := &fakeShareVerifyingEngine{
		verdict: NativeShareVerdictInvalid, // even a blame verdict must be ignored on error
		err:     errTestEngine,
	}
	verifier := mustVerifier(t, engine, "session-1", attempt, nil)

	pkgEnv, pkgHash := testSigningPackage(t, attempt, nil, []byte{0xde, 0xad})
	shareEnv := testShareSubmissionEnvelope(t, attempt, 2, pkgHash, []byte{0xbe, 0xef})

	if result := verifier.VerifyRetainedShare(pkgEnv, shareEnv, 2); result != roast.ShareIndeterminate {
		t.Fatalf("expected ShareIndeterminate on engine error, got [%v]", result)
	}
}

func TestEngineRound2ShareVerifier_FailClosedWithoutCallingEngine(t *testing.T) {
	attempt := [32]byte{0xaa}
	root := [32]byte{0xbb}
	otherAttempt := [32]byte{0xcc}
	otherRoot := [32]byte{0xdd}
	frostPackage := []byte{0xde, 0xad}
	frostShare := []byte{0xbe, 0xef}

	validPkgEnv, _ := testSigningPackage(t, attempt, root[:], frostPackage)
	otherAttemptPkgEnv, _ := testSigningPackage(t, otherAttempt, root[:], frostPackage)
	otherRootPkgEnv, _ := testSigningPackage(t, attempt, otherRoot[:], frostPackage)

	// A dummy package hash for the share in cases that fail closed BEFORE the
	// share-commits-to-package check is reached.
	dummyHash := make([]byte, 32)

	tests := map[string]struct {
		bindRoot  *[32]byte
		pkgEnv    []byte
		shareEnv  []byte
		submitter group.MemberIndex
	}{
		"submitter zero": {
			bindRoot:  &root,
			pkgEnv:    validPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, dummyHash, frostShare),
			submitter: 0,
		},
		"unparseable signing-package envelope": {
			bindRoot:  &root,
			pkgEnv:    []byte("not a signing package envelope"),
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, dummyHash, frostShare),
			submitter: 2,
		},
		"attempt-context mismatch": {
			bindRoot:  &root,
			pkgEnv:    otherAttemptPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, dummyHash, frostShare),
			submitter: 2,
		},
		"taproot root mismatch": {
			bindRoot:  &root,
			pkgEnv:    otherRootPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, dummyHash, frostShare),
			submitter: 2,
		},
		"key-path verifier rejects a rooted package": {
			bindRoot:  nil,
			pkgEnv:    validPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, dummyHash, frostShare),
			submitter: 2,
		},
		"unparseable share envelope": {
			bindRoot:  &root,
			pkgEnv:    validPkgEnv,
			shareEnv:  []byte("not a share submission envelope"),
			submitter: 2,
		},
		"share submitter id mismatch": {
			bindRoot:  &root,
			pkgEnv:    validPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 3, dummyHash, frostShare), // envelope says member 3
			submitter: 2,                                                                 // adjudicating member 2
		},
		"share commits to a different package": {
			bindRoot:  &root,
			pkgEnv:    validPkgEnv,
			shareEnv:  testShareSubmissionEnvelope(t, attempt, 2, bytes.Repeat([]byte{0xee}, 32), frostShare),
			submitter: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// A valid verdict would map to ShareValid IF the engine were called -
			// so a ShareIndeterminate result also proves the engine was bypassed.
			engine := &fakeShareVerifyingEngine{verdict: NativeShareVerdictValid}
			verifier := mustVerifier(t, engine, "session-1", attempt, test.bindRoot)

			result := verifier.VerifyRetainedShare(test.pkgEnv, test.shareEnv, test.submitter)
			if result != roast.ShareIndeterminate {
				t.Fatalf("expected ShareIndeterminate, got [%v]", result)
			}
			if engine.called {
				t.Fatal("engine must not be called when context fails closed before verification")
			}
		})
	}
}

// The constructor must copy the bound taproot root so a later mutation of the
// caller's array cannot change the verifier's root binding.
func TestEngineRound2ShareVerifier_ConstructorCopiesTaprootRoot(t *testing.T) {
	attempt := [32]byte{0xaa}
	root := [32]byte{0xbb}
	originalRoot := root // value copy of the original bytes

	engine := &fakeShareVerifyingEngine{verdict: NativeShareVerdictValid}
	verifier, err := NewEngineRound2ShareVerifier(engine, Round2ShareVerificationBinding{
		SessionID:          "session-1",
		AttemptContextHash: attempt,
		TaprootMerkleRoot:  &root,
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}

	// Mutate the caller's array AFTER construction. If the verifier retained the
	// caller's pointer instead of copying, its root would now differ from
	// originalRoot and the original-root package below would (wrongly) mismatch.
	root[0] = 0xff

	pkgEnv, pkgHash := testSigningPackage(t, attempt, originalRoot[:], []byte{0xde, 0xad})
	shareEnv := testShareSubmissionEnvelope(t, attempt, 2, pkgHash, []byte{0xbe, 0xef})

	if result := verifier.VerifyRetainedShare(pkgEnv, shareEnv, 2); result != roast.ShareValid {
		t.Fatalf("expected ShareValid (verifier root unaffected by caller mutation), got [%v]", result)
	}
	if !engine.called {
		t.Fatal("expected the engine to be called against the copied root")
	}
}

func mustVerifier(
	t *testing.T,
	engine Round2ShareVerifyingEngine,
	sessionID string,
	attemptContextHash [32]byte,
	taprootMerkleRoot *[32]byte,
) *EngineRound2ShareVerifier {
	t.Helper()
	verifier, err := NewEngineRound2ShareVerifier(engine, Round2ShareVerificationBinding{
		SessionID:          sessionID,
		AttemptContextHash: attemptContextHash,
		TaprootMerkleRoot:  taprootMerkleRoot,
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: [%v]", err)
	}
	return verifier
}
