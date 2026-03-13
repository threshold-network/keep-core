package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

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
		executor.wallet(),
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
	if certificate.EndBlock < startBlock {
		t.Fatalf(
			"expected end block [%v] to be >= start block [%v]",
			certificate.EndBlock,
			startBlock,
		)
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
		executor.wallet(),
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

	baseWallet := wallet{
		publicKey: walletPublicKey,
	}
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
		baseWallet,
		baseWalletChainData,
		baseGroupParameters,
	)
	if err != nil {
		t.Fatal(err)
	}

	changedMembersHash, err := computeSignerApprovalCertificateSignerSetHash(
		baseWallet,
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
		baseWallet,
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
		baseWallet,
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
