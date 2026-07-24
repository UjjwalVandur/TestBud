package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/service"
)

// --- Fake ExecutionService ---

type fakeExecutionService struct {
	result *service.ExecutionListResult
	err    error
}

func (f *fakeExecutionService) ExecuteSchemaTests(_ context.Context, _ service.ExecuteSchemaInput) (service.ExecutionRunResult, error) {
	return service.ExecutionRunResult{}, nil
}

func (f *fakeExecutionService) ListExecutions(_ context.Context, _ uuid.UUID) (*service.ExecutionListResult, error) {
	return f.result, f.err
}

func TestExecutionHandlerList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	schemaID := uuid.New()

	tests := []struct {
		name       string
		id         string
		lister     *fakeExecutionService
		wantStatus int
	}{
		{
			name: "success",
			id:   schemaID.String(),
			lister: &fakeExecutionService{
				result: &service.ExecutionListResult{
					SchemaID: schemaID,
					Summary:  service.ExecutionSummary{Total: 5, Passed: 4, Failed: 1},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid uuid",
			id:         "bad-id",
			lister:     &fakeExecutionService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service error",
			id:         schemaID.String(),
			lister:     &fakeExecutionService{err: errors.New("db error")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			handler := NewExecutionHandler(tt.lister)
			router.GET("/api/schemas/:id/executions", handler.List)

			req := httptest.NewRequest(http.MethodGet, "/api/schemas/"+tt.id+"/executions", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
