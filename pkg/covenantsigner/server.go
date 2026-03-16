package covenantsigner

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-common/pkg/persistence"
)

var logger = log.Logger("keep-covenant-signer")

type Server struct {
	service    *Service
	httpServer *http.Server
}

const maxRequestBodyBytes = 2 << 20

func Initialize(
	ctx context.Context,
	config Config,
	handle persistence.BasicHandle,
	engine Engine,
) (*Server, bool, error) {
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
		WithMigrationPlanQuoteTrustRoots(config.MigrationPlanQuoteTrustRoots),
		WithDepositorTrustRoots(config.DepositorTrustRoots),
		WithCustodianTrustRoots(config.CustodianTrustRoots),
	)
	if err != nil {
		return nil, false, err
	}
	if err := validateRequiredApprovalTrustRoots(config, service); err != nil {
		return nil, false, err
	}
	if service.signerApprovalVerifier == nil {
		logger.Warn(
			"covenant signer started without a signer approval verifier; " +
				"structured signerApproval certificates will not be verified and " +
				"requests without signerApproval will be accepted",
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

	server := &Server{
		service: service,
		httpServer: &http.Server{
			Addr:              net.JoinHostPort(listenAddress, strconv.Itoa(config.Port)),
			Handler:           newHandler(service, config.AuthToken, config.EnableSelfV1),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 13,
		},
	}

	listener, err := net.Listen("tcp", server.httpServer.Addr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to bind covenant signer port [%d]: %w", config.Port, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.WithoutCancel(ctx),
			5*time.Second,
		)
		defer cancelShutdown()

		_ = server.httpServer.Shutdown(shutdownCtx)
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

func newHandler(service *Service, authToken string, enableSelfV1 bool) http.Handler {
	mux := http.NewServeMux()
	protectedHandler := withBearerAuth(mux, authToken)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("POST /v1/qc_v1/signer/requests", submitHandler(service, TemplateQcV1))
	mux.HandleFunc("POST /v1/qc_v1/signer/requests:poll", pollBodyHandler(service, TemplateQcV1))
	mux.HandleFunc("/v1/qc_v1/signer/requests/", pollPathHandler(service, TemplateQcV1))
	if enableSelfV1 {
		mux.HandleFunc("POST /v1/self_v1/signer/requests", submitHandler(service, TemplateSelfV1))
		mux.HandleFunc("POST /v1/self_v1/signer/requests:poll", pollBodyHandler(service, TemplateSelfV1))
		mux.HandleFunc("/v1/self_v1/signer/requests/", pollPathHandler(service, TemplateSelfV1))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
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
	if err := decoder.Decode(target); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

func submitHandler(service *Service, route TemplateID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := SignerSubmitInput{}
		if !decodeJSON(w, r, &input) {
			return
		}

		result, err := service.Submit(r.Context(), route, input)
		if err != nil {
			handleError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func pollBodyHandler(service *Service, route TemplateID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func pollPathHandler(service *Service, route TemplateID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		prefix := "/v1/" + string(route) + "/signer/requests/"
		if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, ":poll") {
			http.NotFound(w, r)
			return
		}

		pathRequestID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), ":poll")
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
