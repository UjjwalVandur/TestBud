package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/service"
)

// SchemaTestExecutor is the interface the handler uses to trigger test runs.
type SchemaTestExecutor interface {
	ExecuteSchemaTests(ctx context.Context, input service.ExecuteSchemaInput) (service.ExecutionRunResult, error)
}

// ExecutionHandler handles API requests for test execution.
type ExecutionHandler struct {
	service SchemaTestExecutor
}

// NewExecutionHandler creates a new ExecutionHandler.
func NewExecutionHandler(svc SchemaTestExecutor) *ExecutionHandler {
	return &ExecutionHandler{service: svc}
}

// executeRequest is the JSON body expected by the Execute endpoint.
type executeRequest struct {
	TargetURL      string            `json:"target_url" binding:"required"`
	AuthHeaders    map[string]string `json:"auth_headers"`
	AltAuthHeaders map[string]string `json:"alt_auth_headers"`
}

// Execute triggers test execution for all test cases under the given schema.
//
//	POST /api/schemas/:id/executions
func (h *ExecutionHandler) Execute(c *gin.Context) {
	schemaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a valid UUID"})
		return
	}

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.ExecuteSchemaTests(c.Request.Context(), service.ExecuteSchemaInput{
		SchemaID:       schemaID,
		TargetURL:      req.TargetURL,
		AuthHeaders:    req.AuthHeaders,
		AltAuthHeaders: req.AltAuthHeaders,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
