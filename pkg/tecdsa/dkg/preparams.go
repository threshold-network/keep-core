package dkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/keepcommon/persistence"
)

// PreParams represents tECDSA DKG pre-parameters that were not yet consumed
// by DKG protocol execution.
type PreParams struct {
	data *keygen.LocalPreParams
	// Timestamp of the PreParams creation. The value is used to help the PreParams
	// storage enforce a First In, First Out algorithm.
	creationTimestamp time.Time
}

// newPreParams constructs a new instance of tECDSA DKG pre-parameters based on
// the generated numbers.
func newPreParams(data *keygen.LocalPreParams) *PreParams {
	return &PreParams{data, time.Now().UTC()}
}

// tssPreParamsPool is a pool holding TSS pre parameters. It autogenerates
// entries up to the pool size. When an entry is pulled from the pool it
// will generate a new entry.
type tssPreParamsPool struct {
	*generator.ParameterPool[PreParams]
	logger log.StandardLogger
}

// newTssPreParamsPool initializes a new TSS pre-parameters pool.
func newTssPreParamsPool(
	logger log.StandardLogger,
	scheduler *generator.Scheduler,
	persistence persistence.BasicHandle,
	poolSize int,
	generationTimeout time.Duration,
	generationDelay time.Duration,
	generationConcurrency int,
) *tssPreParamsPool {
	logger.Infof(
		"TSS pre-parameters target pool size is [%d], generation timeout is [%s] "+
			"generation delay is [%v], and concurrency level is [%d]",
		poolSize,
		generationTimeout,
		generationDelay,
		generationConcurrency,
	)

	newPreParamsFn := func(ctx context.Context) *PreParams {
		timingOutCtx, cancel := context.WithTimeout(ctx, generationTimeout)
		defer cancel()

		preParams, err := keygen.GeneratePreParamsWithContext(
			timingOutCtx,
			generationConcurrency,
		)
		// tss-lib returns generic errors saying "timeout or error while ...".
		// There are three possibilities:
		// 1. Pool canceled the parent `ctx`. This is normal and we should not
		//    log anything in this case.
		// 2. `timingOutCtx` timed out. It means the machine is not fast enough
		//    or that it was just unlucky. We should log a warning.
		// 3. There is some error from tss-lib generator. We log it as a warning
		//    because we'll re-attempt to generate parameters again.
		if err != nil && ctx.Err() == nil {
			logger.Warnf("failed to generate TSS pre-params: [%v]", err)
		}

		// If the context is done, GeneratePreParamsWithContext that got
		// interrupted returns nil result. We do not want to wrap nil inside
		// PreParams structure, so we return nil here.
		if preParams == nil {
			return nil
		}

		return newPreParams(preParams)
	}

	tssPreParamsPersistance := newPreParamsStorage(persistence, logger)

	return &tssPreParamsPool{
		generator.NewParameterPool[PreParams](
			logger,
			scheduler,
			&tssPreParamsPersistance,
			poolSize,
			newPreParamsFn,
			generationDelay,
		),
		logger,
	}
}

const (
	dirName = "preparams"
)

// PersistedPreParams is an alias for Persisted PreParams used in generator.Persistence
// interface implementation.
type PersistedPreParams = generator.Persisted[PreParams]

type preParamsStorage struct {
	// mutex is a single struct-wide lock that ensures all functions
	// of the storage are thread-safe.
	mutex sync.Mutex

	persistence persistence.BasicHandle
	logger      log.StandardLogger
}

func newPreParamsStorage(
	persistence persistence.BasicHandle,
	logger log.StandardLogger,
) preParamsStorage {
	return preParamsStorage{
		persistence: persistence,
		logger:      logger,
	}
}

// Saves provided PreParams to the storage.
func (p *preParamsStorage) Save(pp *PreParams) (*PersistedPreParams, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	ppBytes, err := pp.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshalling of the preparams failed: [%v]", err)
	}
	ppHash := sha256.Sum256(ppBytes)

	fileName := fmt.Sprintf(
		"pp_%d_%s",
		// Use timestamp in the filename so that when the data are read from the
		// disk the First In, First Out algorithm applies.
		pp.creationTimestamp.UnixMilli(),
		// Add part of the hash for an ultra unlikely scenario that two saves
		// happen in the exactly the same millisecond.
		hex.EncodeToString(ppHash[:7]),
	)

	if err := p.persistence.Save(
		ppBytes,
		dirName,
		fileName,
	); err != nil {
		return nil, fmt.Errorf("saving preparams failed: [%w]", err)
	}

	return &PersistedPreParams{Data: *pp, ID: fileName}, nil
}

// Deletes provided PreParams from the storage.
func (p *preParamsStorage) Delete(pp *PersistedPreParams) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.logger.Debugf("deleting preparams [%s]...", pp.ID)

	return p.persistence.Delete(dirName, pp.ID)
}

// ReadAll reads all the PreParams stored in the storage and returns them as a
// slice.
func (p *preParamsStorage) ReadAll() ([]*PersistedPreParams, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	allPreParams := make([]*PersistedPreParams, 0)

	descriptorsChan, errorsChan := p.persistence.ReadAll()

	// Two goroutines read from descriptors and errors channels and either
	// add the PreParams to the result slice or outputs a log error.
	// The reason for using two goroutines at the same time - one for
	// descriptors and one for errors - is that channels do not have to be
	// buffered, and we do not know in what order the information is written to
	// channels.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for descriptor := range descriptorsChan {
			// Read only the files located in the `dirName` subdirectory.
			if descriptor.Directory() != dirName {
				continue
			}

			content, err := descriptor.Content()
			if err != nil {
				p.logger.Errorf(
					"could not read PreParams from file [%s] in directory [%s]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}

			persistedPreParams := &PersistedPreParams{}
			if err = persistedPreParams.Data.Unmarshal(content); err != nil {
				p.logger.Errorf(
					"could not unmarshal PreParams from file [%s] in directory [%s]: [%v]",
					descriptor.Name(),
					descriptor.Directory(),
					err,
				)
				continue
			}
			// Validate recovered PreParams with the same function that is used
			// in tss-lib and causes panic if the PreParams fail the validation.
			// Ref: https://github.com/bnb-chain/tss-lib/blob/cbfa6cf63f18f471429eaab0a5f51cf72b7e9df8/ecdsa/keygen/local_party.go#L71-L73
			if !persistedPreParams.Data.data.ValidateWithProof() {
				p.logger.Errorf(
					"PreParams recovered from file [%s] in directory [%s] failed validation",
					descriptor.Name(),
					descriptor.Directory(),
				)
				continue
			}

			persistedPreParams.ID = descriptor.Name()

			allPreParams = append(allPreParams, persistedPreParams)
		}

		sort.Slice(allPreParams, func(i, j int) bool {
			return allPreParams[i].Data.creationTimestamp.
				Before(allPreParams[j].Data.creationTimestamp)
		})

		wg.Done()
	}()

	go func() {
		for err := range errorsChan {
			p.logger.Errorf(
				"could not load preparams from disk: [%v]",
				err,
			)
		}

		wg.Done()
	}()

	wg.Wait()

	return allPreParams, nil
}
