// Package reservations holds the test vectors that the keep-core
// packages asserting on the reservations feature share.
//
// The satoshi-to-TBTC-base-unit vector lives here because the two
// production consumers of the unit boundary sit in two different
// packages: the chain adapters that read the ReservationVault's
// inKindFeeDebtSat view and TBTC balance (pkg/chain/ethereum) and the
// gauge publisher that reports them (pkg/tbtcpg). A vector file
// readable from only one of them would let one side assert the
// boundary against its own copy of the numbers.
package reservations

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
)

// satoshiUnitsJSON is the shared cross-repo test vector for the
// satoshi-to-TBTC-base-unit conversion. The same file is read by the
// tbtc-v2 Solidity suite (as a Foundry test data file) and by the
// tbtc-v2 TypeScript SDK suite, so one set of numbers pins the unit
// across all three code bases.
//
// The two copies of the file (this one and tbtc-v2's) are kept in
// agreement BY HAND, not by a cross-repo check: neither repository's CI
// can see the other repository's file, so there is no automated
// comparison between them. What IS enforced by the keep-core tests is
// that this copy carries every key the shared file carries: the parser
// rejects unknown keys, so a key added to the shared file without being
// added here fails the keep-core tests rather than being silently
// ignored.
//
//go:embed satoshi-units.json
var satoshiUnitsJSON []byte

// SatoshiUnitsVectorFile is the repository path of the embedded vector
// file, for error messages and for the tests that name the file.
const SatoshiUnitsVectorFile = "internal/testdata/reservations/satoshi-units.json"

// SatoshiUnitsVector is the parsed shared vector file.
//
// Every key the shared file carries is declared here, and the parser
// rejects unknown keys, so the two copies cannot silently drift apart
// on the key set. RoundingCases is the shared file's list of base-unit
// amounts that are not multiples of 1e10, with the truncated satoshi
// value and the remainder; keep-core does not compute a conversion of
// its own, so it carries the key without asserting on it.
type SatoshiUnitsVector struct {
	Vector   string `json:"vector"`
	Version  int    `json:"version"`
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

// SatoshiUnitsJSON returns the raw bytes of the shared vector file.
//
// The bytes are the source of truth for the strict parse below, so a
// test can apply its own decoding rules to the very same content.
func SatoshiUnitsJSON() []byte {
	return bytes.Clone(satoshiUnitsJSON)
}

// ParseSatoshiUnitsVector parses the shared vector file's bytes.
//
// The decode rejects unknown keys. The shared file is the contract
// between the repositories, so a key that this side does not know about
// is a sign that the two copies have drifted, and it must fail the
// caller rather than be silently ignored.
//
// NEGATIVE CONTROL: add any unknown top-level key to the vector file
// and this fails with a "json: unknown field" error.
func ParseSatoshiUnitsVector(raw []byte) (SatoshiUnitsVector, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var vector SatoshiUnitsVector
	if err := decoder.Decode(&vector); err != nil {
		return SatoshiUnitsVector{}, fmt.Errorf(
			"failed to parse the satoshi units vector file: %w",
			err,
		)
	}

	return vector, nil
}

// BaseUnitsPerSatoshi returns the conversion factor the vector itself
// states: how many TBTC base units one satoshi is.
//
// The factor is READ from the vector's defining pair rather than
// declared in Go, so a test that compares the vector against it is a
// check of the shared file's self-consistency and not a check of a
// test-local constant. The production unit handling is asserted
// separately, by driving the production consumers with the vector's
// own values.
func (v SatoshiUnitsVector) BaseUnitsPerSatoshi() (*big.Int, error) {
	for _, pair := range v.Pairs {
		if pair.Satoshis != 1 {
			continue
		}

		factor, ok := new(big.Int).SetString(pair.TbtcBaseUnits, 10)
		if !ok {
			return nil, fmt.Errorf(
				"failed to parse the vector's base units per satoshi [%s]",
				pair.TbtcBaseUnits,
			)
		}

		return factor, nil
	}

	return nil, fmt.Errorf(
		"the satoshi units vector carries no defining pair " +
			"(one satoshi in TBTC base units)",
	)
}

// UnitViolations returns the unit mismatches between a reading the
// production code returned and the reading the shared vector states.
//
// The two quantities that cross the unit boundary are read on different
// scales: the vault's in-kind fee DEBT is denominated in satoshi and the
// vault's fee RESERVE is a TBTC token balance in TBTC base units. A
// reader that returned either value on the other scale would produce a
// plausible-looking number in the wrong unit, so the comparison is an
// exact-equality check against the vector's own values.
//
// The helper returns the violations instead of failing the caller, so
// the negative control can require that the mis-scaled readings - the
// ones a unit error would produce - ARE rejected.
func UnitViolations(
	debtSat uint64,
	debtReading uint64,
	reserveUnits *big.Int,
	reserveReading *big.Int,
) []string {
	var violations []string

	if debtReading != debtSat {
		violations = append(
			violations,
			fmt.Sprintf(
				"expected the fee-debt reading to carry the vault's %d "+
					"satoshis unchanged, got %d",
				debtSat,
				debtReading,
			),
		)
	}

	switch {
	case reserveReading == nil:
		violations = append(
			violations,
			"expected a fee-reserve reading, got none",
		)
	case reserveReading.Cmp(reserveUnits) != 0:
		violations = append(
			violations,
			fmt.Sprintf(
				"expected the fee-reserve reading to carry the vault's %s "+
					"TBTC base units unchanged, got %s",
				reserveUnits.String(),
				reserveReading.String(),
			),
		)
	}

	return violations
}

// GaugeUnitViolations returns the unit mismatches between the gauge
// values the production publisher emitted and the vault readings it was
// driven with.
//
// The gauge API takes float64, and the publisher converts the big.Int
// reserve with Float64 before publishing it, so the comparison is made
// on the same float64 values the publisher produced: the expected gauge
// for a reading is that reading converted the way the publisher
// converts it.
//
// A publisher that scaled either reading would publish a plausible-
// looking number in the wrong unit, so the comparison is an exact-
// equality check.
func GaugeUnitViolations(
	debtSat uint64,
	debtGauge float64,
	reserveUnits *big.Int,
	reserveGauge float64,
) []string {
	var violations []string

	wantDebt := float64(debtSat)
	if debtGauge != wantDebt {
		violations = append(
			violations,
			fmt.Sprintf(
				"expected the fee-debt gauge to carry the vault's %d "+
					"satoshis unchanged, got %v",
				debtSat,
				debtGauge,
			),
		)
	}

	wantReserve, _ := reserveUnits.Float64()
	if reserveGauge != wantReserve {
		violations = append(
			violations,
			fmt.Sprintf(
				"expected the fee-reserve gauge to carry the vault's %s "+
					"TBTC base units unchanged, got %v",
				reserveUnits.String(),
				reserveGauge,
			),
		)
	}

	return violations
}

// MisScaledReadings returns the readings a unit error in a reader would
// produce for the given pair: the debt on the base-unit scale and the
// reserve on the satoshi scale.
//
// The factor is the scale between the two readings, so the mis-scaled
// debt is the debt multiplied by it and the mis-scaled reserve is the
// reserve divided by it. They are the negative control's input: the
// comparison that accepts the right-scale readings must reject these.
func MisScaledReadings(
	debtSat uint64,
	reserveUnits *big.Int,
) (debtReading uint64, reserveReading *big.Int) {
	factor := new(big.Int)
	if debtSat != 0 {
		factor.Div(reserveUnits, big.NewInt(int64(debtSat)))
	}

	debtReading = new(big.Int).Mul(
		big.NewInt(int64(debtSat)),
		factor,
	).Uint64()

	reserveReading = new(big.Int).Div(reserveUnits, factor)

	return debtReading, reserveReading
}
