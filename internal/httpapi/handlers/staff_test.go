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
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/staff"
)

type fakeStaffService struct {
	get          func(ctx context.Context, id uuid.UUID) (*staff.Staff, error)
	create       func(ctx context.Context, branchID uuid.UUID, name, email, phone string, permissions []auth.Permission) (*staff.Staff, error)
	update       func(ctx context.Context, id uuid.UUID, patch staff.Patch) (*staff.Staff, error)
	listByBranch func(ctx context.Context, branchID uuid.UUID, p staff.BranchListParams) (*staff.BranchListResult, error)
}

func (f fakeStaffService) Get(ctx context.Context, id uuid.UUID) (*staff.Staff, error) {
	return f.get(ctx, id)
}
func (f fakeStaffService) Create(ctx context.Context, branchID uuid.UUID, name, email, phone string, permissions []auth.Permission) (*staff.Staff, error) {
	return f.create(ctx, branchID, name, email, phone, permissions)
}
func (f fakeStaffService) Update(ctx context.Context, id uuid.UUID, patch staff.Patch) (*staff.Staff, error) {
	return f.update(ctx, id, patch)
}
func (f fakeStaffService) ListByBranch(ctx context.Context, branchID uuid.UUID, p staff.BranchListParams) (*staff.BranchListResult, error) {
	return f.listByBranch(ctx, branchID, p)
}

func newStaffTestRouter(svc staffService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{staffHandler: &staffHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListBranchStaff(t *testing.T) {
	cursor := "abc"
	branchID := uuid.New()
	svc := fakeStaffService{listByBranch: func(_ context.Context, got uuid.UUID, p staff.BranchListParams) (*staff.BranchListResult, error) {
		if got != branchID {
			t.Fatalf("branchID = %v, want %v", got, branchID)
		}
		return &staff.BranchListResult{
			Items:      []*staff.Staff{{ID: uuid.New(), Name: "Sam", Email: "sam@example.com"}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+branchID.String()+"/staff?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.StaffPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "Sam" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListBranchStaffEmpty(t *testing.T) {
	svc := fakeStaffService{listByBranch: func(context.Context, uuid.UUID, staff.BranchListParams) (*staff.BranchListResult, error) {
		return &staff.BranchListResult{Items: []*staff.Staff{}}, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String()+"/staff", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListBranchStaffPassesParams(t *testing.T) {
	branchID := uuid.New()
	svc := fakeStaffService{listByBranch: func(_ context.Context, got uuid.UUID, p staff.BranchListParams) (*staff.BranchListResult, error) {
		if got != branchID {
			t.Fatalf("branchID = %v, want %v", got, branchID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("BranchListParams = %+v", p)
		}
		return &staff.BranchListResult{Items: []*staff.Staff{}}, nil
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+branchID.String()+"/staff?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListBranchStaffInvalid(t *testing.T) {
	svc := fakeStaffService{listByBranch: func(context.Context, uuid.UUID, staff.BranchListParams) (*staff.BranchListResult, error) {
		return nil, staff.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newStaffTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String()+"/staff?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
