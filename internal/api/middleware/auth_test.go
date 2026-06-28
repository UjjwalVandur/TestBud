package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubLookup struct {
	userID uuid.UUID
	err    error
}

func (s stubLookup) FindUserIDByAPIKey(_ context.Context, _ string) (uuid.UUID, error) {
	return s.userID, s.err
}

func TestAPIKeyAuth_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(stubLookup{userID: uuid.New()}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(stubLookup{userID: uuid.Nil}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "bad-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	expectedID := uuid.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(stubLookup{userID: expectedID}))
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

func TestAPIKeyAuth_BearerToken(t *testing.T) {
	expectedID := uuid.New()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(stubLookup{userID: expectedID}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer my-api-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyAuth_LookupError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(stubLookup{err: context.DeadlineExceeded}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "some-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
