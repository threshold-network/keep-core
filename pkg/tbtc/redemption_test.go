package tbtc

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tecdsa"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
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

			// The signature within the scenario fixture is in the format
			// suitable for applying them directly to a Bitcoin transaction.
			// However, the signing executor operates on raw tECDSA signatures
			// so, we need to unpack it first.
			rawSignature := &tecdsa.Signature{
				R: scenario.Signature.R,
				S: scenario.Signature.S,
			}

			// Set up the signing executor mock to return the signature from
			// the test fixture when called with the expected parameters.
			// Note that the start block is set based on the proposal
			// processing start block as done within the action.
			signingExecutor.setSignatures(
				[]*big.Int{scenario.ExpectedSigHash},
				proposalProcessingStartBlock,
				[]*tecdsa.Signature{rawSignature},
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
				nil,
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
