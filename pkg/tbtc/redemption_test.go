package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// TODO: Think about covering unhappy paths for specific steps of the redemption action.
func TestRedemptionAction_Execute(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			hostChain := Connect()
			bitcoinChain := newLocalBitcoinChain()

			wallet := wallet{
				// Set only relevant fields.
				publicKey: scenario.WalletPublicKey,
			}
			walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

			// Record the transaction that will serve as redemption transaction's
			// input in the Bitcoin local chain.
			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			// redeemersOutputScripts will be needed to build the proposal instance.
			redeemersOutputScripts := make(
				[]bitcoin.Script,
				len(scenario.RedemptionRequests),
			)

			// Record all necessary requests' data on the local host chain.
			for i, request := range scenario.RedemptionRequests {
				hostChain.setPendingRedemptionRequest(
					walletPublicKeyHash,
					&RedemptionRequest{
						Redeemer:             request.Redeemer,
						RedeemerOutputScript: request.RedeemerOutputScript,
						RequestedAmount:      request.RequestedAmount,
						TreasuryFee:          request.TreasuryFee,
						TxMaxFee:             request.TxMaxFee,
						RequestedAt:          request.RequestedAt,
					},
				)

				redeemersOutputScripts[i] = request.RedeemerOutputScript
			}

			totalFee := int64(0)
			for _, feeShare := range scenario.FeeShares {
				totalFee += feeShare
			}

			// Build the redemption proposal based on the scenario data.
			proposal := &RedemptionProposal{
				RedeemersOutputScripts: redeemersOutputScripts,
				RedemptionTxFee:        big.NewInt(totalFee),
			}

			// Choose an arbitrary start block and expiration time.
			proposalProcessingStartBlock := uint64(100)
			proposalExpiryBlock := proposalProcessingStartBlock +
				redemptionProposalValidityBlocks

			// Simulate the on-chain proposal validation passes with success.
			err = hostChain.setRedemptionProposalValidationResult(
				walletPublicKeyHash,
				proposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			// Record the wallet main UTXO hash in the local host chain so
			// the redemption action can detect it.
			var walletMainUtxoHash [32]byte
			if scenario.WalletMainUtxo != nil {
				walletMainUtxoHash = hostChain.ComputeMainUtxoHash(
					scenario.WalletMainUtxo,
				)
			}
			hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
				MainUtxoHash: walletMainUtxoHash,
			})

			// Create a signing executor mock instance.
			signingExecutor := newMockWalletSigningExecutor()

			// The signature within the scenario fixture is represented as
			// big integer components and needs conversion to runtime signature
			// container used by signing executor.
			rawSignature := mustFrostSignatureFromBigInts(
				scenario.Signature.R,
				scenario.Signature.S,
			)

			// Set up the signing executor mock to return the signature from
			// the test fixture when called with the expected parameters.
			// Note that the start block is set based on the proposal
			// processing start block as done within the action.
			signingExecutor.setSignatures(
				[]*big.Int{scenario.ExpectedSigHash},
				proposalProcessingStartBlock,
				[]*frost.Signature{rawSignature},
			)

			action := newRedemptionAction(
				logger.With(),
				hostChain,
				bitcoinChain,
				wallet,
				signingExecutor,
				proposal,
				proposalProcessingStartBlock,
				proposalExpiryBlock,
				func(ctx context.Context, blockHeight uint64) error {
					return nil
				},
			)

			// Modify the default parameters of the action to make
			// it possible to execute in the current test environment.
			action.broadcastCheckDelay = 1 * time.Second

			// Test scenarios use a different fee distribution than the
			// default one used by the redemption action. Here we override
			// the default distribution by using the one appropriate for
			// test scenarios.
			action.feeDistribution = func(requests []*RedemptionRequest) []int64 {
				return scenario.FeeShares
			}

			// Test scenarios use the RedemptionChangeLast shape which is
			// different from the default RedemptionChangeFirst shape used
			// by the redemption action. We need to override it.
			action.transactionShape = RedemptionChangeLast

			err = action.execute()
			if err != nil {
				t.Fatal(err)
			}

			// Action execution that completes without an error is a sign of
			// success. However, just in case, make an additional check that
			// the expected redemption transaction was actually broadcasted
			// on the local Bitcoin chain.
			broadcastedRedemptionTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedRedemptionTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedRedemptionTransaction.Serialize(),
				broadcastedRedemptionTransaction.Serialize(),
			)
		})
	}
}

type failIfCalledRedemptionSigningExecutor struct {
	called bool
}

func (executor *failIfCalledRedemptionSigningExecutor) signBatch(
	context.Context,
	[]*big.Int,
	uint64,
) ([]*frost.Signature, error) {
	executor.called = true
	return nil, fmt.Errorf("unexpected signing call")
}

func TestRedemptionAction_RejectsActualFeeBeforeSigning(t *testing.T) {
	hostChain := Connect()
	bitcoinChain := newLocalBitcoinChain()
	redeemingWallet := generateWallet(big.NewInt(1))
	walletPublicKey := redeemingWallet.publicKey
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		t.Fatal(err)
	}
	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{0x01},
			},
			Sequence: 0xffffffff,
		}},
		Outputs: []*bitcoin.TransactionOutput{{
			Value:           9293,
			PublicKeyScript: walletScript,
		}},
	}
	if err := bitcoinChain.BroadcastTransaction(
		fundingTransaction,
	); err != nil {
		t.Fatal(err)
	}
	walletMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTransaction.Hash(),
		},
		Value: 9293,
	}
	redeemerScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	request := &RedemptionRequest{
		RedeemerOutputScript: redeemerScript,
		RequestedAmount:      10000,
		TreasuryFee:          1000,
		TxMaxFee:             1000,
		RequestedAt:          time.Now(),
	}
	hostChain.setPendingRedemptionRequest(
		walletPublicKeyHash,
		request,
	)
	proposal := &RedemptionProposal{
		RedeemersOutputScripts: []bitcoin.Script{redeemerScript},
		RedemptionTxFee:        big.NewInt(1000),
	}
	if err := hostChain.setRedemptionProposalValidationResult(
		walletPublicKeyHash,
		proposal,
		true,
	); err != nil {
		t.Fatal(err)
	}
	hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
		MainUtxoHash: hostChain.ComputeMainUtxoHash(walletMainUtxo),
	})
	hostChain.setRedemptionParameters(
		0,
		0,
		0,
		1292,
		0,
		big.NewInt(0),
		0,
	)
	signingExecutor := &failIfCalledRedemptionSigningExecutor{}
	action := newRedemptionAction(
		logger.With(),
		hostChain,
		bitcoinChain,
		redeemingWallet,
		signingExecutor,
		proposal,
		100,
		100+redemptionProposalValidityBlocks,
		func(context.Context, uint64) error { return nil },
	)

	err = action.execute()
	if err == nil || !strings.Contains(
		err.Error(),
		"redemption transaction total fee [1293] exceeds maximum [1292]",
	) {
		t.Fatalf("unexpected action result: [%v]", err)
	}
	if signingExecutor.called {
		t.Fatal("signing executor called for an over-limit redemption fee")
	}
}

func TestAssembleRedemptionTransaction(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			requests := make([]*RedemptionRequest, len(scenario.RedemptionRequests))
			for i, r := range scenario.RedemptionRequests {
				requests[i] = &RedemptionRequest{
					Redeemer:             r.Redeemer,
					RedeemerOutputScript: r.RedeemerOutputScript,
					RequestedAmount:      r.RequestedAmount,
					TreasuryFee:          r.TreasuryFee,
					TxMaxFee:             r.TxMaxFee,
					RequestedAt:          r.RequestedAt,
				}
			}

			feeDistribution := func(requests []*RedemptionRequest) []int64 {
				return scenario.FeeShares
			}

			builder, err := assembleRedemptionTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				scenario.WalletMainUtxo,
				requests,
				feeDistribution,
				RedemptionChangeLast,
			)
			if err != nil {
				t.Fatal(err)
			}

			sigHashes, err := builder.ComputeSignatureHashes()
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"sighash count",
				1,
				len(sigHashes),
			)

			testutils.AssertBigIntsEqual(
				t,
				"sighash",
				scenario.ExpectedSigHash,
				sigHashes[0],
			)

			transaction, err := builder.AddSignatures(
				[]*bitcoin.SignatureContainer{scenario.Signature},
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedRedemptionTransaction.Serialize(),
				transaction.Serialize(),
			)
			testutils.AssertStringsEqual(
				t,
				"redemption transaction hash",
				scenario.ExpectedRedemptionTransactionHash.Hex(bitcoin.InternalByteOrder),
				transaction.Hash().Hex(bitcoin.InternalByteOrder),
			)
			testutils.AssertStringsEqual(
				t,
				"redemption transaction witness hash",
				scenario.ExpectedRedemptionTransactionWitnessHash.Hex(bitcoin.InternalByteOrder),
				transaction.WitnessHash().Hex(bitcoin.InternalByteOrder),
			)
		})
	}
}

func TestAssembleRedemptionTransaction_DustChangePolicy(t *testing.T) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	scenario := scenarios[0]

	tests := map[string]struct {
		changeValue         int64
		expectedOutputCount int
	}{
		"zero change omitted": {
			changeValue:         0,
			expectedOutputCount: 1,
		},
		"P2WPKH below dust omitted": {
			changeValue:         293,
			expectedOutputCount: 1,
		},
		"P2WPKH at dust threshold retained": {
			changeValue:         294,
			expectedOutputCount: 2,
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()
			if err := bitcoinChain.BroadcastTransaction(
				scenario.InputTransaction,
			); err != nil {
				t.Fatal(err)
			}

			request := scenario.RedemptionRequests[0]
			feeShare := scenario.FeeShares[0]
			redemptionOutputValue :=
				int64(request.RequestedAmount-request.TreasuryFee) - feeShare
			mainUtxo := &bitcoin.UnspentTransactionOutput{
				Outpoint: scenario.WalletMainUtxo.Outpoint,
				Value: redemptionOutputValue +
					feeShare +
					testCase.changeValue,
			}

			builder, err := assembleRedemptionTransaction(
				bitcoinChain,
				scenario.WalletPublicKey,
				mainUtxo,
				[]*RedemptionRequest{{
					Redeemer:             request.Redeemer,
					RedeemerOutputScript: request.RedeemerOutputScript,
					RequestedAmount:      request.RequestedAmount,
					TreasuryFee:          request.TreasuryFee,
					TxMaxFee:             request.TxMaxFee,
					RequestedAt:          request.RequestedAt,
				}},
				func([]*RedemptionRequest) []int64 {
					return []int64{feeShare}
				},
				RedemptionChangeLast,
			)
			if err != nil {
				t.Fatal(err)
			}

			outputs := builder.UnsignedTransaction().Outputs
			if len(outputs) != testCase.expectedOutputCount {
				t.Fatalf(
					"unexpected output count [%d]",
					len(outputs),
				)
			}
			if len(outputs) == 2 &&
				outputs[1].Value != testCase.changeValue {
				t.Fatalf(
					"unexpected change value [%d]",
					outputs[1].Value,
				)
			}
		})
	}
}

func TestAssembleRedemptionTransaction_TaprootDustChangePolicy(t *testing.T) {
	walletPublicKey := testWalletPublicKeyFromXOnly(
		t,
		"2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008",
	)
	redeemerScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	request := &RedemptionRequest{
		RedeemerOutputScript: redeemerScript,
		RequestedAmount:      10000,
		TreasuryFee:          1000,
		TxMaxFee:             1000,
	}

	for _, testCase := range []struct {
		name                string
		changeValue         int64
		expectedOutputCount int
	}{
		{"P2TR below dust omitted", 329, 1},
		{"P2TR at dust threshold retained", 330, 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()
			mainUtxo := testTaprootWalletMainUtxoWithValue(
				t,
				bitcoinChain,
				walletPublicKey,
				9000+testCase.changeValue,
			)
			builder, err := assembleRedemptionTransaction(
				bitcoinChain,
				walletPublicKey,
				mainUtxo,
				[]*RedemptionRequest{request},
				func([]*RedemptionRequest) []int64 {
					return []int64{1000}
				},
				RedemptionChangeLast,
			)
			if err != nil {
				t.Fatal(err)
			}

			outputs := builder.UnsignedTransaction().Outputs
			if len(outputs) != testCase.expectedOutputCount {
				t.Fatalf("unexpected output count [%d]", len(outputs))
			}
			if len(outputs) == 2 &&
				outputs[1].Value != testCase.changeValue {
				t.Fatalf("unexpected change value [%d]", outputs[1].Value)
			}
		})
	}
}

func TestValidateRedemptionTransactionTotalFee_IncludesOmittedDustChange(
	t *testing.T,
) {
	scenarios, err := test.LoadRedemptionTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	scenario := scenarios[0]
	bitcoinChain := newLocalBitcoinChain()
	if err := bitcoinChain.BroadcastTransaction(
		scenario.InputTransaction,
	); err != nil {
		t.Fatal(err)
	}
	request := scenario.RedemptionRequests[0]
	feeShare := scenario.FeeShares[0]
	changeValue := int64(293)
	redemptionOutputValue :=
		int64(request.RequestedAmount-request.TreasuryFee) - feeShare
	mainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: scenario.WalletMainUtxo.Outpoint,
		Value:    redemptionOutputValue + feeShare + changeValue,
	}
	builder, err := assembleRedemptionTransaction(
		bitcoinChain,
		scenario.WalletPublicKey,
		mainUtxo,
		[]*RedemptionRequest{{
			Redeemer:             request.Redeemer,
			RedeemerOutputScript: request.RedeemerOutputScript,
			RequestedAmount:      request.RequestedAmount,
			TreasuryFee:          request.TreasuryFee,
			TxMaxFee:             request.TxMaxFee,
			RequestedAt:          request.RequestedAt,
		}},
		func([]*RedemptionRequest) []int64 {
			return []int64{feeShare}
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	actualFee := uint64(feeShare + changeValue)
	if err := validateRedemptionTransactionTotalFee(
		builder,
		actualFee,
	); err != nil {
		t.Fatalf("fee at maximum rejected: [%v]", err)
	}
	if err := validateRedemptionTransactionTotalFee(
		builder,
		actualFee-1,
	); err == nil {
		t.Fatal("fee above maximum accepted")
	}
}

func TestWithRedemptionTotalFee(t *testing.T) {
	var tests = map[string]struct {
		totalFee          int64
		requestsCount     int
		expectedFeeShares []int64
	}{
		"total fee divisible by the requests count": {
			totalFee:          10000,
			requestsCount:     5,
			expectedFeeShares: []int64{2000, 2000, 2000, 2000, 2000},
		},
		"total fee indivisible by the requests count": {
			totalFee:          10000,
			requestsCount:     6,
			expectedFeeShares: []int64{1666, 1666, 1666, 1666, 1666, 1670},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			requests := make([]*RedemptionRequest, test.requestsCount)

			feeShares := withRedemptionTotalFee(test.totalFee)(requests)

			if diff := deep.Equal(test.expectedFeeShares, feeShares); diff != nil {
				t.Errorf(
					"unexpected fee shares\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedFeeShares,
					feeShares,
				)
			}
		})
	}
}
