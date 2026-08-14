//go:build frost_native

package signing

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/operator"
)

func TestOperatorKeyConversion_SealsAndOpens(t *testing.T) {
	priv, pub, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatalf("operator key: %v", err)
	}

	// Convert the operator keypair separately, then verify a share sealed to the
	// converted PUBLIC key opens with the converted PRIVATE key - i.e. the
	// conversions produce a matching ECDH keypair.
	recipientPub, err := operatorPublicKeyToEphemeral(pub)
	if err != nil {
		t.Fatalf("convert public: %v", err)
	}
	recipientPriv, err := operatorPrivateKeyToEphemeral(priv)
	if err != nil {
		t.Fatalf("convert private: %v", err)
	}

	share := []byte("a-share-sealed-to-an-operator-key")
	sealed, err := sealRound2Share(share, recipientPub)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	opened, err := openRound2Share(sealed, recipientPriv)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(opened, share) {
		t.Fatal("share sealed to a converted operator public key did not open with the converted private key")
	}
}

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

func TestSealRound2Share_MalformedEphemeralKeyFails(t *testing.T) {
	recipient, err := ephemeral.GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient key: %v", err)
	}
	sealed, err := sealRound2Share([]byte("secret-share"), recipient.PublicKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// A garbage ephemeral public key must fail to parse, not panic or open.
	sealed.EphemeralPublicKey = []byte{0x00, 0x01, 0x02}
	if _, err := openRound2Share(sealed, recipient.PrivateKey); err == nil {
		t.Fatal("open with a malformed ephemeral public key must error")
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
