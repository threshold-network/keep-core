package tbtcpg_test

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// setupReleasedDeposit models a reserved deposit that was released back to the
// pool via the router's release-to-pool entry: the DepositRevealed event still
// carries the reservation vault (it is immutable), but the stored deposit
// request's vault has been re-routed to the release target and the reserved
// mark has been cleared (IsReservedDeposit reports false).
func setupReleasedDeposit(
	t *testing.T,
	tbtcChain *tbtcpg.LocalChain,
	btcChain *tbtcpg.LocalBitcoinChain,
	walletPublicKeyHash [20]byte,
	filterStartBlock uint64,
	fundingTxHashHex string,
	outputIndex uint32,
	blockNumber uint64,
	reservationVault chain.Address,
	targetVault chain.Address,
) bitcoin.Hash {
	t.Helper()

	fundingTxHash := hashFromString(fundingTxHashHex)

	// The stored deposit request is what findDeposits and the proposal read
	// after release: its vault is the release target, not the reservation
	// vault.
	tbtcChain.SetDepositRequest(
		fundingTxHash,
		outputIndex,
		&tbtc.DepositChainRequest{
			RevealedAt: time.Now().Add(-2 * time.Hour),
			SweptAt:    time.Unix(0, 0),
			Vault:      &targetVault,
		},
	)

	btcChain.SetTransaction(fundingTxHash, &bitcoin.Transaction{})
	btcChain.SetTransactionConfirmations(
		fundingTxHash,
		tbtc.DepositSweepRequiredFundingTxConfirmations,
	)

	// The reveal event is immutable and still names the reservation vault,
	// so the cheap event-vault pre-filter flags this deposit as a
	// reservation candidate and consults IsReservedDeposit.
	if err := tbtcChain.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			BlockNumber:         blockNumber,
			WalletPublicKeyHash: walletPublicKeyHash,
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  outputIndex,
			Vault:               &reservationVault,
		},
	); err != nil {
		t.Fatal(err)
	}

	// Released: the reserved mark is cleared, so the chain reports it as
	// not reserved even though its reveal event targeted the reservation
	// vault.
	tbtcChain.SetReservedDeposit(
		tbtcChain.BuildDepositKey(fundingTxHash, outputIndex), false,
	)

	return fundingTxHash
}

// TestFindDepositsToSweep_IncludesReleasedDeposit verifies the leader's
// deposit sweep picks up a reserved deposit that was released back to the
// pool - one whose reveal event still names the reservation vault but whose
// stored deposit request has been re-routed to the release target and whose
// reserved mark has been cleared - and that it proposes that deposit with the
// stored (target) vault rather than the immutable event vault. A released
// deposit must not strand until its refund locktime: an ordinary pool sweep
// is exactly what release-to-pool restores.
func TestFindDepositsToSweep_IncludesReleasedDeposit(t *testing.T) {
	currentBlock := uint64(300000)
	filterStartBlock := currentBlock - tbtcpg.DepositSweepLookBackBlocks
	walletPublicKeyHash := hexToByte20(
		"7670343fc00ccc2d0cd65360e6ad400697ea0fed",
	)

	tbtcChain := tbtcpg.NewLocalChain()
	btcChain := tbtcpg.NewLocalBitcoinChain()

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	tbtcChain.SetBlockCounter(blockCounter)
	tbtcChain.SetDepositMinAge(3600)

	reservationVault := testReservationVaultAddress
	tbtcChain.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault: reservationVault,
	})

	targetVault := chain.Address(
		"0xPoolReleaseTargetVault1234567890abcdef1234567",
	)

	releasedHash := setupReleasedDeposit(
		t, tbtcChain, btcChain, walletPublicKeyHash, filterStartBlock,
		"5555555555555555555555555555555555555555555555555555555555555555",
		0, 290000, reservationVault, targetVault,
	)

	task := tbtcpg.NewDepositSweepTask(tbtcChain, btcChain)
	deposits, err := task.FindDepositsToSweep(
		&testutils.MockLogger{},
		walletPublicKeyHash,
		10,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(deposits) != 1 {
		t.Fatalf(
			"expected exactly 1 deposit (the released one), got %d",
			len(deposits),
		)
	}
	if deposits[0].FundingTxHash != releasedHash {
		t.Fatalf("expected the released deposit, got %v", deposits[0].FundingTxHash)
	}
	// The proposal must carry the stored release-target vault, not the
	// immutable reveal-event reservation vault: an ordinary pool sweep of a
	// released deposit settles it into the target, which is only valid when
	// the proposal - and the on-chain validation that follows it - route the
	// deposit through the stored vault. targetVault and reservationVault are
	// distinct constants, so the stored-vault equality check below also
	// proves the deposit was not re-routed back into the reservation vault.
	if deposits[0].Vault == nil || *deposits[0].Vault != targetVault {
		t.Errorf(
			"expected the proposal to carry the stored vault %v, got %v",
			targetVault,
			deposits[0].Vault,
		)
	}
}
