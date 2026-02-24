package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestNode_GetSigningExecutor(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()
	localProvider := local.Connect()

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

	// Populate the mock keystore with the mock signer's data. This is
	// required to make the node controlling the signer's wallet.
	keyStorePersistence := createMockKeyStorePersistence(t, signer)

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

	walletPublicKey := signer.wallet.publicKey
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"cache size",
		0,
		len(node.signingExecutors),
	)

	executor, ok, err := node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	testutils.AssertIntsEqual(
		t,
		"cache size",
		1,
		len(node.signingExecutors),
	)

	testutils.AssertIntsEqual(
		t,
		"signers count",
		1,
		len(executor.signers),
	)

	assertSignerEquivalent(t, "executor signer", signer, executor.signers[0])

	expectedChannel := fmt.Sprintf(
		"%s-%s",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)
	testutils.AssertStringsEqual(
		t,
		"broadcast channel",
		expectedChannel,
		executor.broadcastChannel.Name(),
	)

	_, ok, err = node.getSigningExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	// The executor was already created in the previous call so cached instance
	// should be returned and no new executors should be created.
	testutils.AssertIntsEqual(
		t,
		"cache size",
		1,
		len(node.signingExecutors),
	)

	// Construct an arbitrary public key representing a wallet that is not
	// controlled by the node. We need to make sure the public key's points
	// are on the curve to avoid troubles during processing.
	x, y := walletPublicKey.Curve.Double(walletPublicKey.X, walletPublicKey.Y)
	nonControlledWalletPublicKey := &ecdsa.PublicKey{
		Curve: walletPublicKey.Curve,
		X:     x,
		Y:     y,
	}

	_, ok, err = node.getSigningExecutor(nonControlledWalletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("node is not supposed to control wallet signers")
	}
}

func TestNode_GetCoordinationExecutor(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()
	localProvider := local.Connect()

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

	// Populate the mock keystore with the mock signer's data. This is
	// required to make the node controlling the signer's wallet.
	keyStorePersistence := createMockKeyStorePersistence(t, signer)

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

	walletPublicKey := signer.wallet.publicKey
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"cache size",
		0,
		len(node.coordinationExecutors),
	)

	executor, ok, err := node.getCoordinationExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	testutils.AssertIntsEqual(
		t,
		"cache size",
		1,
		len(node.coordinationExecutors),
	)

	testutils.AssertIntsEqual(
		t,
		"signers count",
		1,
		len(executor.membersIndexes),
	)

	if !reflect.DeepEqual(
		signer.signingGroupMemberIndex,
		executor.membersIndexes[0],
	) {
		t.Errorf("executor holds an unexpected signer")
	}

	expectedChannel := fmt.Sprintf(
		"%s-%s-coordination",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)
	testutils.AssertStringsEqual(
		t,
		"broadcast channel",
		expectedChannel,
		executor.broadcastChannel.Name(),
	)

	_, ok, err = node.getCoordinationExecutor(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("node is supposed to control wallet signers")
	}

	// The executor was already created in the previous call so cached instance
	// should be returned and no new executors should be created.
	testutils.AssertIntsEqual(
		t,
		"cache size",
		1,
		len(node.coordinationExecutors),
	)

	// Construct an arbitrary public key representing a wallet that is not
	// controlled by the node. We need to make sure the public key's points
	// are on the curve to avoid troubles during processing.
	x, y := walletPublicKey.Curve.Double(walletPublicKey.X, walletPublicKey.Y)
	nonControlledWalletPublicKey := &ecdsa.PublicKey{
		Curve: walletPublicKey.Curve,
		X:     x,
		Y:     y,
	}

	_, ok, err = node.getCoordinationExecutor(nonControlledWalletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("node is not supposed to control wallet signers")
	}
}

func TestNode_RunCoordinationLayer(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	blockTime := 1 * time.Millisecond

	localChain := Connect(blockTime)
	localProvider := local.Connect()

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

	// Populate the mock keystore with the mock signer's data. This is
	// required to make the node controlling the signer's wallet.
	keyStorePersistence := createMockKeyStorePersistence(t, signer)

	n, err := newNode(
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

	// Mock the coordination procedure execution. Return predefined results
	// on specific coordination windows.
	executeCoordinationProcedureFn := func(
		_ *node,
		window *coordinationWindow,
		walletPublicKey *ecdsa.PublicKey,
	) (*coordinationResult, bool) {
		if signer.wallet.publicKey.Equal(walletPublicKey) {
			result, ok := map[uint64]*coordinationResult{
				900: {
					proposal: &mockCoordinationProposal{ActionDepositSweep},
				},
				// Omit window at block 1800 to make sure the layer doesn't
				// crash if no result is produced.
				2700: {
					proposal: &mockCoordinationProposal{ActionRedemption},
				},
				// Put some trash value to make sure coordination windows
				// are distributed correctly.
				2705: {
					proposal: &mockCoordinationProposal{ActionMovingFunds},
				},
				3600: {
					proposal: &mockCoordinationProposal{ActionNoop},
				},
				4500: {
					proposal: &mockCoordinationProposal{ActionMovedFundsSweep},
				},
			}[window.coordinationBlock]

			return result, ok
		}

		return nil, false
	}

	// Simply pass processed results to the channel.
	processedResultsChan := make(chan *coordinationResult, 5)
	processCoordinationResultFn := func(
		_ *node,
		result *coordinationResult,
	) {
		processedResultsChan <- result
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	err = n.runCoordinationLayer(
		ctx,
		&coordinationLayerSettings{
			executeCoordinationProcedureFn: executeCoordinationProcedureFn,
			processCoordinationResultFn:    processCoordinationResultFn,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Set up a stop signal that will be triggered after the last coordination
	// window passes.
	waiter, err := localChain.blockCounter.BlockHeightWaiter(5000)
	if err != nil {
		t.Fatal(err)
	}

	var processedResults []*coordinationResult
loop:
	for {
		select {
		case result := <-processedResultsChan:
			processedResults = append(processedResults, result)

			// Once the second-last coordination window is processed, stop the
			// coordination layer. In that case, the last window should not be
			// processed. This allows us to test that the coordination layer's
			// shutdown works as expected.
			if len(processedResults) == 3 {
				cancelCtx()
			}
		case <-waiter:
			break loop
		}
	}

	testutils.AssertIntsEqual(
		t,
		"processed results count",
		3,
		len(processedResults),
	)
	testutils.AssertStringsEqual(
		t,
		"first result",
		ActionDepositSweep.String(),
		processedResults[0].proposal.ActionType().String(),
	)
	testutils.AssertStringsEqual(
		t,
		"second result",
		ActionRedemption.String(),
		processedResults[1].proposal.ActionType().String(),
	)
	testutils.AssertStringsEqual(
		t,
		"third result",
		ActionNoop.String(),
		processedResults[2].proposal.ActionType().String(),
	)
}

type mockCoordinationProposal struct {
	action WalletActionType
}

func (mcp *mockCoordinationProposal) ActionType() WalletActionType {
	return mcp.action
}

func (mcp *mockCoordinationProposal) ValidityBlocks() uint64 {
	panic("unsupported")
}

func (mcp *mockCoordinationProposal) Marshal() ([]byte, error) {
	panic("unsupported")
}

func (mcp *mockCoordinationProposal) Unmarshal(bytes []byte) error {
	panic("unsupported")
}

// createMockSigner creates a mock signer instance that can be used for
// test cases that needs a placeholder signer. The produced signer cannot
// be used to test actual signing scenarios.
func createMockSigner(t *testing.T) *signer {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	privateKeyShare := tecdsa.NewPrivateKeyShare(testData[0])

	signingGroupOperators := []chain.Address{
		"address-1",
		"address-2",
		"address-3",
		"address-3",
		"address-5",
	}

	return &signer{
		wallet: wallet{
			publicKey:             privateKeyShare.PublicKey(),
			signingGroupOperators: signingGroupOperators,
		},
		signingGroupMemberIndex: group.MemberIndex(1),
		privateKeyShare:         privateKeyShare,
		signerMaterial:          privateKeyShare,
	}
}

// createMockKeyStorePersistence creates a mock key store that can be used
// to create test node instances. The key store is populated with the given
// signers.
func createMockKeyStorePersistence(
	t *testing.T,
	signers ...*signer,
) *mockPersistenceHandle {
	walletToSigners := make(map[string][]*signer)
	for _, signer := range signers {
		keyBytes, err := marshalPublicKey(signer.wallet.publicKey)
		if err != nil {
			t.Fatal(err)
		}

		key := hex.EncodeToString(keyBytes)

		walletToSigners[key] = append(walletToSigners[key], signer)
	}

	descriptors := make([]persistence.DataDescriptor, 0)

	for key, signers := range walletToSigners {
		for i, signer := range signers {
			signerBytes, err := signer.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			// Construct the descriptor in the same way as it happens in the
			// real world.
			descriptor := &mockDescriptor{
				name:      fmt.Sprintf("membership_%v", i+1),
				directory: key[2:], // trim the 04 prefix
				content:   signerBytes,
			}

			descriptors = append(descriptors, descriptor)
		}
	}

	return &mockPersistenceHandle{
		saved: descriptors,
	}
}
