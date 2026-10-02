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
	"github.com/PandaX185/fitcore/internal/modules/packages"
)

func TestOpsCreatePackageHappy(t *testing.T) {
	id := uuid.New()
	svc := fakePackageService{create: func(_ context.Context, name string, duration int, price int64, currency string) (*packages.Package, error) {
		if name != "Monthly" || duration != 30 || price != 1000 || currency != "BHD" {
			t.Fatalf("Create(%q, %d, %d, %q)", name, duration, price, currency)
		}
		return &packages.Package{ID: id, Name: name, DurationDays: duration, PriceCents: price, Currency: currency, Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"name":"Monthly","durationDays":30,"priceCents":1000,"currency":"BHD"}`
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/packages", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Package
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Monthly" || got.PriceCents != 1000 {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetPackageHappy(t *testing.T) {
	id := uuid.New()
	svc := fakePackageService{get: func(_ context.Context, got uuid.UUID) (*packages.Package, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &packages.Package{ID: id, Name: "Monthly", DurationDays: 30, PriceCents: 1000, Currency: "BHD", Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Package
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Monthly" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetPackageNotFound(t *testing.T) {
	svc := fakePackageService{get: func(context.Context, uuid.UUID) (*packages.Package, error) {
		return nil, packages.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "package not found")
}

func TestOpsGetPackageInvalidID(t *testing.T) {
	svc := fakePackageService{get: func(context.Context, uuid.UUID) (*packages.Package, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdatePackageHappy(t *testing.T) {
	id := uuid.New()
	svc := fakePackageService{update: func(_ context.Context, got uuid.UUID, patch packages.Patch) (*packages.Package, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		return &packages.Package{ID: id, Name: *patch.Name, Active: true}, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/packages/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Package
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q, want %q", got.Name, "Renamed")
	}
}

func TestOpsUpdatePackageNotFound(t *testing.T) {
	svc := fakePackageService{update: func(context.Context, uuid.UUID, packages.Patch) (*packages.Package, error) {
		return nil, packages.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/packages/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "package not found")
}

func TestOpsPackagesFailDuplicateName(t *testing.T) {
	svc := fakePackageService{create: func(context.Context, string, int, int64, string) (*packages.Package, error) {
		return nil, packages.ErrDuplicateName
	}}
	rec := httptest.NewRecorder()
	body := `{"name":"Monthly","durationDays":30,"priceCents":1000,"currency":"BHD"}`
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/packages", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusConflict, "package name already in use")
}
