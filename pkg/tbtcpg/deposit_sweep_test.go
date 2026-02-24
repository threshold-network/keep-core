package tbtcpg_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/ipfs/go-log"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
	"github.com/keep-network/keep-core/pkg/tbtcpg/internal/test"
)

func TestDepositSweepLookBackBlocks(t *testing.T) {
	// The look-back period should be 30 days at 12 seconds per block,
	// matching the value used by MovedFundsSweepLookBackBlocks.
	expectedValue := uint64(216000)

	if tbtcpg.DepositSweepLookBackBlocks != expectedValue {
		t.Errorf(
			"unexpected DepositSweepLookBackBlocks\n"+
				"expected: %d\n"+
				"actual:   %d",
			expectedValue,
			tbtcpg.DepositSweepLookBackBlocks,
		)
	}
}

func TestDepositSweepTask_FindDepositsToSweep_BoundedLookback(t *testing.T) {
	// When the current block (300000) exceeds the look-back window
	// (216000 blocks), the filter start block should be bounded:
	// filterStartBlock = 300000 - 216000 = 84000.
	currentBlock := uint64(300000)
	expectedStartBlock := currentBlock - tbtcpg.DepositSweepLookBackBlocks

	walletPublicKeyHash := hexToByte20(
		"7670343fc00ccc2d0cd65360e6ad400697ea0fed",
	)

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)

	tbtcChain.SetDepositMinAge(3600)

	fundingTxHash := hashFromString(
		"a8c3b3c1975094550d481bdffdee1b7b7613dd74dbce37a5f6dce7fd9ac0ace1",
	)

	tbtcChain.SetDepositRequest(
		fundingTxHash,
		uint32(1),
		&tbtc.DepositChainRequest{
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
		},
	)

	btcChain.SetTransaction(fundingTxHash, &bitcoin.Transaction{})
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	// Register events under the filter with the bounded start block.
	// The production code will query with StartBlock = expectedStartBlock,
	// so the event registration must use the same filter key.
	err := tbtcChain.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          expectedStartBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         290000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)

	deposits, err := task.FindDepositsToSweep(
		&testutils.MockLogger{},
		walletPublicKeyHash,
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(deposits) != 1 {
		t.Fatalf("expected 1 deposit, got %d", len(deposits))
	}

	if deposits[0].FundingTxHash != fundingTxHash {
		t.Errorf("unexpected funding tx hash")
	}

	if deposits[0].FundingOutputIndex != 1 {
		t.Errorf("unexpected funding output index")
	}
}

func TestDepositSweepTask_FindDepositsToSweep_UnderflowGuard(t *testing.T) {
	// When the current block is less than the look-back window, the filter
	// start block should remain 0 to avoid underflow.
	currentBlock := uint64(100000)

	walletPublicKeyHash := hexToByte20(
		"7670343fc00ccc2d0cd65360e6ad400697ea0fed",
	)

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)

	tbtcChain.SetDepositMinAge(3600)

	fundingTxHash := hashFromString(
		"d91868ca43db4deb96047d727a5e782f282864fde2d9364f8c562c8998ba64bf",
	)

	tbtcChain.SetDepositRequest(
		fundingTxHash,
		uint32(1),
		&tbtc.DepositChainRequest{
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
		},
	)

	btcChain.SetTransaction(fundingTxHash, &bitcoin.Transaction{})
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	// Register events under the filter with StartBlock = 0,
	// since the underflow guard should keep the filter at 0.
	err := tbtcChain.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          0,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         90000,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)

	deposits, err := task.FindDepositsToSweep(
		&testutils.MockLogger{},
		walletPublicKeyHash,
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(deposits) != 1 {
		t.Fatalf("expected 1 deposit, got %d", len(deposits))
	}

	if deposits[0].FundingTxHash != fundingTxHash {
		t.Errorf("unexpected funding tx hash")
	}
}

func TestDepositSweepTask_FindDepositsToSweep(t *testing.T) {
	err := log.SetLogLevel("*", "DEBUG")
	if err != nil {
		t.Fatal(err)
	}

	scenarios, err := test.LoadFindDepositsToSweepTestScenario()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			tbtcChain.SetDepositMinAge(scenario.ChainParameters.DepositMinAge)

			// Wire the MockBlockCounter using the scenario's current
			// block value. FindDepositsToSweep requires a block
			// counter to compute the bounded lookback window.
			blockCounter := tbtcpg.NewMockBlockCounter()
			blockCounter.SetCurrentBlock(scenario.ChainParameters.CurrentBlock)
			tbtcChain.SetBlockCounter(blockCounter)

			// Compute the expected filter start block using the same
			// logic as the production code.
			filterStartBlock := uint64(0)
			if scenario.ChainParameters.CurrentBlock > tbtcpg.DepositSweepLookBackBlocks {
				filterStartBlock = scenario.ChainParameters.CurrentBlock - tbtcpg.DepositSweepLookBackBlocks
			}

			// Chain setup.
			for _, deposit := range scenario.Deposits {
				tbtcChain.SetDepositRequest(
					deposit.FundingTxHash,
					deposit.FundingOutputIndex,
					&tbtc.DepositChainRequest{
						RevealedAt: deposit.RevealedAt,
						SweptAt:    deposit.SweptAt,
					},
				)
				btcChain.SetTransaction(deposit.FundingTxHash, deposit.FundingTx)
				btcChain.SetTransactionConfirmations(
					deposit.FundingTxHash,
					deposit.FundingTxConfirmations,
				)

				err := tbtcChain.AddPastDepositRevealedEvent(
					&tbtc.DepositRevealedEventFilter{
						StartBlock:          filterStartBlock,
						WalletPublicKeyHash: [][20]byte{deposit.WalletPublicKeyHash},
					},
					&tbtc.DepositRevealedEvent{
						BlockNumber:         deposit.RevealBlockNumber,
						WalletPublicKeyHash: deposit.WalletPublicKeyHash,
						FundingTxHash:       deposit.FundingTxHash,
						FundingOutputIndex:  deposit.FundingOutputIndex,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)

			// Test execution.
			actualDeposits, err := task.FindDepositsToSweep(
				&testutils.MockLogger{},
				scenario.WalletPublicKeyHash,
				scenario.MaxNumberOfDeposits,
			)

			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(
				scenario.ExpectedUnsweptDeposits,
				actualDeposits,
			); diff != nil {
				t.Errorf("invalid deposits: %v", diff)
			}
		})
	}
}

func TestDepositSweepTask_ProposeDepositsSweep(t *testing.T) {
	err := log.SetLogLevel("*", "DEBUG")
	if err != nil {
		t.Fatal(err)
	}

	scenarios, err := test.LoadProposeSweepTestScenario()
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Title, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			btcChain := tbtcpg.NewLocalBitcoinChain()

			// Chain setup.
			tbtcChain.SetDepositParameters(0, 0, scenario.DepositTxMaxFee, 0)

			for _, deposit := range scenario.Deposits {
				err := tbtcChain.AddPastDepositRevealedEvent(
					&tbtc.DepositRevealedEventFilter{
						StartBlock:          deposit.RevealBlock,
						EndBlock:            &deposit.RevealBlock,
						WalletPublicKeyHash: [][20]byte{scenario.WalletPublicKeyHash},
					},
					&tbtc.DepositRevealedEvent{
						WalletPublicKeyHash: scenario.WalletPublicKeyHash,
						FundingTxHash:       deposit.FundingTxHash,
						FundingOutputIndex:  deposit.FundingOutputIndex,
					},
				)
				if err != nil {
					t.Fatal(err)
				}

				tbtcChain.SetDepositRequest(
					deposit.FundingTxHash,
					deposit.FundingOutputIndex,
					&tbtc.DepositChainRequest{
						// Set only relevant fields.
						ExtraData: nil,
					},
				)

				btcChain.SetTransaction(deposit.FundingTxHash, &bitcoin.Transaction{})
				btcChain.SetTransactionConfirmations(deposit.FundingTxHash, tbtc.DepositSweepRequiredFundingTxConfirmations)
			}

			if scenario.ExpectedDepositSweepProposal != nil {
				err := tbtcChain.SetDepositSweepProposalValidationResult(
					scenario.WalletPublicKeyHash,
					scenario.ExpectedDepositSweepProposal,
					nil,
					true,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			btcChain.SetEstimateSatPerVByteFee(1, scenario.EstimateSatPerVByteFee)

			task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)

			// Test execution.
			proposal, err := task.ProposeDepositsSweep(
				&testutils.MockLogger{},
				scenario.WalletPublicKeyHash,
				scenario.DepositsReferences(),
				scenario.SweepTxFee,
			)

			if !reflect.DeepEqual(scenario.ExpectedErr, err) {
				t.Errorf(
					"unexpected error\n"+
						"expected: [%+v]\n"+
						"actual:   [%+v]",
					scenario.ExpectedErr,
					err,
				)
			}

			var actualDepositSweepProposals []*tbtc.DepositSweepProposal
			if proposal != nil {
				actualDepositSweepProposals = append(actualDepositSweepProposals, proposal)
			}

			var expectedDepositSweepProposals []*tbtc.DepositSweepProposal
			if p := scenario.ExpectedDepositSweepProposal; p != nil {
				expectedDepositSweepProposals = append(expectedDepositSweepProposals, p)
			}

			if diff := deep.Equal(
				actualDepositSweepProposals,
				expectedDepositSweepProposals,
			); diff != nil {
				t.Errorf("invalid deposit sweep proposal: %v", diff)
			}
		})
	}
}

// setupVaultGroupingDeposit registers a single deposit in the mock chains
// with the given vault and returns the funding tx hash used.
func setupVaultGroupingDeposit(
	t *testing.T,
	tbtcChain *tbtcpg.LocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
	walletPublicKeyHash [20]byte,
	filterStartBlock uint64,
	fundingTxHashHex string,
	outputIndex uint32,
	blockNumber uint64,
	vault *chain.Address,
) bitcoin.Hash {
	t.Helper()

	fundingTxHash := hashFromString(fundingTxHashHex)

	tbtcChain.SetDepositRequest(
		fundingTxHash,
		outputIndex,
		&tbtc.DepositChainRequest{
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      vault,
		},
	)

	btcChain.SetTransaction(fundingTxHash, &bitcoin.Transaction{})
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	err := tbtcChain.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         blockNumber,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  outputIndex,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	return fundingTxHash
}

func TestFindDepositsToSweep_VaultGrouping(t *testing.T) {
	currentBlock := uint64(300000)
	filterStartBlock := currentBlock - tbtcpg.DepositSweepLookBackBlocks
	walletPublicKeyHash := hexToByte20(
		"7670343fc00ccc2d0cd65360e6ad400697ea0fed",
	)

	t.Run("all nil vaults form single group", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		hash1 := setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f01",
			0, 290000, nil,
		)
		hash2 := setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"b2c3d4e5f6071829304b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f0102",
			0, 290001, nil,
		)
		hash3 := setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c3d4e5f607182930415c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f010203",
			0, 290002, nil,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 3 {
			t.Fatalf("expected 3 deposits, got %d", len(deposits))
		}

		expectedHashes := map[bitcoin.Hash]bool{
			hash1: true, hash2: true, hash3: true,
		}
		for _, d := range deposits {
			if !expectedHashes[d.FundingTxHash] {
				t.Errorf("unexpected funding tx hash: %v", d.FundingTxHash)
			}
		}
	})

	t.Run("mixed vaults largest group selected", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		// 3 deposits with vaultA (largest group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"1111111111111111111111111111111111111111111111111111111111111111",
			0, 290000, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"2222222222222222222222222222222222222222222222222222222222222222",
			0, 290001, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"3333333333333333333333333333333333333333333333333333333333333333",
			0, 290002, &vaultA,
		)

		// 2 deposits with nil vault (minority group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"4444444444444444444444444444444444444444444444444444444444444444",
			0, 290003, nil,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"5555555555555555555555555555555555555555555555555555555555555555",
			0, 290004, nil,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		// Only the 3 vaultA deposits should be returned.
		if len(deposits) != 3 {
			t.Fatalf("expected 3 deposits from largest vault group, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault == nil {
				t.Errorf("expected non-nil vault for selected deposit")
			} else if *d.Vault != vaultA {
				t.Errorf(
					"expected vault %s, got %s",
					string(vaultA),
					string(*d.Vault),
				)
			}
		}
	})

	t.Run("mixed vaults minority group excluded", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		// 3 deposits with vaultA (largest group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"1111111111111111111111111111111111111111111111111111111111111111",
			0, 290000, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"2222222222222222222222222222222222222222222222222222222222222222",
			0, 290001, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"3333333333333333333333333333333333333333333333333333333333333333",
			0, 290002, &vaultA,
		)

		// 2 nil vault deposits (minority group).
		nilHash1 := setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"4444444444444444444444444444444444444444444444444444444444444444",
			0, 290003, nil,
		)
		nilHash2 := setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"5555555555555555555555555555555555555555555555555555555555555555",
			0, 290004, nil,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		// Verify nil vault deposits are NOT in the results.
		for _, d := range deposits {
			if d.FundingTxHash == nilHash1 || d.FundingTxHash == nilHash2 {
				t.Errorf(
					"nil vault deposit %v should have been excluded",
					d.FundingTxHash,
				)
			}
		}
	})

	t.Run("vault address case normalization", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		// Same vault address in different cases; should be treated
		// as the same group after normalization.
		vaultUpper := chain.Address("0xAbCdEf0011223344556677889900AaBbCcDdEeFf")
		vaultLower := chain.Address("0xabcdef0011223344556677889900aabbccddeeff")

		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"aa11111111111111111111111111111111111111111111111111111111111111",
			0, 290000, &vaultUpper,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"bb22222222222222222222222222222222222222222222222222222222222222",
			0, 290001, &vaultLower,
		)

		// 1 nil vault deposit (minority group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"cc33333333333333333333333333333333333333333333333333333333333333",
			0, 290002, nil,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		// The two vault deposits should form a single group (case-insensitive
		// normalization), making it the largest group (2 vs 1).
		if len(deposits) != 2 {
			t.Fatalf("expected 2 deposits from case-normalized vault group, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault == nil {
				t.Errorf("expected non-nil vault for selected deposit")
			}
		}
	})

	t.Run("nil vault is separate group from named vault", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		// 3 nil vault deposits (largest group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"dd11111111111111111111111111111111111111111111111111111111111111",
			0, 290000, nil,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"dd22222222222222222222222222222222222222222222222222222222222222",
			0, 290001, nil,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"dd33333333333333333333333333333333333333333333333333333333333333",
			0, 290002, nil,
		)

		// 1 deposit with named vault (minority group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"dd44444444444444444444444444444444444444444444444444444444444444",
			0, 290003, &vaultA,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		// Only the 3 nil vault deposits should be returned.
		if len(deposits) != 3 {
			t.Fatalf("expected 3 nil-vault deposits, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault != nil {
				t.Errorf("expected nil vault for selected deposit, got %v", *d.Vault)
			}
		}
	})

	t.Run("single deposit with vault", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"ee11111111111111111111111111111111111111111111111111111111111111",
			0, 290000, &vaultA,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 1 {
			t.Fatalf("expected 1 deposit, got %d", len(deposits))
		}

		if deposits[0].Vault == nil {
			t.Errorf("expected non-nil vault for the single deposit")
		} else if *deposits[0].Vault != vaultA {
			t.Errorf(
				"expected vault %s, got %s",
				string(vaultA),
				string(*deposits[0].Vault),
			)
		}
	})

	t.Run("tied vault groups deterministic selection", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		// 2 nil vault deposits.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"ff11111111111111111111111111111111111111111111111111111111111111",
			0, 290000, nil,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"ff22222222222222222222222222222222222222222222222222222222222222",
			0, 290001, nil,
		)

		// 2 deposits with vaultA.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"ff33333333333333333333333333333333333333333333333333333333333333",
			0, 290002, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"ff44444444444444444444444444444444444444444444444444444444444444",
			0, 290003, &vaultA,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		// Exactly one complete group should be selected (2 deposits).
		if len(deposits) != 2 {
			t.Fatalf("expected 2 deposits from one tied group, got %d", len(deposits))
		}

		// All returned deposits must belong to the same vault group:
		// either all nil or all the same non-nil vault.
		allNil := true
		allVaultA := true
		for _, d := range deposits {
			if d.Vault != nil {
				allNil = false
			}
			if d.Vault == nil || *d.Vault != vaultA {
				allVaultA = false
			}
		}
		if !allNil && !allVaultA {
			t.Errorf("deposits are from mixed vault groups; expected a single group")
		}
	})

	t.Run("8 deposits 5 TBTCVault 2 nil 1 other returns 5 TBTCVault", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		tbtcVault := chain.Address("0xTBTCVaultAddress1234567890abcdef12345678")
		otherVault := chain.Address("0xOtherVaultAddr1234567890abcdef1234567890")

		// 5 deposits with TBTCVault (largest group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a100000000000000000000000000000000000000000000000000000000000001",
			0, 290000, &tbtcVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a200000000000000000000000000000000000000000000000000000000000002",
			0, 290001, &tbtcVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a300000000000000000000000000000000000000000000000000000000000003",
			0, 290002, &tbtcVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a400000000000000000000000000000000000000000000000000000000000004",
			0, 290003, &tbtcVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a500000000000000000000000000000000000000000000000000000000000005",
			0, 290004, &tbtcVault,
		)

		// 2 deposits with nil vault.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a600000000000000000000000000000000000000000000000000000000000006",
			0, 290005, nil,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a700000000000000000000000000000000000000000000000000000000000007",
			0, 290006, nil,
		)

		// 1 deposit with a different vault.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"a800000000000000000000000000000000000000000000000000000000000008",
			0, 290007, &otherVault,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			20,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 5 {
			t.Fatalf("expected 5 TBTCVault deposits, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault == nil {
				t.Errorf("expected non-nil vault for selected deposit")
			} else if *d.Vault != tbtcVault {
				t.Errorf(
					"expected vault %s, got %s",
					string(tbtcVault),
					string(*d.Vault),
				)
			}
		}
	})

	t.Run("4 same vault deposits all returned", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		singleVault := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")

		// 4 deposits all targeting the same vault.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"b100000000000000000000000000000000000000000000000000000000000001",
			0, 290000, &singleVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"b200000000000000000000000000000000000000000000000000000000000002",
			0, 290001, &singleVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"b300000000000000000000000000000000000000000000000000000000000003",
			0, 290002, &singleVault,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"b400000000000000000000000000000000000000000000000000000000000004",
			0, 290003, &singleVault,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 4 {
			t.Fatalf("expected 4 deposits from single vault group, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault == nil {
				t.Errorf("expected non-nil vault for selected deposit")
			} else if *d.Vault != singleVault {
				t.Errorf(
					"expected vault %s, got %s",
					string(singleVault),
					string(*d.Vault),
				)
			}
		}
	})

	t.Run("3 vault groups returns largest", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		vaultA := chain.Address("0xAA1122BB3344CC5566DD7788EE9900FF00112233")
		vaultB := chain.Address("0xBB2233CC4455DD6677EE8899FF00AA1122334455")
		vaultC := chain.Address("0xCC3344DD5566EE7788FF9900AA1122BB33445566")

		// 3 deposits with vaultA (largest group).
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c100000000000000000000000000000000000000000000000000000000000001",
			0, 290000, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c200000000000000000000000000000000000000000000000000000000000002",
			0, 290001, &vaultA,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c300000000000000000000000000000000000000000000000000000000000003",
			0, 290002, &vaultA,
		)

		// 2 deposits with vaultB.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c400000000000000000000000000000000000000000000000000000000000004",
			0, 290003, &vaultB,
		)
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c500000000000000000000000000000000000000000000000000000000000005",
			0, 290004, &vaultB,
		)

		// 1 deposit with vaultC.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"c600000000000000000000000000000000000000000000000000000000000006",
			0, 290005, &vaultC,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			20,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 3 {
			t.Fatalf("expected 3 deposits from vaultA group, got %d", len(deposits))
		}

		for _, d := range deposits {
			if d.Vault == nil {
				t.Errorf("expected non-nil vault for selected deposit")
			} else if *d.Vault != vaultA {
				t.Errorf(
					"expected vault %s, got %s",
					string(vaultA),
					string(*d.Vault),
				)
			}
		}
	})

	t.Run("post-reinitializer vault from chain request", func(t *testing.T) {
		tbtcChain := tbtcpg.NewLocalChain()
		btcChain := tbtcpg.NewLocalBitcoinChain()

		blockCounter := tbtcpg.NewMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		tbtcChain.SetBlockCounter(blockCounter)
		tbtcChain.SetDepositMinAge(3600)

		tbtcVault := chain.Address("0xTBTCVaultAddress1234567890abcdef12345678")

		// Register a deposit where the chain request has a vault set
		// (simulating a reinitializer having assigned the vault after
		// the original deposit reveal). The setupVaultGroupingDeposit
		// helper passes vault to DepositChainRequest.Vault which is
		// the field used by findDeposits() for building the Deposit
		// object.
		setupVaultGroupingDeposit(
			t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
			"d100000000000000000000000000000000000000000000000000000000000001",
			0, 290000, &tbtcVault,
		)

		task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
		deposits, err := task.FindDepositsToSweep(
			&testutils.MockLogger{},
			walletPublicKeyHash,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(deposits) != 1 {
			t.Fatalf("expected 1 deposit, got %d", len(deposits))
		}

		if deposits[0].Vault == nil {
			t.Fatal("expected non-nil vault from chain request after reinitializer")
		}

		if *deposits[0].Vault != tbtcVault {
			t.Errorf(
				"expected vault %s from chain request, got %s",
				string(tbtcVault),
				string(*deposits[0].Vault),
			)
		}
	})
}
