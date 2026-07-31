package covenantsigner

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
)

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

// normalizeScopedTrustRoots validates and normalizes a slice of trust roots
// of any scoped-approval type. getFields extracts (Route, Reserve, Network,
// PublicKey) from each element; build constructs a normalized element from
// the validated fields. Duplicates within the same route/reserve/network
// scope are rejected.
func normalizeScopedTrustRoots[T any](
	typeName string,
	trustRoots []T,
	getFields func(T) (TemplateID, string, string, string),
	build func(route TemplateID, reserve, network, publicKey string) T,
) ([]T, error) {
	if len(trustRoots) == 0 {
		return nil, nil
	}

	normalized := make([]T, len(trustRoots))
	seen := make(map[string]int, len(trustRoots))

	for i, trustRoot := range trustRoots {
		name := fmt.Sprintf("%s[%d]", typeName, i)
		r, res, net, pk := getFields(trustRoot)
		route, reserve, network, publicKey, err := normalizeScopedApprovalTrustRoot(
			name, r, res, net, pk,
		)
		if err != nil {
			return nil, err
		}

		scopeKey := string(route) + "|" + reserve + "|" + network
		if previousIndex, ok := seen[scopeKey]; ok {
			return nil, &inputError{
				fmt.Sprintf(
					"%s duplicates %s[%d] for route %s reserve %s network %s",
					name,
					typeName,
					previousIndex,
					route,
					reserve,
					network,
				),
			}
		}
		seen[scopeKey] = i
		normalized[i] = build(route, reserve, network, publicKey)
	}

	return normalized, nil
}

func normalizeDepositorTrustRoots(
	trustRoots []DepositorTrustRoot,
) ([]DepositorTrustRoot, error) {
	normalized, err := normalizeScopedTrustRoots(
		"depositorTrustRoots",
		trustRoots,
		func(t DepositorTrustRoot) (TemplateID, string, string, string) {
			return t.Route, t.Reserve, t.Network, t.PublicKey
		},
		func(route TemplateID, reserve, network, publicKey string) DepositorTrustRoot {
			return DepositorTrustRoot{
				Route: route, Reserve: reserve,
				Network: network, PublicKey: publicKey,
			}
		},
	)
	if err != nil {
		return nil, err
	}

	// normalizeScopedTrustRoots preserves input order, so entries are
	// index-aligned with trustRoots. Attach the optional pinned depositor ETH
	// address (enables ecrecover-based v2 approval verification).
	for i := range normalized {
		ethAddress := strings.TrimSpace(trustRoots[i].EthAddress)
		if ethAddress == "" {
			continue
		}
		normalizedEth, err := normalizeEthAddress(
			fmt.Sprintf("depositorTrustRoots[%d].ethAddress", i),
			ethAddress,
		)
		if err != nil {
			return nil, err
		}
		normalized[i].EthAddress = normalizedEth
	}

	// Prevent a silent ETH->secp verification downgrade: within a single
	// (route, reserve) pair, ethAddress must be set on every network entry or on
	// none of them. A mix would let a request steer verification to the
	// secp-only sibling scope through its (partially caller-influenced) network
	// value, bypassing an operator's intended wallet-signed enforcement.
	ethPresenceByPair := make(map[string]bool, len(normalized))
	for i := range normalized {
		pairKey := string(normalized[i].Route) + "|" + normalized[i].Reserve
		hasEth := normalized[i].EthAddress != ""
		if existing, ok := ethPresenceByPair[pairKey]; ok {
			if existing != hasEth {
				return nil, &inputError{
					fmt.Sprintf(
						"depositorTrustRoots for route %s reserve %s must set ethAddress on all network entries or on none",
						normalized[i].Route,
						normalized[i].Reserve,
					),
				}
			}
		} else {
			ethPresenceByPair[pairKey] = hasEth
		}
	}

	return normalized, nil
}

// normalizeEthAddress validates a 20-byte hex Ethereum address and returns it in
// lowercase 0x-prefixed form.
func normalizeEthAddress(name, value string) (string, error) {
	trimmed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	raw, err := hex.DecodeString(trimmed)
	if err != nil || len(raw) != 20 {
		return "", &inputError{
			fmt.Sprintf("%s must be a 20-byte hex ETH address", name),
		}
	}
	// Reject the zero address: an operator misconfiguring it would make the
	// ecrecover-based approval check compare against 0x0, silently weakening the
	// depositor binding to whatever recovers to the zero address.
	if strings.Trim(trimmed, "0") == "" {
		return "", &inputError{
			fmt.Sprintf("%s must not be the zero ETH address", name),
		}
	}
	return "0x" + trimmed, nil
}

func normalizeCustodianTrustRoots(
	trustRoots []CustodianTrustRoot,
) ([]CustodianTrustRoot, error) {
	return normalizeScopedTrustRoots(
		"custodianTrustRoots",
		trustRoots,
		func(t CustodianTrustRoot) (TemplateID, string, string, string) {
			return t.Route, t.Reserve, t.Network, t.PublicKey
		},
		func(route TemplateID, reserve, network, publicKey string) CustodianTrustRoot {
			return CustodianTrustRoot{
				Route: route, Reserve: reserve,
				Network: network, PublicKey: publicKey,
			}
		},
	)
}

func trustRootLookupScope(request RouteSubmitRequest) (TemplateID, string, string) {
	// The trust-root scope network comes from the destination reservation for the
	// request's action (migration/redeem/renew).
	network := ""
	switch {
	case request.MigrationDestination != nil:
		network = request.MigrationDestination.Network
	case request.RedeemDestination != nil:
		network = request.RedeemDestination.Network
	case request.RenewDestination != nil:
		network = request.RenewDestination.Network
	}

	return request.Route, normalizeLowerHex(request.Reserve), strings.ToLower(strings.TrimSpace(network))
}

func migrationPlanQuoteSigningPayloadBytes(
	quote *MigrationDestinationPlanQuote,
) ([]byte, error) {
	return canonicaljson.Marshal(migrationPlanQuoteSigningPayload{
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
		if len(options.migrationPlanQuoteTrustRoots) > 0 && !options.policyIndependentDigest {
			return nil, &inputError{
				"request.migrationPlanQuote is required when migrationPlanQuoteTrustRoots are configured",
			}
		}

		return nil, nil
	}
	if len(options.migrationPlanQuoteTrustRoots) == 0 && !options.policyIndependentDigest {
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
	if !migrationTransactionPlansEqual(normalizedQuotePlan, normalizeMigrationTransactionPlan(request.MigrationTransactionPlan)) {
		return nil, &inputError{"request.migrationPlanQuote.migrationTransactionPlan must match request.migrationTransactionPlan"}
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
	if options.policyIndependentDigest {
		return normalizedQuote, nil
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
