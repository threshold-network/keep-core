package covenantsigner

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

const jobsDirectory = "covenant-signer/jobs"

type Store struct {
	handle      persistence.BasicHandle
	mutex       sync.Mutex
	byRequestID map[string]*Job
	byRouteKey  map[string]string
}

func NewStore(handle persistence.BasicHandle) (*Store, error) {
	store := &Store{
		handle:      handle,
		byRequestID: make(map[string]*Job),
		byRouteKey:  make(map[string]string),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
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
				return err
			}

			job := &Job{}
			if err := json.Unmarshal(content, job); err != nil {
				return err
			}

			existingID, ok := s.byRouteKey[routeKey(job.Route, job.RouteRequestID)]
			if ok {
				existing := s.byRequestID[existingID]
				if existing != nil {
					existingIsNewerOrSame, err := isNewerOrSameJobRevision(existing, job)
					if err != nil {
						return err
					}
					if existingIsNewerOrSame {
						continue
					}
				}
			}

			s.byRequestID[job.RequestID] = job
			s.byRouteKey[routeKey(job.Route, job.RouteRequestID)] = job.RequestID
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
