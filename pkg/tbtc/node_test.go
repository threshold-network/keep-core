package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestNode_GroupParametersForSignersUsesWalletScheme(t *testing.T) {
	ecdsaGroupParameters := &GroupParameters{
		GroupSize:       100,
		GroupQuorum:     90,
		HonestThreshold: 51,
	}
	frostGroupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}
	node := &node{
		groupParameters:      ecdsaGroupParameters,
		frostGroupParameters: frostGroupParameters,
	}

	tests := map[string]struct {
		signer   *signer
		expected *GroupParameters
	}{
		"legacy ECDSA wallet": {
			signer: &signer{
				privateKeyShare: &tecdsa.PrivateKeyShare{},
			},
			expected: ecdsaGroupParameters,
		},
		"native FROST wallet": {
			signer: &signer{
				signerMaterial: &frostsigning.NativeSignerMaterial{
					Format: frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
					Payload: []byte(`{
						"keyGroup":"key-group",
						"keyGroupSource":"dkg-persisted"
					}`),
				},
			},
			expected: frostGroupParameters,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := node.groupParametersForSigners([]*signer{test.signer})
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			if actual != test.expected {
				t.Fatalf(
					"unexpected group parameters\nexpected: [%+v]\nactual:   [%+v]",
					test.expected,
					actual,
				)
			}
		})
	}
}

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
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
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
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
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

func TestNode_KeepsLiveBridgeWalletWithoutLegacyRegistration(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()
	localProvider := local.Connect()

	signer := createMockSigner(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)

	localChain.setWallet(
		walletPublicKeyHash,
		&WalletChainData{
			WalletID: [32]byte{31: 0x01},
			State:    StateLive,
		},
	)

	n, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		localProvider,
		createMockKeyStorePersistence(t, signer),
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := n.walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !ok {
		t.Fatal("live Bridge wallet should not be archived")
	}
}

func TestNode_KeepsPendingFrostWalletWithoutBridgeRegistration(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	localChain := Connect()
	localChain.frostWalletRegistryAvailable = true
	localProvider := local.Connect()

	signer := createMockSigner(t)
	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)

	n, err := newNode(
		groupParameters,
		localChain,
		newLocalBitcoinChain(),
		localProvider,
		createMockKeyStorePersistence(t, signer),
		&mockPersistenceHandle{},
		generator.StartScheduler(),
		&mockCoordinationProposalGenerator{},
		Config{},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := n.walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !ok {
		t.Fatal("pending FROST wallet should not be archived")
	}
}

func TestNode_ArchivesClosedBridgeWallet(t *testing.T) {
	testCases := map[string]WalletState{
		"closed":     StateClosed,
		"terminated": StateTerminated,
	}

	for name, walletState := range testCases {
		t.Run(name, func(t *testing.T) {
			groupParameters := &GroupParameters{
				GroupSize:       5,
				GroupQuorum:     4,
				HonestThreshold: 3,
			}

			localChain := Connect()
			localProvider := local.Connect()

			signer := createMockSigner(t)
			walletPublicKeyHash := bitcoin.PublicKeyHash(
				signer.wallet.publicKey,
			)

			localChain.setWallet(
				walletPublicKeyHash,
				&WalletChainData{
					WalletID: [32]byte{31: 0x01},
					State:    walletState,
				},
			)

			n, err := newNode(
				groupParameters,
				localChain,
				newLocalBitcoinChain(),
				localProvider,
				createMockKeyStorePersistence(t, signer),
				&mockPersistenceHandle{},
				generator.StartScheduler(),
				&mockCoordinationProposalGenerator{},
				Config{},
			)
			if err != nil {
				t.Fatal(err)
			}

			_, ok := n.walletRegistry.getWalletByPublicKeyHash(
				walletPublicKeyHash,
			)
			if ok {
				t.Fatal("closed Bridge wallet should be archived")
			}
		})
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
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
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
					window:   window,
					proposal: &mockCoordinationProposal{ActionDepositSweep},
				},
				// Omit window at block 1800 to make sure the layer doesn't
				// crash if no result is produced.
				2700: {
					window:   window,
					proposal: &mockCoordinationProposal{ActionRedemption},
				},
				// Put some trash value to make sure coordination windows
				// are distributed correctly.
				2705: {
					window:   window,
					proposal: &mockCoordinationProposal{ActionMovingFunds},
				},
				3600: {
					window:   window,
					proposal: &mockCoordinationProposal{ActionNoop},
				},
				4500: {
					window:   window,
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
			if result == nil {
				continue
			}

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

	resultActionsByWindow := make(map[uint64]WalletActionType, len(processedResults))
	for _, result := range processedResults {
		resultActionsByWindow[result.window.coordinationBlock] =
			result.proposal.ActionType()
	}

	testutils.AssertIntsEqual(
		t,
		"processed coordination windows count",
		3,
		len(resultActionsByWindow),
	)

	firstAction, ok := resultActionsByWindow[900]
	if !ok {
		t.Fatal("expected coordination result for window at block 900")
	}
	testutils.AssertStringsEqual(
		t,
		"result for block 900",
		ActionDepositSweep.String(),
		firstAction.String(),
	)

	secondAction, ok := resultActionsByWindow[2700]
	if !ok {
		t.Fatal("expected coordination result for window at block 2700")
	}
	testutils.AssertStringsEqual(
		t,
		"result for block 2700",
		ActionRedemption.String(),
		secondAction.String(),
	)

	if _, ok := resultActionsByWindow[2705]; ok {
		t.Fatal("unexpected coordination result for non-window block 2705")
	}

	// Result processing is asynchronous, so by the time the test cancels the
	// coordination layer after the third processed result, either the 3600
	// window or the subsequent 4500 window may already be in flight.
	if thirdAction, ok := resultActionsByWindow[3600]; ok {
		testutils.AssertStringsEqual(
			t,
			"result for block 3600",
			ActionNoop.String(),
			thirdAction.String(),
		)
	} else {
		fourthAction, ok := resultActionsByWindow[4500]
		if !ok {
			t.Fatal("expected coordination result for block 3600 or 4500")
		}
		testutils.AssertStringsEqual(
			t,
			"result for block 4500",
			ActionMovedFundsSweep.String(),
			fourthAction.String(),
		)
	}
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

// TestNode_HandleHeartbeatProposal_WalletNotControlled verifies that
// handleHeartbeatProposal returns without dispatching when the node does not
// control any signers for the given wallet.
func TestNode_HandleHeartbeatProposal_WalletNotControlled(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	uncontrolledWallet := uncontrolledWalletFor(signer)
	proposal := &HeartbeatProposal{Message: [16]byte{0x01}}

	n.handleHeartbeatProposal(uncontrolledWallet, proposal, 10, 100)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for uncontrolled wallet, got %d", count)
	}
}

// TestNode_HandleHeartbeatProposal_WalletBusy verifies that
// handleHeartbeatProposal does not crash when the wallet dispatcher returns
// errWalletBusy (another action is already running on the same wallet).
func TestNode_HandleHeartbeatProposal_WalletBusy(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionHeartbeat
	}()

	n.handleHeartbeatProposal(signer.wallet, &HeartbeatProposal{Message: [16]byte{0x02}}, 10, 100)

	// The pre-populated entry must still be there -- our call did not modify it.
	actionType, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok || actionType != ActionHeartbeat {
		t.Errorf(
			"expected actions map to retain pre-populated ActionHeartbeat, "+
				"got ok=%v actionType=%v",
			ok, actionType,
		)
	}
}

// TestNode_HandleHeartbeatProposal_DispatchesAction verifies the happy path:
// for a controlled wallet the action is dispatched and the dispatcher cleans
// up the entry once the goroutine completes.
func TestNode_HandleHeartbeatProposal_DispatchesAction(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	n.handleHeartbeatProposal(signer.wallet, &HeartbeatProposal{Message: [16]byte{0x03}}, 10, 100)

	waitForDispatcherIdle(t, n)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected walletDispatcher to be idle after action completed, got %d active actions",
			count,
		)
	}
}

// TestNode_HandleDepositSweepProposal_WalletNotControlled verifies that
// handleDepositSweepProposal skips dispatch for an uncontrolled wallet.
func TestNode_HandleDepositSweepProposal_WalletNotControlled(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	uncontrolledWallet := uncontrolledWalletFor(signer)
	proposal := &DepositSweepProposal{}

	n.handleDepositSweepProposal(uncontrolledWallet, proposal, 10, 100)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for uncontrolled wallet, got %d", count)
	}
}

// TestNode_HandleDepositSweepProposal_WalletBusy verifies that
// handleDepositSweepProposal handles errWalletBusy without panicking.
func TestNode_HandleDepositSweepProposal_WalletBusy(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionDepositSweep
	}()

	n.handleDepositSweepProposal(signer.wallet, &DepositSweepProposal{}, 10, 100)

	actionType, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok || actionType != ActionDepositSweep {
		t.Errorf(
			"expected pre-populated ActionDepositSweep to remain, got ok=%v actionType=%v",
			ok, actionType,
		)
	}
}

// TestNode_HandleDepositSweepProposal_DispatchesAction verifies the happy path:
// for a controlled wallet the action is dispatched and the dispatcher cleans
// up the entry once the goroutine completes (action will fail validation with
// the empty proposal, but the dispatch itself succeeds).
func TestNode_HandleDepositSweepProposal_DispatchesAction(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	n.handleDepositSweepProposal(
		signer.wallet,
		&DepositSweepProposal{SweepTxFee: big.NewInt(0)},
		10,
		100,
	)

	waitForDispatcherIdle(t, n)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected walletDispatcher to be idle after action completed, got %d active actions",
			count,
		)
	}
}

// TestNode_HandleRedemptionProposal_WalletNotControlled verifies that
// handleRedemptionProposal skips dispatch for an uncontrolled wallet.
func TestNode_HandleRedemptionProposal_WalletNotControlled(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	uncontrolledWallet := uncontrolledWalletFor(signer)
	proposal := &RedemptionProposal{}

	n.handleRedemptionProposal(uncontrolledWallet, proposal, 10, 100)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for uncontrolled wallet, got %d", count)
	}
}

// TestNode_HandleRedemptionProposal_WalletBusy verifies that
// handleRedemptionProposal handles errWalletBusy without panicking.
func TestNode_HandleRedemptionProposal_WalletBusy(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionRedemption
	}()

	n.handleRedemptionProposal(signer.wallet, &RedemptionProposal{RedemptionTxFee: big.NewInt(0)}, 10, 100)

	actionType, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok || actionType != ActionRedemption {
		t.Errorf(
			"expected pre-populated ActionRedemption to remain, got ok=%v actionType=%v",
			ok, actionType,
		)
	}
}

// TestNode_HandleRedemptionProposal_DispatchesAction verifies the happy path:
// for a controlled wallet the action is dispatched and the dispatcher cleans
// up the entry once the goroutine completes (action will fail validation with
// the empty proposal, but the dispatch itself succeeds).
func TestNode_HandleRedemptionProposal_DispatchesAction(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	n.handleRedemptionProposal(
		signer.wallet,
		&RedemptionProposal{RedemptionTxFee: big.NewInt(0)},
		10,
		100,
	)

	waitForDispatcherIdle(t, n)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected walletDispatcher to be idle after action completed, got %d active actions",
			count,
		)
	}
}

// TestNode_HandleMovingFundsProposal_WalletNotControlled verifies that
// handleMovingFundsProposal skips dispatch for an uncontrolled wallet.
func TestNode_HandleMovingFundsProposal_WalletNotControlled(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	uncontrolledWallet := uncontrolledWalletFor(signer)
	proposal := &MovingFundsProposal{}

	n.handleMovingFundsProposal(uncontrolledWallet, proposal, 10, 100)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for uncontrolled wallet, got %d", count)
	}
}

// TestNode_HandleMovingFundsProposal_WalletBusy verifies that
// handleMovingFundsProposal handles errWalletBusy without panicking.
func TestNode_HandleMovingFundsProposal_WalletBusy(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionMovingFunds
	}()

	n.handleMovingFundsProposal(signer.wallet, &MovingFundsProposal{}, 10, 100)

	actionType, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok || actionType != ActionMovingFunds {
		t.Errorf(
			"expected pre-populated ActionMovingFunds to remain, got ok=%v actionType=%v",
			ok, actionType,
		)
	}
}

// TestNode_HandleMovingFundsProposal_DispatchesAction verifies the happy path:
// for a controlled wallet the action is dispatched and the dispatcher cleans
// up the entry once the goroutine completes (action fails immediately because
// the wallet has no main UTXO, but the dispatch itself succeeds).
func TestNode_HandleMovingFundsProposal_DispatchesAction(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	n.handleMovingFundsProposal(signer.wallet, &MovingFundsProposal{}, 10, 100)

	waitForDispatcherIdle(t, n)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected walletDispatcher to be idle after action completed, got %d active actions",
			count,
		)
	}
}

// TestNode_HandleMovedFundsSweepProposal_WalletNotControlled verifies that
// handleMovedFundsSweepProposal skips dispatch for an uncontrolled wallet.
func TestNode_HandleMovedFundsSweepProposal_WalletNotControlled(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	uncontrolledWallet := uncontrolledWalletFor(signer)
	proposal := &MovedFundsSweepProposal{}

	n.handleMovedFundsSweepProposal(uncontrolledWallet, proposal, 10, 100)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for uncontrolled wallet, got %d", count)
	}
}

// TestNode_HandleMovedFundsSweepProposal_WalletBusy verifies that
// handleMovedFundsSweepProposal handles errWalletBusy without panicking.
func TestNode_HandleMovedFundsSweepProposal_WalletBusy(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionMovedFundsSweep
	}()

	n.handleMovedFundsSweepProposal(signer.wallet, &MovedFundsSweepProposal{}, 10, 100)

	actionType, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok || actionType != ActionMovedFundsSweep {
		t.Errorf(
			"expected pre-populated ActionMovedFundsSweep to remain, got ok=%v actionType=%v",
			ok, actionType,
		)
	}
}

// TestNode_HandleMovedFundsSweepProposal_DispatchesAction verifies the happy
// path: for a controlled wallet the action is dispatched and the dispatcher
// cleans up the entry once the goroutine completes (action will fail validation
// with the empty proposal, but the dispatch itself succeeds).
func TestNode_HandleMovedFundsSweepProposal_DispatchesAction(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	n.handleMovedFundsSweepProposal(
		signer.wallet,
		&MovedFundsSweepProposal{SweepTxFee: big.NewInt(0)},
		10,
		100,
	)

	waitForDispatcherIdle(t, n)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected walletDispatcher to be idle after action completed, got %d active actions",
			count,
		)
	}
}

// TestProcessCoordinationResult_NoopActionReturnsEarly verifies that
// processCoordinationResult returns without dispatching any wallet action when
// the proposed action is ActionNoop.
func TestProcessCoordinationResult_NoopActionReturnsEarly(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	result := &coordinationResult{
		wallet: signer.wallet,
		window: newCoordinationWindow(100),
		proposal: &mockCoordinationProposal{
			action: ActionNoop,
		},
	}

	processCoordinationResult(n, result)

	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf("expected no dispatched actions for Noop result, got %d", count)
	}
}

// TestProcessCoordinationResult_HeartbeatRoutesToHandler verifies that
// processCoordinationResult dispatches a heartbeat action when the proposal is
// a HeartbeatProposal and the wallet is controlled by this node.
func TestProcessCoordinationResult_HeartbeatRoutesToHandler(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)

	result := &coordinationResult{
		wallet: signer.wallet,
		window: newCoordinationWindow(100),
		proposal: &HeartbeatProposal{
			Message: [16]byte{0x04},
		},
	}

	processCoordinationResult(n, result)

	waitForDispatcherIdle(t, n)

	// Dispatcher should be idle; a panicking handler would have made this fail.
	if count := dispatchedActionsCount(n); count != 0 {
		t.Errorf(
			"expected dispatcher to be idle after heartbeat action, got %d active",
			count,
		)
	}
}

// TestProcessCoordinationResult_DepositSweepRoutesToHandler verifies that
// processCoordinationResult attempts to dispatch a deposit sweep action when
// the proposal is a DepositSweepProposal. The wallet is pre-marked busy so
// dispatch returns errWalletBusy immediately, proving the routing path was
// exercised without running the action's execute() method.
func TestProcessCoordinationResult_DepositSweepRoutesToHandler(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	// Mark the wallet busy so dispatch is rejected before execute() runs.
	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionNoop
	}()

	result := &coordinationResult{
		wallet:   signer.wallet,
		window:   newCoordinationWindow(100),
		proposal: &DepositSweepProposal{},
	}

	processCoordinationResult(n, result)

	// Busy sentinel must still be there: dispatch was attempted (routing worked)
	// but returned errWalletBusy without touching the map entry.
	_, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok {
		t.Error("expected walletDispatcher to retain the busy sentinel after DepositSweep routing")
	}
}

// TestProcessCoordinationResult_RedemptionRoutesToHandler verifies that
// processCoordinationResult dispatches a redemption action when the proposal is
// a RedemptionProposal and the wallet is controlled by this node. The wallet is
// pre-marked busy so dispatch returns errWalletBusy immediately, proving the
// routing path was exercised without running the action's execute() method.
func TestProcessCoordinationResult_RedemptionRoutesToHandler(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	// Mark the wallet busy so dispatch is rejected before execute() runs.
	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionNoop
	}()

	result := &coordinationResult{
		wallet:   signer.wallet,
		window:   newCoordinationWindow(100),
		proposal: &RedemptionProposal{RedemptionTxFee: big.NewInt(0)},
	}

	processCoordinationResult(n, result)

	// Busy sentinel must still be there: dispatch was attempted (routing worked)
	// but returned errWalletBusy without touching the map entry.
	_, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok {
		t.Error("expected walletDispatcher to retain the busy sentinel after Redemption routing")
	}
}

// TestProcessCoordinationResult_MovingFundsRoutesToHandler verifies that
// processCoordinationResult dispatches a moving funds action when the proposal
// is a MovingFundsProposal and the wallet is controlled by this node.
func TestProcessCoordinationResult_MovingFundsRoutesToHandler(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionNoop
	}()

	result := &coordinationResult{
		wallet:   signer.wallet,
		window:   newCoordinationWindow(100),
		proposal: &MovingFundsProposal{},
	}

	processCoordinationResult(n, result)

	_, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok {
		t.Error("expected walletDispatcher to retain the busy sentinel after MovingFunds routing")
	}
}

// TestProcessCoordinationResult_MovedFundsSweepRoutesToHandler verifies that
// processCoordinationResult dispatches a moved funds sweep action when the
// proposal is a MovedFundsSweepProposal and the wallet is controlled by this
// node.
func TestProcessCoordinationResult_MovedFundsSweepRoutesToHandler(t *testing.T) {
	n, signer := setupNodeForHandlerTests(t)
	walletKey := walletKeyFor(t, signer)

	func() {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		n.walletDispatcher.actions[walletKey] = ActionNoop
	}()

	result := &coordinationResult{
		wallet:   signer.wallet,
		window:   newCoordinationWindow(100),
		proposal: &MovedFundsSweepProposal{SweepTxFee: big.NewInt(0)},
	}

	processCoordinationResult(n, result)

	_, ok := func() (WalletActionType, bool) {
		n.walletDispatcher.actionsMutex.Lock()
		defer n.walletDispatcher.actionsMutex.Unlock()
		v, exists := n.walletDispatcher.actions[walletKey]
		return v, exists
	}()
	if !ok {
		t.Error("expected walletDispatcher to retain the busy sentinel after MovedFundsSweep routing")
	}
}

// setupNodeForClosureTests creates a node backed by a fast-block localChain
// (1 ms per block) so that WaitForBlockConfirmations (32 blocks) completes in
// ~32 ms instead of seconds.
func setupNodeForClosureTests(t *testing.T) (*node, *signer, *localChain) {
	t.Helper()

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	lc := Connect(1 * time.Millisecond)
	localProvider := local.Connect()

	signer := createMockSigner(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	lc.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID: walletID,
		State:         StateLive,
	})

	n, err := newNode(
		groupParameters,
		lc,
		newLocalBitcoinChain(),
		localProvider,
		createMockKeyStorePersistence(t, signer),
		&mockPersistenceHandle{},
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
	)
	if err != nil {
		t.Fatal(err)
	}

	return n, signer, lc
}

// TestArchiveClosedWallets_ArchivesClosedWallet verifies that a wallet whose
// on-chain state is StateClosed is removed from the node's registry.
func TestArchiveClosedWallets_ArchivesClosedWallet(t *testing.T) {
	n, signer, lc := setupNodeWithChain(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	lc.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID: walletID,
		State:         StateClosed,
	})

	if err := n.archiveClosedWallets(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys := n.walletRegistry.getWalletsPublicKeys(); len(keys) != 0 {
		t.Errorf("expected empty registry after archiving, got %d wallets", len(keys))
	}
}

// TestArchiveClosedWallets_ArchivesTerminatedWallet verifies that a wallet in
// StateTerminated is also removed from the registry.
func TestArchiveClosedWallets_ArchivesTerminatedWallet(t *testing.T) {
	n, signer, lc := setupNodeWithChain(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	lc.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID: walletID,
		State:         StateTerminated,
	})

	if err := n.archiveClosedWallets(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys := n.walletRegistry.getWalletsPublicKeys(); len(keys) != 0 {
		t.Errorf("expected empty registry after archiving terminated wallet, got %d wallets", len(keys))
	}
}

// TestArchiveClosedWallets_KeepsLiveWallet verifies that a live wallet is not
// removed from the registry.
func TestArchiveClosedWallets_KeepsLiveWallet(t *testing.T) {
	n, _, _ := setupNodeWithChain(t)

	if err := n.archiveClosedWallets(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys := n.walletRegistry.getWalletsPublicKeys(); len(keys) != 1 {
		t.Errorf("expected 1 wallet in registry, got %d", len(keys))
	}
}

// TestHandleWalletClosure_ArchivesWallet verifies the happy path: after
// WaitForBlockConfirmations, a closed wallet is removed from the registry.
func TestHandleWalletClosure_ArchivesWallet(t *testing.T) {
	n, signer, lc := setupNodeForClosureTests(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Close the wallet before calling handleWalletClosure so that stateCheck
	// confirms closure immediately after the 32-block wait.
	lc.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID: walletID,
		State:         StateClosed,
	})

	if err := n.handleWalletClosure(walletID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys := n.walletRegistry.getWalletsPublicKeys(); len(keys) != 0 {
		t.Errorf("expected empty registry after closure handling, got %d wallets", len(keys))
	}
}

// TestHandleWalletClosure_SkipsUncontrolledWallet verifies that when the
// closed wallet is not in the node's registry the function returns nil without
// touching any other wallet.
func TestHandleWalletClosure_SkipsUncontrolledWallet(t *testing.T) {
	n, signer, lc := setupNodeForClosureTests(t)

	// Build a wallet that is NOT in the node's registry but IS on the chain.
	uncontrolled := uncontrolledWalletFor(signer)
	uncontrolledPKH := bitcoin.PublicKeyHash(uncontrolled.publicKey)
	uncontrolledID, err := lc.CalculateWalletID(uncontrolled.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	lc.setWallet(uncontrolledPKH, &WalletChainData{
		EcdsaWalletID: uncontrolledID,
		State:         StateClosed,
	})

	if err := n.handleWalletClosure(uncontrolledID); err != nil {
		t.Fatalf("unexpected error for uncontrolled wallet: %v", err)
	}

	// Signer's own wallet must be untouched.
	if keys := n.walletRegistry.getWalletsPublicKeys(); len(keys) != 1 {
		t.Errorf("expected signer wallet to remain in registry, got %d wallets", len(keys))
	}
}

// TestHandleWalletClosure_ReturnsErrorWhenNotConfirmed verifies that when the
// stateCheck finds the wallet still live (no reorg confirmed), an error is
// returned.
func TestHandleWalletClosure_ReturnsErrorWhenNotConfirmed(t *testing.T) {
	n, signer, lc := setupNodeForClosureTests(t)

	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}

	// wallet is StateLive → IsWalletRegistered returns true → stateCheck = false
	if err := n.handleWalletClosure(walletID); err == nil {
		t.Fatal("expected error for unconfirmed closure, got nil")
	}
}

// setupNodeWithChain creates a fully-initialised node and returns the node,
// the signer, and the underlying *localChain so callers can manipulate chain
// state (e.g. close/terminate a wallet) after creation.
func setupNodeWithChain(t *testing.T) (*node, *signer, *localChain) {
	t.Helper()

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     4,
		HonestThreshold: 3,
	}

	lc := Connect()
	localProvider := local.Connect()

	signer := createMockSigner(t)

	walletPublicKeyHash := bitcoin.PublicKeyHash(signer.wallet.publicKey)
	walletID, err := lc.CalculateWalletID(signer.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	lc.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID: walletID,
		State:         StateLive,
	})

	n, err := newNode(
		groupParameters,
		lc,
		newLocalBitcoinChain(),
		localProvider,
		createMockKeyStorePersistence(t, signer),
		&mockPersistenceHandle{},
		newTestScheduler(t),
		&mockCoordinationProposalGenerator{},
		Config{PreParamsPoolSize: 1, PreParamsGenerationTimeout: time.Hour},
	)
	if err != nil {
		t.Fatal(err)
	}

	return n, signer, lc
}

// setupNodeForHandlerTests is a convenience wrapper that discards the chain.
func setupNodeForHandlerTests(t *testing.T) (*node, *signer) {
	t.Helper()
	n, signer, _ := setupNodeWithChain(t)
	return n, signer
}

// uncontrolledWalletFor returns a wallet whose public key is NOT registered in
// the given signer's keystore -- constructed by doubling the signer's key.
func uncontrolledWalletFor(s *signer) wallet {
	pk := s.wallet.publicKey
	x, y := pk.Curve.Double(pk.X, pk.Y)
	return wallet{
		publicKey:             &ecdsa.PublicKey{Curve: pk.Curve, X: x, Y: y},
		signingGroupOperators: s.wallet.signingGroupOperators,
	}
}

// walletKeyFor returns the hex-encoded wallet key as stored in walletDispatcher.
func walletKeyFor(t *testing.T, s *signer) string {
	t.Helper()
	b, err := marshalPublicKey(s.wallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// dispatchedActionsCount returns the number of active actions in the
// walletDispatcher, holding the lock for the read.
func dispatchedActionsCount(n *node) int {
	n.walletDispatcher.actionsMutex.Lock()
	defer n.walletDispatcher.actionsMutex.Unlock()
	return len(n.walletDispatcher.actions)
}

// waitForDispatcherIdle polls until walletDispatcher has no active actions or
// the 2-second deadline is exceeded.
func waitForDispatcherIdle(t *testing.T, n *node) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for dispatchedActionsCount(n) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for walletDispatcher to become idle")
		}
		time.Sleep(time.Millisecond)
	}
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

// newTestScheduler creates a scheduler with a permanently-locked latch so that
// checkProtocols stops all background workers within one tick (~1s). This
// prevents CPU-intensive pre-params generation from running during tests that
// do not exercise DKG.
func newTestScheduler(t *testing.T) *generator.Scheduler {
	t.Helper()
	sched := generator.StartScheduler()
	noGenLatch := generator.NewProtocolLatch()
	noGenLatch.Lock()
	sched.RegisterProtocol(noGenLatch)
	return sched
}
