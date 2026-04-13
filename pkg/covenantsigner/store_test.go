package covenantsigner

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestStoreReloadPreservesJobs(t *testing.T) {
	handle := newMemoryHandle()
	store, err := NewStore(handle, "")
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

	reloaded, err := NewStore(handle, "")
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

func TestStorePutReturnsErrorWhenSaveFails(t *testing.T) {
	handle := newFaultingMemoryHandle()
	handle.saveErrByName["kcs_self_fail_save.json"] = errors.New("injected save failure")

	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatal(err)
	}

	err = store.Put(&Job{
		RequestID:       "kcs_self_fail_save",
		RouteRequestID:  "ors_fail_save",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_fail_save",
		FacadeRequestID: "rf_fail_save",
		RequestDigest:   "0xdeadbeef",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	})
	if err == nil || !strings.Contains(err.Error(), "injected save failure") {
		t.Fatalf("expected injected save failure, got: %v", err)
	}

	_, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_fail_save")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no route mapping after failed save")
	}
}

func TestStorePutKeepsNewRouteMappingWhenOldDeleteFails(t *testing.T) {
	handle := newFaultingMemoryHandle()
	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatal(err)
	}

	initial := &Job{
		RequestID:       "kcs_self_old",
		RouteRequestID:  "ors_replace",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_old",
		FacadeRequestID: "rf_old",
		RequestDigest:   "0x1111",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(initial); err != nil {
		t.Fatal(err)
	}

	handle.deleteErrByName["kcs_self_old.json"] = errors.New("injected delete failure")

	replacement := &Job{
		RequestID:       "kcs_self_new",
		RouteRequestID:  "ors_replace",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_new",
		FacadeRequestID: "rf_new",
		RequestDigest:   "0x2222",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "2026-03-10T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	if err := store.Put(replacement); err != nil {
		t.Fatalf("expected replacement put to succeed, got: %v", err)
	}

	loaded, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_replace")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected route mapping to exist")
	}
	if loaded.RequestID != "kcs_self_new" {
		t.Fatalf("expected route key to map to replacement job, got: %s", loaded.RequestID)
	}
}

func TestStoreLoadSelectsNewestJobForDuplicateRouteKeys(t *testing.T) {
	handle := newMemoryHandle()

	oldJob := &Job{
		RequestID:       "kcs_self_old_load",
		RouteRequestID:  "ors_load_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_old_load",
		FacadeRequestID: "rf_old_load",
		RequestDigest:   "0xaaa",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}
	newJob := &Job{
		RequestID:       "kcs_self_new_load",
		RouteRequestID:  "ors_load_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_new_load",
		FacadeRequestID: "rf_new_load",
		RequestDigest:   "0xbbb",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "2026-03-10T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	oldPayload, err := json.Marshal(oldJob)
	if err != nil {
		t.Fatal(err)
	}
	newPayload, err := json.Marshal(newJob)
	if err != nil {
		t.Fatal(err)
	}

	if err := handle.Save(oldPayload, jobsDirectory, oldJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(newPayload, jobsDirectory, newJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatal(err)
	}

	loaded, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_load_dupe")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected loaded route mapping")
	}
	if loaded.RequestID != newJob.RequestID {
		t.Fatalf("expected newest request ID %s, got %s", newJob.RequestID, loaded.RequestID)
	}

	if _, ok, err := store.GetByRequestID(oldJob.RequestID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("expected superseded request ID %s to be removed", oldJob.RequestID)
	}
}

func TestStoreLoadResolvesInvalidUpdatedAtForDuplicateRouteKeys(t *testing.T) {
	handle := newMemoryHandle()

	first := &Job{
		RequestID:       "kcs_self_first_load",
		RouteRequestID:  "ors_load_invalid_updated_at",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_first_load",
		FacadeRequestID: "rf_first_load",
		RequestDigest:   "0xaaa",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}
	second := &Job{
		RequestID:       "kcs_self_second_load",
		RouteRequestID:  "ors_load_invalid_updated_at",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_second_load",
		FacadeRequestID: "rf_second_load",
		RequestDigest:   "0xbbb",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "invalid-timestamp",
		Request:         baseRequest(TemplateSelfV1),
	}

	firstPayload, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondPayload, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}

	if err := handle.Save(firstPayload, jobsDirectory, first.RequestID+".json"); err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(secondPayload, jobsDirectory, second.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatalf(
			"expected store to load despite invalid timestamp on duplicate route key, got error: %v",
			err,
		)
	}

	loaded, ok, err := store.GetByRouteRequest(TemplateSelfV1, "ors_load_invalid_updated_at")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected valid job to be loaded despite invalid-timestamp sibling")
	}
	if loaded.RequestID != first.RequestID {
		t.Fatalf("expected request ID %s, got %s", first.RequestID, loaded.RequestID)
	}

	if _, ok, err := store.GetByRequestID(second.RequestID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("expected invalid duplicate request ID %s to be removed", second.RequestID)
	}
}

func TestStoreLoadRejectsUnreadablePersistedJobFile(t *testing.T) {
	handle := newContentFaultingHandle()

	validJob := &Job{
		RequestID:       "kcs_self_valid_readable",
		RouteRequestID:  "ors_readable",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_readable",
		FacadeRequestID: "rf_readable",
		RequestDigest:   "0xaaa",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	payload, err := json.Marshal(validJob)
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(payload, jobsDirectory, validJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	handle.AddFaultingDescriptor(
		"corrupted_file.json",
		jobsDirectory,
		errors.New("simulated disk read error"),
	)

	_, err = NewStore(handle, "")
	if err == nil {
		t.Fatal("expected unreadable persisted job file to fail store load")
	}
	if !strings.Contains(err.Error(), "cannot read persisted covenant signer job file [corrupted_file.json]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreLoadRejectsMalformedPersistedJobFile(t *testing.T) {
	handle := newMemoryHandle()

	validJob := &Job{
		RequestID:       "kcs_self_valid_json",
		RouteRequestID:  "ors_valid_json",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_valid_json",
		FacadeRequestID: "rf_valid_json",
		RequestDigest:   "0xbbb",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	validPayload, err := json.Marshal(validJob)
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(validPayload, jobsDirectory, validJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	if err := handle.Save([]byte("not valid json content"), jobsDirectory, "malformed.json"); err != nil {
		t.Fatal(err)
	}

	_, err = NewStore(handle, "")
	if err == nil {
		t.Fatal("expected malformed persisted job file to fail store load")
	}
	if !strings.Contains(err.Error(), "cannot parse persisted covenant signer job file [malformed.json]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreLoadSkipsInvalidTimestampOnDuplicateRouteKey(t *testing.T) {
	handle := newMemoryHandle()

	validJob := &Job{
		RequestID:       "kcs_self_valid_ts",
		RouteRequestID:  "ors_ts_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_valid_ts",
		FacadeRequestID: "rf_valid_ts",
		RequestDigest:   "0xccc",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-09T00:00:00Z",
		UpdatedAt:       "2026-03-09T00:00:00Z",
		Request:         baseRequest(TemplateSelfV1),
	}

	badTimestampJob := &Job{
		RequestID:       "kcs_self_bad_ts",
		RouteRequestID:  "ors_ts_dupe",
		Route:           TemplateSelfV1,
		IdempotencyKey:  "idem_bad_ts",
		FacadeRequestID: "rf_bad_ts",
		RequestDigest:   "0xddd",
		State:           JobStatePending,
		Detail:          "queued",
		CreatedAt:       "2026-03-10T00:00:00Z",
		UpdatedAt:       "invalid-timestamp",
		Request:         baseRequest(TemplateSelfV1),
	}

	validPayload, err := json.Marshal(validJob)
	if err != nil {
		t.Fatal(err)
	}
	badPayload, err := json.Marshal(badTimestampJob)
	if err != nil {
		t.Fatal(err)
	}

	if err := handle.Save(validPayload, jobsDirectory, validJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}
	if err := handle.Save(badPayload, jobsDirectory, badTimestampJob.RequestID+".json"); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(handle, "")
	if err != nil {
		t.Fatalf("expected store to load despite invalid timestamp on duplicate route key, got error: %v", err)
	}

	loaded, ok, err := store.GetByRequestID("kcs_self_valid_ts")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected valid job to be accessible after skipping bad-timestamp sibling")
	}
	if loaded.RequestID != validJob.RequestID {
		t.Fatalf("expected request ID %s, got %s", validJob.RequestID, loaded.RequestID)
	}

	if _, ok, err := store.GetByRequestID(badTimestampJob.RequestID); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("expected invalid duplicate request ID %s to be removed", badTimestampJob.RequestID)
	}
}
