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

const jobsDirectory = "covenant-signer/jobs"
const lockFileName = ".lock"

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
	}

	if err := store.load(); err != nil {
		// Release the lock if loading fails after successful acquisition.
		store.Close()
		return nil, err
	}

	return store, nil
}

// acquireFileLock creates and acquires an exclusive non-blocking advisory lock
// on a lock file inside the jobs directory. The returned file handle must be
// kept open for the lifetime of the lock; closing it releases the lock.
func acquireFileLock(dataDir string) (*os.File, error) {
	lockPath := filepath.Join(dataDir, jobsDirectory, lockFileName)

	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, fmt.Errorf(
			"cannot create lock directory [%s]: %w",
			filepath.Dir(lockPath),
			err,
		)
	}

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
		lockFile.Close()
		return nil, fmt.Errorf(
			"cannot acquire exclusive lock on [%s]: "+
				"another process may already own the store: %w",
			lockPath,
			err,
		)
	}

	return lockFile, nil
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

			content, err := descriptor.Content()
			if err != nil {
				logger.Warnf(
					"skipping unreadable job file [%s]: [%v]",
					descriptor.Name(),
					err,
				)
				continue
			}

			job := &Job{}
			if err := json.Unmarshal(content, job); err != nil {
				logger.Warnf(
					"skipping malformed job file [%s]: [%v]",
					descriptor.Name(),
					err,
				)
				continue
			}

			key := routeKey(job.Route, job.RouteRequestID)

			if existingID, ok := s.byRouteKey[key]; ok {
				if existing := s.byRequestID[existingID]; existing != nil {
					existingIsNewerOrSame, err := isNewerOrSameJobRevision(existing, job)
					if err != nil {
						// When the timestamp comparison fails, prefer
						// whichever job has a parseable timestamp. If the
						// candidate's timestamp is valid, the failure is on
						// the existing job -- replace it. Otherwise skip the
						// candidate.
						if _, parseErr := time.Parse(time.RFC3339Nano, job.UpdatedAt); parseErr != nil {
							logger.Warnf(
								"skipping job [%s] with invalid timestamp on duplicate route key [%s/%s]: [%v]",
								job.RequestID,
								job.Route,
								job.RouteRequestID,
								err,
							)
							continue
						}
						logger.Warnf(
							"replacing job [%s] with invalid timestamp on duplicate route key [%s/%s]: [%v]",
							existing.RequestID,
							job.Route,
							job.RouteRequestID,
							err,
						)
					} else if existingIsNewerOrSame {
						continue
					}
				}
			}

			s.byRequestID[job.RequestID] = job
			s.byRouteKey[key] = job.RequestID
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

	requestID, ok := s.byRouteKey[routeKey(route, routeRequestID)]
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
		} else {
			delete(s.byRequestID, existingRequestID)
		}
	}

	return nil
}
