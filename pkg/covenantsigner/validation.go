package covenantsigner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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

func requestDigest(request RouteSubmitRequest) (string, error) {
	payload, err := json.Marshal(request)
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
	if len(request.ArtifactSignatures) == 0 {
		return &inputError{"request.artifactSignatures must not be empty"}
	}
	for i, signature := range request.ArtifactSignatures {
		if err := validateHexString(fmt.Sprintf("request.artifactSignatures[%d]", i), signature); err != nil {
			return err
		}
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
