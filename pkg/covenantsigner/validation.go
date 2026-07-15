package covenantsigner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
)

const (
	canonicalCovenantInputSequence     uint32 = 0xFFFFFFFD
	canonicalAnchorValueSats           uint64 = 330
	migrationTransactionPlanVersion    uint32 = 1
	artifactApprovalVersion            uint32 = 1
	signerApprovalCertificateVersion   uint32 = 2
	migrationPlanQuoteVersion          uint32 = 1
	migrationPlanQuoteSignatureVersion uint32 = 1
)

const (
	migrationPlanQuoteSignatureAlgorithm = "ed25519"
	migrationPlanQuoteSigningDomain      = "migration-plan-quote-v1:"
	signerApprovalSignatureAlgorithm     = "tecdsa-secp256k1"
	covenantSignerRequestDigestDomain    = "covenant-signer-request-v1:"
)

var artifactApprovalTypeHash = crypto.Keccak256Hash([]byte(
	"ArtifactApproval(" +
		"uint8 approvalVersion," +
		"bytes32 route," +
		"bytes32 scriptTemplateId," +
		"bytes32 destinationCommitmentHash," +
		"bytes32 planCommitmentHash)",
))

var canonicalTimestampPattern = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`,
)

var requestIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,255}$`)

type inputError struct {
	message string
}

func (ie *inputError) Error() string {
	return ie.message
}

func NewInputError(message string) error {
	return &inputError{message: message}
}

func strictUnmarshal(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected trailing content after JSON object")
	}
	return nil
}

type validationOptions struct {
	migrationPlanQuoteTrustRoots      []MigrationPlanQuoteTrustRoot
	depositorTrustRoots               []DepositorTrustRoot
	custodianTrustRoots               []CustodianTrustRoot
	requireFreshMigrationPlanQuote    bool
	migrationPlanQuoteVerificationNow time.Time
	signerApprovalVerifier            SignerApprovalVerifier
	policyIndependentDigest           bool
	currentBlock                      *uint64
}

// requestDigest accepts raw requests because Poll validates equivalence against
// whatever the caller resubmits. Submit should use requestDigestFromNormalized
// after it has already normalized the request once for storage.
func requestDigest(
	request RouteSubmitRequest,
	options validationOptions,
) (string, error) {
	normalizedRequest, err := normalizeRouteSubmitRequest(
		request,
		options,
	)
	if err != nil {
		return "", err
	}

	return requestDigestFromNormalized(normalizedRequest)
}

// requestDigestFromNormalized computes a domain-separated SHA256 digest of
// the canonical JSON encoding of the already-normalized request. The domain
// prefix prevents cross-context hash collisions with other SHA256-based
// identifiers in the protocol.
func requestDigestFromNormalized(request RouteSubmitRequest) (string, error) {
	payload, err := canonicaljson.Marshal(request)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(append([]byte(covenantSignerRequestDigestDomain), payload...))
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

func validateRequestIdentifier(name string, value string) error {
	if !requestIdentifierPattern.MatchString(value) {
		return &inputError{fmt.Sprintf("%s must match [a-zA-Z0-9_-] and be at most 255 chars", name)}
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

func validateBytes32HexString(name string, value string) error {
	if err := validateHexString(name, value); err != nil {
		return err
	}

	if len(value) != 66 {
		return &inputError{fmt.Sprintf("%s must be a 32-byte 0x-prefixed hex string", name)}
	}

	return nil
}

func validateUint32Range(name string, value uint64) error {
	if value > math.MaxUint32 {
		return &inputError{fmt.Sprintf("%s must fit in uint32", name)}
	}

	return nil
}

func decodeBytes32HexString(name string, value string) ([32]byte, error) {
	var decoded [32]byte

	if err := validateBytes32HexString(name, value); err != nil {
		return decoded, err
	}

	rawValue, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return decoded, &inputError{fmt.Sprintf("%s must be valid hex", name)}
	}

	copy(decoded[:], rawValue)
	return decoded, nil
}

func normalizeLowerHex(value string) string {
	return strings.ToLower(value)
}

func normalizeRequestType(
	route TemplateID,
	requestType RequestType,
) (RequestType, error) {
	switch requestType {
	case RequestTypeReconstruct:
		return requestType, nil
	case RequestTypePresignSelfV1:
		if route != TemplateSelfV1 {
			return "", &inputError{"request.requestType must be reconstruct for qc_v1"}
		}
		return requestType, nil
	default:
		return "", &inputError{"request.requestType must be reconstruct or presign_self_v1"}
	}
}

func normalizeRouteSubmitRequest(
	request RouteSubmitRequest,
	options validationOptions,
) (RouteSubmitRequest, error) {
	normalizedArtifactApprovals, normalizedSignerApproval, normalizedArtifactSignatures, err := normalizeArtifactApprovals(
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

	normalizedMigrationPlanQuote, err := normalizeMigrationPlanQuote(
		request,
		options,
	)
	if err != nil {
		return RouteSubmitRequest{}, err
	}
	normalizedRequestType, err := normalizeRequestType(request.Route, request.RequestType)
	if err != nil {
		return RouteSubmitRequest{}, err
	}

	return RouteSubmitRequest{
		FacadeRequestID: request.FacadeRequestID,
		IdempotencyKey:  request.IdempotencyKey,
		RequestType:     normalizedRequestType,
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
		MigrationPlanQuote:        normalizedMigrationPlanQuote,
		MigrationTransactionPlan:  normalizeMigrationTransactionPlan(request.MigrationTransactionPlan),
		ArtifactApprovals:         normalizedArtifactApprovals,
		SignerApproval:            normalizedSignerApproval,
		ArtifactSignatures:        normalizedArtifactSignatures,
		Artifacts:                 normalizeArtifacts(request.Artifacts),
		ScriptTemplate:            normalizedScriptTemplate,
		Signing:                   request.Signing,
	}, nil
}

func validateCommonRequest(
	route TemplateID,
	request RouteSubmitRequest,
	options validationOptions,
) error {
	if request.FacadeRequestID == "" {
		return &inputError{"request.facadeRequestId is required"}
	}
	if err := validateRequestIdentifier("request.facadeRequestId", request.FacadeRequestID); err != nil {
		return err
	}
	if request.IdempotencyKey == "" {
		return &inputError{"request.idempotencyKey is required"}
	}
	if err := validateRequestIdentifier("request.idempotencyKey", request.IdempotencyKey); err != nil {
		return err
	}
	if request.Route != route {
		return &inputError{"request.route does not match endpoint route"}
	}
	if _, err := normalizeRequestType(route, request.RequestType); err != nil {
		return err
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
	if _, err := normalizeMigrationPlanQuote(request, options); err != nil {
		return err
	}
	if request.ArtifactApprovals == nil {
		return &inputError{"request.artifactApprovals is required"}
	}
	if options.signerApprovalVerifier != nil && request.SignerApproval == nil {
		return &inputError{
			"request.signerApproval is required when the signer approval verifier is configured",
		}
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

		depositorPublicKey := template.DepositorPublicKey
		if len(options.depositorTrustRoots) > 0 && !options.policyIndependentDigest {
			expectedDepositorPublicKey, ok := resolveExpectedDepositorPublicKey(
				request,
				options.depositorTrustRoots,
			)
			if !ok {
				return &inputError{
					"request.scriptTemplate.depositorPublicKey requires a matching configured depositorTrustRoots entry for self_v1",
				}
			}
			if normalizeLowerHex(template.DepositorPublicKey) != expectedDepositorPublicKey {
				return &inputError{
					"request.scriptTemplate.depositorPublicKey must match the configured depositorTrustRoots publicKey for self_v1",
				}
			}
			depositorPublicKey = expectedDepositorPublicKey
		}

		if err := validateArtifactApprovalAuthenticity(
			request,
			depositorPublicKey,
			"",
		); err != nil {
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

		depositorPublicKey := template.DepositorPublicKey
		if len(options.depositorTrustRoots) > 0 && !options.policyIndependentDigest {
			expectedDepositorPublicKey, ok := resolveExpectedDepositorPublicKey(
				request,
				options.depositorTrustRoots,
			)
			if !ok {
				return &inputError{
					"request.scriptTemplate.depositorPublicKey requires a matching configured depositorTrustRoots entry for qc_v1",
				}
			}
			if normalizeLowerHex(template.DepositorPublicKey) != expectedDepositorPublicKey {
				return &inputError{
					"request.scriptTemplate.depositorPublicKey must match the configured depositorTrustRoots publicKey for qc_v1",
				}
			}
			depositorPublicKey = expectedDepositorPublicKey
		}

		custodianPublicKey := template.CustodianPublicKey
		if len(options.custodianTrustRoots) > 0 && !options.policyIndependentDigest {
			expectedCustodianPublicKey, ok := resolveExpectedCustodianPublicKey(
				request,
				options.custodianTrustRoots,
			)
			if !ok {
				return &inputError{
					"request.scriptTemplate.custodianPublicKey requires a matching configured custodianTrustRoots entry for qc_v1",
				}
			}
			if normalizeLowerHex(template.CustodianPublicKey) != expectedCustodianPublicKey {
				return &inputError{
					"request.scriptTemplate.custodianPublicKey must match the configured custodianTrustRoots publicKey for qc_v1",
				}
			}
			custodianPublicKey = expectedCustodianPublicKey
		}

		if err := validateArtifactApprovalAuthenticity(
			request,
			depositorPublicKey,
			custodianPublicKey,
		); err != nil {
			return err
		}
	default:
		return &inputError{"unsupported request.route"}
	}

	if request.SignerApproval != nil {
		// Certificate v2 carries an explicit inclusive expiration block that is
		// bound into the threshold signature. Enforce expiry unconditionally
		// against the current host-chain height before any signature
		// verification: an expired certificate is rejected outright and never
		// re-verified, because re-verifying only proves the same expired
		// authorization; renewal requires a fresh certificate with a later
		// signed EndBlock.
		if request.SignerApproval.EndBlock == nil {
			return &inputError{"request.signerApproval.endBlock is required"}
		}
		if options.currentBlock == nil {
			// Fail closed: a certificate with an expiration block cannot be
			// honored without a current host-chain height to compare against.
			return &inputError{
				"request.signerApproval cannot be verified without a current block height provider",
			}
		}
		if certificateExpired(*options.currentBlock, *request.SignerApproval.EndBlock) {
			return &inputError{"signer approval certificate has expired"}
		}

		// Poll re-validates the resubmitted request with a policy-independent
		// digest and no verifier; the stored job was already signature-verified
		// at Submit time and is re-checked for expiry separately. Submit falls
		// through to signature verification below.
		if options.policyIndependentDigest {
			return nil
		}

		if options.signerApprovalVerifier == nil {
			return &inputError{
				"request.signerApproval cannot be verified by this signer deployment",
			}
		}

		normalizedRequest, err := normalizeRouteSubmitRequest(request, options)
		if err != nil {
			return err
		}

		if err := options.signerApprovalVerifier.VerifySignerApproval(
			normalizedRequest,
		); err != nil {
			return err
		}
	}

	return nil
}

// certificateExpired reports whether a signer approval certificate whose
// inclusive last valid block is endBlock has expired at currentBlock. EndBlock
// is a closed interval: the certificate remains valid up to and including
// endBlock and is expired only once currentBlock strictly exceeds it.
func certificateExpired(currentBlock uint64, endBlock uint64) bool {
	return currentBlock > endBlock
}

func validateSubmitInput(
	route TemplateID,
	input SignerSubmitInput,
	options validationOptions,
) error {
	if input.RouteRequestID == "" {
		return &inputError{"routeRequestId is required"}
	}
	if input.Stage != StageSignerCoordination {
		return &inputError{"stage must be SIGNER_COORDINATION"}
	}
	return validateCommonRequest(route, input.Request, options)
}

func validatePollInput(
	route TemplateID,
	input SignerPollInput,
	options validationOptions,
) error {
	if input.RequestID == "" {
		return &inputError{"requestId is required"}
	}
	if err := validateSubmitInput(route, SignerSubmitInput{
		RouteRequestID: input.RouteRequestID,
		Request:        input.Request,
		Stage:          input.Stage,
	}, options); err != nil {
		return err
	}
	return nil
}

// migrationPlanQuoteSigningPayload is a private struct used only within the
// quote signing functions in validation_quote.go.
type migrationPlanQuoteSigningPayload struct {
	QuoteVersion              uint32 `json:"quoteVersion"`
	QuoteID                   string `json:"quoteId"`
	ReservationID             string `json:"reservationId"`
	Reserve                   string `json:"reserve"`
	Epoch                     uint64 `json:"epoch"`
	Route                     string `json:"route"`
	Revealer                  string `json:"revealer"`
	Vault                     string `json:"vault"`
	Network                   string `json:"network"`
	DestinationCommitmentHash string `json:"destinationCommitmentHash"`
	ActiveOutpointTxID        string `json:"activeOutpointTxid"`
	ActiveOutpointVout        uint32 `json:"activeOutpointVout"`
	PlanCommitmentHash        string `json:"planCommitmentHash"`
	IssuedAt                  string `json:"issuedAt"`
	ExpiresAt                 string `json:"expiresAt"`
	ExpiresInSeconds          uint64 `json:"expiresInSeconds"`
}
