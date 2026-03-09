package covenantsigner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/persistence"
)

type memoryDescriptor struct {
	name      string
	directory string
	content   []byte
}

func (md *memoryDescriptor) Name() string      { return md.name }
func (md *memoryDescriptor) Directory() string { return md.directory }
func (md *memoryDescriptor) Content() ([]byte, error) {
	return md.content, nil
}

type memoryHandle struct {
	items map[string]*memoryDescriptor
}

func newMemoryHandle() *memoryHandle {
	return &memoryHandle{items: make(map[string]*memoryDescriptor)}
}

func (mh *memoryHandle) key(directory, name string) string {
	return directory + "/" + name
}

func (mh *memoryHandle) Save(data []byte, directory string, name string) error {
	mh.items[mh.key(directory, name)] = &memoryDescriptor{
		name:      name,
		directory: directory,
		content:   append([]byte{}, data...),
	}
	return nil
}

func (mh *memoryHandle) Delete(directory string, name string) error {
	delete(mh.items, mh.key(directory, name))
	return nil
}

func (mh *memoryHandle) ReadAll() (<-chan persistence.DataDescriptor, <-chan error) {
	dataChan := make(chan persistence.DataDescriptor, len(mh.items))
	errorChan := make(chan error)
	for _, item := range mh.items {
		dataChan <- item
	}
	close(dataChan)
	close(errorChan)
	return dataChan, errorChan
}

type scriptedEngine struct {
	submit func(*Job) (*Transition, error)
	poll   func(*Job) (*Transition, error)
}

func (se *scriptedEngine) OnSubmit(_ context.Context, job *Job) (*Transition, error) {
	if se.submit == nil {
		return nil, nil
	}
	return se.submit(job)
}

func (se *scriptedEngine) OnPoll(_ context.Context, job *Job) (*Transition, error) {
	if se.poll == nil {
		return nil, nil
	}
	return se.poll(job)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func validSelfTemplate() json.RawMessage {
	return mustTemplate(SelfV1Template{
		Template:           TemplateSelfV1,
		DepositorPublicKey: "0x021111",
		SignerPublicKey:    "0x022222",
		Delta2:             4320,
	})
}

func validQcTemplate() json.RawMessage {
	return mustTemplate(QcV1Template{
		Template:           TemplateQcV1,
		DepositorPublicKey: "0x021111",
		CustodianPublicKey: "0x023333",
		SignerPublicKey:    "0x022222",
		Beta:               144,
		Delta2:             4320,
	})
}

func mustTemplate(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func baseRequest(route TemplateID) RouteSubmitRequest {
	migrationDestination := validMigrationDestination()
	request := RouteSubmitRequest{
		FacadeRequestID:           "rf_123",
		IdempotencyKey:            "idem_123",
		Route:                     route,
		Strategy:                  "0x1234",
		Reserve:                   migrationDestination.Reserve,
		Epoch:                     12,
		MaturityHeight:            912345,
		ActiveOutpoint:            CovenantOutpoint{TxID: "0x0102", Vout: 1, ScriptHash: "0x0304"},
		DestinationCommitmentHash: migrationDestination.DestinationCommitmentHash,
		MigrationDestination:      migrationDestination,
		ArtifactSignatures:        []string{"0x0708"},
		Artifacts:                 map[RecoveryPathID]ArtifactRecord{},
	}

	switch route {
	case TemplateSelfV1:
		request.ScriptTemplate = validSelfTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: false}
	case TemplateQcV1:
		request.ScriptTemplate = validQcTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: true}
	}

	return request
}

func validMigrationDestination() *MigrationDestinationReservation {
	reservation := &MigrationDestinationReservation{
		ReservationID: "cmdr_12345678",
		Reserve:       "0x1111111111111111111111111111111111111111",
		Epoch:         12,
		Route:         ReservationRouteMigration,
		Revealer:      "0x2222222222222222222222222222222222222222",
		Vault:         "0x3333333333333333333333333333333333333333",
		Network:       "regtest",
		Status:        ReservationStatusReserved,
		DepositScript: "0x0014aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	reservation.DepositScriptHash, _ = computeDepositScriptHash(reservation.DepositScript)
	reservation.MigrationExtraData = computeMigrationExtraData(reservation.Revealer)
	reservation.DestinationCommitmentHash, _ = computeDestinationCommitmentHash(reservation)

	return reservation
}

func TestServiceSubmitDeduplicatesByRouteRequestID(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_123",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	first, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.Submit(context.Background(), TemplateSelfV1, input)
	if err != nil {
		t.Fatal(err)
	}

	if first.RequestID == "" {
		t.Fatal("expected durable request id")
	}
	if first.RequestID != second.RequestID {
		t.Fatalf("expected dedupe on routeRequestId, got %s vs %s", first.RequestID, second.RequestID)
	}
}

func TestServicePollCanTransitionToReady(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_ready",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_ready",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected READY, got %s", pollResult.Status)
	}
	if pollResult.PSBTHash != "0x090a" || pollResult.TransactionHex != "0x0b0c" {
		t.Fatalf("unexpected ready payload: %#v", pollResult)
	}
}

func TestServiceTimestampsAdvanceAcrossTransitions(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_timestamps",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	submittedJob, ok, err := service.store.GetByRequestID(submitResult.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job")
	}

	time.Sleep(5 * time.Millisecond)

	_, err = service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
		RouteRequestID: "ors_timestamps",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	polledJob, ok, err := service.store.GetByRequestID(submitResult.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected polled job")
	}

	if submittedJob.CreatedAt == polledJob.UpdatedAt {
		t.Fatalf("expected updated timestamp to advance, got created=%s updated=%s", submittedJob.CreatedAt, polledJob.UpdatedAt)
	}
	if polledJob.CompletedAt == "" {
		t.Fatal("expected completed timestamp to be populated")
	}
}

func TestServicePollMapsJobNotFoundToFailed(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			return nil, errJobNotFound
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitResult, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateQcV1, SignerPollInput{
		RouteRequestID: "orq_missing",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusFailed || pollResult.Reason != ReasonJobNotFound {
		t.Fatalf("unexpected failed result: %#v", pollResult)
	}
}

func TestMigrationDestinationMatchesKnownVector(t *testing.T) {
	reservation := validMigrationDestination()

	if reservation.DepositScriptHash != "0x8532ec6785e391b2af968b5728d574e271c7f46658f5ed10845d9ad5b23ac6d3" {
		t.Fatalf("unexpected depositScriptHash: %s", reservation.DepositScriptHash)
	}
	if reservation.MigrationExtraData != "0x41435f4d49475241544556312222222222222222222222222222222222222222" {
		t.Fatalf("unexpected migrationExtraData: %s", reservation.MigrationExtraData)
	}
	if reservation.DestinationCommitmentHash != "0x3efc50372759413e0f1900a2340fbb947648c524e5ec3cb4cf8887ea2d7df474" {
		t.Fatalf("unexpected destinationCommitmentHash: %s", reservation.DestinationCommitmentHash)
	}
}

func TestServiceRejectsMismatchedMigrationDestinationArtifact(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)
	request.MigrationDestination.DepositScriptHash = "0xdeadbeef"

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_bad_reservation",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "depositScriptHash does not match depositScript") {
		t.Fatalf("expected depositScriptHash mismatch, got %v", err)
	}
}

func TestServiceRejectsInvalidMigrationDestinationVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		mutate    func(request *RouteSubmitRequest)
		expectErr string
	}{
		{
			name: "missing reservation artifact",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination = nil
			},
			expectErr: "request.migrationDestination is required",
		},
		{
			name: "wrong reservation route",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Route = "COOPERATIVE"
			},
			expectErr: "request.migrationDestination.route must be MIGRATION",
		},
		{
			name: "retired reservation status",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Status = ReservationStatusRetired
			},
			expectErr: "request.migrationDestination.status must be RESERVED or COMMITTED_TO_EPOCH",
		},
		{
			name: "epoch mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Epoch = 13
			},
			expectErr: "request.migrationDestination.epoch does not match request.epoch",
		},
		{
			name: "reserve mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.Reserve = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			expectErr: "request.migrationDestination.reserve does not match request.reserve",
		},
		{
			name: "request commitment mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.DestinationCommitmentHash = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.destinationCommitmentHash does not match request.destinationCommitmentHash",
		},
		{
			name: "migration extraData mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.MigrationExtraData = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.migrationExtraData does not match migration revealer encoding",
		},
		{
			name: "canonical commitment mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationDestination.DestinationCommitmentHash = "0xdeadbeef"
				request.DestinationCommitmentHash = "0xdeadbeef"
			},
			expectErr: "request.migrationDestination.destinationCommitmentHash does not match canonical reservation artifact",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(TemplateSelfV1)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
				RouteRequestID: "ors_invalid_variant_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestStoreReloadPreservesJobs(t *testing.T) {
	handle := newMemoryHandle()
	store, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	job := &Job{
		RequestID:       "kcs_self_1234",
		RouteRequestID:  "ors_reload",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_reload",
		FacadeRequestID: "rf_reload",
		RequestDigest:   "0xdeadbeef",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(job); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewStore(handle)
	if err != nil {
		t.Fatal(err)
	}

	loadedJob, ok, err := reloaded.GetByRouteRequest(TemplateSelfV1, "ors_reload")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job")
	}
	if !reflect.DeepEqual(job.Request, loadedJob.Request) {
		t.Fatalf("unexpected reloaded request: %#v", loadedJob.Request)
	}
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

	server := httptest.NewServer(newHandler(service))
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

func TestServerIgnoresUnknownFieldsOnSubmit(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newHandler(service))
	defer server.Close()

	payload := bytes.NewBufferString(`{
		"routeRequestId":"ors_http_unknown",
		"stage":"SIGNER_COORDINATION",
		"request":{
			"facadeRequestId":"rf_123",
			"idempotencyKey":"idem_123",
			"route":"self_v1",
			"strategy":"0x1234",
			"reserve":"0x1111111111111111111111111111111111111111",
			"epoch":12,
			"maturityHeight":912345,
			"activeOutpoint":{"txid":"0x0102","vout":1,"scriptHash":"0x0304"},
			"destinationCommitmentHash":"0x3efc50372759413e0f1900a2340fbb947648c524e5ec3cb4cf8887ea2d7df474",
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
				"destinationCommitmentHash":"0x3efc50372759413e0f1900a2340fbb947648c524e5ec3cb4cf8887ea2d7df474"
			},
			"artifactSignatures":["0x0708"],
			"artifacts":{},
			"scriptTemplate":{"template":"self_v1","depositorPublicKey":"0x021111","signerPublicKey":"0x022222","delta2":4320},
			"signing":{"signerRequired":true,"custodianRequired":false},
			"futureField":"ignored"
		},
		"futureTopLevel":"ignored"
	}`)

	response, err := http.Post(server.URL+"/v1/self_v1/signer/requests", "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected submit status: %d %s", response.StatusCode, string(body))
	}
}

func TestInitializeRejectsInvalidOrUnavailablePort(t *testing.T) {
	handle := newMemoryHandle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, enabled, err := Initialize(ctx, Config{Port: -1}, handle); err == nil || enabled {
		t.Fatalf("expected invalid negative port to fail, got enabled=%v err=%v", enabled, err)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	if _, enabled, err := Initialize(ctx, Config{Port: port}, handle); err == nil || enabled {
		t.Fatalf("expected occupied port to fail, got enabled=%v err=%v", enabled, err)
	}
}
