package tbtc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	frostRetainedKeyGroupBindingDirectory = "frost-retained-key-groups"
	frostRetainedKeyGroupBindingSchema    = "tbtc-frost-retained-key-group-binding/v1"
	frostRetainedKeyGroupBindingDomain    = "tbtc-frost-retained-key-group-binding-v1\x00"
	frostRetainedKeyGroupBindingMaxSize   = 1024
)

type frostRetainedKeyGroupBinding struct {
	Schema   string   `json:"schema"`
	WalletID [32]byte `json:"walletID"`
	KeyGroup string   `json:"keyGroup"`
	Checksum [32]byte `json:"checksum"`
}

func frostRetainedKeyGroupBindingFile(walletID [32]byte) string {
	return hex.EncodeToString(walletID[:]) + ".json"
}

func frostRetainedKeyGroupBindingChecksum(
	walletID [32]byte,
	keyGroup string,
) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(frostRetainedKeyGroupBindingDomain))
	_, _ = digest.Write(walletID[:])
	var keyGroupLength [2]byte
	binary.BigEndian.PutUint16(keyGroupLength[:], uint16(len(keyGroup)))
	_, _ = digest.Write(keyGroupLength[:])
	_, _ = digest.Write([]byte(keyGroup))
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validateFrostKeyGroupForWallet(
	keyGroup string,
	walletID [32]byte,
) error {
	if walletID == [32]byte{} {
		return fmt.Errorf("wallet ID is zero")
	}
	if keyGroup != strings.ToLower(keyGroup) ||
		(len(keyGroup) != 64 && len(keyGroup) != 66) ||
		strings.HasPrefix(keyGroup, "0x") {
		return fmt.Errorf(
			"key group is not canonical lowercase x-only or compressed SEC1 hex",
		)
	}
	outputKey, err := frostsigning.TaprootOutputKeyFromTBTCSignerKey(keyGroup)
	if err != nil || !bytes.Equal(outputKey, walletID[:]) {
		return fmt.Errorf("key group does not identify its wallet")
	}
	return nil
}

func frostKeyGroupFromSignerMaterial(
	material *frostsigning.NativeSignerMaterial,
	walletID [32]byte,
) (string, error) {
	keyGroup, err := frostsigning.KeyGroupIDFromSignerMaterial(material)
	if err != nil {
		return "", err
	}
	if err := validateFrostKeyGroupForWallet(keyGroup, walletID); err != nil {
		return "", err
	}
	outputKey, err := frostsigning.ExtractTaprootOutputKeyFromMaterial(material)
	if err != nil || !bytes.Equal(outputKey, walletID[:]) {
		return "", fmt.Errorf(
			"signer material Taproot output key does not identify its wallet",
		)
	}
	return keyGroup, nil
}

func frostKeyGroupFromWalletCacheValue(
	value *walletCacheValue,
) (string, bool, error) {
	if value == nil || len(value.signers) == 0 {
		return "", false, fmt.Errorf("wallet cache contains an empty session")
	}

	var keyGroup string
	nativeCount := 0
	for _, walletSigner := range value.signers {
		if walletSigner == nil {
			return "", false, fmt.Errorf("wallet cache contains a nil signer")
		}
		var material *frostsigning.NativeSignerMaterial
		switch typed := walletSigner.signingMaterial().(type) {
		case *frostsigning.NativeSignerMaterial:
			material = typed
		case frostsigning.NativeSignerMaterial:
			materialCopy := typed
			material = &materialCopy
		default:
			continue
		}
		if material == nil {
			return "", false, fmt.Errorf("wallet cache contains nil native signer material")
		}
		if material.Format !=
			frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
			continue
		}
		var payload frostsigning.NativeTBTCSignerMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			return "", false, fmt.Errorf(
				"cannot decode FrostTBTCSignerV1 signer material: [%w]",
				err,
			)
		}
		// Scaffold-era material folds the legacy wallet public key in as its
		// key group: the wallet keeps its legacy identity (mirroring
		// frostWalletIDFromSigner) and must not bind a retained FROST key
		// group derived from that fold.
		if payload.KeyGroupSource ==
			frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey {
			continue
		}
		nativeCount++
		resolved, err := frostKeyGroupFromSignerMaterial(
			material,
			value.walletID,
		)
		if err != nil {
			return "", false, err
		}
		if keyGroup == "" {
			keyGroup = resolved
		} else if keyGroup != resolved {
			return "", false, fmt.Errorf(
				"FROST wallet signer key-group handles disagree",
			)
		}
	}

	if nativeCount == 0 {
		return "", false, nil
	}
	if nativeCount != len(value.signers) {
		return "", false, fmt.Errorf(
			"wallet cache mixes FROST and legacy signer material",
		)
	}
	return keyGroup, true, nil
}

func (ws *walletStorage) saveRetainedFrostKeyGroupBinding(
	walletID [32]byte,
	keyGroup string,
) error {
	if ws == nil || ws.persistence == nil {
		return fmt.Errorf("wallet storage persistence is unavailable")
	}
	if err := validateFrostKeyGroupForWallet(keyGroup, walletID); err != nil {
		return err
	}
	binding := frostRetainedKeyGroupBinding{
		Schema:   frostRetainedKeyGroupBindingSchema,
		WalletID: walletID,
		KeyGroup: keyGroup,
		Checksum: frostRetainedKeyGroupBindingChecksum(walletID, keyGroup),
	}
	encoded, err := json.Marshal(&binding)
	if err != nil {
		return err
	}
	return ws.persistence.Save(
		encoded,
		frostRetainedKeyGroupBindingDirectory,
		frostRetainedKeyGroupBindingFile(walletID),
	)
}

func ensureRetainedFrostKeyGroupBinding(
	ws *walletStorage,
	bindings map[[32]byte]string,
	walletID [32]byte,
	keyGroup string,
) error {
	if bindings == nil {
		return fmt.Errorf("retained FROST key-group binding map is nil")
	}
	if existing, ok := bindings[walletID]; ok {
		if existing != keyGroup {
			return fmt.Errorf(
				"retained FROST wallet key group conflicts with durable binding",
			)
		}
		return nil
	}
	if err := ws.saveRetainedFrostKeyGroupBinding(walletID, keyGroup); err != nil {
		return err
	}
	bindings[walletID] = keyGroup
	return nil
}

func (ws *walletStorage) loadRetainedFrostKeyGroupBindings() (
	map[[32]byte]string,
	error,
) {
	if ws == nil || ws.persistence == nil {
		return nil, fmt.Errorf("wallet storage persistence is unavailable")
	}
	result := make(map[[32]byte]string)
	descriptors, readErrors := ws.persistence.ReadAll()
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var firstErr error
	setError := func(err error) {
		mutex.Lock()
		defer mutex.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		for descriptor := range descriptors {
			if descriptor.Directory() !=
				frostRetainedKeyGroupBindingDirectory {
				continue
			}
			content, err := descriptor.Content()
			if err != nil {
				setError(fmt.Errorf(
					"cannot read retained key-group binding [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			if len(content) == 0 ||
				len(content) > frostRetainedKeyGroupBindingMaxSize {
				setError(fmt.Errorf(
					"retained key-group binding [%s] has invalid size",
					descriptor.Name(),
				))
				continue
			}
			binding := frostRetainedKeyGroupBinding{}
			decoder := json.NewDecoder(bytes.NewReader(content))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&binding); err != nil {
				setError(fmt.Errorf(
					"cannot decode retained key-group binding [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			var trailing any
			if err := decoder.Decode(&trailing); err != io.EOF {
				setError(fmt.Errorf(
					"retained key-group binding [%s] has trailing data",
					descriptor.Name(),
				))
				continue
			}
			if binding.Schema != frostRetainedKeyGroupBindingSchema ||
				binding.WalletID == [32]byte{} ||
				descriptor.Name() !=
					frostRetainedKeyGroupBindingFile(binding.WalletID) ||
				binding.Checksum != frostRetainedKeyGroupBindingChecksum(
					binding.WalletID,
					binding.KeyGroup,
				) {
				setError(fmt.Errorf(
					"retained key-group binding [%s] is invalid",
					descriptor.Name(),
				))
				continue
			}
			if err := validateFrostKeyGroupForWallet(
				binding.KeyGroup,
				binding.WalletID,
			); err != nil {
				setError(fmt.Errorf(
					"retained key-group binding [%s] is invalid: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			mutex.Lock()
			if _, exists := result[binding.WalletID]; exists {
				if firstErr == nil {
					firstErr = fmt.Errorf(
						"duplicate retained key-group binding for wallet [0x%x]",
						binding.WalletID,
					)
				}
			} else {
				result[binding.WalletID] = binding.KeyGroup
			}
			mutex.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for err := range readErrors {
			if err != nil {
				setError(fmt.Errorf(
					"cannot enumerate retained key-group bindings: [%w]",
					err,
				))
			}
		}
	}()
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func (wr *walletRegistry) frostReadinessMaterialSnapshot() (
	[]frostLocalSessionSnapshot,
	map[[32]byte]string,
	uint64,
	error,
) {
	if wr == nil {
		return nil, nil, 0, fmt.Errorf("wallet registry is nil")
	}
	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	sessions, err := wr.frostLocalSessionSnapshotLocked()
	if err != nil {
		return nil, nil, 0, err
	}
	result := make(map[[32]byte]string, len(wr.retainedFrostKeyGroups))
	for walletID, keyGroup := range wr.retainedFrostKeyGroups {
		if err := validateFrostKeyGroupForWallet(keyGroup, walletID); err != nil {
			return nil, nil, 0, fmt.Errorf(
				"retained FROST key-group binding is invalid: [%w]",
				err,
			)
		}
		result[walletID] = keyGroup
	}
	return sessions, result, wr.revision, nil
}

func (wr *walletRegistry) frostReadinessRevisionMatches(revision uint64) bool {
	if wr == nil {
		return false
	}
	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	return wr.revision == revision
}
