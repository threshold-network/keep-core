package tbtcpg_test

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

func TestMovingFundsAction_FindTargetWallets_CommitmentNotSubmittedYet(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
	)

	var tests = map[string]struct {
		walletBalance         uint64
		walletMaxBtcTransfer  uint64
		registeredWallets     []walletInfo
		liveWalletsCount      uint32
		expectedTargetWallets [][20]byte
		expectedError         error
	}{
		"success scenario": {
			walletBalance:        2000000,
			walletMaxBtcTransfer: 300000,
			registeredWallets: []walletInfo{
				{
					publicKeyHash: hexToByte20(
						"92a6ec889a8fa34f731e639edede4c75e184307c",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"fdfa28e238734271f5e0d4f53d3843ae6cc09b24",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"840dac51a6346e9372efbdc5d3503ed9fd32abdf",
					),
					state: tbtc.StateMovingFunds,
				},
				{
					publicKeyHash: hexToByte20(
						"3091d288521caec06ea912eacfd733edc5a36d6e",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"c7302d75072d78be94eb8d36c4b77583c7abb06e",
					),
					state: tbtc.StateLive,
				},
			},
			liveWalletsCount: 4,
			expectedTargetWallets: [][20]byte{
				// The target wallets list should include all the Live wallets
				// and the wallets should be sorted according to their numerical
				// representation.
				hexToByte20(
					"3091d288521caec06ea912eacfd733edc5a36d6e",
				),
				hexToByte20(
					"92a6ec889a8fa34f731e639edede4c75e184307c",
				),
				hexToByte20(
					"c7302d75072d78be94eb8d36c4b77583c7abb06e",
				),
				hexToByte20(
					"fdfa28e238734271f5e0d4f53d3843ae6cc09b24",
				),
			},
			expectedError: nil,
		},
		"wallet max BTC transfer is zero": {
			walletBalance:         10000,
			walletMaxBtcTransfer:  0, // Set to zero.
			registeredWallets:     []walletInfo{},
			liveWalletsCount:      4,
			expectedTargetWallets: nil,
			expectedError:         tbtcpg.ErrMaxBtcTransferZero,
		},
		"not enough live wallets": {
			walletBalance:        2000000,
			walletMaxBtcTransfer: 300000,
			// Simulate there should be two Live wallets, but set only one Live
			// wallet. This could only happen if one of the target wallets
			// changed its state between getting the live wallets count and
			// getting wallets chain data.
			registeredWallets: []walletInfo{
				{
					publicKeyHash: hexToByte20(
						"92a6ec889a8fa34f731e639edede4c75e184307c",
					),
					state: tbtc.StateClosing,
				},
				{
					publicKeyHash: hexToByte20(
						"fdfa28e238734271f5e0d4f53d3843ae6cc09b24",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"840dac51a6346e9372efbdc5d3503ed9fd32abdf",
					),
					state: tbtc.StateMovingFunds,
				},
			},
			liveWalletsCount:      2,
			expectedTargetWallets: nil,
			expectedError:         tbtcpg.ErrNotEnoughTargetWallets,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			tbtcChain.SetWalletParameters(
				0,
				0,
				0,
				0,
				0,
				test.walletMaxBtcTransfer,
				0,
			)

			for _, walletInfo := range test.registeredWallets {
				tbtcChain.AddPastNewWalletRegisteredEvent(
					nil,
					&tbtc.NewWalletRegisteredEvent{
						WalletPublicKeyHash: walletInfo.publicKeyHash,
					},
				)
				tbtcChain.SetWallet(
					walletInfo.publicKeyHash,
					&tbtc.WalletChainData{State: walletInfo.state},
				)
			}

			task := tbtcpg.NewMovingFundsTask(tbtcChain, nil)

			// Always simulate the moving funds commitment has not been
			// submitted yet.
			targetWalletsCommitmentHash := [32]byte{}

			targetWallets, alreadySubmitted, err := task.FindTargetWallets(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				targetWalletsCommitmentHash,
				test.walletBalance,
				test.liveWalletsCount,
			)

			if diff := deep.Equal(
				test.expectedTargetWallets,
				targetWallets,
			); diff != nil {
				t.Errorf("unexpected target wallets: %v", diff)
			}

			// Returned value for the already submitted commitment should
			// always be false.
			expectedAlreadySubmitted := false
			testutils.AssertBoolsEqual(
				t,
				"already submitted flag",
				expectedAlreadySubmitted,
				alreadySubmitted,
			)

			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)
		})
	}
}

func TestMovingFundsAction_FindTargetWallets_CommitmentAlreadySubmitted(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
	)

	currentBlock := uint64(1000000)

	averageBlockTime := 10 * time.Second

	var tests = map[string]struct {
		targetWalletsCommitmentHash [32]byte
		targetWallets               []walletInfo
		expectedTargetWallets       [][20]byte
		expectedError               error
	}{
		"success scenario": {
			targetWalletsCommitmentHash: hexToByte32(
				"7820cd666bf13bda0850e52cfacf64140716e578f7f6a0567cae9b002fc83775",
			),
			targetWallets: []walletInfo{
				{
					publicKeyHash: hexToByte20(
						"92a6ec889a8fa34f731e639edede4c75e184307c",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"c7302d75072d78be94eb8d36c4b77583c7abb06e",
					),
					state: tbtc.StateLive,
				},
				{
					publicKeyHash: hexToByte20(
						"fdfa28e238734271f5e0d4f53d3843ae6cc09b24",
					),
					state: tbtc.StateLive,
				},
			},
			expectedTargetWallets: [][20]byte{
				hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
				hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
				hexToByte20("fdfa28e238734271f5e0d4f53d3843ae6cc09b24"),
			},
			expectedError: nil,
		},
		"target wallet commitment hash mismatch": {
			targetWalletsCommitmentHash: hexToByte32(
				"7820cd666bf13bda0850e52cfacf64140716e578f7f6a0567cae9b002fc83775",
			),
			targetWallets: []walletInfo{
				{ // Use only one target wallet.
					publicKeyHash: hexToByte20(
						"92a6ec889a8fa34f731e639edede4c75e184307c",
					),
					state: tbtc.StateLive,
				},
			},
			expectedTargetWallets: nil,
			expectedError:         tbtcpg.ErrWrongCommitmentHash,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()

			tbtcChain.SetAverageBlockTime(averageBlockTime)

			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(currentBlock)
			tbtcChain.SetBlockCounter(blockCounter)

			startBlock := currentBlock - tbtcpg.MovingFundsCommitmentLookBackBlocks

			targetWallets := [][20]byte{}
			for _, walletInfo := range test.targetWallets {
				targetWallets = append(targetWallets, walletInfo.publicKeyHash)
				tbtcChain.SetWallet(
					walletInfo.publicKeyHash,
					&tbtc.WalletChainData{State: walletInfo.state},
				)
			}

			err := tbtcChain.AddPastMovingFundsCommitmentSubmittedEvent(
				&tbtc.MovingFundsCommitmentSubmittedEventFilter{
					StartBlock:          startBlock,
					WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
				},
				&tbtc.MovingFundsCommitmentSubmittedEvent{
					TargetWallets: targetWallets,
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			task := tbtcpg.NewMovingFundsTask(tbtcChain, nil)

			// Live wallets count and wallet's balance don't matter, as we are
			// retrieving target wallets from an already submitted commitment.
			walletBalance := uint64(2000000)
			liveWalletsCount := uint32(5)

			targetWallets, alreadySubmitted, err := task.FindTargetWallets(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				test.targetWalletsCommitmentHash,
				walletBalance,
				liveWalletsCount,
			)

			if diff := deep.Equal(
				test.expectedTargetWallets,
				targetWallets,
			); diff != nil {
				t.Errorf("unexpected target wallets: %v", diff)
			}

			// Returned value for the already submitted commitment should
			// always be true.
			expectedAlreadySubmitted := true
			testutils.AssertBoolsEqual(
				t,
				"already submitted flag",
				expectedAlreadySubmitted,
				alreadySubmitted,
			)

			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)
		})
	}
}

func TestMovingFundsAction_GetWalletMembersInfo(t *testing.T) {
	var tests = map[string]struct {
		walletOperators          []operatorInfo
		frostOperatorIDs         []operatorInfo
		executingOperator        chain.Address
		useFrostOperatorIDs      bool
		expectedMemberIDs        []uint32
		expectedOperatorPosition uint32
		expectedError            error
	}{
		"success case": {
			walletOperators: []operatorInfo{
				// The executing operator controls two wallet members.
				{"5df232b0348928793658dd05dfc6b05a59d11ae8", 3},
				{"dcc895d32b74b34cef2baa6546884fcda65da1e9", 1},
				{"28759deda2ea33bd72f68ea2e8f60cd670c2549f", 2},
				{"f7891d42f3c61a49e0aed1e31b151877c0905cf7", 4},
				{"28759deda2ea33bd72f68ea2e8f60cd670c2549f", 2},
			},
			executingOperator:        "28759deda2ea33bd72f68ea2e8f60cd670c2549f",
			expectedMemberIDs:        []uint32{3, 1, 2, 4, 2},
			expectedOperatorPosition: 3,
			expectedError:            nil,
		},
		"success case with FROST operator IDs": {
			walletOperators: []operatorInfo{
				// Legacy IDs are intentionally different to prove the FROST
				// sortition pool is used.
				{"5df232b0348928793658dd05dfc6b05a59d11ae8", 30},
				{"dcc895d32b74b34cef2baa6546884fcda65da1e9", 10},
				{"28759deda2ea33bd72f68ea2e8f60cd670c2549f", 20},
				{"f7891d42f3c61a49e0aed1e31b151877c0905cf7", 40},
				{"28759deda2ea33bd72f68ea2e8f60cd670c2549f", 20},
			},
			frostOperatorIDs: []operatorInfo{
				{"5df232b0348928793658dd05dfc6b05a59d11ae8", 3},
				{"dcc895d32b74b34cef2baa6546884fcda65da1e9", 1},
				{"28759deda2ea33bd72f68ea2e8f60cd670c2549f", 2},
				{"f7891d42f3c61a49e0aed1e31b151877c0905cf7", 4},
			},
			executingOperator:        "28759deda2ea33bd72f68ea2e8f60cd670c2549f",
			useFrostOperatorIDs:      true,
			expectedMemberIDs:        []uint32{3, 1, 2, 4, 2},
			expectedOperatorPosition: 3,
			expectedError:            nil,
		},
		"executing operator not among operators": {
			walletOperators: []operatorInfo{
				{"5df232b0348928793658dd05dfc6b05a59d11ae8", 2},
				{"dcc895d32b74b34cef2baa6546884fcda65da1e9", 1},
			},
			executingOperator:        "28759deda2ea33bd72f68ea2e8f60cd670c2549f",
			expectedMemberIDs:        nil,
			expectedOperatorPosition: 0,
			expectedError:            tbtcpg.ErrNoExecutingOperator,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()

			task := tbtcpg.NewMovingFundsTask(tbtcChain, nil)

			walletOperators := []chain.Address{}
			for _, operatorInfo := range test.walletOperators {
				err := tbtcChain.SetOperatorID(
					operatorInfo.Address,
					operatorInfo.OperatorID,
				)
				if err != nil {
					t.Fatal(err)
				}
				walletOperators = append(walletOperators, operatorInfo.Address)
			}
			for _, operatorInfo := range test.frostOperatorIDs {
				err := tbtcChain.SetFrostOperatorID(
					operatorInfo.Address,
					operatorInfo.OperatorID,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			memberIDs, operatorPosition, err := task.GetWalletMembersInfo(
				walletOperators,
				test.executingOperator,
				test.useFrostOperatorIDs,
			)

			if diff := deep.Equal(test.expectedMemberIDs, memberIDs); diff != nil {
				t.Errorf("unexpected memberIDs: %v", diff)
			}

			testutils.AssertUintsEqual(
				t,
				"operator position",
				uint64(test.expectedOperatorPosition),
				uint64(operatorPosition),
			)

			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)
		})
	}
}

func TestMovingFundsAction_SubmitMovingFundsCommitment(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
	)

	walletMainUtxo := bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: hexToByte32(
				"102414558e061ea6e73d5a7bdbf1159b1518c071c22005475d0215ec78a0b911",
			),
			OutputIndex: 11,
		},
		Value: 111,
	}

	currentBlock := uint64(200000)

	var tests = map[string]struct {
		targetWalletsCommitmentHash [32]byte
		walletMemberIDs             []uint32
		walletMemberIndex           uint32
		targetWallets               [][20]byte
		expectedError               error
	}{
		"submission successful": {
			// Simulate the commitment has updated.
			targetWalletsCommitmentHash: hexToByte32(
				"9d9368117956680760fa27bb9542ceba2d4fcc398d640a5a0769f5a9593afb0e",
			),
			walletMemberIDs:   []uint32{11, 22, 33, 44},
			walletMemberIndex: 1,
			targetWallets: [][20]byte{
				hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
				hexToByte20("fdfa28e238734271f5e0d4f53d3843ae6cc09b24"),
			},
			expectedError: nil,
		},
		"submission unsuccessful": {
			// Simulate the commitment has not been updated by setting target
			// wallets commitment has to zero. The rest of the parameters is
			// not important.
			targetWalletsCommitmentHash: [32]byte{},
			walletMemberIDs:             []uint32{},
			walletMemberIndex:           0,
			targetWallets:               [][20]byte{},
			expectedError:               tbtcpg.ErrTransactionNotIncluded,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()

			tbtcChain.SetWallet(
				walletPublicKeyHash,
				&tbtc.WalletChainData{
					MovingFundsTargetWalletsCommitmentHash: test.targetWalletsCommitmentHash,
				},
			)

			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(currentBlock)
			tbtcChain.SetBlockCounter(blockCounter)

			task := tbtcpg.NewMovingFundsTask(tbtcChain, nil)

			err := task.SubmitMovingFundsCommitment(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				&walletMainUtxo,
				test.walletMemberIDs,
				test.walletMemberIndex,
				test.targetWallets,
			)

			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)

			submittedMovingFundsCommitments := tbtcChain.GetMovingFundsSubmissions()
			testutils.AssertIntsEqual(
				t,
				"commitment submission count",
				1,
				len(submittedMovingFundsCommitments),
			)

			submittedMovingFundsCommitment := submittedMovingFundsCommitments[0]

			expectedWalletPublicKeyHash := walletPublicKeyHash
			actualWalletPublicKeyHash := submittedMovingFundsCommitment.WalletPublicKeyHash
			testutils.AssertBytesEqual(
				t,
				expectedWalletPublicKeyHash[:],
				actualWalletPublicKeyHash[:],
			)

			expectedWalletMainUtxo := &walletMainUtxo
			actualWalletMainUtxo := submittedMovingFundsCommitment.WalletMainUtxo
			if diff := deep.Equal(expectedWalletMainUtxo, actualWalletMainUtxo); diff != nil {
				t.Errorf("invalid wallet main utxo: %v", diff)
			}

			expectedWalletMemberIDs := test.walletMemberIDs
			actualWalletMemberIDs := submittedMovingFundsCommitment.WalletMembersIDs
			if diff := deep.Equal(expectedWalletMemberIDs, actualWalletMemberIDs); diff != nil {
				t.Errorf("invalid wallet member IDs: %v", diff)
			}

			expectedWalletMemberIndex := test.walletMemberIndex
			actualWalletMemberIndex := submittedMovingFundsCommitment.WalletMemberIndex
			if diff := deep.Equal(expectedWalletMemberIndex, actualWalletMemberIndex); diff != nil {
				t.Errorf("invalid wallet member index: %v", diff)
			}

			expectedTargetWallets := test.targetWallets
			actualTargetWallets := submittedMovingFundsCommitment.TargetWallets
			if diff := deep.Equal(expectedTargetWallets, actualTargetWallets); diff != nil {
				t.Errorf("invalid target wallets: %v", diff)
			}
		})
	}
}

func TestMovingFundsAction_ProposeMovingFunds(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
	)

	targetWallets := [][20]byte{
		hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
		hexToByte20("fdfa28e238734271f5e0d4f53d3843ae6cc09b24"),
		hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
	}

	walletMainUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: hexToByte32(
				"102414558e061ea6e73d5a7bdbf1159b1518c071c22005475d0215ec78a0b911",
			),
			OutputIndex: 11,
		},
		Value: 111,
	}

	txMaxTotalFee := uint64(6000)

	var tests = map[string]struct {
		fee              int64
		expectedProposal *tbtc.MovingFundsProposal
	}{
		"fee provided": {
			fee: 10000,
			expectedProposal: &tbtc.MovingFundsProposal{
				TargetWallets:    targetWallets,
				MovingFundsTxFee: big.NewInt(10000),
			},
		},
		"fee estimated": {
			fee: 0, // trigger fee estimation
			expectedProposal: &tbtc.MovingFundsProposal{
				TargetWallets:    targetWallets,
				MovingFundsTxFee: big.NewInt(4300),
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			currentBlock := uint64(200000)

			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(currentBlock)
			tbtcChain.SetBlockCounter(blockCounter)

			btcChain.SetEstimateSatPerVByteFee(1, 25)

			tbtcChain.SetWallet(
				walletPublicKeyHash,
				&tbtc.WalletChainData{
					MovingFundsRequestedAt: time.Now().Add(-25 * time.Hour),
				},
			)

			// Simulate the wallet was not chosen as a target wallet for another
			// moving funds wallet.
			tbtcChain.AddPastMovingFundsCommitmentSubmittedEvent(
				&tbtc.MovingFundsCommitmentSubmittedEventFilter{
					StartBlock: 0,
				},
				&tbtc.MovingFundsCommitmentSubmittedEvent{},
			)

			tbtcChain.SetMovingFundsParameters(
				txMaxTotalFee,
				0,
				0,
				604800,
				nil,
				0,
				0,
				0,
				0,
				nil,
				0,
			)

			err := tbtcChain.SetMovingFundsProposalValidationResult(
				walletPublicKeyHash,
				walletMainUtxo,
				test.expectedProposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			task := tbtcpg.NewMovingFundsTask(tbtcChain, btcChain)

			proposal, err := task.ProposeMovingFunds(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				walletMainUtxo,
				targetWallets,
				test.fee,
			)
			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(proposal, test.expectedProposal); diff != nil {
				t.Errorf("invalid moving funds proposal: %v", diff)
			}
		})
	}
}

func TestEstimateMovingFundsFee(t *testing.T) {
	var tests = map[string]struct {
		txMaxTotalFee uint64
		expectedFee   uint64
		expectedError error
	}{
		"estimated fee correct": {
			txMaxTotalFee: 6000,
			expectedFee:   3248,
			expectedError: nil,
		},
		"estimated fee too high": {
			txMaxTotalFee: 3000,
			expectedFee:   0,
			expectedError: tbtcpg.ErrFeeTooHigh,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, 16)

			targetWalletsCount := 4

			actualFee, err := tbtcpg.EstimateMovingFundsFee(
				btcChain,
				targetWalletsCount,
				test.txMaxTotalFee,
			)

			testutils.AssertUintsEqual(
				t,
				"fee",
				test.expectedFee,
				uint64(actualFee),
			)

			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)
		})
	}
}

type walletInfo struct {
	publicKeyHash [20]byte
	state         tbtc.WalletState
}

type operatorInfo struct {
	chain.Address
	chain.OperatorID
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

func hexToByte32(hexStr string) [32]byte {
	if len(hexStr) != 64 {
		panic("hex string length incorrect")
	}
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}
	var result [32]byte
	copy(result[:], decoded)
	return result
}
