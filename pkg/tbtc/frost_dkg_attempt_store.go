package tbtc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sync"
)

const (
	frostDKGAttemptDirectory = "frost-dkg-attempts"
	frostDKGAttemptSchema    = "tbtc-frost-dkg-attempt/v1"
	frostDKGAttemptDomain    = "tbtc-frost-dkg-attempt-v1\x00"
	frostDKGAttemptMaxSize   = 1024
)

type frostDKGAttemptRecord struct {
	Schema     string   `json:"schema"`
	Seed       string   `json:"seed"`
	StartBlock uint64   `json:"startBlock"`
	Checksum   [32]byte `json:"checksum"`
}

func canonicalFrostDKGAttemptSeed(seed *big.Int) (string, error) {
	if seed == nil || seed.Sign() < 0 || seed.BitLen() > 256 {
		return "", fmt.Errorf("FROST DKG seed is not a uint256")
	}
	return fmt.Sprintf("%064x", seed), nil
}

func validateCanonicalFrostDKGAttemptSeed(seed string) error {
	if len(seed) != 64 {
		return fmt.Errorf("FROST DKG seed is not canonical uint256 hex")
	}
	decoded, err := hex.DecodeString(seed)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("FROST DKG seed is not canonical uint256 hex")
	}
	if fmt.Sprintf("%064x", new(big.Int).SetBytes(decoded)) != seed {
		return fmt.Errorf("FROST DKG seed is not canonical uint256 hex")
	}
	return nil
}

func frostDKGAttemptFile(seed string) string {
	return seed + ".json"
}

func frostDKGAttemptChecksum(seed string, startBlock uint64) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(frostDKGAttemptDomain))
	_, _ = digest.Write([]byte(seed))
	var encodedStartBlock [8]byte
	binary.BigEndian.PutUint64(encodedStartBlock[:], startBlock)
	_, _ = digest.Write(encodedStartBlock[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func (ws *walletStorage) saveFrostDKGAttempt(
	seed string,
	startBlock uint64,
) error {
	if ws == nil || ws.persistence == nil {
		return fmt.Errorf("wallet storage persistence is unavailable")
	}
	if err := validateCanonicalFrostDKGAttemptSeed(seed); err != nil {
		return err
	}
	if startBlock == 0 {
		return fmt.Errorf("FROST DKG start block is zero")
	}
	record := frostDKGAttemptRecord{
		Schema:     frostDKGAttemptSchema,
		Seed:       seed,
		StartBlock: startBlock,
		Checksum:   frostDKGAttemptChecksum(seed, startBlock),
	}
	encoded, err := json.Marshal(&record)
	if err != nil {
		return err
	}
	return ws.persistence.Save(
		encoded,
		frostDKGAttemptDirectory,
		frostDKGAttemptFile(seed),
	)
}

func (ws *walletStorage) loadFrostDKGAttemptStartBlocks() (
	map[string]uint64,
	error,
) {
	if ws == nil || ws.persistence == nil {
		return nil, fmt.Errorf("wallet storage persistence is unavailable")
	}

	result := make(map[string]uint64)
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
			if descriptor.Directory() != frostDKGAttemptDirectory {
				continue
			}
			content, err := descriptor.Content()
			if err != nil {
				setError(fmt.Errorf(
					"cannot read FROST DKG attempt [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			if len(content) == 0 || len(content) > frostDKGAttemptMaxSize {
				setError(fmt.Errorf(
					"FROST DKG attempt [%s] has invalid size",
					descriptor.Name(),
				))
				continue
			}
			record := frostDKGAttemptRecord{}
			decoder := json.NewDecoder(bytes.NewReader(content))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&record); err != nil {
				setError(fmt.Errorf(
					"cannot decode FROST DKG attempt [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			var trailing any
			if err := decoder.Decode(&trailing); err != io.EOF {
				setError(fmt.Errorf(
					"FROST DKG attempt [%s] has trailing data",
					descriptor.Name(),
				))
				continue
			}
			if record.Schema != frostDKGAttemptSchema ||
				record.StartBlock == 0 ||
				descriptor.Name() != frostDKGAttemptFile(record.Seed) ||
				record.Checksum != frostDKGAttemptChecksum(
					record.Seed,
					record.StartBlock,
				) {
				setError(fmt.Errorf(
					"FROST DKG attempt [%s] is invalid",
					descriptor.Name(),
				))
				continue
			}
			if err := validateCanonicalFrostDKGAttemptSeed(record.Seed); err != nil {
				setError(fmt.Errorf(
					"FROST DKG attempt [%s] is invalid: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}

			mutex.Lock()
			if _, exists := result[record.Seed]; exists {
				if firstErr == nil {
					firstErr = fmt.Errorf(
						"duplicate durable FROST DKG attempt [%s]",
						record.Seed,
					)
				}
			} else {
				result[record.Seed] = record.StartBlock
			}
			mutex.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for err := range readErrors {
			if err != nil {
				setError(fmt.Errorf(
					"cannot enumerate durable FROST DKG attempts: [%w]",
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

func (wr *walletRegistry) recordFrostDKGAttempt(
	seed *big.Int,
	startBlock uint64,
) error {
	if wr == nil {
		return fmt.Errorf("wallet registry is nil")
	}
	canonicalSeed, err := canonicalFrostDKGAttemptSeed(seed)
	if err != nil {
		return err
	}
	if startBlock == 0 {
		return fmt.Errorf("FROST DKG start block is zero")
	}

	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	if existing, ok := wr.frostDKGAttemptStartBlocks[canonicalSeed]; ok {
		if existing != startBlock {
			return fmt.Errorf(
				"durable FROST DKG attempt [%s] has conflicting start blocks [%d] and [%d]",
				canonicalSeed,
				existing,
				startBlock,
			)
		}
		return nil
	}
	if err := wr.walletStorage.saveFrostDKGAttempt(
		canonicalSeed,
		startBlock,
	); err != nil {
		return fmt.Errorf("cannot persist FROST DKG attempt boundary: [%w]", err)
	}
	if wr.frostDKGAttemptStartBlocks == nil {
		wr.frostDKGAttemptStartBlocks = make(map[string]uint64)
	}
	wr.frostDKGAttemptStartBlocks[canonicalSeed] = startBlock
	wr.revision++
	return nil
}

func (wr *walletRegistry) frostDKGRetirementMaterialSnapshot() (
	[]frostLocalSessionSnapshot,
	uint64,
	bool,
	error,
) {
	if wr == nil {
		return nil, 0, false, fmt.Errorf("wallet registry is nil")
	}

	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	sessions, err := wr.frostLocalSessionSnapshotLocked()
	if err != nil {
		return nil, 0, false, err
	}

	var latestStartBlock uint64
	for seed, startBlock := range wr.frostDKGAttemptStartBlocks {
		if err := validateCanonicalFrostDKGAttemptSeed(seed); err != nil {
			return nil, 0, false, fmt.Errorf(
				"durable FROST DKG attempt is invalid: [%w]",
				err,
			)
		}
		if startBlock == 0 {
			return nil, 0, false, fmt.Errorf(
				"durable FROST DKG attempt [%s] has zero start block",
				seed,
			)
		}
		if startBlock > latestStartBlock {
			latestStartBlock = startBlock
		}
	}
	return sessions, latestStartBlock, latestStartBlock != 0, nil
}
