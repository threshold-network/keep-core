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
	eip712ChainID                uint64
	eip712Salt                   [32]byte
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

// WithEIP712Domain pins the EIP-712 domain (chainId + salt) used to compute the
// v2 domain-wrapped artifact approval digest. An empty saltHex falls back to the
// default program-namespace salt. The chainId and salt must match the client
// (wallet / covenant-manager / dashboard) domain construction.
func WithEIP712Domain(chainID uint64, saltHex string) (ServiceOption, error) {
	salt, err := ResolveEIP712DomainSalt(saltHex)
	if err != nil {
		return nil, err
	}

	return func(service *Service) {
		service.eip712ChainID = chainID
		service.eip712Salt = salt
	}, nil
}

// ResolveEIP712DomainSalt resolves the EIP-712 domain salt from its configured
// hex form, defaulting to the fixed program-namespace salt when empty. It is the
// single source of truth shared by the signer service and the tBTC engine so
// both compute identical approval digests.
func ResolveEIP712DomainSalt(saltHex string) ([32]byte, error) {
	if trimmed := strings.TrimSpace(saltHex); trimmed != "" {
		return decodeBytes32HexString("eip712Salt", trimmed)
	}
	return defaultArtifactApprovalDomainSalt, nil
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
	// Auto-detect the current block height provider from the engine, mirroring
	// the SignerApprovalVerifier auto-detection above. Correctness must not
	// depend on callers remembering to pass WithCurrentBlockProvider: a caller
	// that forgets it would otherwise silently get fail-open "never expires"
	// certificate handling. WithCurrentBlockProvider (below, via options) can
	// still override this when a caller genuinely needs a different provider
	// than the engine itself.
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

// currentBlockForRequest returns the current block height needed to validate
// request's signer approval certificate expiry, or nil if request carries no
// certificate at all, or the certificate has no EndBlock (expiry does not
// apply either way, and the provider is not queried unnecessarily). Gating on
// EndBlock here -- not just on SignerApproval being present -- keeps the
// required validation order (EndBlock structural presence before any
// provider query) even though this helper runs ahead of full request
// validation: a request with a present-but-nil EndBlock must never trigger a
// provider RPC before normalizeSignerApprovalCertificate gets a chance to
// reject it structurally. When request does carry a certificate with an
// EndBlock but no block height provider is configured, this returns (nil,
// nil) rather than a fabricated zero height: the absence stays
// distinguishable so callers fail closed instead of silently treating the
// certificate as unexpired. Provider errors are propagated so callers fail
// closed on provider failures too.
func (s *Service) currentBlockForRequest(request RouteSubmitRequest) (*uint64, error) {
	if request.SignerApproval == nil {
		return nil, nil
	}
	if request.SignerApproval.EndBlock == nil {
		return nil, nil
	}
	if s.currentBlockProvider == nil {
		return nil, nil
	}

	height, err := s.currentBlockProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block height: %w", err)
	}

	return &height, nil
}

// ensureStoredCertificateTimely rejects a job whose signer approval
// certificate has expired, whose EndBlock is missing (e.g. a legacy v1 job
// persisted before v2 enforcement rolled out), or whose expiry cannot be
// determined because no current block height provider is configured. Jobs
// without a signer approval certificate are always timely, since expiry only
// applies to certificate-bearing requests. This is shared by every place a
// durable job with a certificate is trusted again after having been
// persisted: loadPollJob's two call sites, and Submit's pre-lock and
// authoritative post-lock rechecks.
func (s *Service) ensureStoredCertificateTimely(job *Job) error {
	certificate := job.Request.SignerApproval
	if certificate == nil {
		return nil
	}
	if certificate.EndBlock == nil {
		return &inputError{"signer approval certificate has expired"}
	}

	currentBlock, err := s.currentBlockForRequest(job.Request)
	if err != nil {
		return err
	}
	if currentBlock == nil {
		return fmt.Errorf(
			"cannot determine signer approval certificate expiry: " +
				"no current block height provider is configured",
		)
	}
	if certificateExpired(*currentBlock, *certificate.EndBlock) {
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

	// Check if the signer approval certificate has expired since submit. If
	// expired, reject the poll to avoid producing a signature with an
	// authorization that is no longer valid.
	if err := s.ensureStoredCertificateTimely(job); err != nil {
		return nil, err
	}

	digest, err := requestDigest(
		input.Request,
		validationOptions{
			policyIndependentDigest: true,
			eip712ChainID:           s.eip712ChainID,
			eip712Salt:              s.eip712Salt,
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
//
// A dedup hit -- including one that resolves to an already-terminal
// ready/failed job -- is only returned after a fresh ensureStoredCertificateTimely
// check, run here while s.mutex is held. Without this, a certificate that was
// valid when the original job was created but has since expired (or whose
// provider now errors) could be echoed back as if it were still valid: the
// dedup path short-circuits Submit before any of its other expiry/provider
// rechecks ever run. On failure this returns the expiry/provider error and
// leaves the durable job untouched, the same fail-closed contract Submit
// itself uses for its own rechecks.
func (s *Service) createOrDedup(
	route TemplateID,
	input SignerSubmitInput,
	normalizedRequest RouteSubmitRequest,
	requestDigest string,
	depositorEthAddress string,
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
		if err := s.ensureStoredCertificateTimely(existing); err != nil {
			return nil, nil, err
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
		RequestID:           requestID,
		RouteRequestID:      input.RouteRequestID,
		Route:               route,
		IdempotencyKey:      input.Request.IdempotencyKey,
		FacadeRequestID:     input.Request.FacadeRequestID,
		RequestDigest:       requestDigest,
		DepositorEthAddress: depositorEthAddress,
		State:               JobStateSubmitted,
		Detail:              "accepted for covenant signing",
		CreatedAt:           now.Format(time.RFC3339Nano),
		UpdatedAt:           now.Format(time.RFC3339Nano),
		Request:             normalizedRequest,
	}

	if err := s.store.Put(job); err != nil {
		return nil, nil, err
	}

	return job, nil, nil
}

func (s *Service) Submit(ctx context.Context, route TemplateID, input SignerSubmitInput) (StepResult, error) {
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
		eip712ChainID:                     s.eip712ChainID,
		eip712Salt:                        s.eip712Salt,
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
			eip712ChainID:                s.eip712ChainID,
			eip712Salt:                   s.eip712Salt,
		},
	)
	if err != nil {
		return StepResult{}, err
	}

	requestDigest, err := requestDigestFromNormalized(normalizedRequest)
	if err != nil {
		return StepResult{}, err
	}

	// Pin the depositor's ETH identity (if any) to the durable job record now,
	// at submit time, using the exact same resolution validateSubmitInput just
	// used to decide whether this request's approval verified. Poll's
	// re-validation is policy-independent (see policyIndependentDigest) and
	// must not re-resolve depositorTrustRoots - which could change after
	// submit - so it reuses this pinned snapshot instead.
	depositorEthAddress := resolveExpectedDepositorEthAddress(input.Request, s.depositorTrustRoots)

	job, existingResult, err := s.createOrDedup(route, input, normalizedRequest, requestDigest, depositorEthAddress)
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

	// The certificate may have expired while this submission waited on the
	// signer approval verifier or for an in-flight slot. Recheck before
	// starting synchronous threshold signing so signing never begins on an
	// authorization that has already lapsed.
	if err := s.ensureStoredCertificateTimely(job); err != nil {
		return StepResult{}, err
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

	// Fast rejection path: synchronous threshold signing itself can take long
	// enough for the certificate to expire. Recheck before acquiring the
	// service mutex so an already-stale result does not even reach the
	// authoritative check below. This check is optimistic, not authoritative
	// -- the check after acquiring the mutex is what actually protects the
	// persisted state from a concurrent expiry.
	if err := s.ensureStoredCertificateTimely(job); err != nil {
		return StepResult{}, err
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

	// Authoritative recheck: the last point before any stored job state --
	// whether this call's own transition or a newer/terminal state a
	// concurrent Submit or Poll already persisted -- is persisted or
	// returned, running while s.mutex is held so no concurrent Submit or
	// Poll can advance the current height and slip past it. This closes the
	// TOCTOU window between the fast check above and the store write/return
	// below. It must run here, immediately after currentJob is loaded and
	// before the early-return just below: a concurrently-advanced or
	// terminal currentJob must never be handed back without this check, the
	// same fail-closed contract createOrDedup's dedup hit and loadPollJob's
	// every call site already use.
	if err := s.ensureStoredCertificateTimely(currentJob); err != nil {
		return StepResult{}, err
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
	currentBlock, err := s.currentBlockForRequest(input.Request)
	if err != nil {
		return StepResult{}, err
	}

	// Look up the depositor ETH address pinned on the job at submit time, if
	// any, so the signature re-verification below can use it. This must come
	// from the durable job record rather than from live depositorTrustRoots
	// config: policyIndependentDigest re-validation is deliberately isolated
	// from config that could have changed since submit, and unlike the
	// secp256k1 depositor key, the ETH identity has no equivalent field
	// embedded in the resubmitted request for Poll to read back directly.
	var pinnedDepositorEthAddress string
	if storedJob, ok, err := s.store.GetByRequestID(input.RequestID); err != nil {
		return StepResult{}, err
	} else if ok && storedJob.Route == route && storedJob.RouteRequestID == input.RouteRequestID {
		pinnedDepositorEthAddress = storedJob.DepositorEthAddress
	}

	if err := validatePollInput(
		route,
		input,
		validationOptions{
			policyIndependentDigest:   true,
			currentBlock:              currentBlock,
			pinnedDepositorEthAddress: pinnedDepositorEthAddress,
			eip712ChainID:             s.eip712ChainID,
			eip712Salt:                s.eip712Salt,
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
