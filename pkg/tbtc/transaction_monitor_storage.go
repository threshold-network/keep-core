package tbtc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

const transactionMonitorDirectory = "transaction_monitor"

// Each transaction has an initial record and, once alerted, a second complete
// record. BasicHandle.Save overwrites files in place, so updating the initial
// record could lose its broadcast time if the process crashes during the write.
// Keeping it intact also lets recovery tolerate a partial alert record. Once a
// record is successfully saved it is never rewritten.
type transactionMonitorRecord struct {
	Version             int       `json:"version"`
	WalletPublicKeyHash string    `json:"wallet_public_key_hash"`
	BroadcastAt         time.Time `json:"broadcast_at"`
	Alerted             bool      `json:"alerted"`
}

func transactionMonitorRecordName(txHash bitcoin.Hash, alerted bool) string {
	name := txHash.Hex(bitcoin.ReversedByteOrder)
	if alerted {
		name += ".alerted"
	}
	return name + ".json"
}

// persist requires tm.mu. A failed save leaves the entry dirty for retry.
func (tm *transactionMonitor) persist(txHash bitcoin.Hash) {
	tracked := tm.tracked[txHash]
	if !tracked.dirty || tracked.pendingRemoval {
		return
	}
	// A nil handle is used by monitors in tests that need no persistent state.
	if tm.persistence == nil {
		tracked.dirty = false
		return
	}

	data, err := json.Marshal(transactionMonitorRecord{
		Version:             1,
		WalletPublicKeyHash: hex.EncodeToString(tracked.walletPublicKeyHash[:]),
		BroadcastAt:         tracked.broadcastAt,
		Alerted:             tracked.alerted,
	})
	if err == nil {
		err = tm.persistence.Save(
			data,
			transactionMonitorDirectory,
			transactionMonitorRecordName(txHash, tracked.alerted),
		)
	}
	if err != nil {
		logger.Errorf("could not persist monitored transaction [%s]; "+
			"will retry: [%v]", txHash.Hex(bitcoin.ReversedByteOrder), err)
		return
	}
	tracked.dirty = false
}

// restore runs synchronously during construction. ReadAll includes other work
// data (such as DKG pre-parameters), which must be ignored. Both channels must
// be drained together because either may be unbuffered.
func (tm *transactionMonitor) restore() {
	if tm.persistence == nil {
		return
	}
	descriptors, failures := tm.persistence.ReadAll()
	for descriptors != nil || failures != nil {
		select {
		case descriptor, ok := <-descriptors:
			if !ok {
				descriptors = nil
				continue
			}
			if descriptor.Directory() != transactionMonitorDirectory {
				continue
			}
			txHash, tracked, err := decodeTransactionMonitorRecord(descriptor)
			if err != nil {
				logger.Errorf("could not restore monitored transaction from [%s]: [%v]",
					descriptor.Name(), err)
				continue
			}
			if previous, exists := tm.tracked[txHash]; exists {
				// Prefer the alerted record regardless of enumeration order.
				if previous.alerted {
					continue
				}
			} else if len(tm.tracked) >= transactionMonitorMaxTracked {
				logger.Warnf("transaction monitor tracking table is full ([%d]); "+
					"cannot restore transaction [%s]", transactionMonitorMaxTracked,
					txHash.Hex(bitcoin.ReversedByteOrder))
				continue
			}
			tm.tracked[txHash] = tracked
		case err, ok := <-failures:
			if !ok {
				failures = nil
				continue
			}
			logger.Errorf("could not read transaction monitor storage: [%v]", err)
		}
	}
}

func decodeTransactionMonitorRecord(
	descriptor persistence.DataDescriptor,
) (bitcoin.Hash, *trackedTransaction, error) {
	name := strings.TrimSuffix(descriptor.Name(), ".json")
	alerted := strings.HasSuffix(name, ".alerted")
	txHash, err := bitcoin.NewHashFromString(
		strings.TrimSuffix(name, ".alerted"),
		bitcoin.ReversedByteOrder,
	)
	if err != nil || descriptor.Name() != transactionMonitorRecordName(txHash, alerted) {
		return bitcoin.Hash{}, nil, errors.New("invalid transaction record name")
	}
	data, err := descriptor.Content()
	if err != nil {
		return bitcoin.Hash{}, nil, err
	}
	var record transactionMonitorRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return bitcoin.Hash{}, nil, err
	}
	if record.Version != 1 || record.BroadcastAt.IsZero() || record.Alerted != alerted {
		return bitcoin.Hash{}, nil, errors.New("invalid transaction record metadata")
	}
	walletPublicKeyHash, err := hex.DecodeString(record.WalletPublicKeyHash)
	if err != nil || len(walletPublicKeyHash) != 20 {
		return bitcoin.Hash{}, nil, errors.New("invalid wallet public key hash")
	}
	tracked := &trackedTransaction{
		broadcastAt: record.BroadcastAt,
		alerted:     record.Alerted,
	}
	copy(tracked.walletPublicKeyHash[:], walletPublicKeyHash)
	return txHash, tracked, nil
}

func (tm *transactionMonitor) deletePersisted(txHash bitcoin.Hash) error {
	if tm.persistence == nil {
		return nil
	}
	// Delete the initial record first. If interrupted, the remaining alerted
	// record still carries the original age and suppresses duplicate alerts.
	for _, alerted := range []bool{false, true} {
		err := tm.persistence.Delete(
			transactionMonitorDirectory,
			transactionMonitorRecordName(txHash, alerted),
		)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
