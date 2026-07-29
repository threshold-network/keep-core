package tbtc

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"

	"github.com/keep-network/keep-common/pkg/persistence"
)

// CalculateWalletIDFunc calculates the ECDSA wallet ID based on the provided
// wallet public key.
type CalculateWalletIdFunc func(walletPublicKey *ecdsa.PublicKey) ([32]byte, error)

// walletRegistry is the component that holds the data of the wallets managed
// by the given node. All functions of the registry are safe for concurrent use.
type walletRegistry struct {
	// mutex is a single struct-wide lock that ensures all functions
	// of the registry are thread-safe.
	mutex sync.Mutex

	// walletCache is a cache of maintained wallets. The cache's key is the
	// uncompressed public key of the given wallet.
	walletCache map[string]*walletCacheValue

	// walletStorage is the handle to the wallet storage responsible for
	// wallet persistence.
	walletStorage *walletStorage

	// retainedFrostKeyGroups contains public, exact key-group handles captured
	// before FROST wallet signer records are archived. The active wallet cache
	// intentionally drops terminal wallets, while the native signer retains
	// their key packages; this durable binding lets activation readiness
	// reconcile those retained packages without trusting the native inventory
	// to identify itself.
	retainedFrostKeyGroups map[[32]byte]string

	// revision advances after each successful active-session registration or
	// archive. Readiness pins it across its Go/Rust reconciliation so a wallet
	// cannot disappear or gain a local seat mid-check.
	revision uint64

	// calculateWalletIdFunc calculates the ECDSA wallet ID based on the
	// provided wallet public key.
	calculateWalletIdFunc CalculateWalletIdFunc
}

type walletCacheValue struct {
	// SHA-256+RIPEMD-160 hash computed over the compressed ECDSA public key of
	// the wallet.
	walletPublicKeyHash [20]byte
	// ECDSA wallet ID calculated as the keccak256 of the 64-byte-long
	// concatenation of the X and Y coordinates of the wallet's public key.
	walletID [32]byte
	// Array of wallet signers controlled by this node.
	signers []*signer
}

// newWalletRegistry creates a new instance of the walletRegistry.
func newWalletRegistry(
	persistence persistence.ProtectedHandle,
	calculateWalletIdFunc CalculateWalletIdFunc,
) (*walletRegistry, error) {
	walletStorage := newWalletStorage(persistence)
	retainedFrostKeyGroups, err :=
		walletStorage.loadRetainedFrostKeyGroupBindings()
	if err != nil {
		return nil, fmt.Errorf(
			"could not load retained FROST key-group bindings: [%w]",
			err,
		)
	}

	// Pre-populate the wallet cache using the wallet storage.
	walletCache := make(map[string]*walletCacheValue)
	walletSigners := walletStorage.loadSigners()
	if len(walletSigners) > 0 {
		for walletStorageKey, signers := range walletSigners {
			// We need to extract the wallet from the signers array. The
			// walletStorage.loadSigners function guarantees there is always
			// at least one signer for the given walletStorageKey so, we
			// don't need to check len(signers). Then, we can just take the
			// wallet from the first signer as the wallet is same for all of
			// them.
			wallet := signers[0].wallet
			walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)
			walletID, err := calculateWalletIDForSigner(
				signers[0],
				calculateWalletIdFunc,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"error while calculating wallet ID for wallet with public "+
						"key hash [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
			}

			walletCache[walletStorageKey] = &walletCacheValue{
				walletPublicKeyHash: walletPublicKeyHash,
				walletID:            walletID,
				signers:             signers,
			}
			keyGroup, isFrostWallet, err :=
				frostKeyGroupFromWalletCacheValue(walletCache[walletStorageKey])
			if err != nil {
				return nil, fmt.Errorf(
					"cannot resolve stored FROST wallet key group: [%w]",
					err,
				)
			}
			if isFrostWallet {
				if err := ensureRetainedFrostKeyGroupBinding(
					walletStorage,
					retainedFrostKeyGroups,
					walletID,
					keyGroup,
				); err != nil {
					return nil, fmt.Errorf(
						"cannot persist stored FROST wallet key-group binding: [%w]",
						err,
					)
				}
			}

			logger.Infof(
				"wallet signing group [0x%v] loaded from storage "+
					"with [%v] members and wallet public key hash [0x%x]",
				walletStorageKey,
				len(signers),
				walletPublicKeyHash,
			)
		}
	} else {
		logger.Infof("no wallet signing groups found in the storage")
	}

	return &walletRegistry{
		walletCache:            walletCache,
		walletStorage:          walletStorage,
		retainedFrostKeyGroups: retainedFrostKeyGroups,
		calculateWalletIdFunc:  calculateWalletIdFunc,
	}, nil
}

// getWalletsPublicKeys returns public keys of all registered wallets.
func (wr *walletRegistry) getWalletsPublicKeys() []*ecdsa.PublicKey {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	keys := make([]*ecdsa.PublicKey, 0)
	for _, value := range wr.walletCache {
		// We can take the wallet from the first signer. All signers for the
		// given cache value belong to the same wallet.
		keys = append(keys, value.signers[0].wallet.publicKey)
	}

	return keys
}

// registerSigner registers the given signer using in the walletRegistry.
func (wr *walletRegistry) registerSigner(walletSigner *signer) error {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	if walletSigner == nil || walletSigner.wallet.publicKey == nil {
		return fmt.Errorf("cannot register malformed signer")
	}

	walletStorageKey := getWalletStorageKey(walletSigner.wallet.publicKey)
	candidate, exists := wr.walletCache[walletStorageKey]

	// If the wallet cache does not have the given entry yet, initialize
	// the value and compute the wallet ID and wallet public key hash. This way,
	// the hashes are computed only once. No need to initialize signers slice as
	// appending works with nil values.
	if !exists {
		walletID, err := calculateWalletIDForSigner(
			walletSigner,
			wr.calculateWalletIdFunc,
		)
		if err != nil {
			return fmt.Errorf("cannot calculate wallet ID: [%v]", err)
		}

		candidate = &walletCacheValue{
			walletPublicKeyHash: bitcoin.PublicKeyHash(walletSigner.wallet.publicKey),
			walletID:            walletID,
		}
	}
	candidate = &walletCacheValue{
		walletPublicKeyHash: candidate.walletPublicKeyHash,
		walletID:            candidate.walletID,
		signers: append(
			append([]*signer{}, candidate.signers...),
			walletSigner,
		),
	}

	keyGroup, isFrostWallet, err := frostKeyGroupFromWalletCacheValue(candidate)
	if err != nil {
		return fmt.Errorf("cannot resolve FROST wallet key group: [%w]", err)
	}
	if isFrostWallet {
		if wr.retainedFrostKeyGroups == nil {
			wr.retainedFrostKeyGroups = make(map[[32]byte]string)
		}
		if err := ensureRetainedFrostKeyGroupBinding(
			wr.walletStorage,
			wr.retainedFrostKeyGroups,
			candidate.walletID,
			keyGroup,
		); err != nil {
			return fmt.Errorf(
				"cannot persist FROST wallet key-group binding: [%w]",
				err,
			)
		}
	}

	if err := wr.walletStorage.saveSigner(walletSigner); err != nil {
		return fmt.Errorf("cannot save signer in the storage: [%w]", err)
	}
	wr.walletCache[walletStorageKey] = candidate
	wr.revision++
	return nil
}

// getSigners gets all signers for the given wallet held by the walletRegistry.
func (wr *walletRegistry) getSigners(
	walletPublicKey *ecdsa.PublicKey,
) []*signer {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	if value, ok := wr.walletCache[getWalletStorageKey(walletPublicKey)]; ok {
		return value.signers
	}

	return nil
}

// getWalletByPublicKeyHash gets the given wallet by its 20-byte wallet
// public key hash. Second boolean return value denotes whether the wallet
// was found in the registry or not.
func (wr *walletRegistry) getWalletByPublicKeyHash(
	walletPublicKeyHash [20]byte,
) (wallet, bool) {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	for _, value := range wr.walletCache {
		if value.walletPublicKeyHash == walletPublicKeyHash {
			// All signers belong to one wallet. Take that wallet from the
			// first signer.
			return value.signers[0].wallet, true
		}
	}

	return wallet{}, false
}

func (wr *walletRegistry) getWalletIDByPublicKeyHash(
	walletPublicKeyHash [20]byte,
) ([32]byte, bool) {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	for _, value := range wr.walletCache {
		if value.walletPublicKeyHash == walletPublicKeyHash {
			return value.walletID, true
		}
	}

	return [32]byte{}, false
}

func (wr *walletRegistry) isFrostWalletByPublicKeyHash(
	walletPublicKeyHash [20]byte,
) (bool, error) {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	for _, value := range wr.walletCache {
		if value.walletPublicKeyHash != walletPublicKeyHash {
			continue
		}
		_, isFrostWallet, err := frostKeyGroupFromWalletCacheValue(value)
		if err != nil {
			return false, err
		}
		return isFrostWallet, nil
	}

	return false, fmt.Errorf("wallet not found in the wallet cache")
}

// getWalletByID gets the given wallet by its 32-byte wallet ID. Second boolean
// return value denotes whether the wallet was found in the registry or not.
func (wr *walletRegistry) getWalletByID(walletID [32]byte) (wallet, bool) {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	for _, value := range wr.walletCache {
		if value.walletID == walletID {
			// All signers belong to one wallet. Take that wallet from the
			// first signer.
			return value.signers[0].wallet, true
		}
	}

	return wallet{}, false
}

// archiveWallet archives the wallet with the given public key hash. The wallet
// data is removed from the wallet cache and the entire wallet storage directory
// is moved to the archive directory.
func (wr *walletRegistry) archiveWallet(
	walletPublicKeyHash [20]byte,
) error {
	wr.mutex.Lock()
	defer wr.mutex.Unlock()

	var walletPublicKey *ecdsa.PublicKey
	var walletValue *walletCacheValue

	for _, value := range wr.walletCache {
		if value.walletPublicKeyHash == walletPublicKeyHash {
			// All signers belong to one wallet. Take the wallet public key from
			//  the first signer.
			walletPublicKey = value.signers[0].wallet.publicKey
			walletValue = value
			break
		}
	}

	if walletPublicKey == nil {
		return fmt.Errorf("wallet not found in the wallet cache")
	}

	walletStorageKey := getWalletStorageKey(walletPublicKey)

	keyGroup, isFrostWallet, err := frostKeyGroupFromWalletCacheValue(walletValue)
	if err != nil {
		return fmt.Errorf(
			"could not resolve FROST wallet key group before archive: [%w]",
			err,
		)
	}
	if isFrostWallet {
		if wr.retainedFrostKeyGroups == nil {
			wr.retainedFrostKeyGroups = make(map[[32]byte]string)
		}
		if err := ensureRetainedFrostKeyGroupBinding(
			wr.walletStorage,
			wr.retainedFrostKeyGroups,
			walletValue.walletID,
			keyGroup,
		); err != nil {
			return fmt.Errorf(
				"could not persist retained FROST wallet key group: [%w]",
				err,
			)
		}
	}

	// Archive the entire wallet storage.
	err = wr.walletStorage.archiveWallet(walletStorageKey)
	if err != nil {
		return fmt.Errorf("could not archive wallet: [%v]", err)
	}

	// Remove the wallet from the wallet cache.
	delete(wr.walletCache, walletStorageKey)
	wr.revision++

	return nil
}

// walletStorage is the component that persists data of the wallets managed
// by the given node using the underlying persistence layer. It should be
// used directly only by the walletRegistry.
type walletStorage struct {
	// persistence is the handle to the underlying persistence layer.
	persistence persistence.ProtectedHandle
}

// newWalletStorage creates a new instance of the walletStorage.
func newWalletStorage(persistence persistence.ProtectedHandle) *walletStorage {
	return &walletStorage{persistence}
}

// saveSigner saves the given signer using the underlying persistence layer
// of the walletStorage. It does not add the signer to any in-memory cache
// and should not be called from any other place than walletRegistry.
func (ws *walletStorage) saveSigner(signer *signer) error {
	signerBytes, err := signer.Marshal()
	if err != nil {
		return fmt.Errorf("could not marshal signer: [%w]", err)
	}

	err = ws.persistence.Save(
		signerBytes,
		getWalletStorageKey(signer.wallet.publicKey),
		fmt.Sprintf("/membership_%v", signer.signingGroupMemberIndex),
	)
	if err != nil {
		return fmt.Errorf(
			"could not save membership using the "+
				"underlying persistence layer: [%w]",
			err,
		)
	}

	return nil
}

// archiveWallet archives the given wallet data in the underlying persistence
// layer of the walletStorage.
func (ws *walletStorage) archiveWallet(walletStorageKey string) error {
	err := ws.persistence.Archive(walletStorageKey)
	if err != nil {
		return fmt.Errorf(
			"could not archive wallet storage using the "+
				"underlying persistence layer: [%w]",
			err,
		)
	}

	return nil
}

// loadSigners loads all signers stored using the underlying persistence layer.
// This function should not be called from any other place than walletRegistry.
func (ws *walletStorage) loadSigners() map[string][]*signer {
	signersByWallet := make(map[string][]*signer)

	descriptorsChan, errorsChan := ws.persistence.ReadAll()

	// Two goroutines read from descriptors and errors channels and either
	// add the signer to the result map or outputs a log error.
	// The reason for using two goroutines at the same time - one for
	// descriptors and one for errors - is that channels do not have to be
	// buffered, and we do not know in what order the information is written to
	// channels.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for descriptor := range descriptorsChan {
			if descriptor.Directory() ==
				frostRetainedKeyGroupBindingDirectory {
				continue
			}
			content, err := descriptor.Content()
			if err != nil {
				logger.Errorf(
					"could not get content from file [%v] "+
						"in directory [%v]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}

			signer := &signer{}
			if err := signer.Unmarshal(content); err != nil {
				logger.Errorf(
					"could not unmarshal signer from file [%v] "+
						"in directory [%v]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}

			walletStorageKey := getWalletStorageKey(signer.wallet.publicKey)

			signersByWallet[walletStorageKey] = append(
				signersByWallet[walletStorageKey],
				signer,
			)
		}

		wg.Done()
	}()

	go func() {
		for err := range errorsChan {
			logger.Errorf(
				"could not load signer from disk: [%v]",
				err,
			)
		}

		wg.Done()
	}()

	wg.Wait()

	return signersByWallet
}

// getWalletStorageKey compute the wallet storage key that is used to identify
// the given wallet for caching and storage purposes.
func getWalletStorageKey(walletPublicKey *ecdsa.PublicKey) string {
	walletPublicKeyBytes := secp256k1.Marshal(walletPublicKey)

	// Strip the 04 prefix to limit the key length to 128 characters in order
	// to make it usable as a directory name.
	return hex.EncodeToString(walletPublicKeyBytes)[2:]
}
