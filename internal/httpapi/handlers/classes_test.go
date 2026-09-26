package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/classes"
)

type fakeClassService struct {
	get    func(ctx context.Context, id uuid.UUID) (*classes.Class, error)
	create func(ctx context.Context, branchID uuid.UUID, trainerID *uuid.UUID, name string, startsAt, endsAt time.Time, capacity int) (*classes.Class, error)
	update func(ctx context.Context, id uuid.UUID, patch classes.Patch) (*classes.Class, error)
	delete func(ctx context.Context, id uuid.UUID) error
	list   func(ctx context.Context, p classes.ListParams) (*classes.ListResult, error)
}

func (f fakeClassService) Get(ctx context.Context, id uuid.UUID) (*classes.Class, error) {
	return f.get(ctx, id)
}
func (f fakeClassService) Create(ctx context.Context, branchID uuid.UUID, trainerID *uuid.UUID, name string, startsAt, endsAt time.Time, capacity int) (*classes.Class, error) {
	return f.create(ctx, branchID, trainerID, name, startsAt, endsAt, capacity)
}
func (f fakeClassService) Update(ctx context.Context, id uuid.UUID, patch classes.Patch) (*classes.Class, error) {
	return f.update(ctx, id, patch)
}
func (f fakeClassService) Delete(ctx context.Context, id uuid.UUID) error {
	return f.delete(ctx, id)
}
func (f fakeClassService) List(ctx context.Context, p classes.ListParams) (*classes.ListResult, error) {
	return f.list(ctx, p)
}

func newClassesTestRouter(svc classService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{classesHandler: &classesHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListClasses(t *testing.T) {
	cursor := "abc"
	svc := fakeClassService{list: func(_ context.Context, p classes.ListParams) (*classes.ListResult, error) {
		return &classes.ListResult{
			Items:      []*classes.Class{{ID: uuid.New(), Name: "Spin"}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.ClassPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "Spin" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListClassesEmpty(t *testing.T) {
	svc := fakeClassService{list: func(context.Context, classes.ListParams) (*classes.ListResult, error) {
		return &classes.ListResult{Items: []*classes.Class{}}, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListClassesPassesParams(t *testing.T) {
	branchID, trainerID := uuid.New(), uuid.New()
	svc := fakeClassService{list: func(_ context.Context, p classes.ListParams) (*classes.ListResult, error) {
		if p.BranchID == nil || *p.BranchID != branchID || p.TrainerID == nil || *p.TrainerID != trainerID {
			t.Fatalf("filters = %+v", p)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("ListParams = %+v", p)
		}
		return &classes.ListResult{Items: []*classes.Class{}}, nil
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/classes?branchId="+branchID.String()+"&trainerId="+trainerID.String()+"&limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListClassesInvalid(t *testing.T) {
	svc := fakeClassService{list: func(context.Context, classes.ListParams) (*classes.ListResult, error) {
		return nil, classes.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newClassesTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
