package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// archiveClosedWallets archives closed or terminated wallets.
func (n *node) archiveClosedWallets() error {
	// Get all the wallets controlled by the node.
	walletPublicKeys := n.walletRegistry.getWalletsPublicKeys()

	for _, walletPublicKey := range walletPublicKeys {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

		walletID, err := n.chain.CalculateWalletID(walletPublicKey)
		if err != nil {
			return fmt.Errorf(
				"could not calculate wallet ID for wallet with public key "+
					"hash [0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}

		isRegistered, err := n.chain.IsWalletRegistered(walletID)
		if err != nil {
			return fmt.Errorf(
				"could not check if wallet is registered for wallet with ID "+
					"[0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}

		if !isRegistered {
			// If the wallet is no longer registered it means the wallet has
			// been closed or terminated.
			err := n.walletRegistry.archiveWallet(walletPublicKeyHash)
			if err != nil {
				return fmt.Errorf(
					"could not archive wallet with public key hash [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
			}

			logger.Infof(
				"successfully archived wallet with ID [0x%x] and public key "+
					"hash [0x%x]",
				walletID,
				walletPublicKeyHash,
			)
		}
	}

	return nil
}

// handleWalletClosure handles the wallet termination or closing process.
func (n *node) handleWalletClosure(walletID [32]byte) error {
	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// To verify there was no chain reorg and the wallet is really closed check
	// if it is registered. Both terminated and closed wallets are removed
	// from the ECDSA registry.
	stateCheck := func() (bool, error) {
		isRegistered, err := n.chain.IsWalletRegistered(walletID)
		if err != nil {
			return false, err
		}

		return !isRegistered, nil
	}

	// Wait a significant number of blocks to make sure the transaction has not
	// been reverted for some reason, e.g. due to a chain reorganization.
	result, err := ethereum.WaitForBlockConfirmations(
		blockCounter,
		currentBlock,
		walletClosureConfirmationBlocks,
		stateCheck,
	)
	if err != nil {
		return fmt.Errorf(
			"error while waiting for wallet closure confirmation [%w]",
			err,
		)
	}

	if !result {
		return fmt.Errorf("wallet closure not confirmed")
	}

	wallet, ok := n.walletRegistry.getWalletByID(walletID)
	if !ok {
		// Wallet was not found in the registry. The wallet is not controlled by
		// this node.
		logger.Infof(
			"node does not control wallet with ID [0x%x]; quitting wallet "+
				"archiving",
			walletID,
		)
		return nil
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

	err = n.walletRegistry.archiveWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("failed to archive the wallet: [%v]", err)
	}

	logger.Infof(
		"successfully archived wallet with wallet ID [0x%x] and public key "+
			"hash [0x%x]",
		walletID,
		walletPublicKeyHash,
	)

	return nil
}
