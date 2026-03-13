package covenantsigner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

type Service struct {
	store                        *Store
	engine                       Engine
	signerApprovalVerifier       SignerApprovalVerifier
	now                          func() time.Time
	mutex                        sync.Mutex
	migrationPlanQuoteTrustRoots []MigrationPlanQuoteTrustRoot
}

type ServiceOption func(*Service)

func WithMigrationPlanQuoteTrustRoots(
	trustRoots []MigrationPlanQuoteTrustRoot,
) ServiceOption {
	cloned := append([]MigrationPlanQuoteTrustRoot{}, trustRoots...)

	return func(service *Service) {
		service.migrationPlanQuoteTrustRoots = cloned
	}
}

func WithSignerApprovalVerifier(
	verifier SignerApprovalVerifier,
) ServiceOption {
	return func(service *Service) {
		service.signerApprovalVerifier = verifier
	}
}

func NewService(
	handle persistence.BasicHandle,
	engine Engine,
	options ...ServiceOption,
) (*Service, error) {
	if engine == nil {
		engine = NewPassiveEngine()
	}

	store, err := NewStore(handle)
	if err != nil {
		return nil, err
	}

	service := &Service{
		store:  store,
		engine: engine,
		now:    func() time.Time { return time.Now().UTC() },
	}
	if verifier, ok := engine.(SignerApprovalVerifier); ok {
		service.signerApprovalVerifier = verifier
	}
	for _, option := range options {
		option(service)
	}

	return service, nil
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

func isTerminalJobState(state JobState) bool {
	return state == JobStateArtifactReady ||
		state == JobStateHandoffReady ||
		state == JobStateFailed
}

func sameJobRevision(current *Job, snapshot *Job) bool {
	return current.RequestID == snapshot.RequestID &&
		current.State == snapshot.State &&
		current.Detail == snapshot.Detail &&
		current.Reason == snapshot.Reason &&
		current.PSBTHash == snapshot.PSBTHash &&
		current.TransactionHex == snapshot.TransactionHex &&
		current.UpdatedAt == snapshot.UpdatedAt &&
		current.CompletedAt == snapshot.CompletedAt &&
		current.FailedAt == snapshot.FailedAt &&
		reflect.DeepEqual(current.Handoff, snapshot.Handoff)
}

func (s *Service) loadPollJob(route TemplateID, input SignerPollInput) (*Job, error) {
	job, ok, err := s.store.GetByRequestID(input.RequestID)
	if err != nil {
		return nil, err
	}
	if !ok || job.Route != route {
		return nil, errJobNotFound
	}
	if job.RouteRequestID != input.RouteRequestID {
		return nil, &inputError{"routeRequestId does not match stored job"}
	}

	digest, err := requestDigest(
		input.Request,
		validationOptions{
			migrationPlanQuoteTrustRoots: s.migrationPlanQuoteTrustRoots,
			signerApprovalVerifier:       s.signerApprovalVerifier,
		},
	)
	if err != nil {
		return nil, err
	}
	if digest != job.RequestDigest {
		return nil, &inputError{"request does not match stored job payload"}
	}

	return job, nil
}

func (s *Service) Submit(ctx context.Context, route TemplateID, input SignerSubmitInput) (StepResult, error) {
	submitValidationOptions := validationOptions{
		migrationPlanQuoteTrustRoots:      s.migrationPlanQuoteTrustRoots,
		requireFreshMigrationPlanQuote:    true,
		migrationPlanQuoteVerificationNow: s.now(),
		signerApprovalVerifier:            s.signerApprovalVerifier,
	}
	if err := validateSubmitInput(route, input, submitValidationOptions); err != nil {
		return StepResult{}, err
	}

	normalizedRequest, err := normalizeRouteSubmitRequest(
		input.Request,
		validationOptions{
			migrationPlanQuoteTrustRoots: s.migrationPlanQuoteTrustRoots,
			signerApprovalVerifier:       s.signerApprovalVerifier,
		},
	)
	if err != nil {
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
	requestDigest, err := requestDigestFromNormalized(normalizedRequest)
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
		Request:         normalizedRequest,
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

	currentJob, ok, err := s.store.GetByRequestID(requestID)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{}, errJobNotFound
	}

	// Another poll already advanced the stored job while submit was waiting on
	// signer work. Return the newer durable state instead of overwriting it with
	// a transition computed from an older snapshot.
	if !sameJobRevision(currentJob, job) || isTerminalJobState(currentJob.State) {
		return mapJobResult(currentJob), nil
	}

	applyTransition(currentJob, transition, s.now())
	if err := s.store.Put(currentJob); err != nil {
		return StepResult{}, err
	}

	return mapJobResult(currentJob), nil
}

func (s *Service) Poll(ctx context.Context, route TemplateID, input SignerPollInput) (StepResult, error) {
	if err := validatePollInput(
		route,
		input,
		validationOptions{
			migrationPlanQuoteTrustRoots: s.migrationPlanQuoteTrustRoots,
			signerApprovalVerifier:       s.signerApprovalVerifier,
		},
	); err != nil {
		return StepResult{}, err
	}

	s.mutex.Lock()
	job, err := s.loadPollJob(route, input)
	if err != nil {
		s.mutex.Unlock()
		return StepResult{}, err
	}
	if isTerminalJobState(job.State) {
		result := mapJobResult(job)
		s.mutex.Unlock()
		return result, nil
	}
	s.mutex.Unlock()

	transition, pollErr := s.engine.OnPoll(ctx, job)
	if pollErr != nil {
		if pollErr != errJobNotFound {
			return StepResult{}, pollErr
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	currentJob, err := s.loadPollJob(route, input)
	if err != nil {
		return StepResult{}, err
	}

	// Another Submit/Poll already advanced the stored job while this poll was
	// in-flight. Return the newer durable state instead of overwriting it with a
	// stale transition computed from an older snapshot.
	if !sameJobRevision(currentJob, job) || isTerminalJobState(currentJob.State) {
		return mapJobResult(currentJob), nil
	}

	if pollErr == errJobNotFound {
		applyTransition(currentJob, &Transition{
			State:  JobStateFailed,
			Reason: ReasonJobNotFound,
			Detail: "signer job no longer exists",
		}, s.now())
		if storeErr := s.store.Put(currentJob); storeErr != nil {
			return StepResult{}, storeErr
		}
		return mapJobResult(currentJob), nil
	}

	if transition != nil {
		applyTransition(currentJob, transition, s.now())
		if err := s.store.Put(currentJob); err != nil {
			return StepResult{}, err
		}
	}

	return mapJobResult(currentJob), nil
}
