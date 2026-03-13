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
		MigrationTransactionPlan: &MigrationTransactionPlan{
			PlanVersion:          migrationTransactionPlanVersion,
			InputValueSats:       1000000,
			DestinationValueSats: 998000,
			AnchorValueSats:      canonicalAnchorValueSats,
			FeeSats:              1670,
			InputSequence:        canonicalCovenantInputSequence,
			LockTime:             912345,
		},
		ArtifactSignatures: []string{"0x0708"},
		Artifacts:          map[RecoveryPathID]ArtifactRecord{},
	}

	switch route {
	case TemplateSelfV1:
		request.ScriptTemplate = validSelfTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: false}
	case TemplateQcV1:
		request.ScriptTemplate = validQcTemplate()
		request.Signing = SigningRequirements{SignerRequired: true, CustodianRequired: true}
	}

	request.MigrationTransactionPlan.PlanCommitmentHash, _ =
		computeMigrationTransactionPlanCommitmentHash(
			request,
			request.MigrationTransactionPlan,
		)

	return request
}

func validArtifactApprovals(request RouteSubmitRequest) *ArtifactApprovalEnvelope {
	approvals := []ArtifactRoleApproval{
		{Role: ArtifactApprovalRoleDepositor, Signature: "0xd0d0"},
		{Role: ArtifactApprovalRoleSigner, Signature: "0x5050"},
	}

	if request.Route == TemplateQcV1 {
		approvals = []ArtifactRoleApproval{
			{Role: ArtifactApprovalRoleDepositor, Signature: "0xd0d0"},
			{Role: ArtifactApprovalRoleCustodian, Signature: "0xc0c0"},
			{Role: ArtifactApprovalRoleSigner, Signature: "0x5050"},
		}
	}

	return &ArtifactApprovalEnvelope{
		Payload: ArtifactApprovalPayload{
			ApprovalVersion:           artifactApprovalVersion,
			Route:                     request.Route,
			ScriptTemplateID:          request.Route,
			DestinationCommitmentHash: request.DestinationCommitmentHash,
			PlanCommitmentHash:        request.MigrationTransactionPlan.PlanCommitmentHash,
		},
		Approvals: approvals,
	}
}

func canonicalArtifactApprovalRequest(route TemplateID) RouteSubmitRequest {
	request := baseRequest(route)
	request.ArtifactApprovals = validArtifactApprovals(request)
	request.ArtifactSignatures = []string{"0xd0d0", "0x5050"}
	if route == TemplateQcV1 {
		request.ArtifactSignatures = []string{"0xd0d0", "0xc0c0", "0x5050"}
	}

	return request
}

func upperHexBody(value string) string {
	if !strings.HasPrefix(value, "0x") {
		return strings.ToUpper(value)
	}

	return "0x" + strings.ToUpper(strings.TrimPrefix(value, "0x"))
}

func equivalentArtifactApprovalVariant(route TemplateID) RouteSubmitRequest {
	request := canonicalArtifactApprovalRequest(route)

	request.Strategy = upperHexBody(request.Strategy)
	request.Reserve = upperHexBody(request.Reserve)
	request.ActiveOutpoint.TxID = upperHexBody(request.ActiveOutpoint.TxID)
	request.ActiveOutpoint.ScriptHash = upperHexBody(request.ActiveOutpoint.ScriptHash)
	request.DestinationCommitmentHash = upperHexBody(request.DestinationCommitmentHash)
	request.MigrationDestination.Reserve = upperHexBody(request.MigrationDestination.Reserve)
	request.MigrationDestination.Revealer = upperHexBody(request.MigrationDestination.Revealer)
	request.MigrationDestination.Vault = upperHexBody(request.MigrationDestination.Vault)
	request.MigrationDestination.DepositScript = upperHexBody(request.MigrationDestination.DepositScript)
	request.MigrationDestination.DepositScriptHash = upperHexBody(request.MigrationDestination.DepositScriptHash)
	request.MigrationDestination.MigrationExtraData = upperHexBody(request.MigrationDestination.MigrationExtraData)
	request.MigrationDestination.DestinationCommitmentHash = upperHexBody(request.MigrationDestination.DestinationCommitmentHash)
	request.MigrationTransactionPlan.PlanCommitmentHash = upperHexBody(request.MigrationTransactionPlan.PlanCommitmentHash)
	for i := range request.ArtifactSignatures {
		request.ArtifactSignatures[i] = upperHexBody(request.ArtifactSignatures[i])
	}

	if route == TemplateQcV1 {
		request.ScriptTemplate = mustTemplate(QcV1Template{
			Template:           TemplateQcV1,
			DepositorPublicKey: upperHexBody("0x021111"),
			CustodianPublicKey: upperHexBody("0x023333"),
			SignerPublicKey:    upperHexBody("0x022222"),
			Beta:               144,
			Delta2:             4320,
		})
		request.ArtifactApprovals.Payload.DestinationCommitmentHash = upperHexBody(
			request.ArtifactApprovals.Payload.DestinationCommitmentHash,
		)
		request.ArtifactApprovals.Payload.PlanCommitmentHash = upperHexBody(
			request.ArtifactApprovals.Payload.PlanCommitmentHash,
		)
		request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
			{
				Role:      ArtifactApprovalRoleSigner,
				Signature: upperHexBody("0x5050"),
			},
			{
				Role:      ArtifactApprovalRoleDepositor,
				Signature: upperHexBody("0xd0d0"),
			},
			{
				Role:      ArtifactApprovalRoleCustodian,
				Signature: upperHexBody("0xc0c0"),
			},
		}
	} else {
		request.ScriptTemplate = mustTemplate(SelfV1Template{
			Template:           TemplateSelfV1,
			DepositorPublicKey: upperHexBody("0x021111"),
			SignerPublicKey:    upperHexBody("0x022222"),
			Delta2:             4320,
		})
		request.ArtifactApprovals.Payload.DestinationCommitmentHash = upperHexBody(
			request.ArtifactApprovals.Payload.DestinationCommitmentHash,
		)
		request.ArtifactApprovals.Payload.PlanCommitmentHash = upperHexBody(
			request.ArtifactApprovals.Payload.PlanCommitmentHash,
		)
		request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
			{
				Role:      ArtifactApprovalRoleSigner,
				Signature: upperHexBody("0x5050"),
			},
			{
				Role:      ArtifactApprovalRoleDepositor,
				Signature: upperHexBody("0xd0d0"),
			},
		}
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

func TestServiceSubmitReturnsExistingJobWhileInitialEngineCallIsInFlight(t *testing.T) {
	handle := newMemoryHandle()
	engineStarted := make(chan struct{})
	releaseEngine := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-engineStarted:
			default:
				close(engineStarted)
			}

			<-releaseEngine
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_inflight",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	firstResultChan := make(chan StepResult, 1)
	firstErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			firstErrChan <- err
			return
		}

		firstResultChan <- result
	}()

	<-engineStarted

	secondResultChan := make(chan StepResult, 1)
	secondErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			secondErrChan <- err
			return
		}

		secondResultChan <- result
	}()

	var secondResult StepResult
	select {
	case err := <-secondErrChan:
		t.Fatal(err)
	case secondResult = <-secondResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected deduplicated submit to return while initial engine call is in flight")
	}

	close(releaseEngine)

	var firstResult StepResult
	select {
	case err := <-firstErrChan:
		t.Fatal(err)
	case firstResult = <-firstResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected initial submit to finish after engine release")
	}

	if firstResult.RequestID == "" {
		t.Fatal("expected durable request id on initial submit")
	}
	if firstResult.RequestID != secondResult.RequestID {
		t.Fatalf("expected in-flight dedupe to reuse request id, got %s vs %s", firstResult.RequestID, secondResult.RequestID)
	}
}

func TestServicePollReturnsNewerPersistedStateWhenItsTransitionBecomesStale(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x090a",
				TransactionHex: "0x0b0c",
			}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
			return &Transition{State: JobStatePending, Detail: "stale pending"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_poll_stale_pending",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after engine release")
	}

	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected stale poll to return latest READY state, got %#v", pollResult)
	}
	if pollResult.PSBTHash != submitResult.PSBTHash || pollResult.TransactionHex != submitResult.TransactionHex {
		t.Fatalf("expected poll to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
	}

	persistedJob, ok, err := service.store.GetByRequestID(storedJob.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job after stale poll")
	}
	if persistedJob.State != JobStateArtifactReady {
		t.Fatalf("expected persisted READY state, got %s", persistedJob.State)
	}
}

func TestServiceSubmitReturnsNewerPersistedStateWhenItsTransitionBecomesStale(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{State: JobStatePending, Detail: "stale pending"}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
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

	input := SignerSubmitInput{
		RouteRequestID: "ors_submit_stale_pending",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after release")
	}

	if submitResult.Status != StepStatusReady {
		t.Fatalf("expected stale submit to return latest READY state, got %#v", submitResult)
	}
	if submitResult.PSBTHash != pollResult.PSBTHash || submitResult.TransactionHex != pollResult.TransactionHex {
		t.Fatalf("expected submit to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
	}

	persistedJob, ok, err := service.store.GetByRequestID(storedJob.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected persisted job after stale submit")
	}
	if persistedJob.State != JobStateArtifactReady {
		t.Fatalf("expected persisted READY state, got %s", persistedJob.State)
	}
}

func TestServicePollDoesNotOverwriteNewerPersistedStateWithJobNotFound(t *testing.T) {
	handle := newMemoryHandle()
	submitStarted := make(chan struct{})
	releaseSubmit := make(chan struct{})
	pollStarted := make(chan struct{})
	releasePoll := make(chan struct{})

	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			select {
			case <-submitStarted:
			default:
				close(submitStarted)
			}

			<-releaseSubmit
			return &Transition{
				State:          JobStateArtifactReady,
				Detail:         "artifact ready",
				PSBTHash:       "0x0d0e",
				TransactionHex: "0x0f10",
			}, nil
		},
		poll: func(*Job) (*Transition, error) {
			select {
			case <-pollStarted:
			default:
				close(pollStarted)
			}

			<-releasePoll
			return nil, errJobNotFound
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := SignerSubmitInput{
		RouteRequestID: "ors_poll_stale_missing",
		Stage:          StageSignerCoordination,
		Request:        baseRequest(TemplateSelfV1),
	}

	submitResultChan := make(chan StepResult, 1)
	submitErrChan := make(chan error, 1)
	go func() {
		result, err := service.Submit(context.Background(), TemplateSelfV1, input)
		if err != nil {
			submitErrChan <- err
			return
		}

		submitResultChan <- result
	}()

	<-submitStarted

	storedJob, ok, err := service.store.GetByRouteRequest(TemplateSelfV1, input.RouteRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected submitted job to exist while submit engine is in flight")
	}

	pollResultChan := make(chan StepResult, 1)
	pollErrChan := make(chan error, 1)
	go func() {
		result, err := service.Poll(context.Background(), TemplateSelfV1, SignerPollInput{
			RouteRequestID: input.RouteRequestID,
			RequestID:      storedJob.RequestID,
			Stage:          StageSignerCoordination,
			Request:        input.Request,
		})
		if err != nil {
			pollErrChan <- err
			return
		}

		pollResultChan <- result
	}()

	<-pollStarted
	close(releaseSubmit)

	var submitResult StepResult
	select {
	case err := <-submitErrChan:
		t.Fatal(err)
	case submitResult = <-submitResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected submit to finish after engine release")
	}

	close(releasePoll)

	var pollResult StepResult
	select {
	case err := <-pollErrChan:
		t.Fatal(err)
	case pollResult = <-pollResultChan:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected poll to finish after release")
	}

	if pollResult.Status != StepStatusReady {
		t.Fatalf("expected stale job-not-found poll to return latest READY state, got %#v", pollResult)
	}
	if pollResult.PSBTHash != submitResult.PSBTHash || pollResult.TransactionHex != submitResult.TransactionHex {
		t.Fatalf("expected poll to return persisted READY payload, got submit=%#v poll=%#v", submitResult, pollResult)
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

func TestServiceRejectsInvalidMigrationTransactionPlanVariants(t *testing.T) {
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
			name: "missing transaction plan",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan = nil
			},
			expectErr: "request.migrationTransactionPlan is required",
		},
		{
			name: "missing plan version",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanVersion = 0
			},
			expectErr: "request.migrationTransactionPlan.planVersion must equal 1",
		},
		{
			name: "wrong commitment hash",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanCommitmentHash = ""
			},
			expectErr: "request.migrationTransactionPlan.planCommitmentHash must be a 0x-prefixed even-length hex string",
		},
		{
			name: "zero input value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = 0
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must be greater than zero",
		},
		{
			name: "zero destination value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.DestinationValueSats = 0
			},
			expectErr: "request.migrationTransactionPlan.destinationValueSats must be greater than zero",
		},
		{
			name: "zero fee",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.FeeSats = 0
				request.MigrationTransactionPlan.DestinationValueSats =
					request.MigrationTransactionPlan.InputValueSats -
						request.MigrationTransactionPlan.AnchorValueSats
			},
			expectErr: "request.migrationTransactionPlan.feeSats must be greater than zero",
		},
		{
			name: "wrong anchor value",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.AnchorValueSats = 331
			},
			expectErr: "request.migrationTransactionPlan.anchorValueSats must equal the canonical 330 sat anchor",
		},
		{
			name: "wrong input sequence",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputSequence = 0xFFFFFFFF
			},
			expectErr: "request.migrationTransactionPlan.inputSequence must equal 0xFFFFFFFD",
		},
		{
			name: "locktime mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.LockTime = uint32(request.MaturityHeight + 1)
			},
			expectErr: "request.migrationTransactionPlan.lockTime must match request.maturityHeight",
		},
		{
			name: "maturity height exceeds uint32",
			mutate: func(request *RouteSubmitRequest) {
				request.MaturityHeight = 0x1_0000_0000
			},
			expectErr: "request.maturityHeight must fit in uint32",
		},
		{
			name: "insufficient input for destination",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = request.MigrationTransactionPlan.DestinationValueSats - 1
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must cover destinationValueSats",
		},
		{
			name: "insufficient input for anchor",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.InputValueSats = request.MigrationTransactionPlan.DestinationValueSats + canonicalAnchorValueSats - 1
			},
			expectErr: "request.migrationTransactionPlan.inputValueSats must cover anchorValueSats",
		},
		{
			name: "tampered commitment hash",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.PlanCommitmentHash = "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			},
			expectErr: "request.migrationTransactionPlan.planCommitmentHash does not match canonical migration transaction plan",
		},
		{
			name: "accounting mismatch",
			mutate: func(request *RouteSubmitRequest) {
				request.MigrationTransactionPlan.FeeSats++
			},
			expectErr: "request.migrationTransactionPlan values must satisfy inputValueSats = destinationValueSats + anchorValueSats + feeSats",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(TemplateSelfV1)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
				RouteRequestID: "ors_invalid_plan_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestServiceRejectsMigrationTransactionPlanBoundToDifferentDestinationCommitment(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateSelfV1)

	mutatedDestination := validMigrationDestination()
	mutatedDestination.Revealer = "0x4444444444444444444444444444444444444444"
	mutatedDestination.MigrationExtraData = computeMigrationExtraData(mutatedDestination.Revealer)
	mutatedDestination.DestinationCommitmentHash, err = computeDestinationCommitmentHash(mutatedDestination)
	if err != nil {
		t.Fatal(err)
	}

	request.DestinationCommitmentHash = mutatedDestination.DestinationCommitmentHash
	request.MigrationDestination = mutatedDestination

	_, err = service.Submit(context.Background(), TemplateSelfV1, SignerSubmitInput{
		RouteRequestID: "ors_invalid_plan_destination_binding",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err == nil || !strings.Contains(err.Error(), "request.migrationTransactionPlan.planCommitmentHash does not match canonical migration transaction plan") {
		t.Fatalf("expected plan binding error, got %v", err)
	}
}

func TestServiceAcceptsArtifactApprovalsWithCanonicalLegacySignatures(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := baseRequest(TemplateQcV1)
	request.ArtifactApprovals = validArtifactApprovals(request)
	request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
		request.ArtifactApprovals.Approvals[2],
		request.ArtifactApprovals.Approvals[0],
		request.ArtifactApprovals.Approvals[1],
	}
	request.ArtifactSignatures = []string{"0xd0d0", "0xc0c0", "0x5050"}

	result, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_artifact_approvals",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != StepStatusPending {
		t.Fatalf("expected PENDING, got %#v", result)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateQcV1, "orq_artifact_approvals")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored job")
	}
	if job.Request.ArtifactApprovals == nil {
		t.Fatal("expected stored artifact approvals")
	}
}

func TestServiceRejectsInvalidArtifactApprovalVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		route     TemplateID
		mutate    func(request *RouteSubmitRequest)
		expectErr string
	}{
		{
			name:  "missing qc custodian approval",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = validArtifactApprovals(*request)
				request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
					request.ArtifactApprovals.Approvals[0],
					request.ArtifactApprovals.Approvals[2],
				}
				request.ArtifactSignatures = []string{"0xd0d0", "0x5050"}
			},
			expectErr: "request.artifactApprovals.approvals must include role C for qc_v1",
		},
		{
			name:  "self route rejects custodian approval role",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = validArtifactApprovals(*request)
				request.ArtifactApprovals.Approvals = []ArtifactRoleApproval{
					request.ArtifactApprovals.Approvals[0],
					{Role: ArtifactApprovalRoleCustodian, Signature: "0xc0c0"},
					request.ArtifactApprovals.Approvals[1],
				}
				request.ArtifactSignatures = []string{"0xd0d0", "0xc0c0", "0x5050"}
			},
			expectErr: "request.artifactApprovals.approvals[1].role is not allowed for self_v1",
		},
		{
			name:  "plan commitment mismatch",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = validArtifactApprovals(*request)
				request.ArtifactApprovals.Payload.PlanCommitmentHash = "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
				request.ArtifactSignatures = []string{"0xd0d0", "0xc0c0", "0x5050"}
			},
			expectErr: "request.artifactApprovals.payload.planCommitmentHash must match request.migrationTransactionPlan.planCommitmentHash",
		},
		{
			name:  "legacy signature mismatch",
			route: TemplateQcV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = validArtifactApprovals(*request)
				request.ArtifactSignatures = []string{"0x5050", "0xd0d0", "0xc0c0"}
			},
			expectErr: "request.artifactSignatures must match canonical approval role order derived from request.artifactApprovals",
		},
		{
			name:  "legacy signatures remain required when approvals are present",
			route: TemplateSelfV1,
			mutate: func(request *RouteSubmitRequest) {
				request.ArtifactApprovals = validArtifactApprovals(*request)
				request.ArtifactSignatures = nil
			},
			expectErr: "request.artifactSignatures must not be empty",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(testCase.route)
			testCase.mutate(&request)

			_, err := service.Submit(context.Background(), testCase.route, SignerSubmitInput{
				RouteRequestID: "ors_invalid_artifact_approval_" + strings.ReplaceAll(testCase.name, " ", "_"),
				Stage:          StageSignerCoordination,
				Request:        request,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected %q, got %v", testCase.expectErr, err)
			}
		})
	}
}

func TestRequestDigestNormalizesEquivalentArtifactApprovalVariants(t *testing.T) {
	canonicalDigest, err := requestDigest(canonicalArtifactApprovalRequest(TemplateQcV1))
	if err != nil {
		t.Fatal(err)
	}

	variantDigest, err := requestDigest(equivalentArtifactApprovalVariant(TemplateQcV1))
	if err != nil {
		t.Fatal(err)
	}

	if canonicalDigest != variantDigest {
		t.Fatalf("expected matching request digest, got %s vs %s", canonicalDigest, variantDigest)
	}
}

func TestRequestDigestDoesNotEscapeHTMLSensitiveCharacters(t *testing.T) {
	request := canonicalArtifactApprovalRequest(TemplateSelfV1)
	request.FacadeRequestID = "rf_<tag>&sink"
	request.IdempotencyKey = "idem_>bridge"

	normalizedRequest, err := normalizeRouteSubmitRequest(request)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := marshalCanonicalJSON(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(payload, []byte(`"facadeRequestId":"rf_<tag>&sink"`)) {
		t.Fatalf("expected raw HTML-sensitive characters in payload, got %s", payload)
	}
	if bytes.Contains(payload, []byte(`\u003c`)) ||
		bytes.Contains(payload, []byte(`\u003e`)) ||
		bytes.Contains(payload, []byte(`\u0026`)) {
		t.Fatalf("expected unescaped HTML-sensitive characters in payload, got %s", payload)
	}

	digestFromRawRequest, err := requestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	digestFromNormalizedRequest, err := requestDigestFromNormalized(normalizedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if digestFromRawRequest != digestFromNormalizedRequest {
		t.Fatalf(
			"expected matching digests, got %s vs %s",
			digestFromRawRequest,
			digestFromNormalizedRequest,
		)
	}
}

func TestServicePollAcceptsEquivalentArtifactApprovalRequestVariants(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{
		submit: func(*Job) (*Transition, error) {
			return &Transition{State: JobStatePending, Detail: "queued"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	submitRequest := equivalentArtifactApprovalVariant(TemplateQcV1)
	submitResult, err := service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_equivalent_digest",
		Stage:          StageSignerCoordination,
		Request:        submitRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	pollResult, err := service.Poll(context.Background(), TemplateQcV1, SignerPollInput{
		RouteRequestID: "orq_equivalent_digest",
		RequestID:      submitResult.RequestID,
		Stage:          StageSignerCoordination,
		Request:        canonicalArtifactApprovalRequest(TemplateQcV1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if pollResult.Status != StepStatusPending {
		t.Fatalf("expected PENDING, got %#v", pollResult)
	}
}

func TestServiceStoresNormalizedArtifactApprovalRequest(t *testing.T) {
	handle := newMemoryHandle()
	service, err := NewService(handle, &scriptedEngine{})
	if err != nil {
		t.Fatal(err)
	}

	request := equivalentArtifactApprovalVariant(TemplateQcV1)
	_, err = service.Submit(context.Background(), TemplateQcV1, SignerSubmitInput{
		RouteRequestID: "orq_normalized_store",
		Stage:          StageSignerCoordination,
		Request:        request,
	})
	if err != nil {
		t.Fatal(err)
	}

	job, ok, err := service.store.GetByRouteRequest(TemplateQcV1, "orq_normalized_store")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored job")
	}

	expected := canonicalArtifactApprovalRequest(TemplateQcV1)
	if !reflect.DeepEqual(job.Request, expected) {
		t.Fatalf("expected normalized request %#v, got %#v", expected, job.Request)
	}
}

func TestRequestDigestRejectsArtifactApprovalsWithoutMigrationTransactionPlan(t *testing.T) {
	request := canonicalArtifactApprovalRequest(TemplateSelfV1)
	request.MigrationTransactionPlan = nil

	_, err := requestDigest(request)
	if err == nil || !strings.Contains(
		err.Error(),
		"request.migrationTransactionPlan is required when request.artifactApprovals is present",
	) {
		t.Fatalf("expected missing plan error, got %v", err)
	}
}

func TestMigrationTransactionPlanCommitmentHashMatchesCanonicalVectors(t *testing.T) {
	testCases := []struct {
		name     string
		request  RouteSubmitRequest
		plan     *MigrationTransactionPlan
		expected string
	}{
		{
			name: "canonical cross-stack vector",
			request: RouteSubmitRequest{
				Reserve:                   "0x2000000000000000000000000000000000000002",
				Epoch:                     12,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0x1111111111111111111111111111111111111111111111111111111111111111", Vout: 1},
				DestinationCommitmentHash: "0xf1b1739d99ea890ea6d419d6db28f4d5fe0871c32619a0984c1bfdbe4025f768",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       1_000_000,
				DestinationValueSats: 998_000,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              1_670,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             950000,
			},
			expected: "0x8dcafe57b888040d644e80dfd1b8b089dfd5016205d78316549ef71d032070f2",
		},
		{
			name: "mixed-case hex inputs normalize before hashing",
			request: RouteSubmitRequest{
				Reserve:                   "0xAbCd00000000000000000000000000000000Ef01",
				Epoch:                     0,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0xAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDdAaBbCcDd", Vout: 0},
				DestinationCommitmentHash: "0xFfEeDdCcBbAa00998877665544332211FfEeDdCcBbAa00998877665544332211",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       100_000,
				DestinationValueSats: 99_300,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              370,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             1,
			},
			expected: "0x626ce76714e04a41a5ec06a96082cac2ebd4d8f687fdc77766ffd9c0d11dac14",
		},
		{
			name: "max uint32 fields and large safe integer amounts remain stable",
			request: RouteSubmitRequest{
				Reserve:                   "0x9999999999999999999999999999999999999999",
				Epoch:                     4294967295,
				ActiveOutpoint:            CovenantOutpoint{TxID: "0x0000000000000000000000000000000000000000000000000000000000000001", Vout: 4294967295},
				DestinationCommitmentHash: "0x00000000000000000000000000000000000000000000000000000000000000aa",
			},
			plan: &MigrationTransactionPlan{
				PlanVersion:          migrationTransactionPlanVersion,
				PlanCommitmentHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
				InputValueSats:       9_007_199_254_740_000,
				DestinationValueSats: 9_007_199_254_737_000,
				AnchorValueSats:      canonicalAnchorValueSats,
				FeeSats:              2670,
				InputSequence:        canonicalCovenantInputSequence,
				LockTime:             0xffffffff,
			},
			expected: "0x42983bef3abb9680093ca0254c780c6ed4e6178405649bf1846ebb381ca89e02",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := computeMigrationTransactionPlanCommitmentHash(testCase.request, testCase.plan)
			if err != nil {
				t.Fatal(err)
			}

			if actual != testCase.expected {
				t.Fatalf("unexpected plan commitment hash: %s", actual)
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

	server := httptest.NewServer(newHandler(service, "", true))
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

	server := httptest.NewServer(newHandler(service, "", true))
	defer server.Close()

	base := baseRequest(TemplateSelfV1)
	payload := bytes.NewBufferString(fmt.Sprintf(`{
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
			"artifactSignatures":["0x0708"],
			"artifacts":{},
			"scriptTemplate":{"template":"self_v1","depositorPublicKey":"0x021111","signerPublicKey":"0x022222","delta2":4320},
			"signing":{"signerRequired":true,"custodianRequired":false},
			"futureField":"ignored"
		},
		"futureTopLevel":"ignored"
	}`, base.DestinationCommitmentHash, base.DestinationCommitmentHash, base.MigrationTransactionPlan.PlanCommitmentHash))

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

	server := httptest.NewServer(newHandler(service, "test-token", true))
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

	server := httptest.NewServer(newHandler(service, "", false))
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
