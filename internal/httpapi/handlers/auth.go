package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// authService is the slice of the auth domain the adapter consumes.
type authService interface {
	Login(ctx context.Context, email, password string) (*auth.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*auth.LoginResult, error)
	Logout(ctx context.Context, p auth.Principal) error
}

type authHandler struct {
	svc     authService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *authHandler) Login(c *gin.Context) {
	var req oapi.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "auth", "login", http.StatusBadRequest, "invalid request body", err)
		return
	}

	res, err := h.svc.Login(c.Request.Context(), string(req.Email), req.Password)
	if err != nil {
		h.fail(c, "login", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toLoginResponse(res))
}

func (h *authHandler) Refresh(c *gin.Context) {
	var req oapi.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "auth", "refresh", http.StatusBadRequest, "invalid request body", err)
		return
	}

	res, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.fail(c, "refresh", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toLoginResponse(res))
}

func (h *authHandler) Logout(c *gin.Context) {
	p, ok := middleware.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, h.log, h.metrics, "auth", "logout", http.StatusInternalServerError, "internal error", nil)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), p); err != nil {
		httpx.Error(c, h.log, h.metrics, "auth", "logout", http.StatusInternalServerError, "internal error", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// fail maps auth sentinel errors to HTTP statuses and responds consistently.
func (h *authHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		status = http.StatusUnauthorized
		msg = "invalid email or password"
	case errors.Is(err, auth.ErrInvalidToken):
		status = http.StatusUnauthorized
		msg = "invalid or expired token"
	}
	httpx.Error(c, h.log, h.metrics, "auth", op, status, msg, err)
}

func toLoginResponse(res *auth.LoginResult) oapi.LoginResponse {
	return oapi.LoginResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    res.TokenType,
		ExpiresIn:    int(res.ExpiresIn),
	}
}
