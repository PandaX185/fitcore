package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/billing"
)

func TestOpsCreateInvoiceHappy(t *testing.T) {
	id := uuid.New()
	memberID := uuid.New()
	membershipID := uuid.New()
	svc := fakeInvoiceService{create: func(_ context.Context, gotMember, gotMembership uuid.UUID, amount int64, currency string, _ time.Time) (*billing.Invoice, error) {
		if gotMember != memberID || gotMembership != membershipID || amount != 1000 || currency != "BHD" {
			t.Fatalf("Create(%v, %v, %d, %q)", gotMember, gotMembership, amount, currency)
		}
		return &billing.Invoice{ID: id, MemberID: gotMember, MembershipID: gotMembership, AmountCents: amount, Currency: currency, Status: billing.StatusPending}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + memberID.String() + `","membershipId":"` + membershipID.String() + `","amountCents":1000,"currency":"BHD","dueAt":"2030-02-01T00:00:00Z"}`
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) || got.AmountCents != 1000 || got.Currency != "BHD" {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetInvoiceHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeInvoiceService{get: func(_ context.Context, got uuid.UUID) (*billing.Invoice, error) {
		if got != id {
			t.Fatalf("Get id = %v, want %v", got, id)
		}
		return &billing.Invoice{ID: id, MemberID: uuid.New(), MembershipID: uuid.New(), AmountCents: 1000, Currency: "BHD", Status: billing.StatusPending}, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoices/"+id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Id != oapi.UUID(id) {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsGetInvoiceNotFound(t *testing.T) {
	svc := fakeInvoiceService{get: func(context.Context, uuid.UUID) (*billing.Invoice, error) {
		return nil, billing.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoices/"+uuid.New().String(), nil))

	assertErrorStatus(t, rec, http.StatusNotFound, "invoice not found")
}

func TestOpsGetInvoiceInvalidID(t *testing.T) {
	svc := fakeInvoiceService{get: func(context.Context, uuid.UUID) (*billing.Invoice, error) {
		t.Fatal("service must not be called for a malformed id")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoices/not-a-uuid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsUpdateInvoiceHappy(t *testing.T) {
	id := uuid.New()
	svc := fakeInvoiceService{update: func(_ context.Context, got uuid.UUID, patch billing.Patch) (*billing.Invoice, error) {
		if got != id {
			t.Fatalf("Update id = %v, want %v", got, id)
		}
		if patch.Status == nil || *patch.Status != billing.StatusPaid {
			t.Fatalf("patch = %+v", patch)
		}
		return &billing.Invoice{ID: id, Status: *patch.Status}, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/invoices/"+id.String(), strings.NewReader(`{"status":"paid"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Status != oapi.InvoiceStatus(billing.StatusPaid) {
		t.Fatalf("status = %q, want paid", got.Status)
	}
}

func TestOpsUpdateInvoiceNotFound(t *testing.T) {
	svc := fakeInvoiceService{update: func(context.Context, uuid.UUID, billing.Patch) (*billing.Invoice, error) {
		return nil, billing.ErrNotFound
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/invoices/"+uuid.New().String(), strings.NewReader(`{"status":"paid"}`)))

	assertErrorStatus(t, rec, http.StatusNotFound, "invoice not found")
}

func TestOpsBillingFailMembershipNotFound(t *testing.T) {
	svc := fakeInvoiceService{create: func(context.Context, uuid.UUID, uuid.UUID, int64, string, time.Time) (*billing.Invoice, error) {
		return nil, billing.ErrMembershipNotFound
	}}
	rec := httptest.NewRecorder()
	body := `{"memberId":"` + uuid.New().String() + `","membershipId":"` + uuid.New().String() + `","amountCents":1000,"currency":"BHD","dueAt":"2030-02-01T00:00:00Z"}`
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusNotFound, "member or membership not found")
}
