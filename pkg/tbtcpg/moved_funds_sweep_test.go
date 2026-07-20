package tbtcpg_test

import (
	"math/big"
	"testing"

	"github.com/go-test/deep"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

func TestMovedFundsSweepAction_FindMovingFundsTxData_Successful(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"92a6ec889a8fa34f731e639edede4c75e184307c",
	)

	currentBlock := uint64(1000000)

	tbtcChain := tbtcpg.NewLocalChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)

	startBlock := currentBlock - tbtcpg.MovedFundsSweepLookBackBlocks

	// Test data on all the moving funds commitments.
	movingFundsCommitments := []struct {
		sourceWallet  [20]byte
		targetWallets [][20]byte
	}{
		// Moving funds 1: transfers funds to the test wallet. The proof has
		//                 not been submitted yet.
		{
			sourceWallet: hexToByte20(
				"1e37ea706ec1300d518ac6e4f58d1f4390443566",
			),
			targetWallets: [][20]byte{
				// unrelated wallet at index 0
				hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
				// test wallet at index 1
				hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
			},
		},

		// Moving funds 2: transfers funds to the test wallet. The proof has
		//                 been submitted but the moved funds sweep request is
		//                 already processed.
		{
			sourceWallet: hexToByte20(
				"d4162e0c6522eafac43650c78928910ceda4aef7",
			),
			targetWallets: [][20]byte{
				// test wallet at index 0
				hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
			},
		},

		// Moving funds 3: transfers funds to the test wallet. The proof has
		//                 been submitted and the moved funds sweep request is
		//                 pending.
		{
			sourceWallet: hexToByte20(
				"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
			),
			targetWallets: [][20]byte{
				// unrelated wallet at index 0
				hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
				// test wallet at index 1
				hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
				// unrelated wallet at index 2
				hexToByte20("c7302d75072d78be94eb8d36c4b77583c7abb06e"),
			},
		},

		// Moving funds 4: transfers funds to unrelated wallet.
		{
			sourceWallet: hexToByte20(
				"a37c0a3e56decce984e4e6ccda7394c09b507fb5",
			),
			targetWallets: [][20]byte{
				// unrelated wallet at index 0
				hexToByte20("3091d288521caec06ea912eacfd733edc5a36d6e"),
			},
		},
	}

	for _, movingFundsTx := range movingFundsCommitments {
		err := tbtcChain.AddPastMovingFundsCommitmentSubmittedEvent(
			&tbtc.MovingFundsCommitmentSubmittedEventFilter{
				StartBlock: startBlock,
			},
			&tbtc.MovingFundsCommitmentSubmittedEvent{
				WalletPublicKeyHash: movingFundsTx.sourceWallet,
				TargetWallets:       movingFundsTx.targetWallets,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The filter for moving funds completed events contains the starting block
	// and the public key hashes of all the source wallets that committed to
	// move funds to the test wallet.
	movingFundsCompletedFilter := &tbtc.MovingFundsCompletedEventFilter{
		StartBlock: startBlock,
		WalletPublicKeyHash: [][20]byte{
			hexToByte20("1e37ea706ec1300d518ac6e4f58d1f4390443566"),
			hexToByte20("d4162e0c6522eafac43650c78928910ceda4aef7"),
			hexToByte20("ffb3f7538bfa98a511495dd96027cfbd57baf2fa"),
		},
	}

	// Tests data on all the completed moving funds transactions. Note it does
	// not contain data from moving funds 1 as the transaction is not completed.
	movingFundsTransactions := []struct {
		sourceWallet [20]byte
		txHash       bitcoin.Hash
	}{
		// Data from Moving funds 2.
		{
			sourceWallet: hexToByte20(
				"d4162e0c6522eafac43650c78928910ceda4aef7",
			),
			txHash: hashFromString(
				"722b27db8c84795a0eff3103394d0d34469dc79b822981eeb2ae0c023d7ce1e0",
			),
		},

		// Data from Moving funds 3.
		{
			sourceWallet: hexToByte20(
				"ffb3f7538bfa98a511495dd96027cfbd57baf2fa",
			),
			txHash: hashFromString(
				"9a745afae8ad4e3a50f64da9ad23e6d14f9bd5e13ae6266d2099e6d16388161c",
			),
		},
	}

	for _, movingFundsTx := range movingFundsTransactions {
		err := tbtcChain.AddPastMovingFundsCompletedEvent(
			movingFundsCompletedFilter,
			&tbtc.MovingFundsCompletedEvent{
				WalletPublicKeyHash: movingFundsTx.sourceWallet,
				MovingFundsTxHash:   movingFundsTx.txHash,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Data to build moved funds sweep requests. Only the
	movedFundsSweepRequests := []struct {
		txHash      bitcoin.Hash
		outputIndex uint32
		state       tbtc.MovedFundsSweepRequestState
	}{
		// Data from Moving funds 2.
		{
			txHash: hashFromString(
				"722b27db8c84795a0eff3103394d0d34469dc79b822981eeb2ae0c023d7ce1e0",
			),
			outputIndex: 0,
			state:       tbtc.MovedFundsStateProcessed,
		},

		// Data from Moving funds 3.
		{
			txHash: hashFromString(
				"9a745afae8ad4e3a50f64da9ad23e6d14f9bd5e13ae6266d2099e6d16388161c",
			),
			outputIndex: 1,
			state:       tbtc.MovedFundsStatePending,
		},
	}

	for _, request := range movedFundsSweepRequests {
		tbtcChain.SetMovedFundsSweepRequest(
			request.txHash,
			request.outputIndex,
			&tbtc.MovedFundsSweepRequest{
				State: request.state,
			},
		)
	}

	task := tbtcpg.NewMovedFundsSweepTask(
		tbtcChain,
		nil,
	)

	movingFundsTxHash, movingFundsOutputIdx, err := task.FindMovingFundsTxData(
		walletPublicKeyHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The expected result comes from the Moving funds 3.
	expectedOutputIdx := uint32(1)
	expectedTxHash := hashFromString(
		"9a745afae8ad4e3a50f64da9ad23e6d14f9bd5e13ae6266d2099e6d16388161c",
	)

	testutils.AssertBytesEqual(
		t,
		expectedTxHash[:],
		movingFundsTxHash[:],
	)

	testutils.AssertUintsEqual(
		t,
		"fee",
		uint64(expectedOutputIdx),
		uint64(movingFundsOutputIdx),
	)
}

func TestMovedFundsSweepAction_FindMovingFundsTxData_Failure(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"92a6ec889a8fa34f731e639edede4c75e184307c",
	)

	currentBlock := uint64(1000000)

	tbtcChain := tbtcpg.NewLocalChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)

	startBlock := currentBlock - tbtcpg.MovedFundsSweepLookBackBlocks

	// Test data containing moving funds commitment to this wallet that has
	// already been sweep.
	movingFundsCommitment := struct {
		sourceWallet  [20]byte
		targetWallets [][20]byte
	}{
		sourceWallet: hexToByte20(
			"d4162e0c6522eafac43650c78928910ceda4aef7",
		),
		targetWallets: [][20]byte{
			// test wallet at index 0
			hexToByte20("92a6ec889a8fa34f731e639edede4c75e184307c"),
		},
	}

	err := tbtcChain.AddPastMovingFundsCommitmentSubmittedEvent(
		&tbtc.MovingFundsCommitmentSubmittedEventFilter{
			StartBlock: startBlock,
		},
		&tbtc.MovingFundsCommitmentSubmittedEvent{
			WalletPublicKeyHash: movingFundsCommitment.sourceWallet,
			TargetWallets:       movingFundsCommitment.targetWallets,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// The filter for moving funds completed events contains the starting block
	// and the public key hashes of the source wallet that committed to move
	// funds to the test wallet.
	movingFundsCompletedFilter := &tbtc.MovingFundsCompletedEventFilter{
		StartBlock: startBlock,
		WalletPublicKeyHash: [][20]byte{
			hexToByte20("d4162e0c6522eafac43650c78928910ceda4aef7"),
		},
	}

	// Tests data on all the completed moving funds transactions.
	movingFundsTransactions := struct {
		sourceWallet [20]byte
		txHash       bitcoin.Hash
	}{
		sourceWallet: hexToByte20(
			"d4162e0c6522eafac43650c78928910ceda4aef7",
		),
		txHash: hashFromString(
			"722b27db8c84795a0eff3103394d0d34469dc79b822981eeb2ae0c023d7ce1e0",
		),
	}

	err = tbtcChain.AddPastMovingFundsCompletedEvent(
		movingFundsCompletedFilter,
		&tbtc.MovingFundsCompletedEvent{
			WalletPublicKeyHash: movingFundsTransactions.sourceWallet,
			MovingFundsTxHash:   movingFundsTransactions.txHash,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Data to build moved funds sweep request.
	movedFundsSweepRequest := struct {
		txHash      bitcoin.Hash
		outputIndex uint32
		state       tbtc.MovedFundsSweepRequestState
	}{
		txHash: hashFromString(
			"722b27db8c84795a0eff3103394d0d34469dc79b822981eeb2ae0c023d7ce1e0",
		),
		outputIndex: 0,
		state:       tbtc.MovedFundsStateProcessed,
	}

	tbtcChain.SetMovedFundsSweepRequest(
		movedFundsSweepRequest.txHash,
		movedFundsSweepRequest.outputIndex,
		&tbtc.MovedFundsSweepRequest{
			State: movedFundsSweepRequest.state,
		},
	)

	task := tbtcpg.NewMovedFundsSweepTask(
		tbtcChain,
		nil,
	)

	_, _, err = task.FindMovingFundsTxData(
		walletPublicKeyHash,
	)

	expectedError := tbtcpg.ErrNoPendingMovedFundsSweepRequests

	testutils.AssertAnyErrorInChainMatchesTarget(t, expectedError, err)
}

func TestMovedFundsSweepAction_ProposeMovedFundsSweep(t *testing.T) {
	walletPublicKeyHash := hexToByte20(
		"92a6ec889a8fa34f731e639edede4c75e184307c",
	)

	sweepTxMaxTotalFee := uint64(6000)

	movingFundsTxHash := hashFromString(
		"9a745afae8ad4e3a50f64da9ad23e6d14f9bd5e13ae6266d2099e6d16388161c",
	)

	movingFundsTxOutputIndex := uint32(1)

	hasMainUtxo := true

	var tests = map[string]struct {
		fee              int64
		expectedProposal *tbtc.MovedFundsSweepProposal
	}{
		"fee provided": {
			fee: 10000,
			expectedProposal: &tbtc.MovedFundsSweepProposal{
				MovingFundsTxHash:        movingFundsTxHash,
				MovingFundsTxOutputIndex: movingFundsTxOutputIndex,
				SweepTxFee:               big.NewInt(10000),
			},
		},
		"fee estimated": {
			fee: 0, // trigger fee estimation
			expectedProposal: &tbtc.MovedFundsSweepProposal{
				MovingFundsTxHash:        movingFundsTxHash,
				MovingFundsTxOutputIndex: movingFundsTxOutputIndex,
				// raw 4450 (178 vByte * 25 sat/vByte), buffered to
				// ceil(25*1.25)=32 sat/vByte * 178 = 5696, below the 6000 cap.
				SweepTxFee: big.NewInt(5696),
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			btcChain.SetEstimateSatPerVByteFee(1, 25)

			tbtcChain.SetMovingFundsParameters(
				0,
				0,
				0,
				0,
				nil,
				0,
				0,
				sweepTxMaxTotalFee,
				0,
				nil,
				0,
			)

			err := tbtcChain.SetMovedFundsSweepProposalValidationResult(
				walletPublicKeyHash,
				test.expectedProposal,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}

			task := tbtcpg.NewMovedFundsSweepTask(tbtcChain, btcChain)

			proposal, err := task.ProposeMovedFundsSweep(
				&testutils.MockLogger{},
				walletPublicKeyHash,
				movingFundsTxHash,
				movingFundsTxOutputIndex,
				hasMainUtxo,
				test.fee,
			)
			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(proposal, test.expectedProposal); diff != nil {
				t.Errorf("invalid moved funds sweep proposal: %v", diff)
			}
		})
	}
}

func TestEstimateMovedFundsSweepFee(t *testing.T) {
	var tests = map[string]struct {
		sweepTxMaxTotalFee uint64
		hasMainUtxo        bool
		expectedFee        uint64
		expectedError      error
	}{
		"estimated fee correct, one input": {
			sweepTxMaxTotalFee: 3000,
			hasMainUtxo:        false,
			// raw 1760 (110 vByte * 16 sat/vByte), buffered to
			// ceil(16*1.25)=20 sat/vByte * 110 = 2200, below the cap.
			expectedFee:   2200,
			expectedError: nil,
		},
		"estimated fee correct, two inputs": {
			sweepTxMaxTotalFee: 3000,
			hasMainUtxo:        true,
			// raw 2848 (178 vByte * 16 sat/vByte); buffered 20 sat/vByte * 178
			// = 3560 exceeds the 3000 cap, so it is bounded down to the cap.
			expectedFee:   3000,
			expectedError: nil,
		},
		"estimated fee too high": {
			sweepTxMaxTotalFee: 2500,
			hasMainUtxo:        true,
			expectedFee:        0,
			expectedError:      tbtcpg.ErrSweepTxFeeTooHigh,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, 16)

			actualFee, err := tbtcpg.EstimateMovedFundsSweepFee(
				btcChain,
				test.hasMainUtxo,
				test.sweepTxMaxTotalFee,
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

func hashFromString(str string) bitcoin.Hash {
	hash, err := bitcoin.NewHashFromString(
		str,
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		panic(err)
	}

	return hash
}
