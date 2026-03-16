package covenantsigner

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	canonicalCovenantInputSequence     uint32 = 0xFFFFFFFD
	canonicalAnchorValueSats           uint64 = 330
	migrationTransactionPlanVersion    uint32 = 1
	artifactApprovalVersion            uint32 = 1
	signerApprovalCertificateVersion   uint32 = 1
	migrationPlanQuoteVersion          uint32 = 1
	migrationPlanQuoteSignatureVersion uint32 = 1
)

const (
	migrationPlanQuoteSignatureAlgorithm = "ed25519"
	migrationPlanQuoteSigningDomain      = "migration-plan-quote-v1:"
	signerApprovalSignatureAlgorithm     = "tecdsa-secp256k1"
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

type validationOptions struct {
	migrationPlanQuoteTrustRoots      []MigrationPlanQuoteTrustRoot
	depositorTrustRoots               []DepositorTrustRoot
	custodianTrustRoots               []CustodianTrustRoot
	requireFreshMigrationPlanQuote    bool
	migrationPlanQuoteVerificationNow time.Time
	signerApprovalVerifier            SignerApprovalVerifier
}

func resolveValidationOptions(options []validationOptions) validationOptions {
	if len(options) == 0 {
		return validationOptions{}
	}

	return options[0]
}

// requestDigest accepts raw requests because Poll validates equivalence against
// whatever the caller resubmits. Submit should use requestDigestFromNormalized
// after it has already normalized the request once for storage.
func requestDigest(
	request RouteSubmitRequest,
	options ...validationOptions,
) (string, error) {
	normalizedRequest, err := normalizeRouteSubmitRequest(
		request,
		resolveValidationOptions(options),
	)
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

func normalizeSignerApprovalMemberIndexes(
	name string,
	values []uint32,
) ([]uint32, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := append([]uint32{}, values...)
	seen := make(map[uint32]struct{}, len(normalized))
	for i, value := range normalized {
		if value == 0 {
			return nil, &inputError{
				fmt.Sprintf("%s[%d] must be greater than zero", name, i),
			}
		}
		if err := validateUint32Range(name, uint64(value)); err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			return nil, &inputError{
				fmt.Sprintf("%s[%d] duplicates member %d", name, i, value),
			}
		}
		seen[value] = struct{}{}
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})

	return normalized, nil
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

func normalizeSignerApprovalCertificate(
	request RouteSubmitRequest,
) (*SignerApprovalCertificate, error) {
	if request.SignerApproval == nil {
		return nil, nil
	}
	if request.ArtifactApprovals == nil {
		return nil, &inputError{
			"request.artifactApprovals is required when request.signerApproval is present",
		}
	}

	signerApproval := request.SignerApproval
	if signerApproval.CertificateVersion != signerApprovalCertificateVersion {
		return nil, &inputError{
			fmt.Sprintf(
				"request.signerApproval.certificateVersion must equal %d",
				signerApprovalCertificateVersion,
			),
		}
	}
	if signerApproval.SignatureAlgorithm != signerApprovalSignatureAlgorithm {
		return nil, &inputError{
			fmt.Sprintf(
				"request.signerApproval.signatureAlgorithm must equal %s",
				signerApprovalSignatureAlgorithm,
			),
		}
	}
	if err := validateBytes32HexString(
		"request.signerApproval.approvalDigest",
		signerApproval.ApprovalDigest,
	); err != nil {
		return nil, err
	}
	if err := validateHexString(
		"request.signerApproval.walletPublicKey",
		signerApproval.WalletPublicKey,
	); err != nil {
		return nil, err
	}
	if len(signerApproval.WalletPublicKey) != 132 {
		return nil, &inputError{
			"request.signerApproval.walletPublicKey must be a 65-byte uncompressed secp256k1 public key",
		}
	}
	normalizedWalletPublicKey := normalizeLowerHex(signerApproval.WalletPublicKey)
	if !strings.HasPrefix(normalizedWalletPublicKey, "0x04") {
		return nil, &inputError{
			"request.signerApproval.walletPublicKey must be a 65-byte uncompressed secp256k1 public key",
		}
	}
	if err := validateBytes32HexString(
		"request.signerApproval.signerSetHash",
		signerApproval.SignerSetHash,
	); err != nil {
		return nil, err
	}
	if err := validateHexString(
		"request.signerApproval.signature",
		signerApproval.Signature,
	); err != nil {
		return nil, err
	}

	expectedApprovalDigest, err := artifactApprovalDigest(request.ArtifactApprovals.Payload)
	if err != nil {
		return nil, err
	}

	normalizedApprovalDigest := normalizeLowerHex(signerApproval.ApprovalDigest)
	if normalizedApprovalDigest != "0x"+hex.EncodeToString(expectedApprovalDigest) {
		return nil, &inputError{
			"request.signerApproval.approvalDigest must match the canonical artifactApprovals payload digest",
		}
	}

	normalizedSignerApproval := &SignerApprovalCertificate{
		CertificateVersion: signerApprovalCertificateVersion,
		SignatureAlgorithm: signerApprovalSignatureAlgorithm,
		ApprovalDigest:     normalizedApprovalDigest,
		WalletPublicKey:    normalizedWalletPublicKey,
		SignerSetHash:      normalizeLowerHex(signerApproval.SignerSetHash),
		Signature:          normalizeLowerHex(signerApproval.Signature),
	}

	activeMembers, err := normalizeSignerApprovalMemberIndexes(
		"request.signerApproval.activeMembers",
		signerApproval.ActiveMembers,
	)
	if err != nil {
		return nil, err
	}
	if len(activeMembers) > 0 {
		normalizedSignerApproval.ActiveMembers = activeMembers
	}

	inactiveMembers, err := normalizeSignerApprovalMemberIndexes(
		"request.signerApproval.inactiveMembers",
		signerApproval.InactiveMembers,
	)
	if err != nil {
		return nil, err
	}
	if len(inactiveMembers) > 0 {
		normalizedSignerApproval.InactiveMembers = inactiveMembers
	}

	if len(activeMembers) > 0 && len(inactiveMembers) > 0 {
		activeSet := make(map[uint32]struct{}, len(activeMembers))
		for _, value := range activeMembers {
			activeSet[value] = struct{}{}
		}
		for _, value := range inactiveMembers {
			if _, ok := activeSet[value]; ok {
				return nil, &inputError{
					"request.signerApproval.activeMembers and request.signerApproval.inactiveMembers must not overlap",
				}
			}
		}
	}

	if signerApproval.EndBlock != nil {
		if err := validateUint32Range(
			"request.signerApproval.endBlock",
			*signerApproval.EndBlock,
		); err != nil {
			return nil, err
		}
		endBlock := *signerApproval.EndBlock
		normalizedSignerApproval.EndBlock = &endBlock
	}

	return normalizedSignerApproval, nil
}

func normalizeLowerHex(value string) string {
	return strings.ToLower(value)
}

func abiEncodeUint32Word(value uint32) [32]byte {
	var encoded [32]byte
	binary.BigEndian.PutUint32(encoded[28:], value)
	return encoded
}

func keccakTemplateIdentifier(id TemplateID) [32]byte {
	hash := crypto.Keccak256Hash([]byte(id))

	var encoded [32]byte
	copy(encoded[:], hash.Bytes())

	return encoded
}

// artifactApprovalDigest pins the current phase-1 approval payload contract to
// a deterministic EIP-712-compatible struct hash, without yet committing to a
// chain-specific domain separator.
func artifactApprovalDigest(payload ArtifactApprovalPayload) ([]byte, error) {
	destinationCommitmentHash, err := decodeBytes32HexString(
		"request.artifactApprovals.payload.destinationCommitmentHash",
		payload.DestinationCommitmentHash,
	)
	if err != nil {
		return nil, err
	}

	planCommitmentHash, err := decodeBytes32HexString(
		"request.artifactApprovals.payload.planCommitmentHash",
		payload.PlanCommitmentHash,
	)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, 32*6)
	approvalVersionWord := abiEncodeUint32Word(payload.ApprovalVersion)
	routeIdentifier := keccakTemplateIdentifier(payload.Route)
	scriptTemplateIdentifier := keccakTemplateIdentifier(payload.ScriptTemplateID)

	copy(encoded[0:32], artifactApprovalTypeHash.Bytes())
	copy(encoded[32:64], approvalVersionWord[:])
	copy(encoded[64:96], routeIdentifier[:])
	copy(encoded[96:128], scriptTemplateIdentifier[:])
	copy(encoded[128:160], destinationCommitmentHash[:])
	copy(encoded[160:192], planCommitmentHash[:])

	digest := crypto.Keccak256Hash(encoded)
	return digest.Bytes(), nil
}

// ComputeArtifactApprovalDigest exposes the current phase-1 approval payload
// digest contract to cross-package verifiers that need to bind
// signerApproval.approvalDigest to request.artifactApprovals.payload.
func ComputeArtifactApprovalDigest(payload ArtifactApprovalPayload) ([]byte, error) {
	return artifactApprovalDigest(payload)
}

func parseCompressedSecp256k1PublicKey(
	name string,
	value string,
) (*btcec.PublicKey, error) {
	if err := validateHexString(name, value); err != nil {
		return nil, err
	}

	rawValue, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return nil, &inputError{fmt.Sprintf("%s must be valid hex", name)}
	}

	if len(rawValue) != 33 || (rawValue[0] != 0x02 && rawValue[0] != 0x03) {
		return nil, &inputError{fmt.Sprintf("%s must be a compressed secp256k1 public key", name)}
	}

	publicKey, err := btcec.ParsePubKey(rawValue, btcec.S256())
	if err != nil {
		return nil, &inputError{fmt.Sprintf("%s must be a compressed secp256k1 public key", name)}
	}

	return publicKey, nil
}

func verifyCompactSecp256k1Signature(
	publicKey *btcec.PublicKey,
	digest []byte,
	signature []byte,
) bool {
	return ecdsa.Verify(
		publicKey.ToECDSA(),
		digest,
		new(big.Int).SetBytes(signature[:32]),
		new(big.Int).SetBytes(signature[32:]),
	)
}

func verifySecp256k1Signature(
	name string,
	publicKey *btcec.PublicKey,
	digest []byte,
	signature string,
) error {
	rawSignature, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return &inputError{fmt.Sprintf("%s must be valid hex", name)}
	}

	switch {
	case len(rawSignature) == 64:
		if verifyCompactSecp256k1Signature(publicKey, digest, rawSignature) {
			return nil
		}
	case len(rawSignature) == 65 &&
		(rawSignature[64] == 0 || rawSignature[64] == 1 || rawSignature[64] == 27 || rawSignature[64] == 28):
		if verifyCompactSecp256k1Signature(publicKey, digest, rawSignature[:64]) {
			return nil
		}
	default:
		parsedSignature, err := btcec.ParseDERSignature(rawSignature, btcec.S256())
		if err != nil {
			return &inputError{
				fmt.Sprintf(
					"%s must be a DER or 64/65-byte secp256k1 signature",
					name,
				),
			}
		}
		if parsedSignature.Verify(digest, publicKey) {
			return nil
		}
	}

	return &inputError{fmt.Sprintf("%s does not verify against the required public key", name)}
}

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

func normalizeCanonicalTimestamp(name string, value string) (string, error) {
	if !canonicalTimestampPattern.MatchString(value) {
		return "", &inputError{
			fmt.Sprintf(
				"%s must be a UTC ISO-8601 timestamp from Date.toISOString()",
				name,
			),
		}
	}

	return value, nil
}

func normalizeMigrationPlanQuotePublicKeyPEM(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\\n", "\n"))
}

func parseMigrationPlanQuoteTrustRoot(
	name string,
	trustRoot MigrationPlanQuoteTrustRoot,
) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizeMigrationPlanQuotePublicKeyPEM(trustRoot.PublicKeyPEM)))
	if block == nil {
		return nil, &inputError{fmt.Sprintf("%s.publicKeyPem must be a PEM-encoded public key", name)}
	}

	publicKeyValue, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, &inputError{fmt.Sprintf("%s.publicKeyPem must be a PEM-encoded Ed25519 public key", name)}
	}

	publicKey, ok := publicKeyValue.(ed25519.PublicKey)
	if !ok {
		return nil, &inputError{fmt.Sprintf("%s.publicKeyPem must be a PEM-encoded Ed25519 public key", name)}
	}

	return publicKey, nil
}

func normalizeScopedApprovalTrustRoot(
	name string,
	route TemplateID,
	reserve string,
	network string,
	publicKey string,
) (TemplateID, string, string, string, error) {
	switch route {
	case TemplateSelfV1, TemplateQcV1:
	default:
		return "", "", "", "", &inputError{
			fmt.Sprintf("%s.route must be self_v1 or qc_v1", name),
		}
	}

	if err := validateHexString(name+".reserve", reserve); err != nil {
		return "", "", "", "", err
	}

	trimmedNetwork := strings.TrimSpace(network)
	if trimmedNetwork == "" {
		return "", "", "", "", &inputError{
			fmt.Sprintf("%s.network is required", name),
		}
	}

	normalizedPublicKey := normalizeLowerHex(publicKey)
	if _, err := parseCompressedSecp256k1PublicKey(
		name+".publicKey",
		normalizedPublicKey,
	); err != nil {
		return "", "", "", "", err
	}

	return route,
		normalizeLowerHex(reserve),
		strings.ToLower(trimmedNetwork),
		normalizedPublicKey,
		nil
}

func normalizeDepositorTrustRoots(
	trustRoots []DepositorTrustRoot,
) ([]DepositorTrustRoot, error) {
	if len(trustRoots) == 0 {
		return nil, nil
	}

	normalized := make([]DepositorTrustRoot, len(trustRoots))
	seen := make(map[string]int, len(trustRoots))

	for i, trustRoot := range trustRoots {
		name := fmt.Sprintf("depositorTrustRoots[%d]", i)
		route, reserve, network, publicKey, err := normalizeScopedApprovalTrustRoot(
			name,
			trustRoot.Route,
			trustRoot.Reserve,
			trustRoot.Network,
			trustRoot.PublicKey,
		)
		if err != nil {
			return nil, err
		}

		scopeKey := string(route) + "|" + reserve + "|" + network
		if previousIndex, ok := seen[scopeKey]; ok {
			return nil, &inputError{
				fmt.Sprintf(
					"%s duplicates depositorTrustRoots[%d] for route %s reserve %s network %s",
					name,
					previousIndex,
					route,
					reserve,
					network,
				),
			}
		}
		seen[scopeKey] = i

		normalized[i] = DepositorTrustRoot{
			Route:     route,
			Reserve:   reserve,
			Network:   network,
			PublicKey: publicKey,
		}
	}

	return normalized, nil
}

func normalizeCustodianTrustRoots(
	trustRoots []CustodianTrustRoot,
) ([]CustodianTrustRoot, error) {
	if len(trustRoots) == 0 {
		return nil, nil
	}

	normalized := make([]CustodianTrustRoot, len(trustRoots))
	seen := make(map[string]int, len(trustRoots))

	for i, trustRoot := range trustRoots {
		name := fmt.Sprintf("custodianTrustRoots[%d]", i)
		route, reserve, network, publicKey, err := normalizeScopedApprovalTrustRoot(
			name,
			trustRoot.Route,
			trustRoot.Reserve,
			trustRoot.Network,
			trustRoot.PublicKey,
		)
		if err != nil {
			return nil, err
		}

		scopeKey := string(route) + "|" + reserve + "|" + network
		if previousIndex, ok := seen[scopeKey]; ok {
			return nil, &inputError{
				fmt.Sprintf(
					"%s duplicates custodianTrustRoots[%d] for route %s reserve %s network %s",
					name,
					previousIndex,
					route,
					reserve,
					network,
				),
			}
		}
		seen[scopeKey] = i

		normalized[i] = CustodianTrustRoot{
			Route:     route,
			Reserve:   reserve,
			Network:   network,
			PublicKey: publicKey,
		}
	}

	return normalized, nil
}

func trustRootLookupScope(request RouteSubmitRequest) (TemplateID, string, string) {
	network := ""
	if request.MigrationDestination != nil {
		network = strings.ToLower(strings.TrimSpace(request.MigrationDestination.Network))
	}

	return request.Route, normalizeLowerHex(request.Reserve), network
}

func resolveExpectedDepositorPublicKey(
	request RouteSubmitRequest,
	trustRoots []DepositorTrustRoot,
) (string, bool) {
	route, reserve, network := trustRootLookupScope(request)
	for _, trustRoot := range trustRoots {
		if trustRoot.Route == route &&
			trustRoot.Reserve == reserve &&
			trustRoot.Network == network {
			return trustRoot.PublicKey, true
		}
	}

	return "", false
}

func resolveExpectedCustodianPublicKey(
	request RouteSubmitRequest,
	trustRoots []CustodianTrustRoot,
) (string, bool) {
	route, reserve, network := trustRootLookupScope(request)
	for _, trustRoot := range trustRoots {
		if trustRoot.Route == route &&
			trustRoot.Reserve == reserve &&
			trustRoot.Network == network {
			return trustRoot.PublicKey, true
		}
	}

	return "", false
}

func migrationPlanQuoteSigningPayloadBytes(
	quote *MigrationDestinationPlanQuote,
) ([]byte, error) {
	return marshalCanonicalJSON(migrationPlanQuoteSigningPayload{
		QuoteVersion:              quote.QuoteVersion,
		QuoteID:                   quote.QuoteID,
		ReservationID:             quote.ReservationID,
		Reserve:                   normalizeLowerHex(quote.Reserve),
		Epoch:                     quote.Epoch,
		Route:                     string(quote.Route),
		Revealer:                  normalizeLowerHex(quote.Revealer),
		Vault:                     normalizeLowerHex(quote.Vault),
		Network:                   quote.Network,
		DestinationCommitmentHash: normalizeLowerHex(quote.DestinationCommitmentHash),
		ActiveOutpointTxID:        normalizeLowerHex(quote.ActiveOutpointTxID),
		ActiveOutpointVout:        quote.ActiveOutpointVout,
		PlanCommitmentHash:        normalizeLowerHex(quote.PlanCommitmentHash),
		IssuedAt:                  quote.IssuedAt,
		ExpiresAt:                 quote.ExpiresAt,
		ExpiresInSeconds:          quote.ExpiresInSeconds,
	})
}

func migrationPlanQuoteSigningPreimage(
	quote *MigrationDestinationPlanQuote,
) ([]byte, error) {
	payload, err := migrationPlanQuoteSigningPayloadBytes(quote)
	if err != nil {
		return nil, err
	}

	return []byte(migrationPlanQuoteSigningDomain + string(payload)), nil
}

func migrationPlanQuoteSigningHash(
	quote *MigrationDestinationPlanQuote,
) ([]byte, error) {
	preimage, err := migrationPlanQuoteSigningPreimage(quote)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(preimage)
	return sum[:], nil
}

func normalizeMigrationPlanQuote(
	request RouteSubmitRequest,
	options validationOptions,
) (*MigrationDestinationPlanQuote, error) {
	quote := request.MigrationPlanQuote
	if quote == nil {
		if len(options.migrationPlanQuoteTrustRoots) > 0 {
			return nil, &inputError{
				"request.migrationPlanQuote is required when migrationPlanQuoteTrustRoots are configured",
			}
		}

		return nil, nil
	}
	if len(options.migrationPlanQuoteTrustRoots) == 0 {
		return nil, &inputError{"request.migrationPlanQuote verification requires configured trust roots"}
	}
	if request.MigrationDestination == nil {
		return nil, &inputError{"request.migrationDestination is required when request.migrationPlanQuote is present"}
	}
	if request.MigrationTransactionPlan == nil {
		return nil, &inputError{"request.migrationTransactionPlan is required when request.migrationPlanQuote is present"}
	}
	if quote.QuoteVersion != migrationPlanQuoteVersion {
		return nil, &inputError{"request.migrationPlanQuote.quoteVersion must equal 1"}
	}
	if strings.TrimSpace(quote.QuoteID) == "" {
		return nil, &inputError{"request.migrationPlanQuote.quoteId is required"}
	}
	if strings.TrimSpace(quote.ReservationID) == "" {
		return nil, &inputError{"request.migrationPlanQuote.reservationId is required"}
	}
	if strings.TrimSpace(quote.IdempotencyKey) == "" {
		return nil, &inputError{"request.migrationPlanQuote.idempotencyKey is required"}
	}
	if quote.Route != ReservationRouteMigration {
		return nil, &inputError{"request.migrationPlanQuote.route must be MIGRATION"}
	}
	if err := validateAddressString("request.migrationPlanQuote.reserve", quote.Reserve); err != nil {
		return nil, err
	}
	if err := validateAddressString("request.migrationPlanQuote.revealer", quote.Revealer); err != nil {
		return nil, err
	}
	if err := validateAddressString("request.migrationPlanQuote.vault", quote.Vault); err != nil {
		return nil, err
	}
	if strings.TrimSpace(quote.Network) == "" {
		return nil, &inputError{"request.migrationPlanQuote.network is required"}
	}
	if err := validateBytes32HexString(
		"request.migrationPlanQuote.destinationCommitmentHash",
		quote.DestinationCommitmentHash,
	); err != nil {
		return nil, err
	}
	if err := validateBytes32HexString(
		"request.migrationPlanQuote.activeOutpointTxid",
		quote.ActiveOutpointTxID,
	); err != nil {
		return nil, err
	}
	if err := validateBytes32HexString(
		"request.migrationPlanQuote.planCommitmentHash",
		quote.PlanCommitmentHash,
	); err != nil {
		return nil, err
	}
	if quote.ExpiresInSeconds == 0 {
		return nil, &inputError{"request.migrationPlanQuote.expiresInSeconds must be greater than zero"}
	}
	if quote.Signature.SignatureVersion != migrationPlanQuoteSignatureVersion {
		return nil, &inputError{"request.migrationPlanQuote.signature.signatureVersion must equal 1"}
	}
	if quote.Signature.Algorithm != migrationPlanQuoteSignatureAlgorithm {
		return nil, &inputError{"request.migrationPlanQuote.signature.algorithm must equal ed25519"}
	}
	if strings.TrimSpace(quote.Signature.KeyID) == "" {
		return nil, &inputError{"request.migrationPlanQuote.signature.keyId is required"}
	}
	if err := validateHexString("request.migrationPlanQuote.signature.signature", quote.Signature.Signature); err != nil {
		return nil, err
	}

	normalizedIssuedAt, err := normalizeCanonicalTimestamp(
		"request.migrationPlanQuote.issuedAt",
		quote.IssuedAt,
	)
	if err != nil {
		return nil, err
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, normalizedIssuedAt)
	if err != nil {
		return nil, &inputError{
			"request.migrationPlanQuote.issuedAt must be a parseable UTC ISO-8601 timestamp",
		}
	}
	normalizedExpiresAt, err := normalizeCanonicalTimestamp(
		"request.migrationPlanQuote.expiresAt",
		quote.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, normalizedExpiresAt)
	if err != nil {
		return nil, &inputError{
			"request.migrationPlanQuote.expiresAt must be a parseable UTC ISO-8601 timestamp",
		}
	}
	if !expiresAt.After(issuedAt) {
		return nil, &inputError{"request.migrationPlanQuote.expiresAt must be after request.migrationPlanQuote.issuedAt"}
	}
	if expiresAt.Sub(issuedAt) != time.Duration(quote.ExpiresInSeconds)*time.Second {
		return nil, &inputError{"request.migrationPlanQuote.expiresAt must equal request.migrationPlanQuote.issuedAt + expiresInSeconds"}
	}
	if quote.Epoch != request.Epoch {
		return nil, &inputError{"request.migrationPlanQuote.epoch must match request.epoch"}
	}
	if normalizeLowerHex(quote.Reserve) != normalizeLowerHex(request.Reserve) {
		return nil, &inputError{"request.migrationPlanQuote.reserve must match request.reserve"}
	}
	if quote.ReservationID != request.MigrationDestination.ReservationID {
		return nil, &inputError{"request.migrationPlanQuote.reservationId must match request.migrationDestination.reservationId"}
	}
	if normalizeLowerHex(quote.Revealer) != normalizeLowerHex(request.MigrationDestination.Revealer) {
		return nil, &inputError{"request.migrationPlanQuote.revealer must match request.migrationDestination.revealer"}
	}
	if normalizeLowerHex(quote.Vault) != normalizeLowerHex(request.MigrationDestination.Vault) {
		return nil, &inputError{"request.migrationPlanQuote.vault must match request.migrationDestination.vault"}
	}
	if strings.TrimSpace(quote.Network) != strings.TrimSpace(request.MigrationDestination.Network) {
		return nil, &inputError{"request.migrationPlanQuote.network must match request.migrationDestination.network"}
	}
	if normalizeLowerHex(quote.DestinationCommitmentHash) != normalizeLowerHex(request.DestinationCommitmentHash) {
		return nil, &inputError{"request.migrationPlanQuote.destinationCommitmentHash must match request.destinationCommitmentHash"}
	}
	if normalizeLowerHex(quote.DestinationCommitmentHash) != normalizeLowerHex(request.MigrationDestination.DestinationCommitmentHash) {
		return nil, &inputError{"request.migrationPlanQuote.destinationCommitmentHash must match request.migrationDestination.destinationCommitmentHash"}
	}
	if normalizeLowerHex(quote.ActiveOutpointTxID) != normalizeLowerHex(request.ActiveOutpoint.TxID) {
		return nil, &inputError{"request.migrationPlanQuote.activeOutpointTxid must match request.activeOutpoint.txid"}
	}
	if quote.ActiveOutpointVout != request.ActiveOutpoint.Vout {
		return nil, &inputError{"request.migrationPlanQuote.activeOutpointVout must match request.activeOutpoint.vout"}
	}
	if normalizeLowerHex(quote.PlanCommitmentHash) != normalizeLowerHex(request.MigrationTransactionPlan.PlanCommitmentHash) {
		return nil, &inputError{"request.migrationPlanQuote.planCommitmentHash must match request.migrationTransactionPlan.planCommitmentHash"}
	}

	normalizedQuotePlan := normalizeMigrationTransactionPlan(quote.MigrationTransactionPlan)
	if normalizedQuotePlan == nil {
		return nil, &inputError{"request.migrationPlanQuote.migrationTransactionPlan is required"}
	}
	if err := validateMigrationTransactionPlan(request, quote.MigrationTransactionPlan); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(normalizedQuotePlan, normalizeMigrationTransactionPlan(request.MigrationTransactionPlan)) {
		return nil, &inputError{"request.migrationPlanQuote.migrationTransactionPlan must match request.migrationTransactionPlan"}
	}

	var publicKey ed25519.PublicKey
	foundTrustRoot := false
	for i, trustRoot := range options.migrationPlanQuoteTrustRoots {
		if trustRoot.KeyID != quote.Signature.KeyID {
			continue
		}

		publicKey, err = parseMigrationPlanQuoteTrustRoot(
			fmt.Sprintf("migrationPlanQuoteTrustRoots[%d]", i),
			trustRoot,
		)
		if err != nil {
			return nil, err
		}
		foundTrustRoot = true
		break
	}
	if !foundTrustRoot {
		return nil, &inputError{"request.migrationPlanQuote.signature.keyId does not match a configured trust root"}
	}

	normalizedQuote := &MigrationDestinationPlanQuote{
		QuoteID:                   strings.TrimSpace(quote.QuoteID),
		QuoteVersion:              migrationPlanQuoteVersion,
		ReservationID:             strings.TrimSpace(quote.ReservationID),
		Reserve:                   normalizeLowerHex(quote.Reserve),
		Epoch:                     quote.Epoch,
		Route:                     ReservationRouteMigration,
		Revealer:                  normalizeLowerHex(quote.Revealer),
		Vault:                     normalizeLowerHex(quote.Vault),
		Network:                   strings.TrimSpace(quote.Network),
		DestinationCommitmentHash: normalizeLowerHex(quote.DestinationCommitmentHash),
		ActiveOutpointTxID:        normalizeLowerHex(quote.ActiveOutpointTxID),
		ActiveOutpointVout:        quote.ActiveOutpointVout,
		PlanCommitmentHash:        normalizeLowerHex(quote.PlanCommitmentHash),
		MigrationTransactionPlan:  normalizedQuotePlan,
		IdempotencyKey:            strings.TrimSpace(quote.IdempotencyKey),
		ExpiresInSeconds:          quote.ExpiresInSeconds,
		IssuedAt:                  normalizedIssuedAt,
		ExpiresAt:                 normalizedExpiresAt,
		Signature: MigrationDestinationPlanQuoteSignature{
			SignatureVersion: migrationPlanQuoteSignatureVersion,
			Algorithm:        migrationPlanQuoteSignatureAlgorithm,
			KeyID:            strings.TrimSpace(quote.Signature.KeyID),
			Signature:        normalizeLowerHex(quote.Signature.Signature),
		},
	}

	signingHash, err := migrationPlanQuoteSigningHash(normalizedQuote)
	if err != nil {
		return nil, err
	}

	rawSignature, err := hex.DecodeString(strings.TrimPrefix(normalizedQuote.Signature.Signature, "0x"))
	if err != nil {
		return nil, &inputError{"request.migrationPlanQuote.signature.signature must be valid hex"}
	}
	if !ed25519.Verify(publicKey, signingHash, rawSignature) {
		return nil, &inputError{"request.migrationPlanQuote.signature does not verify against the configured trust root"}
	}

	if options.requireFreshMigrationPlanQuote {
		verificationNow := options.migrationPlanQuoteVerificationNow
		if verificationNow.IsZero() {
			verificationNow = time.Now().UTC()
		}
		// Submit freshness is intentionally strict. Poll omits this check so
		// already-accepted jobs remain addressable after quote expiry; operators
		// must keep the destination service and keep-core on synchronized UTC
		// time when enforcing quote freshness.
		if expiresAt.Before(verificationNow) {
			return nil, &inputError{"request.migrationPlanQuote is expired"}
		}
	}

	return normalizedQuote, nil
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
	payload, err := marshalCanonicalJSON(destinationCommitmentPayload{
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
	payload, err := marshalCanonicalJSON(migrationPlanCommitmentPayload{
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

func requiredStructuredArtifactApprovalRoles(route TemplateID) ([]ArtifactApprovalRole, error) {
	switch route {
	case TemplateQcV1:
		return []ArtifactApprovalRole{
			ArtifactApprovalRoleDepositor,
			ArtifactApprovalRoleCustodian,
		}, nil
	case TemplateSelfV1:
		return []ArtifactApprovalRole{
			ArtifactApprovalRoleDepositor,
		}, nil
	default:
		return nil, &inputError{"unsupported request.route"}
	}
}

func validateArtifactApprovals(route TemplateID, request RouteSubmitRequest) error {
	_, _, _, err := normalizeArtifactApprovals(route, request)
	return err
}

func normalizeArtifactApprovals(
	route TemplateID,
	request RouteSubmitRequest,
) (*ArtifactApprovalEnvelope, *SignerApprovalCertificate, []string, error) {
	normalizedSignerApproval, err := normalizeSignerApprovalCertificate(request)
	if err != nil {
		return nil, nil, nil, err
	}

	normalizedLegacySignatures, err := validateArtifactSignatures(request.ArtifactSignatures)
	if err != nil {
		return nil, nil, nil, err
	}

	if request.ArtifactApprovals == nil {
		return nil, normalizedSignerApproval, normalizedLegacySignatures, nil
	}
	if request.MigrationTransactionPlan == nil {
		return nil, nil, nil, &inputError{"request.migrationTransactionPlan is required when request.artifactApprovals is present"}
	}

	if request.ArtifactApprovals.Payload.ApprovalVersion != artifactApprovalVersion {
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.approvalVersion must equal 1"}
	}
	if request.ArtifactApprovals.Payload.Route != route {
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.route must match request.route"}
	}
	if request.ArtifactApprovals.Payload.ScriptTemplateID != route {
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.scriptTemplateId must match request.route"}
	}
	if err := validateBytes32HexString(
		"request.artifactApprovals.payload.destinationCommitmentHash",
		request.ArtifactApprovals.Payload.DestinationCommitmentHash,
	); err != nil {
		return nil, nil, nil, err
	}
	if err := validateBytes32HexString(
		"request.artifactApprovals.payload.planCommitmentHash",
		request.ArtifactApprovals.Payload.PlanCommitmentHash,
	); err != nil {
		return nil, nil, nil, err
	}

	normalizedDestinationCommitmentHash := normalizeLowerHex(
		request.ArtifactApprovals.Payload.DestinationCommitmentHash,
	)
	if normalizedDestinationCommitmentHash != normalizeLowerHex(request.DestinationCommitmentHash) {
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.destinationCommitmentHash must match request.destinationCommitmentHash"}
	}

	normalizedPlanCommitmentHash := normalizeLowerHex(
		request.ArtifactApprovals.Payload.PlanCommitmentHash,
	)
	if normalizedPlanCommitmentHash != normalizeLowerHex(request.MigrationTransactionPlan.PlanCommitmentHash) {
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.planCommitmentHash must match request.migrationTransactionPlan.planCommitmentHash"}
	}
	if len(request.ArtifactApprovals.Approvals) == 0 {
		return nil, nil, nil, &inputError{"request.artifactApprovals.approvals must not be empty"}
	}

	requiredRoles, err := requiredStructuredArtifactApprovalRoles(route)
	if err != nil {
		return nil, nil, nil, err
	}

	allowedRoles := make(map[ArtifactApprovalRole]struct{}, len(requiredRoles))
	for _, role := range requiredRoles {
		allowedRoles[role] = struct{}{}
	}

	approvalsByRole := make(map[ArtifactApprovalRole]string, len(requiredRoles))
	for i, approval := range request.ArtifactApprovals.Approvals {
		if _, ok := allowedRoles[approval.Role]; !ok {
			return nil, nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals[%d].role is not allowed for %s",
				i,
				route,
			)}
		}
		if _, ok := approvalsByRole[approval.Role]; ok {
			return nil, nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals[%d].role duplicates role %s",
				i,
				approval.Role,
			)}
		}
		if err := validateHexString(
			fmt.Sprintf("request.artifactApprovals.approvals[%d].signature", i),
			approval.Signature,
		); err != nil {
			return nil, nil, nil, err
		}

		approvalsByRole[approval.Role] = normalizeLowerHex(approval.Signature)
	}

	derivedLegacySignatures := make([]string, 0, len(requiredRoles)+1)
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
			return nil, nil, nil, &inputError{fmt.Sprintf(
				"request.artifactApprovals.approvals must include role %s for %s",
				role,
				route,
			)}
		}

		derivedLegacySignatures = append(derivedLegacySignatures, signature)
		normalizedApprovals.Approvals[i] = ArtifactRoleApproval{
			Role:      role,
			Signature: signature,
		}
	}

	if normalizedSignerApproval != nil {
		derivedLegacySignatures = append(
			derivedLegacySignatures,
			normalizedSignerApproval.Signature,
		)
	}

	canonicalSignatureError := "request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals"
	if normalizedSignerApproval != nil {
		canonicalSignatureError = "request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals and request.signerApproval"
	}

	if len(normalizedLegacySignatures) != len(derivedLegacySignatures) {
		return nil, nil, nil, &inputError{canonicalSignatureError}
	}
	for i := range derivedLegacySignatures {
		if normalizedLegacySignatures[i] != derivedLegacySignatures[i] {
			return nil, nil, nil, &inputError{canonicalSignatureError}
		}
	}

	return normalizedApprovals, normalizedSignerApproval, derivedLegacySignatures, nil
}

func validateArtifactApprovalAuthenticity(
	request RouteSubmitRequest,
	depositorPublicKey string,
	custodianPublicKey string,
) error {
	payloadDigest, err := artifactApprovalDigest(request.ArtifactApprovals.Payload)
	if err != nil {
		return err
	}

	depositorKey, err := parseCompressedSecp256k1PublicKey(
		"request.scriptTemplate.depositorPublicKey",
		depositorPublicKey,
	)
	if err != nil {
		return err
	}

	var custodianKey *btcec.PublicKey
	if custodianPublicKey != "" {
		custodianKey, err = parseCompressedSecp256k1PublicKey(
			"request.scriptTemplate.custodianPublicKey",
			custodianPublicKey,
		)
		if err != nil {
			return err
		}
	}

	for i, approval := range request.ArtifactApprovals.Approvals {
		signaturePath := fmt.Sprintf(
			"request.artifactApprovals.approvals[%d].signature",
			i,
		)

		switch approval.Role {
		case ArtifactApprovalRoleDepositor:
			if err := verifySecp256k1Signature(
				signaturePath,
				depositorKey,
				payloadDigest,
				approval.Signature,
			); err != nil {
				return err
			}
		case ArtifactApprovalRoleCustodian:
			if custodianKey == nil {
				return &inputError{
					"request.artifactApprovals.approvals includes unexpected custodian role",
				}
			}
			if err := verifySecp256k1Signature(
				signaturePath,
				custodianKey,
				payloadDigest,
				approval.Signature,
			); err != nil {
				return err
			}
		}
	}

	return nil
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

func normalizeRouteSubmitRequest(
	request RouteSubmitRequest,
	options ...validationOptions,
) (RouteSubmitRequest, error) {
	resolvedOptions := resolveValidationOptions(options)
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
		resolvedOptions,
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
	options ...validationOptions,
) error {
	resolvedOptions := resolveValidationOptions(options)
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
	if _, err := normalizeMigrationPlanQuote(request, resolvedOptions); err != nil {
		return err
	}
	if request.ArtifactApprovals == nil {
		return &inputError{"request.artifactApprovals is required"}
	}
	if resolvedOptions.signerApprovalVerifier != nil && request.SignerApproval == nil {
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
		if len(resolvedOptions.depositorTrustRoots) > 0 {
			expectedDepositorPublicKey, ok := resolveExpectedDepositorPublicKey(
				request,
				resolvedOptions.depositorTrustRoots,
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
		if len(resolvedOptions.depositorTrustRoots) > 0 {
			expectedDepositorPublicKey, ok := resolveExpectedDepositorPublicKey(
				request,
				resolvedOptions.depositorTrustRoots,
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
		if len(resolvedOptions.custodianTrustRoots) > 0 {
			expectedCustodianPublicKey, ok := resolveExpectedCustodianPublicKey(
				request,
				resolvedOptions.custodianTrustRoots,
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
		if resolvedOptions.signerApprovalVerifier == nil {
			return &inputError{
				"request.signerApproval cannot be verified by this signer deployment",
			}
		}

		normalizedRequest, err := normalizeRouteSubmitRequest(request, resolvedOptions)
		if err != nil {
			return err
		}

		if err := resolvedOptions.signerApprovalVerifier.VerifySignerApproval(
			normalizedRequest,
		); err != nil {
			return err
		}
	}

	return nil
}

func validateSubmitInput(
	route TemplateID,
	input SignerSubmitInput,
	options ...validationOptions,
) error {
	if input.RouteRequestID == "" {
		return &inputError{"routeRequestId is required"}
	}
	if input.Stage != StageSignerCoordination {
		return &inputError{"stage must be SIGNER_COORDINATION"}
	}
	return validateCommonRequest(route, input.Request, resolveValidationOptions(options))
}

func validatePollInput(
	route TemplateID,
	input SignerPollInput,
	options ...validationOptions,
) error {
	if input.RequestID == "" {
		return &inputError{"requestId is required"}
	}
	if err := validateSubmitInput(route, SignerSubmitInput{
		RouteRequestID: input.RouteRequestID,
		Request:        input.Request,
		Stage:          input.Stage,
	}, resolveValidationOptions(options)); err != nil {
		return err
	}
	return nil
}
