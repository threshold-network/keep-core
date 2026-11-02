package tbtc

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc/internal/test"
)

// TestValidateDepositSweepProposal_AcceptsReleasedDeposit is the inverse of
// TestValidateDepositSweepProposal_RejectsReservedDeposit: a deposit whose
// reveal event names the reservation vault but whose reserved mark has been
// cleared - a reserved deposit released back to the pool - must be accepted
// by the follower. The follower's defense-in-depth reservation check gates on
// the live IsReservedDeposit state, not the immutable reveal-event vault, so
// a released deposit passes it and the proposal is swept through the stored
// vault. This locks the released-deposit sweep from the follower side: a
// correctly proposed pool sweep of a released deposit must not be rejected.
func TestValidateDepositSweepProposal_AcceptsReleasedDeposit(t *testing.T) {
	scenarios, err := test.LoadDepositSweepTestScenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no deposit sweep test scenarios available")
	}
	scenario := scenarios[0]
	if len(scenario.Deposits) == 0 {
		t.Fatal("scenario has no deposits to release")
	}

	hostChain := Connect()
	bitcoinChain := newLocalBitcoinChain()

	wallet := wallet{publicKey: scenario.WalletPublicKey}
	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

	for _, transaction := range scenario.InputTransactions {
		if err := bitcoinChain.BroadcastTransaction(transaction); err != nil {
			t.Fatal(err)
		}
	}

	depositsKeys := make([]DepositKey, len(scenario.Deposits))
	depositsExtraInfo := make([]struct {
		*Deposit
		FundingTx *bitcoin.Transaction
	}, len(scenario.Deposits))
	depositsRevealBlocks := make([]*big.Int, len(scenario.Deposits))

	reservationVault := chain.Address("0xReservationVaultForFollowerTest0000001")
	// The release target: the stored deposit request is re-routed here by
	// release-to-pool, distinct from the reservation vault.
	releaseTargetVault := chain.Address(
		"0xPoolReleaseTargetVaultForFollowerTest0001",
	)

	for i, deposit := range scenario.Deposits {
		fundingTxHash := deposit.Utxo.Outpoint.TransactionHash
		fundingOutputIndex := deposit.Utxo.Outpoint.OutputIndex

		fundingTx, err := bitcoinChain.GetTransaction(fundingTxHash)
		if err != nil {
			t.Fatal(err)
		}

		depositsKeys[i] = DepositKey{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
		}
		depositsExtraInfo[i] = struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		}{
			Deposit:   (*Deposit)(deposit),
			FundingTx: fundingTx,
		}

		depositRevealBlock := uint64(100 * i)
		depositsRevealBlocks[i] = big.NewInt(int64(depositRevealBlock))

		// The first deposit is modeled as released: its reveal event still
		// names the reservation vault (immutable), but its stored deposit
		// request is re-routed to the release target and its reserved mark is
		// cleared. Other deposits keep their scenario-fixture vault, which
		// does not match the reservation vault, so they sweep unaffected.
		eventVault := deposit.Vault
		storedVault := deposit.Vault
		if i == 0 {
			eventVault = &reservationVault
			storedVault = &releaseTargetVault
		}

		err = hostChain.setPastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				StartBlock:          depositRevealBlock,
				EndBlock:            &depositRevealBlock,
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
			},
			[]*DepositRevealedEvent{
				{
					FundingTxHash:       fundingTxHash,
					FundingOutputIndex:  fundingOutputIndex,
					Depositor:           deposit.Depositor,
					Amount:              uint64(deposit.Utxo.Value),
					BlindingFactor:      deposit.BlindingFactor,
					WalletPublicKeyHash: deposit.WalletPublicKeyHash,
					RefundPublicKeyHash: deposit.RefundPublicKeyHash,
					RefundLocktime:      deposit.RefundLocktime,
					Vault:               eventVault,
					BlockNumber:         depositRevealBlock,
				},
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		hostChain.setDepositRequest(
			fundingTxHash,
			fundingOutputIndex,
			&DepositChainRequest{
				Depositor: deposit.Depositor,
				Amount:    uint64(deposit.Utxo.Value),
				Vault:     storedVault,
				ExtraData: deposit.ExtraData,
			},
		)
	}

	proposal := &DepositSweepProposal{
		DepositsKeys:         depositsKeys,
		SweepTxFee:           big.NewInt(scenario.Fee),
		DepositsRevealBlocks: depositsRevealBlocks,
	}

	if err := hostChain.setDepositSweepProposalValidationResult(
		walletPublicKeyHash,
		proposal,
		depositsExtraInfo,
		true,
	); err != nil {
		t.Fatal(err)
	}

	// The first deposit's reveal event targets the reservation vault, so the
	// pre-filter consults IsReservedDeposit for it. It reports the deposit
	// as NOT reserved (it was released), so the follower must accept rather
	// than reject the proposal.
	releasedKey := hostChain.BuildDepositKey(
		depositsKeys[0].FundingTxHash,
		depositsKeys[0].FundingOutputIndex,
	)
	hostChain.setReservedDeposit(releasedKey, false)
	hostChain.setReservationParameters(&ReservationParameters{
		ReservationVault: reservationVault,
	})

	_, err = ValidateDepositSweepProposal(
		&capturingLogger{},
		walletPublicKeyHash,
		proposal,
		0,
		true,
		hostChain,
		bitcoinChain,
	)
	if err != nil {
		t.Fatalf(
			"expected the follower to accept a proposal sweeping a "+
				"released deposit, got: [%v]",
			err,
		)
	}
}
