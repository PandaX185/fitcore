package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/classes"
)

func TestOpsCreateClassHappy(t *testing.T) {
	id := uuid.New()
	branchID := uuid.New()
	svc := fakeClassService{create: func(_ context.Context, gotBranch uuid.UUID, _ *uuid.UUID, name string, _, _ time.Time, capacity int) (*classes.Class, error) {
		if gotBranch != branchID || name != "Spin" || capacity != 20 {
			t.Fatalf("Create(%v, %q, capacity %d)", gotBranch, name, capacity)
		}
		return &classes.Class{ID: id, BranchID: gotBranch, Name: name, Capacity: capacity}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + branchID.String() + `","name":"Spin","startsAt":"2030-01-02T10:00:00Z","endsAt":"2030-01-02T11:00:00Z","capacity":20}`
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/classes", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Class
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Spin" || got.Capacity != 20 {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetClassHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeClassService{get: func(_ context.Context, got uuid.UUID) (*classes.Class, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &classes.Class{ID: id, BranchID: uuid.New(), Name: "Spin", Capacity: 20}, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Class
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Spin" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetClassNotFound(t *testing.T) {
	svc := fakeClassService{get: func(context.Context, uuid.UUID) (*classes.Class, error) {
		return nil, classes.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "class not found")
}

func TestOpsGetClassInvalidID(t *testing.T) {
	svc := fakeClassService{get: func(context.Context, uuid.UUID) (*classes.Class, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdateClassHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeClassService{update: func(_ context.Context, got uuid.UUID, patch classes.Patch) (*classes.Class, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		return &classes.Class{ID: id, Name: *patch.Name, Capacity: 20}, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/classes/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Class
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q, want %q", got.Name, "Renamed")
	}
}

func TestOpsUpdateClassNotFound(t *testing.T) {
	svc := fakeClassService{update: func(context.Context, uuid.UUID, classes.Patch) (*classes.Class, error) {
		return nil, classes.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/classes/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "class not found")
}

func TestOpsDeleteClassHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeClassService{delete: func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Fatalf("Delete id = %v, want %v", got, id)
		}
		return nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/classes/"+id.String(), nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsDeleteClassNotFound(t *testing.T) {
	svc := fakeClassService{delete: func(context.Context, uuid.UUID) error {
		return classes.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/classes/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "class not found")
}

func TestOpsClassesFailBranchNotFound(t *testing.T) {
	svc := fakeClassService{get: func(context.Context, uuid.UUID) (*classes.Class, error) {
		return nil, classes.ErrBranchNotFound
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "branch not found")
}
