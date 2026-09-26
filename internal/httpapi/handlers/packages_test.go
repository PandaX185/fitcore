package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/packages"
)

type fakePackageService struct {
	get    func(ctx context.Context, id uuid.UUID) (*packages.Package, error)
	create func(ctx context.Context, name string, durationDays int, priceCents int64, currency string) (*packages.Package, error)
	update func(ctx context.Context, id uuid.UUID, patch packages.Patch) (*packages.Package, error)
	list   func(ctx context.Context, p packages.ListParams) (*packages.ListResult, error)
}

func (f fakePackageService) Get(ctx context.Context, id uuid.UUID) (*packages.Package, error) {
	return f.get(ctx, id)
}
func (f fakePackageService) Create(ctx context.Context, name string, durationDays int, priceCents int64, currency string) (*packages.Package, error) {
	return f.create(ctx, name, durationDays, priceCents, currency)
}
func (f fakePackageService) Update(ctx context.Context, id uuid.UUID, patch packages.Patch) (*packages.Package, error) {
	return f.update(ctx, id, patch)
}
func (f fakePackageService) List(ctx context.Context, p packages.ListParams) (*packages.ListResult, error) {
	return f.list(ctx, p)
}

func newPackagesTestRouter(svc packageService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{packagesHandler: &packagesHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListPackages(t *testing.T) {
	cursor := "abc"
	svc := fakePackageService{list: func(_ context.Context, p packages.ListParams) (*packages.ListResult, error) {
		return &packages.ListResult{
			Items:      []*packages.Package{{ID: uuid.New(), Name: "Monthly"}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.PackagePage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "Monthly" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListPackagesEmpty(t *testing.T) {
	svc := fakePackageService{list: func(context.Context, packages.ListParams) (*packages.ListResult, error) {
		return &packages.ListResult{Items: []*packages.Package{}}, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListPackagesPassesParams(t *testing.T) {
	svc := fakePackageService{list: func(_ context.Context, p packages.ListParams) (*packages.ListResult, error) {
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("ListParams = %+v", p)
		}
		return &packages.ListResult{Items: []*packages.Package{}}, nil
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListPackagesInvalid(t *testing.T) {
	svc := fakePackageService{list: func(context.Context, packages.ListParams) (*packages.ListResult, error) {
		return nil, packages.ErrInvalid
	}}
	rec := httptest.NewRecorder()
	newPackagesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/packages?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
