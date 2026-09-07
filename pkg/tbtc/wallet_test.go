package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestParseWalletActionType(t *testing.T) {
	tests := map[string]struct {
		value          uint8
		expectedAction WalletActionType
		expectedErr    error
	}{
		"noop": {
			value:          0,
			expectedAction: ActionNoop,
		},
		"heartbeat": {
			value:          1,
			expectedAction: ActionHeartbeat,
		},
		"deposit sweep": {
			value:          2,
			expectedAction: ActionDepositSweep,
		},
		"redemption": {
			value:          3,
			expectedAction: ActionRedemption,
		},
		"moving funds": {
			value:          4,
			expectedAction: ActionMovingFunds,
		},
		"moved funds sweep": {
			value:          5,
			expectedAction: ActionMovedFundsSweep,
		},
		"unknown": {
			value:       6,
			expectedErr: fmt.Errorf("unknown wallet action type [6]"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			action, err := ParseWalletActionType(test.value)

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					test.expectedErr,
					err,
				)
			}

			if test.expectedAction != action {
				t.Errorf(
					"unexpected action type\nexpected: [%v]\nactual:   [%v]",
					test.expectedAction,
					action,
				)
			}
		})
	}
}

func TestWalletDispatcher_Dispatch(t *testing.T) {
	walletDispatcher := newWalletDispatcher()

	wallet1 := generateWallet(big.NewInt(100))
	wallet2 := generateWallet(big.NewInt(101))

	// Ctx for first actions of both wallets.
	ctxActions1, cancelCtxActions1 := context.WithCancel(context.Background())
	defer cancelCtxActions1()
	// Ctx for second actions of both wallets.
	ctxActions2, cancelCtxActions2 := context.WithCancel(context.Background())
	defer cancelCtxActions2()

	wallet1Action1 := &mockWalletAction{
		executeFn: func() error {
			<-ctxActions1.Done()
			return nil // complete with success
		},
		actionWallet: wallet1,
	}
	wallet1Action2 := &mockWalletAction{
		executeFn: func() error {
			<-ctxActions2.Done()
			return nil // complete with success
		},
		actionWallet: wallet1,
	}
	wallet2Action1 := &mockWalletAction{
		executeFn: func() error {
			<-ctxActions1.Done()
			return fmt.Errorf("unexpected error") // complete with error
		},
		actionWallet: wallet2,
	}
	wallet2Action2 := &mockWalletAction{
		executeFn: func() error {
			<-ctxActions2.Done()
			return nil // complete with success
		},
		actionWallet: wallet2,
	}

	// Dispatch Action 1 for Wallet 1.
	err := walletDispatcher.dispatch(wallet1Action1)
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}

	// Another Action 1 for Wallet 2.
	err = walletDispatcher.dispatch(wallet2Action1)
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}

	// Try to dispatch Action 1 for Wallet 1 again.
	err = walletDispatcher.dispatch(wallet1Action1)
	testutils.AssertErrorsSame(t, errWalletBusy, err)

	// Try to dispatch Action 1 for Wallet 2 again.
	err = walletDispatcher.dispatch(wallet2Action1)
	testutils.AssertErrorsSame(t, errWalletBusy, err)

	// Try to dispatch Action 2 for Wallet 1.
	err = walletDispatcher.dispatch(wallet1Action2)
	testutils.AssertErrorsSame(t, errWalletBusy, err)

	// Try to dispatch Action 2 for Wallet 2.
	err = walletDispatcher.dispatch(wallet2Action2)
	testutils.AssertErrorsSame(t, errWalletBusy, err)

	// Complete dispatched actions.
	cancelCtxActions1()
	<-ctxActions1.Done()

	// Give some time to release the lock.
	time.Sleep(1 * time.Second)

	// Dispatch Action 2 for Wallet 1.
	err = walletDispatcher.dispatch(wallet1Action2)
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}

	// Dispatch Action 2 for Wallet 2.
	err = walletDispatcher.dispatch(wallet2Action2)
	if err != nil {
		t.Errorf("unexpected error: [%v]", err)
	}
}

func TestEnsureWalletSyncedBetweenChains_TransactionWithoutInputs(t *testing.T) {
	walletPublicKeyHash := [20]byte{}
	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}

	malformedTransaction := &bitcoin.Transaction{
		Version: 1,
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: outputScript,
			},
		},
	}

	btcChain := newLocalBitcoinChain()
	if err := btcChain.BroadcastTransaction(malformedTransaction); err != nil {
		t.Fatal(err)
	}

	err = EnsureWalletSyncedBetweenChains(
		walletPublicKeyHash,
		nil,
		Connect(),
		btcChain,
	)
	if err == nil {
		t.Fatal("expected transaction-without-inputs error")
	}
	if !strings.Contains(err.Error(), "has no inputs") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestDetermineWalletMainUtxo(t *testing.T) {
	// In this scenario, we are using e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0
	// as the wallet public key hash. This PKH translates to two testnet addresses:
	// - P2WPKH: https://live.blockcypher.com/btc-testnet/address/tb1qumuaw3exkxdhtut0u85latkqfz4ylgwstkdzsx
	// - P2PKH:  https://live.blockcypher.com/btc-testnet/address/n2aF1Rj6PK26quhGRo8YoRQYjwm37Zjnkb
	// Those addresses contain some testnet transactions that can be used
	// for this scenario.
	walletPublicKeyHashBytes, err := hex.DecodeString("e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0")
	if err != nil {
		t.Fatal(err)
	}
	var walletPublicKeyHash [20]byte
	copy(walletPublicKeyHash[:], walletPublicKeyHashBytes)

	// Take six arbitrary testnet transactions paying the aforementioned
	// P2WPKH or P2PKH address. For the purpose of this scenario, we assume
	// those are the only transactions targeting our wallet public key hash.
	// We use six transactions in order to test the limitations mentioned
	// in the docstring of the DetermineWalletMainUtxo function. The following
	// list is ordered in the blockchain order so the latest transaction is at
	// the end of the list
	serializedTransactions := []string{
		// https://live.blockcypher.com/btc-testnet/tx/3ca4ae3f8ee3b48949192bc7a146c8d9862267816258c85e02a44678364551e1/
		"01000000000101aa485c8a2fd30844d085cedb3a1b48d791a85bd7e8b5891f9c9f5c0f232ca1e90100000000ffffffff03c0900400000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488acc090040000000000160014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0e77207000000000017a9147ac2d9378a1c47e589dfb8095ca95ed2140d2726870247304402201609722b767e15bc3ec578127b33c959983878ddff7748940e293ebedf04aff9022064811500e614639dbf5b59de390197609ac80d167077dd6021b3fa358316cb5e012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000",
		// https://live.blockcypher.com/btc-testnet/tx/f65bc5029251f0042aedb37f90dbb2bfb63a2e81694beef9cae5ec62e954c22e
		"010000000001015a18b556ae4aab57197fa064a67d33c059efe9fd47c7fe71e18806b9aef6cdf80100000000ffffffff03c0900400000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488acc090040000000000160014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d000000000000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d27260247304402202e7e3d5cf7c163cef907ff1c8f2f5f4e655710019991fd0584b1d884a1119a980220214e523780d7d16a40d220d1e61b673706f1a75f32e6f5c5ad82e769eeb3e137012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000",
		// https://live.blockcypher.com/btc-testnet/tx/44863a79ce2b8fec9792403d5048506e50ffa7338191db0e6c30d3d3358ea2f6
		"010000000001015a019e75ab13d8e7296ad0365cc0e58585c5420e374d1248a29798db1ada73400100000000ffffffff04c0900400000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488acc090040000000000160014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0a0860100000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d2726f2122108000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d27260247304402205e20324a9e43c98ccd29d757dd8edc3cbd3efd59ed6335407d44cada7788227a02201cdb84259a0956882c0e2f0171e40fc5ca9a08e705c56d076b9117b8eb0b4ebe012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000",
		// https://live.blockcypher.com/btc-testnet/tx/4c6b33b7c0550e0e536a5d119ac7189d71e1296fcb0c258e0c115356895bc0e6
		"010000000001011c2d4f9383d2607e4e369753d086f2b02d65c272b70856c8110c5d6a8c3e1a920100000000ffffffff04c0900400000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488acc090040000000000160014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d00000000000000000176a0f6d6f6e6579627574746f6e2e636f6d0568656c6c6fb4340400000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d2726024830450221008ec00e510e1a960029bf9ff1b29345b1f2bbaa831d32b9b90f154f75210b925c02201f903e7fad15501efa763053a02ffbace22e67da24509d6f354a9a2eb658cd29012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000",
		// https://live.blockcypher.com/btc-testnet/tx/605edd75ae0b4fa7cfc7aae8f1399119e9d7ecc212e6253156b60d60f4925d44
		"0100000000010225a666beb7380a3fa2a0a8f64a562c7f1749a131bfee26ff61e4cee07cb3dd030100000000ffffffffc9e58780c6c289c25ae1fe293f85a4db4d0af4f305172f2a1868ddd917458bdf0100000000ffffffff03c0900400000000001976a9142cd680318747b720d67bf4246eb7403b476adb3488acc090040000000000160014e6f9d74726b19b75f16fe1e9feaec048aa4fa1d0041d0800000000001600147ac2d9378a1c47e589dfb8095ca95ed2140d27260247304402202a81b6d58977ced45dd7f1e0be1f941e8a30f11ae390d0f6a047c45bab32292e02206e869c12d9c2623640e426673b12a50fc2b161fc5cabacdd2a975446cbb715ef012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e02483045022100e811056a08176d14f4159ec6c97739d223cd8876a1d7b95172dee2fac46c5290022077bfc3a3ecfac4609ce7cc4a329fc73ee4085ab6863203d2b725b7ecf8f9f307012102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e00000000",
		// https://live.blockcypher.com/btc-testnet/tx/4f9affc5b418385d5aa61e23caa0b55156bf0682d5fedf2d905446f3f88aec6c
		"01000000000101a06e1c482f57029480987c07c5aa9da41f419ad4373c01d586f620564feca39d0100000023220020e57edf10136b0434e46bc08c5ac5a1e45f64f778a96f984d0051873c7a8240f2ffffffff02a0860100000000001976a914e6f9d74726b19b75f16fe1e9feaec048aa4fa1d088ac61f3640f0000000017a91486884e6be1525dab5ae0b451bd2c72cee67dcf4187040047304402201d749233580bc759278701147ba4f956c026ea7a7c7820a8dc5a938415c928430220623727886997806031fef81eedfed15f17f710b7b4dc0794469a85efebd54aad014730440220688a9c1afa516ab76d181d3e635c6c1713ab21eb4b05806d34dede41091b21a3022042026d0f3e2f863c15713689dd3ae18fd7e4b237aefa212ec555f00f00a54f8701475221021492848b2f95c74059edfbc2b3892de0fdba85f03d3e4015d4afbbd295631bff2102ee067a0273f2e3ba88d23140a24fdb290f27bbcd0f94117a9c65be3911c5c04e52ae00000000",
	}

	chain := Connect()
	bitcoinChain := newLocalBitcoinChain()

	// Record the transactions in the local Bitcoin chain.
	transactions := make([]*bitcoin.Transaction, len(serializedTransactions))
	for i, serializedTransaction := range serializedTransactions {
		serializedTransactionBytes, err := hex.DecodeString(serializedTransaction)
		if err != nil {
			t.Fatal(err)
		}

		transaction := new(bitcoin.Transaction)
		err = transaction.Deserialize(serializedTransactionBytes)
		if err != nil {
			t.Fatal(err)
		}

		err = bitcoinChain.BroadcastTransaction(transaction)
		if err != nil {
			t.Fatal(err)
		}

		transactions[i] = transaction
	}

	// Helper function allowing to extract an UTXO related with the wallet
	// public key hash from the given transaction.
	walletUtxoFrom := func(
		transaction *bitcoin.Transaction,
	) *bitcoin.UnspentTransactionOutput {
		p2pkh, err := bitcoin.PayToPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		for outputIndex, output := range transaction.Outputs {
			script := output.PublicKeyScript
			if bytes.Equal(script, p2pkh) || bytes.Equal(script, p2wpkh) {
				return &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: transaction.Hash(),
						OutputIndex:     uint32(outputIndex),
					},
					Value: output.Value,
				}
			}
		}

		t.Fatalf("no output related with the wallet")

		return nil
	}

	tests := map[string]struct {
		mainUtxoHash     [32]byte
		expectedMainUtxo *bitcoin.UnspentTransactionOutput
		expectedErr      error
	}{
		"wallet does not have a main UTXO": {
			mainUtxoHash:     [32]byte{},
			expectedMainUtxo: nil,
			expectedErr:      nil,
		},
		"wallet main UTXO comes from the oldest transaction": {
			mainUtxoHash:     chain.ComputeMainUtxoHash(walletUtxoFrom(transactions[0])),
			expectedMainUtxo: walletUtxoFrom(transactions[0]),
		},
		"wallet main UTXO comes from the middle transaction": {
			mainUtxoHash:     chain.ComputeMainUtxoHash(walletUtxoFrom(transactions[1])),
			expectedMainUtxo: walletUtxoFrom(transactions[1]),
		},
		"wallet main UTXO comes from the latest transaction": {
			mainUtxoHash:     chain.ComputeMainUtxoHash(walletUtxoFrom(transactions[5])),
			expectedMainUtxo: walletUtxoFrom(transactions[5]),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			chain.setWallet(walletPublicKeyHash, &WalletChainData{
				// Set only fields relevant for this test scenario.
				MainUtxoHash: test.mainUtxoHash,
			})

			mainUtxo, err := DetermineWalletMainUtxo(
				walletPublicKeyHash,
				chain,
				bitcoinChain,
			)

			if !reflect.DeepEqual(test.expectedMainUtxo, mainUtxo) {
				t.Errorf(
					"unexpected main UTXO\nexpected: %+v\nactual:   %+v\n",
					test.expectedMainUtxo,
					mainUtxo,
				)
			}

			if !reflect.DeepEqual(test.expectedErr, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedErr,
					err,
				)
			}
		})
	}
}

func TestWallet_MembersByOperator(t *testing.T) {
	wallet := &wallet{
		// Set only relevant fields.
		signingGroupOperators: []chain.Address{
			"0x2",
			"0x1",
			"0x3",
			"0x2",
			"0x1",
			"0x6",
			"0x5",
			"0x3",
			"0x4",
			"0x3",
		},
	}

	tests := map[string]struct {
		operator        chain.Address
		expectedMembers []group.MemberIndex
	}{
		"operator 1": {
			operator:        "0x1",
			expectedMembers: []group.MemberIndex{2, 5},
		},
		"operator 2": {
			operator:        "0x2",
			expectedMembers: []group.MemberIndex{1, 4},
		},
		"operator 3": {
			operator:        "0x3",
			expectedMembers: []group.MemberIndex{3, 8, 10},
		},
		"operator 4": {
			operator:        "0x4",
			expectedMembers: []group.MemberIndex{9},
		},
		"operator 5": {
			operator:        "0x5",
			expectedMembers: []group.MemberIndex{7},
		},
		"operator 6": {
			operator:        "0x6",
			expectedMembers: []group.MemberIndex{6},
		},
		"operator 7": {
			operator:        "0x7",
			expectedMembers: []group.MemberIndex{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			members := wallet.membersByOperator(test.operator)

			if !reflect.DeepEqual(test.expectedMembers, members) {
				t.Errorf(
					"unexpected members\nexpected: %+v\nactual:   %+v\n",
					test.expectedMembers,
					members,
				)
			}
		})
	}
}

// buildSignTransactionFixture creates a funded localBitcoinChain, a
// P2WPKH-locked UTXO belonging to walletObj, and a TransactionBuilder
// with that input added and one OP_TRUE output.
func buildSignTransactionFixture(
	t *testing.T,
	walletObj wallet,
) (*localBitcoinChain, *bitcoin.TransactionBuilder) {
	t.Helper()

	btcChain := newLocalBitcoinChain()

	walletPKH := bitcoin.PublicKeyHash(walletObj.publicKey)
	p2wpkhScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPKH)
	if err != nil {
		t.Fatal(err)
	}

	fundingTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 100000, PublicKeyScript: p2wpkhScript},
		},
	}
	if err := btcChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	utxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: 100000,
	}

	txBuilder := bitcoin.NewTransactionBuilder(btcChain)
	if err := txBuilder.AddPublicKeyHashInput(utxo); err != nil {
		t.Fatal(err)
	}
	txBuilder.AddOutput(&bitcoin.TransactionOutput{
		Value:           90000,
		PublicKeyScript: []byte{0x51}, // OP_TRUE
	})

	return btcChain, txBuilder
}

func TestWalletTransactionExecutor_SignTransaction_Success(t *testing.T) {
	// Use deterministic private key 100 on secp256k1.
	privKeyScalar := big.NewInt(100)
	walletObj := generateWallet(privKeyScalar)

	btcChain, txBuilder := buildSignTransactionFixture(t, walletObj)

	// Pre-compute sig hashes to produce valid ECDSA signatures for the mock.
	sigHashes, err := txBuilder.ComputeSignatureHashes()
	if err != nil {
		t.Fatal(err)
	}

	privKey := &ecdsa.PrivateKey{PublicKey: *walletObj.publicKey, D: privKeyScalar}
	sigs := make([]*tecdsa.Signature, len(sigHashes))
	for i, h := range sigHashes {
		r, s, err := ecdsa.Sign(rand.Reader, privKey, h.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		sigs[i] = &tecdsa.Signature{R: r, S: s}
	}

	const startBlock = uint64(0)
	mockExec := newMockWalletSigningExecutor()
	mockExec.setSignatures(sigHashes, startBlock, sigs)

	executor := &walletTransactionExecutor{
		btcChain:        btcChain,
		executingWallet: walletObj,
		signingExecutor: mockExec,
		// Block until context is done so the signing window stays open.
		waitForBlockFn: func(ctx context.Context, _ uint64) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	}

	tx, err := executor.signTransaction(&testutils.MockLogger{}, txBuilder, startBlock, 1000)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tx == nil {
		t.Fatal("expected non-nil signed transaction")
	}
}

func TestWalletTransactionExecutor_SignTransaction_Timeout(t *testing.T) {
	walletObj := generateWallet(big.NewInt(200))
	_, txBuilder := buildSignTransactionFixture(t, walletObj)

	// Mock executor with no pre-set signatures returns "signing error".
	mockExec := newMockWalletSigningExecutor()

	executor := &walletTransactionExecutor{
		btcChain:        newLocalBitcoinChain(),
		executingWallet: walletObj,
		signingExecutor: mockExec,
		// Return immediately -- simulates the timeout block being reached,
		// which cancels the signing context.
		waitForBlockFn: func(_ context.Context, _ uint64) error { return nil },
	}

	_, err := executor.signTransaction(&testutils.MockLogger{}, txBuilder, 0, 1)

	if err == nil {
		t.Fatal("expected error on signing timeout, got nil")
	}
}

func TestWalletTransactionExecutor_SignTransaction_InsufficientSigners(t *testing.T) {
	walletObj := generateWallet(big.NewInt(300))
	_, txBuilder := buildSignTransactionFixture(t, walletObj)

	// Mock executor that always returns an "insufficient signers" error.
	mockExec := newMockWalletSigningExecutor() // no signatures set -> always errors

	executor := &walletTransactionExecutor{
		btcChain:        newLocalBitcoinChain(),
		executingWallet: walletObj,
		signingExecutor: mockExec,
		waitForBlockFn: func(ctx context.Context, _ uint64) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	}

	_, err := executor.signTransaction(&testutils.MockLogger{}, txBuilder, 0, 1000)

	if err == nil {
		t.Fatal("expected error for insufficient signers, got nil")
	}
}

type mockWalletAction struct {
	executeFn    func() error
	actionWallet wallet
}

func (mwa *mockWalletAction) execute() error {
	return mwa.executeFn()
}

func (mwa *mockWalletAction) wallet() wallet {
	return mwa.actionWallet
}

func (mwa *mockWalletAction) actionType() WalletActionType {
	return ActionNoop
}

func generateWallet(privateKey *big.Int) wallet {
	x, y := tecdsa.Curve.ScalarBaseMult(privateKey.Bytes())
	publicKey := &ecdsa.PublicKey{
		Curve: tecdsa.Curve,
		X:     x,
		Y:     y,
	}

	return wallet{
		publicKey: publicKey,
	}
}

type mockWalletSigningExecutor struct {
	signaturesMutex sync.Mutex
	signatures      map[[32]byte][]*tecdsa.Signature
}

func newMockWalletSigningExecutor() *mockWalletSigningExecutor {
	return &mockWalletSigningExecutor{
		signatures: make(map[[32]byte][]*tecdsa.Signature),
	}
}

func (mwse *mockWalletSigningExecutor) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*tecdsa.Signature, error) {
	mwse.signaturesMutex.Lock()
	defer mwse.signaturesMutex.Unlock()

	key := mwse.buildSignaturesKey(messages, startBlock)

	signatures, ok := mwse.signatures[key]
	if !ok {
		return nil, fmt.Errorf("signing error")
	}

	return signatures, nil
}

func (mwse *mockWalletSigningExecutor) setSignatures(
	messages []*big.Int,
	startBlock uint64,
	signatures []*tecdsa.Signature,
) {
	mwse.signaturesMutex.Lock()
	defer mwse.signaturesMutex.Unlock()

	key := mwse.buildSignaturesKey(messages, startBlock)

	mwse.signatures[key] = signatures
}

func (mwse *mockWalletSigningExecutor) buildSignaturesKey(
	messages []*big.Int,
	startBlock uint64,
) [32]byte {
	var buffer bytes.Buffer
	for _, message := range messages {
		buffer.Write(message.Bytes())
	}

	startBlockBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlockBytes, startBlock)
	buffer.Write(startBlockBytes)

	return sha256.Sum256(buffer.Bytes())
}

// noConfirmBtcChain wraps localBitcoinChain but always fails GetTransactionConfirmations,
// simulating a Bitcoin node that never acknowledges the transaction.
type noConfirmBtcChain struct {
	*localBitcoinChain
}

func (c *noConfirmBtcChain) GetTransactionConfirmations(context.Context, bitcoin.Hash) (uint, error) {
	return 0, fmt.Errorf("rpc unavailable")
}

func TestWalletTransactionExecutor_BroadcastTransaction_Success(t *testing.T) {
	executor := &walletTransactionExecutor{
		btcChain:        newLocalBitcoinChain(),
		executingWallet: generateWallet(big.NewInt(1)),
	}

	tx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 1000, PublicKeyScript: []byte{0x51}},
		},
	}

	err := executor.broadcastTransaction(
		&testutils.MockLogger{},
		tx,
		5*time.Second,
		1*time.Millisecond,
	)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestWalletTransactionExecutor_BroadcastTransaction_Timeout(t *testing.T) {
	executor := &walletTransactionExecutor{
		btcChain:        &noConfirmBtcChain{newLocalBitcoinChain()},
		executingWallet: generateWallet(big.NewInt(1)),
	}

	tx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x02},
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 2000, PublicKeyScript: []byte{0x51}},
		},
	}

	err := executor.broadcastTransaction(
		&testutils.MockLogger{},
		tx,
		50*time.Millisecond,
		5*time.Millisecond,
	)

	expectedMsg := "broadcast timeout exceeded"
	if err == nil || err.Error() != expectedMsg {
		t.Errorf("expected error %q, got: %v", expectedMsg, err)
	}
}

// walletSyncBtcChain is a minimal bitcoin.Chain stub for EnsureWalletSyncedBetweenChains tests.
// Only GetUtxosForPublicKeyHash, GetMempoolUtxosForPublicKeyHash, and GetTransaction
// are meaningful; all other methods panic if called.
type walletSyncBtcChain struct {
	utxos   []*bitcoin.UnspentTransactionOutput
	mempool []*bitcoin.UnspentTransactionOutput
	txs     map[bitcoin.Hash]*bitcoin.Transaction
}

func (c *walletSyncBtcChain) GetUtxosForPublicKeyHash([20]byte) ([]*bitcoin.UnspentTransactionOutput, error) {
	return c.utxos, nil
}

func (c *walletSyncBtcChain) GetMempoolUtxosForPublicKeyHash([20]byte) ([]*bitcoin.UnspentTransactionOutput, error) {
	return c.mempool, nil
}

func (c *walletSyncBtcChain) GetTransaction(hash bitcoin.Hash) (*bitcoin.Transaction, error) {
	if tx, ok := c.txs[hash]; ok {
		return tx, nil
	}
	return nil, fmt.Errorf("tx not found: %s", hash.String())
}

func (c *walletSyncBtcChain) GetTransactionConfirmations(context.Context, bitcoin.Hash) (uint, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) BroadcastTransaction(*bitcoin.Transaction) error {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetLatestBlockHeight() (uint, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetBlockHeader(uint) (*bitcoin.BlockHeader, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetTransactionMerkleProof(bitcoin.Hash, uint) (*bitcoin.TransactionMerkleProof, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetTransactionsForPublicKeyHash([20]byte, int) ([]*bitcoin.Transaction, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetTxHashesForPublicKeyHash([20]byte) ([]bitcoin.Hash, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetMempoolForPublicKeyHash([20]byte) ([]*bitcoin.Transaction, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) EstimateSatPerVByteFee(uint32) (int64, error) {
	panic("unused in wallet sync tests")
}
func (c *walletSyncBtcChain) GetCoinbaseTxHash(uint) (bitcoin.Hash, error) {
	panic("unused in wallet sync tests")
}

func TestEnsureWalletSyncedBetweenChains_FreshWalletNoUtxos(t *testing.T) {
	var walletPKH [20]byte
	walletPKH[0] = 0xaa

	btcChain := &walletSyncBtcChain{
		utxos:   []*bitcoin.UnspentTransactionOutput{},
		mempool: []*bitcoin.UnspentTransactionOutput{},
	}

	err := EnsureWalletSyncedBetweenChains(walletPKH, nil, Connect(), btcChain)

	if err != nil {
		t.Errorf("expected no error for fresh wallet with no UTXOs, got: %v", err)
	}
}

func TestEnsureWalletSyncedBetweenChains_FreshWalletSpamUtxos(t *testing.T) {
	var walletPKH [20]byte
	walletPKH[0] = 0xaa

	// Outputs with OutputIndex != 0 cannot be produced by the wallet as its
	// first transaction and are classified as spam.
	spamUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x01},
			OutputIndex:     1,
		},
		Value: 5000,
	}

	btcChain := &walletSyncBtcChain{
		utxos:   []*bitcoin.UnspentTransactionOutput{spamUtxo},
		mempool: []*bitcoin.UnspentTransactionOutput{},
	}

	err := EnsureWalletSyncedBetweenChains(walletPKH, nil, Connect(), btcChain)

	if err != nil {
		t.Errorf("expected no error when all UTXOs are spam (OutputIndex != 0), got: %v", err)
	}
}

func TestEnsureWalletSyncedBetweenChains_FreshWalletDepositSweepFirstTx(t *testing.T) {
	var walletPKH [20]byte
	walletPKH[0] = 0xaa

	// The deposit that the wallet swept.
	depositFundingTxHash := bitcoin.Hash{0xdd}
	var depositFundingOutputIndex uint32 = 0

	// The sweep transaction spending the deposit UTXO. Its single output
	// lands at index 0, which is the tell-tale sign of a wallet-produced tx.
	sweepTx := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: depositFundingTxHash,
					OutputIndex:     depositFundingOutputIndex,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 9000, PublicKeyScript: []byte{0x51}},
		},
	}

	sweepUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: sweepTx.Hash(),
			OutputIndex:     0,
		},
		Value: 9000,
	}

	localChain := Connect()
	localChain.setDepositRequest(
		depositFundingTxHash,
		depositFundingOutputIndex,
		&DepositChainRequest{},
	)

	btcChain := &walletSyncBtcChain{
		utxos:   []*bitcoin.UnspentTransactionOutput{sweepUtxo},
		mempool: []*bitcoin.UnspentTransactionOutput{},
		txs:     map[bitcoin.Hash]*bitcoin.Transaction{sweepTx.Hash(): sweepTx},
	}

	err := EnsureWalletSyncedBetweenChains(walletPKH, nil, localChain, btcChain)

	if err == nil {
		t.Error("expected error for fresh wallet that produced a deposit sweep, got nil")
	}
}

func TestEnsureWalletSyncedBetweenChains_MainUtxoInSync(t *testing.T) {
	var walletPKH [20]byte
	walletPKH[0] = 0xbb

	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x10},
			OutputIndex:     0,
		},
		Value: 100000,
	}

	// The bitcoin chain still holds the main UTXO — wallet has not spent it.
	btcChain := &walletSyncBtcChain{
		utxos: []*bitcoin.UnspentTransactionOutput{mainUtxo},
	}

	err := EnsureWalletSyncedBetweenChains(walletPKH, mainUtxo, Connect(), btcChain)

	if err != nil {
		t.Errorf("expected no error when main UTXO is still unspent, got: %v", err)
	}
}

func TestEnsureWalletSyncedBetweenChains_MainUtxoSpent(t *testing.T) {
	var walletPKH [20]byte
	walletPKH[0] = 0xcc

	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x20},
			OutputIndex:     0,
		},
		Value: 100000,
	}

	// The main UTXO has been spent; only a new change UTXO remains.
	changeUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: bitcoin.Hash{0x21},
			OutputIndex:     0,
		},
		Value: 99000,
	}

	btcChain := &walletSyncBtcChain{
		utxos: []*bitcoin.UnspentTransactionOutput{changeUtxo},
	}

	err := EnsureWalletSyncedBetweenChains(walletPKH, mainUtxo, Connect(), btcChain)

	if err == nil {
		t.Error("expected error when main UTXO has been spent on Bitcoin, got nil")
	}
}
