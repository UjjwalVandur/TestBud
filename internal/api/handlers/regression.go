package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/regression"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

// RegressionReporter is the interface the handler uses to fetch regression reports.
type RegressionReporter interface {
	GetRegressionReport(ctx context.Context, schemaID uuid.UUID) (*regression.RegressionReport, error)
}

// RegressionHandler handles API requests for regression reports.
type RegressionHandler struct {
	service RegressionReporter
}

// NewRegressionHandler creates a new RegressionHandler.
func NewRegressionHandler(svc RegressionReporter) *RegressionHandler {
	return &RegressionHandler{service: svc}
}

// Get returns the regression report for the specified schema compared to
// its predecessor in the same project.
//
//	GET /api/schemas/:id/regression
func (h *RegressionHandler) Get(c *gin.Context) {
	schemaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a valid UUID"})
		return
	}

	result, err := h.service.GetRegressionReport(c.Request.Context(), schemaID)
	if err != nil {
		if errors.Is(err, service.ErrSchemaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "schema not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
