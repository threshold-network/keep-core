package tbtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

const (
	signerApprovalCertificateVersion            uint32 = 1
	signerApprovalCertificateSignatureAlgorithm        = "tecdsa-secp256k1"
	signerApprovalCertificateSignerSetDomain           = "covenant-signer-set-v1:"
)

// signerApprovalCertificate is a spike artifact for evaluating whether the
// current tECDSA signer stack can emit a single offline-verifiable `S`
// approval over an arbitrary approval digest.
type signerApprovalCertificate struct {
	CertificateVersion uint32   `json:"certificateVersion"`
	SignatureAlgorithm string   `json:"signatureAlgorithm"`
	WalletPublicKey    string   `json:"walletPublicKey"`
	SignerSetHash      string   `json:"signerSetHash"`
	ApprovalDigest     string   `json:"approvalDigest"`
	Signature          string   `json:"signature"`
	ActiveMembers      []uint32 `json:"activeMembers,omitempty"`
	InactiveMembers    []uint32 `json:"inactiveMembers,omitempty"`
	EndBlock           uint64   `json:"endBlock"`
}

type signerApprovalCertificateSignerSetPayload struct {
	WalletPublicKey       string   `json:"walletPublicKey"`
	SigningGroupOperators []string `json:"signingGroupOperators"`
	HonestThreshold       int      `json:"honestThreshold"`
}

func (se *signingExecutor) issueSignerApprovalCertificate(
	ctx context.Context,
	approvalDigest []byte,
	startBlock uint64,
) (*signerApprovalCertificate, error) {
	if len(approvalDigest) != sha256.Size {
		return nil, fmt.Errorf(
			"approval digest must be exactly %d bytes",
			sha256.Size,
		)
	}

	signature, activityReport, endBlock, err := se.sign(
		ctx,
		new(big.Int).SetBytes(approvalDigest),
		startBlock,
	)
	if err != nil {
		return nil, err
	}

	return buildSignerApprovalCertificate(
		se.wallet(),
		se.groupParameters,
		approvalDigest,
		signature,
		activityReport,
		endBlock,
	)
}

func buildSignerApprovalCertificate(
	wallet wallet,
	groupParameters *GroupParameters,
	approvalDigest []byte,
	signature *tecdsa.Signature,
	activityReport *signingActivityReport,
	endBlock uint64,
) (*signerApprovalCertificate, error) {
	if len(approvalDigest) != sha256.Size {
		return nil, fmt.Errorf(
			"approval digest must be exactly %d bytes",
			sha256.Size,
		)
	}
	if groupParameters == nil {
		return nil, fmt.Errorf("group parameters are required")
	}
	if signature == nil || signature.R == nil || signature.S == nil {
		return nil, fmt.Errorf("threshold signature is required")
	}

	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		return nil, err
	}

	signerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		wallet,
		groupParameters,
	)
	if err != nil {
		return nil, err
	}

	signatureBytes := (&btcec.Signature{
		R: signature.R,
		S: signature.S,
	}).Serialize()

	certificate := &signerApprovalCertificate{
		CertificateVersion: signerApprovalCertificateVersion,
		SignatureAlgorithm: signerApprovalCertificateSignatureAlgorithm,
		WalletPublicKey:    "0x" + hex.EncodeToString(walletPublicKeyBytes),
		SignerSetHash:      signerSetHash,
		ApprovalDigest:     "0x" + hex.EncodeToString(approvalDigest),
		Signature:          "0x" + hex.EncodeToString(signatureBytes),
		EndBlock:           endBlock,
	}

	if activityReport != nil {
		certificate.ActiveMembers = normalizeSignerApprovalMemberIndexes(
			activityReport.activeMembers,
		)
		certificate.InactiveMembers = normalizeSignerApprovalMemberIndexes(
			activityReport.inactiveMembers,
		)
	}

	return certificate, nil
}

func computeSignerApprovalCertificateSignerSetHash(
	wallet wallet,
	groupParameters *GroupParameters,
) (string, error) {
	if groupParameters == nil {
		return "", fmt.Errorf("group parameters are required")
	}

	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		return "", err
	}

	signingGroupOperators := make([]string, len(wallet.signingGroupOperators))
	for i, operator := range wallet.signingGroupOperators {
		signingGroupOperators[i] = operator.String()
	}

	payload, err := json.Marshal(signerApprovalCertificateSignerSetPayload{
		WalletPublicKey:       "0x" + hex.EncodeToString(walletPublicKeyBytes),
		SigningGroupOperators: signingGroupOperators,
		HonestThreshold:       groupParameters.HonestThreshold,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(
		append([]byte(signerApprovalCertificateSignerSetDomain), payload...),
	)

	return "0x" + hex.EncodeToString(sum[:]), nil
}

func verifySignerApprovalCertificate(
	certificate *signerApprovalCertificate,
	expectedSignerSetHash string,
) error {
	if certificate == nil {
		return fmt.Errorf("certificate is required")
	}
	if certificate.CertificateVersion != signerApprovalCertificateVersion {
		return fmt.Errorf("unsupported certificate version: %d", certificate.CertificateVersion)
	}
	if certificate.SignatureAlgorithm != signerApprovalCertificateSignatureAlgorithm {
		return fmt.Errorf("unsupported signature algorithm: %s", certificate.SignatureAlgorithm)
	}
	if expectedSignerSetHash != "" &&
		strings.ToLower(expectedSignerSetHash) != strings.ToLower(certificate.SignerSetHash) {
		return fmt.Errorf("signer set hash does not match the expected signer set")
	}

	approvalDigest, err := decodeSignerApprovalCertificateHex(
		certificate.ApprovalDigest,
		sha256.Size,
	)
	if err != nil {
		return fmt.Errorf("invalid approval digest: %w", err)
	}
	signatureBytes, err := decodeSignerApprovalCertificateHex(
		certificate.Signature,
		0,
	)
	if err != nil {
		return fmt.Errorf("invalid threshold signature: %w", err)
	}
	walletPublicKeyBytes, err := decodeSignerApprovalCertificateHex(
		certificate.WalletPublicKey,
		0,
	)
	if err != nil {
		return fmt.Errorf("invalid wallet public key: %w", err)
	}

	walletPublicKey := unmarshalPublicKey(walletPublicKeyBytes)
	if walletPublicKey == nil || walletPublicKey.X == nil || walletPublicKey.Y == nil {
		return fmt.Errorf("wallet public key is not a valid uncompressed secp256k1 key")
	}

	parsedSignature, err := btcec.ParseDERSignature(signatureBytes, btcec.S256())
	if err != nil {
		return fmt.Errorf("cannot parse threshold signature: %w", err)
	}

	if !ecdsa.Verify(walletPublicKey, approvalDigest, parsedSignature.R, parsedSignature.S) {
		return fmt.Errorf("threshold signature does not verify against wallet public key")
	}

	return nil
}

func decodeSignerApprovalCertificateHex(
	value string,
	expectedBytes int,
) ([]byte, error) {
	normalized := strings.TrimSpace(value)
	if !strings.HasPrefix(normalized, "0x") {
		return nil, fmt.Errorf("value must be 0x-prefixed")
	}

	decoded, err := hex.DecodeString(strings.TrimPrefix(normalized, "0x"))
	if err != nil {
		return nil, err
	}
	if expectedBytes > 0 && len(decoded) != expectedBytes {
		return nil, fmt.Errorf(
			"value must be exactly %d bytes, got %d",
			expectedBytes,
			len(decoded),
		)
	}

	return decoded, nil
}

func normalizeSignerApprovalMemberIndexes(
	memberIndexes []group.MemberIndex,
) []uint32 {
	normalized := make([]uint32, len(memberIndexes))
	for i, memberIndex := range memberIndexes {
		normalized[i] = uint32(memberIndex)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})
	return normalized
}
