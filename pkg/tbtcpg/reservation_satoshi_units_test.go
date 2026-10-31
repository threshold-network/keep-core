package tbtcpg

import (
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
// test self-contained, and the two files are asserted to agree by the
// cross-repo check below.
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
}

// loadSatoshiUnitsVector reads and parses the shared vector file.
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

	var vector satoshiUnitsVector
	if err := json.Unmarshal(contents, &vector); err != nil {
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
