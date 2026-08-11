//go:build frost_native

package tbtc

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestRunFrostShareRepairTransportPreflightDisabled(t *testing.T) {
	requested, err := runFrostShareRepairTransportPreflight(
		"",
		"",
		FrostPreSignActivationRuntimeManifest{},
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("disabled preflight returned an error: [%v]", err)
	}
	if requested {
		t.Fatal("disabled preflight was reported as requested")
	}
}

func TestRunFrostShareRepairTransportPreflightRequiresPairedPaths(t *testing.T) {
	testCases := []struct {
		name              string
		authorizationPath string
		outputPath        string
	}{
		{
			name:              "input only",
			authorizationPath: "/tmp/share-repair-authorization.json",
		},
		{
			name:       "output only",
			outputPath: "/tmp/share-repair-transport-preflight.json",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requested, err := runFrostShareRepairTransportPreflight(
				testCase.authorizationPath,
				testCase.outputPath,
				FrostPreSignActivationRuntimeManifest{},
				nil,
				"",
			)
			if !requested {
				t.Fatal("partially configured preflight was not reported as requested")
			}
			if err == nil || !strings.Contains(err.Error(), "must be configured together") {
				t.Fatalf("unexpected paired-path validation result: [%v]", err)
			}
		})
	}
}

func TestRunFrostShareRepairTransportPreflightRejectsMalformedArtifact(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	authorizationPath := filepath.Join(directory, "authorization.json")
	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		authorizationPath,
		[]byte("{"),
	); err != nil {
		t.Fatalf("write malformed authorization: [%v]", err)
	}

	requested, err := runFrostShareRepairTransportPreflight(
		authorizationPath,
		filepath.Join(directory, "preflight.json"),
		FrostPreSignActivationRuntimeManifest{
			ActivationAuthorityPublicKey: [32]byte{1},
		},
		&node{walletRegistry: &walletRegistry{}},
		"operator-1",
	)
	if !requested {
		t.Fatal("malformed preflight was not reported as requested")
	}
	if err == nil || !strings.Contains(err.Error(), "cannot decode share-repair authorization") {
		t.Fatalf("unexpected malformed-artifact validation result: [%v]", err)
	}
}

func TestRunFrostShareRepairTransportPreflightRejectsInvalidAuthorizationSignature(
	t *testing.T,
) {
	fixture := newShareRepairTransportPreflightTestFixture(t)
	fixture.authorization.SignatureHex = "0x" + strings.Repeat("00", ed25519.SignatureSize)
	authorizationPath, outputPath := fixture.writeAuthorization(t)
	registerShareRepairTransportPreflightTestEngine(t, fixture.engine)

	requested, err := runFrostShareRepairTransportPreflight(
		authorizationPath,
		outputPath,
		fixture.manifest,
		fixture.node,
		fixture.operator,
	)
	if !requested {
		t.Fatal("invalidly signed preflight was not reported as requested")
	}
	if err == nil || !strings.Contains(err.Error(), "authorization signature is invalid") {
		t.Fatalf("unexpected signature validation result: [%v]", err)
	}
	if fixture.engine.beginCalls != 0 || fixture.engine.finishCalls != 0 {
		t.Fatalf(
			"native session started before signature validation: begin=[%d] finish=[%d]",
			fixture.engine.beginCalls,
			fixture.engine.finishCalls,
		)
	}
}

func TestRunFrostShareRepairTransportPreflightPublishesOwnerOnlyNoReplaceArtifact(
	t *testing.T,
) {
	fixture := newShareRepairTransportPreflightTestFixture(t)
	authorizationPath, outputPath := fixture.writeAuthorization(t)
	registerShareRepairTransportPreflightTestEngine(t, fixture.engine)

	requested, err := runFrostShareRepairTransportPreflight(
		authorizationPath,
		outputPath,
		fixture.manifest,
		fixture.node,
		fixture.operator,
	)
	if err != nil {
		t.Fatalf("run transport preflight: [%v]", err)
	}
	if !requested {
		t.Fatal("configured preflight was not reported as requested")
	}
	if fixture.engine.beginCalls != 1 || fixture.engine.finishCalls != 1 {
		t.Fatalf(
			"unexpected native session lifecycle: begin=[%d] finish=[%d]",
			fixture.engine.beginCalls,
			fixture.engine.finishCalls,
		)
	}

	info, err := os.Lstat(outputPath)
	if err != nil {
		t.Fatalf("stat preflight output: [%v]", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 ||
		info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("preflight output is not an owner-only regular file: [%v]", info.Mode())
	}

	artifactBytes, err := ReadFrostNativeSignerAnchorProvisioningArtifact(
		outputPath,
		frostShareRepairTransportPreflightMaximumBytes,
	)
	if err != nil {
		t.Fatalf("read preflight output: [%v]", err)
	}
	artifact := &frostsigning.ShareRepairTransportPreflight{}
	decoder := json.NewDecoder(bytes.NewReader(artifactBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(artifact); err != nil {
		t.Fatalf("decode preflight output: [%v]", err)
	}
	digest, err := frostsigning.ComputeShareRepairAuthorizationDigest(
		fixture.authorization,
	)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Schema != frostsigning.ShareRepairTransportPreflightSchema ||
		artifact.AuthorizationDigest != "0x"+hex.EncodeToString(digest[:]) ||
		len(artifact.ParticipantPublicKeys) != 1 {
		t.Fatalf("unexpected preflight artifact: [%+v]", artifact)
	}
	entry := artifact.ParticipantPublicKeys[0]
	if entry.ParticipantIdentifier != 1 ||
		entry.StoreFingerprint != fixture.engine.storeFingerprint ||
		entry.PublicKeyHex != hex.EncodeToString(fixture.engine.publicKey) {
		t.Fatalf("unexpected preflight transport entry: [%+v]", entry)
	}

	original := append([]byte(nil), artifactBytes...)
	requested, err = runFrostShareRepairTransportPreflight(
		authorizationPath,
		outputPath,
		fixture.manifest,
		fixture.node,
		fixture.operator,
	)
	if !requested {
		t.Fatal("repeated configured preflight was not reported as requested")
	}
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing output was replaced or returned an unexpected error: [%v]", err)
	}
	after, readErr := ReadFrostNativeSignerAnchorProvisioningArtifact(
		outputPath,
		frostShareRepairTransportPreflightMaximumBytes,
	)
	if readErr != nil {
		t.Fatalf("read preflight output after rejected replacement: [%v]", readErr)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("rejected replacement changed the existing preflight artifact")
	}
}

type shareRepairTransportPreflightTestFixture struct {
	authorization *frostsigning.ShareRepairAuthorization
	manifest      FrostPreSignActivationRuntimeManifest
	node          *node
	operator      chain.Address
	engine        *shareRepairTransportPreflightTestEngine
	directory     string
}

func newShareRepairTransportPreflightTestFixture(
	t *testing.T,
) *shareRepairTransportPreflightTestFixture {
	t.Helper()
	_, keyGroup := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x09}, 32))
	keyGroupHex := hex.EncodeToString(keyGroup.SerializeCompressed())
	walletIDBytes := keyGroup.X().FillBytes(make([]byte, 32))
	var walletID [32]byte
	copy(walletID[:], walletIDBytes)

	authority := ed25519.NewKeyFromSeed(
		bytes.Repeat([]byte{0x42}, ed25519.SeedSize),
	)
	now := uint64(time.Now().Unix())
	authorization := &frostsigning.ShareRepairAuthorization{
		Schema:                     frostsigning.ShareRepairAuthorizationSchema,
		SessionID:                  "preflight-test-repair",
		WalletID:                   "0x" + hex.EncodeToString(walletID[:]),
		KeyGroup:                   keyGroupHex,
		PublicKeyPackageCommitment: preflightTestHex32(0x31),
		TargetIdentifier:           3,
		HelperIdentifiers:          []uint16{1, 2},
		Threshold:                  2,
		ParticipantCount:           3,
		OldStoreFingerprint:        preflightTestHex32(0x51),
		NewStoreFingerprint:        preflightTestHex32(0x52),
		RecoveryEpoch:              1,
		IssuedAtUnix:               now - 60,
		NotBeforeUnix:              now + 600,
		ExpiresAtUnix:              now + 3600,
		Nonce:                      preflightTestHex32(0x61),
	}
	digest, err := frostsigning.ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		t.Fatalf("compute share-repair authorization digest: [%v]", err)
	}
	authorization.SignatureHex = "0x" + hex.EncodeToString(
		ed25519.Sign(authority, digest[:]),
	)

	materialPayload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:         keyGroupHex,
		TaprootOutputKey: hex.EncodeToString(walletID[:]),
		KeyGroupSource:   frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	if err != nil {
		t.Fatal(err)
	}
	operator := chain.Address("operator-1")
	wallet := wallet{
		publicKey: keyGroup.ToECDSA(),
		signingGroupOperators: []chain.Address{
			operator,
			"operator-2",
			"operator-3",
		},
	}
	localSigner := &signer{
		wallet:                  wallet,
		signingGroupMemberIndex: group.MemberIndex(1),
		signerMaterial: &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: materialPayload,
		},
	}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			getWalletStorageKey(wallet.publicKey): {
				walletID: walletID,
				signers:  []*signer{localSigner},
			},
		},
	}

	_, transportPublicKey := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x23}, 32))
	engine := &shareRepairTransportPreflightTestEngine{
		storeFingerprint: preflightTestHex32(0x71),
		publicKey:        transportPublicKey.SerializeCompressed(),
	}
	manifest := FrostPreSignActivationRuntimeManifest{}
	copy(manifest.ActivationAuthorityPublicKey[:], authority.Public().(ed25519.PublicKey))

	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}

	return &shareRepairTransportPreflightTestFixture{
		authorization: authorization,
		manifest:      manifest,
		node:          &node{walletRegistry: registry},
		operator:      operator,
		engine:        engine,
		directory:     directory,
	}
}

func (fixture *shareRepairTransportPreflightTestFixture) writeAuthorization(
	t *testing.T,
) (string, string) {
	t.Helper()
	payload, err := json.Marshal(fixture.authorization)
	if err != nil {
		t.Fatal(err)
	}
	authorizationPath := filepath.Join(fixture.directory, "authorization.json")
	if err := WriteFrostNativeSignerAnchorProvisioningArtifact(
		authorizationPath,
		payload,
	); err != nil {
		t.Fatalf("write authorization: [%v]", err)
	}
	return authorizationPath, filepath.Join(fixture.directory, "preflight.json")
}

func preflightTestHex32(value byte) string {
	return "0x" + hex.EncodeToString(bytes.Repeat([]byte{value}, 32))
}

type shareRepairTransportPreflightTestEngine struct {
	storeFingerprint string
	publicKey        []byte
	beginCalls       int
	finishCalls      int
}

func (*shareRepairTransportPreflightTestEngine) BuildTaprootTx(
	string,
	[]frostsigning.NativeTBTCSignerTxInput,
	[]frostsigning.NativeTBTCSignerTxOutput,
	*string,
) (*frostsigning.NativeTBTCSignerTxResult, error) {
	return nil, errors.New("not implemented")
}

func (*shareRepairTransportPreflightTestEngine) VerifySignatureShare(
	string,
	[]byte,
	[]byte,
	uint16,
	*[32]byte,
) (frostsigning.NativeShareVerificationVerdict, error) {
	return frostsigning.NativeShareVerdictIndeterminate, errors.New("not implemented")
}

func (engine *shareRepairTransportPreflightTestEngine) BeginShareRepairSession(
	authorization *frostsigning.ShareRepairAuthorization,
	participantIdentifier uint16,
) (*frostsigning.NativeShareRepairSession, error) {
	engine.beginCalls++
	digest, err := frostsigning.ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return nil, err
	}
	return &frostsigning.NativeShareRepairSession{
		ContextDigest:         "0x" + hex.EncodeToString(digest[:]),
		ParticipantIdentifier: participantIdentifier,
		StoreFingerprint:      engine.storeFingerprint,
		TransportPublicKey:    append([]byte(nil), engine.publicKey...),
	}, nil
}

func (engine *shareRepairTransportPreflightTestEngine) FinishShareRepairSession(
	*frostsigning.ShareRepairAuthorization,
	uint16,
) error {
	engine.finishCalls++
	return nil
}

func (*shareRepairTransportPreflightTestEngine) ShareRepairPart1(
	*frostsigning.ShareRepairAuthorization,
	uint16,
	*frostsigning.ShareRepairTransportRoster,
) (*frostsigning.NativeShareRepairPart1Result, error) {
	return nil, errors.New("not implemented")
}

func (*shareRepairTransportPreflightTestEngine) ShareRepairPart2(
	*frostsigning.ShareRepairAuthorization,
	uint16,
	[]*frostsigning.NativeShareRepairEncryptedDelta,
	*frostsigning.ShareRepairTransportRoster,
) (*frostsigning.NativeShareRepairPart2Result, error) {
	return nil, errors.New("not implemented")
}

func (*shareRepairTransportPreflightTestEngine) InstallRepairedShare(
	*frostsigning.ShareRepairAuthorization,
	*frostsigning.NativeFROSTPublicKeyPackage,
	[]*frostsigning.NativeShareRepairEncryptedSigma,
	*frostsigning.ShareRepairTransportRoster,
) (*frostsigning.NativeShareRepairInstallResult, error) {
	return nil, errors.New("not implemented")
}

func registerShareRepairTransportPreflightTestEngine(
	t *testing.T,
	engine frostsigning.NativeTBTCSignerEngine,
) {
	t.Helper()
	previous := frostsigning.CurrentNativeTBTCSignerEngine()
	frostsigning.UnregisterNativeTBTCSignerEngine()
	if err := frostsigning.RegisterNativeTBTCSignerEngine(engine); err != nil {
		t.Fatalf("register native signer engine: [%v]", err)
	}
	t.Cleanup(func() {
		frostsigning.UnregisterNativeTBTCSignerEngine()
		if previous != nil {
			if err := frostsigning.RegisterNativeTBTCSignerEngine(previous); err != nil {
				t.Errorf("restore native signer engine: [%v]", err)
			}
		}
	})
}
