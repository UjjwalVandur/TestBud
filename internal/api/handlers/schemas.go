package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/api/middleware"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

const maxSchemaUploadBytes = 5 << 20

// SchemaService is the interface the schema handler uses for all operations.
type SchemaService interface {
	UploadSchema(ctx context.Context, input service.UploadSchemaInput) (service.UploadSchemaResult, error)
	ListSchemas(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) ([]service.SchemaListItem, error)
	GetSchemaDetail(ctx context.Context, schemaID uuid.UUID) (*service.SchemaDetailResult, error)
}

type SchemaHandler struct {
	service SchemaService
}

func NewSchemaHandler(svc SchemaService) *SchemaHandler {
	return &SchemaHandler{service: svc}
}

func (h *SchemaHandler) Upload(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "schema service is not configured"})
		return
	}

	// Derive the uploader identity from the authenticated user in context,
	// not from user-supplied form data (DEV-5 security fix).
	uploadedBy := middleware.AuthenticatedUserID(c.Request.Context())
	if uploadedBy == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	projectID, err := uuid.Parse(c.PostForm("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id must be a valid UUID"})
		return
	}
	version := c.PostForm("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if file.Size > maxSchemaUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "schema file exceeds 5MB limit"})
		return
	}

	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "open uploaded file"})
		return
	}
	defer opened.Close()

	raw, err := io.ReadAll(io.LimitReader(opened, maxSchemaUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read uploaded file"})
		return
	}
	if len(raw) > maxSchemaUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "schema file exceeds 5MB limit"})
		return
	}

	result, err := h.service.UploadSchema(c.Request.Context(), service.UploadSchemaInput{
		ProjectID:  projectID,
		Version:    version,
		UploadedBy: uploadedBy,
		RawBytes:   raw,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// List returns all schemas for the authenticated user.
// Accepts an optional ?project_id= query parameter to filter by project.
//
//	GET /api/schemas
func (h *SchemaHandler) List(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "schema service is not configured"})
		return
	}

	userID := middleware.AuthenticatedUserID(c.Request.Context())
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var projectID uuid.UUID
	if pidStr := c.Query("project_id"); pidStr != "" {
		parsed, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id must be a valid UUID"})
			return
		}
		projectID = parsed
	}

	items, err := h.service.ListSchemas(c.Request.Context(), userID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

// GetByID returns full schema details including endpoints and test case breakdowns.
//
//	GET /api/schemas/:id
func (h *SchemaHandler) GetByID(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "schema service is not configured"})
		return
	}

	schemaID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a valid UUID"})
		return
	}

	result, err := h.service.GetSchemaDetail(c.Request.Context(), schemaID)
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
