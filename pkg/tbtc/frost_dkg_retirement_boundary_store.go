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
	frostDKGAttemptDirectory         = "frost-dkg-attempts"
	frostDKGAttemptLegacySchema      = "tbtc-frost-dkg-attempt/v1"
	frostDKGAttemptLegacyDomain      = "tbtc-frost-dkg-attempt-v1\x00"
	frostDKGRetirementBoundarySchema = "tbtc-frost-dkg-retirement-boundary/v2"
	frostDKGRetirementBoundaryDomain = "tbtc-frost-dkg-retirement-boundary-v2\x00"
	frostDKGAttemptMaxSize           = 1024

	frostDKGRetirementBoundaryKindAttempt   = "attempt"
	frostDKGRetirementBoundaryKindMigration = "migration"
)

type frostDKGRetirementBoundaryRecord struct {
	Schema     string   `json:"schema"`
	Kind       string   `json:"kind,omitempty"`
	Seed       string   `json:"seed,omitempty"`
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

func frostDKGLegacyAttemptFile(seed string) string {
	return seed + ".json"
}

func frostDKGLegacyAttemptChecksum(seed string, startBlock uint64) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(frostDKGAttemptLegacyDomain))
	_, _ = digest.Write([]byte(seed))
	var encodedStartBlock [8]byte
	binary.BigEndian.PutUint64(encodedStartBlock[:], startBlock)
	_, _ = digest.Write(encodedStartBlock[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func frostDKGRetirementBoundaryIdentity(
	kind string,
	seed string,
	startBlock uint64,
) string {
	return fmt.Sprintf("%s:%s:%020d", kind, seed, startBlock)
}

func frostDKGRetirementBoundaryFile(
	kind string,
	seed string,
	startBlock uint64,
) string {
	return frostDKGRetirementBoundaryIdentity(kind, seed, startBlock) + ".json"
}

func frostDKGRetirementBoundaryChecksum(
	kind string,
	seed string,
	startBlock uint64,
) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(frostDKGRetirementBoundaryDomain))
	for _, value := range []string{kind, seed} {
		var valueLength [2]byte
		binary.BigEndian.PutUint16(valueLength[:], uint16(len(value)))
		_, _ = digest.Write(valueLength[:])
		_, _ = digest.Write([]byte(value))
	}
	var encodedStartBlock [8]byte
	binary.BigEndian.PutUint64(encodedStartBlock[:], startBlock)
	_, _ = digest.Write(encodedStartBlock[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validateFrostDKGRetirementBoundary(
	kind string,
	seed string,
	startBlock uint64,
) error {
	if startBlock == 0 {
		return fmt.Errorf("FROST DKG start block is zero")
	}
	switch kind {
	case frostDKGRetirementBoundaryKindAttempt:
		if err := validateCanonicalFrostDKGAttemptSeed(seed); err != nil {
			return err
		}
	case frostDKGRetirementBoundaryKindMigration:
		if seed != "" {
			return fmt.Errorf("FROST DKG migration boundary has a seed")
		}
	default:
		return fmt.Errorf("FROST DKG retirement boundary kind is invalid")
	}
	return nil
}

func (ws *walletStorage) saveFrostDKGRetirementBoundary(
	kind string,
	seed string,
	startBlock uint64,
) error {
	if ws == nil || ws.persistence == nil {
		return fmt.Errorf("wallet storage persistence is unavailable")
	}
	if err := validateFrostDKGRetirementBoundary(
		kind,
		seed,
		startBlock,
	); err != nil {
		return err
	}
	record := frostDKGRetirementBoundaryRecord{
		Schema:     frostDKGRetirementBoundarySchema,
		Kind:       kind,
		Seed:       seed,
		StartBlock: startBlock,
		Checksum: frostDKGRetirementBoundaryChecksum(
			kind,
			seed,
			startBlock,
		),
	}
	encoded, err := json.Marshal(&record)
	if err != nil {
		return err
	}
	return ws.persistence.Save(
		encoded,
		frostDKGAttemptDirectory,
		frostDKGRetirementBoundaryFile(kind, seed, startBlock),
	)
}

func (ws *walletStorage) loadFrostDKGRetirementBoundaries() (
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
					"cannot read FROST DKG retirement boundary [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			if len(content) == 0 || len(content) > frostDKGAttemptMaxSize {
				setError(fmt.Errorf(
					"FROST DKG retirement boundary [%s] has invalid size",
					descriptor.Name(),
				))
				continue
			}
			record := frostDKGRetirementBoundaryRecord{}
			decoder := json.NewDecoder(bytes.NewReader(content))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&record); err != nil {
				setError(fmt.Errorf(
					"cannot decode FROST DKG retirement boundary [%s]: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}
			var trailing any
			if err := decoder.Decode(&trailing); err != io.EOF {
				setError(fmt.Errorf(
					"FROST DKG retirement boundary [%s] has trailing data",
					descriptor.Name(),
				))
				continue
			}
			var identity string
			switch record.Schema {
			case frostDKGAttemptLegacySchema:
				if record.Kind != "" ||
					record.StartBlock == 0 ||
					descriptor.Name() !=
						frostDKGLegacyAttemptFile(record.Seed) ||
					record.Checksum != frostDKGLegacyAttemptChecksum(
						record.Seed,
						record.StartBlock,
					) {
					setError(fmt.Errorf(
						"FROST DKG attempt [%s] is invalid",
						descriptor.Name(),
					))
					continue
				}
				record.Kind = frostDKGRetirementBoundaryKindAttempt
				identity = frostDKGRetirementBoundaryIdentity(
					record.Kind,
					record.Seed,
					record.StartBlock,
				)
			case frostDKGRetirementBoundarySchema:
				if descriptor.Name() != frostDKGRetirementBoundaryFile(
					record.Kind,
					record.Seed,
					record.StartBlock,
				) ||
					record.Checksum != frostDKGRetirementBoundaryChecksum(
						record.Kind,
						record.Seed,
						record.StartBlock,
					) {
					setError(fmt.Errorf(
						"FROST DKG retirement boundary [%s] is invalid",
						descriptor.Name(),
					))
					continue
				}
				identity = frostDKGRetirementBoundaryIdentity(
					record.Kind,
					record.Seed,
					record.StartBlock,
				)
			default:
				setError(fmt.Errorf(
					"FROST DKG retirement boundary [%s] has an unsupported schema",
					descriptor.Name(),
				))
				continue
			}
			if err := validateFrostDKGRetirementBoundary(
				record.Kind,
				record.Seed,
				record.StartBlock,
			); err != nil {
				setError(fmt.Errorf(
					"FROST DKG retirement boundary [%s] is invalid: [%w]",
					descriptor.Name(),
					err,
				))
				continue
			}

			mutex.Lock()
			if _, exists := result[identity]; exists {
				if firstErr == nil {
					firstErr = fmt.Errorf(
						"duplicate durable FROST DKG retirement boundary [%s]",
						identity,
					)
				}
			} else {
				result[identity] = record.StartBlock
			}
			mutex.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for err := range readErrors {
			if err != nil {
				setError(fmt.Errorf(
					"cannot enumerate durable FROST DKG retirement boundaries: [%w]",
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
	identity := frostDKGRetirementBoundaryIdentity(
		frostDKGRetirementBoundaryKindAttempt,
		canonicalSeed,
		startBlock,
	)
	if _, ok := wr.frostDKGRetirementBoundaries[identity]; ok {
		return nil
	}
	if err := wr.walletStorage.saveFrostDKGRetirementBoundary(
		frostDKGRetirementBoundaryKindAttempt,
		canonicalSeed,
		startBlock,
	); err != nil {
		return fmt.Errorf("cannot persist FROST DKG attempt boundary: [%w]", err)
	}
	if wr.frostDKGRetirementBoundaries == nil {
		wr.frostDKGRetirementBoundaries = make(map[string]uint64)
	}
	wr.frostDKGRetirementBoundaries[identity] = startBlock
	wr.revision++
	return nil
}

func (wr *walletRegistry) recordFrostDKGMigrationBoundary(
	startBlock uint64,
) error {
	if wr == nil {
		return fmt.Errorf("wallet registry is nil")
	}
	if startBlock == 0 {
		return fmt.Errorf("FROST DKG migration boundary block is zero")
	}

	identity := frostDKGRetirementBoundaryIdentity(
		frostDKGRetirementBoundaryKindMigration,
		"",
		startBlock,
	)
	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	if len(wr.frostDKGRetirementBoundaries) != 0 {
		return fmt.Errorf(
			"cannot migrate FROST DKG inventory after retirement boundaries exist",
		)
	}
	if err := wr.walletStorage.saveFrostDKGRetirementBoundary(
		frostDKGRetirementBoundaryKindMigration,
		"",
		startBlock,
	); err != nil {
		return fmt.Errorf(
			"cannot persist FROST DKG migration boundary: [%w]",
			err,
		)
	}
	if wr.frostDKGRetirementBoundaries == nil {
		wr.frostDKGRetirementBoundaries = make(map[string]uint64)
	}
	wr.frostDKGRetirementBoundaries[identity] = startBlock
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

	var latestBoundary uint64
	for identity, startBlock := range wr.frostDKGRetirementBoundaries {
		if startBlock == 0 {
			return nil, 0, false, fmt.Errorf(
				"durable FROST DKG retirement boundary [%s] has zero start block",
				identity,
			)
		}
		if startBlock > latestBoundary {
			latestBoundary = startBlock
		}
	}
	return sessions, latestBoundary, latestBoundary != 0, nil
}
