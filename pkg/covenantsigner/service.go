package covenantsigner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

type Service struct {
	store                        *Store
	engine                       Engine
	signerApprovalVerifier       SignerApprovalVerifier
	now                          func() time.Time
	currentBlockProvider         func() (uint64, error)
	maxInFlight                  int
	inFlightSlots                chan struct{}
	mutex                        sync.Mutex
	dataDir                      string
	migrationPlanQuoteTrustRoots []MigrationPlanQuoteTrustRoot
	depositorTrustRoots          []DepositorTrustRoot
	custodianTrustRoots          []CustodianTrustRoot
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

func WithDepositorTrustRoots(
	trustRoots []DepositorTrustRoot,
) ServiceOption {
	cloned := append([]DepositorTrustRoot{}, trustRoots...)

	return func(service *Service) {
		service.depositorTrustRoots = cloned
	}
}

func WithCustodianTrustRoots(
	trustRoots []CustodianTrustRoot,
) ServiceOption {
	cloned := append([]CustodianTrustRoot{}, trustRoots...)

	return func(service *Service) {
		service.custodianTrustRoots = cloned
	}
}

func WithCurrentBlockProvider(engine Engine) ServiceOption {
	var provider func() (uint64, error)
	if cbp, ok := engine.(CurrentBlockHeightProvider); ok {
		provider = func() (uint64, error) {
			return cbp.CurrentBlockHeight(context.Background())
		}
	}

	return func(service *Service) {
		service.currentBlockProvider = provider
	}
}

// WithMaxInFlight sets the maximum number of submissions that may be in
// flight (waiting for signature) at any time. When n > 0, a semaphore
// channel of size n is created; submissions acquire a slot before
// proceeding and release it when the signature response is received.
// When n <= 0, the limit is disabled: all submissions proceed immediately
// without waiting. Defaults to 0 (disabled).
func WithMaxInFlight(n int) ServiceOption {
	return func(service *Service) {
		service.maxInFlight = n
	}
}

func WithSignerApprovalVerifier(
	verifier SignerApprovalVerifier,
) ServiceOption {
	return func(service *Service) {
		service.signerApprovalVerifier = verifier
	}
}

// WithDataDir sets the data directory path for file-level locking. When
// provided, the store acquires an exclusive advisory lock to prevent
// concurrent process corruption. When empty, file locking is skipped.
func WithDataDir(dataDir string) ServiceOption {
	return func(service *Service) {
		service.dataDir = dataDir
	}
}

func NewService(
	handle persistence.BasicHandle,
	engine Engine,
	options ...ServiceOption,
) (_ *Service, retErr error) {
	if engine == nil {
		engine = NewPassiveEngine()
	}

	service := &Service{
		engine: engine,
		now:    func() time.Time { return time.Now().UTC() },
	}
	if verifier, ok := engine.(SignerApprovalVerifier); ok {
		service.signerApprovalVerifier = verifier
	}
	// Auto-detect the host-chain block provider so production correctness does
	// not depend on callers remembering the otherwise-redundant
	// WithCurrentBlockProvider option. An explicit option may still override
	// this (e.g. for test doubles).
	if provider, ok := engine.(CurrentBlockHeightProvider); ok {
		service.currentBlockProvider = func() (uint64, error) {
			return provider.CurrentBlockHeight(context.Background())
		}
	}
	for _, option := range options {
		option(service)
	}

	if service.maxInFlight > 0 {
		service.inFlightSlots = make(chan struct{}, service.maxInFlight)
	}

	store, err := NewStore(handle, service.dataDir)
	if err != nil {
		return nil, err
	}
	service.store = store
	// Release the file lock if any subsequent initialization step fails.
	defer func() {
		if retErr != nil {
			if closeErr := service.store.Close(); closeErr != nil {
				logger.Warnf("failed to close store after init failure: [%v]", closeErr)
			}
		}
	}()

	normalizedDepositorTrustRoots, err := normalizeDepositorTrustRoots(
		service.depositorTrustRoots,
	)
	if err != nil {
		return nil, err
	}
	service.depositorTrustRoots = normalizedDepositorTrustRoots

	normalizedCustodianTrustRoots, err := normalizeCustodianTrustRoots(
		service.custodianTrustRoots,
	)
	if err != nil {
		return nil, err
	}
	service.custodianTrustRoots = normalizedCustodianTrustRoots

	for i := range service.migrationPlanQuoteTrustRoots {
		trimmed := strings.TrimSpace(service.migrationPlanQuoteTrustRoots[i].KeyID)
		if trimmed == "" {
			return nil, fmt.Errorf("migration plan quote trust root KeyID at index %d is empty after trimming", i)
		}
		service.migrationPlanQuoteTrustRoots[i].KeyID = trimmed
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

// currentBlockForRequest returns the current host-chain height used to evaluate
// the request's certificate expiration, or nil when the request carries no
// signer approval certificate or no block-height provider is configured. When
// a certificate is present without a provider, nil is returned so that
// validateCommonRequest can fail the request closed. Provider errors are
// propagated so callers fail closed rather than proceeding blind.
func (s *Service) currentBlockForRequest(request RouteSubmitRequest) (*uint64, error) {
	if request.SignerApproval == nil || s.currentBlockProvider == nil {
		return nil, nil
	}

	currentBlock, err := s.currentBlockProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block height: %w", err)
	}

	return &currentBlock, nil
}

// ensureStoredCertificateTimely rejects a stored job whose signer approval
// certificate has expired at the current host-chain height. It fails closed
// when a certificate is present but its expiration block or a block-height
// provider is missing, and propagates provider errors.
func (s *Service) ensureStoredCertificateTimely(job *Job) error {
	signerApproval := job.Request.SignerApproval
	if signerApproval == nil {
		return nil
	}
	if signerApproval.EndBlock == nil {
		return &inputError{"stored signer approval certificate is missing endBlock"}
	}
	if s.currentBlockProvider == nil {
		return &inputError{
			"signer approval certificate cannot be verified without a current block height provider",
		}
	}

	currentBlock, err := s.currentBlockProvider()
	if err != nil {
		return fmt.Errorf("failed to get current block height: %w", err)
	}
	if certificateExpired(currentBlock, *signerApproval.EndBlock) {
		return &inputError{"signer approval certificate has expired"}
	}

	return nil
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

	// Verify this job is still the current holder of its route key. A Put()
	// for a newer job may have evicted the in-memory entry while the file
	// delete failed, leaving a stale byRequestID entry. If the route key
	// now points to a different request, treat this job as not found.
	holder, holderOk, holderErr := s.store.GetByRouteRequest(route, job.RouteRequestID)
	if holderErr != nil || !holderOk || holder.RequestID != job.RequestID {
		return nil, errJobNotFound
	}

	// Reject the poll if the stored job's signer approval certificate has
	// expired since submit, to avoid producing a signature under an
	// authorization that is no longer valid. loadPollJob is called both before
	// and after OnPoll, so this guards against expiry reached while signing
	// work is in flight. The comparison is inclusive (currentBlock > EndBlock);
	// see certificateExpired.
	if err := s.ensureStoredCertificateTimely(job); err != nil {
		return nil, err
	}

	digest, err := requestDigest(
		input.Request,
		validationOptions{
			policyIndependentDigest: true,
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

// createOrDedup creates a new job under the service mutex, or returns the
// existing job result if the route request is already known. Returns
// (job, nil, nil) for a new job, or (nil, result, nil) for a dedup hit.
func (s *Service) createOrDedup(
	route TemplateID,
	input SignerSubmitInput,
	normalizedRequest RouteSubmitRequest,
	requestDigest string,
) (*Job, *StepResult, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if existing, ok, err := s.store.GetByRouteRequest(route, input.RouteRequestID); err != nil {
		return nil, nil, err
	} else if ok {
		if existing.RequestDigest != requestDigest {
			return nil, nil, &inputError{
				"routeRequestId already exists with a different request payload",
			}
		}
		result := mapJobResult(existing)
		return nil, &result, nil
	}

	requestIDPrefix := ""
	switch route {
	case TemplateQcV1:
		requestIDPrefix = "kcs_qc"
	case TemplateSelfV1:
		requestIDPrefix = "kcs_self"
	default:
		return nil, nil, fmt.Errorf("unsupported route: %s", route)
	}

	requestID, err := newRequestID(requestIDPrefix)
	if err != nil {
		return nil, nil, err
	}

	now := s.now()

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
		return nil, nil, err
	}

	return job, nil, nil
}

func (s *Service) Submit(ctx context.Context, route TemplateID, input SignerSubmitInput) (StepResult, error) {
	// Resolve the current host-chain height up front so certificate expiry is
	// enforced during Submit validation. Fetching before validation means a
	// provider error fails the submit closed instead of proceeding to sign.
	currentBlock, err := s.currentBlockForRequest(input.Request)
	if err != nil {
		return StepResult{}, err
	}

	submitValidationOptions := validationOptions{
		migrationPlanQuoteTrustRoots:      s.migrationPlanQuoteTrustRoots,
		depositorTrustRoots:               s.depositorTrustRoots,
		custodianTrustRoots:               s.custodianTrustRoots,
		requireFreshMigrationPlanQuote:    true,
		migrationPlanQuoteVerificationNow: s.now(),
		signerApprovalVerifier:            s.signerApprovalVerifier,
		currentBlock:                      currentBlock,
	}
	if err := validateSubmitInput(route, input, submitValidationOptions); err != nil {
		return StepResult{}, err
	}

	normalizedRequest, err := normalizeRouteSubmitRequest(
		input.Request,
		validationOptions{
			migrationPlanQuoteTrustRoots: s.migrationPlanQuoteTrustRoots,
			depositorTrustRoots:          s.depositorTrustRoots,
			custodianTrustRoots:          s.custodianTrustRoots,
			signerApprovalVerifier:       s.signerApprovalVerifier,
		},
	)
	if err != nil {
		return StepResult{}, err
	}

	requestDigest, err := requestDigestFromNormalized(normalizedRequest)
	if err != nil {
		return StepResult{}, err
	}

	job, existingResult, err := s.createOrDedup(route, input, normalizedRequest, requestDigest)
	if err != nil {
		return StepResult{}, err
	}
	if existingResult != nil {
		return *existingResult, nil
	}

	if s.inFlightSlots != nil {
		select {
		case s.inFlightSlots <- struct{}{}:
		case <-ctx.Done():
			return StepResult{}, ctx.Err()
		}
		defer func() { <-s.inFlightSlots }()
	}

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

	currentJob, ok, err := s.store.GetByRequestID(job.RequestID)
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
	// Validate the resubmitted request against the actual current host-chain
	// height. When no provider is configured this returns nil so a certificate-
	// bearing request fails closed rather than being checked against a synthetic
	// zero block.
	currentBlock, err := s.currentBlockForRequest(input.Request)
	if err != nil {
		return StepResult{}, err
	}
	if err := validatePollInput(
		route,
		input,
		validationOptions{
			policyIndependentDigest: true,
			currentBlock:            currentBlock,
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

	if s.inFlightSlots != nil {
		select {
		case s.inFlightSlots <- struct{}{}:
		case <-ctx.Done():
			return StepResult{}, ctx.Err()
		}
		defer func() { <-s.inFlightSlots }()
	}

	transition, pollErr := s.engine.OnPoll(ctx, job)
	if pollErr != nil {
		if !errors.Is(pollErr, errJobNotFound) {
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

	if errors.Is(pollErr, errJobNotFound) {
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

// Close releases the resources held by the service, including the store's
// exclusive file lock when one was acquired.
func (s *Service) Close() error {
	if s.store != nil {
		return s.store.Close()
	}

	return nil
}
