//go:build frost_native

package signing

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
)

func TestSealRound2Share_RoundTrip(t *testing.T) {
	recipient, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient key: %v", err)
	}
	share := []byte("a-secret-frost-round-2-share-payload")

	sealed, err := sealRound2Share(share, recipient.PublicKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// The plaintext share must not appear anywhere in the sealed envelope.
	if bytes.Contains(sealed.Ciphertext, share) {
		t.Fatal("sealed ciphertext leaks the plaintext share")
	}
	if len(sealed.EphemeralPublicKey) == 0 {
		t.Fatal("sealed envelope is missing its ephemeral public key")
	}

	opened, err := openRound2Share(sealed, recipient.PrivateKey)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(opened, share) {
		t.Fatalf("round-trip mismatch: got %q, want %q", opened, share)
	}
}

func TestSealRound2Share_WrongRecipientCannotOpen(t *testing.T) {
	recipient, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient key: %v", err)
	}
	intruder, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("intruder key: %v", err)
	}
	share := []byte("secret-share")

	sealed, err := sealRound2Share(share, recipient.PublicKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	// A member the share was NOT sealed to must not recover it: either the
	// authenticated box fails to open, or (defensively) the result differs.
	opened, err := openRound2Share(sealed, intruder.PrivateKey)
	if err == nil && bytes.Equal(opened, share) {
		t.Fatal("a non-recipient recovered the sealed share")
	}
}

func TestSealRound2Share_TamperedEnvelopeFails(t *testing.T) {
	recipient, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient key: %v", err)
	}
	share := []byte("secret-share")
	sealed, err := sealRound2Share(share, recipient.PublicKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if len(sealed.Ciphertext) == 0 {
		t.Fatal("empty ciphertext")
	}
	// Flip a ciphertext bit; the authenticated box must not open it to the share.
	sealed.Ciphertext[len(sealed.Ciphertext)-1] ^= 0xff

	opened, err := openRound2Share(sealed, recipient.PrivateKey)
	if err == nil && bytes.Equal(opened, share) {
		t.Fatal("a tampered envelope still opened to the original share")
	}
}

func TestSealRound2Share_NilInputs(t *testing.T) {
	recipient, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient key: %v", err)
	}
	if _, err := sealRound2Share([]byte("x"), nil); err == nil {
		t.Fatal("seal to a nil recipient must error")
	}
	if _, err := openRound2Share(nil, recipient.PrivateKey); err == nil {
		t.Fatal("open of a nil envelope must error")
	}
	sealed, err := sealRound2Share([]byte("x"), recipient.PublicKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := openRound2Share(sealed, nil); err == nil {
		t.Fatal("open with a nil private key must error")
	}
}
