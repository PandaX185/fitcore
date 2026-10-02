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
	"github.com/PandaX185/fitcore/internal/modules/bookings"
)

func TestOpsCreateBookingHappy(t *testing.T) {
	id := uuid.New()
	classID := uuid.New()
	memberID := uuid.New()
	svc := fakeBookingService{create: func(_ context.Context, gotClass, gotMember uuid.UUID) (*bookings.Booking, error) {
		if gotClass != classID || gotMember != memberID {
			t.Fatalf("Create(%v, %v), want (%v, %v)", gotClass, gotMember, classID, memberID)
		}
		return &bookings.Booking{ID: id, ClassID: gotClass, MemberID: gotMember, Status: bookings.StatusBooked}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"classId":"` + classID.String() + `","memberId":"` + memberID.String() + `"}`
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Booking
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.ClassId != oapi.UUID(classID) || got.MemberId != oapi.UUID(memberID) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetBookingHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeBookingService{get: func(_ context.Context, got uuid.UUID) (*bookings.Booking, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &bookings.Booking{ID: id, ClassID: uuid.New(), MemberID: uuid.New(), Status: bookings.StatusBooked}, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bookings/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Booking
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetBookingNotFound(t *testing.T) {
	svc := fakeBookingService{get: func(context.Context, uuid.UUID) (*bookings.Booking, error) {
		return nil, bookings.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bookings/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "booking not found")
}

func TestOpsGetBookingInvalidID(t *testing.T) {
	svc := fakeBookingService{get: func(context.Context, uuid.UUID) (*bookings.Booking, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bookings/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsCancelBookingHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeBookingService{cancel: func(_ context.Context, got uuid.UUID) (*bookings.Booking, error) {
		if got != id {
			t.Fatalf("Cancel id = %v, want %v", got, id)
		}
		return &bookings.Booking{ID: id, ClassID: uuid.New(), MemberID: uuid.New(), Status: bookings.StatusCancelled}, nil
	}}
	rec := httptest.NewRecorder()
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/bookings/"+id.String()+"/cancel", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Booking
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.Status != oapi.BookingStatus(bookings.StatusCancelled) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsBookingsFailDuplicate(t *testing.T) {
	svc := fakeBookingService{create: func(context.Context, uuid.UUID, uuid.UUID) (*bookings.Booking, error) {
		return nil, bookings.ErrDuplicate
	}}
	rec := httptest.NewRecorder()
	body := `{"classId":"` + uuid.New().String() + `","memberId":"` + uuid.New().String() + `"}`
	newBookingsTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusConflict, "booking conflict")
}
