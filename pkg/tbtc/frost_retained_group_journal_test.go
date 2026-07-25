package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type journalHistorySource struct {
	identity    FrostRetainedGroupHistoryIdentity
	checkpoint  FrostPreSignFinality
	head        FrostPreSignFinality
	descriptor  [32]byte
	mutations   []FrostRetainedGroupMutation
	complete    bool
	emptyAtFrom bool
	points      map[uint64][32]byte
	operators   map[chain.Address]chain.OperatorID
	verifyErr   error
}

func (jhs *journalHistorySource) Identity(
	context.Context,
) (FrostRetainedGroupHistoryIdentity, error) {
	return jhs.identity, nil
}

func (jhs *journalHistorySource) FinalizedHead(
	context.Context,
) (FrostPreSignFinality, error) {
	return jhs.head, nil
}

func (jhs *journalHistorySource) VerifyPoint(
	_ context.Context,
	point FrostPreSignFinality,
) error {
	if jhs.verifyErr != nil {
		return jhs.verifyErr
	}
	if expected, ok := jhs.points[point.BlockNumber]; !ok || expected != point.BlockHash {
		return fmt.Errorf("point is not canonical")
	}
	return nil
}

func (jhs *journalHistorySource) ReadCompleteHistory(
	_ context.Context,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
) (*FrostRetainedGroupHistory, error) {
	mutations := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range jhs.mutations {
		if mutation.Point.BlockNumber <= to.BlockNumber {
			mutations = append(mutations, mutation)
		}
	}
	return &FrostRetainedGroupHistory{
		From:              from,
		To:                to,
		Mutations:         cloneFrostRetainedGroupMutations(mutations),
		Complete:          jhs.complete,
		EmptyAtFrom:       jhs.emptyAtFrom,
		DescriptorSetHash: jhs.descriptor,
	}, nil
}

func (jhs *journalHistorySource) ResolveOperatorID(
	_ context.Context,
	address chain.Address,
	_ FrostPreSignFinality,
) (chain.OperatorID, error) {
	operatorID := jhs.operators[address]
	if operatorID == 0 {
		return 0, fmt.Errorf("unknown operator")
	}
	return operatorID, nil
}

type journalTestFixture struct {
	manifest           FrostRetainedGroupCanonicalJournalManifest
	quarantine         FrostRetainedGroupQuarantineJournalManifest
	runtime            FrostPreSignActivationRuntimeManifest
	manifestHash       [32]byte
	bindingHash        [32]byte
	liftPrivateKeys    []ed25519.PrivateKey
	liftPublicKeySPKIs []string
	checkpoint         FrostPreSignFinality
	target             FrostPreSignFinality
	later              FrostPreSignFinality
	walletID           [32]byte
	walletPKH          [20]byte
	operatorIDs        []uint32
	operatorAddrs      []chain.Address
	localOperator      chain.Address
	registry           *walletRegistry
	source             *journalHistorySource
	admission          FrostRetainedGroupMutation
}

func newJournalTestFixture(t *testing.T) *journalTestFixture {
	t.Helper()
	checkpoint := FrostPreSignFinality{BlockNumber: 1, BlockHash: [32]byte{0x01}}
	target := FrostPreSignFinality{BlockNumber: 10, BlockHash: [32]byte{0x0a}}
	later := FrostPreSignFinality{BlockNumber: 20, BlockHash: [32]byte{0x14}}
	operatorIDs := make([]uint32, 51)
	operatorAddrs := make([]chain.Address, 51)
	operators := make(map[chain.Address]chain.OperatorID, 51)
	for index := range operatorIDs {
		operatorIDs[index] = uint32(index + 1)
		operatorAddrs[index] = chain.Address(fmt.Sprintf("operator-%02d", index+1))
		operators[operatorAddrs[index]] = chain.OperatorID(index + 1)
	}
	walletID := [32]byte{0x91}
	walletPKH := [20]byte{0x92}
	localOperator := operatorAddrs[6]
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256()}
	signerMaterialPayload, err := json.Marshal(
		frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup:       fmt.Sprintf("%x", walletID),
			KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	localSigner := &signer{
		wallet: wallet{
			publicKey:             publicKey,
			signingGroupOperators: append([]chain.Address{}, operatorAddrs...),
		},
		signingGroupMemberIndex: group.MemberIndex(7),
		signerMaterial: &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: signerMaterialPayload,
		},
	}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			"wallet": {
				walletPublicKeyHash: walletPKH,
				walletID:            walletID,
				signers:             []*signer{localSigner},
			},
		},
	}
	manifest := FrostRetainedGroupCanonicalJournalManifest{
		StoreID:                   "journal-store-uuid",
		StoreFingerprint:          [32]byte{0x31},
		ClusterFingerprint:        [32]byte{0x32},
		Checkpoint:                checkpoint,
		DescriptorSetHash:         [32]byte{0x33},
		SourceTrustDomainID:       "independent-journal-source",
		SourceEndpointFingerprint: [32]byte{0x34},
		SourceOperatorFingerprint: [32]byte{0x35},
		MinimumGeneration:         1,
	}
	checkpointAuthorities := make([]FrostRetainedGroupAuthority, 3)
	for index := range checkpointAuthorities {
		authority, _, _ := journalTestAuthority(
			t,
			fmt.Sprintf("checkpoint-%d", index+1),
			byte(0x60+index),
		)
		checkpointAuthorities[index] = authority
	}
	liftAuthorities := make([]FrostRetainedGroupAuthority, 3)
	liftPrivateKeys := make([]ed25519.PrivateKey, 3)
	liftPublicKeySPKIs := make([]string, 3)
	for index := range liftAuthorities {
		authority, privateKey, publicKeySPKI := journalTestAuthority(
			t,
			fmt.Sprintf("lift-%d", index+1),
			byte(0x70+index),
		)
		liftAuthorities[index] = authority
		liftPrivateKeys[index] = privateKey
		liftPublicKeySPKIs[index] = publicKeySPKI
	}
	quarantine := FrostRetainedGroupQuarantineJournalManifest{
		ProtocolID:                   [32]byte{0x41},
		LiftProtocolID:               [32]byte{0x45},
		TombstoneProtocolID:          [32]byte{0x46},
		CheckpointAuthorityThreshold: 2,
		CheckpointAuthorities:        checkpointAuthorities,
		LiftAuthorityThreshold:       2,
		LiftAuthorities:              liftAuthorities,
		StoreID:                      "quarantine-store-uuid",
		StoreFingerprint:             [32]byte{0x42},
		ClusterFingerprint:           [32]byte{0x43},
	}
	manifestHash := [32]byte{0x42}
	bindingHash := [32]byte{0x44}
	domainChainID := [32]byte{}
	domainChainID[31] = 1
	runtime := FrostPreSignActivationRuntimeManifest{
		ManifestHash:                 manifestHash,
		ActivationAuthorityKeyHash:   [32]byte{0x47},
		VerifierOperatorFingerprint:  [32]byte{0x48},
		HandshakeOperatorFingerprint: [32]byte{0x4d},
		DomainChainID:                domainChainID,
		GenesisBlockHash:             [32]byte{0x49},
		ProfileHash:                  [32]byte{0x4a},
		ImplementationSetHash:        [32]byte{0x4b},
		AttestationSignerKeyHash:     [32]byte{0x4c},
		CanonicalJournal:             manifest,
		QuarantineJournal:            quarantine,
	}
	source := &journalHistorySource{
		identity: FrostRetainedGroupHistoryIdentity{
			TrustDomainID:       manifest.SourceTrustDomainID,
			EndpointFingerprint: manifest.SourceEndpointFingerprint,
			OperatorFingerprint: manifest.SourceOperatorFingerprint,
		},
		checkpoint:  checkpoint,
		head:        FrostPreSignFinality{BlockNumber: 100, BlockHash: [32]byte{0x64}},
		descriptor:  manifest.DescriptorSetHash,
		complete:    true,
		emptyAtFrom: true,
		points: map[uint64][32]byte{
			1:   checkpoint.BlockHash,
			10:  target.BlockHash,
			20:  later.BlockHash,
			100: {0x64},
		},
		operators: operators,
	}
	admission := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      2,
			BlockHash:        [32]byte{0x02},
			TransactionHash:  [32]byte{0xa2},
			TransactionIndex: 1,
			LogIndex:         5,
		},
		Kind:                    FrostRetainedGroupAdmissionMutation,
		WalletID:                walletID,
		WalletPublicKeyHash:     walletPKH,
		OperatorIDs:             append([]uint32{}, operatorIDs...),
		RetainedGroupHash:       [32]byte{0x93},
		DkgResultHash:           [32]byte{0x94},
		DkgSubmissionPoint:      FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa1}, TransactionIndex: 0, LogIndex: 1},
		DkgApprovalPoint:        FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa2}, TransactionIndex: 1, LogIndex: 3},
		CreationPoint:           FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa2}, TransactionIndex: 1, LogIndex: 4},
		BridgeRegistrationPoint: FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa2}, TransactionIndex: 1, LogIndex: 5},
	}
	source.mutations = []FrostRetainedGroupMutation{admission}
	return &journalTestFixture{
		manifest:           manifest,
		quarantine:         quarantine,
		runtime:            runtime,
		manifestHash:       manifestHash,
		bindingHash:        bindingHash,
		liftPrivateKeys:    liftPrivateKeys,
		liftPublicKeySPKIs: liftPublicKeySPKIs,
		checkpoint:         checkpoint,
		target:             target,
		later:              later,
		walletID:           walletID,
		walletPKH:          walletPKH,
		operatorIDs:        operatorIDs,
		operatorAddrs:      operatorAddrs,
		localOperator:      localOperator,
		registry:           registry,
		source:             source,
		admission:          admission,
	}
}

func TestFrostLocalSessionSnapshotBindsExactSignerMaterial(t *testing.T) {
	fixture := newJournalTestFixture(t)
	sessions, err := fixture.registry.frostLocalSessionSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	expectedKeyGroup := fmt.Sprintf("%x", fixture.walletID)
	if len(sessions) != 1 || sessions[0].WalletID != fixture.walletID ||
		sessions[0].KeyGroup != expectedKeyGroup {
		t.Fatalf("unexpected local FROST session snapshot: [%+v]", sessions)
	}

	mismatchedPayload, err := json.Marshal(
		frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup:       strings.Repeat("22", 32),
			KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.registry.walletCache["wallet"].signers[0].signerMaterial =
		&frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: mismatchedPayload,
		}
	if _, err := fixture.registry.frostLocalSessionSnapshot(); err == nil ||
		!strings.Contains(err.Error(), "does not identify its wallet") {
		t.Fatalf("mismatched local key-group material was accepted: [%v]", err)
	}

	scaffoldPayload, err := json.Marshal(
		frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup: strings.Repeat("33", 32),
			KeyGroupSource: frostsigning.
				NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.registry.walletCache["wallet"].signers[0].signerMaterial =
		&frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: scaffoldPayload,
		}
	sessions, err = fixture.registry.frostLocalSessionSnapshot()
	if err != nil {
		t.Fatalf("scaffold session failed readiness classification: [%v]", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("scaffold session classified as retained FROST: [%+v]", sessions)
	}
}

func journalTestAuthority(
	t *testing.T,
	authorityID string,
	seedByte byte,
) (FrostRetainedGroupAuthority, ed25519.PrivateKey, string) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = seedByte
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKeyDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return FrostRetainedGroupAuthority{
			AuthorityID:       authorityID,
			PublicKeySPKIHash: sha256.Sum256(publicKeyDER),
		},
		privateKey,
		base64.StdEncoding.EncodeToString(publicKeyDER)
}

func (fixture *journalTestFixture) liftMutation(
	t *testing.T,
	journal *frostRetainedGroupJournal,
	quarantine FrostRetainedGroupMutation,
	point FrostRetainedGroupEventPoint,
) FrostRetainedGroupMutation {
	t.Helper()
	var raisedRecord FrostRetainedGroupQuarantineRaisedRecord
	for _, existing := range journal.quarantineState.Quarantines {
		if existing.RaisedRecord.QuarantineID == quarantine.QuarantineID {
			raisedRecord = existing.RaisedRecord
			break
		}
	}
	if raisedRecord.QuarantineID == [32]byte{} {
		t.Fatal("active quarantine is absent from the durable journal")
	}
	resolutionFinality := FrostPreSignFinality{
		BlockNumber: 14,
		BlockHash:   [32]byte{0x0e},
	}
	fixture.source.points[resolutionFinality.BlockNumber] =
		resolutionFinality.BlockHash
	body := FrostRetainedGroupQuarantineLiftBody{
		Schema:                 frostRetainedGroupLiftBodySchema,
		ProtocolBindingHash:    journal.liftPolicy.ProtocolBindingHash,
		ManifestHash:           journal.liftPolicy.ManifestHash,
		ProfileHash:            journal.liftPolicy.ProfileHash,
		ImplementationSetHash:  journal.liftPolicy.ImplementationSetHash,
		ChainID:                journal.liftPolicy.ChainID,
		DomainChainID:          journal.liftPolicy.DomainChainID,
		GenesisBlockHash:       journal.liftPolicy.GenesisBlockHash,
		QuarantineProtocolID:   journal.liftPolicy.QuarantineProtocolID,
		LiftProtocolID:         journal.liftPolicy.LiftProtocolID,
		TombstoneProtocolID:    journal.liftPolicy.TombstoneProtocolID,
		AuthoritySetHash:       journal.liftPolicy.AuthoritySetHash,
		QuarantineID:           quarantine.QuarantineID,
		WalletID:               quarantine.WalletID,
		OriginalRaisedRecord:   raisedRecord,
		PriorGeneration:        journal.quarantineState.Generation,
		PriorEventRoot:         journal.quarantineState.Root,
		PriorActiveRoot:        journal.quarantineState.ActiveRoot,
		PriorTombstoneRoot:     journal.quarantineState.TombstoneRoot,
		LiftPoint:              point,
		ResolutionEvidenceHash: [32]byte{0x53},
		ResolutionFinality:     resolutionFinality,
		NotBeforeBlock:         14,
		ExpiresAtBlock:         19,
	}
	lift := FrostRetainedGroupMutation{
		Point:        point,
		Kind:         FrostRetainedGroupQuarantineLiftMutation,
		WalletID:     quarantine.WalletID,
		QuarantineID: quarantine.QuarantineID,
		LiftCertificate: &FrostRetainedGroupQuarantineLiftCertificate{
			Schema: frostRetainedGroupLiftCertificateSchema,
			Body:   body,
		},
	}
	authorityIndexes := make([]int, journal.liftPolicy.AuthorityThreshold)
	for index := range authorityIndexes {
		authorityIndexes[index] = index
	}
	fixture.resignLiftMutation(t, &lift, authorityIndexes)
	return lift
}

func (fixture *journalTestFixture) resignLiftMutation(
	t *testing.T,
	mutation *FrostRetainedGroupMutation,
	authorityIndexes []int,
) {
	t.Helper()
	if mutation == nil || mutation.LiftCertificate == nil {
		t.Fatal("lift mutation certificate is nil")
	}
	bodyHash, err := frostRetainedGroupLiftBodyHash(
		mutation.LiftCertificate.Body,
	)
	if err != nil {
		t.Fatal(err)
	}
	signatureHash := frostRetainedGroupLiftSignatureHash(bodyHash)
	signatures := make(
		[]FrostRetainedGroupQuarantineLiftSignature,
		len(authorityIndexes),
	)
	for index, authorityIndex := range authorityIndexes {
		if authorityIndex < 0 ||
			authorityIndex >= len(fixture.runtime.QuarantineJournal.LiftAuthorities) ||
			authorityIndex >= len(fixture.liftPrivateKeys) ||
			authorityIndex >= len(fixture.liftPublicKeySPKIs) {
			t.Fatalf("invalid lift authority index [%d]", authorityIndex)
		}
		authority := fixture.runtime.QuarantineJournal.
			LiftAuthorities[authorityIndex]
		signatures[index] = FrostRetainedGroupQuarantineLiftSignature{
			AuthorityID:         authority.AuthorityID,
			SignerPublicKeySPKI: fixture.liftPublicKeySPKIs[authorityIndex],
			Signature: base64.StdEncoding.EncodeToString(
				ed25519.Sign(
					fixture.liftPrivateKeys[authorityIndex],
					signatureHash[:],
				),
			),
		}
	}
	mutation.LiftCertificate.Schema = frostRetainedGroupLiftCertificateSchema
	mutation.LiftCertificate.BodyHash = bodyHash
	mutation.LiftCertificate.Signatures = signatures
	refreshJournalTestLiftCertificateHash(t, mutation)
}

func refreshJournalTestLiftCertificateHash(
	t *testing.T,
	mutation *FrostRetainedGroupMutation,
) {
	t.Helper()
	if mutation == nil || mutation.LiftCertificate == nil {
		t.Fatal("lift mutation certificate is nil")
	}
	certificateHash, err := frostRetainedGroupLiftCertificateHash(
		*mutation.LiftCertificate,
	)
	if err != nil {
		t.Fatal(err)
	}
	mutation.LiftCertificateHash = certificateHash
}

func (fixture *journalTestFixture) openJournal(
	t *testing.T,
	directory string,
) *frostRetainedGroupJournal {
	t.Helper()
	journal, err := newFrostRetainedGroupJournal(
		directory,
		fixture.bindingHash,
		fixture.runtime,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	)
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func (fixture *journalTestFixture) openJournalError(
	directory string,
) error {
	journal, err := newFrostRetainedGroupJournal(
		directory,
		fixture.bindingHash,
		fixture.runtime,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	)
	if journal != nil {
		_ = journal.close()
	}
	return err
}

func (fixture *journalTestFixture) openActiveQuarantine(
	t *testing.T,
	directory string,
) (*frostRetainedGroupJournal, FrostRetainedGroupMutation) {
	t.Helper()
	quarantine := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      5,
			BlockHash:        [32]byte{0x05},
			TransactionHash:  [32]byte{0xa5},
			TransactionIndex: 1,
			LogIndex:         1,
		},
		Kind:         FrostRetainedGroupRecoveryRequiredMutation,
		WalletID:     fixture.walletID,
		QuarantineID: [32]byte{0x51},
		EvidenceHash: [32]byte{0x52},
		Reason:       "manual recovery is required",
	}
	fixture.source.mutations = append(fixture.source.mutations, quarantine)
	journal := fixture.openJournal(t, directory)
	snapshot, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		_ = journal.close()
		t.Fatal(err)
	}
	if snapshot.QuarantineCount != 1 ||
		snapshot.QuarantineTombstoneCount != 0 {
		_ = journal.close()
		t.Fatalf("unexpected active quarantine snapshot: %+v", snapshot)
	}
	return journal, quarantine
}

func validateJournalTestLift(
	journal *frostRetainedGroupJournal,
	lift FrostRetainedGroupMutation,
) error {
	if journal == nil || len(journal.quarantineState.Quarantines) != 1 {
		return fmt.Errorf("journal does not contain exactly one quarantine")
	}
	_, err := validateFrostRetainedGroupLiftCertificate(
		journal.liftPolicy,
		journal.quarantineState,
		lift,
		journal.quarantineState.Quarantines[0],
	)
	return err
}

func TestFrostRetainedGroupJournal_ReconcilesAndRejectsRewrittenHistory(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	defer journal.close()
	snapshot, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Complete || snapshot.SnapshotGeneration != 1 ||
		snapshot.WalletCount != 1 || snapshot.LocalSessionCount != 1 ||
		snapshot.QuarantineCount != 0 || snapshot.InventoryRoot == [32]byte{} {
		t.Fatalf("unexpected journal snapshot: %+v", snapshot)
	}

	fixture.source.mutations[0].OperatorIDs[0] = 52
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "rewrote, omitted, or reordered") {
		t.Fatalf("expected canonical prefix rewrite failure, got [%v]", err)
	}
	fixture.source.mutations[0] = fixture.admission
	fixture.source.complete = false
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete-history failure, got [%v]", err)
	}
	fixture.source.complete = true
	fixture.source.emptyAtFrom = false
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected nonempty-checkpoint failure, got [%v]", err)
	}
}

func TestFrostRetainedGroupJournal_IntegratesCommittedOrphanBatchExactlyOnce(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	journal.persistFailureHook = func(stage string) error {
		if stage != "after-batch-before-state" {
			t.Fatalf("unexpected failure stage [%s]", stage)
		}
		return fmt.Errorf("simulated crash")
	}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "simulated crash") {
		t.Fatalf("expected simulated crash, got [%v]", err)
	}
	if err := journal.close(); err != nil {
		t.Fatal(err)
	}

	restarted := fixture.openJournal(t, directory)
	defer restarted.close()
	snapshot, err := restarted.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SnapshotGeneration != 1 || restarted.state.BatchSequence != 1 ||
		len(restarted.mutations) != 1 {
		t.Fatalf("orphan batch was not integrated exactly once: %+v", restarted.state)
	}
}

func TestFrostRetainedGroupJournal_RejectsAuthenticatedPriorSchemaFixtures(
	t *testing.T,
) {
	type legacyFixture struct {
		schemas [2]string
		mutate  func(*testing.T, string, string)
	}
	testCases := map[string]legacyFixture{
		"canonical metadata": {
			schemas: [2]string{
				frostRetainedGroupJournalMetadataSchemaV1,
				frostRetainedGroupJournalMetadataSchemaV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupCanonicalDirectory)
				metadata := frostRetainedGroupJournalMetadata{}
				if err := readFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalMetadataFile,
					&metadata,
				); err != nil {
					t.Fatal(err)
				}
				metadata.Schema = schema
				if err := persistFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalMetadataFile,
					&metadata,
					true,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		"canonical state": {
			schemas: [2]string{
				frostRetainedGroupJournalStateSchemaV1,
				frostRetainedGroupJournalStateSchemaV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupCanonicalDirectory)
				state := frostRetainedGroupJournalState{}
				if err := readFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalStateFile,
					&state,
				); err != nil {
					t.Fatal(err)
				}
				state.Schema = schema
				if err := persistFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalStateFile,
					&state,
					true,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		"canonical batch": {
			schemas: [2]string{
				frostRetainedGroupJournalBatchSchemaV1,
				frostRetainedGroupJournalBatchSchemaV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupCanonicalDirectory)
				name := frostRetainedGroupBatchFileName(1)
				batch := frostRetainedGroupJournalBatch{}
				if err := readFrostRetainedGroupEnvelopeAt(path, name, &batch); err != nil {
					t.Fatal(err)
				}
				batch.Schema = schema
				if schema == frostRetainedGroupJournalBatchSchemaV1 {
					for index := range batch.Mutations {
						batch.Mutations[index].DkgResultHash = [32]byte{}
						batch.Mutations[index].DkgSubmissionPoint = FrostRetainedGroupEventPoint{}
						batch.Mutations[index].DkgApprovalPoint = FrostRetainedGroupEventPoint{}
					}
				}
				batch.Checksum = [32]byte{}
				payload, err := frostRetainedGroupCanonicalValue(batch)
				if err != nil {
					t.Fatal(err)
				}
				batch.Checksum = sha256.Sum256(payload)
				if err := persistFrostRetainedGroupEnvelopeAt(path, name, &batch, true); err != nil {
					t.Fatal(err)
				}
			},
		},
		"quarantine metadata": {
			schemas: [2]string{
				frostRetainedGroupQuarantineMetadataV1,
				frostRetainedGroupQuarantineMetadataV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupQuarantineDirectory)
				metadata := frostRetainedGroupQuarantineMetadata{}
				if err := readFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalMetadataFile,
					&metadata,
				); err != nil {
					t.Fatal(err)
				}
				metadata.Schema = schema
				if err := persistFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalMetadataFile,
					&metadata,
					true,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		"quarantine state": {
			schemas: [2]string{
				frostRetainedGroupQuarantineStateV1,
				frostRetainedGroupQuarantineStateV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupQuarantineDirectory)
				state := frostRetainedGroupQuarantineJournalState{}
				if err := readFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalStateFile,
					&state,
				); err != nil {
					t.Fatal(err)
				}
				state.Schema = schema
				if err := persistFrostRetainedGroupEnvelopeAt(
					path,
					frostRetainedGroupJournalStateFile,
					&state,
					true,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		"quarantine batch": {
			schemas: [2]string{
				frostRetainedGroupQuarantineBatchV1,
				frostRetainedGroupQuarantineBatchV2,
			},
			mutate: func(t *testing.T, directory string, schema string) {
				path := filepath.Join(directory, frostRetainedGroupQuarantineDirectory)
				name := frostRetainedGroupBatchFileName(1)
				batch := frostRetainedGroupWireQuarantineJournalBatch{}
				if err := readFrostRetainedGroupEnvelopeAt(path, name, &batch); err != nil {
					t.Fatal(err)
				}
				batch.Schema = schema
				batch.Checksum = frostActivationHex32([32]byte{})
				payload, err := frostRetainedGroupCanonicalValue(batch)
				if err != nil {
					t.Fatal(err)
				}
				batch.Checksum = frostActivationHex32(sha256.Sum256(payload))
				if err := persistFrostRetainedGroupEnvelopeAt(path, name, &batch, true); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for componentName, testCase := range testCases {
		for versionIndex, schema := range testCase.schemas {
			t.Run(fmt.Sprintf("%s/v%d", componentName, versionIndex+1), func(t *testing.T) {
				fixture := newJournalTestFixture(t)
				quarantine := FrostRetainedGroupMutation{
					Point: FrostRetainedGroupEventPoint{
						BlockNumber:      5,
						BlockHash:        [32]byte{0x05},
						TransactionHash:  [32]byte{0xa5},
						TransactionIndex: 1,
						LogIndex:         1,
					},
					Kind:         FrostRetainedGroupQuarantineMutation,
					WalletID:     fixture.walletID,
					QuarantineID: [32]byte{0x61},
					EvidenceHash: [32]byte{0x62},
					Reason:       "authenticated prior-schema fixture",
				}
				fixture.source.mutations = append(fixture.source.mutations, quarantine)
				directory := filepath.Join(t.TempDir(), "journal")
				journal := fixture.openJournal(t, directory)
				if _, err := journal.reconcile(context.Background(), fixture.target); err != nil {
					t.Fatal(err)
				}
				if err := journal.close(); err != nil {
					t.Fatal(err)
				}

				testCase.mutate(t, directory, schema)
				err := fixture.openJournalError(directory)
				if err == nil ||
					!strings.Contains(err.Error(), "prior FROST retained-group") ||
					!strings.Contains(err.Error(), "not safely migratable") ||
					!strings.Contains(err.Error(), "new empty v3") {
					t.Fatalf(
						"expected explicit manifest-pinned prior-schema rejection, got [%v]",
						err,
					)
				}
			})
		}
	}
}

func TestFrostRetainedGroupJournal_RejectsCrossBindingBatchReplay(
	t *testing.T,
) {
	testCases := map[string]struct {
		addQuarantine bool
		directory     string
	}{
		"canonical": {
			directory: frostRetainedGroupCanonicalDirectory,
		},
		"quarantine": {
			addQuarantine: true,
			directory:     frostRetainedGroupQuarantineDirectory,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := newJournalTestFixture(t)
			if testCase.addQuarantine {
				fixture.source.mutations = append(
					fixture.source.mutations,
					FrostRetainedGroupMutation{
						Point: FrostRetainedGroupEventPoint{
							BlockNumber:      5,
							BlockHash:        [32]byte{0x05},
							TransactionHash:  [32]byte{0xa5},
							TransactionIndex: 1,
							LogIndex:         1,
						},
						Kind:         FrostRetainedGroupQuarantineMutation,
						WalletID:     fixture.walletID,
						QuarantineID: [32]byte{0x61},
						EvidenceHash: [32]byte{0x62},
						Reason:       "binding replay test",
					},
				)
			}

			rootDirectory := filepath.Join(t.TempDir(), "journal")
			journal := fixture.openJournal(t, rootDirectory)
			if _, err := journal.reconcile(context.Background(), fixture.target); err != nil {
				t.Fatal(err)
			}
			if err := journal.close(); err != nil {
				t.Fatal(err)
			}

			directory := filepath.Join(rootDirectory, testCase.directory)
			batchName := frostRetainedGroupBatchFileName(1)
			if testCase.addQuarantine {
				batch := frostRetainedGroupWireQuarantineJournalBatch{}
				if err := readFrostRetainedGroupEnvelopeAt(
					directory,
					batchName,
					&batch,
				); err != nil {
					t.Fatal(err)
				}
				batch.BindingHash = frostActivationHex32([32]byte{0xff})
				batch.Checksum = frostActivationHex32([32]byte{})
				payload, err := frostRetainedGroupCanonicalValue(batch)
				if err != nil {
					t.Fatal(err)
				}
				batch.Checksum = frostActivationHex32(sha256.Sum256(payload))
				if err := persistFrostRetainedGroupEnvelopeAt(
					directory,
					batchName,
					&batch,
					true,
				); err != nil {
					t.Fatal(err)
				}
			} else {
				batch := frostRetainedGroupJournalBatch{}
				if err := readFrostRetainedGroupEnvelopeAt(
					directory,
					batchName,
					&batch,
				); err != nil {
					t.Fatal(err)
				}
				batch.BindingHash = [32]byte{0xff}
				batch.Checksum = [32]byte{}
				payload, err := frostRetainedGroupCanonicalValue(batch)
				if err != nil {
					t.Fatal(err)
				}
				batch.Checksum = sha256.Sum256(payload)
				if err := persistFrostRetainedGroupEnvelopeAt(
					directory,
					batchName,
					&batch,
					true,
				); err != nil {
					t.Fatal(err)
				}
			}

			err := fixture.openJournalError(rootDirectory)
			if err == nil || !strings.Contains(err.Error(), "batch header is invalid") {
				t.Fatalf("expected cross-binding batch replay rejection, got [%v]", err)
			}
		})
	}
}

func TestFrostRetainedGroupJournal_QuarantineAndAuthenticatedLiftAreIndependent(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	quarantine := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      5,
			BlockHash:        [32]byte{0x05},
			TransactionHash:  [32]byte{0xa5},
			TransactionIndex: 1,
			LogIndex:         1,
		},
		Kind:         FrostRetainedGroupRecoveryRequiredMutation,
		WalletID:     fixture.walletID,
		QuarantineID: [32]byte{0x51},
		EvidenceHash: [32]byte{0x52},
		Reason:       "manual recovery is required",
	}
	fixture.source.mutations = append(fixture.source.mutations, quarantine)
	journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
	defer journal.close()
	first, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if first.SnapshotGeneration != 1 || first.QuarantineGeneration != 1 ||
		first.QuarantineCount != 1 {
		t.Fatalf("unexpected quarantined snapshot: %+v", first)
	}
	if journal.directory == journal.quarantineDirectory {
		t.Fatal("canonical and quarantine journals share a physical store")
	}
	if len(journal.mutations) != 1 ||
		len(journal.quarantineMutations) != 1 ||
		isFrostRetainedGroupQuarantineMutation(journal.mutations[0].Kind) ||
		!isFrostRetainedGroupQuarantineMutation(journal.quarantineMutations[0].Kind) {
		t.Fatal("canonical and quarantine mutations were not durably partitioned")
	}
	for _, metadataPath := range []string{
		filepath.Join(journal.directory, frostRetainedGroupJournalMetadataFile),
		filepath.Join(journal.quarantineDirectory, frostRetainedGroupJournalMetadataFile),
	} {
		if info, err := os.Lstat(metadataPath); err != nil ||
			!info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("independent journal metadata is not durable and private: [%s] [%v]", metadataPath, err)
		}
	}
	lift := fixture.liftMutation(
		t,
		journal,
		quarantine,
		FrostRetainedGroupEventPoint{
			BlockNumber:      15,
			BlockHash:        [32]byte{0x0f},
			TransactionHash:  [32]byte{0xaf},
			TransactionIndex: 2,
			LogIndex:         3,
		},
	)
	fixture.source.mutations = append(fixture.source.mutations, lift)
	second, err := journal.reconcile(context.Background(), fixture.later)
	if err != nil {
		t.Fatal(err)
	}
	if second.SnapshotGeneration != 1 || second.QuarantineGeneration != 2 ||
		second.QuarantineCount != 0 || second.QuarantineTombstoneCount != 1 ||
		second.QuarantineRoot == first.QuarantineRoot ||
		second.QuarantineActiveRoot == first.QuarantineActiveRoot ||
		second.QuarantineTombstoneRoot == first.QuarantineTombstoneRoot {
		t.Fatalf("unexpected lifted snapshot: %+v", second)
	}
	if len(journal.quarantineState.Quarantines) != 1 ||
		journal.quarantineState.Quarantines[0].Status != frostRetainedGroupQuarantineLifted ||
		len(journal.quarantineState.Tombstones) != 1 {
		t.Fatal("lift did not retain its immutable record and permanent tombstone")
	}
}

func TestFrostRetainedGroupJournal_LiftCertificateRejectsBypasses(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	journal, quarantine := fixture.openActiveQuarantine(
		t,
		filepath.Join(t.TempDir(), "journal"),
	)
	defer journal.close()
	lift := fixture.liftMutation(
		t,
		journal,
		quarantine,
		FrostRetainedGroupEventPoint{
			BlockNumber:      15,
			BlockHash:        [32]byte{0x0f},
			TransactionHash:  [32]byte{0xaf},
			TransactionIndex: 2,
			LogIndex:         3,
		},
	)
	if err := validateJournalTestLift(journal, lift); err != nil {
		t.Fatalf("valid lift certificate was rejected: [%v]", err)
	}

	substitutions := map[string]struct {
		mutate func(*FrostRetainedGroupMutation)
		resign bool
	}{
		"unsigned body substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ManifestHash[0] ^= 0xff
			},
		},
		"signed manifest substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ManifestHash[0] ^= 0xff
			},
			resign: true,
		},
		"signed quarantine ID substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.QuarantineID[0] ^= 0xff
			},
			resign: true,
		},
		"signed wallet substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.WalletID[0] ^= 0xff
			},
			resign: true,
		},
		"signed raised evidence substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.OriginalRaisedRecord.
					EvidenceHash[0] ^= 0xff
			},
			resign: true,
		},
		"signed raised reason substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.OriginalRaisedRecord.Reason +=
					" altered"
			},
			resign: true,
		},
		"signed recovery flag substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.OriginalRaisedRecord.
					RecoveryRequired = false
			},
			resign: true,
		},
		"signed prior generation substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.PriorGeneration++
			},
			resign: true,
		},
		"signed prior event root substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.PriorEventRoot[0] ^= 0xff
			},
			resign: true,
		},
		"signed prior active root substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.PriorActiveRoot[0] ^= 0xff
			},
			resign: true,
		},
		"signed prior tombstone root substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.PriorTombstoneRoot[0] ^= 0xff
			},
			resign: true,
		},
		"signed lift point substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.LiftPoint.
					TransactionHash[0] ^= 0xff
			},
			resign: true,
		},
		"future resolution finality": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ResolutionFinality =
					FrostPreSignFinality{
						BlockNumber: 16,
						BlockHash:   [32]byte{0x10},
					}
			},
			resign: true,
		},
		"resolution finality before quarantine": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ResolutionFinality =
					FrostPreSignFinality{
						BlockNumber: 4,
						BlockHash:   [32]byte{0x04},
					}
			},
			resign: true,
		},
		"same-height conflicting resolution hash": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ResolutionFinality =
					FrostPreSignFinality{
						BlockNumber: 15,
						BlockHash:   [32]byte{0xee},
					}
			},
			resign: true,
		},
		"unsafe canonical JSON integer": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificate.Body.ExpiresAtBlock =
					frostRetainedGroupMaximumCanonicalJSONInteger + 1
			},
		},
		"certificate reference substitution": {
			mutate: func(candidate *FrostRetainedGroupMutation) {
				candidate.LiftCertificateHash[0] ^= 0xff
			},
		},
	}
	for name, testCase := range substitutions {
		t.Run(name, func(t *testing.T) {
			candidate := cloneFrostRetainedGroupMutations(
				[]FrostRetainedGroupMutation{lift},
			)[0]
			testCase.mutate(&candidate)
			if testCase.resign {
				fixture.resignLiftMutation(t, &candidate, []int{0, 1})
			}
			if err := validateJournalTestLift(journal, candidate); err == nil {
				t.Fatal("substituted lift certificate was accepted")
			}
		})
	}

	irrelevantFields := map[string]func(*FrostRetainedGroupMutation){
		"wallet public key hash": func(candidate *FrostRetainedGroupMutation) {
			candidate.WalletPublicKeyHash = [20]byte{0x99}
		},
		"whitespace reason": func(candidate *FrostRetainedGroupMutation) {
			candidate.Reason = " "
		},
	}
	for name, mutate := range irrelevantFields {
		t.Run("irrelevant "+name, func(t *testing.T) {
			candidate := cloneFrostRetainedGroupMutations(
				[]FrostRetainedGroupMutation{lift},
			)[0]
			mutate(&candidate)
			state := cloneFrostRetainedGroupQuarantineState(
				journal.quarantineState,
			)
			if err := applyFrostRetainedGroupQuarantineMutations(
				&state,
				[]FrostRetainedGroupMutation{candidate},
				journal.liftPolicy,
			); err == nil {
				t.Fatal("lift with an unsigned irrelevant field was accepted")
			}
		})
	}

	t.Run("insufficient quorum", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		fixture.resignLiftMutation(t, &candidate, []int{0})
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("one-of-three lift quorum was accepted")
		}
	})
	t.Run("unsorted signatures", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		fixture.resignLiftMutation(t, &candidate, []int{1, 0})
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("unsorted lift signatures were accepted")
		}
	})
	t.Run("duplicate signatures", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		fixture.resignLiftMutation(t, &candidate, []int{0, 0})
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("duplicate lift signatures were accepted")
		}
	})
	t.Run("unknown extra signature", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		fixture.resignLiftMutation(t, &candidate, []int{0, 1})
		_, unknownPrivateKey, unknownSPKI := journalTestAuthority(
			t,
			"zz-unknown",
			0x7f,
		)
		signatureHash := frostRetainedGroupLiftSignatureHash(
			candidate.LiftCertificate.BodyHash,
		)
		candidate.LiftCertificate.Signatures = append(
			candidate.LiftCertificate.Signatures,
			FrostRetainedGroupQuarantineLiftSignature{
				AuthorityID:         "zz-unknown",
				SignerPublicKeySPKI: unknownSPKI,
				Signature: base64.StdEncoding.EncodeToString(
					ed25519.Sign(unknownPrivateKey, signatureHash[:]),
				),
			},
		)
		refreshJournalTestLiftCertificateHash(t, &candidate)
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("unknown extra lift signature was accepted")
		}
	})
	t.Run("invalid extra signature", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		fixture.resignLiftMutation(t, &candidate, []int{0, 1, 2})
		signature, err := base64.StdEncoding.Strict().DecodeString(
			candidate.LiftCertificate.Signatures[2].Signature,
		)
		if err != nil {
			t.Fatal(err)
		}
		signature[0] ^= 0xff
		candidate.LiftCertificate.Signatures[2].Signature =
			base64.StdEncoding.EncodeToString(signature)
		refreshJournalTestLiftCertificateHash(t, &candidate)
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("invalid extra lift signature was accepted")
		}
	})
	t.Run("noncanonical base64", func(t *testing.T) {
		candidate := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{lift},
		)[0]
		candidate.LiftCertificate.Signatures[0].SignerPublicKeySPKI =
			"\n" + candidate.LiftCertificate.Signatures[0].SignerPublicKeySPKI
		refreshJournalTestLiftCertificateHash(t, &candidate)
		if err := validateJournalTestLift(journal, candidate); err == nil {
			t.Fatal("noncanonical base64 lift credential was accepted")
		}
	})
}

func TestFrostRetainedGroupJournal_LiftAuthorityStrictMajority(
	t *testing.T,
) {
	addFourthAuthority := func(
		t *testing.T,
		fixture *journalTestFixture,
		threshold uint64,
	) {
		authority, privateKey, publicKeySPKI := journalTestAuthority(
			t,
			"lift-4",
			0x73,
		)
		authorities := append(
			[]FrostRetainedGroupAuthority{},
			fixture.runtime.QuarantineJournal.LiftAuthorities...,
		)
		authorities = append(authorities, authority)
		fixture.runtime.QuarantineJournal.LiftAuthorityThreshold = threshold
		fixture.runtime.QuarantineJournal.LiftAuthorities = authorities
		fixture.quarantine = fixture.runtime.QuarantineJournal
		fixture.liftPrivateKeys = append(
			fixture.liftPrivateKeys,
			privateKey,
		)
		fixture.liftPublicKeySPKIs = append(
			fixture.liftPublicKeySPKIs,
			publicKeySPKI,
		)
	}
	t.Run("2-of-4 rejected", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		addFourthAuthority(t, fixture, 2)
		err := fixture.openJournalError(
			filepath.Join(t.TempDir(), "journal"),
		)
		if err == nil || !strings.Contains(err.Error(), "strict majority") {
			t.Fatalf("expected 2-of-4 policy rejection, got [%v]", err)
		}
	})
	t.Run("3-of-4 accepted", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		addFourthAuthority(t, fixture, 3)
		journal, quarantine := fixture.openActiveQuarantine(
			t,
			filepath.Join(t.TempDir(), "journal"),
		)
		defer journal.close()
		lift := fixture.liftMutation(
			t,
			journal,
			quarantine,
			FrostRetainedGroupEventPoint{
				BlockNumber:      15,
				BlockHash:        [32]byte{0x0f},
				TransactionHash:  [32]byte{0xaf},
				TransactionIndex: 2,
				LogIndex:         3,
			},
		)
		if len(lift.LiftCertificate.Signatures) != 3 {
			t.Fatal("3-of-4 policy did not produce three signatures")
		}
		if err := validateJournalTestLift(journal, lift); err != nil {
			t.Fatalf("valid 3-of-4 lift was rejected: [%v]", err)
		}
	})
}

func TestFrostRetainedGroupJournal_LiftCertificateFrozenWireVector(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	journal, quarantine := fixture.openActiveQuarantine(
		t,
		filepath.Join(t.TempDir(), "journal"),
	)
	defer journal.close()
	lift := fixture.liftMutation(
		t,
		journal,
		quarantine,
		FrostRetainedGroupEventPoint{
			BlockNumber:      15,
			BlockHash:        [32]byte{0x0f},
			TransactionHash:  [32]byte{0xaf},
			TransactionIndex: 2,
			LogIndex:         3,
		},
	)
	wire := frostRetainedGroupLiftCertificateToWire(lift.LiftCertificate)
	canonical, err := frostRetainedGroupCanonicalValue(wire)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(
		string(canonical),
		`"protocolBindingHash":[`,
	) || !strings.Contains(
		string(canonical),
		`"protocolBindingHash":"0x`,
	) {
		t.Fatal("lift certificate did not use the explicit hex32 wire contract")
	}
	vectors := map[string]struct {
		actual   [32]byte
		expected string
	}{
		"authority set": {
			actual:   journal.liftPolicy.AuthoritySetHash,
			expected: "b08dcb52b095337400460f2df2a4b490c1d59e9995a0980489eb0d5540a2ee57",
		},
		"body": {
			actual:   lift.LiftCertificate.BodyHash,
			expected: "a39727457786e7ad6bf67f3ea3cd7777c766e54d761684324bfdfa19c8030686",
		},
		"certificate": {
			actual:   lift.LiftCertificateHash,
			expected: "7000fbb0730db155daa390ba9f8a0641ebc20ce7ee1ccee34b9ff987d7501e0d",
		},
	}
	for name, vector := range vectors {
		if fmt.Sprintf("%x", vector.actual) != vector.expected {
			t.Fatalf(
				"%s wire vector changed: got [%x], expected [%s]",
				name,
				vector.actual,
				vector.expected,
			)
		}
	}
}

func TestFrostRetainedGroupJournal_IntegratesQuarantineOrphanBatchExactlyOnce(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	fixture.source.mutations = append(
		fixture.source.mutations,
		FrostRetainedGroupMutation{
			Point: FrostRetainedGroupEventPoint{
				BlockNumber:      5,
				BlockHash:        [32]byte{0x05},
				TransactionHash:  [32]byte{0xa5},
				TransactionIndex: 1,
				LogIndex:         1,
			},
			Kind:         FrostRetainedGroupQuarantineMutation,
			WalletID:     fixture.walletID,
			QuarantineID: [32]byte{0x61},
			EvidenceHash: [32]byte{0x62},
			Reason:       "independent quarantine crash test",
		},
	)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	journal.persistFailureHook = func(stage string) error {
		switch stage {
		case "after-batch-before-state":
			return nil
		case "after-quarantine-batch-before-state":
			return fmt.Errorf("simulated quarantine crash")
		default:
			t.Fatalf("unexpected failure stage [%s]", stage)
			return nil
		}
	}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "simulated quarantine crash") {
		t.Fatalf("expected simulated quarantine crash, got [%v]", err)
	}
	if err := journal.close(); err != nil {
		t.Fatal(err)
	}

	restarted := fixture.openJournal(t, directory)
	defer restarted.close()
	snapshot, err := restarted.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.QuarantineGeneration != 1 || snapshot.QuarantineCount != 1 ||
		restarted.quarantineState.BatchSequence != 1 ||
		len(restarted.quarantineMutations) != 1 {
		t.Fatalf("quarantine orphan batch was not integrated exactly once: %+v", restarted.quarantineState)
	}
}

func TestFrostRetainedGroupJournal_LiftCrashRecoveryAndContentAddressing(
	t *testing.T,
) {
	liftPoint := FrostRetainedGroupEventPoint{
		BlockNumber:      15,
		BlockHash:        [32]byte{0x0f},
		TransactionHash:  [32]byte{0xaf},
		TransactionIndex: 2,
		LogIndex:         3,
	}
	t.Run("certificate-only orphan is inert", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		directory := filepath.Join(t.TempDir(), "journal")
		journal, quarantine := fixture.openActiveQuarantine(
			t,
			directory,
		)
		lift := fixture.liftMutation(t, journal, quarantine, liftPoint)
		fixture.source.mutations = append(fixture.source.mutations, lift)
		journal.persistFailureHook = func(stage string) error {
			switch stage {
			case "after-batch-before-state":
				return nil
			case "after-quarantine-lift-certificate-before-batch":
				return fmt.Errorf("simulated certificate-only crash")
			default:
				t.Fatalf("unexpected failure stage [%s]", stage)
				return nil
			}
		}
		if _, err := journal.reconcile(
			context.Background(),
			fixture.later,
		); err == nil || !strings.Contains(err.Error(), "certificate-only") {
			t.Fatalf("expected certificate-only crash, got [%v]", err)
		}
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}

		restarted := fixture.openJournal(t, directory)
		defer restarted.close()
		if len(restarted.liftCertificates) != 1 ||
			frostRetainedGroupActiveQuarantineCount(
				restarted.quarantineState,
			) != 1 ||
			len(restarted.quarantineState.Tombstones) != 0 {
			t.Fatal("certificate-only orphan changed quarantine state")
		}
		snapshot, err := restarted.reconcile(
			context.Background(),
			fixture.later,
		)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.QuarantineCount != 0 ||
			snapshot.QuarantineTombstoneCount != 1 {
			t.Fatalf("certificate-only recovery did not lift exactly once: %+v", snapshot)
		}
	})

	t.Run("certificate and batch orphan integrate exactly once", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		directory := filepath.Join(t.TempDir(), "journal")
		journal, quarantine := fixture.openActiveQuarantine(
			t,
			directory,
		)
		lift := fixture.liftMutation(t, journal, quarantine, liftPoint)
		fixture.source.mutations = append(fixture.source.mutations, lift)
		journal.persistFailureHook = func(stage string) error {
			switch stage {
			case "after-batch-before-state",
				"after-quarantine-lift-certificate-before-batch":
				return nil
			case "after-quarantine-batch-before-state":
				return fmt.Errorf("simulated certificate-and-batch crash")
			default:
				t.Fatalf("unexpected failure stage [%s]", stage)
				return nil
			}
		}
		if _, err := journal.reconcile(
			context.Background(),
			fixture.later,
		); err == nil || !strings.Contains(err.Error(), "certificate-and-batch") {
			t.Fatalf("expected certificate-and-batch crash, got [%v]", err)
		}
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}

		restarted := fixture.openJournal(t, directory)
		defer restarted.close()
		if frostRetainedGroupActiveQuarantineCount(
			restarted.quarantineState,
		) != 0 ||
			len(restarted.quarantineState.Tombstones) != 1 ||
			restarted.quarantineState.BatchSequence != 2 {
			t.Fatalf(
				"certificate-and-batch orphan was not integrated: %+v",
				restarted.quarantineState,
			)
		}
		snapshot, err := restarted.reconcile(
			context.Background(),
			fixture.later,
		)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.QuarantineCount != 0 ||
			snapshot.QuarantineTombstoneCount != 1 ||
			restarted.quarantineState.BatchSequence != 2 {
			t.Fatalf("orphan lift was not integrated exactly once: %+v", snapshot)
		}
	})

	t.Run("conflicting valid certificate orphan has a distinct digest", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		directory := filepath.Join(t.TempDir(), "journal")
		journal, quarantine := fixture.openActiveQuarantine(
			t,
			directory,
		)
		firstLift := fixture.liftMutation(t, journal, quarantine, liftPoint)
		fixture.source.mutations = append(
			fixture.source.mutations,
			firstLift,
		)
		journal.persistFailureHook = func(stage string) error {
			switch stage {
			case "after-batch-before-state":
				return nil
			case "after-quarantine-lift-certificate-before-batch":
				return fmt.Errorf("simulated first certificate crash")
			default:
				t.Fatalf("unexpected failure stage [%s]", stage)
				return nil
			}
		}
		if _, err := journal.reconcile(
			context.Background(),
			fixture.later,
		); err == nil {
			t.Fatal("expected first certificate crash")
		}
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}

		restarted := fixture.openJournal(t, directory)
		defer restarted.close()
		secondLift := cloneFrostRetainedGroupMutations(
			[]FrostRetainedGroupMutation{firstLift},
		)[0]
		fixture.resignLiftMutation(t, &secondLift, []int{0, 2})
		if secondLift.LiftCertificateHash == firstLift.LiftCertificateHash {
			t.Fatal("different quorum certificates have the same full digest")
		}
		fixture.source.mutations[len(fixture.source.mutations)-1] =
			secondLift
		snapshot, err := restarted.reconcile(
			context.Background(),
			fixture.later,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(restarted.liftCertificates) != 2 ||
			snapshot.QuarantineTombstoneCount != 1 ||
			restarted.quarantineState.Tombstones[0].LiftCertificateHash !=
				secondLift.LiftCertificateHash {
			t.Fatal("content-addressed conflicting certificate handling failed")
		}
	})
}

func TestFrostRetainedGroupJournal_TombstoneRejectsReplayAndReraise(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal, quarantine := fixture.openActiveQuarantine(t, directory)
	lift := fixture.liftMutation(
		t,
		journal,
		quarantine,
		FrostRetainedGroupEventPoint{
			BlockNumber:      15,
			BlockHash:        [32]byte{0x0f},
			TransactionHash:  [32]byte{0xaf},
			TransactionIndex: 2,
			LogIndex:         3,
		},
	)
	fixture.source.mutations = append(fixture.source.mutations, lift)
	if _, err := journal.reconcile(
		context.Background(),
		fixture.later,
	); err != nil {
		t.Fatal(err)
	}

	replayState := cloneFrostRetainedGroupQuarantineState(
		journal.quarantineState,
	)
	if err := applyFrostRetainedGroupQuarantineMutations(
		&replayState,
		[]FrostRetainedGroupMutation{lift},
		journal.liftPolicy,
	); err == nil {
		t.Fatal("tombstoned lift replay was accepted")
	}
	reraise := quarantine
	reraise.Point = FrostRetainedGroupEventPoint{
		BlockNumber:      18,
		BlockHash:        [32]byte{0x12},
		TransactionHash:  [32]byte{0xb2},
		TransactionIndex: 1,
		LogIndex:         1,
	}
	reraise.EvidenceHash = [32]byte{0x54}
	if err := applyFrostRetainedGroupQuarantineMutations(
		&replayState,
		[]FrostRetainedGroupMutation{reraise},
		journal.liftPolicy,
	); err == nil {
		t.Fatal("tombstoned quarantine ID was raised again")
	}
	if err := journal.close(); err != nil {
		t.Fatal(err)
	}
	restarted := fixture.openJournal(t, directory)
	defer restarted.close()
	if len(restarted.quarantineState.Quarantines) != 1 ||
		restarted.quarantineState.Quarantines[0].Status !=
			frostRetainedGroupQuarantineLifted ||
		len(restarted.quarantineState.Tombstones) != 1 {
		t.Fatal("lifted record or permanent tombstone was lost on restart")
	}
}

func TestApplyFrostRetainedGroupMutations_EnforcesLifecycleAndRegistryClosure(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	state := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: fixture.checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	closing := lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosingMutation, [32]byte{0xb3})
	closed := lifecycleMutation(fixture, 4, 1, FrostRetainedGroupClosedMutation, [32]byte{0xb4})
	registryClosed := lifecycleMutation(fixture, 4, 2, FrostRetainedGroupRegistryClosureMutation, [32]byte{0xb4})
	if err := applyFrostRetainedGroupMutations(
		&state,
		[]FrostRetainedGroupMutation{fixture.admission, closing, closed, registryClosed},
	); err != nil {
		t.Fatal(err)
	}
	if state.SnapshotGeneration != 4 || !state.Wallets[0].RegistryClosed ||
		state.Wallets[0].Lifecycle != FrostRetainedGroupClosed {
		t.Fatalf("unexpected lifecycle state: %+v", state)
	}

	invalid := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: fixture.checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	directClose := lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosedMutation, [32]byte{0xc3})
	if err := applyFrostRetainedGroupMutations(
		&invalid,
		[]FrostRetainedGroupMutation{fixture.admission, directClose},
	); err == nil {
		t.Fatal("expected Live -> Closed transition to fail")
	}
}

func TestApplyFrostRetainedGroupMutations_AllowsRepeatedStakeWeightedSeats(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	duplicate := fixture.admission
	duplicate.OperatorIDs = append([]uint32{}, fixture.admission.OperatorIDs...)
	duplicate.OperatorIDs[1] = duplicate.OperatorIDs[0]
	state := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: fixture.manifest.Checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	if err := applyFrostRetainedGroupMutations(
		&state,
		[]FrostRetainedGroupMutation{duplicate},
	); err != nil {
		t.Fatalf("expected repeated operator seats to be retained, got [%v]", err)
	}
	if len(state.Wallets) != 1 ||
		state.Wallets[0].OperatorIDs[0] != state.Wallets[0].OperatorIDs[1] {
		t.Fatalf("repeated stake-weighted seats were not preserved: [%+v]", state.Wallets)
	}
}

func TestApplyFrostRetainedGroupMutations_EnforcesWalletLimit(t *testing.T) {
	state := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: FrostPreSignFinality{BlockNumber: 1, BlockHash: [32]byte{1}},
		Wallets:      []frostRetainedGroupWalletState{},
	}
	err := applyFrostRetainedGroupMutations(
		&state,
		journalTestBoundedAdmissions(frostRetainedGroupMaximumWallets+1),
	)
	if err == nil || !strings.Contains(err.Error(), "wallet limit") {
		t.Fatalf("expected retained-wallet limit rejection, got [%v]", err)
	}
}

func TestValidateCompleteFrostRetainedGroupHistory_EnforcesMutationLimit(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	policy, err := frostRetainedGroupLiftPolicyFromRuntimeManifest(
		fixture.bindingHash,
		fixture.runtime,
	)
	if err != nil {
		t.Fatal(err)
	}
	history := &FrostRetainedGroupHistory{
		From: FrostPreSignFinality{BlockNumber: 1, BlockHash: [32]byte{1}},
		To:   FrostPreSignFinality{BlockNumber: 2, BlockHash: [32]byte{2}},
		Mutations: make(
			[]FrostRetainedGroupMutation,
			frostRetainedGroupMaximumMutations+1,
		),
	}
	err = validateCompleteFrostRetainedGroupHistory(history, policy)
	if err == nil || !strings.Contains(err.Error(), "mutation limit") {
		t.Fatalf("expected aggregate mutation limit rejection, got [%v]", err)
	}
}

func BenchmarkApplyFrostRetainedGroupMutations_MaximumWalletSet(
	b *testing.B,
) {
	mutations := journalTestBoundedAdmissions(
		frostRetainedGroupMaximumWallets,
	)
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		state := frostRetainedGroupJournalState{
			Schema: frostRetainedGroupJournalStateSchema,
			CurrentPoint: FrostPreSignFinality{
				BlockNumber: 1,
				BlockHash:   [32]byte{1},
			},
			Wallets: []frostRetainedGroupWalletState{},
		}
		if err := applyFrostRetainedGroupMutations(
			&state,
			mutations,
		); err != nil {
			b.Fatal(err)
		}
	}
}

func journalTestBoundedAdmissions(
	count int,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, count)
	operatorIDs := make([]uint32, 51)
	for index := range operatorIDs {
		operatorIDs[index] = uint32((index % 17) + 1)
	}
	for index := range result {
		identifier := uint64(index + 1)
		walletID := [32]byte{0xa1}
		binary.BigEndian.PutUint64(walletID[24:], identifier)
		walletPublicKeyHash := [20]byte{0xa2}
		binary.BigEndian.PutUint64(walletPublicKeyHash[12:], identifier)
		blockNumber := uint64(index + 2)
		blockHash := [32]byte{0xa3}
		binary.BigEndian.PutUint64(blockHash[24:], blockNumber)
		submissionTransaction := [32]byte{0xa4}
		binary.BigEndian.PutUint64(submissionTransaction[24:], identifier)
		admissionTransaction := [32]byte{0xa5}
		binary.BigEndian.PutUint64(admissionTransaction[24:], identifier)
		submission := FrostRetainedGroupEventPoint{
			BlockNumber:      blockNumber,
			BlockHash:        blockHash,
			TransactionHash:  submissionTransaction,
			TransactionIndex: 0,
			LogIndex:         1,
		}
		approval := FrostRetainedGroupEventPoint{
			BlockNumber:      blockNumber,
			BlockHash:        blockHash,
			TransactionHash:  admissionTransaction,
			TransactionIndex: 1,
			LogIndex:         1,
		}
		creation := approval
		creation.LogIndex = 2
		registration := approval
		registration.LogIndex = 3
		retainedGroupHash := [32]byte{0xa6}
		binary.BigEndian.PutUint64(retainedGroupHash[24:], identifier)
		dkgResultHash := [32]byte{0xa7}
		binary.BigEndian.PutUint64(dkgResultHash[24:], identifier)
		result[index] = FrostRetainedGroupMutation{
			Point:                   registration,
			Kind:                    FrostRetainedGroupAdmissionMutation,
			WalletID:                walletID,
			WalletPublicKeyHash:     walletPublicKeyHash,
			OperatorIDs:             append([]uint32{}, operatorIDs...),
			RetainedGroupHash:       retainedGroupHash,
			DkgResultHash:           dkgResultHash,
			DkgSubmissionPoint:      submission,
			DkgApprovalPoint:        approval,
			CreationPoint:           creation,
			BridgeRegistrationPoint: registration,
		}
	}
	return result
}

func lifecycleMutation(
	fixture *journalTestFixture,
	block uint64,
	logIndex uint32,
	kind FrostRetainedGroupMutationKind,
	transactionHash [32]byte,
) FrostRetainedGroupMutation {
	return FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      block,
			BlockHash:        [32]byte{byte(block)},
			TransactionHash:  transactionHash,
			TransactionIndex: 1,
			LogIndex:         logIndex,
		},
		Kind:                kind,
		WalletID:            fixture.walletID,
		WalletPublicKeyHash: fixture.walletPKH,
	}
}

func TestFrostRetainedGroupJournal_RejectsIdentityMismatchAndConcurrentOwner(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	defer journal.close()
	if _, err := newFrostRetainedGroupJournal(
		directory,
		fixture.bindingHash,
		fixture.runtime,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("expected exclusive-lock failure, got [%v]", err)
	}

	otherDirectory := filepath.Join(t.TempDir(), "identity")
	fixture.source.identity.EndpointFingerprint = [32]byte{0xff}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("expected runtime identity mismatch, got [%v]", err)
	}
	if _, err := newFrostRetainedGroupJournal(
		otherDirectory,
		fixture.bindingHash,
		fixture.runtime,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("expected identity mismatch, got [%v]", err)
	}
}

func TestFrostRetainedGroupJournal_RejectsSymlinkEntry(t *testing.T) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), filepath.Join(directory, "evil")); err != nil {
		t.Fatal(err)
	}
	if _, err := newFrostRetainedGroupJournal(
		directory,
		fixture.bindingHash,
		fixture.runtime,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "unsafe entry") {
		t.Fatalf("expected symlink rejection, got [%v]", err)
	}
}

func TestPersistFrostRetainedGroupEnvelopeAt_RestrictsFilesToJournal(
	t *testing.T,
) {
	root := t.TempDir()
	directory := filepath.Join(root, "journal")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "outside.json")
	outsideContents := []byte("must not change")
	if err := os.WriteFile(outsidePath, outsideContents, 0600); err != nil {
		t.Fatal(err)
	}

	invalidNames := []string{
		"../outside.json",
		filepath.Join("nested", frostRetainedGroupJournalStateFile),
		outsidePath,
		".",
		frostRetainedGroupJournalLockFile,
		"batch-1.json",
		"batch-00000000000000000000.json",
		"batch-00000000000000000001.json/../../outside.json",
		frostRetainedGroupJournalStateFile + "\x00",
	}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := persistFrostRetainedGroupEnvelopeAt(
				directory,
				name,
				map[string]uint64{"generation": 1},
				true,
			)
			if err == nil {
				t.Fatalf("expected unsafe journal file name [%q] to be rejected", name)
			}
			actual, readErr := os.ReadFile(outsidePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(actual, outsideContents) {
				t.Fatalf("unsafe journal file name [%q] modified an outside file", name)
			}
		})
	}
	if err := persistFrostRetainedGroupEnvelopeAt(
		".",
		frostRetainedGroupJournalStateFile,
		map[string]uint64{"generation": 1},
		true,
	); err == nil {
		t.Fatal("expected a noncanonical journal directory to be rejected")
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected paths left files in the journal directory: [%v]", entries)
	}
}

func TestPersistFrostRetainedGroupEnvelopeAt_PreservesInternalFiles(
	t *testing.T,
) {
	directory := filepath.Join(t.TempDir(), "journal")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	type payload struct {
		Generation uint64 `json:"generation"`
	}

	tests := []struct {
		name    string
		replace bool
		value   uint64
	}{
		{frostRetainedGroupJournalMetadataFile, false, 1},
		{frostRetainedGroupJournalStateFile, true, 2},
		{frostRetainedGroupBatchFileName(1), false, 3},
	}
	for _, test := range tests {
		if err := persistFrostRetainedGroupEnvelopeAt(
			directory,
			test.name,
			payload{Generation: test.value},
			test.replace,
		); err != nil {
			t.Fatalf("cannot persist legitimate journal file [%s]: [%v]", test.name, err)
		}
		var actual payload
		if err := readFrostRetainedGroupEnvelopeAt(
			directory,
			test.name,
			&actual,
		); err != nil {
			t.Fatalf("cannot read legitimate journal file [%s]: [%v]", test.name, err)
		}
		if actual.Generation != test.value {
			t.Fatalf("unexpected journal file [%s] payload: [%+v]", test.name, actual)
		}
		info, err := os.Lstat(filepath.Join(directory, test.name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("legitimate journal file [%s] is not private and regular", test.name)
		}
	}

	if err := persistFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalMetadataFile,
		payload{Generation: 4},
		false,
	); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected immutable journal replacement to fail, got [%v]", err)
	}
	if err := persistFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalStateFile,
		payload{Generation: 5},
		true,
	); err != nil {
		t.Fatal(err)
	}
	var replaced payload
	if err := readFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalStateFile,
		&replaced,
	); err != nil {
		t.Fatal(err)
	}
	if replaced.Generation != 5 {
		t.Fatalf("replaceable journal state was not updated: [%+v]", replaced)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), frostRetainedGroupJournalTempSuffix) {
			t.Fatalf("successful persistence left temporary file [%s]", entry.Name())
		}
	}
}

func TestFrostRetainedGroupJournal_RejectsCorruptOrPublicStoreFiles(t *testing.T) {
	t.Run("quarantine batch checksum", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations = append(
			fixture.source.mutations,
			FrostRetainedGroupMutation{
				Point: FrostRetainedGroupEventPoint{
					BlockNumber:      5,
					BlockHash:        [32]byte{0x05},
					TransactionHash:  [32]byte{0xa5},
					TransactionIndex: 1,
					LogIndex:         1,
				},
				Kind:         FrostRetainedGroupQuarantineMutation,
				WalletID:     fixture.walletID,
				QuarantineID: [32]byte{0x71},
				EvidenceHash: [32]byte{0x72},
				Reason:       "checksum test",
			},
		)
		directory := filepath.Join(t.TempDir(), "journal")
		journal := fixture.openJournal(t, directory)
		if _, err := journal.reconcile(context.Background(), fixture.target); err != nil {
			t.Fatal(err)
		}
		quarantineBatchPath := filepath.Join(
			journal.quarantineDirectory,
			frostRetainedGroupBatchFileName(1),
		)
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(quarantineBatchPath)
		if err != nil {
			t.Fatal(err)
		}
		envelope := frostRetainedGroupEnvelope{}
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		envelope.Checksum[0] ^= 0xff
		data, err = json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(quarantineBatchPath, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := newFrostRetainedGroupJournal(
			directory,
			fixture.bindingHash,
			fixture.runtime,
			fixture.source,
			fixture.registry,
			fixture.localOperator,
		); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected quarantine checksum failure, got [%v]", err)
		}
	})

	t.Run("canonical metadata permissions", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		directory := filepath.Join(t.TempDir(), "journal")
		journal := fixture.openJournal(t, directory)
		metadataPath := filepath.Join(journal.directory, frostRetainedGroupJournalMetadataFile)
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(metadataPath, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := newFrostRetainedGroupJournal(
			directory,
			fixture.bindingHash,
			fixture.runtime,
			fixture.source,
			fixture.registry,
			fixture.localOperator,
		); err == nil {
			t.Fatal("expected public canonical metadata to be rejected")
		}
	})
}

func TestFrostRetainedGroupJournal_RejectsLocalSessionReconciliationDrift(
	t *testing.T,
) {
	t.Run("missing controlled group", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.registry.walletCache = make(map[string]*walletCacheValue)
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "presence differs") {
			t.Fatalf("expected missing local-session failure, got [%v]", err)
		}
	})

	t.Run("nonmember local group", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations[0].OperatorIDs[6] = 52
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "presence differs") {
			t.Fatalf("expected nonmember local-session failure, got [%v]", err)
		}
	})

	t.Run("terminal group retained locally", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations = append(
			fixture.source.mutations,
			lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosingMutation, [32]byte{0xb3}),
			lifecycleMutation(fixture, 4, 1, FrostRetainedGroupClosedMutation, [32]byte{0xb4}),
			lifecycleMutation(fixture, 4, 2, FrostRetainedGroupRegistryClosureMutation, [32]byte{0xb4}),
		)
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "terminal FROST retained group") {
			t.Fatalf("expected terminal local-session failure, got [%v]", err)
		}
	})

	t.Run("operator ordering mismatch", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		signer := fixture.registry.walletCache["wallet"].signers[0]
		signer.wallet.signingGroupOperators[0], signer.wallet.signingGroupOperators[1] =
			signer.wallet.signingGroupOperators[1], signer.wallet.signingGroupOperators[0]
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "operator ordering differs") {
			t.Fatalf("expected operator-ordering failure, got [%v]", err)
		}
	})
}

func TestFrostRetainedGroupInventoryRoot_IsOrderIndependent(t *testing.T) {
	state := frostRetainedGroupJournalState{
		CurrentPoint: FrostPreSignFinality{
			BlockNumber: 120,
			BlockHash:   repeatedJournalTestBytes32(0x12),
		},
		SnapshotGeneration: 9,
		Wallets: []frostRetainedGroupWalletState{
			journalTestInventoryWallet(0x03, 100, FrostRetainedGroupClosed),
			journalTestInventoryWallet(0x01, 51, FrostRetainedGroupLive),
			journalTestInventoryWallet(0x02, 73, FrostRetainedGroupClosing),
		},
	}
	root, count, minimum, maximum, err := frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	state.Wallets[0], state.Wallets[2] = state.Wallets[2], state.Wallets[0]
	reordered, _, _, _, err := frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	// Fixed vector produced by computeP2TRFrostWalletGroupInventory at runtime
	// commit cb39161d6 for the same three entries and snapshot point.
	expected := [32]byte{
		0x9d, 0xd9, 0xca, 0x84, 0xc5, 0x20, 0x8c, 0x6b,
		0x73, 0x62, 0xe3, 0x72, 0xb7, 0x94, 0xfc, 0x93,
		0xbc, 0x88, 0xc1, 0x61, 0x8e, 0xa1, 0x0f, 0x74,
		0xe4, 0x13, 0x51, 0xeb, 0x9a, 0x54, 0xfb, 0xe3,
	}
	if root != expected || root != reordered || count != 3 || minimum != 51 || maximum != 100 {
		t.Fatalf("unexpected inventory commitment [%x]", root)
	}
}

func repeatedJournalTestBytes32(value byte) [32]byte {
	result := [32]byte{}
	for index := range result {
		result[index] = value
	}
	return result
}

func journalTestInventoryWallet(
	walletByte byte,
	groupSize int,
	lifecycle FrostRetainedGroupLifecycle,
) frostRetainedGroupWalletState {
	retainedGroupHash := repeatedJournalTestBytes32(0xab)
	retainedGroupHash[len(retainedGroupHash)-1] = walletByte
	creation := FrostRetainedGroupEventPoint{
		BlockNumber:      20,
		BlockHash:        repeatedJournalTestBytes32(0x20),
		TransactionHash:  repeatedJournalTestBytes32(0x21),
		TransactionIndex: 1,
		LogIndex:         4,
	}
	registration := creation
	registration.LogIndex = 5
	lifecyclePoint := registration
	if lifecycle != FrostRetainedGroupLive {
		lifecyclePoint = FrostRetainedGroupEventPoint{
			BlockNumber:      80,
			BlockHash:        repeatedJournalTestBytes32(0x80),
			TransactionHash:  repeatedJournalTestBytes32(0x81),
			TransactionIndex: 1,
			LogIndex:         2,
		}
	}
	wallet := frostRetainedGroupWalletState{
		WalletID:                repeatedJournalTestBytes32(walletByte),
		OperatorIDs:             make([]uint32, groupSize),
		RetainedGroupHash:       retainedGroupHash,
		Lifecycle:               lifecycle,
		CreationPoint:           creation,
		BridgeRegistrationPoint: registration,
		LifecyclePoint:          lifecyclePoint,
		LastBridgePoint:         lifecyclePoint,
	}
	if lifecycle.terminal() {
		wallet.RegistryClosed = true
		wallet.RegistryClosurePoint = lifecyclePoint
		wallet.RegistryClosurePoint.LogIndex = 3
	}
	return wallet
}
