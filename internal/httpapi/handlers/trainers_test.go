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
	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

type fakeTrainerService struct {
	get          func(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error)
	create       func(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*trainers.Trainer, error)
	update       func(ctx context.Context, id uuid.UUID, patch trainers.Patch) (*trainers.Trainer, error)
	listByBranch func(ctx context.Context, branchID uuid.UUID, p trainers.BranchListParams) (*trainers.BranchListResult, error)
}

func (f fakeTrainerService) Get(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error) {
	return f.get(ctx, id)
}
func (f fakeTrainerService) Create(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*trainers.Trainer, error) {
	return f.create(ctx, branchID, name, email, phone)
}
func (f fakeTrainerService) Update(ctx context.Context, id uuid.UUID, patch trainers.Patch) (*trainers.Trainer, error) {
	return f.update(ctx, id, patch)
}
func (f fakeTrainerService) ListByBranch(ctx context.Context, branchID uuid.UUID, p trainers.BranchListParams) (*trainers.BranchListResult, error) {
	return f.listByBranch(ctx, branchID, p)
}

func newTrainersTestRouter(svc trainerService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{trainersHandler: &trainersHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListBranchTrainers(t *testing.T) {
	cursor := "abc"
	branchID := uuid.New()
	svc := fakeTrainerService{listByBranch: func(_ context.Context, got uuid.UUID, p trainers.BranchListParams) (*trainers.BranchListResult, error) {
		if got != branchID {
			t.Fatalf("branchID = %v, want %v", got, branchID)
		}
		return &trainers.BranchListResult{
			Items:      []*trainers.Trainer{{ID: uuid.New(), Name: "Tess", Email: "tess@example.com"}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+branchID.String()+"/trainers?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.TrainerPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "Tess" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListBranchTrainersEmpty(t *testing.T) {
	svc := fakeTrainerService{listByBranch: func(context.Context, uuid.UUID, trainers.BranchListParams) (*trainers.BranchListResult, error) {
		return &trainers.BranchListResult{Items: []*trainers.Trainer{}}, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String()+"/trainers", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListBranchTrainersPassesParams(t *testing.T) {
	branchID := uuid.New()
	svc := fakeTrainerService{listByBranch: func(_ context.Context, got uuid.UUID, p trainers.BranchListParams) (*trainers.BranchListResult, error) {
		if got != branchID {
			t.Fatalf("branchID = %v, want %v", got, branchID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("BranchListParams = %+v", p)
		}
		return &trainers.BranchListResult{Items: []*trainers.Trainer{}}, nil
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+branchID.String()+"/trainers?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListBranchTrainersInvalid(t *testing.T) {
	svc := fakeTrainerService{listByBranch: func(context.Context, uuid.UUID, trainers.BranchListParams) (*trainers.BranchListResult, error) {
		return nil, trainers.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newTrainersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/branches/"+uuid.New().String()+"/trainers?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
