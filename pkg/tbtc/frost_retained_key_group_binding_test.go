package tbtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	frostBindingGeneratorX = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	frostBindingEvenKey    = "02" + frostBindingGeneratorX
	frostBindingOddKey     = "03" + frostBindingGeneratorX
)

func frostBindingWalletID(t *testing.T) [32]byte {
	t.Helper()
	decoded, err := hex.DecodeString(frostBindingGeneratorX)
	if err != nil {
		t.Fatal(err)
	}
	var result [32]byte
	copy(result[:], decoded)
	return result
}

func frostBindingSignerMaterial(t *testing.T, keyGroup string) *frostsigning.NativeSignerMaterial {
	t.Helper()
	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:         keyGroup,
		TaprootOutputKey: frostBindingGeneratorX,
		KeyGroupSource:   frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &frostsigning.NativeSignerMaterial{
		Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}
}

func frostBindingWalletPublicKey() *ecdsa.PublicKey {
	x, y := elliptic.P256().ScalarBaseMult([]byte{1})
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
}

func TestWalletRegistryArchiveFrostWalletPersistsExactKeyGroupBinding(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	publicKey := frostBindingWalletPublicKey()
	publicKeyHash := [20]byte{0x41}
	persistenceHandle := &mockPersistenceHandle{}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			getWalletStorageKey(publicKey): {
				walletID:            walletID,
				walletPublicKeyHash: publicKeyHash,
				signers: []*signer{{
					wallet:         wallet{publicKey: publicKey},
					signerMaterial: frostBindingSignerMaterial(t, frostBindingEvenKey),
				}},
			},
		},
		walletStorage:          newWalletStorage(persistenceHandle),
		retainedFrostKeyGroups: make(map[[32]byte]string),
	}

	if err := registry.archiveWallet(publicKeyHash); err != nil {
		t.Fatal(err)
	}
	if registry.retainedFrostKeyGroups[walletID] != frostBindingEvenKey {
		t.Fatal("registry did not retain the exact FROST key-group handle")
	}
	if len(registry.walletCache) != 0 || len(persistenceHandle.archived) != 1 {
		t.Fatalf("wallet was not archived: cache=%d archive=%v", len(registry.walletCache), persistenceHandle.archived)
	}
	loaded, err := registry.walletStorage.loadRetainedFrostKeyGroupBindings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[walletID] != frostBindingEvenKey {
		t.Fatalf("durable key-group binding mismatch: [%v]", loaded)
	}
}

func TestWalletRegistryArchiveFrostWalletRejectsExactParityConflict(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	publicKey := frostBindingWalletPublicKey()
	publicKeyHash := [20]byte{0x42}
	persistenceHandle := &mockPersistenceHandle{}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			getWalletStorageKey(publicKey): {
				walletID:            walletID,
				walletPublicKeyHash: publicKeyHash,
				signers: []*signer{{
					wallet:         wallet{publicKey: publicKey},
					signerMaterial: frostBindingSignerMaterial(t, frostBindingEvenKey),
				}},
			},
		},
		walletStorage: newWalletStorage(persistenceHandle),
		retainedFrostKeyGroups: map[[32]byte]string{
			walletID: frostBindingOddKey,
		},
	}

	err := registry.archiveWallet(publicKeyHash)
	if err == nil || !strings.Contains(err.Error(), "conflicts with durable binding") {
		t.Fatalf("opposite-parity key-group conflict was accepted: [%v]", err)
	}
	if len(persistenceHandle.archived) != 0 || len(registry.walletCache) != 1 {
		t.Fatal("wallet changed after key-group binding conflict")
	}
}

func TestNewWalletRegistryBackfillsFrostKeyGroupBindingBeforeArchive(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	walletSigner := createMockSigner(t)
	walletSigner.signerMaterial =
		frostBindingSignerMaterial(t, frostBindingEvenKey)
	encoded, err := walletSigner.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	persistenceHandle := &mockPersistenceHandle{
		saved: []persistence.DataDescriptor{&mockDescriptor{
			name:      "membership_1",
			directory: getWalletStorageKey(walletSigner.wallet.publicKey),
			content:   encoded,
		}},
	}

	registry, err := newWalletRegistry(
		persistenceHandle,
		func(*ecdsa.PublicKey) ([32]byte, error) {
			return walletID, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if registry.retainedFrostKeyGroups[walletID] != frostBindingEvenKey {
		t.Fatal("startup did not backfill the exact active FROST key group")
	}
	loaded, err := registry.walletStorage.loadRetainedFrostKeyGroupBindings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[walletID] != frostBindingEvenKey {
		t.Fatal("startup FROST key-group backfill was not durable")
	}
}

type failingFrostBindingPersistence struct {
	*mockPersistenceHandle
}

func (ffbp *failingFrostBindingPersistence) Save(
	data []byte,
	directory string,
	name string,
) error {
	if directory == frostRetainedKeyGroupBindingDirectory {
		return fmt.Errorf("injected binding save failure")
	}
	return ffbp.mockPersistenceHandle.Save(data, directory, name)
}

func TestWalletRegistryArchiveFrostWalletBindingFailureLeavesWalletActive(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	publicKey := frostBindingWalletPublicKey()
	publicKeyHash := [20]byte{0x43}
	persistenceHandle := &failingFrostBindingPersistence{
		mockPersistenceHandle: &mockPersistenceHandle{},
	}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			getWalletStorageKey(publicKey): {
				walletID:            walletID,
				walletPublicKeyHash: publicKeyHash,
				signers: []*signer{{
					wallet:         wallet{publicKey: publicKey},
					signerMaterial: frostBindingSignerMaterial(t, frostBindingEvenKey),
				}},
			},
		},
		walletStorage:          newWalletStorage(persistenceHandle),
		retainedFrostKeyGroups: make(map[[32]byte]string),
	}

	err := registry.archiveWallet(publicKeyHash)
	if err == nil || !strings.Contains(err.Error(), "injected binding save failure") {
		t.Fatalf("binding persistence failure was ignored: [%v]", err)
	}
	if len(persistenceHandle.archived) != 0 || len(registry.walletCache) != 1 {
		t.Fatal("wallet was archived after binding persistence failed")
	}
}

func TestRetainedFrostKeyGroupBindingSurvivesRealPersistenceArchiveAndRestart(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	storagePath := t.TempDir()
	handle, err := persistence.NewProtectedDiskHandle(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	calculateWalletID := func(*ecdsa.PublicKey) ([32]byte, error) {
		return walletID, nil
	}
	registry, err := newWalletRegistry(handle, calculateWalletID)
	if err != nil {
		t.Fatal(err)
	}
	walletSigner := createMockSigner(t)
	walletSigner.signerMaterial =
		frostBindingSignerMaterial(t, frostBindingEvenKey)
	if err := registry.registerSigner(walletSigner); err != nil {
		t.Fatal(err)
	}
	if err := registry.archiveWallet(
		bitcoin.PublicKeyHash(walletSigner.wallet.publicKey),
	); err != nil {
		t.Fatal(err)
	}

	reopenedHandle, err := persistence.NewProtectedDiskHandle(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := newWalletRegistry(reopenedHandle, calculateWalletID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.walletCache) != 0 {
		t.Fatal("archived wallet signer reappeared after restart")
	}
	if reopened.retainedFrostKeyGroups[walletID] != frostBindingEvenKey {
		t.Fatal("exact retained key-group binding did not survive restart")
	}
}

func TestVerifyFrostNativeSignerInventoryEntriesRejectsOppositeParityKeyGroup(
	t *testing.T,
) {
	walletID := frostBindingWalletID(t)
	actual := []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
		WalletID:         walletID,
		KeyGroup:         frostBindingOddKey,
		Threshold:        51,
		ParticipantCount: 100,
		KeyPackages: []frostsigning.NativeTBTCSignerRetainedKeyPackage{{
			ParticipantSeat: 1,
		}},
	}}
	expected := []frostNativeSignerInventoryExpectation{{
		WalletID:         walletID,
		KeyGroup:         frostBindingEvenKey,
		Threshold:        51,
		ParticipantCount: 100,
		ParticipantSeats: []uint16{1},
	}}
	err := verifyFrostNativeSignerInventoryEntries(actual, expected)
	if err == nil || !strings.Contains(err.Error(), "exact local signer material") {
		t.Fatalf("opposite-parity native key group was accepted: [%v]", err)
	}

	expected[0].KeyGroup = ""
	err = verifyFrostNativeSignerInventoryEntries(actual, expected)
	if err == nil || !strings.Contains(err.Error(), "binding is empty") {
		t.Fatalf("empty expected native key group was accepted: [%v]", err)
	}
}
