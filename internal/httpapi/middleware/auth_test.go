package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	"github.com/PandaX185/fitcore/internal/modules/auth"
)

type stubVerifier struct {
	principal auth.Principal
	err       error
}

func (s stubVerifier) Verify(raw string) (auth.Principal, error) {
	if s.err != nil {
		return auth.Principal{}, s.err
	}
	return s.principal, nil
}

type stubRevocations struct {
	revoked bool
	err     error
}

func (s stubRevocations) IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.revoked, nil
}

// guardEngine builds a gin engine guarded by AuthGuard with the default
// registry plus one route (/unlisted) that has no registry rule at all.
func guardEngine(verify stubVerifier, revoked stubRevocations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.NewAuthGuard(verify, revoked, middleware.DefaultRegistry(), nil).Gin())
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/branches", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/unlisted", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doRequest(r *gin.Engine, method, target, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var testPrincipal = auth.Principal{
	StaffID:     uuid.New(),
	JTI:         uuid.New(),
	Permissions: []auth.Permission{auth.PermBranchesRead},
}

// TestAuthGuardDefaultDeny pins the fail-closed contract: a route with no
// registry rule requires a valid token. Anonymous callers get 401 (even on
// paths gin would otherwise 404), while a valid token falls through to the
// handler (or gin's 404 for truly unmatched paths).
func TestAuthGuardDefaultDeny(t *testing.T) {
	valid := guardEngine(stubVerifier{principal: testPrincipal}, stubRevocations{})

	tests := []struct {
		name   string
		method string
		target string
		token  string
		want   int
	}{
		{name: "anonymous unlisted denied", method: "GET", target: "/unlisted", token: "", want: http.StatusUnauthorized},
		{name: "valid token unlisted allowed", method: "GET", target: "/unlisted", token: "good", want: http.StatusOK},
		{name: "anonymous unmatched denied", method: "GET", target: "/no-such-route", token: "", want: http.StatusUnauthorized},
		{name: "valid token unmatched falls through to 404", method: "GET", target: "/no-such-route", token: "good", want: http.StatusNotFound},
		{name: "public healthz stays open", method: "GET", target: "/healthz", token: "", want: http.StatusOK},
		{name: "protected route still needs permission", method: "GET", target: "/branches", token: "", want: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doRequest(valid, tt.method, tt.target, tt.token).Code; got != tt.want {
				t.Fatalf("%s %s token=%v: status = %d, want %d", tt.method, tt.target, tt.token != "", got, tt.want)
			}
		})
	}
}

// TestAuthGuardUnlistedWithoutPermission checks that an authenticated caller
// whose token carries no grants still passes an unlisted route (there is no
// rule to enforce), while a registry-protected route yields 403.
func TestAuthGuardUnlistedWithoutPermission(t *testing.T) {
	bare := auth.Principal{StaffID: uuid.New(), JTI: uuid.New()}
	r := guardEngine(stubVerifier{principal: bare}, stubRevocations{})

	if got := doRequest(r, "GET", "/unlisted", "good").Code; got != http.StatusOK {
		t.Fatalf("unlisted with permissionless token = %d, want 200", got)
	}
	if got := doRequest(r, "GET", "/branches", "good").Code; got != http.StatusForbidden {
		t.Fatalf("protected with permissionless token = %d, want 403", got)
	}
}

// TestAuthGuardBadTokenStays401 pins the 401 semantics for bad tokens on
// unlisted routes: an invalid token is unauthorized, never forbidden.
func TestAuthGuardBadTokenStays401(t *testing.T) {
	r := guardEngine(stubVerifier{err: auth.ErrInvalidToken}, stubRevocations{})

	for _, target := range []string{"/unlisted", "/branches", "/no-such-route"} {
		if got := doRequest(r, "GET", target, "bad").Code; got != http.StatusUnauthorized {
			t.Fatalf("GET %s with bad token = %d, want 401", target, got)
		}
	}
}
