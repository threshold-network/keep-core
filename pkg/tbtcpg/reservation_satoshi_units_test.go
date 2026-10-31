package tbtcpg

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testdata/reservations"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// satoshiUnitsVectorFile is the repository path of the shared cross-repo
// test vector for the satoshi-to-TBTC-base-unit conversion. The same file
// is read by the tbtc-v2 Solidity suite (as a Foundry test data file) and
// by the tbtc-v2 TypeScript SDK suite, so one set of numbers pins the unit
// across all three code bases.
//
// The file lives in the shared internal/testdata/reservations package
// because the production consumers of the unit boundary sit in two
// different packages: the chain adapters that read the vault's
// inKindFeeDebtSat view and TBTC balance (pkg/chain/ethereum) and the
// gauge publisher that reports them (this package). A vector readable from
// only one of them would let one side assert the boundary against its own
// copy of the numbers.
//
// The two copies of the file (this one and tbtc-v2's) are kept in
// agreement BY HAND, not by a cross-repo check: neither repository's CI
// can see the other repository's file, so there is no automated comparison
// between them. What IS enforced here is that this copy carries every key
// the shared file carries: the parser rejects unknown keys, so a key added
// to the shared file without being added to this copy fails this test
// rather than being silently ignored.
const satoshiUnitsVectorFile = reservations.SatoshiUnitsVectorFile

// satoshiUnitsVector is the parsed shared vector file.
//
// Every key the shared file carries is declared here, and the parser
// rejects unknown keys, so the two copies cannot silently drift apart on
// the key set. RoundingCases is the shared file's list of base-unit
// amounts that are not multiples of 1e10, with the truncated satoshi value
// and the remainder; keep-core does not compute a conversion of its own,
// so it carries the key without asserting on it.
type satoshiUnitsVector = reservations.SatoshiUnitsVector

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

	vector, err := reservations.ParseSatoshiUnitsVector(
		reservations.SatoshiUnitsJSON(),
	)
	if err != nil {
		t.Fatalf("%v", err)
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

// TestSatoshiToTbtcBaseUnits_Vector pins the shared vector's own
// self-consistency: every pair it lists is the pair's satoshi count
// multiplied by the factor its defining pair states.
//
// The factor is READ from the vector rather than declared in Go, so this
// test is a check of the shared FILE and not of a test-local constant.
// The production unit handling is asserted separately, by driving the
// production consumers with the vector's own values (see
// TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale and the matching
// pkg/chain/ethereum test).
//
// NEGATIVE CONTROL: change any pair's satoshi count or base-unit amount in
// the vector file and this test fails, because the pair no longer equals
// its satoshi count multiplied by the factor the defining pair states.
func TestSatoshiToTbtcBaseUnits_Vector(t *testing.T) {
	vector := loadSatoshiUnitsVector(t)

	factor, err := vector.BaseUnitsPerSatoshi()
	if err != nil {
		t.Fatalf("%v", err)
	}

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
			factor,
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
// NEGATIVE CONTROL: change the vector's defining pair or its constant
// string and this test fails.
func TestSatoshiToTbtcBaseUnits_Constant(t *testing.T) {
	vector := loadSatoshiUnitsVector(t)

	const expectedConstant = "1 satoshi = 10000000000 TBTC base units (1e10)"
	if vector.Constant != expectedConstant {
		t.Errorf(
			"expected the shared vector constant to be [%s], got [%s]",
			expectedConstant,
			vector.Constant,
		)
	}

	factor, err := vector.BaseUnitsPerSatoshi()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if factor.Cmp(big.NewInt(10_000_000_000)) != 0 {
		t.Errorf(
			"expected one satoshi to be 10000000000 TBTC base units, got %s",
			factor.String(),
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

// feeGaugeUnitViolations returns the unit mismatches between the values
// the production gauge publisher published and the vault readings it was
// driven with.
//
// The two keep-core quantities that cross the unit boundary are read in
// different units: the vault's in-kind fee DEBT is denominated in
// satoshi and the vault's fee RESERVE is a TBTC token balance in TBTC
// base units. The publisher reports them on two separate gauges, so each
// gauge must carry its own reading unchanged: a publisher that scaled
// either value by the satoshi-to-base-unit factor would publish a
// plausible-looking number in the wrong unit.
//
// The comparison lives in the shared vector package so the chain-side
// adapter test (pkg/chain/ethereum) asserts the same thing about the
// same vector.
func feeGaugeUnitViolations(
	debtGauge float64,
	reserveGauge float64,
	debtSat uint64,
	reserveUnits *big.Int,
) []string {
	return reservations.GaugeUnitViolations(
		debtSat,
		debtGauge,
		reserveUnits,
		reserveGauge,
	)
}

// TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale ties the shared
// vector's unit factor to the two keep-core quantities that cross the
// unit boundary, through the production gauge publisher.
//
// The vault retains the initiation fee in TBTC base units while the
// in-kind fee debt is denominated in satoshi, so the two are related by
// exactly the satoshi-to-base-unit factor: a debt of one satoshi
// corresponds to a base-unit obligation of 10^10, a debt of a million
// satoshis to 10^16 base units.
//
// The test drives the vector's non-zero pairs through the REAL publisher:
// the ReservationAcceptanceTask's Run publishes
// reservation_vault_fee_debt_sat and
// reservation_vault_fee_reserve_tbtc_base_units from the chain's fee
// readings, and the assertions require each gauge to carry its reading
// unchanged. That is what makes the factor a production claim: a
// publisher that converted either reading would be caught here, and the
// vector is what says which scale each reading is on.
//
// The matching chain-side test (pkg/chain/ethereum) drives the same
// vector through the adapters that read the vault's on-chain values.
//
// NEGATIVE CONTROL: the mis-scaled readings the helper is asked to
// reject are exactly the ones a scale error in the publisher would
// produce - the debt published in base units and the reserve published in
// satoshi - and the negative control requires the helper to reject them.
// A publisher that scaled a reading would produce one of those, and the
// positive assertion above would fail.
func TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale(t *testing.T) {
	vector := loadSatoshiUnitsVector(t)

	for _, pair := range vector.Pairs {
		pair := pair

		t.Run(fmt.Sprintf("%d satoshis", pair.Satoshis), func(t *testing.T) {
			debtSat := uint64(pair.Satoshis)

			reserveUnits, ok := new(big.Int).SetString(pair.TbtcBaseUnits, 10)
			if !ok {
				t.Fatalf(
					"failed to parse the expected base units [%s]",
					pair.TbtcBaseUnits,
				)
			}

			lc := NewLocalChain()
			btcChain := NewLocalBitcoinChain()

			runFeeGaugeFixture(t, lc)

			// The vault reads the pair's two values, in their own
			// units: the debt in satoshi, the reserve in TBTC base
			// units.
			lc.SetReservationVaultFeeDebtSat(debtSat, nil)
			lc.SetReservationVaultFeeReserveBalance(reserveUnits, nil)

			task := NewReservationAcceptanceTask(lc, btcChain)
			recorder := newFakeMetricsRecorder()
			task.setMetricsRecorder(recorder)

			walletPublicKeyHash := [20]byte{
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
			}

			if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
				WalletPublicKeyHash: walletPublicKeyHash,
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			debtGauge, ok := recorder.calls["reservation_vault_fee_debt_sat"]
			if !ok {
				t.Fatal(
					"expected the reservation_vault_fee_debt_sat gauge " +
						"to be published",
				)
			}

			reserveGauge, ok :=
				recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]
			if !ok {
				t.Fatal(
					"expected the reservation_vault_fee_reserve_tbtc_base_units " +
						"gauge to be published",
				)
			}

			violations := feeGaugeUnitViolations(
				debtGauge,
				reserveGauge,
				debtSat,
				reserveUnits,
			)
			for _, violation := range violations {
				t.Error(violation)
			}

			// NEGATIVE CONTROL: the two mis-scaled readings a scale
			// error in the publisher would produce - the debt
			// published in base units, the reserve published in
			// satoshi - must be rejected by the same comparison.
			misScaledDebt, misScaledReserve :=
				reservations.MisScaledReadings(debtSat, reserveUnits)
			misScaledReserveFloat, _ := misScaledReserve.Float64()

			misScaledViolations := feeGaugeUnitViolations(
				float64(misScaledDebt),
				misScaledReserveFloat,
				debtSat,
				reserveUnits,
			)
			if len(misScaledViolations) == 0 {
				t.Fatal(
					"expected the comparison to reject a fee-debt " +
						"reading published in base units and a " +
						"fee-reserve reading published in satoshi",
				)
			}
		})
	}
}
