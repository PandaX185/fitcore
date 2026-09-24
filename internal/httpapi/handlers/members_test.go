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
	"github.com/PandaX185/fitcore/internal/modules/members"
)

type fakeMemberService struct {
	get    func(ctx context.Context, id uuid.UUID) (*members.Member, error)
	create func(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*members.Member, error)
	update func(ctx context.Context, id uuid.UUID, patch members.Patch) (*members.Member, error)
	delete func(ctx context.Context, id uuid.UUID) error
	list   func(ctx context.Context) ([]*members.Member, error)
}

func (f fakeMemberService) Get(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	return f.get(ctx, id)
}
func (f fakeMemberService) Create(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*members.Member, error) {
	return f.create(ctx, branchID, name, email, phone)
}
func (f fakeMemberService) Update(ctx context.Context, id uuid.UUID, patch members.Patch) (*members.Member, error) {
	return f.update(ctx, id, patch)
}
func (f fakeMemberService) Delete(ctx context.Context, id uuid.UUID) error {
	return f.delete(ctx, id)
}
func (f fakeMemberService) List(ctx context.Context) ([]*members.Member, error) {
	return f.list(ctx)
}

func newMembersTestRouter(svc memberService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{membersHandler: &membersHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestGetMember(t *testing.T) {
	id := uuid.New()
	branchID := uuid.New()
	svc := fakeMemberService{get: func(ctx context.Context, got uuid.UUID) (*members.Member, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &members.Member{
			ID: id, BranchID: branchID, Name: "Ada", Email: "ada@example.com",
			Phone: "123", Status: members.StatusActive,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Member
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Name != "Ada" || got.Email != "ada@example.com" {
		t.Fatalf("body = %+v", got)
	}
}

func TestGetMemberNotFound(t *testing.T) {
	svc := fakeMemberService{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "member not found")
}

func TestGetMemberInvalidID(t *testing.T) {
	svc := fakeMemberService{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/00000000-0000-0000-0000-000000000000", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestGetMemberMalformedID(t *testing.T) {
	svc := fakeMemberService{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateMember(t *testing.T) {
	id := uuid.New()
	branchID := uuid.New()
	svc := fakeMemberService{create: func(ctx context.Context, gotBranch uuid.UUID, name, email, phone string) (*members.Member, error) {
		if gotBranch != branchID || name != "Ada" || email != "ada@example.com" {
			t.Fatalf("Create(%v, %q, %q, %q)", gotBranch, name, email, phone)
		}
		return &members.Member{ID: id, BranchID: gotBranch, Name: name, Email: email, Status: members.StatusActive}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + branchID.String() + `","name":"Ada","email":"ada@example.com"}`
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateMemberBadBody(t *testing.T) {
	svc := fakeMemberService{create: func(context.Context, uuid.UUID, string, string, string) (*members.Member, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(`{not json`)))

	assertErrorStatus(t, rec, http.StatusBadRequest, "invalid request body")
}

func TestCreateMemberInvalid(t *testing.T) {
	svc := fakeMemberService{create: func(context.Context, uuid.UUID, string, string, string) (*members.Member, error) {
		return nil, members.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"00000000-0000-0000-0000-000000000000","name":"","email":"a@b.com"}`
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusBadRequest, "invalid member")
}

func TestCreateMemberDuplicate(t *testing.T) {
	svc := fakeMemberService{create: func(context.Context, uuid.UUID, string, string, string) (*members.Member, error) {
		return nil, members.ErrDuplicateEmail
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + uuid.New().String() + `","name":"Ada","email":"dup@example.com"}`
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusConflict, "email already in use")
}

func TestCreateMemberInternalError(t *testing.T) {
	svc := fakeMemberService{create: func(context.Context, uuid.UUID, string, string, string) (*members.Member, error) {
		return nil, errors.New("boom")
	}}
	rec := httptest.NewRecorder()
	body := `{"branchId":"` + uuid.New().String() + `","name":"Ada","email":"ada@example.com"}`
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusInternalServerError, "internal error")
}

func TestUpdateMember(t *testing.T) {
	id := uuid.New()
	svc := fakeMemberService{update: func(ctx context.Context, got uuid.UUID, patch members.Patch) (*members.Member, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Name == nil || *patch.Name != "Renamed" {
			t.Fatalf("patch = %+v", patch)
		}
		return &members.Member{ID: id, Name: *patch.Name, Status: members.StatusActive}, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/members/"+id.String(), strings.NewReader(`{"name":"Renamed"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMemberNotFound(t *testing.T) {
	svc := fakeMemberService{update: func(context.Context, uuid.UUID, members.Patch) (*members.Member, error) {
		return nil, members.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/members/"+uuid.New().String(), strings.NewReader(`{"name":"x"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "member not found")
}

func TestUpdateMemberDuplicate(t *testing.T) {
	svc := fakeMemberService{update: func(context.Context, uuid.UUID, members.Patch) (*members.Member, error) {
		return nil, members.ErrDuplicateEmail
	}}
	rec := httptest.NewRecorder()
	id := uuid.New()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/members/"+id.String(), strings.NewReader(`{"email":"dup@example.com"}`)))

	assertErrorStatus(t, rec, http.StatusConflict, "email already in use")
}

func TestUpdateMemberBadBody(t *testing.T) {
	svc := fakeMemberService{update: func(context.Context, uuid.UUID, members.Patch) (*members.Member, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/members/"+uuid.New().String(), strings.NewReader(`{"name": 42}`)))

	assertErrorStatus(t, rec, http.StatusBadRequest, "invalid request body")
}

func TestDeleteMember(t *testing.T) {
	id := uuid.New()
	svc := fakeMemberService{delete: func(ctx context.Context, got uuid.UUID) error {
		if got != id {
			t.Fatalf("Delete id = %v, want %v", got, id)
		}
		return nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/members/"+id.String(), nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteMemberNotFound(t *testing.T) {
	svc := fakeMemberService{delete: func(context.Context, uuid.UUID) error {
		return members.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/members/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "member not found")
}

func TestListMembers(t *testing.T) {
	svc := fakeMemberService{list: func(context.Context) ([]*members.Member, error) {
		return []*members.Member{{ID: uuid.New(), Name: "Ada", Email: "ada@example.com", Status: members.StatusActive}}, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got []oapi.Member
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Ada" {
		t.Fatalf("items = %+v", got)
	}
}

func TestListMembersEmpty(t *testing.T) {
	svc := fakeMemberService{list: func(context.Context) ([]*members.Member, error) {
		return []*members.Member{}, nil
	}}
	rec := httptest.NewRecorder()
	newMembersTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `[]` {
		t.Fatalf("body = %s, want %q", body, `[]`)
	}
}

func assertErrorStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, msg string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, want, rec.Body.String())
	}
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message != msg {
		t.Fatalf("message = %q, want %q", body.Error.Message, msg)
	}
}
