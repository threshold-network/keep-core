package roast

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const testElectedCoordinator = group.MemberIndex(3)

func TestSignSigningPackage_RoundTripAuthenticates(t *testing.T) {
	pkg := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  uint32(testElectedCoordinator),
		SigningPackageBytes: []byte("frost-signing-package-bytes"),
	}
	if err := SignSigningPackage(&fakeSigner{id: testElectedCoordinator}, pkg); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(pkg.CoordinatorSignature) == 0 {
		t.Fatal("SignSigningPackage must set a coordinator signature")
	}

	// A member receives the envelope off the wire and authenticates it as
	// genuine evidence from the attempt's elected coordinator.
	wire, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var received SigningPackage
	if err := received.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := AuthenticateSigningPackage(
		fakeVerifier{},
		&received,
		testElectedCoordinator,
		pinnedContextHash[:],
	); err != nil {
		t.Fatalf("authenticate a genuine package: %v", err)
	}
}

func TestAuthenticateSigningPackage_Rejections(t *testing.T) {
	signed := func() *SigningPackage {
		pkg := &SigningPackage{
			AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
			CoordinatorIDValue:  uint32(testElectedCoordinator),
			SigningPackageBytes: []byte("pkg"),
		}
		if err := SignSigningPackage(&fakeSigner{id: testElectedCoordinator}, pkg); err != nil {
			t.Fatalf("sign: %v", err)
		}
		return pkg
	}
	otherAttempt := bytes.Repeat([]byte{0x09}, attempt.MessageDigestLength)

	t.Run("missing signature is rejected", func(t *testing.T) {
		pkg := signed()
		pkg.CoordinatorSignature = nil
		err := AuthenticateSigningPackage(fakeVerifier{}, pkg, testElectedCoordinator, pinnedContextHash[:])
		if !errors.Is(err, ErrSignatureMissing) {
			t.Fatalf("want ErrSignatureMissing, got %v", err)
		}
	})

	t.Run("non-elected coordinator is rejected without retention", func(t *testing.T) {
		err := AuthenticateSigningPackage(fakeVerifier{}, signed(), testElectedCoordinator+1, pinnedContextHash[:])
		if !errors.Is(err, ErrSigningPackageWrongCoordinator) {
			t.Fatalf("want ErrSigningPackageWrongCoordinator, got %v", err)
		}
	})

	t.Run("wrong attempt context is rejected", func(t *testing.T) {
		err := AuthenticateSigningPackage(fakeVerifier{}, signed(), testElectedCoordinator, otherAttempt)
		if !errors.Is(err, ErrSigningPackageWrongAttempt) {
			t.Fatalf("want ErrSigningPackageWrongAttempt, got %v", err)
		}
	})

	t.Run("tampered signature fails verification", func(t *testing.T) {
		pkg := signed()
		pkg.CoordinatorSignature[0] ^= 0xff
		err := AuthenticateSigningPackage(fakeVerifier{}, pkg, testElectedCoordinator, pinnedContextHash[:])
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("want ErrSignatureInvalid, got %v", err)
		}
	})

	t.Run("package signed by a non-elected operator is rejected", func(t *testing.T) {
		// A non-elected operator signs a body carrying the elected
		// coordinator's id (attempt_context_hash is public, so it can). The
		// signature does not verify under the elected coordinator's key.
		pkg := &SigningPackage{
			AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
			CoordinatorIDValue:  uint32(testElectedCoordinator),
			SigningPackageBytes: []byte("pkg"),
		}
		if err := SignSigningPackage(&fakeSigner{id: testElectedCoordinator + 7}, pkg); err != nil {
			t.Fatalf("sign: %v", err)
		}
		err := AuthenticateSigningPackage(fakeVerifier{}, pkg, testElectedCoordinator, pinnedContextHash[:])
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("want ErrSignatureInvalid, got %v", err)
		}
	})
}

func TestSigningPackage_MatchesRoot(t *testing.T) {
	root := bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength)
	other := bytes.Repeat([]byte{0xcd}, TaprootMerkleRootLength)
	keyPath := &SigningPackage{}
	scriptPath := &SigningPackage{TaprootMerkleRoot: root}

	if !keyPath.MatchesRoot(nil) {
		t.Fatal("a key-path package must match an empty live root")
	}
	if keyPath.MatchesRoot(root) {
		t.Fatal("a key-path package must not match a script-path live root")
	}
	if !scriptPath.MatchesRoot(root) {
		t.Fatal("a script-path package must match its own root")
	}
	if scriptPath.MatchesRoot(nil) {
		t.Fatal("a script-path package must not match an empty (key-path) live root")
	}
	if scriptPath.MatchesRoot(other) {
		t.Fatal("a script-path package must not match a divergent root")
	}
}
