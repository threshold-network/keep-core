package covenantsigner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	canonicalCovenantInputSequence  uint32 = 0xFFFFFFFD
	canonicalAnchorValueSats        uint64 = 330
	migrationTransactionPlanVersion uint32 = 1
	artifactApprovalVersion         uint32 = 1
)

type inputError struct {
	message string
}

func (ie *inputError) Error() string {
	return ie.message
}

func strictUnmarshal(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// requestDigest accepts raw requests because Poll validates equivalence against
// whatever the caller resubmits. Submit should use requestDigestFromNormalized
// after it has already normalized the request once for storage.
func requestDigest(request RouteSubmitRequest) (string, error) {
	normalizedRequest, err := normalizeRouteSubmitRequest(request)
	if err != nil {
		return "", err
	}

	return requestDigestFromNormalized(normalizedRequest)
}

func requestDigestFromNormalized(request RouteSubmitRequest) (string, error) {
	payload, err := marshalCanonicalJSON(request)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

func validateHexString(name string, value string) error {
	if !strings.HasPrefix(value, "0x") || len(value) <= 2 || len(value)%2 != 0 {
		return &inputError{fmt.Sprintf("%s must be a 0x-prefixed even-length hex string", name)}
	}

	if _, err := hex.DecodeString(strings.TrimPrefix(value, "0x")); err != nil {
		return &inputError{fmt.Sprintf("%s must be valid hex", name)}
	}

	return nil
}

func validateAddressString(name string, value string) error {
	if err := validateHexString(name, value); err != nil {
		return err
	}

	if len(value) != 42 {
		return &inputError{fmt.Sprintf("%s must be a 20-byte 0x-prefixed hex address", name)}
	}

	return nil
}

func normalizeLowerHex(value string) string {
	return strings.ToLower(value)
}

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
	payload, err := json.Marshal(destinationCommitmentPayload{
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

func computeMigrationTransactionPlanCommitmentHash(
	request RouteSubmitRequest,
	plan *MigrationTransactionPlan,
) (string, error) {
	payload, err := json.Marshal(migrationPlanCommitmentPayload{
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

func validateArtifactSignatures(signatures []string) ([]string, error) {
	if len(signatures) == 0 {
		return nil, &inputError{"request.artifactSignatures must not be empty"}
	}

	normalizedSignatures := make([]string, len(signatures))
	for i, signature := range signatures {
		if err := validateHexString(
			fmt.Sprintf("request.artifactSignatures[%d]", i),
			signature,
		); err != nil {
			return nil, err
		}

		normalizedSignatures[i] = normalizeLowerHex(signature)
	}

	return normalizedSignatures, nil
}

func requiredArtifactApprovalRoles(route TemplateID) ([]ArtifactApprovalRole, error) {
	switch route {
	case TemplateQcV1:
		return []ArtifactApprovalRole{
			ArtifactApprovalRoleDepositor,
			ArtifactApprovalRoleCustodian,
			ArtifactApprovalRoleSigner,
		}, nil
	case TemplateSelfV1:
		return []ArtifactApprovalRole{
			ArtifactApprovalRoleDepositor,
			ArtifactApprovalRoleSigner,
		}, nil
	default:
		return nil, &inputError{"unsupported request.route"}
	}
}

func validateArtifactApprovals(route TemplateID, request RouteSubmitRequest) error {
	_, _, err := normalizeArtifactApprovals(route, request)
	return err
}

func normalizeArtifactApprovals(
	route TemplateID,
	request RouteSubmitRequest,
) (*ArtifactApprovalEnvelope, []string, error) {
	normalizedLegacySignatures, err := validateArtifactSignatures(request.ArtifactSignatures)
	if err != nil {
		return nil, nil, err
	}

	if request.ArtifactApprovals == nil {
		return nil, normalizedLegacySignatures, nil
	}
	if request.MigrationTransactionPlan == nil {
		return nil, nil, &inputError{"request.migrationTransactionPlan is required when request.artifactApprovals is present"}
	}

	if request.ArtifactApprovals.Payload.ApprovalVersion != artifactApprovalVersion {
		return nil, nil, &inputError{"request.artifactApprovals.payload.approvalVersion must equal 1"}
	}
	if request.ArtifactApprovals.Payload.Route != route {
		return nil, nil, &inputError{"request.artifactApprovals.payload.route must match request.route"}
	}
	if request.ArtifactApprovals.Payload.ScriptTemplateID != route {
		return nil, nil, &inputError{"request.artifactApprovals.payload.scriptTemplateId must match request.route"}
	}
	if err := validateHexString(
		"request.artifactApprovals.payload.destinationCommitmentHash",
		request.ArtifactApprovals.Payload.DestinationCommitmentHash,
	); err != nil {
		return nil, nil, err
	}
	if err := validateHexString(
		"request.artifactApprovals.payload.planCommitmentHash",
		request.ArtifactApprovals.Payload.PlanCommitmentHash,
	); err != nil {
		return nil, nil, err
	}

	normalizedDestinationCommitmentHash := normalizeLowerHex(
		request.ArtifactApprovals.Payload.DestinationCommitmentHash,
	)
	if normalizedDestinationCommitmentHash != normalizeLowerHex(request.DestinationCommitmentHash) {
		return nil, nil, &inputError{"request.artifactApprovals.payload.destinationCommitmentHash must match request.destinationCommitmentHash"}
	}

	normalizedPlanCommitmentHash := normalizeLowerHex(
		request.ArtifactApprovals.Payload.PlanCommitmentHash,
	)
	if normalizedPlanCommitmentHash != normalizeLowerHex(request.MigrationTransactionPlan.PlanCommitmentHash) {
		return nil, nil, &inputError{"request.artifactApprovals.payload.planCommitmentHash must match request.migrationTransactionPlan.planCommitmentHash"}
	}
	if len(request.ArtifactApprovals.Approvals) == 0 {
		return nil, nil, &inputError{"request.artifactApprovals.approvals must not be empty"}
	}

	requiredRoles, err := requiredArtifactApprovalRoles(route)
	if err != nil {
		return nil, nil, err
	}

	allowedRoles := make(map[ArtifactApprovalRole]struct{}, len(requiredRoles))
	for _, role := range requiredRoles {
		allowedRoles[role] = struct{}{}
	}

	approvalsByRole := make(map[ArtifactApprovalRole]string, len(requiredRoles))
	for i, approval := range request.ArtifactApprovals.Approvals {
		if _, ok := allowedRoles[approval.Role]; !ok {
			return nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals[%d].role is not allowed for %s",
				i,
				route,
			)}
		}
		if _, ok := approvalsByRole[approval.Role]; ok {
			return nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals[%d].role duplicates role %s",
				i,
				approval.Role,
			)}
		}
		if err := validateHexString(
			fmt.Sprintf("request.artifactApprovals.approvals[%d].signature", i),
			approval.Signature,
		); err != nil {
			return nil, nil, err
		}

		approvalsByRole[approval.Role] = normalizeLowerHex(approval.Signature)
	}

	derivedLegacySignatures := make([]string, len(requiredRoles))
	normalizedApprovals := &ArtifactApprovalEnvelope{
		Payload: ArtifactApprovalPayload{
			ApprovalVersion:           artifactApprovalVersion,
			Route:                     route,
			ScriptTemplateID:          route,
			DestinationCommitmentHash: normalizedDestinationCommitmentHash,
			PlanCommitmentHash:        normalizedPlanCommitmentHash,
		},
		Approvals: make([]ArtifactRoleApproval, len(requiredRoles)),
	}
	for i, role := range requiredRoles {
		signature, ok := approvalsByRole[role]
		if !ok {
			return nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals must include role %s for %s",
				role,
				route,
			)}
		}

		derivedLegacySignatures[i] = signature
		normalizedApprovals.Approvals[i] = ArtifactRoleApproval{
			Role:      role,
			Signature: signature,
		}
	}

	if len(normalizedLegacySignatures) != len(derivedLegacySignatures) {
		return nil, nil, &inputError{"request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals"}
	}
	for i := range derivedLegacySignatures {
		if normalizedLegacySignatures[i] != derivedLegacySignatures[i] {
			return nil, nil, &inputError{"request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals"}
		}
	}

	return normalizedApprovals, derivedLegacySignatures, nil
}

func normalizeArtifactRecord(record ArtifactRecord) ArtifactRecord {
	normalized := ArtifactRecord{
		PSBTHash:                  normalizeLowerHex(record.PSBTHash),
		DestinationCommitmentHash: normalizeLowerHex(record.DestinationCommitmentHash),
	}
	if record.TransactionHex != "" {
		normalized.TransactionHex = normalizeLowerHex(record.TransactionHex)
	}
	if record.TransactionID != "" {
		normalized.TransactionID = normalizeLowerHex(record.TransactionID)
	}

	return normalized
}

func normalizeArtifacts(artifacts map[RecoveryPathID]ArtifactRecord) map[RecoveryPathID]ArtifactRecord {
	if artifacts == nil {
		return nil
	}

	normalized := make(map[RecoveryPathID]ArtifactRecord, len(artifacts))
	for pathID, artifact := range artifacts {
		normalized[pathID] = normalizeArtifactRecord(artifact)
	}

	return normalized
}

func normalizeMigrationDestination(
	destination *MigrationDestinationReservation,
) *MigrationDestinationReservation {
	if destination == nil {
		return nil
	}

	return &MigrationDestinationReservation{
		ReservationID:             destination.ReservationID,
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

func normalizeRouteSubmitRequest(request RouteSubmitRequest) (RouteSubmitRequest, error) {
	normalizedArtifactApprovals, normalizedArtifactSignatures, err := normalizeArtifactApprovals(
		request.Route,
		request,
	)
	if err != nil {
		return RouteSubmitRequest{}, err
	}

	normalizedScriptTemplate, err := normalizeScriptTemplate(request.Route, request.ScriptTemplate)
	if err != nil {
		return RouteSubmitRequest{}, err
	}

	return RouteSubmitRequest{
		FacadeRequestID: request.FacadeRequestID,
		IdempotencyKey:  request.IdempotencyKey,
		Route:           request.Route,
		Strategy:        normalizeLowerHex(request.Strategy),
		Reserve:         normalizeLowerHex(request.Reserve),
		Epoch:           request.Epoch,
		MaturityHeight:  request.MaturityHeight,
		ActiveOutpoint: CovenantOutpoint{
			TxID: normalizeLowerHex(request.ActiveOutpoint.TxID),
			Vout: request.ActiveOutpoint.Vout,
			ScriptHash: func() string {
				if request.ActiveOutpoint.ScriptHash == "" {
					return ""
				}
				return normalizeLowerHex(request.ActiveOutpoint.ScriptHash)
			}(),
		},
		DestinationCommitmentHash: normalizeLowerHex(request.DestinationCommitmentHash),
		MigrationDestination:      normalizeMigrationDestination(request.MigrationDestination),
		MigrationTransactionPlan:  normalizeMigrationTransactionPlan(request.MigrationTransactionPlan),
		ArtifactApprovals:         normalizedArtifactApprovals,
		ArtifactSignatures:        normalizedArtifactSignatures,
		Artifacts:                 normalizeArtifacts(request.Artifacts),
		ScriptTemplate:            normalizedScriptTemplate,
		Signing:                   request.Signing,
	}, nil
}

func validateCommonRequest(route TemplateID, request RouteSubmitRequest) error {
	if request.FacadeRequestID == "" {
		return &inputError{"request.facadeRequestId is required"}
	}
	if request.IdempotencyKey == "" {
		return &inputError{"request.idempotencyKey is required"}
	}
	if request.Route != route {
		return &inputError{"request.route does not match endpoint route"}
	}
	if err := validateHexString("request.strategy", request.Strategy); err != nil {
		return err
	}
	if err := validateHexString("request.reserve", request.Reserve); err != nil {
		return err
	}
	if err := validateHexString("request.activeOutpoint.txid", request.ActiveOutpoint.TxID); err != nil {
		return err
	}
	if request.ActiveOutpoint.ScriptHash != "" {
		if err := validateHexString("request.activeOutpoint.scriptHash", request.ActiveOutpoint.ScriptHash); err != nil {
			return err
		}
	}
	if err := validateHexString("request.destinationCommitmentHash", request.DestinationCommitmentHash); err != nil {
		return err
	}
	// This intentionally creates a deployment ordering constraint: the
	// orchestrator must supply the concrete migration destination artifact
	// before this signer version can accept requests.
	if err := validateMigrationDestination(request, request.MigrationDestination); err != nil {
		return err
	}
	// This intentionally creates the next deployment ordering constraint: the
	// orchestrator must supply the canonical migration transaction plan before
	// this signer version can accept requests.
	if err := validateMigrationTransactionPlan(request, request.MigrationTransactionPlan); err != nil {
		return err
	}
	if err := validateArtifactApprovals(route, request); err != nil {
		return err
	}

	switch route {
	case TemplateSelfV1:
		if !request.Signing.SignerRequired || request.Signing.CustodianRequired {
			return &inputError{"request.signing must set signerRequired=true and custodianRequired=false for self_v1"}
		}
		template := &SelfV1Template{}
		if err := strictUnmarshal(request.ScriptTemplate, template); err != nil {
			return &inputError{fmt.Sprintf("request.scriptTemplate is invalid for self_v1: %v", err)}
		}
		if template.Template != TemplateSelfV1 {
			return &inputError{"request.scriptTemplate.template must be self_v1"}
		}
		if err := validateHexString("request.scriptTemplate.depositorPublicKey", template.DepositorPublicKey); err != nil {
			return err
		}
		if err := validateHexString("request.scriptTemplate.signerPublicKey", template.SignerPublicKey); err != nil {
			return err
		}
	case TemplateQcV1:
		if !request.Signing.SignerRequired || !request.Signing.CustodianRequired {
			return &inputError{"request.signing must set signerRequired=true and custodianRequired=true for qc_v1"}
		}
		template := &QcV1Template{}
		if err := strictUnmarshal(request.ScriptTemplate, template); err != nil {
			return &inputError{fmt.Sprintf("request.scriptTemplate is invalid for qc_v1: %v", err)}
		}
		if template.Template != TemplateQcV1 {
			return &inputError{"request.scriptTemplate.template must be qc_v1"}
		}
		if err := validateHexString("request.scriptTemplate.depositorPublicKey", template.DepositorPublicKey); err != nil {
			return err
		}
		if err := validateHexString("request.scriptTemplate.custodianPublicKey", template.CustodianPublicKey); err != nil {
			return err
		}
		if err := validateHexString("request.scriptTemplate.signerPublicKey", template.SignerPublicKey); err != nil {
			return err
		}
	default:
		return &inputError{"unsupported request.route"}
	}

	return nil
}

func validateSubmitInput(route TemplateID, input SignerSubmitInput) error {
	if input.RouteRequestID == "" {
		return &inputError{"routeRequestId is required"}
	}
	if input.Stage != StageSignerCoordination {
		return &inputError{"stage must be SIGNER_COORDINATION"}
	}
	return validateCommonRequest(route, input.Request)
}

func validatePollInput(route TemplateID, input SignerPollInput) error {
	if input.RequestID == "" {
		return &inputError{"requestId is required"}
	}
	if err := validateSubmitInput(route, SignerSubmitInput{
		RouteRequestID: input.RouteRequestID,
		Request:        input.Request,
		Stage:          input.Stage,
	}); err != nil {
		return err
	}
	return nil
}
