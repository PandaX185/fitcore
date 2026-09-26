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
	"github.com/PandaX185/fitcore/internal/modules/billing"
)

type fakeInvoiceService struct {
	get          func(ctx context.Context, id uuid.UUID) (*billing.Invoice, error)
	create       func(ctx context.Context, memberID, membershipID uuid.UUID, amountCents int64, currency string, dueAt time.Time) (*billing.Invoice, error)
	update       func(ctx context.Context, id uuid.UUID, patch billing.Patch) (*billing.Invoice, error)
	listByMember func(ctx context.Context, memberID uuid.UUID, p billing.MemberListParams) (*billing.MemberListResult, error)
}

func (f fakeInvoiceService) Get(ctx context.Context, id uuid.UUID) (*billing.Invoice, error) {
	return f.get(ctx, id)
}
func (f fakeInvoiceService) Create(ctx context.Context, memberID, membershipID uuid.UUID, amountCents int64, currency string, dueAt time.Time) (*billing.Invoice, error) {
	return f.create(ctx, memberID, membershipID, amountCents, currency, dueAt)
}
func (f fakeInvoiceService) Update(ctx context.Context, id uuid.UUID, patch billing.Patch) (*billing.Invoice, error) {
	return f.update(ctx, id, patch)
}
func (f fakeInvoiceService) ListByMember(ctx context.Context, memberID uuid.UUID, p billing.MemberListParams) (*billing.MemberListResult, error) {
	return f.listByMember(ctx, memberID, p)
}

func newBillingTestRouter(svc invoiceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{billingHandler: &billingHandler{svc: svc}}
	r := gin.New()
	oapi.RegisterHandlers(r, h)
	return r
}

func TestListMemberInvoices(t *testing.T) {
	cursor := "abc"
	memberID := uuid.New()
	svc := fakeInvoiceService{listByMember: func(_ context.Context, got uuid.UUID, p billing.MemberListParams) (*billing.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		return &billing.MemberListResult{
			Items:      []*billing.Invoice{{ID: uuid.New()}},
			NextCursor: cursor,
		}, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/invoices?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.InvoicePage
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

func TestListMemberInvoicesEmpty(t *testing.T) {
	svc := fakeInvoiceService{listByMember: func(context.Context, uuid.UUID, billing.MemberListParams) (*billing.MemberListResult, error) {
		return &billing.MemberListResult{Items: []*billing.Invoice{}}, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/invoices", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want %q", body, `{"items":[]}`)
	}
}

func TestListMemberInvoicesPassesParams(t *testing.T) {
	memberID := uuid.New()
	svc := fakeInvoiceService{listByMember: func(_ context.Context, got uuid.UUID, p billing.MemberListParams) (*billing.MemberListResult, error) {
		if got != memberID {
			t.Fatalf("memberID = %v, want %v", got, memberID)
		}
		if p.Limit != 5 || p.Cursor != "cur" {
			t.Fatalf("MemberListParams = %+v", p)
		}
		return &billing.MemberListResult{Items: []*billing.Invoice{}}, nil
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+memberID.String()+"/invoices?limit=5&cursor=cur", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
}

func TestListMemberInvoicesInvalid(t *testing.T) {
	svc := fakeInvoiceService{listByMember: func(context.Context, uuid.UUID, billing.MemberListParams) (*billing.MemberListResult, error) {
		return nil, billing.ErrInvalidInput
	}}
	rec := httptest.NewRecorder()
	newBillingTestRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/members/"+uuid.New().String()+"/invoices?cursor=bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}
