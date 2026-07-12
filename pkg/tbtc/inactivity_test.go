package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestInactivityClaimExecutor_ClaimInactivity(t *testing.T) {
	executor, walletEcdsaID, chain := setupInactivityClaimExecutorScenario(t)

	initialNonce, err := chain.GetInactivityClaimNonce(walletEcdsaID)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	inactiveMembersIndexes := []group.MemberIndex{1, 4}

	err = executor.claimInactivity(
		ctx,
		inactiveMembersIndexes,
		true,
		message,
	)
	if err != nil {
		t.Fatal(err)
	}

	currentNonce, err := chain.GetInactivityClaimNonce(walletEcdsaID)
	if err != nil {
		t.Fatal(err)
	}

	expectedNonceDiff := uint64(1)
	nonceDiff := currentNonce.Uint64() - initialNonce.Uint64()

	testutils.AssertUintsEqual(
		t,
		"inactivity nonce difference",
		expectedNonceDiff,
		nonceDiff,
	)
}

func TestInactivityClaimExecutor_ClaimInactivity_FrostRegistry(t *testing.T) {
	operatorPrivateKey, operatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	baseChain := ConnectWithKey(operatorPrivateKey)
	frostChain := &recordingFrostInactivityChain{localChain: baseChain}
	dualStackChain := &dualStackInactivityChain{
		localChain: baseChain,
		frostChain: frostChain,
	}

	localProvider := local.ConnectWithKey(operatorPublicKey)
	operatorAddress, err := baseChain.Signing().PublicKeyToAddress(operatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])
	walletSigner := &signer{
		wallet: wallet{
			publicKey:             privateKeyShare.PublicKey(),
			signingGroupOperators: []chain.Address{operatorAddress},
		},
		signingGroupMemberIndex: 1,
		privateKeyShare:         privateKeyShare,
	}

	var frostWalletID [32]byte
	walletSigner.wallet.publicKey.X.FillBytes(frostWalletID[:])
	baseChain.setWallet(
		bitcoin.PublicKeyHash(walletSigner.wallet.publicKey),
		&WalletChainData{
			WalletID: frostWalletID,
			State:    StateLive,
		},
	)

	broadcastChannel, err := localProvider.BroadcastChannelFor(
		"test-frost-inactivity",
	)
	if err != nil {
		t.Fatal(err)
	}
	inactivity.RegisterUnmarshallers(broadcastChannel)

	membershipValidator := group.NewMembershipValidator(
		logger,
		walletSigner.wallet.signingGroupOperators,
		baseChain.Signing(),
	)
	if err := broadcastChannel.SetFilter(membershipValidator.IsInGroup); err != nil {
		t.Fatal(err)
	}

	executor := newInactivityClaimExecutor(
		dualStackChain,
		[]*signer{walletSigner},
		broadcastChannel,
		membershipValidator,
		&GroupParameters{
			GroupSize:       1,
			GroupQuorum:     1,
			HonestThreshold: 1,
		},
		generator.NewProtocolLatch(),
		func(context.Context, uint64) error { return nil },
	)

	ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCtx()

	err = executor.claimInactivity(
		ctx,
		[]group.MemberIndex{1},
		true,
		big.NewInt(100),
	)
	if err != nil {
		t.Fatal(err)
	}

	nonce, err := baseChain.GetInactivityClaimNonce(frostWalletID)
	if err != nil {
		t.Fatal(err)
	}
	if nonce.Uint64() != 1 {
		t.Fatalf("expected FROST inactivity nonce 1, got [%v]", nonce)
	}

	dualStackChain.mutex.Lock()
	requestedSchemes := append([]WalletScheme{}, dualStackChain.requestedSchemes...)
	dualStackChain.mutex.Unlock()
	if !reflect.DeepEqual(requestedSchemes, []WalletScheme{WalletSchemeFROST}) {
		t.Fatalf("unexpected inactivity chain requests: [%v]", requestedSchemes)
	}

	frostChain.mutex.Lock()
	defer frostChain.mutex.Unlock()
	if frostChain.operatorIDCalls == 0 ||
		frostChain.hashCalls == 0 ||
		frostChain.subscriptionCalls == 0 ||
		frostChain.assembleCalls == 0 ||
		frostChain.submitCalls == 0 {
		t.Fatalf(
			"FROST backend was not used for every inactivity operation: %+v",
			frostChain,
		)
	}
	if !reflect.DeepEqual(frostChain.submittedGroupMembers, []uint32{777}) {
		t.Fatalf(
			"unexpected FROST group member IDs: [%v]",
			frostChain.submittedGroupMembers,
		)
	}
	for _, walletID := range frostChain.walletIDs {
		if walletID != frostWalletID {
			t.Fatalf(
				"inactivity operation used wallet ID [0x%x], expected [0x%x]",
				walletID,
				frostWalletID,
			)
		}
	}
}

type dualStackInactivityChain struct {
	*localChain
	frostChain *recordingFrostInactivityChain

	mutex            sync.Mutex
	requestedSchemes []WalletScheme
}

type inactivityClaimChainProviderStub struct {
	Chain
	inactivityClaimChain WalletInactivityClaimChain
	err                  error
}

func (iccp *inactivityClaimChainProviderStub) InactivityClaimChainForWallet(
	walletScheme WalletScheme,
) (WalletInactivityClaimChain, error) {
	return iccp.inactivityClaimChain, iccp.err
}

func TestInactivityClaimExecutor_InactivityChainRoutingGuards(t *testing.T) {
	legacyChain := &localChain{}
	executor := &inactivityClaimExecutor{chain: legacyChain}

	selectedChain, err := executor.inactivityChain(WalletSchemeECDSA)
	if err != nil {
		t.Fatal(err)
	}
	if selectedChain != legacyChain {
		t.Fatal("legacy chain fallback returned a different chain")
	}

	_, err = executor.inactivityChain(WalletSchemeFROST)
	if err == nil ||
		err.Error() != "chain does not provide a FROST inactivity claim view" {
		t.Fatalf("unexpected missing FROST view error: [%v]", err)
	}

	executor.chain = &inactivityClaimChainProviderStub{Chain: legacyChain}
	_, err = executor.inactivityChain(WalletSchemeFROST)
	expectedError := fmt.Sprintf(
		"wallet inactivity claim chain is nil for scheme [%v]",
		WalletSchemeFROST,
	)
	if err == nil || err.Error() != expectedError {
		t.Fatalf("unexpected nil FROST view error: [%v]", err)
	}

	providerErr := fmt.Errorf("provider failed")
	executor.chain = &inactivityClaimChainProviderStub{
		Chain: legacyChain,
		err:   providerErr,
	}
	_, err = executor.inactivityChain(WalletSchemeFROST)
	if err != providerErr {
		t.Fatalf("expected provider error, got [%v]", err)
	}
}

func (dsic *dualStackInactivityChain) InactivityClaimChainForWallet(
	walletScheme WalletScheme,
) (WalletInactivityClaimChain, error) {
	dsic.mutex.Lock()
	dsic.requestedSchemes = append(dsic.requestedSchemes, walletScheme)
	dsic.mutex.Unlock()

	switch walletScheme {
	case WalletSchemeECDSA:
		return dsic.localChain, nil
	case WalletSchemeFROST:
		return dsic.frostChain, nil
	default:
		return nil, fmt.Errorf("unsupported wallet scheme [%v]", walletScheme)
	}
}

type recordingFrostInactivityChain struct {
	*localChain

	mutex                 sync.Mutex
	operatorIDCalls       int
	hashCalls             int
	subscriptionCalls     int
	assembleCalls         int
	submitCalls           int
	walletIDs             [][32]byte
	submittedGroupMembers []uint32
}

func (rfic *recordingFrostInactivityChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	rfic.mutex.Lock()
	rfic.operatorIDCalls++
	rfic.mutex.Unlock()

	return 777, nil
}

func (rfic *recordingFrostInactivityChain) CalculateInactivityClaimHash(
	claim *inactivity.ClaimPreimage,
) (inactivity.ClaimHash, error) {
	rfic.mutex.Lock()
	rfic.hashCalls++
	rfic.mutex.Unlock()

	return rfic.localChain.CalculateInactivityClaimHash(claim)
}

func (rfic *recordingFrostInactivityChain) OnInactivityClaimed(
	handler func(event *InactivityClaimedEvent),
) subscription.EventSubscription {
	rfic.mutex.Lock()
	rfic.subscriptionCalls++
	rfic.mutex.Unlock()

	return rfic.localChain.OnInactivityClaimed(handler)
}

func (rfic *recordingFrostInactivityChain) GetInactivityClaimNonce(
	walletID [32]byte,
) (*big.Int, error) {
	rfic.recordWalletID(walletID)
	return rfic.localChain.GetInactivityClaimNonce(walletID)
}

func (rfic *recordingFrostInactivityChain) AssembleInactivityClaim(
	walletID [32]byte,
	inactiveMembersIndices []group.MemberIndex,
	signatures map[group.MemberIndex][]byte,
	heartbeatFailed bool,
) (*InactivityClaim, error) {
	rfic.mutex.Lock()
	rfic.assembleCalls++
	rfic.walletIDs = append(rfic.walletIDs, walletID)
	rfic.mutex.Unlock()

	return rfic.localChain.AssembleInactivityClaim(
		walletID,
		inactiveMembersIndices,
		signatures,
		heartbeatFailed,
	)
}

func (rfic *recordingFrostInactivityChain) SubmitInactivityClaim(
	claim *InactivityClaim,
	nonce *big.Int,
	groupMembers []uint32,
) error {
	rfic.mutex.Lock()
	rfic.submitCalls++
	rfic.walletIDs = append(rfic.walletIDs, claim.WalletID)
	rfic.submittedGroupMembers = append([]uint32{}, groupMembers...)
	rfic.mutex.Unlock()

	return rfic.localChain.SubmitInactivityClaim(claim, nonce, groupMembers)
}

func (rfic *recordingFrostInactivityChain) recordWalletID(walletID [32]byte) {
	rfic.mutex.Lock()
	defer rfic.mutex.Unlock()
	rfic.walletIDs = append(rfic.walletIDs, walletID)
}

func TestInactivityClaimExecutor_ClaimInactivity_Busy(t *testing.T) {
	executor, _, _ := setupInactivityClaimExecutorScenario(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	inactiveMembersIndexes := []group.MemberIndex{1, 4}

	errChan := make(chan error, 1)
	go func() {
		err := executor.claimInactivity(
			ctx,
			inactiveMembersIndexes,
			true,
			message,
		)
		errChan <- err
	}()

	time.Sleep(100 * time.Millisecond)

	err := executor.claimInactivity(
		ctx,
		inactiveMembersIndexes,
		true,
		message,
	)
	testutils.AssertErrorsSame(t, errInactivityClaimExecutorBusy, err)

	err = <-errChan
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}
}

func setupInactivityClaimExecutorScenario(t *testing.T) (
	*inactivityClaimExecutor,
	[32]byte,
	*localChain,
) {
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

	keyStorePersistence := createMockKeyStorePersistence(t, signers...)

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

	executor, ok, err := node.getInactivityClaimExecutor(
		signers[0].wallet.publicKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	return executor, walletID, localChain
}

func TestSignClaim_SigningSuccessful(t *testing.T) {
	chain := Connect()
	inactivityClaimSigner := newInactivityClaimSigner(chain)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	claim := inactivity.NewClaimPreimage(
		big.NewInt(5),
		privateKeyShare.PublicKey(),
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signedClaim, err := inactivityClaimSigner.SignClaim(claim)
	if err != nil {
		t.Fatal(err)
	}

	expectedPublicKey := chain.Signing().PublicKey()
	if !reflect.DeepEqual(
		expectedPublicKey,
		signedClaim.PublicKey,
	) {
		t.Errorf(
			"unexpected public key\n"+
				"expected: %v\n"+
				"actual:   %v\n",
			expectedPublicKey,
			signedClaim.PublicKey,
		)
	}

	expectedInactivityClaimHash := inactivity.ClaimHash(
		sha3.Sum256(
			[]byte(fmt.Sprint(
				claim.Nonce,
				claim.WalletPublicKey,
				claim.InactiveMembersIndexes,
				claim.HeartbeatFailed,
			)),
		),
	)
	if expectedInactivityClaimHash != signedClaim.ClaimHash {
		t.Errorf(
			"unexpected claim hash\n"+
				"expected: %v\n"+
				"actual:   %v\n",
			expectedInactivityClaimHash,
			signedClaim.ClaimHash,
		)
	}

	// Since signature is different on every run (even if the same private key
	// and claim hash are used), simply verify if it's correct
	signatureVerification, err := chain.Signing().Verify(
		signedClaim.ClaimHash[:],
		signedClaim.Signature,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !signatureVerification {
		t.Errorf(
			"Signature [0x%x] was not generated properly for the claim hash "+
				"[0x%x]",
			signedClaim.Signature,
			signedClaim.ClaimHash,
		)
	}
}

func TestSignClaim_ErrorDuringInactivityClaimHashCalculation(t *testing.T) {
	chain := Connect()
	inactivityClaimSigner := newInactivityClaimSigner(chain)

	// Use nil as the claim to cause hash calculation error.
	_, err := inactivityClaimSigner.SignClaim(nil)

	expectedError := fmt.Errorf("claim is nil")
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestVerifySignature_VerifySuccessful(t *testing.T) {
	chain := Connect()
	inactivityClaimSigner := newInactivityClaimSigner(chain)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	claim := inactivity.NewClaimPreimage(
		big.NewInt(5),
		privateKeyShare.PublicKey(),
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signedClaim, err := inactivityClaimSigner.SignClaim(claim)
	if err != nil {
		t.Fatal(err)
	}

	verificationSuccessful, err := inactivityClaimSigner.VerifySignature(
		signedClaim,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !verificationSuccessful {
		t.Fatal(
			"Expected successful verification of signature, but it was " +
				"unsuccessful",
		)
	}
}

func TestVerifySignature_VerifyFailure(t *testing.T) {
	chain := Connect()
	inactivityClaimSigner := newInactivityClaimSigner(chain)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	claim := inactivity.NewClaimPreimage(
		big.NewInt(5),
		privateKeyShare.PublicKey(),
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signedClaim, err := inactivityClaimSigner.SignClaim(claim)
	if err != nil {
		t.Fatal(err)
	}

	anotherClaim := inactivity.NewClaimPreimage(
		big.NewInt(6),
		privateKeyShare.PublicKey(),
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	anotherSignedClaim, err := inactivityClaimSigner.SignClaim(anotherClaim)
	if err != nil {
		t.Fatal(err)
	}

	// Assign signature from another claim to cause a signature verification
	// failure.
	signedClaim.Signature = anotherSignedClaim.Signature

	verificationSuccessful, err := inactivityClaimSigner.VerifySignature(
		signedClaim,
	)
	if err != nil {
		t.Fatal(err)
	}

	if verificationSuccessful {
		t.Fatal(
			"Expected unsuccessful verification of signature, but it was " +
				"successful",
		)
	}
}

func TestVerifySignature_VerifyError(t *testing.T) {
	chain := Connect()
	inactivityClaimSigner := newInactivityClaimSigner(chain)

	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	claim := inactivity.NewClaimPreimage(
		big.NewInt(5),
		privateKeyShare.PublicKey(),
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signedClaim, err := inactivityClaimSigner.SignClaim(claim)
	if err != nil {
		t.Fatal(err)
	}

	// Drop the last byte of the signature to cause an error during signature
	// verification.
	signedClaim.Signature = signedClaim.Signature[:len(signedClaim.Signature)-1]

	_, err = inactivityClaimSigner.VerifySignature(signedClaim)

	expectedError := fmt.Errorf(
		"failed to unmarshal signature: [asn1: syntax error: data truncated]",
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			expectedError,
			err,
		)
	}
}

func TestSubmitClaim_MemberSubmitsClaim(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		testWaitForBlockFn(chain),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	memberIndex := group.MemberIndex(1)

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	err = inactivityClaimSubmitter.SubmitClaim(
		ctx,
		memberIndex,
		claim,
		signatures,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedNonce := big.NewInt(1)

	nonce, err := chain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBigIntsEqual(
		t,
		"inactivity nonce",
		expectedNonce,
		nonce,
	)
}

func TestSubmitClaim_AnotherMemberSubmitsClaim(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		testWaitForBlockFn(chain),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	// Set up a global listener that will cancel the common context upon claim
	// submission. That mimics the real-world scenario.
	chain.OnInactivityClaimed(
		func(event *InactivityClaimedEvent) {
			cancelCtx()
		},
	)

	secondMemberSubmissionChannel := make(chan error)
	// Attempt to submit claim for the second member on a separate goroutine.
	go func() {
		secondMemberIndex := group.MemberIndex(2)
		secondMemberErr := inactivityClaimSubmitter.SubmitClaim(
			ctx,
			secondMemberIndex,
			claim,
			signatures,
		)
		secondMemberSubmissionChannel <- secondMemberErr
	}()

	// This sleep is needed to give enough time for the second member to
	// register their claim submission event handler and act properly on the
	// claim submitted by the first member.
	time.Sleep(1 * time.Second)

	// While the second member is waiting for submission eligibility, submit the
	// claim with the first member.
	firstMemberIndex := group.MemberIndex(1)
	firstMemberErr := inactivityClaimSubmitter.SubmitClaim(
		ctx,
		firstMemberIndex,
		claim,
		signatures,
	)
	if firstMemberErr != nil {
		t.Fatal(firstMemberErr)
	}

	// Check that the second member returned without errors
	secondMemberErr := <-secondMemberSubmissionChannel
	if secondMemberErr != nil {
		t.Fatal(secondMemberErr)
	}

	expectedNonce := big.NewInt(1)

	nonce, err := chain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBigIntsEqual(
		t,
		"inactivity nonce",
		expectedNonce,
		nonce,
	)
}

func TestSubmitClaim_StaleNonceAfterDelayTreatedAsSubmitted(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	firstMemberSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		func(context.Context, uint64) error { return nil },
	)

	var firstMemberSubmitErr error
	secondMemberSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		func(ctx context.Context, _ uint64) error {
			// Simulate another member submitting while this member is delayed.
			firstMemberSubmitErr = firstMemberSubmitter.SubmitClaim(
				ctx,
				group.MemberIndex(1),
				claim,
				signatures,
			)
			return nil
		},
	)

	err = secondMemberSubmitter.SubmitClaim(
		context.Background(),
		group.MemberIndex(2),
		claim,
		signatures,
	)
	if err != nil {
		t.Fatalf("expected stale nonce to be treated as already submitted: %v", err)
	}
	if firstMemberSubmitErr != nil {
		t.Fatalf("first member submission failed: %v", firstMemberSubmitErr)
	}

	expectedNonce := big.NewInt(1)
	nonce, err := chain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBigIntsEqual(
		t,
		"inactivity nonce",
		expectedNonce,
		nonce,
	)
}

func TestSubmitClaim_InvalidResult(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		testWaitForBlockFn(chain),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	memberIndex := group.MemberIndex(1)

	claim := inactivity.NewClaimPreimage(
		big.NewInt(12345), // Use wrong nonce.
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	err = inactivityClaimSubmitter.SubmitClaim(
		ctx,
		memberIndex,
		claim,
		signatures,
	)

	expectedErr := fmt.Errorf("wrong inactivity claim nonce")
	if !reflect.DeepEqual(expectedErr, err) {
		t.Errorf(
			"unexpected error \nexpected: [%v]\nactual:   [%v]\n",
			expectedErr,
			err,
		)
	}
}

func TestSubmitClaim_ContextCancelled(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		testWaitForBlockFn(chain),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())

	// Simulate the case when timeout occurs and the context gets cancelled.
	cancelCtx()

	memberIndex := group.MemberIndex(1)

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	err = inactivityClaimSubmitter.SubmitClaim(
		ctx,
		memberIndex,
		claim,
		signatures,
	)
	if err != nil {
		t.Errorf("unexpected error [%v]", err)
	}

	// Check the inactivity nonce is still 0.
	expectedNonce := big.NewInt(0)

	nonce, err := chain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertBigIntsEqual(
		t,
		"inactivity nonce",
		expectedNonce,
		nonce,
	)
}

func TestSubmitClaim_TooFewSignatures(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	chain := Connect()

	chain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		chain,
		groupParameters,
		groupMembers,
		testWaitForBlockFn(chain),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	memberIndex := group.MemberIndex(1)

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
	}

	err = inactivityClaimSubmitter.SubmitClaim(
		ctx,
		memberIndex,
		claim,
		signatures,
	)

	expectedError := fmt.Errorf(
		"could not submit inactivity claim with [2] signatures for group honest threshold [3]",
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			expectedError,
			err,
		)
	}
}

// TestSubmitClaim_NonceChangesDuringWait is a regression test for the TOCTOU
// race between the initial nonce read and the on-chain claim submission: a
// member that wakes from its index-based delay must re-read the nonce and
// abort if a competing member has already submitted, instead of attempting
// a doomed submission that the chain would reject with "wrong nonce".
func TestSubmitClaim_NonceChangesDuringWait(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}
	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	publicKey := privateKeyShare.PublicKey()
	walletPublicKeyHash := bitcoin.PublicKeyHash(publicKey)
	ecdsaWalletID := [32]byte{1, 2, 3}

	localChain := Connect()

	localChain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			EcdsaWalletID: ecdsaWalletID,
		},
	)

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	groupMembers := []uint32{1, 2, 2, 3, 5}

	signatures := map[group.MemberIndex][]byte{
		1: []byte("signature 1"),
		2: []byte("signature 2"),
		3: []byte("signature 3"),
		4: []byte("signature 4"),
	}

	competitorClaim := &InactivityClaim{
		WalletID: ecdsaWalletID,
	}

	// Simulate a competing member submitting on the first wait invocation.
	// The local chain's SubmitInactivityClaim bumps the nonce, so the
	// post-wait re-check should observe the change and abort.
	var hookFired bool
	hookedWaitForBlockFn := func(ctx context.Context, block uint64) error {
		if !hookFired {
			hookFired = true
			if submitErr := localChain.SubmitInactivityClaim(
				competitorClaim,
				big.NewInt(0),
				groupMembers,
			); submitErr != nil {
				return fmt.Errorf("competitor submission failed: %w", submitErr)
			}
		}
		return testWaitForBlockFn(localChain)(ctx, block)
	}

	inactivityClaimSubmitter := newInactivityClaimSubmitter(
		&testutils.MockLogger{},
		localChain,
		groupParameters,
		groupMembers,
		hookedWaitForBlockFn,
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	claim := inactivity.NewClaimPreimage(
		big.NewInt(0),
		publicKey,
		[]group.MemberIndex{11, 22, 33},
		true,
	)

	// memberIndex=2 produces a non-zero submission delay so the wait fires
	// and the post-wait nonce re-check is exercised.
	err = inactivityClaimSubmitter.SubmitClaim(
		ctx,
		group.MemberIndex(2),
		claim,
		signatures,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error after losing the submission race, got: %v",
			err,
		)
	}

	// The competitor's submission bumped the nonce to 1; our member must not
	// have advanced it further.
	finalNonce, err := localChain.GetInactivityClaimNonce(ecdsaWalletID)
	if err != nil {
		t.Fatal(err)
	}

	expectedNonce := big.NewInt(1)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity nonce",
		expectedNonce,
		finalNonce,
	)
}
