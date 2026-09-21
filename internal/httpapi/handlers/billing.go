package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type billing struct{}

func (*billing) CreateInvoice(c *gin.Context) { notImplemented(c) }
func (*billing) GetInvoice(c *gin.Context, invoiceId oapi.InvoiceID) {
	notImplemented(c)
}
func (*billing) UpdateInvoice(c *gin.Context, invoiceId oapi.InvoiceID) {
	notImplemented(c)
}
func (*billing) ListMemberInvoices(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
