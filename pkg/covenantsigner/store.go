package covenantsigner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

const (
	// jobsDirectory is a single-level persistence directory name. It must not
	// contain a path separator: the disk persistence handle creates and
	// enumerates only one directory level, so a nested name is skipped on
	// reload (its descriptor directory is reported as the first-level parent).
	// legacyJobsDirectory is the previously used nested name; job files
	// persisted under it are migrated to jobsDirectory on startup.
	jobsDirectory       = "covenant-signer-jobs"
	legacyJobsDirectory = "covenant-signer/jobs"
	poisonedDirectory   = "covenant-signer/poisoned"
	lockFileName        = ".lock"
)

type Store struct {
	handle      persistence.BasicHandle
	mutex       sync.Mutex
	lockFile    *os.File
	byRequestID map[string]*Job
	byRouteKey  map[string]string
}

// NewStore creates a new Store backed by the given persistence handle. When
// dataDir is non-empty, an exclusive advisory file lock is acquired on a lock
// file inside the jobs directory to prevent concurrent process corruption. If
// the lock cannot be acquired (another process holds it), NewStore returns an
// error. When dataDir is empty (in-memory handles), file locking is skipped.
func NewStore(handle persistence.BasicHandle, dataDir string) (*Store, error) {
	store := &Store{
		handle:      handle,
		byRequestID: make(map[string]*Job),
		byRouteKey:  make(map[string]string),
	}

	if dataDir != "" {
		lockFile, err := acquireFileLock(dataDir)
		if err != nil {
			return nil, err
		}
		store.lockFile = lockFile

		if err := migrateLegacyJobsDirectory(dataDir); err != nil {
			// Release the lock if migration fails after successful acquisition.
			if closeErr := store.Close(); closeErr != nil {
				logger.Warnf(
					"failed to release store lock after migration failure: [%v]",
					closeErr,
				)
			}
			return nil, err
		}
	}

	if err := store.load(); err != nil {
		// Release the lock if loading fails after successful acquisition.
		if closeErr := store.Close(); closeErr != nil {
			logger.Warnf("failed to release store lock after load failure: [%v]", closeErr)
		}
		return nil, err
	}

	return store, nil
}

// acquireFileLock creates and acquires an exclusive non-blocking advisory lock
// on a lock file inside the jobs directory. The returned file handle must be
// kept open for the lifetime of the lock; closing it releases the lock.
//
// IMPORTANT: This uses POSIX flock(2), which is advisory and Linux-specific.
// It protects against concurrent processes on the same host but does NOT
// protect against concurrent access over network filesystems (NFS, EFS,
// CIFS). The data directory MUST reside on local or block-level storage
// with single-writer access (e.g., Kubernetes ReadWriteOnce PV).
func acquireFileLock(dataDir string) (*os.File, error) {
	lockPath := filepath.Join(dataDir, jobsDirectory, lockFileName)

	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, fmt.Errorf(
			"cannot create lock directory [%s]: %w",
			filepath.Dir(lockPath),
			err,
		)
	}

	// #nosec G304 -- lockPath is derived from operator-configured dataDir, not
	// from untrusted user input. The operator controls the data directory.
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot open lock file [%s]: %w",
			lockPath,
			err,
		)
	}

	if err := syscall.Flock(
		int(lockFile.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	); err != nil {
		if closeErr := lockFile.Close(); closeErr != nil {
			logger.Warnf("failed to close lock file after failed flock: [%v]", closeErr)
		}
		return nil, fmt.Errorf(
			"cannot acquire exclusive lock on [%s]: "+
				"another process may already own the store: %w",
			lockPath,
			err,
		)
	}

	return lockFile, nil
}

// migrateLegacyJobsDirectory moves persisted job files from the previously used
// nested legacyJobsDirectory into the flat jobsDirectory. The nested directory
// could not be reliably reloaded by the single-level disk persistence handle,
// so any job files an operator managed to persist under it would otherwise be
// silently skipped on startup. Migration is best-effort per file and
// idempotent: files already present in the destination are left untouched, and
// a missing legacy directory is not an error.
func migrateLegacyJobsDirectory(dataDir string) error {
	legacyDir := filepath.Join(dataDir, legacyJobsDirectory)

	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf(
			"cannot read legacy covenant signer jobs directory [%s]: %w",
			legacyDir,
			err,
		)
	}

	newDir := filepath.Join(dataDir, jobsDirectory)
	if err := os.MkdirAll(newDir, 0700); err != nil {
		return fmt.Errorf(
			"cannot create covenant signer jobs directory [%s]: %w",
			newDir,
			err,
		)
	}

	for _, entry := range entries {
		// Only migrate persisted job files; skip subdirectories and the lock
		// file left behind under the legacy path.
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		newPath := filepath.Join(newDir, entry.Name())
		// Do not clobber a file already present in the destination.
		if _, err := os.Stat(newPath); err == nil {
			continue
		}

		oldPath := filepath.Join(legacyDir, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf(
				"cannot migrate covenant signer job file [%s] to [%s]: %w",
				oldPath,
				newPath,
				err,
			)
		}
	}

	return nil
}

// Close releases the exclusive file lock and closes the underlying lock file
// descriptor. For stores created without a dataDir (in-memory handles), Close
// is a safe no-op. Close is idempotent.
func (s *Store) Close() error {
	if s.lockFile == nil {
		return nil
	}

	// Release the advisory lock before closing the file descriptor.
	_ = syscall.Flock(int(s.lockFile.Fd()), syscall.LOCK_UN)
	err := s.lockFile.Close()
	s.lockFile = nil

	return err
}

func routeKey(route TemplateID, routeRequestID string) string {
	return fmt.Sprintf("%s:%s", route, routeRequestID)
}

func cloneJob(job *Job) (*Job, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}

	cloned := &Job{}
	if err := json.Unmarshal(payload, cloned); err != nil {
		return nil, err
	}

	return cloned, nil
}

func isNewerOrSameJobRevision(existing *Job, candidate *Job) (bool, error) {
	existingUpdatedAt, err := time.Parse(time.RFC3339Nano, existing.UpdatedAt)
	if err != nil {
		return false, fmt.Errorf(
			"cannot parse existing job updatedAt [%s] for request [%s]: %w",
			existing.UpdatedAt,
			existing.RequestID,
			err,
		)
	}

	candidateUpdatedAt, err := time.Parse(time.RFC3339Nano, candidate.UpdatedAt)
	if err != nil {
		return false, fmt.Errorf(
			"cannot parse candidate job updatedAt [%s] for request [%s]: %w",
			candidate.UpdatedAt,
			candidate.RequestID,
			err,
		)
	}

	return !existingUpdatedAt.Before(candidateUpdatedAt), nil
}

func (s *Store) load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	dataChan, errorChan := s.handle.ReadAll()

	var loaded int

	for dataChan != nil || errorChan != nil {
		select {
		case descriptor, ok := <-dataChan:
			if !ok {
				dataChan = nil
				continue
			}

			if descriptor.Directory() != jobsDirectory {
				continue
			}

			// Skip non-job files that share the jobs directory, such as the
			// store lock file. Persisted jobs are always saved with a .json
			// extension.
			if filepath.Ext(descriptor.Name()) != ".json" {
				continue
			}

			content, err := descriptor.Content()
			if err != nil {
				return fmt.Errorf(
					"cannot read persisted covenant signer job file [%s]: %w",
					descriptor.Name(),
					err,
				)
			}

			job := &Job{}
			if err := json.Unmarshal(content, job); err != nil {
				return fmt.Errorf(
					"cannot parse persisted covenant signer job file [%s]: %w",
					descriptor.Name(),
					err,
				)
			}

			key := routeKey(job.Route, job.RouteRequestID)

			// Deduplication: when multiple files share the same route key,
			// keep the job with the newest UpdatedAt timestamp. If timestamps
			// cannot be compared, prefer whichever has a valid timestamp.
			if existingID, ok := s.byRouteKey[key]; ok {
				if existing := s.byRequestID[existingID]; existing != nil {
					existingIsNewerOrSame, err := isNewerOrSameJobRevision(existing, job)
					if err != nil {
						// When the timestamp comparison fails, prefer
						// whichever job has a parseable timestamp. If the
						// candidate's timestamp is valid, the failure is on
						// the existing job -- replace it. Otherwise skip the
						// candidate.
						_, existingParseErr := time.Parse(time.RFC3339Nano, existing.UpdatedAt)
						_, candidateParseErr := time.Parse(time.RFC3339Nano, job.UpdatedAt)

						switch {
						case candidateParseErr != nil && existingParseErr == nil:
							// Only the candidate is unparseable; keep existing.
							logger.Warnf(
								"skipping job [%s] with invalid timestamp on duplicate route key [%s/%s] (keeping [%s]): [%v]",
								job.RequestID,
								job.Route,
								job.RouteRequestID,
								existing.RequestID,
								err,
							)
							continue
						case candidateParseErr == nil && existingParseErr != nil:
							// Only the existing is unparseable; replace with candidate.
							logger.Warnf(
								"replacing job [%s] with invalid timestamp on duplicate route key [%s/%s]: [%v]",
								existing.RequestID,
								job.Route,
								job.RouteRequestID,
								err,
							)
						default:
							// Both timestamps are unparseable. Use
							// lexicographic RequestID as a deterministic
							// tiebreaker so the outcome does not depend
							// on file iteration order.
							if existing.RequestID <= job.RequestID {
								logger.Warnf(
									"skipping job [%s] on duplicate route key [%s/%s] (keeping [%s], lexicographic tiebreak): [%v]",
									job.RequestID,
									job.Route,
									job.RouteRequestID,
									existing.RequestID,
									err,
								)
								continue
							}
							logger.Warnf(
								"replacing job [%s] on duplicate route key [%s/%s] (lexicographic tiebreak): [%v]",
								existing.RequestID,
								job.Route,
								job.RouteRequestID,
								err,
							)
						}
					} else if existingIsNewerOrSame {
						continue
					}
				}

				// Remove the superseded job from the primary index so stale
				// entries do not leak in byRequestID.
				if existingID != job.RequestID {
					delete(s.byRequestID, existingID)
				}
			}

			s.byRequestID[job.RequestID] = job
			s.byRouteKey[key] = job.RequestID
			loaded++
		case err, ok := <-errorChan:
			if !ok {
				errorChan = nil
				continue
			}
			if err != nil {
				return err
			}
		}
	}

	if loaded > 0 {
		logger.Infof("store load complete: loaded [%d] jobs", loaded)
	}

	poisonedDataChan, poisonedErrorChan := s.handle.ReadAll()
	for poisonedDataChan != nil || poisonedErrorChan != nil {
		select {
		case descriptor, ok := <-poisonedDataChan:
			if !ok {
				poisonedDataChan = nil
				continue
			}
			if descriptor.Directory() != poisonedDirectory {
				continue
			}
		case err, ok := <-poisonedErrorChan:
			if !ok {
				poisonedErrorChan = nil
				continue
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) GetByRequestID(requestID string) (*Job, bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	job, ok := s.byRequestID[requestID]
	if !ok {
		return nil, false, nil
	}

	cloned, err := cloneJob(job)
	if err != nil {
		return nil, false, err
	}

	return cloned, true, nil
}

func (s *Store) GetByRouteRequest(route TemplateID, routeRequestID string) (*Job, bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := routeKey(route, routeRequestID)

	requestID, ok := s.byRouteKey[key]
	if !ok {
		return nil, false, nil
	}

	job := s.byRequestID[requestID]
	if job == nil {
		return nil, false, nil
	}

	cloned, err := cloneJob(job)
	if err != nil {
		return nil, false, err
	}

	return cloned, true, nil
}

func (s *Store) Put(job *Job) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	key := routeKey(job.Route, job.RouteRequestID)
	existingRequestID, hasExisting := s.byRouteKey[key]
	if err := s.handle.Save(payload, jobsDirectory, job.RequestID+".json"); err != nil {
		return err
	}

	cloned, err := cloneJob(job)
	if err != nil {
		return err
	}

	s.byRequestID[job.RequestID] = cloned
	s.byRouteKey[key] = job.RequestID

	if hasExisting && existingRequestID != job.RequestID {
		if err := s.handle.Delete(jobsDirectory, existingRequestID+".json"); err != nil {
			logger.Warnf(
				"failed to delete stale covenant signer job file [%s]: [%v]",
				existingRequestID+".json",
				err,
			)
		}
		delete(s.byRequestID, existingRequestID)
	}

	return nil
}
