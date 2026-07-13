package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
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

func TestRedemptionAction_RejectsTransactionFeeAboveMaximumBeforeSigning(
	t *testing.T,
) {
	const mainUtxoValue = int64(24372)

	hostChain := Connect()
	bitcoinChain := newLocalBitcoinChain()
	redeemingWallet := generateWallet(big.NewInt(111))
	walletPublicKey := redeemingWallet.publicKey
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	walletScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
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
			{
				Value:           mainUtxoValue,
				PublicKeyScript: walletScript,
			},
		},
	}
	if err := bitcoinChain.BroadcastTransaction(fundingTx); err != nil {
		t.Fatal(err)
	}

	walletMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTx.Hash(),
			OutputIndex:     0,
		},
		Value: mainUtxoValue,
	}

	redeemerScript, err := bitcoin.PayToWitnessPublicKeyHash([20]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}

	request := &RedemptionRequest{
		Redeemer:             chain.Address("redeemer"),
		RedeemerOutputScript: redeemerScript,
		RequestedAmount:      24372,
		TreasuryFee:          12,
		TxMaxFee:             110,
		RequestedAt:          time.Now(),
	}
	hostChain.setPendingRedemptionRequest(walletPublicKeyHash, request)

	proposal := &RedemptionProposal{
		RedeemersOutputScripts: []bitcoin.Script{redeemerScript},
		RedemptionTxFee:        big.NewInt(110),
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
	hostChain.setRedemptionParameters(0, 0, 0, 121, 0, nil, 0)

	action := newRedemptionAction(
		logger.With(),
		hostChain,
		bitcoinChain,
		redeemingWallet,
		newMockWalletSigningExecutor(),
		proposal,
		100,
		100+redemptionProposalValidityBlocks,
		func(ctx context.Context, blockHeight uint64) error { return nil },
	)

	err = action.execute()
	if err == nil {
		t.Fatal("expected redemption action to reject an excessive actual fee")
	}

	testutils.AssertStringsEqual(
		t,
		"error",
		"error while validating redemption transaction fee: "+
			"[redemption transaction fee [122] exceeds maximum total fee [121]]",
		err.Error(),
	)
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

func TestAssembleRedemptionTransaction_SubDustChange(t *testing.T) {
	// The main UTXO value and the sub-dust case numbers reproduce a live
	// testnet4 redemption of the wallet's full main UTXO whose 12-satoshi
	// change output made the transaction non-relayable:
	// "dust, tx with dust output must be 0-fee".
	const mainUtxoValue = int64(24372)

	walletPublicKey := testWalletPublicKeyFromXOnly(
		t,
		"2336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008",
	)

	walletXOnlyKey, err := walletXOnlyPublicKey(walletPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	walletChangeScript, err := bitcoin.PayToTaproot(walletXOnlyKey)
	if err != nil {
		t.Fatal(err)
	}

	redeemerScript, err := bitcoin.PayToWitnessPublicKeyHash(
		[20]byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
			0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		requestedAmount            uint64
		treasuryFee                uint64
		feeShare                   int64
		maxTotalFee                uint64
		expectedFeeValidationError string
		// expectedOutputs holds the expected output values in order. A -1
		// value denotes the wallet change output; any other value denotes
		// the redeemer output.
		expectedOutputValues []int64
		expectedChangeValue  int64
		expectedImpliedFee   int64
	}{
		// change = mainUtxoValue - (requestedAmount - treasuryFee) = 12.
		// The sub-dust change must be omitted, the redeemer output must be
		// left untouched, and the implied fee must grow by the change value.
		"sub-dust change above the maximum total fee is rejected": {
			requestedAmount:      24372,
			treasuryFee:          12,
			feeShare:             110,
			maxTotalFee:          121,
			expectedOutputValues: []int64{24250},
			expectedChangeValue:  0,
			expectedImpliedFee:   122,
			expectedFeeValidationError: "redemption transaction fee [122] " +
				"exceeds maximum total fee [121]",
		},
		// An actual fee exactly equal to the maximum total fee remains valid.
		"sub-dust change at the maximum total fee is omitted": {
			requestedAmount:      24372,
			treasuryFee:          12,
			feeShare:             110,
			maxTotalFee:          122,
			expectedOutputValues: []int64{24250},
			expectedChangeValue:  0,
			expectedImpliedFee:   122,
		},
		// change = 24372 - (23839 - 12) = 545, just below the dust limit.
		"change just below the dust limit is omitted": {
			requestedAmount:      23839,
			treasuryFee:          12,
			feeShare:             110,
			maxTotalFee:          655,
			expectedOutputValues: []int64{23717},
			expectedChangeValue:  0,
			expectedImpliedFee:   655,
		},
		// change = 24372 - (23838 - 12) = 546, exactly at the dust limit.
		"change at the dust limit is kept": {
			requestedAmount:      23838,
			treasuryFee:          12,
			feeShare:             110,
			maxTotalFee:          110,
			expectedOutputValues: []int64{23716, 546},
			expectedChangeValue:  546,
			expectedImpliedFee:   110,
		},
		// change = 24372 - (24384 - 12) = 0. Same behavior as before the
		// dust guard: no change output at all.
		"zero change produces no change output": {
			requestedAmount:      24384,
			treasuryFee:          12,
			feeShare:             110,
			maxTotalFee:          110,
			expectedOutputValues: []int64{24262},
			expectedChangeValue:  0,
			expectedImpliedFee:   110,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			bitcoinChain := newLocalBitcoinChain()

			walletMainUtxo := testTaprootWalletMainUtxoWithValue(
				t,
				bitcoinChain,
				walletPublicKey,
				mainUtxoValue,
			)

			requests := []*RedemptionRequest{
				{
					Redeemer:             chain.Address("redeemer"),
					RedeemerOutputScript: redeemerScript,
					RequestedAmount:      test.requestedAmount,
					TreasuryFee:          test.treasuryFee,
					TxMaxFee:             1000,
					RequestedAt:          time.Now(),
				},
			}

			feeDistribution := func(requests []*RedemptionRequest) []int64 {
				return []int64{test.feeShare}
			}

			builder, err := assembleRedemptionTransaction(
				bitcoinChain,
				walletPublicKey,
				walletMainUtxo,
				requests,
				feeDistribution,
				RedemptionChangeLast,
			)
			if err != nil {
				t.Fatal(err)
			}

			outputs := builder.UnsignedTransaction().Outputs

			testutils.AssertIntsEqual(
				t,
				"output count",
				len(test.expectedOutputValues),
				len(outputs),
			)

			totalOutputsValue := int64(0)
			for i, output := range outputs {
				totalOutputsValue += output.Value

				testutils.AssertIntsEqual(
					t,
					fmt.Sprintf("output [%v] value", i),
					int(test.expectedOutputValues[i]),
					int(output.Value),
				)

				// The RedemptionChangeLast shape is used so the change
				// output, if present, is the last output.
				expectedScript := redeemerScript
				if test.expectedChangeValue > 0 && i == len(outputs)-1 {
					expectedScript = walletChangeScript
				}

				testutils.AssertBytesEqual(
					t,
					expectedScript,
					output.PublicKeyScript,
				)
			}

			actualFee := builder.TotalInputsValue() - totalOutputsValue

			// The implied transaction fee is the difference between the
			// input value and the total outputs value. When a sub-dust
			// change is omitted, its value must become part of the fee.
			testutils.AssertIntsEqual(
				t,
				"implied transaction fee",
				int(test.expectedImpliedFee),
				int(actualFee),
			)

			err = validateRedemptionTransactionFee(builder, test.maxTotalFee)
			if len(test.expectedFeeValidationError) == 0 {
				if err != nil {
					t.Fatalf("unexpected fee validation error: [%v]", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected fee validation error")
				}

				testutils.AssertStringsEqual(
					t,
					"fee validation error",
					test.expectedFeeValidationError,
					err.Error(),
				)
			}

			// Make sure the downstream sighash computation works for the
			// resulting transaction shape.
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
		})
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
