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
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

type fakeMembershipService struct {
	get          func(ctx context.Context, id uuid.UUID) (*memberships.Membership, error)
	create       func(ctx context.Context, memberID, packageID, branchID uuid.UUID) (*memberships.Membership, error)
	update       func(ctx context.Context, id uuid.UUID, patch memberships.Patch) (*memberships.Membership, error)
	listByMember func(ctx context.Context, memberID uuid.UUID, p memberships.MemberListParams) (*memberships.MemberListResult, error)
}

func (f fakeMembershipService) Get(ctx context.Context, id uuid.UUID) (*memberships.Membership, error) {
	return f.get(ctx, id)
}
func (f fakeMembershipService) Create(ctx context.Context, memberID, packageID, branchID uuid.UUID) (*memberships.Membership, error) {
	return f.create(ctx, memberID, packageID, branchID)
}
func (f fakeMembershipService) Update(ctx context.Context, id uuid.UUID, patch memberships.Patch) (*memberships.Membership, error) {
	return f.update(ctx, id, patch)
}
func (f fakeMembershipService) ListByMember(ctx context.Context, memberID uuid.UUID, p memberships.MemberListParams) (*memberships.MemberListResult, error) {
	return f.listByMember(ctx, memberID, p)
}

func newMembershipsTestRouter(svc membershipService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{membershipsHandler: &membershipsHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListMemberMemberships(t *testing.T) {
	cursor := "abc"
	memberID := uuid.New()
	svc := fakeMembershipService{listByMember: func(_ context.Context, got uuid.UUID, p memberships.MemberListParams) (*memberships.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		return &memberships.MemberListResult{
			Items:      []*memberships.Membership{{ID: uuid.New()}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/memberships?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.MembershipPage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.NextCursor == nil || *got.NextCursor != cursor {
		t.Fatalf("nextCursor = %v, want %q", got.NextCursor, cursor)
	}
}

func TestListMemberMembershipsEmpty(t *testing.T) {
	svc := fakeMembershipService{listByMember: func(context.Context, uuid.UUID, memberships.MemberListParams) (*memberships.MemberListResult, error) {
		return &memberships.MemberListResult{Items: []*memberships.Membership{}}, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/memberships", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListMemberMembershipsPassesParams(t *testing.T) {
	memberID := uuid.New()
	svc := fakeMembershipService{listByMember: func(_ context.Context, got uuid.UUID, p memberships.MemberListParams) (*memberships.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("MemberListParams = %+v", p)
		}
		return &memberships.MemberListResult{Items: []*memberships.Membership{}}, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/memberships?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListMemberMembershipsInvalid(t *testing.T) {
	svc := fakeMembershipService{listByMember: func(context.Context, uuid.UUID, memberships.MemberListParams) (*memberships.MemberListResult, error) {
		return nil, memberships.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/memberships?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
