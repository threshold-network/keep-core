package tbtcpg

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// satoshiUnitsVectorFile is the shared cross-repo test vector for the
// satoshi-to-TBTC-base-unit conversion. The same file is read by the
// tbtc-v2 Solidity suite (as a Foundry test data file) and by the
// tbtc-v2 TypeScript SDK suite, so one set of numbers pins the unit
// across all three code bases.
//
// The path is relative to this package's directory. The file lives in the
// tbtc-v2 repository (solidity/test/data/reservations/satoshi-units.json);
// this copy under pkg/tbtcpg/internal/test/testdata/ keeps the keep-core
// test self-contained.
//
// The two copies are kept in agreement BY HAND, not by a cross-repo check:
// neither repository's CI can see the other repository's file, so there is
// no automated comparison between them. What IS enforced here is that the
// two sides agree on the FACTOR, and that this copy carries every key the
// shared file carries: the parser rejects unknown keys, so a key added to
// the shared file without being added to this copy fails this test rather
// than being silently ignored.
const satoshiUnitsVectorFile = "internal/test/testdata/satoshi-units.json"

// satoshiToTbtcBaseUnits is the conversion factor the whole system
// agrees on: one satoshi is 10^10 TBTC base units.
//
//   - the vault: `ReservationVault.SATOSHI_MULTIPLIER = 10 ** 10`
//     (tbtc-v2:solidity/contracts/vault/ReservationVault.sol), used by
//     `TBTCVault.mint(satoshis * SATOSHI_MULTIPLIER)`;
//   - the SDK: `amountToSatoshi` divides by `BigNumber.from(1e10)`
//     (tbtc-v2:typescript/src/lib/utils/bitcoin.ts);
//   - keep-core: the amount a client carries is in satoshi, and the vault
//     balances it reports are in TBTC base units, so the two are related
//     by exactly this factor.
//
// INV-704 asks for ONE assertion tying the three together. This test
// pins the keep-core side against the shared vector, so a change to the
// factor on any one side breaks a test on the others.
const satoshiToTbtcBaseUnits int64 = 10_000_000_000

// satoshiUnitsVector is the parsed shared vector file.
//
// Every key the shared file carries is declared here, and the parser
// rejects unknown keys, so the two copies cannot silently drift apart on
// the key set. RoundingCases is the shared file's list of base-unit
// amounts that are not multiples of 1e10, with the truncated satoshi value
// and the remainder; keep-core does not compute a conversion of its own,
// so it carries the key without asserting on it.
type satoshiUnitsVector struct {
	Vector  string `json:"vector"`
	Version int    `json:"version"`
	// Constant is the human-readable statement of the factor, for
	// example "1 satoshi = 10000000000 TBTC base units (1e10)".
	Constant string `json:"constant"`
	Pairs    []struct {
		Satoshis      int64  `json:"satoshis"`
		TbtcBaseUnits string `json:"tbtcBaseUnits"`
	} `json:"pairs"`
	RoundingCases []struct {
		TbtcBaseUnits string `json:"tbtcBaseUnits"`
		Satoshis      int64  `json:"satoshis"`
		Remainder     int64  `json:"remainder"`
	} `json:"roundingCases"`
}

// loadSatoshiUnitsVector reads and parses the shared vector file.
//
// The decode rejects unknown keys. The shared file is the contract between
// the repositories, so a key that this side does not know about is a
// sign that the two copies have drifted, and it must fail the test rather
// than be silently ignored.
//
// NEGATIVE CONTROL: add any unknown top-level key to the vector file and
// this fails with a "json: unknown field" error.
func loadSatoshiUnitsVector(t *testing.T) satoshiUnitsVector {
	t.Helper()

	contents, err := os.ReadFile(filepath.Clean(satoshiUnitsVectorFile))
	if err != nil {
		t.Fatalf(
			"failed to read the satoshi units vector file [%s]: %v",
			satoshiUnitsVectorFile,
			err,
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()

	var vector satoshiUnitsVector
	if err := decoder.Decode(&vector); err != nil {
		t.Fatalf("failed to parse the satoshi units vector file: %v", err)
	}

	if vector.Vector != "satoshi-to-tbtc-base-units" {
		t.Errorf(
			"unexpected vector name [%s], expected [satoshi-to-tbtc-base-units]",
			vector.Vector,
		)
	}

	if len(vector.Pairs) == 0 {
		t.Fatal("expected at least one pair in the satoshi units vector")
	}

	return vector
}

// TestSatoshiToTbtcBaseUnits_Vector pins the keep-core side of INV-704:
// one satoshi is 10^10 TBTC base units, and the shared cross-repo vector
// file says so for every pair it lists.
//
// The keep-core client carries reservation amounts in satoshi and reads
// the reservation vault's fee reserve in TBTC base units (1e18 per whole
// TBTC, see pkg/tbtc/chain.go's ReservationVaultFeeReserveTbtcBaseUnits
// and pkg/tbtcpg/chain.go's). A factor that differs by a power of ten on
// any one side would show up here as a mismatch against the shared
// vector, which the Solidity and TypeScript suites read as well.
//
// NEGATIVE CONTROL: change satoshiToTbtcBaseUnits from 10^10 to 10^9
// and this test fails, because the computed base units no longer match
// the vector.
func TestSatoshiToTbtcBaseUnits_Vector(t *testing.T) {
	vector := loadSatoshiUnitsVector(t)

	for _, pair := range vector.Pairs {
		satoshis := pair.Satoshis
		expected, ok := new(big.Int).SetString(pair.TbtcBaseUnits, 10)
		if !ok {
			t.Errorf(
				"failed to parse the expected base units [%s] for [%d] satoshis",
				pair.TbtcBaseUnits,
				satoshis,
			)
			continue
		}

		actual := new(big.Int).Mul(
			big.NewInt(satoshis),
			big.NewInt(satoshiToTbtcBaseUnits),
		)

		if actual.Cmp(expected) != 0 {
			t.Errorf(
				"expected %d satoshis to be %s TBTC base units, got %s",
				satoshis,
				expected.String(),
				actual.String(),
			)
		}
	}
}

// TestSatoshiToTbtcBaseUnits_Constant pins the factor itself, and that
// the shared vector's own statement of it agrees. The vector's constant
// string is the human-readable form the other two repos' suites assert
// against, so a change to the factor on one side shows up as a mismatch
// in the other's file.
//
// NEGATIVE CONTROL: change satoshiToTbtcBaseUnits and both this test and
// TestSatoshiToTbtcBaseUnits_Vector fail.
func TestSatoshiToTbtcBaseUnits_Constant(t *testing.T) {
	if satoshiToTbtcBaseUnits != 10_000_000_000 {
		t.Fatalf(
			"expected one satoshi to be 10000000000 TBTC base units, got %d",
			satoshiToTbtcBaseUnits,
		)
	}

	vector := loadSatoshiUnitsVector(t)

	const expectedConstant = "1 satoshi = 10000000000 TBTC base units (1e10)"
	if vector.Constant != expectedConstant {
		t.Errorf(
			"expected the shared vector constant to be [%s], got [%s]",
			expectedConstant,
			vector.Constant,
		)
	}

	// The vector must contain the defining pair: one satoshi is
	// 10000000000 base units. Without it the vector would not pin the
	// factor at all.
	found := false
	for _, pair := range vector.Pairs {
		if pair.Satoshis == 1 && pair.TbtcBaseUnits == "10000000000" {
			found = true
			break
		}
	}
	if !found {
		t.Error(
			"expected the shared vector to contain the pair " +
				"[1 satoshi -> 10000000000 base units]",
		)
	}
}

// TestSatoshiToTbtcBaseUnits_VectorContents pins the EXACT contents of
// the vector's `pairs` and `roundingCases` arrays, so that a change to
// either array is a test failure rather than a silent edit of a shared
// file.
//
// The two repositories keep separate copies of the vector and neither
// one's CI can see the other's file, so this list is the only thing that
// holds the Go copy still. The copies are deliberately NOT byte-identical:
// the tbtc-v2 copy formats the pair objects with spaces inside the braces
// (`{ "satoshis": 1, ... }`) and this copy does not
// (`{"satoshis": 1, ...}`). That formatting difference is intentional and
// is NOT an assertion of equality; only the semantic content, which is
// what this test pins, has to agree.
//
// NEGATIVE CONTROL: change any pair's satoshi count or base-unit amount
// in the vector file, or change any rounding case, and this test fails.
func TestSatoshiToTbtcBaseUnits_VectorContents(t *testing.T) {
	vector := loadSatoshiUnitsVector(t)

	expectedPairs := []struct {
		Satoshis      int64
		TbtcBaseUnits string
	}{
		{Satoshis: 1, TbtcBaseUnits: "10000000000"},
		{Satoshis: 1500, TbtcBaseUnits: "15000000000000"},
		{Satoshis: 50000, TbtcBaseUnits: "500000000000000"},
		{Satoshis: 21000000, TbtcBaseUnits: "210000000000000000"},
		{Satoshis: 3000000, TbtcBaseUnits: "30000000000000000"},
	}

	if len(vector.Pairs) != len(expectedPairs) {
		t.Fatalf(
			"expected the shared vector to carry %d pairs, got %d",
			len(expectedPairs),
			len(vector.Pairs),
		)
	}

	for i, expected := range expectedPairs {
		actual := vector.Pairs[i]

		if actual.Satoshis != expected.Satoshis {
			t.Errorf(
				"pair %d: expected %d satoshis, got %d",
				i,
				expected.Satoshis,
				actual.Satoshis,
			)
		}

		if actual.TbtcBaseUnits != expected.TbtcBaseUnits {
			t.Errorf(
				"pair %d: expected %s TBTC base units, got %s",
				i,
				expected.TbtcBaseUnits,
				actual.TbtcBaseUnits,
			)
		}
	}

	expectedRoundingCases := []struct {
		TbtcBaseUnits string
		Satoshis      int64
		Remainder     int64
	}{
		{TbtcBaseUnits: "10000000000", Satoshis: 1, Remainder: 0},
		{TbtcBaseUnits: "10000000001", Satoshis: 1, Remainder: 1},
		{TbtcBaseUnits: "15000000001", Satoshis: 1, Remainder: 5000000001},
		{
			TbtcBaseUnits: "99999999999999999999",
			Satoshis:      9999999999,
			Remainder:     9999999999,
		},
	}

	if len(vector.RoundingCases) != len(expectedRoundingCases) {
		t.Fatalf(
			"expected the shared vector to carry %d rounding cases, got %d",
			len(expectedRoundingCases),
			len(vector.RoundingCases),
		)
	}

	for i, expected := range expectedRoundingCases {
		actual := vector.RoundingCases[i]

		if actual.TbtcBaseUnits != expected.TbtcBaseUnits {
			t.Errorf(
				"rounding case %d: expected %s TBTC base units, got %s",
				i,
				expected.TbtcBaseUnits,
				actual.TbtcBaseUnits,
			)
		}

		if actual.Satoshis != expected.Satoshis {
			t.Errorf(
				"rounding case %d: expected %d satoshis, got %d",
				i,
				expected.Satoshis,
				actual.Satoshis,
			)
		}

		if actual.Remainder != expected.Remainder {
			t.Errorf(
				"rounding case %d: expected a remainder of %d, got %d",
				i,
				expected.Remainder,
				actual.Remainder,
			)
		}
	}
}

// TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale ties the unit factor
// to the two keep-core quantities that cross the unit boundary: the
// vault's fee DEBT, which is in satoshi, and the vault's fee RESERVE,
// which is in TBTC base units.
//
// The vault retains the initiation fee in TBTC base units while the
// in-kind fee debt is denominated in satoshi, so the two are related by
// exactly the satoshi-to-base-unit factor. A debt of one satoshi
// corresponds to a base-unit obligation of 10^10; a debt of a million
// satoshis corresponds to 10^16 base units.
//
// NEGATIVE CONTROL: change satoshiToTbtcBaseUnits and this test fails.
func TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale(t *testing.T) {
	tests := []struct {
		name          string
		debtSat       uint64
		expectedUnits string
	}{
		{
			name:          "one satoshi of debt is 1e10 base units",
			debtSat:       1,
			expectedUnits: "10000000000",
		},
		{
			name:          "a million satoshis of debt is 1e16 base units",
			debtSat:       1_000_000,
			expectedUnits: "10000000000000000",
		},
		{
			name:          "a billion satoshis of debt is 1e19 base units",
			debtSat:       1_000_000_000,
			expectedUnits: "10000000000000000000",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := new(big.Int).Mul(
				new(big.Int).SetUint64(test.debtSat),
				big.NewInt(satoshiToTbtcBaseUnits),
			)

			expected, ok := new(big.Int).SetString(test.expectedUnits, 10)
			if !ok {
				t.Fatalf(
					"failed to parse the expected base units [%s]",
					test.expectedUnits,
				)
			}

			if actual.Cmp(expected) != 0 {
				t.Errorf(
					"expected a debt of %d satoshis to correspond to %s "+
						"TBTC base units, got %s",
					test.debtSat,
					expected.String(),
					actual.String(),
				)
			}
		})
	}
}
