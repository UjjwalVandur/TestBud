package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/service"
)

// stubUploader is a minimal SchemaUploader that always succeeds.
type stubUploader struct{}

func (s stubUploader) UploadSchema(_ context.Context, _ service.UploadSchemaInput) (service.UploadSchemaResult, error) {
	return service.UploadSchemaResult{}, nil
}

// stubLookup is a minimal UserLookup that returns a fixed user ID for any key.
type stubLookup struct {
	userID uuid.UUID
}

func (s stubLookup) FindUserIDByAPIKey(_ context.Context, _ string) (uuid.UUID, error) {
	return s.userID, nil
}

func TestRouterHealthEndpoint(t *testing.T) {
	r := NewRouter(RouterDependencies{
		Logger:        logrus.New(),
		SchemaService: stubUploader{},
		UserLookup:    stubLookup{userID: uuid.New()},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterAPIRoutesRequireAuth(t *testing.T) {
	r := NewRouter(RouterDependencies{
		Logger:        logrus.New(),
		SchemaService: stubUploader{},
		UserLookup:    stubLookup{userID: uuid.Nil}, // always returns Nil → invalid key
	})

	// POST to /api/schemas without API key should get 401.
	req := httptest.NewRequest(http.MethodPost, "/api/schemas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/schemas without key: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouterRequestLoggerNilLogger(t *testing.T) {
	// Verify the router doesn't panic with a nil logger.
	r := NewRouter(RouterDependencies{
		Logger:        nil,
		SchemaService: stubUploader{},
		UserLookup:    stubLookup{userID: uuid.New()},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health with nil logger: status = %d, want %d", rec.Code, http.StatusOK)
	}
}
