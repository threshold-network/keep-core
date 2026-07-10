package tbtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"strings"
	"testing"
	"time"

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

func TestSigningSessionID_KeyPathFormatStaysWithinSignerLimit(t *testing.T) {
	message, ok := new(big.Int).SetString(
		"ac692bb7fddf3f7e1e050a83cf3ffb6e8e69888ce980281aa39da169525750ef",
		16,
	)
	if !ok {
		t.Fatal("failed to build test message")
	}

	// A full-length persisted FROST key group (66 hex chars) -- the case that would
	// blow the 128-char signer session-id limit if the key-path branch concatenated the
	// 64-hex message + key group instead of hashing.
	keyGroup := strings.Repeat("ab", 33) // 66 hex chars
	sessionID := signingSessionID(message, nil, 25300, 12, keyGroup)

	if len(sessionID) > 128 {
		t.Fatalf("key-path signing session ID exceeds signer limit: [%d] (%s)", len(sessionID), sessionID)
	}
	if !strings.HasPrefix(sessionID, "kp-") {
		t.Fatalf("unexpected key-path signing session ID prefix: [%s]", sessionID)
	}
	if !strings.HasSuffix(sessionID, "-12") {
		t.Fatalf("unexpected key-path signing session ID attempt suffix: [%s]", sessionID)
	}
	// Binds message, attempt number, and key group (start block is not part of the
	// attempt-specific key-path id, matching the pre-key-group behaviour).
	if signingSessionID(message, nil, 25300, 13, keyGroup) == sessionID {
		t.Fatal("key-path signing session ID must bind the attempt number")
	}
	other, _ := new(big.Int).SetString("01", 16)
	if signingSessionID(other, nil, 25300, 12, keyGroup) == sessionID {
		t.Fatal("key-path signing session ID must bind the message")
	}
	if signingSessionID(message, nil, 25300, 12, keyGroup+"cd") == sessionID {
		t.Fatal("key-path signing session ID must bind the key group")
	}
}

func TestSigningSessionID_TaprootFormatStaysWithinSignerLimit(t *testing.T) {
	message, ok := new(big.Int).SetString(
		"ac692bb7fddf3f7e1e050a83cf3ffb6e8e69888ce980281aa39da169525750ef",
		16,
	)
	if !ok {
		t.Fatal("failed to build test message")
	}

	var merkleRoot [32]byte
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	sessionID := signingSessionID(message, &merkleRoot, 25300, 12, "kg-a")

	if len(sessionID) > 128 {
		t.Fatalf("Taproot signing session ID exceeds signer limit: [%d]", len(sessionID))
	}
	if !strings.HasPrefix(sessionID, "tr-") {
		t.Fatalf("unexpected Taproot signing session ID prefix: [%s]", sessionID)
	}
	if !strings.HasSuffix(sessionID, "-12") {
		t.Fatalf("unexpected Taproot signing session ID attempt suffix: [%s]", sessionID)
	}

	changedMerkleRoot := merkleRoot
	changedMerkleRoot[0] ^= 0xff
	if signingSessionID(message, &changedMerkleRoot, 25300, 12, "kg-a") == sessionID {
		t.Fatal("expected Taproot signing session ID to bind the merkle root")
	}

	if signingSessionID(message, &merkleRoot, 25300, 13, "kg-a") == sessionID {
		t.Fatal("expected Taproot signing session ID to bind the attempt number")
	}

	if signingSessionID(message, &merkleRoot, 28900, 12, "kg-a") == sessionID {
		t.Fatal("expected Taproot signing session ID to bind the signing start block")
	}

	// A DIFFERENT wallet key group must yield a DIFFERENT id, so two wallets on one
	// node reusing a member index never collide on the session-handle registry.
	if signingSessionID(message, &merkleRoot, 25300, 12, "kg-b") == sessionID {
		t.Fatal("expected Taproot signing session ID to bind the key group (multi-wallet)")
	}
}

func TestRoastSessionID_StableAndBinds(t *testing.T) {
	message, ok := new(big.Int).SetString(
		"ac692bb7fddf3f7e1e050a83cf3ffb6e8e69888ce980281aa39da169525750ef",
		16,
	)
	if !ok {
		t.Fatal("failed to build test message")
	}

	// Key-path (nil root): the stable id binds message + startBlock + key group, but
	// takes no attempt number (stable across attempts by construction).
	keyPath := roastSessionID(message, nil, 25300, "kg-a")
	if !strings.HasPrefix(keyPath, "roast-") {
		t.Fatalf("roast session id must be namespaced; got [%s]", keyPath)
	}
	// This id is passed to the native signer as the interactive session id, so it must
	// stay within the 128-char limit even with a full-length (66 hex) FROST key group --
	// the key-path branch hashes rather than concatenating for exactly this reason.
	if long := roastSessionID(message, nil, 25300, strings.Repeat("ab", 33)); len(long) > 128 {
		t.Fatalf("key-path roast session id exceeds signer limit: [%d] (%s)", len(long), long)
	}
	if roastSessionID(message, nil, 28900, "kg-a") == keyPath {
		t.Fatal("key-path roast session id must bind the start block")
	}
	other, _ := new(big.Int).SetString("01", 16)
	if roastSessionID(other, nil, 25300, "kg-a") == keyPath {
		t.Fatal("key-path roast session id must bind the message")
	}
	// A DIFFERENT wallet key group must yield a DIFFERENT id, so two wallets on one
	// node reusing a member index never collide on the session-keyed ROAST registries.
	if roastSessionID(message, nil, 25300, "kg-b") == keyPath {
		t.Fatal("key-path roast session id must bind the key group (multi-wallet)")
	}

	// Taproot branch: binds root + startBlock + key group.
	var merkleRoot [32]byte
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}
	taproot := roastSessionID(message, &merkleRoot, 25300, "kg-a")
	if !strings.HasPrefix(taproot, "roast-tr-") {
		t.Fatalf("unexpected taproot roast session id prefix; got [%s]", taproot)
	}
	changed := merkleRoot
	changed[0] ^= 0xff
	if roastSessionID(message, &changed, 25300, "kg-a") == taproot {
		t.Fatal("taproot roast session id must bind the merkle root")
	}
	if roastSessionID(message, &merkleRoot, 28900, "kg-a") == taproot {
		t.Fatal("taproot roast session id must bind the start block")
	}
	if roastSessionID(message, &merkleRoot, 25300, "kg-b") == taproot {
		t.Fatal("taproot roast session id must bind the key group (multi-wallet)")
	}

	// The stable roast id must be disjoint from any attempt-specific
	// signingSessionID so they never share a registry namespace.
	if keyPath == signingSessionID(message, nil, 25300, 12, "kg-a") {
		t.Fatal("roast and signing session ids must not collide")
	}
}

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
