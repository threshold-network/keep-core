package entry

import (
	"math/big"
	"testing"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"

	"github.com/keep-network/keep-core/pkg/bls"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// extractAndValidateShare is the gatekeeper that decides whether a signature
// share received from another group member is trustworthy. A member that
// accepted a bad share would contribute to a corrupt relay entry, so each
// rejection path matters as much as the accept path.
func TestExtractAndValidateShare(t *testing.T) {
	senderID := group.MemberIndex(2)
	secretKey := big.NewInt(1234567)

	// previousEntry is the G1 point being signed; the share is the point
	// signed with the sender's secret key and the public key share is the
	// matching G2 public key.
	previousEntry := new(bn256.G1).ScalarBaseMult(big.NewInt(7))
	validShare := bls.SignG1(secretKey, previousEntry)
	publicKeyShare := new(bn256.G2).ScalarBaseMult(secretKey)

	t.Run("valid share", func(t *testing.T) {
		message := NewSignatureShareMessage(senderID, validShare.Marshal(), "s")
		shares := map[group.MemberIndex]*bn256.G2{senderID: publicKeyShare}

		share, err := extractAndValidateShare(message, shares, previousEntry)
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if share == nil {
			t.Fatal("expected a non-nil share")
		}
		// The returned share must be the one carried by the message.
		if !bls.VerifyG1(publicKeyShare, previousEntry, share) {
			t.Error("returned share does not verify against the public key share")
		}
	})

	t.Run("unparseable share bytes", func(t *testing.T) {
		message := NewSignatureShareMessage(senderID, []byte{0x01, 0x02}, "s")
		shares := map[group.MemberIndex]*bn256.G2{senderID: publicKeyShare}

		if _, err := extractAndValidateShare(message, shares, previousEntry); err == nil {
			t.Error("expected an error for share bytes that are not a valid G1 point")
		}
	})

	t.Run("no public key share for sender", func(t *testing.T) {
		message := NewSignatureShareMessage(senderID, validShare.Marshal(), "s")
		shares := map[group.MemberIndex]*bn256.G2{} // sender absent

		if _, err := extractAndValidateShare(message, shares, previousEntry); err == nil {
			t.Error("expected an error when the sender has no known public key share")
		}
	})

	t.Run("share does not verify", func(t *testing.T) {
		// A share signed with a different secret key than the public key share
		// advertises must be rejected.
		wrongShare := bls.SignG1(big.NewInt(7654321), previousEntry)
		message := NewSignatureShareMessage(senderID, wrongShare.Marshal(), "s")
		shares := map[group.MemberIndex]*bn256.G2{senderID: publicKeyShare}

		if _, err := extractAndValidateShare(message, shares, previousEntry); err == nil {
			t.Error("expected an error for a share that does not verify")
		}
	})
}
