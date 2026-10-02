package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
)

const contextKeyPrincipal = "auth.principal"

// PrincipalFrom retrieves the authenticated principal set by AuthGuard on
// protected routes.
func PrincipalFrom(c *gin.Context) (auth.Principal, bool) {
	v, ok := c.Get(contextKeyPrincipal)
	if !ok {
		return auth.Principal{}, false
	}
	p, ok := v.(auth.Principal)
	return p, ok
}

// authVerifier parses and validates access tokens.
type authVerifier interface {
	Verify(raw string) (auth.Principal, error)
}

// revocationChecker reports whether a token id has been revoked.
type revocationChecker interface {
	IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error)
}

// AuthGuard enforces per-route authentication and authorization.
type AuthGuard struct {
	verify   authVerifier
	revoked  revocationChecker
	registry Registry
	log      *slog.Logger
}

func NewAuthGuard(verify authVerifier, revoked revocationChecker, registry Registry, log *slog.Logger) *AuthGuard {
	if log == nil {
		log = slog.Default()
	}
	return &AuthGuard{verify: verify, revoked: revoked, registry: registry, log: log}
}

// Gin returns the middleware. Public routes pass through; protected routes
// require a valid, unrevoked access token whose permission set satisfies the
// route's rule. Missing/expired/revoked tokens yield 401; a valid token
// without the required permission yields 403.
//
// The guard is fail-closed: a route with no registry rule is NOT waved
// through. Only the explicit publicFallback allowlist (health/readiness,
// OpenAPI spec, Swagger UI) passes without a token; every other unlisted
// route requires a valid token, so anonymous callers get 401 on unmatched
// paths while authenticated callers fall through to gin's 404.
func (g *AuthGuard) Gin() gin.HandlerFunc {
	return func(c *gin.Context) {
		rule, ok := g.registry.For(c.Request.Method, c.FullPath())
		if !ok {
			if publicFallback(c) {
				c.Next()
				return
			}
			g.requireAuth(c, nil)
			return
		}
		if rule.Public {
			c.Next()
			return
		}
		g.requireAuth(c, rule.Permissions)
	}
}

// requireAuth rejects anonymous callers with 401 and forbidden callers with
// 403; valid callers carrying the required permissions proceed.
func (g *AuthGuard) requireAuth(c *gin.Context, perms []auth.Permission) {
	raw, err := bearerToken(c)
	if err != nil {
		abortAuth(c, g.log, http.StatusUnauthorized, err)
		return
	}
	p, err := g.verify.Verify(raw)
	if err != nil {
		abortAuth(c, g.log, http.StatusUnauthorized, err)
		return
	}
	revoked, rerr := g.revoked.IsRevoked(c.Request.Context(), p.JTI)
	if rerr != nil || revoked {
		// Fail closed: a revocation-store error is treated as revoked.
		abortAuth(c, g.log, http.StatusUnauthorized, rerr)
		return
	}
	if len(perms) > 0 && !auth.ContainsAny(p.Permissions, perms) {
		abortAuth(c, g.log, http.StatusForbidden, nil)
		return
	}

	c.Set(contextKeyPrincipal, p)
	c.Next()
}

// publicFallback reports whether an unlisted route is part of the explicit
// public allowlist: liveness/readiness, the raw OpenAPI spec and the Swagger
// UI. /metrics stays public via its registry rule, not this fallback. The
// gin route path is empty for unmatched requests, so match on the request
// URL path instead.
func publicFallback(c *gin.Context) bool {
	path := c.FullPath()
	if path == "" && c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	switch path {
	case "/healthz", "/readyz", "/openapi.yaml", "/swagger":
		return true
	}
	return strings.HasPrefix(path, "/swagger/")
}

func bearerToken(c *gin.Context) (string, error) {
	h := c.GetHeader("Authorization")
	if h == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("malformed authorization header")
	}
	return parts[1], nil
}

func abortAuth(c *gin.Context, log *slog.Logger, status int, cause error) {
	code := "unauthorized"
	if status == http.StatusForbidden {
		code = "forbidden"
	}
	if log != nil {
		attrs := []any{
			"route", c.FullPath(),
			"status", status,
		}
		if rid, ok := c.Get(RequestIDKey); ok {
			attrs = append(attrs, "request_id", rid)
		}
		// Log unconditionally: 403 rejections carry no cause by design, and
		// a silent deny is invisible in audits.
		if cause != nil {
			attrs = append(attrs, "error", cause)
		}
		log.Warn("request rejected", attrs...)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": code, "code": code})
}
