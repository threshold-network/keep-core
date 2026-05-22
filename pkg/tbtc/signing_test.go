package tbtc

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/generator"
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
		new(big.Int).SetBytes(signature.R[:]),
		new(big.Int).SetBytes(signature.S[:]),
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
			new(big.Int).SetBytes(signature.R[:]),
			new(big.Int).SetBytes(signature.S[:]),
		) {
			t.Errorf("invalid signature [%v]: [%+v]", i, signature)
		}
	}
}

// setupSigningExecutor sets up an instance of the signing executor ready
// to perform test signing.
func setupSigningExecutor(t *testing.T) *signingExecutor {
	// Tests in this suite exercise the keep-tbtc signing executor against
	// in-process tECDSA fixtures. Under the `frost_native frost_tbtc_signer`
	// build tags, the signer-material resolver refuses scaffold-era
	// (legacy-wallet-pubkey) material by default; the fixtures here are
	// inherently scaffold-era so the executor needs the operator opt-in to
	// continue running. Production deployments must never set this env var.
	t.Setenv("KEEP_CORE_FROST_TBTC_SIGNER_ACCEPT_SCAFFOLD_KEY_GROUP", "true")

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
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		localProvider,
		keyStorePersistence,
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{},
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
