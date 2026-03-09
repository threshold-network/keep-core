package covenantsigner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

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
	request := RouteSubmitRequest{
		FacadeRequestID:           "rf_123",
		IdempotencyKey:            "idem_123",
		Route:                     route,
		Strategy:                  "0x1234",
		Reserve:                   "0xabcd",
		Epoch:                     12,
		MaturityHeight:            912345,
		ActiveOutpoint:            CovenantOutpoint{TxID: "0x0102", Vout: 1, ScriptHash: "0x0304"},
		DestinationCommitmentHash: "0x0506",
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
