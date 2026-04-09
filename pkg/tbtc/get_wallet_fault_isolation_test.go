package tbtc

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
)

// TestGetWalletReturnsDataWhenRegistryFails verifies that GetWallet returns
// valid Bridge fields with a zero-valued MembersIDsHash when the wallet
// registry is unavailable. This validates that downstream callers relying
// only on Bridge data (State, timestamps, etc.) are not disrupted by a
// transient registry failure.
func TestGetWalletReturnsDataWhenRegistryFails(t *testing.T) {
	chain := Connect()

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	walletID := [32]byte{0xaa, 0xbb, 0xcc}
	mainUtxoHash := sha256.Sum256([]byte("main-utxo"))
	createdAt := time.Unix(1700000000, 0)
	movingFundsRequestedAt := time.Unix(1700001000, 0)
	closingStartedAt := time.Unix(1700002000, 0)

	chain.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID:                       walletID,
		MembersIDsHash:                      [32]byte{}, // zero -- simulating registry unavailable
		MainUtxoHash:                        mainUtxoHash,
		PendingRedemptionsValue:             500000,
		CreatedAt:                           createdAt,
		MovingFundsRequestedAt:              movingFundsRequestedAt,
		ClosingStartedAt:                    closingStartedAt,
		PendingMovedFundsSweepRequestsCount: 3,
		State:                               StateLive,
		MovingFundsTargetWalletsCommitmentHash: sha256.Sum256(
			[]byte("commitment"),
		),
	})

	// Simulate a wallet registry error so the mock degrades
	// gracefully, returning Bridge-only data with zero MembersIDsHash.
	chain.setWalletRegistryErr(walletPublicKeyHash, fmt.Errorf(
		"rpc: wallet registry unavailable",
	))

	walletData, err := chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		t.Fatalf(
			"GetWallet should not return error on registry failure; got: [%v]",
			err,
		)
	}

	if walletData == nil {
		t.Fatal("GetWallet should return non-nil WalletChainData on registry failure")
	}

	// Verify MembersIDsHash is zero when registry is unavailable.
	if walletData.MembersIDsHash != ([32]byte{}) {
		t.Errorf(
			"unexpected MembersIDsHash\nexpected: zero [32]byte\nactual:   [0x%x]",
			walletData.MembersIDsHash,
		)
	}

	// Verify Bridge-sourced fields are fully populated.
	if walletData.EcdsaWalletID != walletID {
		t.Errorf(
			"unexpected EcdsaWalletID\nexpected: [0x%x]\nactual:   [0x%x]",
			walletID,
			walletData.EcdsaWalletID,
		)
	}

	if walletData.MainUtxoHash != mainUtxoHash {
		t.Errorf(
			"unexpected MainUtxoHash\nexpected: [0x%x]\nactual:   [0x%x]",
			mainUtxoHash,
			walletData.MainUtxoHash,
		)
	}

	testutils.AssertUintsEqual(
		t,
		"PendingRedemptionsValue",
		500000,
		walletData.PendingRedemptionsValue,
	)

	if !walletData.CreatedAt.Equal(createdAt) {
		t.Errorf(
			"unexpected CreatedAt\nexpected: [%v]\nactual:   [%v]",
			createdAt,
			walletData.CreatedAt,
		)
	}

	if walletData.State != StateLive {
		t.Errorf(
			"unexpected State\nexpected: [%v]\nactual:   [%v]",
			StateLive,
			walletData.State,
		)
	}

	if !walletData.MovingFundsRequestedAt.Equal(movingFundsRequestedAt) {
		t.Errorf(
			"unexpected MovingFundsRequestedAt\nexpected: [%v]\nactual:   [%v]",
			movingFundsRequestedAt,
			walletData.MovingFundsRequestedAt,
		)
	}

	if !walletData.ClosingStartedAt.Equal(closingStartedAt) {
		t.Errorf(
			"unexpected ClosingStartedAt\nexpected: [%v]\nactual:   [%v]",
			closingStartedAt,
			walletData.ClosingStartedAt,
		)
	}

	testutils.AssertUintsEqual(
		t,
		"PendingMovedFundsSweepRequestsCount",
		3,
		uint64(walletData.PendingMovedFundsSweepRequestsCount),
	)

	commitmentHash := sha256.Sum256([]byte("commitment"))
	if walletData.MovingFundsTargetWalletsCommitmentHash != commitmentHash {
		t.Errorf(
			"unexpected MovingFundsTargetWalletsCommitmentHash\n"+
				"expected: [0x%x]\nactual:   [0x%x]",
			commitmentHash,
			walletData.MovingFundsTargetWalletsCommitmentHash,
		)
	}
}

// TestGetWalletReturnsFullDataWhenRegistrySucceeds verifies that GetWallet
// returns complete data including a non-zero MembersIDsHash when both the
// Bridge and wallet registry calls succeed. This is the baseline behavior
// that must be preserved after introducing fault isolation.
func TestGetWalletReturnsFullDataWhenRegistrySucceeds(t *testing.T) {
	chain := Connect()

	walletPublicKeyHash := [20]byte{21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
		31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	walletID := [32]byte{0xdd, 0xee, 0xff}
	membersIDsHash := sha256.Sum256([]byte("test-members-ids"))
	mainUtxoHash := sha256.Sum256([]byte("main-utxo-success"))
	createdAt := time.Unix(1700000000, 0)
	movingFundsRequestedAt := time.Unix(1700003000, 0)
	closingStartedAt := time.Unix(1700004000, 0)
	commitmentHash := sha256.Sum256([]byte("commitment-success"))

	chain.setWallet(walletPublicKeyHash, &WalletChainData{
		EcdsaWalletID:                          walletID,
		MembersIDsHash:                         membersIDsHash,
		MainUtxoHash:                           mainUtxoHash,
		PendingRedemptionsValue:                1000000,
		CreatedAt:                              createdAt,
		MovingFundsRequestedAt:                 movingFundsRequestedAt,
		ClosingStartedAt:                       closingStartedAt,
		PendingMovedFundsSweepRequestsCount:    7,
		State:                                  StateMovingFunds,
		MovingFundsTargetWalletsCommitmentHash: commitmentHash,
	})

	walletData, err := chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		t.Fatalf("GetWallet should not return error; got: [%v]", err)
	}

	if walletData == nil {
		t.Fatal("GetWallet should return non-nil WalletChainData")
	}

	// Verify MembersIDsHash is the expected non-zero value.
	if walletData.MembersIDsHash != membersIDsHash {
		t.Errorf(
			"unexpected MembersIDsHash\nexpected: [0x%x]\nactual:   [0x%x]",
			membersIDsHash,
			walletData.MembersIDsHash,
		)
	}

	if walletData.EcdsaWalletID != walletID {
		t.Errorf(
			"unexpected EcdsaWalletID\nexpected: [0x%x]\nactual:   [0x%x]",
			walletID,
			walletData.EcdsaWalletID,
		)
	}

	if walletData.MainUtxoHash != mainUtxoHash {
		t.Errorf(
			"unexpected MainUtxoHash\nexpected: [0x%x]\nactual:   [0x%x]",
			mainUtxoHash,
			walletData.MainUtxoHash,
		)
	}

	testutils.AssertUintsEqual(
		t,
		"PendingRedemptionsValue",
		1000000,
		walletData.PendingRedemptionsValue,
	)

	if walletData.State != StateMovingFunds {
		t.Errorf(
			"unexpected State\nexpected: [%v]\nactual:   [%v]",
			StateMovingFunds,
			walletData.State,
		)
	}

	if !walletData.CreatedAt.Equal(createdAt) {
		t.Errorf(
			"unexpected CreatedAt\nexpected: [%v]\nactual:   [%v]",
			createdAt,
			walletData.CreatedAt,
		)
	}

	if !walletData.MovingFundsRequestedAt.Equal(movingFundsRequestedAt) {
		t.Errorf(
			"unexpected MovingFundsRequestedAt\nexpected: [%v]\nactual:   [%v]",
			movingFundsRequestedAt,
			walletData.MovingFundsRequestedAt,
		)
	}

	if !walletData.ClosingStartedAt.Equal(closingStartedAt) {
		t.Errorf(
			"unexpected ClosingStartedAt\nexpected: [%v]\nactual:   [%v]",
			closingStartedAt,
			walletData.ClosingStartedAt,
		)
	}

	testutils.AssertUintsEqual(
		t,
		"PendingMovedFundsSweepRequestsCount",
		7,
		uint64(walletData.PendingMovedFundsSweepRequestsCount),
	)

	if walletData.MovingFundsTargetWalletsCommitmentHash != commitmentHash {
		t.Errorf(
			"unexpected MovingFundsTargetWalletsCommitmentHash\n"+
				"expected: [0x%x]\nactual:   [0x%x]",
			commitmentHash,
			walletData.MovingFundsTargetWalletsCommitmentHash,
		)
	}
}

// TestGetWalletBridgeFailureStillReturnsError verifies that GetWallet
// continues to return an error when the wallet is not found (Bridge-level
// failure). The fault isolation change must NOT alter the behavior for
// Bridge failures.
func TestGetWalletBridgeFailureStillReturnsError(t *testing.T) {
	chain := Connect()

	unknownPKH := [20]byte{99, 99, 99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99, 99, 99}

	walletData, err := chain.GetWallet(unknownPKH)

	if err == nil {
		t.Fatal("GetWallet should return error for unknown wallet")
	}

	if walletData != nil {
		t.Errorf(
			"GetWallet should return nil data for unknown wallet; got: [%+v]",
			walletData,
		)
	}
}
