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
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/staff"
)

func TestOpsCreateStaffHappy(t *testing.T) {
	id := uuid.New()
	branchID := uuid.New()
	svc := fakeStaffService{create: func(_ context.Context, gotBranch uuid.UUID, name, email, _ string, _ []auth.Permission) (*staff.Staff, error) {
		if gotBranch != branchID || name != "Sam" || email != "sam@example.com" {
			t.Fatalf("Create(%v, %q, %q)", gotBranch, name, email)
		}
		return &staff.Staff{ID: id, BranchID: gotBranch, Name: name, Email: email, Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + branchID.String() + `","name":"Sam","email":"sam@example.com"}`
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/staff", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Staff
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Sam" || string(got.Email) != "sam@example.com" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetStaffHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeStaffService{get: func(_ context.Context, got uuid.UUID) (*staff.Staff, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &staff.Staff{ID: id, BranchID: uuid.New(), Name: "Sam", Email: "sam@example.com", Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/staff/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Staff
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Sam" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetStaffNotFound(t *testing.T) {
	svc := fakeStaffService{get: func(context.Context, uuid.UUID) (*staff.Staff, error) {
		return nil, staff.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/staff/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "staff not found")
}

func TestOpsGetStaffInvalidID(t *testing.T) {
	svc := fakeStaffService{get: func(context.Context, uuid.UUID) (*staff.Staff, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/staff/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdateStaffHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeStaffService{update: func(_ context.Context, got uuid.UUID, patch staff.Patch) (*staff.Staff, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		return &staff.Staff{ID: id, Name: *patch.Name, Email: "sam@example.com", Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/staff/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Staff
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q, want %q", got.Name, "Renamed")
	}
}

func TestOpsUpdateStaffNotFound(t *testing.T) {
	svc := fakeStaffService{update: func(context.Context, uuid.UUID, staff.Patch) (*staff.Staff, error) {
		return nil, staff.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/staff/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "staff not found")
}

func TestOpsStaffFailBranchNotFound(t *testing.T) {
	svc := fakeStaffService{create: func(context.Context, uuid.UUID, string, string, string, []auth.Permission) (*staff.Staff, error) {
		return nil, staff.ErrBranchNotFound
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + uuid.New().String() + `","name":"Sam","email":"sam@example.com"}`
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/staff", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusNotFound, "branch not found")
}
