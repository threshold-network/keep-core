package tbtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
	"github.com/keep-network/keep-core/pkg/internal/canonicaljson"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

const (
	signerApprovalCertificateVersion            uint32 = 1
	signerApprovalCertificateSignatureAlgorithm        = "tecdsa-secp256k1"
	signerApprovalCertificateSignerSetDomain           = "covenant-signer-set-v1:"
)

type signerApprovalCertificateSignerSetPayload struct {
	WalletID        string `json:"walletId"`
	WalletPublicKey string `json:"walletPublicKey"`
	MembersIDsHash  string `json:"membersIdsHash"`
	HonestThreshold int    `json:"honestThreshold"`
}

func (se *signingExecutor) issueSignerApprovalCertificate(
	ctx context.Context,
	approvalDigest []byte,
	startBlock uint64,
) (*covenantsigner.SignerApprovalCertificate, error) {
	if len(approvalDigest) != sha256.Size {
		return nil, fmt.Errorf(
			"approval digest must be exactly %d bytes",
			sha256.Size,
		)
	}

	wallet := se.wallet()
	walletChainData, err := se.chain.GetWallet(bitcoin.PublicKeyHash(wallet.publicKey))
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get on-chain wallet data for signer approval certificate: %w",
			err,
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
		wallet,
		walletChainData,
		se.groupParameters,
		approvalDigest,
		signature,
		activityReport,
		endBlock,
	)
}

func buildSignerApprovalCertificate(
	wallet wallet,
	walletChainData *WalletChainData,
	groupParameters *GroupParameters,
	approvalDigest []byte,
	signature *tecdsa.Signature,
	activityReport *signingActivityReport,
	endBlock uint64,
) (*covenantsigner.SignerApprovalCertificate, error) {
	if len(approvalDigest) != sha256.Size {
		return nil, fmt.Errorf(
			"approval digest must be exactly %d bytes",
			sha256.Size,
		)
	}
	if groupParameters == nil {
		return nil, fmt.Errorf("group parameters are required")
	}
	if walletChainData == nil {
		return nil, fmt.Errorf("wallet chain data is required")
	}
	if signature == nil || signature.R == nil || signature.S == nil {
		return nil, fmt.Errorf("threshold signature is required")
	}

	// signerApproval.walletPublicKey intentionally uses uncompressed SEC1
	// encoding (65 bytes, 0x04 prefix) to match wallet-ID derivation and
	// signer-set hash payloads across the signer approval pipeline.
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		return nil, err
	}

	signerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		wallet.publicKey,
		walletChainData,
		groupParameters,
	)
	if err != nil {
		return nil, err
	}

	signatureBytes := (&btcec.Signature{
		R: signature.R,
		S: signature.S,
	}).Serialize()

	certificate := &covenantsigner.SignerApprovalCertificate{
		CertificateVersion: signerApprovalCertificateVersion,
		SignatureAlgorithm: signerApprovalCertificateSignatureAlgorithm,
		WalletPublicKey:    "0x" + hex.EncodeToString(walletPublicKeyBytes),
		SignerSetHash:      signerSetHash,
		ApprovalDigest:     "0x" + hex.EncodeToString(approvalDigest),
		Signature:          "0x" + hex.EncodeToString(signatureBytes),
	}
	certificate.EndBlock = &endBlock

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
	walletPublicKey *ecdsa.PublicKey,
	walletChainData *WalletChainData,
	groupParameters *GroupParameters,
) (string, error) {
	if groupParameters == nil {
		return "", fmt.Errorf("group parameters are required")
	}
	if walletChainData == nil {
		return "", fmt.Errorf("wallet chain data is required")
	}
	if walletChainData.EcdsaWalletID == ([32]byte{}) {
		return "", fmt.Errorf("wallet chain data must include wallet ID")
	}
	if walletChainData.MembersIDsHash == ([32]byte{}) {
		return "", fmt.Errorf("wallet chain data must include members IDs hash")
	}

	// Keep signer-set payload key encoding aligned with certificate issuance:
	// uncompressed SEC1 (65-byte, 0x04-prefixed) wallet public key.
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return "", err
	}

	payload, err := canonicaljson.Marshal(signerApprovalCertificateSignerSetPayload{
		WalletID:        "0x" + hex.EncodeToString(walletChainData.EcdsaWalletID[:]),
		WalletPublicKey: "0x" + hex.EncodeToString(walletPublicKeyBytes),
		MembersIDsHash:  "0x" + hex.EncodeToString(walletChainData.MembersIDsHash[:]),
		HonestThreshold: groupParameters.HonestThreshold,
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
	certificate *covenantsigner.SignerApprovalCertificate,
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
	if strings.TrimSpace(expectedSignerSetHash) == "" {
		return fmt.Errorf("expected signer set hash must not be empty")
	}
	if strings.ToLower(expectedSignerSetHash) != strings.ToLower(certificate.SignerSetHash) {
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
