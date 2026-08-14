package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/sortition"
)

func TestHeartbeatAction_HappyPath(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyStr := hex.EncodeToString(walletPublicKeyHex)

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	// Set the heartbeat failure counter to `1` for the given wallet. The value
	// of the counter should be reset to `0` after executing the action.
	heartbeatFailureCounter := newHeartbeatFailureCounter()
	heartbeatFailureCounter.increment(walletPublicKeyStr)

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(100000))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	// Use a small, FROST-sized activity threshold to verify the action does not
	// retain the legacy 70-member floor.
	minimumActiveMembers := 3
	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.activeOperatorsCount = uint32(minimumActiveMembers)

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		minimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	err = action.execute()
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertUintsEqual(
		t,
		"heartbeat failure count",
		0,
		uint64(heartbeatFailureCounter.get(walletPublicKeyStr)),
	)
	if mockExecutor.requestedMessage != proposal.Message {
		t.Fatalf(
			"heartbeat signing received message [%x], want raw proposal [%x]",
			mockExecutor.requestedMessage,
			proposal.Message,
		)
	}
	testutils.AssertUintsEqual(
		t,
		"start block",
		startBlock,
		mockExecutor.requestedStartBlock,
	)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity claim executor session ID",
		nil, // executor not called.
		inactivityClaimExecutor.sessionID,
	)
}

func TestHeartbeatAction_OperatorUnstaking(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(0))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	// Set the active operators count to the minimum required value.
	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.activeOperatorsCount = heartbeatSigningMinimumActiveMembers

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		heartbeatSigningMinimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	err = action.execute()
	if err != nil {
		t.Fatal(err)
	}

	if mockExecutor.signCalled {
		t.Fatal("heartbeat signing must not be called for an unstaking operator")
	}
}

func TestHeartbeatAction_LegacyWalletUsesEcdsaStakeInDualStack(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKey := mustUnmarshalPublicKey(t, walletPublicKeyHex)

	baseChain := Connect()
	baseChain.setWallet(
		bitcoin.PublicKeyHash(walletPublicKey),
		&WalletChainData{
			WalletID:      [32]byte{0x01},
			EcdsaWalletID: [32]byte{0x02},
			State:         StateLive,
		},
	)

	ecdsaView := &heartbeatSortitionView{
		localChain:    baseChain,
		eligibleStake: big.NewInt(100),
	}
	frostView := &heartbeatSortitionView{
		localChain:    baseChain,
		eligibleStake: big.NewInt(0),
	}
	dualStackChain := &dualStackHeartbeatChain{
		localChain: baseChain,
		ecdsaView:  ecdsaView,
		frostView:  frostView,
	}

	action := &heartbeatAction{
		chain:           dualStackChain,
		executingWallet: wallet{publicKey: walletPublicKey},
	}

	isUnstaking, err := action.isOperatorUnstaking()
	if err != nil {
		t.Fatal(err)
	}
	if isUnstaking {
		t.Fatal("legacy wallet must use its non-zero ECDSA stake")
	}
	if ecdsaView.operatorToStakingProviderCalls != 1 || ecdsaView.eligibleStakeCalls != 1 {
		t.Fatalf("ECDSA stake view was not used: %+v", ecdsaView)
	}
	if frostView.operatorToStakingProviderCalls != 0 || frostView.eligibleStakeCalls != 0 {
		t.Fatalf("FROST stake view must not be used for a legacy wallet: %+v", frostView)
	}
}

func TestHeartbeatAction_FrostWalletUsesFrostStakeInDualStack(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}
	walletPublicKey := mustUnmarshalPublicKey(t, walletPublicKeyHex)

	baseChain := Connect()
	baseChain.setWallet(
		bitcoin.PublicKeyHash(walletPublicKey),
		&WalletChainData{
			WalletID: [32]byte{0x03},
			State:    StateLive,
		},
	)

	ecdsaView := &heartbeatSortitionView{
		localChain:    baseChain,
		eligibleStake: big.NewInt(100),
	}
	frostView := &heartbeatSortitionView{
		localChain:    baseChain,
		eligibleStake: big.NewInt(0),
	}
	dualStackChain := &dualStackHeartbeatChain{
		localChain: baseChain,
		ecdsaView:  ecdsaView,
		frostView:  frostView,
	}

	action := &heartbeatAction{
		chain:           dualStackChain,
		executingWallet: wallet{publicKey: walletPublicKey},
	}

	isUnstaking, err := action.isOperatorUnstaking()
	if err != nil {
		t.Fatal(err)
	}
	if !isUnstaking {
		t.Fatal("FROST wallet must use its zero FROST stake")
	}
	if frostView.operatorToStakingProviderCalls != 1 || frostView.eligibleStakeCalls != 1 {
		t.Fatalf("FROST stake view was not used: %+v", frostView)
	}
	if ecdsaView.operatorToStakingProviderCalls != 0 || ecdsaView.eligibleStakeCalls != 0 {
		t.Fatalf("ECDSA stake view must not be used for a FROST wallet: %+v", ecdsaView)
	}
}

type dualStackHeartbeatChain struct {
	*localChain
	ecdsaView sortition.Chain
	frostView sortition.Chain
}

func (dshc *dualStackHeartbeatChain) LegacyECDSASortitionChain() sortition.Chain {
	return dshc.ecdsaView
}

func (dshc *dualStackHeartbeatChain) FrostSortitionChain() (sortition.Chain, bool) {
	return dshc.frostView, dshc.frostView != nil
}

type heartbeatSortitionView struct {
	*localChain
	eligibleStake *big.Int

	operatorToStakingProviderCalls int
	eligibleStakeCalls             int
}

func (hsv *heartbeatSortitionView) OperatorToStakingProvider() (
	chain.Address,
	bool,
	error,
) {
	hsv.operatorToStakingProviderCalls++
	return stakingProvider, true, nil
}

func (hsv *heartbeatSortitionView) EligibleStake(
	stakingProvider chain.Address,
) (*big.Int, error) {
	hsv.eligibleStakeCalls++
	return new(big.Int).Set(hsv.eligibleStake), nil
}

func TestHeartbeatAction_Failure_SigningError(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyStr := hex.EncodeToString(walletPublicKeyHex)

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(100000))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.shouldFail = true
	mockExecutor.activeOperatorsCount = heartbeatSigningMinimumActiveMembers

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		heartbeatSigningMinimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	// Do not expect the execution to result in an error. Signing error does not
	// mean the procedure failure.
	err = action.execute()

	expectedError := fmt.Errorf("heartbeat signing process errored out: [oofta]")
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: %v\n"+
				"actual:   %v\n",
			expectedError,
			err,
		)
	}

	testutils.AssertUintsEqual(
		t,
		"heartbeat failure count",
		0,
		uint64(heartbeatFailureCounter.get(walletPublicKeyStr)),
	)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity claim executor session ID",
		nil, // executor not called.
		inactivityClaimExecutor.sessionID,
	)
}

func TestHeartbeatAction_Failure_TooFewActiveOperators(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyStr := hex.EncodeToString(walletPublicKeyHex)

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(100000))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	// Use a small, FROST-sized threshold and set the active operators count just
	// below the required number.
	minimumActiveMembers := 4
	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.activeOperatorsCount = uint32(minimumActiveMembers - 1)

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		minimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	// Do not expect the execution to result in an error. Signing error does not
	// mean the procedure failure.
	err = action.execute()
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertUintsEqual(
		t,
		"heartbeat failure count",
		1,
		uint64(heartbeatFailureCounter.get(walletPublicKeyStr)),
	)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity claim executor session ID",
		nil, // executor not called.
		inactivityClaimExecutor.sessionID,
	)
}

func TestHeartbeatAction_Failure_CounterExceeded(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyStr := hex.EncodeToString(walletPublicKeyHex)

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	// sha256(sha256(messageToSign))
	sha256d, err := hex.DecodeString("38d30dacec5083c902952ce99fc0287659ad0b1ca2086827a8e78b0bef2c8bc1")
	if err != nil {
		t.Fatal(err)
	}

	// Set the heartbeat failure counter to `2` so that the next failure will
	// trigger operator inactivity claim execution.
	heartbeatFailureCounter := newHeartbeatFailureCounter()
	heartbeatFailureCounter.increment(walletPublicKeyStr)
	heartbeatFailureCounter.increment(walletPublicKeyStr)

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(100000))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.activeOperatorsCount = heartbeatSigningMinimumActiveMembers - 1

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		heartbeatSigningMinimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	// Do not expect the execution to result in an error. Signing error does not
	// mean the procedure failure.
	err = action.execute()
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertUintsEqual(
		t,
		"heartbeat failure count",
		3,
		uint64(heartbeatFailureCounter.get(walletPublicKeyStr)),
	)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity claim executor session ID",
		new(big.Int).SetBytes(sha256d),
		inactivityClaimExecutor.sessionID,
	)
}

func TestHeartbeatAction_Failure_InactivityExecutionFailure(t *testing.T) {
	walletPublicKeyHex, err := hex.DecodeString(
		"0471e30bca60f6548d7b42582a478ea37ada63b402af7b3ddd57f0c95bb6843175" +
			"aa0d2053a91a050a6797d85c38f2909cb7027f2344a01986aa2f9f8ca7a0c289",
	)
	if err != nil {
		t.Fatal(err)
	}

	walletPublicKeyStr := hex.EncodeToString(walletPublicKeyHex)

	startBlock := uint64(10)
	expiryBlock := startBlock + heartbeatTotalProposalValidityBlocks

	proposal := &HeartbeatProposal{
		Message: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
	}

	// sha256(sha256(messageToSign))
	sha256d, err := hex.DecodeString("38d30dacec5083c902952ce99fc0287659ad0b1ca2086827a8e78b0bef2c8bc1")
	if err != nil {
		t.Fatal(err)
	}

	// Set the heartbeat failure counter to `2` so that the next failure will
	// trigger operator inactivity claim execution.
	heartbeatFailureCounter := newHeartbeatFailureCounter()
	heartbeatFailureCounter.increment(walletPublicKeyStr)
	heartbeatFailureCounter.increment(walletPublicKeyStr)

	hostChain := Connect()
	hostChain.setOperatorsEligibleStake(big.NewInt(100000))
	hostChain.setHeartbeatProposalValidationResult(proposal, true)

	mockExecutor := &mockHeartbeatSigningExecutor{}
	mockExecutor.activeOperatorsCount = heartbeatSigningMinimumActiveMembers - 1

	inactivityClaimExecutor := &mockInactivityClaimExecutor{}
	inactivityClaimExecutor.shouldFail = true

	action := newHeartbeatAction(
		logger,
		hostChain,
		wallet{
			publicKey: mustUnmarshalPublicKey(t, walletPublicKeyHex),
		},
		mockExecutor,
		heartbeatSigningMinimumActiveMembers,
		proposal,
		heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		func(ctx context.Context, blockHeight uint64) error {
			return nil
		},
	)

	err = action.execute()
	if err == nil {
		t.Fatal("expected error to be returned")
	}
	testutils.AssertStringsEqual(
		t,
		"error message",
		"error while notifying about operator inactivity [mock inactivity "+
			"claim executor error]]",
		err.Error(),
	)

	testutils.AssertUintsEqual(
		t,
		"heartbeat failure count",
		3,
		uint64(heartbeatFailureCounter.get(walletPublicKeyStr)),
	)
	testutils.AssertBigIntsEqual(
		t,
		"inactivity claim executor session ID",
		new(big.Int).SetBytes(sha256d),
		inactivityClaimExecutor.sessionID,
	)
}

func TestHeartbeatFailureCounter_Increment(t *testing.T) {
	walletPublicKey := createMockSigner(t).wallet.publicKey
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(t)
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	counterKey := hex.EncodeToString(walletPublicKeyBytes)

	// Check first increment.
	heartbeatFailureCounter.increment(counterKey)
	count := heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		1,
		uint64(count),
	)

	// Check second increment.
	heartbeatFailureCounter.increment(counterKey)
	count = heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		2,
		uint64(count),
	)
}

func TestHeartbeatFailureCounter_Reset(t *testing.T) {
	walletPublicKey := createMockSigner(t).wallet.publicKey
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(t)
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	counterKey := hex.EncodeToString(walletPublicKeyBytes)

	// Check reset works as the first operation.
	heartbeatFailureCounter.reset(counterKey)
	count := heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		0,
		uint64(count),
	)

	// Check reset works after an increment.
	heartbeatFailureCounter.increment(counterKey)
	heartbeatFailureCounter.reset(counterKey)

	count = heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		0,
		uint64(count),
	)
}

func TestHeartbeatFailureCounter_Get(t *testing.T) {
	walletPublicKey := createMockSigner(t).wallet.publicKey
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(t)
	}

	heartbeatFailureCounter := newHeartbeatFailureCounter()

	counterKey := hex.EncodeToString(walletPublicKeyBytes)

	// Check get works as the first operation.
	count := heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		0,
		uint64(count),
	)

	// Check get works after an increment.
	heartbeatFailureCounter.increment(counterKey)
	count = heartbeatFailureCounter.get(counterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		1,
		uint64(count),
	)

	// Construct an arbitrary public key representing a different wallet.
	x, y := walletPublicKey.Curve.Double(walletPublicKey.X, walletPublicKey.Y)
	anotherWalletPublicKey := &ecdsa.PublicKey{
		Curve: walletPublicKey.Curve,
		X:     x,
		Y:     y,
	}
	anotherWalletPublicKeyBytes, err := marshalPublicKey(anotherWalletPublicKey)
	if err != nil {
		t.Fatal(t)
	}
	anotherCounterKey := hex.EncodeToString(anotherWalletPublicKeyBytes)

	// Check get works on another wallet.
	count = heartbeatFailureCounter.get(anotherCounterKey)
	testutils.AssertUintsEqual(
		t,
		"counter value",
		0,
		uint64(count),
	)
}

type mockHeartbeatSigningExecutor struct {
	shouldFail           bool
	activeOperatorsCount uint32

	requestedMessage    [16]byte
	requestedStartBlock uint64
	signCalled          bool
}

func (mhse *mockHeartbeatSigningExecutor) signHeartbeat(
	ctx context.Context,
	message [16]byte,
	startBlock uint64,
) (*frost.Signature, *signingActivityReport, uint64, error) {
	mhse.signCalled = true
	mhse.requestedMessage = message
	mhse.requestedStartBlock = startBlock

	if mhse.shouldFail {
		return nil, nil, 0, fmt.Errorf("oofta")
	}

	activeMembers := make([]group.MemberIndex, 0)
	inactiveMembers := make([]group.MemberIndex, 0)

	for memberIndex := uint32(1); memberIndex <= 100; memberIndex++ {
		if memberIndex <= mhse.activeOperatorsCount {
			activeMembers = append(activeMembers, group.MemberIndex(memberIndex))
		} else {
			inactiveMembers = append(inactiveMembers, group.MemberIndex(memberIndex))
		}
	}

	activityReport := &signingActivityReport{
		activeMembers:   activeMembers,
		inactiveMembers: inactiveMembers,
	}

	return &frost.Signature{}, activityReport, startBlock + 1, nil
}

type mockInactivityClaimExecutor struct {
	shouldFail bool

	sessionID *big.Int
}

func (mice *mockInactivityClaimExecutor) claimInactivity(
	ctx context.Context,
	inactiveMembersIndexes []group.MemberIndex,
	heartbeatFailed bool,
	sessionID *big.Int,
) error {
	mice.sessionID = sessionID

	if mice.shouldFail {
		return fmt.Errorf("mock inactivity claim executor error")
	}

	return nil
}
