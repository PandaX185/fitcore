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
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

func TestOpsCreateMembershipHappy(t *testing.T) {
	id := uuid.New()
	memberID := uuid.New()
	packageID := uuid.New()
	branchID := uuid.New()
	svc := fakeMembershipService{create: func(_ context.Context, gotMember, gotPackage, gotBranch uuid.UUID) (*memberships.Membership, error) {
		if gotMember != memberID || gotPackage != packageID || gotBranch != branchID {
			t.Fatalf("Create(%v, %v, %v)", gotMember, gotPackage, gotBranch)
		}
		return &memberships.Membership{ID: id, MemberID: gotMember, PackageID: gotPackage, BranchID: gotBranch, Status: memberships.StatusActive}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + memberID.String() + `","packageId":"` + packageID.String() + `","branchId":"` + branchID.String() + `"}`
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/memberships", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Membership
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.MemberId != oapi.UUID(memberID) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetMembershipHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeMembershipService{get: func(_ context.Context, got uuid.UUID) (*memberships.Membership, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &memberships.Membership{ID: id, MemberID: uuid.New(), PackageID: uuid.New(), BranchID: uuid.New(), Status: memberships.StatusActive}, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/memberships/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Membership
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetMembershipNotFound(t *testing.T) {
	svc := fakeMembershipService{get: func(context.Context, uuid.UUID) (*memberships.Membership, error) {
		return nil, memberships.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/memberships/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "membership not found")
}

func TestOpsGetMembershipInvalidID(t *testing.T) {
	svc := fakeMembershipService{get: func(context.Context, uuid.UUID) (*memberships.Membership, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/memberships/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdateMembershipHappy(t *testing.T) {
	id := uuid.New()
	frozen := memberships.StatusFrozen
	svc := fakeMembershipService{update: func(_ context.Context, got uuid.UUID, patch memberships.Patch) (*memberships.Membership, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Status == nil || *patch.Status != memberships.StatusFrozen {
			t.Fatalf("patch = %+v", patch)
		}
		return &memberships.Membership{ID: id, Status: frozen}, nil
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/memberships/"+id.String(), strings.NewReader(`{"status":"frozen"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Membership
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Status != oapi.MembershipStatus(memberships.StatusFrozen) {
		t.Fatalf("status = %q, want frozen", got.Status)
	}
}

func TestOpsUpdateMembershipNotFound(t *testing.T) {
	svc := fakeMembershipService{update: func(context.Context, uuid.UUID, memberships.Patch) (*memberships.Membership, error) {
		return nil, memberships.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/memberships/"+uuid.New().String(), strings.NewReader(`{"status":"frozen"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "membership not found")
}

func TestOpsMembershipsFailDuplicateActive(t *testing.T) {
	svc := fakeMembershipService{create: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*memberships.Membership, error) {
		return nil, memberships.ErrDuplicateActive
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + uuid.New().String() + `","packageId":"` + uuid.New().String() + `","branchId":"` + uuid.New().String() + `"}`
	newMembershipsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/memberships", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusConflict, "membership conflict")
}
