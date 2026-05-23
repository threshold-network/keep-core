package frost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ripemd160" //nolint:staticcheck // RIPEMD-160 is intentional for the HASH160 derivation.
)

// Cross-repo derivation fixture (also checked into the tbtc bridge repo
// at docs/test-vectors/wallet-pubkey-hash-derivation-vectors-v1.json).
// Each repo's test must reproduce the expected output from the same
// input; if either side drifts from the other, at least one repo's
// test fails. Drift between bridge and keep-core silently breaks the
// wallet identity contract for any wallet whose canonical identity is
// established cross-repo (in particular, FROST wallets registered via
// the FROST WalletRegistry will use this derivation).
const walletPubKeyHashDerivationVectorsPath = "testdata/wallet-pubkey-hash-derivation-vectors-v1.json"

type ecdsaVector struct {
	Name  string `json:"name"`
	Input struct {
		CompressedPubKey string `json:"compressedPubKey"`
	} `json:"input"`
	Expected struct {
		WalletPubKeyHash string `json:"walletPubKeyHash"`
	} `json:"expected"`
	Note string `json:"note,omitempty"`
}

type frostVector struct {
	Name  string `json:"name"`
	Input struct {
		XOnlyOutputKey string `json:"xOnlyOutputKey"`
	} `json:"input"`
	Expected struct {
		WalletPubKeyHash string `json:"walletPubKeyHash"`
	} `json:"expected"`
	Note string `json:"note,omitempty"`
}

type derivationFixture struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	EcdsaLegacy []ecdsaVector `json:"ecdsa_legacy"`
	FrostP2tr   []frostVector `json:"frost_p2tr"`
	DriftCheck  struct {
		TbtcPath     string `json:"tbtc_path"`
		KeepCorePath string `json:"keep_core_path"`
		Rule         string `json:"rule"`
	} `json:"drift_check"`
}

func loadDerivationFixture(t *testing.T) derivationFixture {
	t.Helper()

	data, err := os.ReadFile(walletPubKeyHashDerivationVectorsPath)
	if err != nil {
		t.Fatalf("fixture read: %v", err)
	}
	var fixture derivationFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("fixture parse: %v", err)
	}
	if fixture.Version != "v1" {
		t.Fatalf(
			"fixture schemaVersion drift: got %q, expected %q -- both repos must update together",
			fixture.Version,
			"v1",
		)
	}
	return fixture
}

// TestFrostWalletPubKeyHashDerivationVectors checks that
// frost.WalletPublicKeyHashCompatibilityAlias produces the expected
// 20-byte HASH160(0x02 || xOnlyOutputKey) for every FROST vector in
// the shared cross-repo fixture. The tbtc bridge runs the equivalent
// check against its own derivation (BitcoinTx.deriveWalletPubKeyHash-
// FromXOnly); if either side drifts, the wallet identity contract
// between the bridge and the protocol silently breaks for any FROST
// wallet whose canonical identity is established cross-repo.
func TestFrostWalletPubKeyHashDerivationVectors(t *testing.T) {
	fixture := loadDerivationFixture(t)

	if len(fixture.FrostP2tr) == 0 {
		t.Fatal("fixture must contain at least one FROST vector")
	}

	for _, vector := range fixture.FrostP2tr {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			xOnlyBytes, err := hex.DecodeString(
				strings.TrimPrefix(vector.Input.XOnlyOutputKey, "0x"),
			)
			if err != nil {
				t.Fatalf("decode xOnlyOutputKey: %v", err)
			}
			if len(xOnlyBytes) != OutputKeySize {
				t.Fatalf(
					"xOnlyOutputKey length: got %d, expected %d",
					len(xOnlyBytes),
					OutputKeySize,
				)
			}

			var outputKey OutputKey
			copy(outputKey[:], xOnlyBytes)

			alias := WalletPublicKeyHashCompatibilityAlias(outputKey)
			got := "0x" + hex.EncodeToString(alias[:])
			want := strings.ToLower(vector.Expected.WalletPubKeyHash)

			if got != want {
				t.Fatalf(
					"derivation drift for vector %q:\n  got:  %s\n  want: %s\n"+
						"\nThis test enforces the cross-repo contract that\n"+
						"frost.WalletPublicKeyHashCompatibilityAlias and the\n"+
						"tbtc bridge's BitcoinTx.deriveWalletPubKeyHashFromXOnly\n"+
						"produce the same 20-byte alias for the same input.\n"+
						"If this test fails, also expect the tbtc-side test to\n"+
						"fail unless the JSON fixture itself has drifted.",
					vector.Name,
					got,
					want,
				)
			}
		})
	}
}

// TestEcdsaCompressedPubKeyHash160Vectors checks the legacy ECDSA
// derivation path: HASH160 of the compressed pubkey. The tbtc bridge
// performs this implicitly during registerNewWallet (compress then
// hash160). The off-chain operator tooling that produces deposit
// scripts performs the same derivation; this test pins the algorithm
// from the keep-core side using the same vectors the bridge pins on
// its side.
func TestEcdsaCompressedPubKeyHash160Vectors(t *testing.T) {
	fixture := loadDerivationFixture(t)

	if len(fixture.EcdsaLegacy) == 0 {
		t.Fatal("fixture must contain at least one ECDSA vector")
	}

	for _, vector := range fixture.EcdsaLegacy {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			compressed, err := hex.DecodeString(
				strings.TrimPrefix(vector.Input.CompressedPubKey, "0x"),
			)
			if err != nil {
				t.Fatalf("decode compressedPubKey: %v", err)
			}

			got := "0x" + hex.EncodeToString(hash160(compressed))
			want := strings.ToLower(vector.Expected.WalletPubKeyHash)

			if got != want {
				t.Fatalf(
					"HASH160 drift for vector %q:\n  got:  %s\n  want: %s",
					vector.Name,
					got,
					want,
				)
			}
		})
	}
}

// TestDriftCheckMetadata asserts the fixture declares the tbtc mirror
// path and a non-empty drift rule. A future CI sync check can use
// these fields to compare files between repos.
func TestDriftCheckMetadata(t *testing.T) {
	fixture := loadDerivationFixture(t)

	if fixture.DriftCheck.TbtcPath != "docs/test-vectors/wallet-pubkey-hash-derivation-vectors-v1.json" {
		t.Errorf(
			"drift_check.tbtc_path drift: got %q",
			fixture.DriftCheck.TbtcPath,
		)
	}
	if fixture.DriftCheck.KeepCorePath != walletPubKeyHashDerivationVectorsPath {
		t.Errorf(
			"drift_check.keep_core_path inconsistency: fixture says %q, this test reads from %q",
			fixture.DriftCheck.KeepCorePath,
			walletPubKeyHashDerivationVectorsPath,
		)
	}
	if fixture.DriftCheck.Rule == "" {
		t.Error("drift_check.rule must be non-empty")
	}
}

// TestFixtureFileShouldExistAtMirrorPath documents the convention that
// the file lives at the path the fixture self-declares. Mostly a
// nudge for anyone moving the file: update the constant AND the
// fixture metadata together.
func TestFixtureFileShouldExistAtMirrorPath(t *testing.T) {
	fixture := loadDerivationFixture(t)

	abs, err := filepath.Abs(fixture.DriftCheck.KeepCorePath)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf(
			"fixture self-declares it lives at %q but the file is not there: %v",
			fixture.DriftCheck.KeepCorePath,
			err,
		)
	}
}

// hash160 reproduces Bitcoin's HASH160 (RIPEMD160(SHA256(x))) using
// the same primitive frost.WalletPublicKeyHashCompatibilityAlias
// invokes via btcutil.Hash160. We compute it directly here so the
// ECDSA test is self-contained and doesn't pull in btcutil for a one-
// liner.
func hash160(b []byte) []byte {
	sha := sha256.Sum256(b)
	rip := ripemd160.New()
	rip.Write(sha[:])
	return rip.Sum(nil)
}
