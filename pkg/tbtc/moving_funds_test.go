package tbtc

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

// TODO: Think about covering unhappy paths for specific steps of the moving funds action.
func TestMovingFundsAction_Execute(t *testing.T) {
	scenarios, err := test.LoadMovingFundsTestScenarios()
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

			// Record the transaction that will serve as moving funds transaction's
			// input in the Bitcoin local chain.
			err := bitcoinChain.BroadcastTransaction(scenario.InputTransaction)
			if err != nil {
				t.Fatal(err)
			}

			// Build the moving funds proposal based on the scenario data.
			proposal := &MovingFundsProposal{
				TargetWallets:    scenario.TargetWallets,
				MovingFundsTxFee: big.NewInt(scenario.Fee),
			}

			// Choose an arbitrary start block and expiration time.
			proposalProcessingStartBlock := uint64(100)
			proposalExpiryBlock := proposalProcessingStartBlock +
				movingFundsProposalValidityBlocks

			// Set arbitrary moving funds timeout.
			hostChain.SetMovingFundsParameters(
				0, 0, 0, 604800, big.NewInt(0), 0, 0, 0, 0, big.NewInt(0), 0,
			)

			// Simulate the on-chain proposal validation passes with success.
			err = hostChain.setMovingFundsProposalValidationResult(
				walletPublicKeyHash,
				scenario.WalletMainUtxo,
				proposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			// Simulate the wallet was not chosen as a target wallet for another
			// moving funds wallet.
			hostChain.setPastMovingFundsCommitmentSubmittedEvents(
				&MovingFundsCommitmentSubmittedEventFilter{
					StartBlock: 0,
				},
				[]*MovingFundsCommitmentSubmittedEvent{},
			)

			// Record the wallet main UTXO hash and moving funds commitment
			// hash in the local host chain so the moving funds action can detect it.
			walletMainUtxoHash := hostChain.ComputeMainUtxoHash(
				scenario.WalletMainUtxo,
			)
			movingFundsCommitmentHash := hostChain.ComputeMovingFundsCommitmentHash(
				scenario.TargetWallets,
			)
			hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
				MainUtxoHash:                           walletMainUtxoHash,
				MovingFundsTargetWalletsCommitmentHash: movingFundsCommitmentHash,
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
				proposalProcessingStartBlock+movingFundsCommitmentConfirmationBlocks,
				[]*tecdsa.Signature{rawSignature},
			)

			action := newMovingFundsAction(
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

			err = action.execute()
			if err != nil {
				t.Fatal(err)
			}

			// Action execution that completes without an error is a sign of
			// success. However, just in case, make an additional check that
			// the expected moving funds transaction was actually broadcasted
			// on the local Bitcoin chain.
			broadcastedMovingFundsTransaction, err := bitcoinChain.GetTransaction(
				scenario.ExpectedMovingFundsTransactionHash,
			)
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertBytesEqual(
				t,
				scenario.ExpectedMovingFundsTransaction.Serialize(),
				broadcastedMovingFundsTransaction.Serialize(),
			)
		})
	}
}

func TestAssembleMovingFundsTransaction(t *testing.T) {
	scenarios, err := test.LoadMovingFundsTestScenarios()
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

			builder, err := assembleMovingFundsTransaction(
				bitcoinChain,
				scenario.WalletMainUtxo,
				scenario.TargetWallets,
				scenario.Fee,
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
				scenario.ExpectedMovingFundsTransaction.Serialize(),
				transaction.Serialize(),
			)
			testutils.AssertStringsEqual(
				t,
				"moving funds transaction hash",
				scenario.ExpectedMovingFundsTransactionHash.Hex(bitcoin.InternalByteOrder),
				transaction.Hash().Hex(bitcoin.InternalByteOrder),
			)
			testutils.AssertStringsEqual(
				t,
				"moving funds transaction witness hash",
				scenario.ExpectedMovingFundsTransactionWitnessHash.Hex(bitcoin.InternalByteOrder),
				transaction.WitnessHash().Hex(bitcoin.InternalByteOrder),
			)
		})
	}
}

func TestValidateMovingFundsSafetyMargin(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
	)

	var tests = map[string]struct {
		events           []*MovingFundsCommitmentSubmittedEvent
		requestedAtDelta time.Duration
		otherWallets     []struct {
			publicKeyHash [20]byte
			state         WalletState
		}
		movingFundsTimeout uint32
		expectedError      error
	}{
		"wallet is not target and safety margin passed": {
			events:             []*MovingFundsCommitmentSubmittedEvent{},
			requestedAtDelta:   -25 * time.Hour,
			movingFundsTimeout: 3628800, // 6 weeks
			expectedError:      nil,
		},
		"wallet is not target and safety margin not passed": {
			events:             []*MovingFundsCommitmentSubmittedEvent{},
			requestedAtDelta:   -23 * time.Hour,
			movingFundsTimeout: 3628800, // 6 weeks
			expectedError:      fmt.Errorf("safety margin in force"),
		},
		"wallet is target and safety margin passed": {
			events: []*MovingFundsCommitmentSubmittedEvent{
				{
					WalletPublicKeyHash: hexToByte20(
						"3091d288521caec06ea912eacfd733edc5a36d6e",
					),
					TargetWallets: [][20]byte{
						hexToByte20(
							"c7302d75072d78be94eb8d36c4b77583c7abb06e",
						),
						// wallet on the target wallets list
						walletPublicKeyHash,
					},
				},
			},
			requestedAtDelta: -15 * 24 * time.Hour,
			otherWallets: []struct {
				publicKeyHash [20]byte
				state         WalletState
			}{
				{
					publicKeyHash: hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
					state:         StateMovingFunds,
				},
			},
			movingFundsTimeout: 3628800, // 6 weeks
			expectedError:      nil,
		},
		"wallet is target and safety margin not passed": {
			events: []*MovingFundsCommitmentSubmittedEvent{
				{
					WalletPublicKeyHash: hexToByte20(
						"3091d288521caec06ea912eacfd733edc5a36d6e",
					),
					TargetWallets: [][20]byte{
						hexToByte20(
							"c7302d75072d78be94eb8d36c4b77583c7abb06e",
						),
						// wallet on the target wallets list
						walletPublicKeyHash,
					},
				},
			},
			requestedAtDelta: -13 * 24 * time.Hour,
			otherWallets: []struct {
				publicKeyHash [20]byte
				state         WalletState
			}{
				{
					publicKeyHash: hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
					state:         StateMovingFunds,
				},
			},
			movingFundsTimeout: 3628800, // 6 weeks
			expectedError:      fmt.Errorf("safety margin in force"),
		},
		"wallet is target and safety margin limited and passed": {
			events: []*MovingFundsCommitmentSubmittedEvent{
				{
					WalletPublicKeyHash: hexToByte20(
						"3091d288521caec06ea912eacfd733edc5a36d6e",
					),
					TargetWallets: [][20]byte{
						hexToByte20(
							"c7302d75072d78be94eb8d36c4b77583c7abb06e",
						),
						// wallet on the target wallets list
						walletPublicKeyHash,
					},
				},
			},
			requestedAtDelta: -8 * 24 * time.Hour,
			otherWallets: []struct {
				publicKeyHash [20]byte
				state         WalletState
			}{
				{
					publicKeyHash: hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
					state:         StateMovingFunds,
				},
			},
			// 2 weeks - it limits safety margin to 1 week (2 * 0.5)
			movingFundsTimeout: 1209600,
			expectedError:      nil,
		},
		"wallet is target and safety margin limited and not passed": {
			events: []*MovingFundsCommitmentSubmittedEvent{
				{
					WalletPublicKeyHash: hexToByte20(
						"3091d288521caec06ea912eacfd733edc5a36d6e",
					),
					TargetWallets: [][20]byte{
						hexToByte20(
							"c7302d75072d78be94eb8d36c4b77583c7abb06e",
						),
						// wallet on the target wallets list
						walletPublicKeyHash,
					},
				},
			},
			requestedAtDelta: -6 * 24 * time.Hour,
			otherWallets: []struct {
				publicKeyHash [20]byte
				state         WalletState
			}{
				{
					publicKeyHash: hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
					state:         StateMovingFunds,
				},
			},
			// 2 weeks - it limits safety margin to 1 week (2 * 0.5)
			movingFundsTimeout: 1209600,
			expectedError:      fmt.Errorf("safety margin in force"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			hostChain := Connect()

			hostChain.SetMovingFundsParameters(
				0,
				0,
				0,
				test.movingFundsTimeout,
				big.NewInt(0),
				0,
				0,
				0,
				0,
				big.NewInt(0),
				0,
			)

			hostChain.setPastMovingFundsCommitmentSubmittedEvents(
				&MovingFundsCommitmentSubmittedEventFilter{
					StartBlock: 0,
				},
				test.events,
			)

			hostChain.setWallet(walletPublicKeyHash, &WalletChainData{
				MovingFundsRequestedAt: time.Now().Add(test.requestedAtDelta),
			})

			for _, wallet := range test.otherWallets {
				hostChain.setWallet(wallet.publicKeyHash, &WalletChainData{
					State: wallet.state,
				})
			}

			err := ValidateMovingFundsSafetyMargin(
				walletPublicKeyHash,
				hostChain,
			)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					err,
				)
			}
		})
	}
}

func hexToByte20(hexStr string) [20]byte {
	if len(hexStr) != 40 {
		panic("hex string length incorrect")
	}
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}
	var result [20]byte
	copy(result[:], decoded)
	return result
}
