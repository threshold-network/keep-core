package tbtc

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
)

func TestNewNode_ConfiguresFrostSigningBackend_NativeUnavailable(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	groupParameters, localChain, netProvider, keyStorePersistence :=
		setupNewNodeSigningBackendTestDependencies(t)

	_, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		netProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{FrostSigningBackend: "native"},
	)
	if err == nil {
		t.Fatal("expected newNode startup error for unavailable native backend")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected newNode startup error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
}

func TestNewNode_ConfiguresFrostSigningBackend_FFIUnavailable(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	groupParameters, localChain, netProvider, keyStorePersistence :=
		setupNewNodeSigningBackendTestDependencies(t)

	_, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		netProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{FrostSigningBackend: "ffi"},
	)
	if err == nil {
		t.Fatal("expected newNode startup error for unavailable ffi backend")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected newNode startup error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
}

func TestNewNode_ConfiguresFrostSigningBackend_NativeRegistered(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := frostsigning.RegisterNativeExecutionAdapter(&noopNativeExecutionAdapter{})
	if err != nil {
		t.Fatalf("unexpected native adapter registration error: [%v]", err)
	}

	groupParameters, localChain, netProvider, keyStorePersistence :=
		setupNewNodeSigningBackendTestDependencies(t)

	node, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		netProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{FrostSigningBackend: "native"},
	)
	if err != nil {
		t.Fatalf("unexpected newNode startup error: [%v]", err)
	}

	if node == nil {
		t.Fatal("expected node instance")
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func TestNewNode_ConfiguresFrostSigningBackend_FFIRegistered(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := frostsigning.RegisterNativeExecutionAdapter(&noopNativeExecutionAdapter{})
	if err != nil {
		t.Fatalf("unexpected native adapter registration error: [%v]", err)
	}

	groupParameters, localChain, netProvider, keyStorePersistence :=
		setupNewNodeSigningBackendTestDependencies(t)

	node, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		netProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{FrostSigningBackend: "ffi"},
	)
	if err != nil {
		t.Fatalf("unexpected newNode startup error: [%v]", err)
	}

	if node == nil {
		t.Fatal("expected node instance")
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func setupNewNodeSigningBackendTestDependencies(
	t *testing.T,
) (
	*GroupParameters,
	Chain,
	net.Provider,
	*mockPersistenceHandle,
) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()
	netProvider := local.Connect()
	signer := createMockSigner(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := localChain.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}

	localChain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: walletID,
			State:         StateLive,
		},
	)

	return groupParameters,
		localChain,
		netProvider,
		createMockKeyStorePersistence(t, signer)
}
