package covenantsigner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedVerifierEngine struct {
	scriptedEngine
}

func (sve *scriptedVerifierEngine) VerifySignerApproval(RouteSubmitRequest) error {
	return nil
}

func TestServerHandlesSubmitAndPathPoll(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	response, err := http.Post(server.URL+"/v1/self_v1/signer/requests", "application/json", bytes.NewReader(submitPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	submitResult := StepResult{}
	if err := json.NewDecoder(response.Body).Decode(&submitResult); err != nil {
		t.Fatal(err)
	}

	pollPayload := mustJSON(t, SignerPollInput{
		RouteRequestID: "ors_http",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	pollResponse, err := http.Post(server.URL+"/v1/self_v1/signer/requests/"+submitResult.RequestID+":poll", "application/json", bytes.NewReader(pollPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer pollResponse.Body.Close()

	if pollResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pollResponse.Body)
		t.Fatalf("unexpected poll status: %d %s", pollResponse.StatusCode, string(body))
	}
}

func TestServerRejectsUnknownFieldsOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "", true))
	defer server.Close()

	base := baseRequest(TemplateSelfV1)
	template := &SelfV1Template{}
	if err := strictUnmarshal(base.ScriptTemplate, template); err != nil {
		t.Fatal(err)
	}
	payload := bytes.NewBufferString(fmt.Sprintf(`{
		"routeRequestId":"ors_http_unknown",
		"stage":"SIGNER_COORDINATION",
		"request":{
			"facadeRequestId":"rf_123",
			"idempotencyKey":"idem_123",
			"route":"self_v1",
			"requestType":"reconstruct",
			"strategy":"0x1234",
			"reserve":"0x1111111111111111111111111111111111111111",
			"epoch":12,
			"maturityHeight":912345,
			"activeOutpoint":{"txid":"0x0102","vout":1,"scriptHash":"0x0304"},
			"destinationCommitmentHash":"%s",
			"migrationDestination":{
				"reservationId":"cmdr_12345678",
				"reserve":"0x1111111111111111111111111111111111111111",
				"epoch":12,
				"route":"MIGRATION",
				"revealer":"0x2222222222222222222222222222222222222222",
				"vault":"0x3333333333333333333333333333333333333333",
				"network":"regtest",
				"status":"RESERVED",
				"depositScript":"0x0014aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"depositScriptHash":"0x8532ec6785e391b2af968b5728d574e271c7f46658f5ed10845d9ad5b23ac6d3",
				"migrationExtraData":"0x41435f4d49475241544556312222222222222222222222222222222222222222",
				"destinationCommitmentHash":"%s"
			},
			"migrationTransactionPlan":{
				"planVersion":1,
				"planCommitmentHash":"%s",
				"inputValueSats":1000000,
				"destinationValueSats":998000,
				"anchorValueSats":330,
				"feeSats":1670,
				"inputSequence":4294967293,
				"lockTime":912345
			},
			"artifactApprovals":{
				"payload":{
					"approvalVersion":1,
					"route":"self_v1",
					"scriptTemplateId":"self_v1",
					"destinationCommitmentHash":"%s",
					"planCommitmentHash":"%s"
				},
				"approvals":[
					{"role":"D","signature":"%s"}
				]
			},
			"artifactSignatures":["%s"],
			"artifacts":{},
			"scriptTemplate":{"template":"self_v1","depositorPublicKey":"%s","signerPublicKey":"%s","delta2":4320},
			"signing":{"signerRequired":true,"custodianRequired":false},
			"futureField":"ignored"
		},
		"futureTopLevel":"ignored"
	}`,
		base.DestinationCommitmentHash,
		base.DestinationCommitmentHash,
		base.MigrationTransactionPlan.PlanCommitmentHash,
		base.ArtifactApprovals.Payload.DestinationCommitmentHash,
		base.ArtifactApprovals.Payload.PlanCommitmentHash,
		base.ArtifactApprovals.Approvals[0].Signature,
		base.ArtifactSignatures[0],
		template.DepositorPublicKey,
		template.SignerPublicKey,
	))

	response, err := http.Post(server.URL+"/v1/self_v1/signer/requests", "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "malformed request body") {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestServerRejectsTrailingJSONOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "", true))
	defer server.Close()

	validPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http_trailing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	payload := append(validPayload, []byte(`{"unexpected":"trailing"}`)...)

	response, err := http.Post(
		server.URL+"/v1/self_v1/signer/requests",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "malformed request body") {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestInitializeRejectsInvalidOrUnavailablePort(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, enabled, err := Initialize(ctx, Config{Port: -1}, handle, nil); err == nil || enabled {
		t.Fatalf("expected invalid negative port to fail, got enabled=%v err=%v", enabled, err)
	}
	if _, enabled, err := Initialize(
		ctx,
		Config{Port: 9711, ListenAddress: "0.0.0.0"},
		handle,
		nil,
	); err == nil || enabled {
		t.Fatalf("expected non-loopback bind without auth token to fail, got enabled=%v err=%v", enabled, err)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(DefaultListenAddress, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	if _, enabled, err := Initialize(
		ctx,
		Config{Port: port, ListenAddress: DefaultListenAddress},
		handle,
		nil,
	); err == nil || enabled {
		t.Fatalf("expected occupied port to fail, got enabled=%v err=%v", enabled, err)
	}
}

func availableLoopbackPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort(DefaultListenAddress, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func TestInitializeRequiresQcV1DepositorTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			RequireApprovalTrustRoots: true,
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing qc_v1 depositor trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer qc_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeRequiresQcV1CustodianTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing qc_v1 custodian trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer qc_v1 routes require custodianTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeRequiresSelfV1DepositorTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			EnableSelfV1:              true,
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
			},
			CustodianTrustRoots: []CustodianTrustRoot{
				testCustodianTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected missing self_v1 depositor trust roots to fail, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"covenant signer self_v1 routes require depositorTrustRoots when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitializeAcceptsRequiredApprovalTrustRootsWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			EnableSelfV1:              true,
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
				testDepositorTrustRoot(TemplateSelfV1),
			},
			CustodianTrustRoots: []CustodianTrustRoot{
				testCustodianTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedVerifierEngine{},
	)
	if err != nil || !enabled || server == nil {
		t.Fatalf("expected startup to succeed with required trust roots, got enabled=%v server=%v err=%v", enabled, server != nil, err)
	}
}

func TestInitializeRequiresSignerApprovalVerifierWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, enabled, err := Initialize(
		ctx,
		Config{
			Port:                      availableLoopbackPort(t),
			EnableSelfV1:              true,
			RequireApprovalTrustRoots: true,
			DepositorTrustRoots: []DepositorTrustRoot{
				testDepositorTrustRoot(TemplateQcV1),
				testDepositorTrustRoot(TemplateSelfV1),
			},
			CustodianTrustRoots: []CustodianTrustRoot{
				testCustodianTrustRoot(TemplateQcV1),
			},
		},
		handle,
		&scriptedEngine{},
	)
	if err == nil || enabled {
		t.Fatalf("expected startup to fail without signer approval verifier, got enabled=%v err=%v", enabled, err)
	}
	if !strings.Contains(
		err.Error(),
		"requires a signerApprovalVerifier when covenantSigner.requireApprovalTrustRoots=true",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsLoopbackListenAddressAcceptsBracketedIPv6Loopback(t *testing.T) {
	if !isLoopbackListenAddress("[::1]") {
		t.Fatal("expected bracketed IPv6 loopback address to be recognized")
	}
}

func TestServerRequiresBearerTokenWhenConfigured(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "test-token", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http_auth",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	response, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected healthz status: %d", response.StatusCode)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected unauthorized submit without bearer token, got %d %s", response.StatusCode, string(body))
	}

	authorizedRequest, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorizedRequest.Header.Set("Content-Type", "application/json")
	authorizedRequest.Header.Set("Authorization", "Bearer test-token")

	authorizedResponse, err := http.DefaultClient.Do(authorizedRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer authorizedResponse.Body.Close()

	if authorizedResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authorizedResponse.Body)
		t.Fatalf("unexpected authorized submit status: %d %s", authorizedResponse.StatusCode, string(body))
	}
}

func TestServerCanKeepSelfV1RoutesDark(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "", false))
	defer server.Close()

	response, err := http.Post(
		server.URL+"/v1/self_v1/signer/requests",
		"application/json",
		bytes.NewReader(mustJSON(t, SignerSubmitInput{
			RouteRequestID: "ors_http_self_dark",
			Stage:          StageSignerCoordination,
			Request:        baseRequest(TemplateSelfV1),
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected disabled self_v1 route to return 404, got %d %s", response.StatusCode, string(body))
	}

	qcResponse, err := http.Post(
		server.URL+"/v1/qc_v1/signer/requests",
		"application/json",
		bytes.NewReader(mustJSON(t, SignerSubmitInput{
			RouteRequestID: "orq_http_qc",
			Stage:          StageSignerCoordination,
			Request:        baseRequest(TemplateQcV1),
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer qcResponse.Body.Close()

	if qcResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(qcResponse.Body)
		t.Fatalf("expected qc_v1 route to remain available, got %d %s", qcResponse.StatusCode, string(body))
	}
}

func TestServerBoundaryErrorMatrix(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service, context.Background(), "test-token", true))
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_http_matrix",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	mismatchedPollPayload := mustJSON(t, SignerPollInput{
		RequestID:      "different_id",
		RouteRequestID: "ors_http_matrix",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	oversizedBody := []byte(
		`{"routeRequestId":"ors_big","stage":"SIGNER_COORDINATION","request":{"facadeRequestId":"` +
			strings.Repeat("a", maxRequestBodyBytes+1) + `"}}`,
	)

	testCases := []struct {
		name             string
		method           string
		path             string
		body             []byte
		authHeader       string
		wantStatus       int
		wantBodyContains string
		wantAllow        string
	}{
		{
			name:             "invalid bearer token",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             submitPayload,
			authHeader:       "Bearer wrong-token",
			wantStatus:       http.StatusUnauthorized,
			wantBodyContains: "invalid bearer token",
		},
		{
			name:             "method mismatch on poll path returns 405",
			method:           http.MethodGet,
			path:             "/v1/self_v1/signer/requests/request_1:poll",
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusMethodNotAllowed,
			wantBodyContains: "method not allowed",
			wantAllow:        http.MethodPost,
		},
		{
			name:             "unknown fields in envelope rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             []byte(`{"routeRequestId":"ors_http_unknown","stage":"SIGNER_COORDINATION","request":{},"futureTopLevel":"ignored"}`),
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "malformed request body",
		},
		{
			name:             "oversized body rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests",
			body:             oversizedBody,
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "malformed request body",
		},
		{
			name:             "poll path and body request id mismatch rejected",
			method:           http.MethodPost,
			path:             "/v1/self_v1/signer/requests/request_from_path:poll",
			body:             mismatchedPollPayload,
			authHeader:       "Bearer test-token",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "requestId in body does not match path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(
				tc.method,
				server.URL+tc.path,
				bytes.NewReader(tc.body),
			)
			if err != nil {
				t.Fatal(err)
			}

			if tc.body != nil {
				request.Header.Set("Content-Type", "application/json")
			}
			if tc.authHeader != "" {
				request.Header.Set("Authorization", tc.authHeader)
			}

			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			if response.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(response.Body)
				t.Fatalf("unexpected status: %d body: %s", response.StatusCode, string(body))
			}

			if tc.wantAllow != "" && response.Header.Get("Allow") != tc.wantAllow {
				t.Fatalf("unexpected Allow header: %q", response.Header.Get("Allow"))
			}

			if tc.wantBodyContains != "" {
				body, _ := io.ReadAll(response.Body)
				if !strings.Contains(string(body), tc.wantBodyContains) {
					t.Fatalf("expected body to contain %q, got %q", tc.wantBodyContains, string(body))
				}
			}
		})
	}
}

// contextCapturingEngine is a test engine that passes the context through
// to its submit function, unlike scriptedEngine which drops it. This allows
// tests to verify context propagation behavior in the submit handler.
type contextCapturingEngine struct {
	submit func(ctx context.Context, job *Job) (*Transition, error)
}

func (cce *contextCapturingEngine) OnSubmit(ctx context.Context, job *Job) (*Transition, error) {
	if cce.submit == nil {
		return nil, nil
	}
	return cce.submit(ctx, job)
}

func (cce *contextCapturingEngine) OnPoll(context.Context, *Job) (*Transition, error) {
	return nil, nil
}

func TestSubmitHandlerDetachesContextFromHTTPLifecycle(t *testing.T) {
	// Channels for synchronizing the engine mock with the test goroutine.
	engineStarted := make(chan struct{})
	proceedCh := make(chan struct{})

	var capturedCtxErr error
	var mu sync.Mutex

	handle := newMemoryHandle()
	engine := &contextCapturingEngine{
		submit: func(ctx context.Context, job *Job) (*Transition, error) {
			// Signal that OnSubmit has started executing.
			close(engineStarted)
			// Wait for the test to cancel the HTTP request context.
			<-proceedCh
			// Capture whether the context received by OnSubmit was cancelled.
			mu.Lock()
			capturedCtxErr = ctx.Err()
			mu.Unlock()
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}

	service, err := NewService(handle, engine)
	if err != nil {
		t.Fatal(err)
	}

	handler := newHandler(service, context.Background(), "", true)

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_detach_cancel",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	// Use a cancellable context for the HTTP request to simulate the HTTP
	// write timeout or client disconnect that cancels r.Context().
	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodPost,
		"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	// Serve the request in a goroutine because the engine will block inside
	// OnSubmit until we signal proceedCh.
	serveDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, req)
		close(serveDone)
	}()

	// Wait for the engine to start processing the submit.
	select {
	case <-engineStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for engine to start")
	}

	// Cancel the request context, simulating a client disconnect or HTTP
	// write timeout expiring while signing is in progress.
	reqCancel()

	// Allow the engine to proceed and check the context it received.
	close(proceedCh)

	// Wait for the handler to finish.
	select {
	case <-serveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler to finish")
	}

	// The critical assertion: the context received by OnSubmit must NOT have
	// been cancelled even though the HTTP request context was.
	mu.Lock()
	defer mu.Unlock()
	if capturedCtxErr != nil {
		t.Fatalf(
			"expected OnSubmit context to be non-cancelled after HTTP "+
				"request cancellation, but got: %v",
			capturedCtxErr,
		)
	}
}

func TestSubmitHandlerPreCancelledContextStillSucceeds(t *testing.T) {
	var capturedCtxErr error
	var mu sync.Mutex

	handle := newMemoryHandle()
	engine := &contextCapturingEngine{
		submit: func(ctx context.Context, job *Job) (*Transition, error) {
			mu.Lock()
			capturedCtxErr = ctx.Err()
			mu.Unlock()
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}

	service, err := NewService(handle, engine)
	if err != nil {
		t.Fatal(err)
	}

	// Test through the handler directly using httptest.ResponseRecorder
	// because an HTTP client would fail to send a request with a
	// pre-cancelled context.
	handler := newHandler(service, context.Background(), "", true)

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_detach_precancel",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	// Create a pre-cancelled context.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	req, err := http.NewRequestWithContext(
		cancelledCtx,
		http.MethodPost,
		"/v1/self_v1/signer/requests",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// The handler should still succeed because the context passed to
	// service.Submit is detached from the HTTP request context.
	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected 200 OK with pre-cancelled context, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	mu.Lock()
	defer mu.Unlock()
	if capturedCtxErr != nil {
		t.Fatalf(
			"expected OnSubmit context to be non-cancelled with pre-cancelled "+
				"HTTP context, but got: %v",
			capturedCtxErr,
		)
	}
}

type contextKey string

func TestSubmitHandlerPreservesServiceContextValues(t *testing.T) {
	const testKey contextKey = "test-trace-id"
	const testValue = "trace-abc-123"

	var capturedValue any
	var mu sync.Mutex

	handle := newMemoryHandle()
	engine := &contextCapturingEngine{
		submit: func(ctx context.Context, job *Job) (*Transition, error) {
			mu.Lock()
			capturedValue = ctx.Value(testKey)
			mu.Unlock()
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	}

	service, err := NewService(handle, engine)
	if err != nil {
		t.Fatal(err)
	}

	// Inject a value into the service context. The submit handler derives
	// its timeout context from serviceCtx (not from the HTTP request), so
	// values on the service context must be visible to the engine.
	serviceCtx := context.WithValue(context.Background(), testKey, testValue)
	handler := newHandler(service, serviceCtx, "", true)

	server := httptest.NewServer(handler)
	defer server.Close()

	submitPayload := mustJSON(t, SignerSubmitInput{
		RouteRequestID: "ors_detach_values",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})

	response, err := http.Post(
		server.URL+"/v1/self_v1/signer/requests",
		"application/json",
		bytes.NewReader(submitPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}

	mu.Lock()
	defer mu.Unlock()
	if capturedValue != testValue {
		t.Fatalf(
			"expected service context value %q to be visible in engine, "+
				"got %v",
			testValue,
			capturedValue,
		)
	}
}
