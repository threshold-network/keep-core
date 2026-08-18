package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/covenantsigner"
)

func validStructuredSignerApprovalVerificationRequest(
	t *testing.T,
	node *node,
	walletPublicKey *ecdsa.PublicKey,
	route covenantsigner.TemplateID,
) covenantsigner.RouteSubmitRequest {
	t.Helper()

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	depositorPrivateKey, _ := btcec.PrivKeyFromBytes(
		btcec.S256(),
		bytes.Repeat([]byte{0xaa}, 32),
	)

	request := covenantsigner.RouteSubmitRequest{
		RequestType: covenantsigner.RequestTypeReconstruct,
		Route:       route,
		ArtifactApprovals: &covenantsigner.ArtifactApprovalEnvelope{
			Payload: covenantsigner.ArtifactApprovalPayload{
				ApprovalVersion:           1,
				Route:                     route,
				ScriptTemplateID:          route,
				DestinationCommitmentHash: "0x" + strings.Repeat("11", 32),
				PlanCommitmentHash:        "0x" + strings.Repeat("22", 32),
			},
		},
	}

	switch route {
	case covenantsigner.TemplateSelfV1:
		templateJSON, err := json.Marshal(&covenantsigner.SelfV1Template{
			Template:           covenantsigner.TemplateSelfV1,
			DepositorPublicKey: "0x" + hex.EncodeToString(depositorPrivateKey.PubKey().SerializeCompressed()),
			SignerPublicKey:    "0x" + hex.EncodeToString((*btcec.PublicKey)(walletPublicKey).SerializeCompressed()),
			Delta2:             4320,
		})
		if err != nil {
			t.Fatal(err)
		}
		request.ScriptTemplate = templateJSON
		request.ArtifactApprovals.Approvals = []covenantsigner.ArtifactRoleApproval{
			{
				Role: covenantsigner.ArtifactApprovalRoleDepositor,
				Signature: testSignArtifactApproval(
					t,
					depositorPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
		}
	case covenantsigner.TemplateQcV1:
		custodianPrivateKey, _ := btcec.PrivKeyFromBytes(
			btcec.S256(),
			bytes.Repeat([]byte{0xbb}, 32),
		)
		templateJSON, err := json.Marshal(&covenantsigner.QcV1Template{
			Template:           covenantsigner.TemplateQcV1,
			DepositorPublicKey: "0x" + hex.EncodeToString(depositorPrivateKey.PubKey().SerializeCompressed()),
			CustodianPublicKey: "0x" + hex.EncodeToString(custodianPrivateKey.PubKey().SerializeCompressed()),
			SignerPublicKey:    "0x" + hex.EncodeToString((*btcec.PublicKey)(walletPublicKey).SerializeCompressed()),
			Beta:               144,
			Delta2:             4320,
		})
		if err != nil {
			t.Fatal(err)
		}
		request.ScriptTemplate = templateJSON
		request.ArtifactApprovals.Approvals = []covenantsigner.ArtifactRoleApproval{
			{
				Role: covenantsigner.ArtifactApprovalRoleDepositor,
				Signature: testSignArtifactApproval(
					t,
					depositorPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
			{
				Role: covenantsigner.ArtifactApprovalRoleCustodian,
				Signature: testSignArtifactApproval(
					t,
					custodianPrivateKey,
					request.ArtifactApprovals.Payload,
				),
			},
		}
	default:
		t.Fatalf("unsupported route %s", route)
	}

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		testArtifactApprovalDigest(t, request.ArtifactApprovals.Payload),
		startBlock,
		startBlock+100000,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.SignerApproval = certificate

	return request
}

func TestSigningExecutorCanIssueSignerApprovalCertificateForArbitraryDigest(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256(
		[]byte("psbt-covenant-signer-approval-certificate-spike"),
	)
	requestedEndBlock := startBlock + 100000

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
		requestedEndBlock,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletChainData, err := executor.chain.GetWallet(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		executor.wallet().publicKey,
		walletChainData,
		executor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySignerApprovalCertificate(certificate, expectedSignerSetHash); err != nil {
		t.Fatalf("expected certificate verification to succeed: %v", err)
	}

	expectedDigest := "0x" + hex.EncodeToString(approvalDigest[:])
	if certificate.ApprovalDigest != expectedDigest {
		t.Fatalf(
			"unexpected approval digest\nexpected: %s\nactual:   %s",
			expectedDigest,
			certificate.ApprovalDigest,
		)
	}
	if certificate.SignerSetHash != expectedSignerSetHash {
		t.Fatalf(
			"unexpected signer set hash\nexpected: %s\nactual:   %s",
			expectedSignerSetHash,
			certificate.SignerSetHash,
		)
	}
	if len(certificate.ActiveMembers) < executor.groupParameters.HonestThreshold {
		t.Fatalf(
			"expected at least honest threshold active members, got %v",
			certificate.ActiveMembers,
		)
	}
	if certificate.EndBlock == nil || *certificate.EndBlock != requestedEndBlock {
		t.Fatalf(
			"expected end block [%v] to equal the requested end block [%v]",
			certificate.EndBlock,
			requestedEndBlock,
		)
	}
}

// TestSigningExecutorIssueSignerApprovalCertificateAcceptsEndBlockAboveUint32Range
// proves an EndBlock above math.MaxUint32 round-trips correctly through
// issuance (signing) and verification. The certificate v2 digest binds
// EndBlock as a full 8-byte big-endian uint64 (see
// signerApprovalCertificateSigningDigest), so the full uint64 range must be
// usable end to end, not just the low 32 bits.
func TestSigningExecutorIssueSignerApprovalCertificateAcceptsEndBlockAboveUint32Range(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256(
		[]byte("psbt-covenant-signer-approval-certificate-uint64-end-block"),
	)
	requestedEndBlock := uint64(math.MaxUint32) + 100000
	if requestedEndBlock <= math.MaxUint32 {
		t.Fatalf("test vector must exceed math.MaxUint32, got %d", requestedEndBlock)
	}

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
		requestedEndBlock,
	)
	if err != nil {
		t.Fatal(err)
	}

	if certificate.EndBlock == nil || *certificate.EndBlock != requestedEndBlock {
		t.Fatalf(
			"expected end block [%v] to equal the requested end block [%v]",
			certificate.EndBlock,
			requestedEndBlock,
		)
	}

	walletChainData, err := executor.chain.GetWallet(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		executor.wallet().publicKey,
		walletChainData,
		executor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySignerApprovalCertificate(certificate, expectedSignerSetHash); err != nil {
		t.Fatalf("expected certificate verification to succeed: %v", err)
	}
}

func TestSigningExecutorIssueSignerApprovalCertificateFailsWhenWalletRegistryUnavailable(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	localChain, ok := executor.chain.(*localChain)
	if !ok {
		t.Fatal("expected local chain implementation")
	}
	localChain.setWalletRegistryErr(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
		errors.New("wallet registry unavailable"),
	)

	approvalDigest := sha256.Sum256([]byte("registry-unavailable"))
	_, err = executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
		startBlock+100000,
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"cannot issue signer approval certificate while wallet registry data is unavailable",
	) {
		t.Fatalf("expected wallet registry unavailable error, got %v", err)
	}
}

func TestSignerApprovalCertificateVerificationRejectsTamperedDigest(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256([]byte("psbt-covenant-signer-approval-certificate"))
	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
		startBlock+100000,
	)
	if err != nil {
		t.Fatal(err)
	}

	walletChainData, err := executor.chain.GetWallet(
		bitcoin.PublicKeyHash(executor.wallet().publicKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		executor.wallet().publicKey,
		walletChainData,
		executor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	tampered := *certificate
	tamperedDigest := sha256.Sum256([]byte("tampered"))
	tampered.ApprovalDigest = "0x" + hex.EncodeToString(tamperedDigest[:])
	if err := verifySignerApprovalCertificate(&tampered, expectedSignerSetHash); err == nil {
		t.Fatal("expected tampered approval digest to fail verification")
	}
}

func TestSignerApprovalCertificateSignerSetHashBindsOnChainWalletIdentityAndThreshold(t *testing.T) {
	_, _, walletPublicKey := setupCovenantSignerTestNode(t)

	baseWalletChainData := &WalletChainData{
		EcdsaWalletID:  sha256.Sum256([]byte("wallet-id-base")),
		MembersIDsHash: sha256.Sum256([]byte("members-hash-base")),
	}
	baseGroupParameters := &GroupParameters{
		GroupSize:       3,
		GroupQuorum:     2,
		HonestThreshold: 2,
	}

	baseHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		baseWalletChainData,
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	changedMembersHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		&WalletChainData{
			EcdsaWalletID:  baseWalletChainData.EcdsaWalletID,
			MembersIDsHash: sha256.Sum256([]byte("members-hash-changed")),
		},
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedMembersHash == baseHash {
		t.Fatal("expected signer set hash to change when members IDs hash changes")
	}

	changedWalletIDHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		&WalletChainData{
			EcdsaWalletID:  sha256.Sum256([]byte("wallet-id-changed")),
			MembersIDsHash: baseWalletChainData.MembersIDsHash,
		},
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedWalletIDHash == baseHash {
		t.Fatal("expected signer set hash to change when wallet ID changes")
	}

	thresholdChangedHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		baseWalletChainData,
		&GroupParameters{
			GroupSize:       3,
			GroupQuorum:     2,
			HonestThreshold: 3,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if thresholdChangedHash == baseHash {
		t.Fatal("expected signer set hash to change when honest threshold changes")
	}
}

func TestCovenantSignerEngineVerifySignerApprovalAcceptsValidCertificate(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	if err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request); err != nil {
		t.Fatalf("expected signer approval verification to succeed: %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsWalletPublicKeyMismatch(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval.WalletPublicKey = "0x04" + strings.Repeat("55", 64)

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.walletPublicKey must match request.scriptTemplate.signerPublicKey",
	) {
		t.Fatalf("expected wallet public key mismatch error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsMissingCertificate(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval = nil

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval is required for signer approval verification",
	) {
		t.Fatalf("expected missing signer approval error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsApprovalDigestMismatch(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)
	request.SignerApproval.ApprovalDigest = "0x" + strings.Repeat("11", 32)

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.approvalDigest must match request.artifactApprovals.payload",
	) {
		t.Fatalf("expected signer approval digest mismatch error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalRejectsMissingOnChainWallet(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	localChain, ok := node.chain.(*localChain)
	if !ok {
		t.Fatal("expected local chain implementation")
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	localChain.walletsMutex.Lock()
	delete(localChain.wallets, walletPublicKeyHash)
	localChain.walletsMutex.Unlock()

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.signerApproval.walletPublicKey must resolve to a registered on-chain wallet",
	) {
		t.Fatalf("expected missing wallet input error, got %v", err)
	}
}

func TestCovenantSignerEngineVerifySignerApprovalFailsWhenWalletRegistryUnavailable(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	localChain, ok := node.chain.(*localChain)
	if !ok {
		t.Fatal("expected local chain implementation")
	}
	localChain.setWalletRegistryErr(
		bitcoin.PublicKeyHash(walletPublicKey),
		errors.New("wallet registry unavailable"),
	)

	err := (&covenantSignerEngine{node: node}).VerifySignerApproval(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"cannot verify signer approval while wallet registry data is unavailable",
	) {
		t.Fatalf("expected wallet registry unavailable error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsEmptyExpectedSignerSetHash(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	err := verifySignerApprovalCertificate(request.SignerApproval, "")
	if err == nil || !strings.Contains(err.Error(), "expected signer set hash must not be empty") {
		t.Fatalf("expected empty signer set hash error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsSignerSetMismatch(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	err := verifySignerApprovalCertificate(
		request.SignerApproval,
		"0x"+strings.Repeat("ab", 32),
	)
	if err == nil || !strings.Contains(err.Error(), "signer set hash does not match the expected signer set") {
		t.Fatalf("expected signer set mismatch error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsMalformedDERSignature(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval
	certificate.Signature = "0xdeadbeef"

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "cannot parse threshold signature") {
		t.Fatalf("expected malformed DER signature error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsHighSSignature(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval

	// Parse the issued DER signature and construct the mathematically
	// equivalent high-S variant (S' = N - S). ECDSA permits both (R, S) and
	// (R, N - S) as valid signatures over the same digest; rejecting the
	// high-S form prevents signature malleability. DER encoding is done via
	// encoding/asn1 rather than btcec.Signature.Serialize(), which silently
	// re-normalizes S back to the low range and would defeat this check.
	signatureBytes, err := hex.DecodeString(strings.TrimPrefix(certificate.Signature, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	parsedSignature, err := btcec.ParseDERSignature(signatureBytes, btcec.S256())
	if err != nil {
		t.Fatal(err)
	}
	curveOrder := btcec.S256().N
	halfOrder := new(big.Int).Rsh(curveOrder, 1)
	highS := new(big.Int).Set(parsedSignature.S)
	if highS.Cmp(halfOrder) <= 0 {
		highS.Sub(curveOrder, highS)
	}
	if highS.Cmp(halfOrder) <= 0 {
		t.Fatalf("could not construct high-S candidate from S=%s", parsedSignature.S.Text(16))
	}
	highSignatureDER, err := asn1.Marshal(struct {
		R, S *big.Int
	}{R: parsedSignature.R, S: highS})
	if err != nil {
		t.Fatal(err)
	}
	certificate.Signature = "0x" + hex.EncodeToString(highSignatureDER)

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "threshold signature S value is not low-S normalized") {
		t.Fatalf("expected low-S normalization error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsMalformedWalletPublicKey(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval
	certificate.WalletPublicKey = "0x02" + strings.Repeat("11", 32)

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "wallet public key is not a valid uncompressed secp256k1 key") {
		t.Fatalf("expected malformed wallet public key error, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsV1CertificateVersion(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval
	certificate.CertificateVersion = 1

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "unsupported certificate version: 1") {
		t.Fatalf("expected v1 certificate version rejection, got %v", err)
	}
}

func TestVerifySignerApprovalCertificateRejectsMissingEndBlock(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval
	certificate.EndBlock = nil

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "certificate end block is required") {
		t.Fatalf("expected missing end block rejection, got %v", err)
	}
}

// TestVerifySignerApprovalCertificateRejectsTamperedEndBlock proves EndBlock
// is bound into the signed digest, not just carried alongside it: changing
// EndBlock on an otherwise-untouched, validly-issued certificate must
// invalidate the threshold signature. Without this, an attacker (or a
// compromised relay) could extend a certificate's validity window after
// issuance without the wallet's cooperation.
func TestVerifySignerApprovalCertificateRejectsTamperedEndBlock(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	request := validStructuredSignerApprovalVerificationRequest(
		t,
		node,
		walletPublicKey,
		covenantsigner.TemplateSelfV1,
	)

	certificate := *request.SignerApproval
	tamperedEndBlock := *certificate.EndBlock + 1_000_000
	certificate.EndBlock = &tamperedEndBlock

	walletExecutor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	walletChainData, err := walletExecutor.chain.GetWallet(bitcoin.PublicKeyHash(walletPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	expectedSignerSetHash, err := computeSignerApprovalCertificateSignerSetHash(
		walletPublicKey,
		walletChainData,
		walletExecutor.groupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifySignerApprovalCertificate(&certificate, expectedSignerSetHash)
	if err == nil || !strings.Contains(err.Error(), "threshold signature does not verify against wallet public key") {
		t.Fatalf("expected tampered end block to invalidate the signature, got %v", err)
	}
}

// TestSigningExecutorIssueSignerApprovalCertificateRejectsCompletionAfterRequestedExpiry
// verifies that issuance is rejected outright -- not merely issued with a
// signing-completion-block EndBlock -- when synchronous threshold signing
// does not finish until after the requested expiry. A short real sleep
// ensures the local block counter has ticked past block zero, so the chosen
// requestedEndBlock (one below the observed start block) is guaranteed to be
// exceeded by the actual signing completion block regardless of how fast
// signing itself completes.
func TestSigningExecutorIssueSignerApprovalCertificateRejectsCompletionAfterRequestedExpiry(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	time.Sleep(600 * time.Millisecond)

	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}
	if startBlock == 0 {
		t.Fatal("expected the local block counter to have advanced past zero")
	}
	requestedEndBlock := startBlock - 1

	approvalDigest := sha256.Sum256([]byte("already-expired-by-completion"))
	_, err = executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
		requestedEndBlock,
	)
	if err == nil || !strings.Contains(err.Error(), "after the requested end block") {
		t.Fatalf("expected completion-after-expiry rejection, got %v", err)
	}
}

// TestSignerApprovalCertificateSigningDigestMatchesCrossLanguageVector pins
// the exact byte layout of the certificate v2 signing digest so independent
// (non-Go) certificate producers can verify their implementation against the
// same inputs and output:
//
//	signingDigest = SHA256(
//	  ASCII("covenant-signer-approval-certificate-v2:") ||
//	  rawApprovalDigest[32] ||
//	  uint64_big_endian(endBlock)
//	)
func TestSignerApprovalCertificateSigningDigestMatchesCrossLanguageVector(t *testing.T) {
	approvalDigest, err := hex.DecodeString(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	const endBlock = uint64(123456)

	// Independently constructed preimage and digest -- deliberately not
	// calling the production helper -- so this test actually catches a
	// broken implementation rather than just echoing it back.
	preimage := append(
		[]byte("covenant-signer-approval-certificate-v2:"),
		approvalDigest...,
	)
	var endBlockBytes [8]byte
	binary.BigEndian.PutUint64(endBlockBytes[:], endBlock)
	preimage = append(preimage, endBlockBytes[:]...)
	expectedDigest := sha256.Sum256(preimage)
	expectedDigestHex := "0x" + hex.EncodeToString(expectedDigest[:])

	// Pinned exact value: SHA256(
	//   "covenant-signer-approval-certificate-v2:" ||
	//   0x1111111111111111111111111111111111111111111111111111111111111111 ||
	//   0x000000000001e240
	// )
	const expectedPinnedHex = "0x636bf1d70b67dd7f54e1d4e090fd258928cce229b66a9a8cae3a5e75e288e19d"
	if expectedDigestHex != expectedPinnedHex {
		t.Fatalf(
			"test vector itself is inconsistent\nindependent: %s\npinned:      %s",
			expectedDigestHex,
			expectedPinnedHex,
		)
	}

	actualDigest, err := signerApprovalCertificateSigningDigest(approvalDigest, endBlock)
	if err != nil {
		t.Fatal(err)
	}
	actualDigestHex := "0x" + hex.EncodeToString(actualDigest)

	if actualDigestHex != expectedDigestHex {
		t.Fatalf(
			"unexpected v2 signing digest\nexpected: %s\nactual:   %s",
			expectedDigestHex,
			actualDigestHex,
		)
	}
}

// TestSignerApprovalCertificateSigningDigestMatchesCrossLanguageVectorAboveUint32Range
// pins a second cross-language vector whose EndBlock exceeds math.MaxUint32,
// so the digest's high-order 4 bytes are nonzero. The vector above alone
// cannot catch an implementation that silently truncates EndBlock to 32 bits
// before hashing it, since 123456 fits entirely in the low-order bytes; this
// one specifically exercises the full 8-byte big-endian encoding.
func TestSignerApprovalCertificateSigningDigestMatchesCrossLanguageVectorAboveUint32Range(t *testing.T) {
	approvalDigest, err := hex.DecodeString(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	// One past math.MaxUint32, plus the same 123456 low-order value used by
	// the vector above.
	const endBlock = uint64(math.MaxUint32) + 1 + 123456
	if endBlock <= math.MaxUint32 {
		t.Fatalf("test vector must exceed math.MaxUint32, got %d", endBlock)
	}

	// Independently constructed preimage and digest -- deliberately not
	// calling the production helper -- so this test actually catches a
	// broken implementation rather than just echoing it back.
	preimage := append(
		[]byte("covenant-signer-approval-certificate-v2:"),
		approvalDigest...,
	)
	var endBlockBytes [8]byte
	binary.BigEndian.PutUint64(endBlockBytes[:], endBlock)
	if endBlockBytes != ([8]byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0xe2, 0x40}) {
		t.Fatalf("unexpected big-endian encoding of endBlock: %x", endBlockBytes)
	}
	preimage = append(preimage, endBlockBytes[:]...)
	expectedDigest := sha256.Sum256(preimage)
	expectedDigestHex := "0x" + hex.EncodeToString(expectedDigest[:])

	// Pinned exact value, independently computed (Python hashlib, not this
	// Go codebase): SHA256(
	//   "covenant-signer-approval-certificate-v2:" ||
	//   0x1111111111111111111111111111111111111111111111111111111111111111 ||
	//   0x000000010001e240
	// )
	const expectedPinnedHex = "0x67e035629f14cad309d349046c6c6ec0ec2c18a79b0c948796a6668b15d4dd60"
	if expectedDigestHex != expectedPinnedHex {
		t.Fatalf(
			"test vector itself is inconsistent\nindependent: %s\npinned:      %s",
			expectedDigestHex,
			expectedPinnedHex,
		)
	}

	actualDigest, err := signerApprovalCertificateSigningDigest(approvalDigest, endBlock)
	if err != nil {
		t.Fatal(err)
	}
	actualDigestHex := "0x" + hex.EncodeToString(actualDigest)

	if actualDigestHex != expectedDigestHex {
		t.Fatalf(
			"unexpected v2 signing digest\nexpected: %s\nactual:   %s",
			expectedDigestHex,
			actualDigestHex,
		)
	}
}

func TestCovenantSignerEngineIssueSignerApprovalCertificateHappyPath(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256([]byte("engine-issue-happy-path"))
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

	certificate, err := cse.IssueSignerApprovalCertificate(
		context.Background(),
		walletPublicKeyHash,
		approvalDigest[:],
		startBlock+100000,
	)
	if err != nil {
		t.Fatal(err)
	}
	if certificate == nil {
		t.Fatal("expected a certificate")
	}
	if !strings.EqualFold(
		certificate.ApprovalDigest,
		"0x"+hex.EncodeToString(approvalDigest[:]),
	) {
		t.Fatalf("unexpected approval digest: %s", certificate.ApprovalDigest)
	}
	if certificate.EndBlock == nil || *certificate.EndBlock != startBlock+100000 {
		t.Fatalf("unexpected end block: %v", certificate.EndBlock)
	}
}

func TestCovenantSignerEngineIssueSignerApprovalCertificateRejectsUnknownWallet(t *testing.T) {
	node, _, _ := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	approvalDigest := sha256.Sum256([]byte("unknown-wallet"))
	var unknownPKH [20]byte
	copy(unknownPKH[:], bytes.Repeat([]byte{0x11}, 20))

	_, err := cse.IssueSignerApprovalCertificate(
		context.Background(),
		unknownPKH,
		approvalDigest[:],
		math.MaxUint64,
	)
	if err == nil || !strings.Contains(err.Error(), "controlled by this node") {
		t.Fatalf("expected uncontrolled-wallet rejection, got %v", err)
	}
}

func TestCovenantSignerEngineIssueSignerApprovalCertificateRejectsMissingOnChainWallet(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	localChain, ok := node.chain.(*localChain)
	if !ok {
		t.Fatal("expected local chain implementation")
	}
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	localChain.walletsMutex.Lock()
	delete(localChain.wallets, walletPublicKeyHash)
	localChain.walletsMutex.Unlock()

	approvalDigest := sha256.Sum256([]byte("missing-on-chain-wallet"))
	_, err := cse.IssueSignerApprovalCertificate(
		context.Background(),
		walletPublicKeyHash,
		approvalDigest[:],
		math.MaxUint64,
	)
	if err == nil || !strings.Contains(err.Error(), "must resolve to a registered on-chain wallet") {
		t.Fatalf("expected missing on-chain wallet rejection, got %v", err)
	}
}
func TestCovenantSignerEngineIssueSignerApprovalCertificateRejectsBadDigestLength(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	_, err := cse.IssueSignerApprovalCertificate(
		context.Background(),
		bitcoin.PublicKeyHash(walletPublicKey),
		[]byte("too-short"),
		math.MaxUint64,
	)
	if err == nil || !strings.Contains(err.Error(), "exactly 32 bytes") {
		t.Fatalf("expected digest-length rejection, got %v", err)
	}
}

func TestCovenantSignerEngineIssueSignerApprovalCertificateRejectsEndBlockNotAfterCurrent(
	t *testing.T,
) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}
	startBlock, err := executor.getCurrentBlockFn()
	if err != nil {
		t.Fatal(err)
	}

	approvalDigest := sha256.Sum256([]byte("end-block-too-soon"))
	_, err = cse.IssueSignerApprovalCertificate(
		context.Background(),
		bitcoin.PublicKeyHash(walletPublicKey),
		approvalDigest[:],
		startBlock,
	)
	if err == nil || !strings.Contains(err.Error(), "must be greater than the current host-chain block") {
		t.Fatalf("expected endBlock rejection, got %v", err)
	}
}

func TestCovenantSignerEngineIssueSignerApprovalCertificateRejectsClosedWallet(t *testing.T) {
	node, _, walletPublicKey := setupCovenantSignerTestNode(t)
	cse := &covenantSignerEngine{node: node}

	localChain, ok := node.chain.(*localChain)
	if !ok {
		t.Fatal("expected local chain implementation")
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	existing, err := localChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	closed := *existing
	closed.State = StateClosed
	localChain.setWallet(walletPublicKeyHash, &closed)

	approvalDigest := sha256.Sum256([]byte("closed-wallet"))
	_, err = cse.IssueSignerApprovalCertificate(
		context.Background(),
		walletPublicKeyHash,
		approvalDigest[:],
		math.MaxUint64,
	)
	if err == nil || !strings.Contains(err.Error(), "not eligible for covenant signing") {
		t.Fatalf("expected closed-wallet rejection, got %v", err)
	}
}
