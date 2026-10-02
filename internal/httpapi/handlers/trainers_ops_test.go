package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

func TestOpsCreateTrainerHappy(t *testing.T) {
	id := uuid.New()
	branchID := uuid.New()
	svc := fakeTrainerService{create: func(_ context.Context, gotBranch uuid.UUID, name, email, _ string) (*trainers.Trainer, error) {
		if gotBranch != branchID || name != "Tess" || email != "tess@example.com" {
			t.Fatalf("Create(%v, %q, %q)", gotBranch, name, email)
		}
		return &trainers.Trainer{ID: id, BranchID: gotBranch, Name: name, Email: email, Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + branchID.String() + `","name":"Tess","email":"tess@example.com"}`
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trainers", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Trainer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Tess" || string(got.Email) != "tess@example.com" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetTrainerHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeTrainerService{get: func(_ context.Context, got uuid.UUID) (*trainers.Trainer, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &trainers.Trainer{ID: id, BranchID: uuid.New(), Name: "Tess", Email: "tess@example.com", Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trainers/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Trainer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Tess" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetTrainerNotFound(t *testing.T) {
	svc := fakeTrainerService{get: func(context.Context, uuid.UUID) (*trainers.Trainer, error) {
		return nil, trainers.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trainers/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "trainer not found")
}

func TestOpsGetTrainerInvalidID(t *testing.T) {
	svc := fakeTrainerService{get: func(context.Context, uuid.UUID) (*trainers.Trainer, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trainers/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdateTrainerHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeTrainerService{update: func(_ context.Context, got uuid.UUID, patch trainers.Patch) (*trainers.Trainer, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		return &trainers.Trainer{ID: id, Name: *patch.Name, Email: "tess@example.com", Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/trainers/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Trainer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q, want %q", got.Name, "Renamed")
	}
}

func TestOpsUpdateTrainerNotFound(t *testing.T) {
	svc := fakeTrainerService{update: func(context.Context, uuid.UUID, trainers.Patch) (*trainers.Trainer, error) {
		return nil, trainers.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/trainers/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "trainer not found")
}

func TestOpsTrainersFailBranchNotFound(t *testing.T) {
	svc := fakeTrainerService{create: func(context.Context, uuid.UUID, string, string, string) (*trainers.Trainer, error) {
		return nil, trainers.ErrBranchNotFound
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + uuid.New().String() + `","name":"Tess","email":"tess@example.com"}`
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trainers", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusNotFound, "branch not found")
}
