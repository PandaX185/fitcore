package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/billing"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// invoiceService is the slice of the billing service the adapter consumes.
type invoiceService interface {
	Get(ctx context.Context, id uuid.UUID) (*billing.Invoice, error)
	Create(ctx context.Context, memberID, membershipID uuid.UUID, amountCents int64, currency string, dueAt time.Time) (*billing.Invoice, error)
	Update(ctx context.Context, id uuid.UUID, patch billing.Patch) (*billing.Invoice, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*billing.Invoice, error)
}

type billingHandler struct {
	svc     invoiceService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *billingHandler) CreateInvoice(c *gin.Context) {
	var req oapi.InvoiceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "billing", "createInvoice", http.StatusBadRequest, "invalid request body", err)
		return
	}
	inv, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.MemberId), uuid.UUID(req.MembershipId), req.AmountCents, req.Currency, req.DueAt)
	if err != nil {
		h.fail(c, "createInvoice", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toInvoiceResponse(inv))
}

func (h *billingHandler) GetInvoice(c *gin.Context, invoiceId oapi.InvoiceID) {
	inv, err := h.svc.Get(c.Request.Context(), uuid.UUID(invoiceId))
	if err != nil {
		h.fail(c, "getInvoice", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toInvoiceResponse(inv))
}

func (h *billingHandler) UpdateInvoice(c *gin.Context, invoiceId oapi.InvoiceID) {
	var req oapi.InvoiceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "billing", "updateInvoice", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := billing.Patch{
		Status: (*billing.InvoiceStatus)(req.Status),
		DueAt:  req.DueAt,
	}
	inv, err := h.svc.Update(c.Request.Context(), uuid.UUID(invoiceId), patch)
	if err != nil {
		h.fail(c, "updateInvoice", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toInvoiceResponse(inv))
}

func (h *billingHandler) ListMemberInvoices(c *gin.Context, memberId oapi.MemberID) {
	invs, err := h.svc.ListByMember(c.Request.Context(), uuid.UUID(memberId))
	if err != nil {
		h.fail(c, "listMemberInvoices", err)
		return
	}
	items := make([]oapi.Invoice, 0, len(invs))
	for _, inv := range invs {
		items = append(items, toInvoiceResponse(inv))
	}
	httpx.JSON(c, http.StatusOK, items)
}

// fail maps billing sentinel errors to HTTP statuses.
func (h *billingHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, billing.ErrNotFound):
		status = http.StatusNotFound
		msg = "invoice not found"
	case errors.Is(err, billing.ErrMemberNotFound), errors.Is(err, billing.ErrMembershipNotFound):
		status = http.StatusNotFound
		msg = "member or membership not found"
	case errors.Is(err, billing.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid invoice"
	}
	httpx.Error(c, h.log, h.metrics, "billing", op, status, msg, err)
}

func toInvoiceResponse(inv *billing.Invoice) oapi.Invoice {
	var paidAt *oapi.Timestamp
	if inv.PaidAt != nil {
		pt := oapi.Timestamp(*inv.PaidAt)
		paidAt = &pt
	}
	return oapi.Invoice{
		Id:           oapi.UUID(inv.ID),
		MemberId:     oapi.UUID(inv.MemberID),
		MembershipId: oapi.UUID(inv.MembershipID),
		AmountCents:  inv.AmountCents,
		Currency:     inv.Currency,
		Status:       oapi.InvoiceStatus(inv.Status),
		DueAt:        inv.DueAt,
		PaidAt:       paidAt,
		CreatedAt:    inv.CreatedAt,
		UpdatedAt:    inv.UpdatedAt,
	}
}
