package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/service"
)

// CoverageReporter is the interface the handler uses to fetch coverage reports.
type CoverageReporter interface {
	GetCoverageReport(ctx context.Context, schemaID uuid.UUID) (*service.CoverageReportResult, error)
}

// CoverageHandler handles API requests for coverage reports.
type CoverageHandler struct {
	service CoverageReporter
}

// NewCoverageHandler creates a new CoverageHandler.
func NewCoverageHandler(svc CoverageReporter) *CoverageHandler {
	return &CoverageHandler{service: svc}
}

// Get returns the coverage report for the specified schema.
//
//	GET /api/schemas/:id/coverage
func (h *CoverageHandler) Get(c *gin.Context) {
	schemaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a valid UUID"})
		return
	}

	result, err := h.service.GetCoverageReport(c.Request.Context(), schemaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
