package covenantsigner

import (
	"context"
	"strings"
	"testing"
)

func validRedeemDestination() *RedeemDestinationReservation {
	reservation := &RedeemDestinationReservation{
		ReservationID:   "crdr_12345678",
		Reserve:         "0x1111111111111111111111111111111111111111",
		Epoch:           12,
		Route:           ReservationRouteRedeem,
		Revealer:        "0x2222222222222222222222222222222222222222",
		Vault:           "0x3333333333333333333333333333333333333333",
		Network:         "regtest",
		Status:          ReservationStatusReserved,
		OutputScript:    "0x0014bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		OutputValueSats: 998000,
	}
	reservation.OutputScriptHash, _ = computeDepositScriptHash(reservation.OutputScript)
	reservation.DestinationCommitmentHash, _ = computeRedeemCommitmentHash(reservation)
	return reservation
}

func validRenewDestination() *RenewDestinationReservation {
	reservation := &RenewDestinationReservation{
		ReservationID:      "crnr_12345678",
		Reserve:            "0x1111111111111111111111111111111111111111",
		Epoch:              12,
		Route:              ReservationRouteRenew,
		Revealer:           "0x2222222222222222222222222222222222222222",
		Vault:              "0x3333333333333333333333333333333333333333",
		Network:            "regtest",
		Status:             ReservationStatusReserved,
		NextCovenantScript: "0x0014cccccccccccccccccccccccccccccccccccccccc",
		NextMaturityHeight: 987654,
		OutputValueSats:    998000,
	}
	reservation.NextCovenantScriptHash, _ = computeDepositScriptHash(reservation.NextCovenantScript)
	reservation.DestinationCommitmentHash, _ = computeRenewCommitmentHash(reservation)
	return reservation
}

// rebuildApprovals recomputes the plan commitment and artifact approvals after a
// request's destination/plan were mutated, keeping the request internally
// consistent (approval payload, signature, and legacy signatures).
func rebuildApprovals(request *RouteSubmitRequest) {
	request.MigrationTransactionPlan.PlanCommitmentHash, _ =
		computeMigrationTransactionPlanCommitmentHash(*request, request.MigrationTransactionPlan)
	request.ArtifactApprovals = validArtifactApprovals(*request)
	request.ArtifactSignatures = canonicalArtifactSignatures(
		request.Route,
		request.ArtifactApprovals,
	)
}

func redeemSelfV1Request(t *testing.T) RouteSubmitRequest {
	t.Helper()
	request := baseRequest(TemplateSelfV1)
	dest := validRedeemDestination()
	request.Action = CovenantActionRedeem
	request.MigrationDestination = nil
	request.RedeemDestination = dest
	request.DestinationCommitmentHash = dest.DestinationCommitmentHash
	request.MigrationTransactionPlan.DestinationValueSats = dest.OutputValueSats
	rebuildApprovals(&request)
	return request
}

func renewSelfV1Request(t *testing.T) RouteSubmitRequest {
	t.Helper()
	request := baseRequest(TemplateSelfV1)
	dest := validRenewDestination()
	request.Action = CovenantActionRenew
	request.MigrationDestination = nil
	request.RenewDestination = dest
	request.DestinationCommitmentHash = dest.DestinationCommitmentHash
	request.MigrationTransactionPlan.DestinationValueSats = dest.OutputValueSats
	rebuildApprovals(&request)
	return request
}

func submitRedeemRenew(t *testing.T, id string, request RouteSubmitRequest) error {
	t.Helper()
	service, err := NewService(newMemoryHandle(), &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: id,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	return err
}

func TestServiceAcceptsRedeemSelfV1(t *testing.T) {
	if err := submitRedeemRenew(t, "ors_redeem_ok", redeemSelfV1Request(t)); err != nil {
		t.Fatalf("expected redeem request to be accepted, got %v", err)
	}
}

func TestServiceAcceptsRenewSelfV1(t *testing.T) {
	if err := submitRedeemRenew(t, "ors_renew_ok", renewSelfV1Request(t)); err != nil {
		t.Fatalf("expected renew request to be accepted, got %v", err)
	}
}

// submitRedeemRenewWithMigrationPlanQuoteTrustRoots mirrors submitRedeemRenew
// but configures the service with migrationPlanQuoteTrustRoots, matching a
// realistic production deployment that also verifies MIGRATION plan quotes.
// REDEEM/RENEW requests have no plan-quote field and must remain unaffected by
// that configuration.
func submitRedeemRenewWithMigrationPlanQuoteTrustRoots(
	t *testing.T,
	id string,
	request RouteSubmitRequest,
) error {
	t.Helper()
	service, err := NewService(
		newMemoryHandle(),
		&scriptedEngine{},
		WithMigrationPlanQuoteTrustRoots([]MigrationPlanQuoteTrustRoot{
			testMigrationPlanQuoteTrustRoot,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: id,
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	return err
}

func TestServiceAcceptsRedeemSelfV1WithMigrationPlanQuoteTrustRootsConfigured(t *testing.T) {
	err := submitRedeemRenewWithMigrationPlanQuoteTrustRoots(
		t, "ors_redeem_trust_roots_configured", redeemSelfV1Request(t),
	)
	if err != nil {
		t.Fatalf(
			"expected redeem request to be accepted when migrationPlanQuoteTrustRoots are configured, got %v",
			err,
		)
	}
}

func TestServiceAcceptsRenewSelfV1WithMigrationPlanQuoteTrustRootsConfigured(t *testing.T) {
	err := submitRedeemRenewWithMigrationPlanQuoteTrustRoots(
		t, "ors_renew_trust_roots_configured", renewSelfV1Request(t),
	)
	if err != nil {
		t.Fatalf(
			"expected renew request to be accepted when migrationPlanQuoteTrustRoots are configured, got %v",
			err,
		)
	}
}

func TestServiceRejectsRedeemWithCommitmentMismatch(t *testing.T) {
	request := redeemSelfV1Request(t)
	// Tamper the output script without recomputing the commitment: the recomputed
	// canonical commitment no longer matches the pinned one.
	request.RedeemDestination.OutputScript = "0x0014dddddddddddddddddddddddddddddddddddddddd"
	request.RedeemDestination.OutputScriptHash, _ =
		computeDepositScriptHash(request.RedeemDestination.OutputScript)

	err := submitRedeemRenew(t, "ors_redeem_commitment_mismatch", request)
	if err == nil || !strings.Contains(err.Error(), "canonical reservation artifact") {
		t.Fatalf("expected redeem commitment mismatch error, got %v", err)
	}
}

func TestServiceRejectsRedeemWithValueMismatch(t *testing.T) {
	request := redeemSelfV1Request(t)
	// The built transaction would pay a value different from the committed one.
	request.MigrationTransactionPlan.DestinationValueSats =
		request.RedeemDestination.OutputValueSats - 1
	rebuildApprovals(&request)

	err := submitRedeemRenew(t, "ors_redeem_value_mismatch", request)
	if err == nil || !strings.Contains(err.Error(), "destinationValueSats") {
		t.Fatalf("expected redeem value mismatch error, got %v", err)
	}
}

func TestServiceRejectsCrossActionDestination(t *testing.T) {
	// A REDEEM request must not also carry a migration destination.
	request := redeemSelfV1Request(t)
	request.MigrationDestination = validMigrationDestination()

	err := submitRedeemRenew(t, "ors_redeem_cross_action", request)
	if err == nil || !strings.Contains(err.Error(), "must be omitted unless action is MIGRATION") {
		t.Fatalf("expected cross-action destination error, got %v", err)
	}
}

func TestServiceRejectsRedeemActionWithoutRedeemDestination(t *testing.T) {
	request := redeemSelfV1Request(t)
	request.RedeemDestination = nil

	err := submitRedeemRenew(t, "ors_redeem_missing_dest", request)
	if err == nil || !strings.Contains(err.Error(), "request.redeemDestination is required") {
		t.Fatalf("expected missing redeem destination error, got %v", err)
	}
}

func TestRedeemRenewCommitmentHashesAreDeterministic(t *testing.T) {
	redeem := validRedeemDestination()
	again, err := computeRedeemCommitmentHash(redeem)
	if err != nil {
		t.Fatal(err)
	}
	if again != redeem.DestinationCommitmentHash {
		t.Fatalf("redeem commitment not deterministic: %s vs %s", again, redeem.DestinationCommitmentHash)
	}

	renew := validRenewDestination()
	againRenew, err := computeRenewCommitmentHash(renew)
	if err != nil {
		t.Fatal(err)
	}
	if againRenew != renew.DestinationCommitmentHash {
		t.Fatalf("renew commitment not deterministic: %s vs %s", againRenew, renew.DestinationCommitmentHash)
	}

	// Redeem and renew commitments over the same identity must differ (distinct
	// route + destination fields).
	if redeem.DestinationCommitmentHash == renew.DestinationCommitmentHash {
		t.Fatal("redeem and renew commitments must not collide")
	}
}
