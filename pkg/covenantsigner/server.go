package covenantsigner

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-common/pkg/persistence"
	"golang.org/x/time/rate"
)

var logger = log.Logger("keep-covenant-signer")

type Server struct {
	service       *Service
	cancelService context.CancelFunc
	httpServer    *http.Server
}

const maxRequestBodyBytes = 2 << 20

func Initialize(
	ctx context.Context,
	config Config,
	handle persistence.BasicHandle,
	engine Engine,
) (_ *Server, _ bool, retErr error) {
	if config.Port == 0 {
		return nil, false, nil
	}
	if config.Port < 0 || config.Port > 65535 {
		return nil, false, fmt.Errorf("invalid covenant signer port [%d]", config.Port)
	}

	listenAddress := config.ListenAddress
	if strings.TrimSpace(listenAddress) == "" {
		listenAddress = DefaultListenAddress
	}

	if !isLoopbackListenAddress(listenAddress) && strings.TrimSpace(config.AuthToken) == "" {
		return nil, false, fmt.Errorf(
			"covenant signer authToken is required for non-loopback listenAddress [%s]",
			listenAddress,
		)
	}

	service, err := NewService(
		handle,
		engine,
		WithDataDir(config.DataDir),
		WithMigrationPlanQuoteTrustRoots(config.MigrationPlanQuoteTrustRoots),
		WithDepositorTrustRoots(config.DepositorTrustRoots),
		WithCustodianTrustRoots(config.CustodianTrustRoots),
		WithCurrentBlockProvider(engine),
	)
	if err != nil {
		return nil, false, err
	}
	// Release the store's advisory file lock if initialization fails after the
	// service (and its store) were created but before the server takes ownership
	// of it. Without this, a rejected configuration would leak the lock and block
	// a corrected retry in the same process. On success retErr is nil and the
	// server's shutdown path owns closing the service instead.
	defer func() {
		if retErr != nil {
			if closeErr := service.Close(); closeErr != nil {
				logger.Warnf(
					"failed to close covenant signer service after initialization failure: [%v]",
					closeErr,
				)
			}
		}
	}()
	if err := validateRequiredApprovalTrustRoots(config, service); err != nil {
		return nil, false, err
	}
	// An engine that verifies signer approval certificates must also provide the
	// host-chain height used to enforce their expiration. Without a provider,
	// expiry would fail open, so reject this configuration before opening the
	// listener rather than accepting certificates that can never expire.
	if service.signerApprovalVerifier != nil && service.currentBlockProvider == nil {
		return nil, false, fmt.Errorf(
			"covenant signer engine implements SignerApprovalVerifier but not " +
				"CurrentBlockHeightProvider; signer approval certificate " +
				"expiration cannot be enforced -- use an engine that also " +
				"provides the current host-chain block height",
		)
	}
	if service.signerApprovalVerifier == nil {
		hasTrustRoots := len(service.depositorTrustRoots) > 0 ||
			len(service.custodianTrustRoots) > 0 ||
			len(service.migrationPlanQuoteTrustRoots) > 0
		if hasTrustRoots {
			return nil, false, fmt.Errorf(
				"trust roots are configured but the engine does not implement " +
					"SignerApprovalVerifier; signer approval certificates cannot " +
					"be verified -- remove trust root configuration or use an " +
					"engine that supports approval verification",
			)
		}
		logger.Warn(
			"covenant signer started without a signer approval verifier; " +
				"structured signerApproval certificates will not be verified and " +
				"requests without signerApproval will be accepted; " +
				"set covenantSigner.requireApprovalTrustRoots=true to enforce approval verification",
		)
	}
	if config.EnableSelfV1 &&
		!hasDepositorTrustRootForRoute(
			service.depositorTrustRoots,
			TemplateSelfV1,
		) {
		logger.Warn(
			"covenant signer self_v1 routes are enabled without depositorTrustRoots; " +
				"self_v1 depositor approvals still rely on request-supplied scriptTemplate keys",
		)
	}
	if !hasDepositorTrustRootForRoute(
		service.depositorTrustRoots,
		TemplateQcV1,
	) {
		logger.Warn(
			"covenant signer started without qc_v1 depositorTrustRoots; " +
				"qc_v1 depositor approvals still rely on request-supplied scriptTemplate keys",
		)
	}
	if !hasCustodianTrustRootForRoute(
		service.custodianTrustRoots,
		TemplateQcV1,
	) {
		logger.Warn(
			"covenant signer started without custodianTrustRoots; " +
				"qc_v1 custodian approvals still rely on request-supplied scriptTemplate keys",
		)
	}

	// Create a service-level context that outlives individual HTTP requests
	// but is cancelled on process shutdown, so in-flight threshold signing
	// operations are cancelled cleanly rather than killed mid-operation.
	serviceCtx, cancelService := context.WithCancel(context.WithoutCancel(ctx))

	server := &Server{
		service:       service,
		cancelService: cancelService,
		httpServer: &http.Server{
			Addr:              net.JoinHostPort(listenAddress, strconv.Itoa(config.Port)),
			Handler:           newHandler(service, serviceCtx, config.AuthToken, config.EnableSelfV1),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 13,
		},
	}

	listener, err := net.Listen("tcp", server.httpServer.Addr)
	if err != nil {
		cancelService()
		return nil, false, fmt.Errorf("failed to bind covenant signer port [%d]: %w", config.Port, err)
	}

	// #nosec G118 -- this goroutine intentionally uses context.Background for
	// the shutdown drain. ctx is already cancelled when <-ctx.Done() unblocks;
	// using it for the Shutdown timeout would cause immediate cancellation.
	go func() {
		<-ctx.Done()

		// Cancel the service context so in-flight threshold signing
		// operations observe shutdown and terminate promptly.
		cancelService()

		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancelShutdown()

		_ = server.httpServer.Shutdown(shutdownCtx)
		_ = server.service.Close()
	}()

	go func() {
		if err := server.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("covenant signer server failed: [%v]", err)
		}
	}()

	logger.Infof(
		"enabled covenant signer provider endpoint on [%v] auth=[%v] self_v1=[%v]",
		server.httpServer.Addr,
		strings.TrimSpace(config.AuthToken) != "",
		config.EnableSelfV1,
	)

	return server, true, nil
}

func validateRequiredApprovalTrustRoots(
	config Config,
	service *Service,
) error {
	if !config.RequireApprovalTrustRoots {
		return nil
	}

	if config.EnableSelfV1 &&
		!hasDepositorTrustRootForRoute(
			service.depositorTrustRoots,
			TemplateSelfV1,
		) {
		return fmt.Errorf(
			"covenant signer self_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
		)
	}

	if !hasDepositorTrustRootForRoute(
		service.depositorTrustRoots,
		TemplateQcV1,
	) {
		return fmt.Errorf(
			"covenant signer qc_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
		)
	}

	if !hasCustodianTrustRootForRoute(
		service.custodianTrustRoots,
		TemplateQcV1,
	) {
		return fmt.Errorf(
			"covenant signer qc_v1 routes require custodianTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
		)
	}

	if service.signerApprovalVerifier == nil {
		return fmt.Errorf(
			"covenant signer requires a signerApprovalVerifier when covenantSigner.requireApprovalTrustRoots=true",
		)
	}

	return nil
}

func hasDepositorTrustRootForRoute(
	trustRoots []DepositorTrustRoot,
	route TemplateID,
) bool {
	for _, trustRoot := range trustRoots {
		if trustRoot.Route == route {
			return true
		}
	}

	return false
}

func hasCustodianTrustRootForRoute(
	trustRoots []CustodianTrustRoot,
	route TemplateID,
) bool {
	for _, trustRoot := range trustRoots {
		if trustRoot.Route == route {
			return true
		}
	}

	return false
}

func newHandler(service *Service, serviceCtx context.Context, authToken string, enableSelfV1 bool) http.Handler {
	mux := http.NewServeMux()
	protectedHandler := withBearerAuth(mux, authToken)

	// Single rate limiter shared across all submit routes. The server uses one
	// static auth token, so this is equivalent to per-token limiting.
	submitLimiter := rate.NewLimiter(
		rate.Every(time.Minute/submitRateLimitPerMinute),
		submitRateLimitPerMinute,
	)

	// Poll operations are rate-limited to prevent a single client from
	// monopolising CPU time on signing operations. Each poll may trigger
	// an expensive threshold signing call, so even a modest burst can
	// degrade responsiveness for other clients.
	pollLimiter := rate.NewLimiter(
		rate.Every(time.Minute/pollRateLimitPerMinute),
		pollRateLimitPerMinute,
	)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("POST /v1/qc_v1/signer/requests", submitHandler(service, serviceCtx, TemplateQcV1, submitLimiter))
	mux.HandleFunc("POST /v1/qc_v1/signer/requests:poll", pollBodyHandler(service, TemplateQcV1, pollLimiter))
	mux.HandleFunc("/v1/qc_v1/signer/requests/", pollPathHandler(service, TemplateQcV1, pollLimiter))
	if enableSelfV1 {
		mux.HandleFunc("POST /v1/self_v1/signer/requests", submitHandler(service, serviceCtx, TemplateSelfV1, submitLimiter))
		mux.HandleFunc("POST /v1/self_v1/signer/requests:poll", pollBodyHandler(service, TemplateSelfV1, pollLimiter))
		mux.HandleFunc("/v1/self_v1/signer/requests/", pollPathHandler(service, TemplateSelfV1, pollLimiter))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			mux.ServeHTTP(w, r)
			return
		}

		protectedHandler.ServeHTTP(w, r)
	})
}

func isLoopbackListenAddress(address string) bool {
	trimmedAddress := strings.TrimSpace(address)
	if trimmedAddress == "" || strings.EqualFold(trimmedAddress, "localhost") {
		return true
	}

	normalizedAddress := trimmedAddress
	if strings.HasPrefix(normalizedAddress, "[") && strings.HasSuffix(normalizedAddress, "]") {
		normalizedAddress = normalizedAddress[1 : len(normalizedAddress)-1]
	}

	ip := net.ParseIP(normalizedAddress)
	return ip != nil && ip.IsLoopback()
}

func withBearerAuth(next http.Handler, authToken string) http.Handler {
	trimmedToken := strings.TrimSpace(authToken)
	if trimmedToken == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizationHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(authorizationHeader, prefix) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		presentedToken := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, prefix))
		if subtle.ConstantTimeCompare([]byte(presentedToken), []byte(trimmedToken)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, target *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func handleError(w http.ResponseWriter, err error) {
	var inputErr *inputError
	if errors.As(err, &inputErr) {
		http.Error(w, inputErr.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, errJobNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	logger.Errorf("covenant signer request failed: [%v]", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// Poll operations are rate-limited to prevent a single client from
// monopolising CPU time on signing operations. Each poll may trigger
// an expensive threshold signing call, so even a modest burst can
// degrade responsiveness for other clients.
const pollRateLimitPerMinute = 60

const submitTimeout = 5 * time.Minute

// submitRateLimitPerMinute is the maximum number of new submit requests
// accepted per minute. Each submit may initiate a 5-minute threshold signing
// operation, so even a small burst can saturate the engine. Idempotent polls
// and dedup hits are not affected by this limit.
const submitRateLimitPerMinute = 5

func submitHandler(service *Service, serviceCtx context.Context, route TemplateID, limiter *rate.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		input := SignerSubmitInput{}
		if !decodeJSON(w, r, &input) {
			return
		}

		// Use the service-level context (cancelled on process shutdown)
		// with a bounded timeout so threshold signing cannot hang
		// indefinitely. This detaches from the HTTP request lifetime so
		// signing survives write-timeout and client disconnects.
		submitCtx, cancelSubmit := context.WithTimeout(serviceCtx, submitTimeout)
		defer cancelSubmit()

		result, err := service.Submit(submitCtx, route, input)
		if err != nil {
			handleError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func pollBodyHandler(service *Service, route TemplateID, limiter *rate.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		input := SignerPollInput{}
		if !decodeJSON(w, r, &input) {
			return
		}

		result, err := service.Poll(r.Context(), route, input)
		if err != nil {
			handleError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func pollPathHandler(service *Service, route TemplateID, limiter *rate.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !limiter.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		prefix := "/v1/" + string(route) + "/signer/requests/"
		if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, ":poll") {
			http.NotFound(w, r)
			return
		}

		rawPathRequestID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), ":poll")
		pathRequestID, err := url.PathUnescape(rawPathRequestID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if pathRequestID == "" || strings.Contains(pathRequestID, "/") {
			http.NotFound(w, r)
			return
		}

		input := SignerPollInput{}
		if !decodeJSON(w, r, &input) {
			return
		}
		if input.RequestID != "" && input.RequestID != pathRequestID {
			http.Error(w, "requestId in body does not match path", http.StatusBadRequest)
			return
		}
		input.RequestID = pathRequestID

		result, err := service.Poll(r.Context(), route, input)
		if err != nil {
			handleError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}
