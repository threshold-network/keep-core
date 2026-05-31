package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

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

	certificate, err := executor.issueSignerApprovalCertificate(
		context.Background(),
		approvalDigest[:],
		startBlock,
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
	if certificate.EndBlock == nil || *certificate.EndBlock < startBlock {
		t.Fatalf(
			"expected end block [%v] to be >= start block [%v]",
			certificate.EndBlock,
			startBlock,
		)
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
