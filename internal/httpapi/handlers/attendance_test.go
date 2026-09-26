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
	"github.com/PandaX185/fitcore/internal/modules/attendance"
)

type fakeAttendanceService struct {
	get          func(ctx context.Context, id uuid.UUID) (*attendance.Attendance, error)
	checkIn      func(ctx context.Context, memberID, branchID uuid.UUID) (*attendance.Attendance, error)
	checkOut     func(ctx context.Context, memberID uuid.UUID) (*attendance.Attendance, error)
	listByMember func(ctx context.Context, memberID uuid.UUID, p attendance.MemberListParams) (*attendance.MemberListResult, error)
}

func (f fakeAttendanceService) Get(ctx context.Context, id uuid.UUID) (*attendance.Attendance, error) {
	return f.get(ctx, id)
}
func (f fakeAttendanceService) CheckIn(ctx context.Context, memberID, branchID uuid.UUID) (*attendance.Attendance, error) {
	return f.checkIn(ctx, memberID, branchID)
}
func (f fakeAttendanceService) CheckOut(ctx context.Context, memberID uuid.UUID) (*attendance.Attendance, error) {
	return f.checkOut(ctx, memberID)
}
func (f fakeAttendanceService) ListByMember(ctx context.Context, memberID uuid.UUID, p attendance.MemberListParams) (*attendance.MemberListResult, error) {
	return f.listByMember(ctx, memberID, p)
}

func newAttendanceTestRouter(svc attendanceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{attendanceHandler: &attendanceHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListMemberAttendance(t *testing.T) {
	cursor := "abc"
	memberID := uuid.New()
	svc := fakeAttendanceService{listByMember: func(_ context.Context, got uuid.UUID, p attendance.MemberListParams) (*attendance.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		return &attendance.MemberListResult{
			Items:      []*attendance.Attendance{{ID: uuid.New()}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/attendance?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.AttendancePage
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

func TestListMemberAttendanceEmpty(t *testing.T) {
	svc := fakeAttendanceService{listByMember: func(context.Context, uuid.UUID, attendance.MemberListParams) (*attendance.MemberListResult, error) {
		return &attendance.MemberListResult{Items: []*attendance.Attendance{}}, nil
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/attendance", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListMemberAttendancePassesParams(t *testing.T) {
	memberID := uuid.New()
	svc := fakeAttendanceService{listByMember: func(_ context.Context, got uuid.UUID, p attendance.MemberListParams) (*attendance.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("MemberListParams = %+v", p)
		}
		return &attendance.MemberListResult{Items: []*attendance.Attendance{}}, nil
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/attendance?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListMemberAttendanceInvalid(t *testing.T) {
	svc := fakeAttendanceService{listByMember: func(context.Context, uuid.UUID, attendance.MemberListParams) (*attendance.MemberListResult, error) {
		return nil, attendance.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newAttendanceTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/attendance?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
