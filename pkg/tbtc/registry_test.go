package tbtc

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/internal/testutils"
)

func TestWalletRegistry_RegisterSigner(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"registered wallets count",
		1,
		len(walletRegistry.walletCache),
	)

	testutils.AssertIntsEqual(
		t,
		"registered wallet signers count",
		1,
		len(walletRegistry.walletCache[walletStorageKey].signers),
	)

	expectedWalletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	testutils.AssertBytesEqual(
		t,
		expectedWalletPublicKeyHash[:],
		walletRegistry.walletCache[walletStorageKey].walletPublicKeyHash[:],
	)

	if !reflect.DeepEqual(signer, walletRegistry.walletCache[walletStorageKey].signers[0]) {
		t.Errorf("registered wallet signer differs from the original one")
	}

	testutils.AssertIntsEqual(
		t,
		"persisted wallet signers count",
		1,
		len(persistenceHandle.saved),
	)
}

func TestWalletRegistry_GetSigners(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	fetchedSigners := walletRegistry.getSigners(signer.wallet.publicKey)

	testutils.AssertIntsEqual(
		t,
		"fetched wallet signers count",
		1,
		len(fetchedSigners),
	)

	if !reflect.DeepEqual(signer, fetchedSigners[0]) {
		t.Errorf("fetched wallet signer differs from the original one")
	}
}

func TestWalletRegistry_getWalletByPublicKeyHash(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)

	wallet, ok := walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !ok {
		t.Error("should return a wallet")
	}

	testutils.AssertStringsEqual(t, "wallet", signer.wallet.String(), wallet.String())
}

func TestWalletRegistry_getWalletByPublicKeyHash_NotFound(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	x, y := tecdsa.Curve.ScalarBaseMult(big.NewInt(100).Bytes())

	walletPublicKeyHash := bitcoin.PublicKeyHash(&ecdsa.PublicKey{
		Curve: tecdsa.Curve,
		X:     x,
		Y:     y,
	})

	_, ok := walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if ok {
		t.Error("should not return a wallet")
	}
}

func TestWalletRegistry_getWalletByID(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	// walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := chain.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}

	wallet, ok := walletRegistry.getWalletByID(walletID)
	if !ok {
		t.Error("should return a wallet")
	}

	testutils.AssertStringsEqual(t, "wallet", signer.wallet.String(), wallet.String())
}

func TestWalletRegistry_getWalletByID_NotFound(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	x, y := tecdsa.Curve.ScalarBaseMult(big.NewInt(100).Bytes())

	walletID, err := chain.CalculateWalletID(&ecdsa.PublicKey{
		Curve: tecdsa.Curve,
		X:     x,
		Y:     y,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, ok := walletRegistry.getWalletByID(walletID)
	if ok {
		t.Error("should not return a wallet")
	}
}

func TestWalletRegistry_PrePopulateWalletCache(t *testing.T) {
	signer := createMockSigner(t)
	signerBytes, err := signer.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)

	persistenceHandle := &mockPersistenceHandle{
		saved: []persistence.DataDescriptor{
			&mockDescriptor{
				name:      "membership_1",
				directory: "wallet_1",
				content:   signerBytes,
			},
		},
	}

	chain := Connect()

	// Cache pre-population happens within newWalletRegistry.
	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"loaded wallets count",
		1,
		len(walletRegistry.walletCache),
	)

	expectedWalletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	testutils.AssertBytesEqual(
		t,
		expectedWalletPublicKeyHash[:],
		walletRegistry.walletCache[walletStorageKey].walletPublicKeyHash[:],
	)

	testutils.AssertIntsEqual(
		t,
		"loaded wallet signers count",
		1,
		len(walletRegistry.walletCache[walletStorageKey].signers),
	)

	assertSignerEquivalent(
		t,
		"pre-populated wallet signer",
		signer,
		walletRegistry.walletCache[walletStorageKey].signers[0],
	)
}

func TestWalletRegistry_GetWalletsPublicKeys(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	keys := walletRegistry.getWalletsPublicKeys()

	testutils.AssertIntsEqual(t, "keys count", 1, len(keys))
	testutils.AssertBoolsEqual(
		t,
		"keys equal",
		true,
		keys[0].Equal(signer.wallet.publicKey),
	)
}

func TestWalletRegistry_ArchiveWallet(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)
	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	err = walletRegistry.archiveWallet(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"registered wallets count",
		0,
		len(walletRegistry.walletCache),
	)

	testutils.AssertIntsEqual(
		t,
		"archived wallets count",
		1,
		len(persistenceHandle.archived),
	)

	testutils.AssertStringsEqual(
		t,
		"archived wallet",
		walletStorageKey,
		persistenceHandle.archived[0],
	)
}

func TestWalletRegistry_ArchiveWallet_NotFound(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	chain := Connect()

	walletRegistry, err := newWalletRegistry(
		persistenceHandle,
		chain.CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}

	signer := createMockSigner(t)

	err = walletRegistry.registerSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	// Public key hash of a wallet that does not exist in the registry.
	anotherWalletPublicKeyHash := [20]byte{1, 1, 2, 2, 3, 3}

	err = walletRegistry.archiveWallet(anotherWalletPublicKeyHash)

	expectedErr := fmt.Errorf("wallet not found in the wallet cache")

	if !reflect.DeepEqual(err, expectedErr) {
		t.Fatalf(
			"unexpected error\nexpected: %v\nactual:   %v",
			expectedErr,
			err,
		)
	}
}

func TestWalletStorage_SaveSigner(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}

	walletStorage := newWalletStorage(persistenceHandle)

	signer := createMockSigner(t)

	err := walletStorage.saveSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"persisted wallet signers count",
		1,
		len(persistenceHandle.saved),
	)
}

func TestWalletStorage_LoadSigners(t *testing.T) {
	signer := createMockSigner(t)
	signerBytes, err := signer.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)

	persistenceHandle := &mockPersistenceHandle{
		saved: []persistence.DataDescriptor{
			&mockDescriptor{
				name:      "membership_1",
				directory: "wallet_1",
				content:   signerBytes,
			},
		},
	}

	walletStorage := newWalletStorage(persistenceHandle)

	signersByWallet := walletStorage.loadSigners()

	testutils.AssertIntsEqual(
		t,
		"loaded wallets count",
		1,
		len(signersByWallet),
	)

	testutils.AssertIntsEqual(
		t,
		"loaded wallet signers count",
		1,
		len(signersByWallet[walletStorageKey]),
	)

	assertSignerEquivalent(
		t,
		"loaded wallet signer",
		signer,
		signersByWallet[walletStorageKey][0],
	)
}

func TestWalletStorage_ArchiveWallet(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}

	walletStorage := newWalletStorage(persistenceHandle)

	signer := createMockSigner(t)

	err := walletStorage.saveSigner(signer)
	if err != nil {
		t.Fatal(err)
	}

	walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)

	err = walletStorage.archiveWallet(walletStorageKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"archived wallets count",
		1,
		len(persistenceHandle.archived),
	)

	testutils.AssertStringsEqual(
		t,
		"archived wallet",
		walletStorageKey,
		persistenceHandle.archived[0],
	)
}

type mockPersistenceHandle struct {
	saved    []persistence.DataDescriptor
	archived []string
}

func (mph *mockPersistenceHandle) Save(
	data []byte,
	directory string,
	name string,
) error {
	mph.saved = append(mph.saved, &mockDescriptor{
		name:      name,
		directory: directory,
		content:   data,
	})

	return nil
}

func (mph *mockPersistenceHandle) Snapshot(
	data []byte,
	directory string,
	name string,
) error {
	panic("not implemented")
}

func (mph *mockPersistenceHandle) ReadAll() (
	<-chan persistence.DataDescriptor,
	<-chan error,
) {
	outputData := make(chan persistence.DataDescriptor, len(mph.saved))
	outputErrors := make(chan error)

	for _, descriptor := range mph.saved {
		outputData <- descriptor
	}

	close(outputData)
	close(outputErrors)

	return outputData, outputErrors
}

func (mph *mockPersistenceHandle) Archive(directory string) error {
	mph.archived = append(mph.archived, directory)
	return nil
}

func (mph *mockPersistenceHandle) Delete(directory string, name string) error {
	panic("not implemented")
}

type mockDescriptor struct {
	name      string
	directory string
	content   []byte
}

func (md *mockDescriptor) Name() string {
	return md.name
}

func (md *mockDescriptor) Directory() string {
	return md.directory
}

func (md *mockDescriptor) Content() ([]byte, error) {
	return md.content, nil
}
