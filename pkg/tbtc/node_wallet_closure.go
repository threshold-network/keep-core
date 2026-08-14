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

		var walletID [32]byte
		var archiveWallet bool

		walletChainData, err := n.chain.GetWallet(walletPublicKeyHash)
		if err != nil {
			walletID, err = n.chain.CalculateWalletID(walletPublicKey)
			if err != nil {
				return fmt.Errorf(
					"could not resolve wallet IDs for wallet with public key "+
						"hash [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
			}

			// Legacy fallback for deployments where Bridge wallet state is
			// unavailable. FROST wallets are registered in the Bridge but not
			// in the legacy ECDSA wallet registry.
			isRegistered, err := n.chain.IsWalletRegistered(walletID)
			if err != nil {
				return fmt.Errorf(
					"could not check if wallet is registered for wallet with ECDSA ID "+
						"[0x%x]: [%v]",
					walletID,
					err,
				)
			}

			if !isRegistered && n.frostWalletRegistryAvailable() {
				logger.Infof(
					"wallet with ECDSA ID [0x%x] and public key hash [0x%x] "+
						"was not found in Bridge or the legacy ECDSA registry; "+
						"preserving local key material because FROST wallet "+
						"registration is available and the wallet may be "+
						"pending Bridge registration",
					walletID,
					walletPublicKeyHash,
				)
				continue
			}

			archiveWallet = !isRegistered
		} else {
			walletID = walletChainData.WalletID
			if walletID == [32]byte{} {
				walletID = DeriveLegacyWalletID(walletPublicKeyHash)
			}

			archiveWallet = walletChainData.State == StateClosed ||
				walletChainData.State == StateTerminated
		}

		if archiveWallet {
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

type frostWalletRegistryAvailability interface {
	FrostWalletRegistryAvailable() bool
}

type frostWalletRegistrationChain interface {
	IsFrostWalletRegistered(walletID [32]byte) (bool, error)
}

func (n *node) frostWalletRegistryAvailable() bool {
	frostChain, ok := n.chain.(frostWalletRegistryAvailability)

	return ok && frostChain.FrostWalletRegistryAvailable()
}

// handleWalletClosure handles the wallet termination or closing process.
func (n *node) handleWalletClosure(
	walletID [32]byte,
	walletScheme WalletScheme,
) error {
	walletScheme = normalizeWalletScheme(walletScheme)

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// To verify there was no chain reorg and the wallet is really closed, check
	// registration in the registry that emitted the closure event.
	stateCheck := func() (bool, error) {
		var isRegistered bool
		var err error

		switch walletScheme {
		case WalletSchemeECDSA:
			isRegistered, err = n.chain.IsWalletRegistered(walletID)
		case WalletSchemeFROST:
			frostChain, ok := n.chain.(frostWalletRegistrationChain)
			if !ok {
				return false, fmt.Errorf(
					"chain does not support FROST wallet registration checks",
				)
			}

			isRegistered, err = frostChain.IsFrostWalletRegistered(walletID)
		default:
			return false, fmt.Errorf("unsupported wallet scheme [%v]", walletScheme)
		}
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

	walletPublicKeyHash, err := n.chain.WalletPublicKeyHashForWalletID(walletID)
	if err != nil {
		// Legacy WalletClosed events carry ECDSA registry IDs rather than
		// canonical Bridge IDs, and canonical resolution can therefore miss.
		// The local registry fallback also protects custom chain adapters that
		// cannot resolve a FROST ID through the Bridge.
		logger.Debugf(
			"cannot resolve wallet public key hash for wallet ID [0x%x]: [%v]; "+
				"falling back to local wallet ID matching",
			walletID,
			err,
		)

		wallet, ok := n.walletRegistry.getWalletByID(walletID)
		if !ok {
			// Wallet was not found in the registry. The wallet is not controlled
			// by this node.
			logger.Infof(
				"node does not control wallet with ID [0x%x]; quitting wallet "+
					"archiving",
				walletID,
			)
			return nil
		}

		walletPublicKeyHash = bitcoin.PublicKeyHash(wallet.publicKey)
	}

	_, ok := n.walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !ok {
		// Wallet was not found in the registry. The wallet is not controlled by
		// this node.
		logger.Infof(
			"node does not control wallet with ID [0x%x] and public key hash "+
				"[0x%x]; quitting wallet archiving",
			walletID,
			walletPublicKeyHash,
		)
		return nil
	}

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
