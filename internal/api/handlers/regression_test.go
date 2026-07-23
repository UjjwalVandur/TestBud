package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/regression"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

type fakeRegressionService struct {
	report *regression.RegressionReport
	err    error
}

func (f *fakeRegressionService) GetRegressionReport(_ context.Context, _ uuid.UUID) (*regression.RegressionReport, error) {
	return f.report, f.err
}

func TestRegressionHandler_Get_ValidID(t *testing.T) {
	schemaID := uuid.New()
	baseID := uuid.New()

	handler := NewRegressionHandler(&fakeRegressionService{
		report: &regression.RegressionReport{
			BaseSchemaID:   baseID,
			TargetSchemaID: schemaID,
			Added:          []regression.EndpointChange{{Method: "POST", Path: "/pets"}},
			Removed:        []regression.EndpointChange{},
			Modified:       []regression.EndpointChange{},
		},
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: schemaID.String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var report regression.RegressionReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(report.Added) != 1 {
		t.Errorf("Added = %d, want 1", len(report.Added))
	}
	if report.TargetSchemaID != schemaID {
		t.Errorf("TargetSchemaID = %v, want %v", report.TargetSchemaID, schemaID)
	}
}

func TestRegressionHandler_Get_InvalidUUID(t *testing.T) {
	handler := NewRegressionHandler(&fakeRegressionService{})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Get(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegressionHandler_Get_SchemaNotFound(t *testing.T) {
	handler := NewRegressionHandler(&fakeRegressionService{
		err: service.ErrSchemaNotFound,
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestRegressionHandler_Get_InternalError(t *testing.T) {
	handler := NewRegressionHandler(&fakeRegressionService{
		err: errors.New("db exploded"),
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Get(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
