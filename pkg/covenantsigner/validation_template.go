package covenantsigner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"

	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
)

func computeMigrationExtraData(revealer string) string {
	return "0x" + hex.EncodeToString([]byte("AC_MIGRATEV1")) + strings.TrimPrefix(normalizeLowerHex(revealer), "0x")
}

func computeDepositScriptHash(depositScript string) (string, error) {
	rawScript, err := hex.DecodeString(strings.TrimPrefix(depositScript, "0x"))
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(rawScript)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

type destinationCommitmentPayload struct {
	// Field order is hash-significant and must stay aligned with the TypeScript
	// reservation-service object literal used to compute the same commitment.
	Reserve            string `json:"reserve"`
	Epoch              uint64 `json:"epoch"`
	Route              string `json:"route"`
	Revealer           string `json:"revealer"`
	Vault              string `json:"vault"`
	Network            string `json:"network"`
	DepositScriptHash  string `json:"depositScriptHash"`
	MigrationExtraData string `json:"migrationExtraData"`
}

type migrationPlanCommitmentPayload struct {
	// Field order is hash-significant and must stay aligned with the TypeScript
	// migration transaction-plan commitment payload. planCommitmentHash is
	// intentionally omitted because it is the output of this computation.
	PlanVersion               uint32 `json:"planVersion"`
	Reserve                   string `json:"reserve"`
	Epoch                     uint64 `json:"epoch"`
	ActiveOutpointTxID        string `json:"activeOutpointTxid"`
	ActiveOutpointVout        uint32 `json:"activeOutpointVout"`
	DestinationCommitmentHash string `json:"destinationCommitmentHash"`
	InputValueSats            uint64 `json:"inputValueSats"`
	DestinationValueSats      uint64 `json:"destinationValueSats"`
	AnchorValueSats           uint64 `json:"anchorValueSats"`
	FeeSats                   uint64 `json:"feeSats"`
	InputSequence             uint32 `json:"inputSequence"`
	LockTime                  uint32 `json:"lockTime"`
}

func computeDestinationCommitmentHash(
	reservation *MigrationDestinationReservation,
) (string, error) {
	payload, err := canonicaljson.Marshal(destinationCommitmentPayload{
		Reserve:            normalizeLowerHex(reservation.Reserve),
		Epoch:              reservation.Epoch,
		Route:              string(reservation.Route),
		Revealer:           normalizeLowerHex(reservation.Revealer),
		Vault:              normalizeLowerHex(reservation.Vault),
		Network:            strings.TrimSpace(reservation.Network),
		DepositScriptHash:  normalizeLowerHex(reservation.DepositScriptHash),
		MigrationExtraData: normalizeLowerHex(reservation.MigrationExtraData),
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

// redeemCommitmentPayload is the canonical-JSON preimage of a REDEEM destination
// commitment. Field order is hash-significant and must stay aligned with the
// client (covenant-manager / dashboard / verification kit) object literal.
type redeemCommitmentPayload struct {
	Reserve          string `json:"reserve"`
	Epoch            uint64 `json:"epoch"`
	Route            string `json:"route"`
	Revealer         string `json:"revealer"`
	Vault            string `json:"vault"`
	Network          string `json:"network"`
	OutputScriptHash string `json:"outputScriptHash"`
	OutputValueSats  uint64 `json:"outputValueSats"`
}

// renewCommitmentPayload is the canonical-JSON preimage of a RENEW destination
// commitment. Field order is hash-significant and must stay aligned with the
// client object literal.
type renewCommitmentPayload struct {
	Reserve                string `json:"reserve"`
	Epoch                  uint64 `json:"epoch"`
	Route                  string `json:"route"`
	Revealer               string `json:"revealer"`
	Vault                  string `json:"vault"`
	Network                string `json:"network"`
	NextCovenantScriptHash string `json:"nextCovenantScriptHash"`
	NextMaturityHeight     uint64 `json:"nextMaturityHeight"`
	OutputValueSats        uint64 `json:"outputValueSats"`
}

// computeRedeemCommitmentHash is the REDEEM analogue of
// computeDestinationCommitmentHash: SHA-256 over the canonical-JSON preimage.
func computeRedeemCommitmentHash(
	reservation *RedeemDestinationReservation,
) (string, error) {
	payload, err := canonicaljson.Marshal(redeemCommitmentPayload{
		Reserve:          normalizeLowerHex(reservation.Reserve),
		Epoch:            reservation.Epoch,
		Route:            string(reservation.Route),
		Revealer:         normalizeLowerHex(reservation.Revealer),
		Vault:            normalizeLowerHex(reservation.Vault),
		Network:          strings.TrimSpace(reservation.Network),
		OutputScriptHash: normalizeLowerHex(reservation.OutputScriptHash),
		OutputValueSats:  reservation.OutputValueSats,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

// computeRenewCommitmentHash is the RENEW analogue of
// computeDestinationCommitmentHash: SHA-256 over the canonical-JSON preimage.
func computeRenewCommitmentHash(
	reservation *RenewDestinationReservation,
) (string, error) {
	payload, err := canonicaljson.Marshal(renewCommitmentPayload{
		Reserve:                normalizeLowerHex(reservation.Reserve),
		Epoch:                  reservation.Epoch,
		Route:                  string(reservation.Route),
		Revealer:               normalizeLowerHex(reservation.Revealer),
		Vault:                  normalizeLowerHex(reservation.Vault),
		Network:                strings.TrimSpace(reservation.Network),
		NextCovenantScriptHash: normalizeLowerHex(reservation.NextCovenantScriptHash),
		NextMaturityHeight:     reservation.NextMaturityHeight,
		OutputValueSats:        reservation.OutputValueSats,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func computeMigrationTransactionPlanCommitmentHash(
	request RouteSubmitRequest,
	plan *MigrationTransactionPlan,
) (string, error) {
	payload, err := canonicaljson.Marshal(migrationPlanCommitmentPayload{
		PlanVersion:               plan.PlanVersion,
		Reserve:                   normalizeLowerHex(request.Reserve),
		Epoch:                     request.Epoch,
		ActiveOutpointTxID:        normalizeLowerHex(request.ActiveOutpoint.TxID),
		ActiveOutpointVout:        request.ActiveOutpoint.Vout,
		DestinationCommitmentHash: normalizeLowerHex(request.DestinationCommitmentHash),
		InputValueSats:            plan.InputValueSats,
		DestinationValueSats:      plan.DestinationValueSats,
		AnchorValueSats:           plan.AnchorValueSats,
		FeeSats:                   plan.FeeSats,
		InputSequence:             plan.InputSequence,
		LockTime:                  plan.LockTime,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func validateMigrationDestination(
	request RouteSubmitRequest,
	reservation *MigrationDestinationReservation,
) error {
	if reservation == nil {
		return &inputError{"request.migrationDestination is required"}
	}
	if reservation.Route != ReservationRouteMigration {
		return &inputError{"request.migrationDestination.route must be MIGRATION"}
	}
	if reservation.Status != ReservationStatusReserved &&
		reservation.Status != ReservationStatusCommittedToEpoch {
		return &inputError{"request.migrationDestination.status must be RESERVED or COMMITTED_TO_EPOCH"}
	}
	if err := validateAddressString("request.migrationDestination.reserve", reservation.Reserve); err != nil {
		return err
	}
	if err := validateAddressString("request.migrationDestination.revealer", reservation.Revealer); err != nil {
		return err
	}
	if err := validateAddressString("request.migrationDestination.vault", reservation.Vault); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.Network) == "" {
		return &inputError{"request.migrationDestination.network is required"}
	}
	if err := validateHexString("request.migrationDestination.depositScript", reservation.DepositScript); err != nil {
		return err
	}
	if err := validateHexString("request.migrationDestination.depositScriptHash", reservation.DepositScriptHash); err != nil {
		return err
	}
	if err := validateHexString("request.migrationDestination.migrationExtraData", reservation.MigrationExtraData); err != nil {
		return err
	}
	if err := validateHexString("request.migrationDestination.destinationCommitmentHash", reservation.DestinationCommitmentHash); err != nil {
		return err
	}
	if request.Epoch != reservation.Epoch {
		return &inputError{"request.migrationDestination.epoch does not match request.epoch"}
	}
	if normalizeLowerHex(request.Reserve) != normalizeLowerHex(reservation.Reserve) {
		return &inputError{"request.migrationDestination.reserve does not match request.reserve"}
	}
	if normalizeLowerHex(request.DestinationCommitmentHash) != normalizeLowerHex(reservation.DestinationCommitmentHash) {
		return &inputError{"request.migrationDestination.destinationCommitmentHash does not match request.destinationCommitmentHash"}
	}

	expectedExtraData := computeMigrationExtraData(reservation.Revealer)
	if normalizeLowerHex(reservation.MigrationExtraData) != expectedExtraData {
		return &inputError{"request.migrationDestination.migrationExtraData does not match migration revealer encoding"}
	}

	depositScriptHash, err := computeDepositScriptHash(reservation.DepositScript)
	if err != nil {
		return &inputError{"request.migrationDestination.depositScript is not valid hex"}
	}
	if normalizeLowerHex(reservation.DepositScriptHash) != depositScriptHash {
		return &inputError{"request.migrationDestination.depositScriptHash does not match depositScript"}
	}

	commitmentHash, err := computeDestinationCommitmentHash(reservation)
	if err != nil {
		return err
	}
	if normalizeLowerHex(reservation.DestinationCommitmentHash) != commitmentHash {
		return &inputError{"request.migrationDestination.destinationCommitmentHash does not match canonical reservation artifact"}
	}

	return nil
}

// validateActionDestination dispatches destination validation on the request's
// covenant action and rejects destinations that do not belong to that action.
func validateActionDestination(request RouteSubmitRequest) error {
	action := request.ResolvedAction()
	if action != CovenantActionMigration && request.MigrationDestination != nil {
		return &inputError{"request.migrationDestination must be omitted unless action is MIGRATION"}
	}
	if action != CovenantActionRedeem && request.RedeemDestination != nil {
		return &inputError{"request.redeemDestination must be omitted unless action is REDEEM"}
	}
	if action != CovenantActionRenew && request.RenewDestination != nil {
		return &inputError{"request.renewDestination must be omitted unless action is RENEW"}
	}

	switch action {
	case CovenantActionMigration:
		return validateMigrationDestination(request, request.MigrationDestination)
	case CovenantActionRedeem:
		return validateRedeemDestination(request, request.RedeemDestination)
	case CovenantActionRenew:
		return validateRenewDestination(request, request.RenewDestination)
	default:
		return &inputError{"request.action is not supported"}
	}
}

func validateRedeemDestination(
	request RouteSubmitRequest,
	reservation *RedeemDestinationReservation,
) error {
	if reservation == nil {
		return &inputError{"request.redeemDestination is required"}
	}
	if reservation.Route != ReservationRouteRedeem {
		return &inputError{"request.redeemDestination.route must be REDEEM"}
	}
	if reservation.Status != ReservationStatusReserved &&
		reservation.Status != ReservationStatusCommittedToEpoch {
		return &inputError{"request.redeemDestination.status must be RESERVED or COMMITTED_TO_EPOCH"}
	}
	if err := validateAddressString("request.redeemDestination.reserve", reservation.Reserve); err != nil {
		return err
	}
	if err := validateAddressString("request.redeemDestination.revealer", reservation.Revealer); err != nil {
		return err
	}
	if err := validateAddressString("request.redeemDestination.vault", reservation.Vault); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.Network) == "" {
		return &inputError{"request.redeemDestination.network is required"}
	}
	if err := validateHexString("request.redeemDestination.outputScript", reservation.OutputScript); err != nil {
		return err
	}
	if err := validateHexString("request.redeemDestination.outputScriptHash", reservation.OutputScriptHash); err != nil {
		return err
	}
	if err := validateHexString("request.redeemDestination.destinationCommitmentHash", reservation.DestinationCommitmentHash); err != nil {
		return err
	}
	if reservation.OutputValueSats == 0 {
		return &inputError{"request.redeemDestination.outputValueSats must be greater than zero"}
	}
	if request.Epoch != reservation.Epoch {
		return &inputError{"request.redeemDestination.epoch does not match request.epoch"}
	}
	if normalizeLowerHex(request.Reserve) != normalizeLowerHex(reservation.Reserve) {
		return &inputError{"request.redeemDestination.reserve does not match request.reserve"}
	}
	if normalizeLowerHex(request.DestinationCommitmentHash) != normalizeLowerHex(reservation.DestinationCommitmentHash) {
		return &inputError{"request.redeemDestination.destinationCommitmentHash does not match request.destinationCommitmentHash"}
	}

	outputScriptHash, err := computeDepositScriptHash(reservation.OutputScript)
	if err != nil {
		return &inputError{"request.redeemDestination.outputScript is not valid hex"}
	}
	if normalizeLowerHex(reservation.OutputScriptHash) != outputScriptHash {
		return &inputError{"request.redeemDestination.outputScriptHash does not match outputScript"}
	}

	commitmentHash, err := computeRedeemCommitmentHash(reservation)
	if err != nil {
		return err
	}
	if normalizeLowerHex(reservation.DestinationCommitmentHash) != commitmentHash {
		return &inputError{"request.redeemDestination.destinationCommitmentHash does not match canonical reservation artifact"}
	}

	// The built transaction pays MigrationTransactionPlan.DestinationValueSats;
	// bind it to the committed output value so the signed output cannot diverge
	// from what the depositor's artifact approval authorized.
	if request.MigrationTransactionPlan != nil &&
		request.MigrationTransactionPlan.DestinationValueSats != reservation.OutputValueSats {
		return &inputError{"request.redeemDestination.outputValueSats does not match request.migrationTransactionPlan.destinationValueSats"}
	}

	return nil
}

func validateRenewDestination(
	request RouteSubmitRequest,
	reservation *RenewDestinationReservation,
) error {
	if reservation == nil {
		return &inputError{"request.renewDestination is required"}
	}
	if reservation.Route != ReservationRouteRenew {
		return &inputError{"request.renewDestination.route must be RENEW"}
	}
	if reservation.Status != ReservationStatusReserved &&
		reservation.Status != ReservationStatusCommittedToEpoch {
		return &inputError{"request.renewDestination.status must be RESERVED or COMMITTED_TO_EPOCH"}
	}
	if err := validateAddressString("request.renewDestination.reserve", reservation.Reserve); err != nil {
		return err
	}
	if err := validateAddressString("request.renewDestination.revealer", reservation.Revealer); err != nil {
		return err
	}
	if err := validateAddressString("request.renewDestination.vault", reservation.Vault); err != nil {
		return err
	}
	if strings.TrimSpace(reservation.Network) == "" {
		return &inputError{"request.renewDestination.network is required"}
	}
	if err := validateHexString("request.renewDestination.nextCovenantScript", reservation.NextCovenantScript); err != nil {
		return err
	}
	if err := validateHexString("request.renewDestination.nextCovenantScriptHash", reservation.NextCovenantScriptHash); err != nil {
		return err
	}
	if err := validateHexString("request.renewDestination.destinationCommitmentHash", reservation.DestinationCommitmentHash); err != nil {
		return err
	}
	if reservation.NextMaturityHeight == 0 {
		return &inputError{"request.renewDestination.nextMaturityHeight must be greater than zero"}
	}
	if reservation.OutputValueSats == 0 {
		return &inputError{"request.renewDestination.outputValueSats must be greater than zero"}
	}
	if request.Epoch != reservation.Epoch {
		return &inputError{"request.renewDestination.epoch does not match request.epoch"}
	}
	if normalizeLowerHex(request.Reserve) != normalizeLowerHex(reservation.Reserve) {
		return &inputError{"request.renewDestination.reserve does not match request.reserve"}
	}
	if normalizeLowerHex(request.DestinationCommitmentHash) != normalizeLowerHex(reservation.DestinationCommitmentHash) {
		return &inputError{"request.renewDestination.destinationCommitmentHash does not match request.destinationCommitmentHash"}
	}

	nextCovenantScriptHash, err := computeDepositScriptHash(reservation.NextCovenantScript)
	if err != nil {
		return &inputError{"request.renewDestination.nextCovenantScript is not valid hex"}
	}
	if normalizeLowerHex(reservation.NextCovenantScriptHash) != nextCovenantScriptHash {
		return &inputError{"request.renewDestination.nextCovenantScriptHash does not match nextCovenantScript"}
	}

	commitmentHash, err := computeRenewCommitmentHash(reservation)
	if err != nil {
		return err
	}
	if normalizeLowerHex(reservation.DestinationCommitmentHash) != commitmentHash {
		return &inputError{"request.renewDestination.destinationCommitmentHash does not match canonical reservation artifact"}
	}

	// The built transaction pays MigrationTransactionPlan.DestinationValueSats;
	// bind it to the committed re-locked value so the signed output cannot diverge
	// from what the depositor's artifact approval authorized.
	if request.MigrationTransactionPlan != nil &&
		request.MigrationTransactionPlan.DestinationValueSats != reservation.OutputValueSats {
		return &inputError{"request.renewDestination.outputValueSats does not match request.migrationTransactionPlan.destinationValueSats"}
	}

	return nil
}

func validateMigrationTransactionPlan(
	request RouteSubmitRequest,
	plan *MigrationTransactionPlan,
) error {
	if plan == nil {
		return &inputError{"request.migrationTransactionPlan is required"}
	}
	if plan.PlanVersion != migrationTransactionPlanVersion {
		return &inputError{"request.migrationTransactionPlan.planVersion must equal 1"}
	}
	if err := validateHexString("request.migrationTransactionPlan.planCommitmentHash", plan.PlanCommitmentHash); err != nil {
		return err
	}
	if plan.InputValueSats == 0 {
		return &inputError{"request.migrationTransactionPlan.inputValueSats must be greater than zero"}
	}
	if plan.DestinationValueSats == 0 {
		return &inputError{"request.migrationTransactionPlan.destinationValueSats must be greater than zero"}
	}
	if plan.FeeSats == 0 {
		return &inputError{"request.migrationTransactionPlan.feeSats must be greater than zero"}
	}
	if plan.AnchorValueSats != canonicalAnchorValueSats {
		return &inputError{"request.migrationTransactionPlan.anchorValueSats must equal the canonical 330 sat anchor"}
	}
	if plan.InputSequence != canonicalCovenantInputSequence {
		return &inputError{"request.migrationTransactionPlan.inputSequence must equal 0xFFFFFFFD"}
	}
	if request.MaturityHeight > math.MaxUint32 {
		return &inputError{"request.maturityHeight must fit in uint32"}
	}
	if uint64(plan.LockTime) != request.MaturityHeight {
		return &inputError{"request.migrationTransactionPlan.lockTime must match request.maturityHeight"}
	}
	if plan.InputValueSats < plan.DestinationValueSats {
		return &inputError{"request.migrationTransactionPlan.inputValueSats must cover destinationValueSats"}
	}
	remainingAfterDestination := plan.InputValueSats - plan.DestinationValueSats
	if remainingAfterDestination < plan.AnchorValueSats {
		return &inputError{"request.migrationTransactionPlan.inputValueSats must cover anchorValueSats"}
	}
	remainingAfterAnchor := remainingAfterDestination - plan.AnchorValueSats
	if remainingAfterAnchor != plan.FeeSats {
		return &inputError{"request.migrationTransactionPlan values must satisfy inputValueSats = destinationValueSats + anchorValueSats + feeSats"}
	}

	expectedCommitmentHash, err := computeMigrationTransactionPlanCommitmentHash(request, plan)
	if err != nil {
		return err
	}
	if normalizeLowerHex(plan.PlanCommitmentHash) != expectedCommitmentHash {
		return &inputError{"request.migrationTransactionPlan.planCommitmentHash does not match canonical migration transaction plan"}
	}

	return nil
}

func normalizeMigrationDestination(
	destination *MigrationDestinationReservation,
) *MigrationDestinationReservation {
	if destination == nil {
		return nil
	}

	return &MigrationDestinationReservation{
		ReservationID:             strings.TrimSpace(destination.ReservationID),
		Reserve:                   normalizeLowerHex(destination.Reserve),
		Epoch:                     destination.Epoch,
		Route:                     destination.Route,
		Revealer:                  normalizeLowerHex(destination.Revealer),
		Vault:                     normalizeLowerHex(destination.Vault),
		Network:                   strings.TrimSpace(destination.Network),
		Status:                    destination.Status,
		DepositScript:             normalizeLowerHex(destination.DepositScript),
		DepositScriptHash:         normalizeLowerHex(destination.DepositScriptHash),
		MigrationExtraData:        normalizeLowerHex(destination.MigrationExtraData),
		DestinationCommitmentHash: normalizeLowerHex(destination.DestinationCommitmentHash),
	}
}

func normalizeRedeemDestination(
	destination *RedeemDestinationReservation,
) *RedeemDestinationReservation {
	if destination == nil {
		return nil
	}

	return &RedeemDestinationReservation{
		ReservationID:             strings.TrimSpace(destination.ReservationID),
		Reserve:                   normalizeLowerHex(destination.Reserve),
		Epoch:                     destination.Epoch,
		Route:                     destination.Route,
		Revealer:                  normalizeLowerHex(destination.Revealer),
		Vault:                     normalizeLowerHex(destination.Vault),
		Network:                   strings.TrimSpace(destination.Network),
		Status:                    destination.Status,
		OutputScript:              normalizeLowerHex(destination.OutputScript),
		OutputScriptHash:          normalizeLowerHex(destination.OutputScriptHash),
		OutputValueSats:           destination.OutputValueSats,
		DestinationCommitmentHash: normalizeLowerHex(destination.DestinationCommitmentHash),
	}
}

func normalizeRenewDestination(
	destination *RenewDestinationReservation,
) *RenewDestinationReservation {
	if destination == nil {
		return nil
	}

	return &RenewDestinationReservation{
		ReservationID:             strings.TrimSpace(destination.ReservationID),
		Reserve:                   normalizeLowerHex(destination.Reserve),
		Epoch:                     destination.Epoch,
		Route:                     destination.Route,
		Revealer:                  normalizeLowerHex(destination.Revealer),
		Vault:                     normalizeLowerHex(destination.Vault),
		Network:                   strings.TrimSpace(destination.Network),
		Status:                    destination.Status,
		NextCovenantScript:        normalizeLowerHex(destination.NextCovenantScript),
		NextCovenantScriptHash:    normalizeLowerHex(destination.NextCovenantScriptHash),
		NextMaturityHeight:        destination.NextMaturityHeight,
		OutputValueSats:           destination.OutputValueSats,
		DestinationCommitmentHash: normalizeLowerHex(destination.DestinationCommitmentHash),
	}
}

func normalizeMigrationTransactionPlan(
	plan *MigrationTransactionPlan,
) *MigrationTransactionPlan {
	if plan == nil {
		return nil
	}

	return &MigrationTransactionPlan{
		PlanVersion:          plan.PlanVersion,
		PlanCommitmentHash:   normalizeLowerHex(plan.PlanCommitmentHash),
		InputValueSats:       plan.InputValueSats,
		DestinationValueSats: plan.DestinationValueSats,
		AnchorValueSats:      plan.AnchorValueSats,
		FeeSats:              plan.FeeSats,
		InputSequence:        plan.InputSequence,
		LockTime:             plan.LockTime,
	}
}

// migrationTransactionPlansEqual compares two normalized MigrationTransactionPlan
// values for equality without using reflect.DeepEqual.
func migrationTransactionPlansEqual(a, b *MigrationTransactionPlan) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.PlanVersion == b.PlanVersion &&
		a.PlanCommitmentHash == b.PlanCommitmentHash &&
		a.InputValueSats == b.InputValueSats &&
		a.DestinationValueSats == b.DestinationValueSats &&
		a.AnchorValueSats == b.AnchorValueSats &&
		a.FeeSats == b.FeeSats &&
		a.InputSequence == b.InputSequence &&
		a.LockTime == b.LockTime
}

func normalizeScriptTemplate(route TemplateID, rawTemplate json.RawMessage) (json.RawMessage, error) {
	switch route {
	case TemplateSelfV1:
		template := &SelfV1Template{}
		if err := strictUnmarshal(rawTemplate, template); err != nil {
			return nil, err
		}
		template.DepositorPublicKey = normalizeLowerHex(template.DepositorPublicKey)
		template.SignerPublicKey = normalizeLowerHex(template.SignerPublicKey)
		return json.Marshal(template)
	case TemplateQcV1:
		template := &QcV1Template{}
		if err := strictUnmarshal(rawTemplate, template); err != nil {
			return nil, err
		}
		template.DepositorPublicKey = normalizeLowerHex(template.DepositorPublicKey)
		template.CustodianPublicKey = normalizeLowerHex(template.CustodianPublicKey)
		template.SignerPublicKey = normalizeLowerHex(template.SignerPublicKey)
		return json.Marshal(template)
	default:
		return nil, &inputError{"unsupported request.route"}
	}
}
