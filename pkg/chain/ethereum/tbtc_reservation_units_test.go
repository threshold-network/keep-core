package ethereum

// This file drives the shared satoshi-to-TBTC-base-unit test vector
// through the production ReservationVault fee adapters, so the unit
// boundary is asserted on the values the adapters actually return.
//
//   - ReservationVaultFeeDebtSat returns the vault's in-kind fee debt in
//     SATOSHI.
//   - ReservationVaultFeeReserveTbtcBaseUnits returns the vault's TBTC fee
//     reserve in TBTC BASE UNITS.
//
// The two are related by the shared vector's factor (one satoshi is 10^10
// TBTC base units), and each adapter must return its own reading on its
// own scale: a unit error in either adapter is a wrong number on the
// gauge that reports it, so the adapters are driven with the vector's
// non-zero pairs and required to return the readings unchanged.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/internal/testdata/reservations"
	"github.com/keep-network/keep-core/pkg/chain"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// TestReservationVaultFeeAdaptersKeepTheVectorUnits drives the shared
// cross-repo vector's non-zero pairs through the production fee
// adapters.
//
// The vault's in-kind fee debt is denominated in satoshi and its fee
// reserve is a TBTC token balance in TBTC base units, so the two
// readings are related by exactly the shared vector's factor: a debt of
// one satoshi corresponds to a base-unit obligation of 10^10.
//
// Each adapter must return its reading on its own scale. The assertions
// are therefore exact-equality checks against the vector's own values:
// an adapter that converted the debt to base units, or the reserve to
// satoshi, would return a different number and fail here.
//
// The gauge publisher on the other side of the boundary (pkg/tbtcpg)
// drives the same vector through the production publisher, so the
// factor is asserted at both ends of the boundary.
//
// NEGATIVE CONTROL: the negative-control subtest registers a vault
// whose readings are on the OTHER scale (the debt on the base-unit scale
// and the reserve on the satoshi scale) and requires the same comparison
// to reject them. An adapter that scaled its reading would produce
// exactly that, and the positive assertions above would fail.
func TestReservationVaultFeeAdaptersKeepTheVectorUnits(t *testing.T) {
	vector, err := reservations.ParseSatoshiUnitsVector(
		reservations.SatoshiUnitsJSON(),
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	factor, err := vector.BaseUnitsPerSatoshi()
	if err != nil {
		t.Fatalf("%v", err)
	}

	routerAddress := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	vaultAddress := common.HexToAddress("0x00000000000000000000000000000000000000b2")
	tokenAddress := common.HexToAddress("0x00000000000000000000000000000000000000b3")

	vaultABI := parseGenABI(t, tbtcabi.ReservationVaultABI)

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

			// The vector's own relationship, checked against the
			// factor the vector's defining pair states: the debt on
			// the satoshi scale and the reserve on the base-unit
			// scale are one factor apart.
			wantReserve := new(big.Int).Mul(big.NewInt(int64(debtSat)), factor)
			if wantReserve.Cmp(reserveUnits) != 0 {
				t.Fatalf(
					"the vector's pair is inconsistent with its own "+
						"factor: %d satoshis should be %s base units, "+
						"the vector says %s",
					debtSat,
					wantReserve.String(),
					reserveUnits.String(),
				)
			}

			client := newReservationFakeClient(t)
			client.setView(
				t,
				vaultAddress,
				methodSelector(t, vaultABI, "inKindFeeDebtSat"),
				func() []byte {
					return packOutputs(t, vaultABI, "inKindFeeDebtSat", debtSat)
				},
			)
			client.setView(
				t,
				vaultAddress,
				methodSelector(t, vaultABI, "tbtcToken"),
				func() []byte {
					return packOutputs(t, vaultABI, "tbtcToken", tokenAddress)
				},
			)
			client.balances[vaultAddress] = reserveUnits

			vaultArg := chain.Address(vaultAddress.Hex())
			tbtcChain := newReservationGaugeChain(t, client, routerAddress)

			debt, err := tbtcChain.ReservationVaultFeeDebtSat(vaultArg)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			reserve, err :=
				tbtcChain.ReservationVaultFeeReserveTbtcBaseUnits(vaultArg)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			violations := reservations.UnitViolations(
				debtSat,
				debt,
				reserveUnits,
				reserve,
			)
			for _, violation := range violations {
				t.Error(violation)
			}

			// NEGATIVE CONTROL: the readings a unit error in an
			// adapter would produce - the debt on the base-unit
			// scale, the reserve on the satoshi scale - must be
			// rejected by the same comparison.
			misScaledDebt, misScaledReserve :=
				reservations.MisScaledReadings(debtSat, reserveUnits)
			misScaledViolations := reservations.UnitViolations(
				debtSat,
				misScaledDebt,
				reserveUnits,
				misScaledReserve,
			)
			if len(misScaledViolations) == 0 {
				t.Fatal(
					"expected the comparison to reject a fee-debt " +
						"reading on the base-unit scale and a " +
						"fee-reserve reading on the satoshi scale",
				)
			}

			// The same readings, driven through the PRODUCTION
			// adapter: a vault whose on-chain values are on the
			// wrong scale must produce readings the comparison
			// rejects. This is the control that shows the
			// comparison above is not vacuous: the adapter really
			// does return whatever the vault reports, so a
			// mis-scaled vault is a mis-scaled reading.
			misScaledClient := newReservationFakeClient(t)
			misScaledClient.setView(
				t,
				vaultAddress,
				methodSelector(t, vaultABI, "inKindFeeDebtSat"),
				func() []byte {
					return packOutputs(
						t,
						vaultABI,
						"inKindFeeDebtSat",
						misScaledDebt,
					)
				},
			)
			misScaledClient.setView(
				t,
				vaultAddress,
				methodSelector(t, vaultABI, "tbtcToken"),
				func() []byte {
					return packOutputs(t, vaultABI, "tbtcToken", tokenAddress)
				},
			)
			misScaledClient.balances[vaultAddress] = misScaledReserve

			misScaledChain :=
				newReservationGaugeChain(t, misScaledClient, routerAddress)

			gotDebt, err :=
				misScaledChain.ReservationVaultFeeDebtSat(vaultArg)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			gotReserve, err :=
				misScaledChain.ReservationVaultFeeReserveTbtcBaseUnits(vaultArg)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			gotViolations := reservations.UnitViolations(
				debtSat,
				gotDebt,
				reserveUnits,
				gotReserve,
			)
			if len(gotViolations) == 0 {
				t.Fatal(
					"expected the comparison to reject the mis-scaled " +
						"vault's readings",
				)
			}
		})
	}
}
