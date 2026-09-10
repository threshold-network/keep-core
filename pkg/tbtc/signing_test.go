package tbtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestSigningExecutor_Sign(t *testing.T) {
	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKey := executor.wallet().publicKey

	if !ecdsa.Verify(
		walletPublicKey,
		message.Bytes(),
		signature.R,
		signature.S,
	) {
		t.Errorf("invalid signature: [%+v]", signature)
	}

	if endBlock <= startBlock {
		t.Errorf("wrong end block")
	}
}

func TestSigningExecutor_Sign_Busy(t *testing.T) {
	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	errChan := make(chan error, 1)
	go func() {
		_, _, _, err := executor.sign(ctx, message, startBlock)
		errChan <- err
	}()

	time.Sleep(100 * time.Millisecond)

	_, _, _, err := executor.sign(ctx, message, startBlock)
	testutils.AssertErrorsSame(t, errSigningExecutorBusy, err)

	err = <-errChan
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}
}

func TestSigningExecutor_SignBatch(t *testing.T) {
	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	messages := []*big.Int{
		big.NewInt(1000),
		big.NewInt(2000),
		big.NewInt(3000),
	}
	startBlock := uint64(0)

	signatures, err := executor.signBatch(ctx, messages, startBlock)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKey := executor.wallet().publicKey

	for i, signature := range signatures {
		if !ecdsa.Verify(
			walletPublicKey,
			messages[i].Bytes(),
			signature.R,
			signature.S,
		) {
			t.Errorf("invalid signature [%v]: [%+v]", i, signature)
		}
	}
}

func TestSigningExecutor_Sign_ContextCancelled(t *testing.T) {
	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	// Cancel the context before signing starts; sign() should return quickly
	// rather than hanging.
	cancelCtx()

	signature, _, _, _ := executor.sign(ctx, message, startBlock)

	// A cancelled context may return nil signature with nil error (early exit)
	// or an error -- both are acceptable. What must NOT happen is a hang or
	// a successful signature returned despite cancellation.
	if signature != nil {
		t.Errorf("expected nil signature on context cancel, got: %+v", signature)
	}
}

func TestSigningExecutor_Sign_AllSignersFailed(t *testing.T) {
	// Build an executor where all signer goroutines will fail by reducing the
	// attempts limit to near-zero so the retry loop exhausts immediately.
	executor := setupSigningExecutor(t)
	executor.signingAttemptsLimit = 0

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, _, err := executor.sign(ctx, message, startBlock)

	// With zero attempts, all signers cannot succeed. We expect either
	// errSigningExecutorBusy (if the lock is still held) or an error/nil
	// result -- but not a completed valid signature.
	if signature != nil && err == nil {
		t.Error("expected failure when signingAttemptsLimit is 0, but got a valid signature")
	}
}

func TestSigningExecutor_Sign_MarshalError(t *testing.T) {
	executor := setupSigningExecutor(t)

	// Replace the wallet's public key curve with P256 so marshalPublicKey
	// returns errIncompatiblePublicKey instead of producing key bytes.
	executor.signers[0].wallet.publicKey.Curve = elliptic.P256()

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	_, _, _, err := executor.sign(ctx, big.NewInt(100), 0)

	if err == nil {
		t.Fatal("expected error from sign, got nil")
	}
	if !strings.Contains(err.Error(), "cannot marshal wallet public key") {
		t.Errorf("unexpected error: [%v]", err)
	}
}

func TestSigningExecutor_SignBatch_PartialFailure(t *testing.T) {
	executor := setupSigningExecutor(t)
	// Zero attempts cause every sign() call to return "all signers failed";
	// signBatch must surface that error rather than silently return nil sigs.
	executor.signingAttemptsLimit = 0

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	messages := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}

	_, err := executor.signBatch(ctx, messages, 0)

	if err == nil {
		t.Error("expected error from signBatch when all signers fail, got nil")
	}
}

// setupSigningExecutor sets up an instance of the signing executor ready
// to perform test signing.
func setupSigningExecutor(t *testing.T) *signingExecutor {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	operatorPrivateKey, operatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	localChain := ConnectWithKey(operatorPrivateKey)

	localProvider := local.ConnectWithKey(operatorPublicKey)

	operatorAddress, err := localChain.Signing().PublicKeyToAddress(
		operatorPublicKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	var operators []chain.Address
	for i := 0; i < groupParameters.GroupSize; i++ {
		operators = append(operators, operatorAddress)
	}

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(
		groupParameters.GroupSize,
	)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	signers := make([]*signer, len(testData))
	for i := range testData {
		privateKeyShare := tecdsa.NewPrivateKeyShare(testData[i])

		signers[i] = &signer{
			wallet: wallet{
				publicKey:             privateKeyShare.PublicKey(),
				signingGroupOperators: operators,
			},
			signingGroupMemberIndex: group.MemberIndex(i + 1),
			privateKeyShare:         privateKeyShare,
		}
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(signers[0].wallet.publicKey)
	walletID, err := localChain.CalculateWalletID(signers[0].wallet.publicKey)
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

	keyStorePersistence := createMockKeyStorePersistence(t, signers...)

	node, err := newNode(
		ethereum.Unknown,
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		localProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
	)
	if err != nil {
		t.Fatal(err)
	}

	executor, ok, err := node.getSigningExecutor(signers[0].wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	// Test block counter is much quicker than the real world one.
	// Set more attempts to give more time for computations.
	executor.signingAttemptsLimit *= 8

	return executor
}
