package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type stubLookup struct {
	userID uuid.UUID
	err    error
}

func (s stubLookup) FindUserIDByAPIKey(_ context.Context, _ string) (uuid.UUID, error) {
	return s.userID, s.err
}

func (s stubLookup) GetOrCreateUserByClerkID(_ context.Context, _ string, _ string) (*models.User, error) {
	return &models.User{ID: s.userID}, s.err
}

func TestAuthMiddleware_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubLookup{userID: uuid.New()}, ""))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubLookup{userID: uuid.Nil}, ""))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "bad-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	expectedID := uuid.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubLookup{userID: expectedID}, ""))
	r.GET("/test", func(c *gin.Context) {
		gotID := AuthenticatedUserID(c.Request.Context())
		if gotID != expectedID {
			t.Errorf("AuthenticatedUserID = %v, want %v", gotID, expectedID)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_BearerToken(t *testing.T) {
	expectedID := uuid.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubLookup{userID: expectedID}, ""))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	// The Bearer token tests in the old version used to just fall back to API Key if prefixed with Bearer
	// Now, if it's "Bearer tb_something" we could test that, but currently our middleware expects Clerk JWTs for Bearer.
	// Actually, the new middleware checks "X-API-Key" and then "Authorization" with "Bearer ". 
	// The new middleware ONLY treats Bearer as Clerk tokens!
	// So passing "Bearer my-api-key" will fail as invalid JWT if we check it.
	// We'll skip testing the Clerk JWT here since we don't have a valid mocked JWT, or we can just let it fail.
	// For now, let's just assert it gets unauthorized if passed a fake Bearer token.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_LookupError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubLookup{err: context.DeadlineExceeded}, ""))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "some-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
