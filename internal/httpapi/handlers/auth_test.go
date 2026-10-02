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
	"github.com/PandaX185/fitcore/internal/modules/auth"
)

type fakeAuthService struct {
	login   func(ctx context.Context, email, password string) (*auth.LoginResult, error)
	refresh func(ctx context.Context, token string) (*auth.LoginResult, error)
	logout  func(ctx context.Context, p auth.Principal) error
}

func (f fakeAuthService) Login(ctx context.Context, email, password string) (*auth.LoginResult, error) {
	return f.login(ctx, email, password)
}

func (f fakeAuthService) Refresh(ctx context.Context, token string) (*auth.LoginResult, error) {
	return f.refresh(ctx, token)
}

func (f fakeAuthService) Logout(ctx context.Context, p auth.Principal) error {
	return f.logout(ctx, p)
}

func newAuthOpsTestRouter(svc authService, withPrincipal bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{authHandler: &authHandler{svc: svc}}
	r := gin.New()
	if withPrincipal {
		p := auth.Principal{StaffID: uuid.New(), JTI: uuid.New()}
		r.Use(func(c *gin.Context) {
			c.Set("auth.principal", p)
			c.Next()
		})
	}
	oapi.RegisterHandlers(r, h)
	return r
}

func TestOpsAuthLoginHappy(t *testing.T) {
	svc := fakeAuthService{login: func(_ context.Context, email, password string) (*auth.LoginResult, error) {
		if email != "sam@example.com" || password != "secret" {
			t.Fatalf("Login(%q, %q)", email, password)
		}
		return &auth.LoginResult{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 3600}, nil
	}}
	rec := httptest.NewRecorder()
	body := `{"email":"sam@example.com","password":"secret"}`
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var got oapi.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.AccessToken != "access" || got.RefreshToken != "refresh" || got.TokenType != "Bearer" || got.ExpiresIn != 3600 {
		t.Fatalf("body = %+v", got)
	}
}

func TestOpsAuthLoginInvalidCredentials(t *testing.T) {
	svc := fakeAuthService{login: func(context.Context, string, string) (*auth.LoginResult, error) {
		return nil, auth.ErrInvalidCredentials
	}}
	rec := httptest.NewRecorder()
	body := `{"email":"sam@example.com","password":"wrong"}`
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusUnauthorized, "invalid email or password")
}

func TestOpsAuthLoginInvalidToken(t *testing.T) {
	svc := fakeAuthService{login: func(context.Context, string, string) (*auth.LoginResult, error) {
		return nil, auth.ErrInvalidToken
	}}
	rec := httptest.NewRecorder()
	body := `{"email":"sam@example.com","password":"secret"}`
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body)))

	assertErrorStatus(t, rec, http.StatusUnauthorized, "invalid or expired token")
}

func TestOpsAuthLoginMalformedBody(t *testing.T) {
	svc := fakeAuthService{login: func(context.Context, string, string) (*auth.LoginResult, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{not json`)))

	assertErrorStatus(t, rec, http.StatusBadRequest, "invalid request body")
}

func TestOpsAuthLogoutHappy(t *testing.T) {
	svc := fakeAuthService{logout: func(context.Context, auth.Principal) error {
		return nil
	}}
	rec := httptest.NewRecorder()
	newAuthOpsTestRouter(svc, true).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body.String())
	}
}

func TestOpsAuthLogoutNoPrincipal(t *testing.T) {
	svc := fakeAuthService{logout: func(context.Context, auth.Principal) error {
		t.Fatal("service must not be called without a principal")
		return nil
	}}
	rec := httptest.NewRecorder()
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))

	assertErrorStatus(t, rec, http.StatusInternalServerError, "internal error")
}

func TestOpsAuthRefreshMalformedBody(t *testing.T) {
	svc := fakeAuthService{refresh: func(context.Context, string) (*auth.LoginResult, error) {
		t.Fatal("service must not be called for a malformed body")
		return nil, nil
	}}
	rec := httptest.NewRecorder()
	newAuthOpsTestRouter(svc, false).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{not json`)))

	assertErrorStatus(t, rec, http.StatusBadRequest, "invalid request body")
}
