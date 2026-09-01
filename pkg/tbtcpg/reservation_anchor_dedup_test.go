package tbtcpg

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// TestBuildReservationAnchorTransaction_MatchesPkgTbtcGoldenOutput is a
// golden-value equivalence check for the reservation-anchor-assembly
// duplication documented at reservation_acceptance.go:582-585:
// buildReservationAnchorTransaction here and
// pkg/tbtc.assembleReservationAnchorTransaction are two independently
// maintained copies of the same 1-input-1-output transaction-building
// logic ("Any change to the anchor transaction shape must be applied to
// both sites").
//
// Both functions are unexported in different packages, so Go's visibility
// rules make a single test that calls both and diffs the result
// impossible without changing production code (exporting one, or
// extracting a shared helper - see the gap-analysis doc for that
// alternative). This test instead pins this package's output to a fixed,
// documented golden value; pkg/tbtc's
// TestAssembleReservationAnchorTransaction pins its sibling function to
// the identical deposit value (100000), fee (1500), and expected output
// value (98500) via a matching comment there. If either copy's shape
// diverges from the other, its own test starts failing against its half
// of this shared golden value - not as tight as calling both from one
// test, but it does catch drift in either direction.
func TestBuildReservationAnchorTransaction_MatchesPkgTbtcGoldenOutput(t *testing.T) {
	bitcoinChain := NewLocalBitcoinChain()

	// Ad-hoc secp256k1 keypair used only to produce a technically valid
	// ECDSA signature over the input's sighash (AddSignatures verifies the
	// signature against the sighash, not against the deposit script's
	// actual spending conditions - no Bitcoin VM execution happens in this
	// unit test). The destination wallet public key hash below is an
	// independent, arbitrary value; it does not need to correspond to
	// this key.
	privateKeyValue := big.NewInt(100)
	x, y := tecdsa.Curve.ScalarBaseMult(privateKeyValue.Bytes())
	publicKey := &ecdsa.PublicKey{Curve: tecdsa.Curve, X: x, Y: y}
	privateKey := &ecdsa.PrivateKey{PublicKey: *publicKey, D: privateKeyValue}

	walletPublicKeyHash := [20]byte{
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33,
		0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd,
	}
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	deposit := &tbtc.Deposit{
		Depositor:           chain.Address("0x1111111111111111111111111111111111111111"),
		BlindingFactor:      [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		WalletPublicKeyHash: walletPublicKeyHash,
		RefundPublicKeyHash: [20]byte{0x02},
		RefundLocktime:      [4]byte{0x03, 0x04, 0x05, 0x06},
	}

	depositScript, err := deposit.Script()
	if err != nil {
		t.Fatal(err)
	}

	depositScriptHash := sha256.Sum256(depositScript)
	depositLockingScript, err := bitcoin.PayToWitnessScriptHash(depositScriptHash)
	if err != nil {
		t.Fatal(err)
	}

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x0b},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: depositLockingScript,
			},
		},
	}
	bitcoinChain.SetTransaction(fundingTransaction.Hash(), fundingTransaction)

	deposit.Utxo = &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	builder, err := buildReservationAnchorTransaction(
		bitcoinChain,
		deposit,
		walletPublicKeyHash,
		1500,
	)
	if err != nil {
		t.Fatal(err)
	}

	sigHashes, err := builder.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}

	signatures := make([]*bitcoin.SignatureContainer, len(sigHashes))
	for i, sigHash := range sigHashes {
		r, s, err := ecdsa.Sign(rand.Reader, privateKey, sigHash.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures[i] = &bitcoin.SignatureContainer{
			R:         r,
			S:         s,
			PublicKey: publicKey,
		}
	}

	transaction, err := builder.AddSignatures(signatures)
	if err != nil {
		t.Fatal(err)
	}

	// Golden value: deposit 100000, fee 1500 -> anchor output 98500.
	// pkg/tbtc.TestAssembleReservationAnchorTransaction asserts the same
	// arithmetic for its sibling function.
	expectedOutputs := []*bitcoin.TransactionOutput{
		{
			Value:           98500,
			PublicKeyScript: walletScript,
		},
	}

	if !reflect.DeepEqual(expectedOutputs, transaction.Outputs) {
		t.Errorf(
			"unexpected outputs\nexpected: [%+v]\nactual:   [%+v]",
			expectedOutputs,
			transaction.Outputs,
		)
	}
}
