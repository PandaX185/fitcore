package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/branches"
)

type fakeBranchService struct {
	get    func(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
	create func(ctx context.Context, name, address string, latitude, longitude float64) (*branches.Branch, error)
	update func(ctx context.Context, id uuid.UUID, patch branches.Patch) (*branches.Branch, error)
	list   func(ctx context.Context, p branches.ListParams) (*branches.ListResult, error)
}

func (f fakeBranchService) Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error) {
	return f.get(ctx, id)
}
func (f fakeBranchService) Create(ctx context.Context, name, address string, lat, lon float64) (*branches.Branch, error) {
	return f.create(ctx, name, address, lat, lon)
}
func (f fakeBranchService) Update(ctx context.Context, id uuid.UUID, patch branches.Patch) (*branches.Branch, error) {
	return f.update(ctx, id, patch)
}
func (f fakeBranchService) List(ctx context.Context, p branches.ListParams) (*branches.ListResult, error) {
	return f.list(ctx, p)
}

type apiError struct {
	Message string `json:"message"`
}

func newTestRouter(svc branchService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{branchesHandler: &branchesHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestGetBranch(t *testing.T) {
	id := uuid.New()
	want := oapi.Branch{Id: oapi.UUID(id), Name: "Central", Address: "1 Main St"}
	svc := fakeBranchService{get: func(ctx context.Context, got uuid.UUID) (*branches.Branch, error) {
		return &branches.Branch{ID: id, Name: "Central", Address: "1 Main St"}, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Branch
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got != want {
		t.Fatalf("body = %+v, want %+v", got, want)
	}
}

func TestGetBranchNotFound(t *testing.T) {
	svc := fakeBranchService{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String(), nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message != "branch not found" {
		t.Fatalf("message = %q, want %q", body.Error.Message, "branch not found")
	}
}

func TestGetBranchInvalidID(t *testing.T) {
	svc := fakeBranchService{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrInvalid
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/00000000-0000-0000-0000-000000000000", nil))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
	}
}

func TestGetBranchMalformedID(t *testing.T) {
	svc := fakeBranchService{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestGetBranchInternalError(t *testing.T) {
	svc := fakeBranchService{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, errors.New("boom")
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String(), nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message != "internal error" {
		t.Fatalf("message = %q, want %q", body.Error.Message, "internal error")
	}
}

func TestListBranches(t *testing.T) {
	a := uuid.New()
	cursor := "abc"
	svc := fakeBranchService{list: func(_ context.Context, p branches.ListParams) (*branches.ListResult, error) {
		return &branches.ListResult{
			Items:      []*branches.Branch{{ID: a, Name: "Alpha"}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches?q=Alpha&limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.BranchPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Id != oapi.UUID(a) || got.Items[0].Name != "Alpha" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListBranchesEmpty(t *testing.T) {
	svc := fakeBranchService{list: func(context.Context, branches.ListParams) (*branches.ListResult, error) {
		return &branches.ListResult{Items: []*branches.Branch{}}, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListBranchesPassesParams(t *testing.T) {
	svc := fakeBranchService{list: func(_ context.Context, p branches.ListParams) (*branches.ListResult, error) {
		if p.Query != "Alpha" || p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("ListParams = %+v", p)
		}
		return &branches.ListResult{Items: []*branches.Branch{}}, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches?q=Alpha&limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListBranchesInvalid(t *testing.T) {
	svc := fakeBranchService{list: func(context.Context, branches.ListParams) (*branches.ListResult, error) {
		return nil, branches.ErrInvalid
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches?cursor=bad", nil))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateBranch(t *testing.T) {
	id := uuid.New()
	svc := fakeBranchService{create: func(_ context.Context, name, address string, lat, lon float64) (*branches.Branch, error) {
		if name != "Central" || address != "1 Main St" || lat != 51.5 || lon != -0.1 {
			t.Fatalf("Create(%q, %q, %v, %v)", name, address, lat, lon)
		}
		return &branches.Branch{ID: id, Name: name, Address: address, Latitude: lat, Longitude: lon}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"name":"Central","address":"1 Main St","latitude":51.5,"longitude":-0.1}`
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/branches", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Branch
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Central" {
		t.Fatalf("body = %+v", got)
	}
}

func TestCreateBranchBadBody(t *testing.T) {
	svc := fakeBranchService{create: func(context.Context, string, string, float64, float64) (*branches.Branch, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/branches", strings.NewReader(`{not json`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateBranchInvalid(t *testing.T) {
	svc := fakeBranchService{create: func(context.Context, string, string, float64, float64) (*branches.Branch, error) {
		return nil, branches.ErrInvalid
	}}
	rec := httptest.NewRecorder()
	body := `{"name":"","address":"","latitude":0,"longitude":0}`
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/branches", strings.NewReader(body)))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateBranch(t *testing.T) {
	id := uuid.New()
	svc := fakeBranchService{update: func(_ context.Context, got uuid.UUID, patch branches.Patch) (*branches.Branch, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		if patch.Address != nil {
			t.Fatalf("patch address = %q, want nil", *patch.Address)
		}
		return &branches.Branch{ID: id, Name: *patch.Name}, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/branches/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Branch
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q, want %q", got.Name, "Renamed")
	}
}

func TestUpdateBranchNotFound(t *testing.T) {
	svc := fakeBranchService{update: func(context.Context, uuid.UUID, branches.Patch) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/branches/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateBranchBadBody(t *testing.T) {
	svc := fakeBranchService{update: func(context.Context, uuid.UUID, branches.Patch) (*branches.Branch, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/branches/"+uuid.New().String(), strings.NewReader(`{"name": 42}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
