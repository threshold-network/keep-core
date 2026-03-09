package covenantsigner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

type Service struct {
	store  *Store
	engine Engine
	now    func() time.Time
	mutex  sync.Mutex
}

func NewService(handle persistence.BasicHandle, engine Engine) (*Service, error) {
	if engine == nil {
		engine = NewPassiveEngine()
	}

	store, err := NewStore(handle)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:  store,
		engine: engine,
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

func newRequestID(prefix string) (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(randomBytes)), nil
}

func applyTransition(job *Job, transition *Transition, now time.Time) {
	if transition == nil {
		return
	}

	job.State = transition.State
	job.Detail = transition.Detail
	job.Reason = transition.Reason
	job.PSBTHash = transition.PSBTHash
	job.TransactionHex = transition.TransactionHex
	job.Handoff = transition.Handoff
	job.UpdatedAt = now.Format(time.RFC3339Nano)

	switch transition.State {
	case JobStateArtifactReady, JobStateHandoffReady:
		job.CompletedAt = job.UpdatedAt
		job.FailedAt = ""
	case JobStateFailed:
		job.FailedAt = job.UpdatedAt
	}
}

func mapJobResult(job *Job) StepResult {
	switch job.State {
	case JobStateArtifactReady:
		return StepResult{
			Status:         StepStatusReady,
			RequestID:      job.RequestID,
			Detail:         job.Detail,
			PSBTHash:       job.PSBTHash,
			TransactionHex: job.TransactionHex,
		}
	case JobStateHandoffReady:
		return StepResult{
			Status:    StepStatusReady,
			RequestID: job.RequestID,
			Detail:    job.Detail,
			Handoff:   job.Handoff,
		}
	case JobStateFailed:
		return StepResult{
			Status:    StepStatusFailed,
			RequestID: job.RequestID,
			Detail:    job.Detail,
			Reason:    job.Reason,
		}
	default:
		return StepResult{
			Status:    StepStatusPending,
			RequestID: job.RequestID,
			Detail:    job.Detail,
		}
	}
}

func (s *Service) Submit(ctx context.Context, route TemplateID, input SignerSubmitInput) (StepResult, error) {
	if err := validateSubmitInput(route, input); err != nil {
		return StepResult{}, err
	}

	s.mutex.Lock()
	if existing, ok, err := s.store.GetByRouteRequest(route, input.RouteRequestID); err != nil {
		s.mutex.Unlock()
		return StepResult{}, err
	} else if ok {
		s.mutex.Unlock()
		return mapJobResult(existing), nil
	}

	requestIDPrefix := "kcs"
	if route == TemplateQcV1 {
		requestIDPrefix = "kcs_qc"
	} else if route == TemplateSelfV1 {
		requestIDPrefix = "kcs_self"
	}

	requestID, err := newRequestID(requestIDPrefix)
	if err != nil {
		s.mutex.Unlock()
		return StepResult{}, err
	}

	now := s.now()
	requestDigest, err := requestDigest(input.Request)
	if err != nil {
		s.mutex.Unlock()
		return StepResult{}, err
	}

	job := &Job{
		RequestID:       requestID,
		RouteRequestID:  input.RouteRequestID,
		Route:           route,
		IdempotencyKey:  input.Request.IdempotencyKey,
		FacadeRequestID: input.Request.FacadeRequestID,
		RequestDigest:   requestDigest,
		State:           JobStateSubmitted,
		Detail:          "accepted for covenant signing",
		CreatedAt:       now.Format(time.RFC3339Nano),
		UpdatedAt:       now.Format(time.RFC3339Nano),
		Request:         input.Request,
	}

	if err := s.store.Put(job); err != nil {
		s.mutex.Unlock()
		return StepResult{}, err
	}
	s.mutex.Unlock()

	transition, err := s.engine.OnSubmit(ctx, job)
	if err != nil {
		return StepResult{}, err
	}

	if transition == nil {
		transition = &Transition{
			State:  JobStatePending,
			Detail: "accepted for covenant signing",
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	applyTransition(job, transition, s.now())
	if err := s.store.Put(job); err != nil {
		return StepResult{}, err
	}

	return mapJobResult(job), nil
}

func (s *Service) Poll(ctx context.Context, route TemplateID, input SignerPollInput) (StepResult, error) {
	if err := validatePollInput(route, input); err != nil {
		return StepResult{}, err
	}

	job, ok, err := s.store.GetByRequestID(input.RequestID)
	if err != nil {
		return StepResult{}, err
	}
	if !ok || job.Route != route {
		return StepResult{}, errJobNotFound
	}
	if job.RouteRequestID != input.RouteRequestID {
		return StepResult{}, &inputError{"routeRequestId does not match stored job"}
	}

	digest, err := requestDigest(input.Request)
	if err != nil {
		return StepResult{}, err
	}
	if digest != job.RequestDigest {
		return StepResult{}, &inputError{"request does not match stored job payload"}
	}

	if job.State == JobStateArtifactReady || job.State == JobStateHandoffReady || job.State == JobStateFailed {
		return mapJobResult(job), nil
	}

	transition, err := s.engine.OnPoll(ctx, job)
	if err != nil {
		if err == errJobNotFound {
			applyTransition(job, &Transition{
				State:  JobStateFailed,
				Reason: ReasonJobNotFound,
				Detail: "signer job no longer exists",
			}, s.now())
			if storeErr := s.store.Put(job); storeErr != nil {
				return StepResult{}, storeErr
			}
			return mapJobResult(job), nil
		}
		return StepResult{}, err
	}

	if transition != nil {
		applyTransition(job, transition, s.now())
		if err := s.store.Put(job); err != nil {
			return StepResult{}, err
		}
	}

	return mapJobResult(job), nil
}
