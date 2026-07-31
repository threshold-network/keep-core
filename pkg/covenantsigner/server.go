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
) (_ *Server, _ bool, err error) {
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

	isLoopback := isLoopbackListenAddress(listenAddress)

	if !isLoopback && strings.TrimSpace(config.AuthToken) == "" {
		return nil, false, fmt.Errorf(
			"covenant signer authToken is required for non-loopback listenAddress [%s]",
			listenAddress,
		)
	}

	eip712DomainOption, err := WithEIP712Domain(config.EIP712ChainID, config.EIP712Salt)
	if err != nil {
		return nil, false, err
	}

	service, err := NewService(
		handle,
		engine,
		WithDataDir(config.DataDir),
		WithMigrationPlanQuoteTrustRoots(config.MigrationPlanQuoteTrustRoots),
		WithDepositorTrustRoots(config.DepositorTrustRoots),
		WithCustodianTrustRoots(config.CustodianTrustRoots),
		WithCurrentBlockProvider(engine),
		eip712DomainOption,
	)
	if err != nil {
		return nil, false, err
	}
	// From this point on, service holds the store's exclusive file lock (when
	// DataDir is configured). Every subsequent startup failure below must
	// release it, or a later retry over the same data directory would fail
	// with a spurious lock-contention error instead of the actual startup
	// problem.
	defer func() {
		if err != nil {
			if closeErr := service.Close(); closeErr != nil {
				logger.Warnf("failed to close covenant signer service after startup failure: [%v]", closeErr)
			}
		}
	}()

	// A non-loopback (production) listen address is treated as requiring the
	// full multi-party approval model: the signer approval verifier and the
	// route trust roots must be configured, mirroring the non-loopback authToken
	// requirement above. Loopback deployments may still run with warnings unless
	// requireApprovalTrustRoots is set explicitly.
	if err := validateRequiredApprovalTrustRoots(config, service, !isLoopback); err != nil {
		return nil, false, err
	}
	if err := validateEIP712ChainIDForEthTrustRoots(service); err != nil {
		return nil, false, err
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
	} else if service.currentBlockProvider == nil {
		// An engine that verifies signer approvals but cannot report a
		// current block height could never determine whether a certificate
		// has expired: every certificate-bearing request would be rejected
		// at runtime. Fail startup instead of that deployment surprise.
		return nil, false, fmt.Errorf(
			"covenant signer engine implements SignerApprovalVerifier but not " +
				"CurrentBlockHeightProvider; signer approval certificate expiry " +
				"could never be determined -- implement CurrentBlockHeight on " +
				"the engine or use an engine that does not verify approvals",
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
	requireForNonLoopbackListenAddress bool,
) error {
	if !config.RequireApprovalTrustRoots && !requireForNonLoopbackListenAddress {
		return nil
	}

	if config.EnableSelfV1 &&
		!hasDepositorTrustRootForRoute(
			service.depositorTrustRoots,
			TemplateSelfV1,
		) {
		return fmt.Errorf(
			"covenant signer self_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true or for a non-loopback listen address",
		)
	}

	if !hasDepositorTrustRootForRoute(
		service.depositorTrustRoots,
		TemplateQcV1,
	) {
		return fmt.Errorf(
			"covenant signer qc_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true or for a non-loopback listen address",
		)
	}

	if !hasCustodianTrustRootForRoute(
		service.custodianTrustRoots,
		TemplateQcV1,
	) {
		return fmt.Errorf(
			"covenant signer qc_v1 routes require custodianTrustRoots when covenantSigner.requireApprovalTrustRoots=true or for a non-loopback listen address",
		)
	}

	if service.signerApprovalVerifier == nil {
		return fmt.Errorf(
			"covenant signer requires a signerApprovalVerifier when covenantSigner.requireApprovalTrustRoots=true or for a non-loopback listen address",
		)
	}

	return nil
}

// validateEIP712ChainIDForEthTrustRoots fails startup loudly when a depositor ETH
// identity is pinned but no EIP-712 chainId is configured. chainId 0 would
// silently produce a domain no real wallet signs against (eth_signTypedData_v4
// binds the wallet's active chain), so approvals would fail closed with no
// diagnostic.
func validateEIP712ChainIDForEthTrustRoots(service *Service) error {
	if service.eip712ChainID != 0 {
		return nil
	}
	for _, trustRoot := range service.depositorTrustRoots {
		if trustRoot.EthAddress != "" {
			return fmt.Errorf(
				"covenant signer depositorTrustRoots pin an ethAddress but " +
					"covenantSigner.eip712ChainId is unset; set it to the covenant's " +
					"Ethereum chainId so wallet-signed approvals can verify",
			)
		}
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
