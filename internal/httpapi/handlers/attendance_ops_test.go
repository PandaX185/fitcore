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
	"github.com/PandaX185/fitcore/internal/modules/attendance"
)

func TestOpsCheckInHappy(t *testing.T) {
	id := uuid.New()
	memberID := uuid.New()
	branchID := uuid.New()
	svc := fakeAttendanceService{checkIn: func(_ context.Context, gotMember, gotBranch uuid.UUID) (*attendance.Attendance, error) {
		if gotMember != memberID || gotBranch != branchID {
			t.Fatalf("CheckIn(%v, %v), want (%v, %v)", gotMember, gotBranch, memberID, branchID)
		}
		return &attendance.Attendance{ID: id, MemberID: gotMember, BranchID: gotBranch}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + memberID.String() + `","branchId":"` + branchID.String() + `"}`
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attendance/check-in", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Attendance
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.MemberId != oapi.UUID(memberID) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsCheckOutHappy(t *testing.T) {
	id := uuid.New()
	memberID := uuid.New()
	svc := fakeAttendanceService{checkOut: func(_ context.Context, gotMember uuid.UUID) (*attendance.Attendance, error) {
		if gotMember != memberID {
			t.Fatalf("CheckOut(%v), want %v", gotMember, memberID)
		}
		return &attendance.Attendance{ID: id, MemberID: gotMember, BranchID: uuid.New()}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + memberID.String() + `"}`
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attendance/check-out", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Attendance
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetAttendanceHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeAttendanceService{get: func(_ context.Context, got uuid.UUID) (*attendance.Attendance, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &attendance.Attendance{ID: id, MemberID: uuid.New(), BranchID: uuid.New()}, nil
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attendance/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Attendance
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetAttendanceNotFound(t *testing.T) {
	svc := fakeAttendanceService{get: func(context.Context, uuid.UUID) (*attendance.Attendance, error) {
		return nil, attendance.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attendance/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "attendance record not found")
}

func TestOpsGetAttendanceInvalidID(t *testing.T) {
	svc := fakeAttendanceService{get: func(context.Context, uuid.UUID) (*attendance.Attendance, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attendance/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsAttendanceFailAlreadyCheckedIn(t *testing.T) {
	svc := fakeAttendanceService{checkIn: func(context.Context, uuid.UUID, uuid.UUID) (*attendance.Attendance, error) {
		return nil, attendance.ErrAlreadyCheckedIn
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + uuid.New().String() + `","branchId":"` + uuid.New().String() + `"}`
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attendance/check-in", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusConflict, "attendance conflict")
}
