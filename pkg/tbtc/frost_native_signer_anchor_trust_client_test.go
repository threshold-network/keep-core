package tbtc

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"testing"
	"time"
)

func TestValidateFrostNativeSignerAnchorTrustTargetAcknowledgementAllowsOnlyCertifiedEpochParent(
	t *testing.T,
) {
	onlinePrivate := ed25519.NewKeyFromSeed(
		bytesRepeatForFrostNativeSignerTrustTest(0x41, ed25519.SeedSize),
	)
	onlinePublic := onlinePrivate.Public().(ed25519.PublicKey)
	onlineSPKI, err := x509.MarshalPKIXPublicKey(onlinePublic)
	if err != nil {
		t.Fatal(err)
	}
	storeFingerprint := [32]byte{0x51}
	identity := FrostNativeSignerAnchorIdentity{
		ProtocolID:                      [32]byte{0x52},
		ActivationManifestHash:          [32]byte{0x53},
		ActivationManifestSequence:      2,
		TrustDomainID:                   "trust-client-test",
		OnlineKeyHash:                   sha256.Sum256(onlineSPKI),
		OperatorFingerprint:             [32]byte{0x54},
		HistoryStoreID:                  "trust-client-history",
		HistoryStoreFingerprint:         [32]byte{0x55},
		HistoryClusterFingerprint:       [32]byte{0x56},
		OfflineAuthorityHash:            [32]byte{0x57},
		ClientSPKIHash:                  [32]byte{0x58},
		SignerStoreFingerprint:          storeFingerprint,
		TransportBinding:                [32]byte{0x59},
		WitnessMaximumRecords:           4096,
		WitnessRotationThresholdRecords: 1024,
	}
	identity.StreamID = ComputeFrostNativeSignerAnchorStreamID(identity)
	checkpoint := testFrostNativeSignerAnchorCheckpoint(
		storeFingerprint,
		7,
		[32]byte{0x61},
		0x62,
	)
	operationID := [32]byte{0x63}
	transitionDigest := [32]byte{0x64}
	previousEventRoot := [32]byte{0x65}
	now := time.Unix(1_900_000_000, 0)
	acknowledgement, raw := testFrostNativeSignerAnchorAcknowledgement(
		t,
		identity,
		checkpoint,
		operationID,
		transitionDigest,
		[32]byte{0x66},
		[32]byte{0x67},
		"applied",
		2,
		1,
		previousEventRoot,
		now,
		onlinePrivate,
	)
	rawOnline := [32]byte{}
	copy(rawOnline[:], onlinePublic)
	certificate := &FrostNativeSignerAnchorTrustCertificate{
		Kind:                   FrostNativeSignerAnchorTrustCertificateRotation,
		ProtocolID:             identity.ProtocolID,
		StreamID:               identity.StreamID,
		SignerStoreFingerprint: storeFingerprint,
		OperationID:            operationID,
		TransitionDigest:       transitionDigest,
		To: FrostNativeSignerAnchorTrustEndpoint{
			BindingHash:                 ComputeFrostNativeSignerAnchorBindingHash(identity),
			ResponsePublicKey:           rawOnline,
			ResponsePublicKeySPKISHA256: identity.OnlineKeyHash,
			Reference: FrostNativeSignerAnchorTrustReference{
				ServiceEpoch:          2,
				Revision:              1,
				PreviousEventRoot:     previousEventRoot,
				EventRoot:             acknowledgement.EventRoot,
				AcknowledgementDigest: acknowledgement.AcknowledgementDigest,
				Checkpoint:            checkpoint,
			},
		},
	}
	if err := ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
		certificate,
		raw,
	); err != nil {
		t.Fatalf("certified cross-epoch acknowledgement was rejected: %v", err)
	}

	genericClient := &FrostNativeSignerAnchorClient{
		identity:       identity,
		bindingHash:    certificate.To.BindingHash,
		onlineKey:      append(ed25519.PublicKey{}, onlinePublic...),
		maximumAckLife: frostNativeSignerAnchorMaximumAcknowledgementLifetime,
		clockSkew:      frostNativeSignerAnchorMaximumClockSkew,
		now:            func() time.Time { return now },
	}
	if _, err := genericClient.verifyAcknowledgement(
		raw,
		nil,
		nil,
		&checkpoint,
		&operationID,
		false,
		"applied",
	); err == nil {
		t.Fatal("generic same-epoch verifier accepted a non-zero revision-one parent")
	}

	tampered := *certificate
	tampered.To.Reference.PreviousEventRoot = [32]byte{0xff}
	if err := ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
		&tampered,
		raw,
	); err == nil {
		t.Fatal("trust verifier accepted a predecessor root not signed by the certificate")
	}
}

func bytesRepeatForFrostNativeSignerTrustTest(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}
