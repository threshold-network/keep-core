package covenantsigner

import (
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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

func normalizeSignerApprovalCertificate(
	request RouteSubmitRequest,
	chainID uint64,
	salt [32]byte,
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
		// This must match tbtc marshalPublicKey/unmarshalPublicKey:
		// uncompressed SEC1 public key (0x04 + 64-byte coordinates).
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

	expectedApprovalDigest, err := artifactApprovalDigest(
		request.ArtifactApprovals.Payload,
		chainID,
		salt,
	)
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

	// EndBlock is a required v2 certificate field: a missing or null EndBlock
	// must fail closed rather than being treated as "never expires". Unlike
	// request.maturityHeight (a Bitcoin nLockTime-style field genuinely
	// bounded to uint32), EndBlock is a host-chain block number bound into
	// the certificate v2 signing digest as a full 8-byte big-endian uint64
	// (see signerApprovalCertificateSigningDigest in pkg/tbtc), so it must
	// accept the entire uint64 range here too -- rejecting values above
	// math.MaxUint32 would contradict that contract.
	if signerApproval.EndBlock == nil {
		return nil, &inputError{
			"request.signerApproval.endBlock is required",
		}
	}
	endBlock := *signerApproval.EndBlock
	normalizedSignerApproval.EndBlock = &endBlock

	return normalizedSignerApproval, nil
}

func abiEncodeUint32Word(value uint32) [32]byte {
	var encoded [32]byte
	binary.BigEndian.PutUint32(encoded[28:], value)
	return encoded
}

// abiEncodeUint64Word right-aligns a uint64 into a 32-byte word, the ABI
// encoding of a uint256 whose value fits in 64 bits (e.g. an EIP-712 chainId).
func abiEncodeUint64Word(value uint64) [32]byte {
	var encoded [32]byte
	binary.BigEndian.PutUint64(encoded[24:], value)
	return encoded
}

func keccakTemplateIdentifier(id TemplateID) [32]byte {
	hash := crypto.Keccak256Hash([]byte(id))

	var encoded [32]byte
	copy(encoded[:], hash.Bytes())

	return encoded
}

// artifactApprovalStructHash computes the EIP-712 hashStruct of the
// ArtifactApproval message. It is the inner half of the domain-wrapped digest.
func artifactApprovalStructHash(payload ArtifactApprovalPayload) ([]byte, error) {
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

	return crypto.Keccak256(encoded), nil
}

// artifactApprovalDomainSeparator computes the EIP-712 domain separator for the
// artifact approval (name, version, chainId, salt; no verifyingContract).
func artifactApprovalDomainSeparator(chainID uint64, salt [32]byte) [32]byte {
	encoded := make([]byte, 32*5)
	chainIDWord := abiEncodeUint64Word(chainID)
	nameHash := crypto.Keccak256Hash([]byte(artifactApprovalDomainName))
	versionHash := crypto.Keccak256Hash([]byte(artifactApprovalDomainVersion))

	copy(encoded[0:32], eip712DomainTypeHash.Bytes())
	copy(encoded[32:64], nameHash.Bytes())
	copy(encoded[64:96], versionHash.Bytes())
	copy(encoded[96:128], chainIDWord[:])
	copy(encoded[128:160], salt[:])

	var separator [32]byte
	copy(separator[:], crypto.Keccak256(encoded))
	return separator
}

// artifactApprovalDigest computes the domain-wrapped EIP-712 v4 digest that a
// wallet's eth_signTypedData_v4 signature is produced over:
// keccak256(0x1901 ‖ domainSeparator ‖ hashStruct(message)). The digest is
// chain-specific: chainID and salt define the domain and must match the client.
func artifactApprovalDigest(
	payload ArtifactApprovalPayload,
	chainID uint64,
	salt [32]byte,
) ([]byte, error) {
	structHash, err := artifactApprovalStructHash(payload)
	if err != nil {
		return nil, err
	}

	domainSeparator := artifactApprovalDomainSeparator(chainID, salt)

	prefixed := make([]byte, 0, 2+32+32)
	prefixed = append(prefixed, 0x19, 0x01)
	prefixed = append(prefixed, domainSeparator[:]...)
	prefixed = append(prefixed, structHash...)

	digest := crypto.Keccak256Hash(prefixed)
	return digest.Bytes(), nil
}

// ComputeArtifactApprovalDigest exposes the v2 domain-wrapped approval digest to
// cross-package verifiers that need to bind signerApproval.approvalDigest to
// request.artifactApprovals.payload. chainID and salt must match the signer's
// configured EIP-712 domain.
func ComputeArtifactApprovalDigest(
	payload ArtifactApprovalPayload,
	chainID uint64,
	salt [32]byte,
) ([]byte, error) {
	return artifactApprovalDigest(payload, chainID, salt)
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

func isLowSSecp256k1(s *big.Int) bool {
	halfOrder := new(big.Int).Rsh(new(big.Int).Set(btcec.S256().N), 1)
	return s.Cmp(halfOrder) <= 0
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
		if !isLowSSecp256k1(new(big.Int).SetBytes(rawSignature[32:])) {
			return &inputError{fmt.Sprintf("%s must be a low-S secp256k1 signature", name)}
		}
		if verifyCompactSecp256k1Signature(publicKey, digest, rawSignature) {
			return nil
		}
	case len(rawSignature) == 65 &&
		(rawSignature[64] == 0 || rawSignature[64] == 1 || rawSignature[64] == 27 || rawSignature[64] == 28):
		if !isLowSSecp256k1(new(big.Int).SetBytes(rawSignature[32:64])) {
			return &inputError{fmt.Sprintf("%s must be a low-S secp256k1 signature", name)}
		}
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
		if !isLowSSecp256k1(parsedSignature.S) {
			return &inputError{fmt.Sprintf("%s must be a low-S secp256k1 signature", name)}
		}
		if parsedSignature.Verify(digest, publicKey) {
			return nil
		}
	}

	return &inputError{fmt.Sprintf("%s does not verify against the required public key", name)}
}

// verifyEthSignature verifies a wallet-produced ECDSA signature over the digest
// against a pinned depositor ETH address (ecrecover-and-compare). It accepts the
// 65-byte r‖s‖v shape that eth_signTypedData_v4 returns (v in {27,28}, also
// tolerating {0,1}) and enforces low-S per EIP-2.
func verifyEthSignature(
	name string,
	ethAddress string,
	digest []byte,
	signature string,
) error {
	rawSignature, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return &inputError{fmt.Sprintf("%s must be valid hex", name)}
	}
	if len(rawSignature) != 65 {
		return &inputError{
			fmt.Sprintf("%s must be a 65-byte secp256k1 signature", name),
		}
	}
	if !isLowSSecp256k1(new(big.Int).SetBytes(rawSignature[32:64])) {
		return &inputError{
			fmt.Sprintf("%s must be a low-S secp256k1 signature", name),
		}
	}

	// crypto.SigToPub expects the recovery id in {0,1}; wallets emit {27,28}.
	normalized := make([]byte, 65)
	copy(normalized, rawSignature)
	switch normalized[64] {
	case 27, 28:
		normalized[64] -= 27
	case 0, 1:
		// already normalized
	default:
		return &inputError{
			fmt.Sprintf("%s has an invalid recovery id", name),
		}
	}

	publicKey, err := crypto.SigToPub(digest, normalized)
	if err != nil {
		return &inputError{
			fmt.Sprintf("%s does not recover to a valid public key", name),
		}
	}

	recovered := crypto.PubkeyToAddress(*publicKey)
	if recovered != common.HexToAddress(ethAddress) {
		return &inputError{
			fmt.Sprintf("%s does not verify against the required depositor ETH address", name),
		}
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

func validateArtifactApprovals(
	route TemplateID,
	request RouteSubmitRequest,
	chainID uint64,
	salt [32]byte,
) error {
	_, _, _, err := normalizeArtifactApprovals(route, request, chainID, salt)
	return err
}

func normalizeArtifactApprovals(
	route TemplateID,
	request RouteSubmitRequest,
	chainID uint64,
	salt [32]byte,
) (*ArtifactApprovalEnvelope, *SignerApprovalCertificate, []string, error) {
	normalizedSignerApproval, err := normalizeSignerApprovalCertificate(request, chainID, salt)
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
		return nil, nil, nil, &inputError{"request.artifactApprovals.payload.approvalVersion must equal 2"}
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

// validateArtifactApprovalAuthenticity verifies each artifact approval signature
// against the payload digest. The depositor approval is verified against a
// pinned depositor ETH identity via ecrecover-and-compare when depositorEthAddress
// is set (the v2 wallet-signed path); otherwise it falls back to the
// script-template secp256k1 depositor key. The custodian approval always uses the
// secp256k1 custodian key.
func validateArtifactApprovalAuthenticity(
	request RouteSubmitRequest,
	depositorPublicKey string,
	depositorEthAddress string,
	custodianPublicKey string,
	chainID uint64,
	salt [32]byte,
) error {
	payloadDigest, err := artifactApprovalDigest(
		request.ArtifactApprovals.Payload,
		chainID,
		salt,
	)
	if err != nil {
		return err
	}

	var depositorKey *btcec.PublicKey
	if depositorEthAddress == "" {
		depositorKey, err = parseCompressedSecp256k1PublicKey(
			"request.scriptTemplate.depositorPublicKey",
			depositorPublicKey,
		)
		if err != nil {
			return err
		}
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
			if depositorEthAddress != "" {
				if err := verifyEthSignature(
					signaturePath,
					depositorEthAddress,
					payloadDigest,
					approval.Signature,
				); err != nil {
					return err
				}
			} else if err := verifySecp256k1Signature(
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
		default:
			// normalizeArtifactApprovals already rejects unrecognized roles
			// before this loop runs; fail closed here as defense in depth so an
			// unexpected role can never be silently skipped without verification.
			return &inputError{
				fmt.Sprintf(
					"request.artifactApprovals.approvals[%d].role is not a recognized role",
					i,
				),
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

// resolveExpectedDepositorEthAddress returns the pinned depositor ETH address
// for the request scope, if one is configured. An empty return means no ETH
// identity is pinned and approval verification falls back to the secp256k1
// depositor key.
func resolveExpectedDepositorEthAddress(
	request RouteSubmitRequest,
	trustRoots []DepositorTrustRoot,
) string {
	route, reserve, network := trustRootLookupScope(request)
	for _, trustRoot := range trustRoots {
		if trustRoot.Route == route &&
			trustRoot.Reserve == reserve &&
			trustRoot.Network == network {
			return trustRoot.EthAddress
		}
	}

	return ""
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

// deriveIssuanceApprovalDigest verifies that artifactApprovals carries
// exactly the roles required for route, each authentically signed by the
// depositor (and, for qc_v1, custodian) identity pinned in this node's own
// depositorTrustRoots/custodianTrustRoots for the (route, reserve, network)
// scope -- the same authenticity check validateArtifactApprovalAuthenticity
// runs on the Submit path -- then returns the EIP-712 digest this node
// derived from the authenticated payload. It never trusts a caller-asserted
// digest: the issuer only ever signs a digest it computed itself.
func deriveIssuanceApprovalDigest(
	route TemplateID,
	reserve string,
	network string,
	artifactApprovals ArtifactApprovalEnvelope,
	depositorTrustRoots []DepositorTrustRoot,
	custodianTrustRoots []CustodianTrustRoot,
	chainID uint64,
	salt [32]byte,
) ([]byte, error) {
	requiredRoles, err := requiredStructuredArtifactApprovalRoles(route)
	if err != nil {
		return nil, err
	}

	payload := artifactApprovals.Payload
	if payload.ApprovalVersion != artifactApprovalVersion {
		return nil, &inputError{"artifactApprovals.payload.approvalVersion must equal 2"}
	}
	if payload.Route != route {
		return nil, &inputError{"artifactApprovals.payload.route must match route"}
	}
	if payload.ScriptTemplateID != route {
		return nil, &inputError{"artifactApprovals.payload.scriptTemplateId must match route"}
	}
	if err := validateBytes32HexString(
		"artifactApprovals.payload.destinationCommitmentHash",
		payload.DestinationCommitmentHash,
	); err != nil {
		return nil, err
	}
	if err := validateBytes32HexString(
		"artifactApprovals.payload.planCommitmentHash",
		payload.PlanCommitmentHash,
	); err != nil {
		return nil, err
	}
	if err := validateIssuanceApprovalRoles(artifactApprovals.Approvals, requiredRoles); err != nil {
		return nil, err
	}

	// trustRootLookupScope only reads Route, Reserve, and the network of
	// whichever destination reservation is set. RedeemDestination is used
	// purely as a carrier for the (reserve, network) trust-root scope here --
	// certificate issuance has no real destination reservation of its own.
	scopeRequest := RouteSubmitRequest{
		Route:   route,
		Reserve: reserve,
		RedeemDestination: &RedeemDestinationReservation{
			Network: strings.ToLower(strings.TrimSpace(network)),
		},
	}

	depositorPublicKey, hasDepositorTrustRoot := resolveExpectedDepositorPublicKey(
		scopeRequest,
		depositorTrustRoots,
	)
	if !hasDepositorTrustRoot {
		return nil, &inputError{
			"no depositorTrustRoots entry is configured for the requested route/reserve/network; certificate issuance requires a pinned depositor identity",
		}
	}
	depositorEthAddress := resolveExpectedDepositorEthAddress(scopeRequest, depositorTrustRoots)

	var custodianPublicKey string
	if containsArtifactApprovalRole(requiredRoles, ArtifactApprovalRoleCustodian) {
		expectedCustodianPublicKey, hasCustodianTrustRoot := resolveExpectedCustodianPublicKey(
			scopeRequest,
			custodianTrustRoots,
		)
		if !hasCustodianTrustRoot {
			return nil, &inputError{
				"no custodianTrustRoots entry is configured for the requested route/reserve/network; certificate issuance requires a pinned custodian identity",
			}
		}
		custodianPublicKey = expectedCustodianPublicKey
	}

	authenticityRequest := RouteSubmitRequest{ArtifactApprovals: &artifactApprovals}
	if err := validateArtifactApprovalAuthenticity(
		authenticityRequest,
		depositorPublicKey,
		depositorEthAddress,
		custodianPublicKey,
		chainID,
		salt,
	); err != nil {
		return nil, err
	}

	return ComputeArtifactApprovalDigest(payload, chainID, salt)
}

func validateIssuanceApprovalRoles(
	approvals []ArtifactRoleApproval,
	requiredRoles []ArtifactApprovalRole,
) error {
	if len(approvals) != len(requiredRoles) {
		return &inputError{"artifactApprovals.approvals must include exactly the roles required for route"}
	}

	seenRoles := make(map[ArtifactApprovalRole]struct{}, len(requiredRoles))
	for i, approval := range approvals {
		if !containsArtifactApprovalRole(requiredRoles, approval.Role) {
			return &inputError{fmt.Sprintf(
				"artifactApprovals.approvals[%d].role is not allowed for route",
				i,
			)}
		}
		if _, ok := seenRoles[approval.Role]; ok {
			return &inputError{fmt.Sprintf(
				"artifactApprovals.approvals[%d].role duplicates role %s",
				i,
				approval.Role,
			)}
		}
		if err := validateHexString(
			fmt.Sprintf("artifactApprovals.approvals[%d].signature", i),
			approval.Signature,
		); err != nil {
			return err
		}
		seenRoles[approval.Role] = struct{}{}
	}

	return nil
}

func containsArtifactApprovalRole(roles []ArtifactApprovalRole, role ArtifactApprovalRole) bool {
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}
