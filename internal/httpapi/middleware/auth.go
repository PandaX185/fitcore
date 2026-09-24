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
func (g *AuthGuard) Gin() gin.HandlerFunc {
	return func(c *gin.Context) {
		rule, ok := g.registry.For(c.Request.Method, c.FullPath())
		if !ok {
			// Unlisted routes (docs, unmatched paths) are not guarded; gin
			// resolves them as usual.
			c.Next()
			return
		}
		if rule.Public {
			c.Next()
			return
		}

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
		if len(rule.Permissions) > 0 && !auth.ContainsAny(p.Permissions, rule.Permissions) {
			abortAuth(c, g.log, http.StatusForbidden, nil)
			return
		}

		c.Set(contextKeyPrincipal, p)
		c.Next()
	}
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
	if log != nil && cause != nil {
		log.Warn("request rejected",
			"route", c.FullPath(),
			"status", status,
			"error", cause,
		)
	}
	code := "unauthorized"
	if status == http.StatusForbidden {
		code = "forbidden"
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"message": code, "code": code}})
}
