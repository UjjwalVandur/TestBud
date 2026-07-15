package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/coverage"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

// --- Fake CoverageReporter ---

type fakeCoverageReporter struct {
	result *service.CoverageReportResult
	err    error
}

func (f *fakeCoverageReporter) GetCoverageReport(_ context.Context, _ uuid.UUID) (*service.CoverageReportResult, error) {
	return f.result, f.err
}

// --- Tests ---

func TestCoverageHandler_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewCoverageHandler(&fakeCoverageReporter{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/schemas/not-a-uuid/coverage", nil)

	handler.Get(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCoverageHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	schemaID := uuid.New()
	now := time.Now().UTC()

	handler := NewCoverageHandler(&fakeCoverageReporter{
		result: &service.CoverageReportResult{
			SchemaID:    schemaID,
			EndpointPct: 75.50,
			Categories: map[string]coverage.CategoryStats{
				"positive": {Total: 4, Executed: 3, Pct: 75},
				"negative": {Total: 2, Executed: 2, Pct: 100},
			},
			ResponseCodes: map[int]int{200: 3, 400: 2},
			Fields:        coverage.FieldStats{Total: 5, Covered: 4, Pct: 80},
			GeneratedAt:   now,
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Params = gin.Params{{Key: "id", Value: schemaID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/schemas/"+schemaID.String()+"/coverage", nil)

	handler.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp service.CoverageReportResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.SchemaID != schemaID {
		t.Errorf("schema_id: got %v, want %v", resp.SchemaID, schemaID)
	}
	if resp.EndpointPct != 75.50 {
		t.Errorf("endpoint_pct: got %v, want 75.50", resp.EndpointPct)
	}
	if resp.Categories["positive"].Pct != 75 {
		t.Errorf("positive pct: got %v, want 75", resp.Categories["positive"].Pct)
	}
	if resp.ResponseCodes[200] != 3 {
		t.Errorf("response code 200: got %d, want 3", resp.ResponseCodes[200])
	}
	if resp.Fields.Covered != 4 {
		t.Errorf("fields covered: got %d, want 4", resp.Fields.Covered)
	}
}

func TestCoverageHandler_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewCoverageHandler(&fakeCoverageReporter{
		err: fmt.Errorf("db connection failed"),
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	schemaID := uuid.New()
	c.Params = gin.Params{{Key: "id", Value: schemaID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/schemas/"+schemaID.String()+"/coverage", nil)

	handler.Get(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
