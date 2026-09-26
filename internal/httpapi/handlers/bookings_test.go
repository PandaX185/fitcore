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
	"github.com/PandaX185/fitcore/internal/modules/bookings"
)

type fakeBookingService struct {
	get         func(ctx context.Context, id uuid.UUID) (*bookings.Booking, error)
	create      func(ctx context.Context, classID, memberID uuid.UUID) (*bookings.Booking, error)
	cancel      func(ctx context.Context, id uuid.UUID) (*bookings.Booking, error)
	listByClass func(ctx context.Context, classID uuid.UUID, p bookings.ClassListParams) (*bookings.ClassListResult, error)
}

func (f fakeBookingService) Get(ctx context.Context, id uuid.UUID) (*bookings.Booking, error) {
	return f.get(ctx, id)
}
func (f fakeBookingService) Create(ctx context.Context, classID, memberID uuid.UUID) (*bookings.Booking, error) {
	return f.create(ctx, classID, memberID)
}
func (f fakeBookingService) Cancel(ctx context.Context, id uuid.UUID) (*bookings.Booking, error) {
	return f.cancel(ctx, id)
}
func (f fakeBookingService) ListByClass(ctx context.Context, classID uuid.UUID, p bookings.ClassListParams) (*bookings.ClassListResult, error) {
	return f.listByClass(ctx, classID, p)
}

func newBookingsTestRouter(svc bookingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{bookingsHandler: &bookingsHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListClassBookings(t *testing.T) {
	cursor := "abc"
	classID := uuid.New()
	svc := fakeBookingService{listByClass: func(_ context.Context, got uuid.UUID, p bookings.ClassListParams) (*bookings.ClassListResult, error) {
		if got != classID {
			t.Fatalf("classID = %v, want %v", got, classID)
		}
		return &bookings.ClassListResult{
			Items:      []*bookings.Booking{{ID: uuid.New()}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+classID.String()+"/bookings?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.BookingPage
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

func TestListClassBookingsEmpty(t *testing.T) {
	svc := fakeBookingService{listByClass: func(context.Context, uuid.UUID, bookings.ClassListParams) (*bookings.ClassListResult, error) {
		return &bookings.ClassListResult{Items: []*bookings.Booking{}}, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+uuid.New().String()+"/bookings", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListClassBookingsPassesParams(t *testing.T) {
	classID := uuid.New()
	svc := fakeBookingService{listByClass: func(_ context.Context, got uuid.UUID, p bookings.ClassListParams) (*bookings.ClassListResult, error) {
		if got != classID {
			t.Fatalf("classID = %v, want %v", got, classID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("ClassListParams = %+v", p)
		}
		return &bookings.ClassListResult{Items: []*bookings.Booking{}}, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+classID.String()+"/bookings?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListClassBookingsInvalid(t *testing.T) {
	svc := fakeBookingService{listByClass: func(context.Context, uuid.UUID, bookings.ClassListParams) (*bookings.ClassListResult, error) {
		return nil, bookings.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/classes/"+uuid.New().String()+"/bookings?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
